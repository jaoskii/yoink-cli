package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/jaoskii/yoink-cli/internal/banner"
	"github.com/jaoskii/yoink-cli/internal/hints"
	"github.com/jaoskii/yoink-cli/internal/netinfo"
	"github.com/jaoskii/yoink-cli/internal/progress"
	"github.com/jaoskii/yoink-cli/internal/qr"
	"github.com/jaoskii/yoink-cli/internal/server"
	"github.com/jaoskii/yoink-cli/internal/token"
	"github.com/jaoskii/yoink-cli/internal/units"
)

// hintDelay is how long to wait for a first connection before suggesting fixes.
const hintDelay = 45 * time.Second

type shareOptions struct {
	version string

	port    int
	once    bool
	timeout time.Duration
	iface   string
	qrPNG   string
	noQR    bool
	zip     bool
	quiet   bool
}

func (o *shareOptions) bind(cmd *cobra.Command) {
	f := cmd.Flags()
	f.IntVarP(&o.port, "port", "p", 0, "port to listen on (0 = random free port)")
	f.BoolVar(&o.once, "once", false, "stop after the first completed download")
	f.DurationVarP(&o.timeout, "timeout", "t", 0, "stop after this long, e.g. 10m (0 = no limit)")
	f.StringVarP(&o.iface, "interface", "i", "", "network interface to use, e.g. en0, wlan0, Wi-Fi")
	f.StringVar(&o.qrPNG, "qr-png", "", "also save the QR code as a PNG file")
	f.BoolVar(&o.noQR, "no-qr", false, "print the link only, no QR code")
	f.BoolVar(&o.zip, "zip", false, "zip a single file instead of sending it as-is")
	f.BoolVarP(&o.quiet, "quiet", "q", false, "print only the link (for scripts)")
}

// monitor adapts progress.Tracker to server.Monitor.
type monitor struct{ t *progress.Tracker }

func (m monitor) Seen(remote string) { m.t.Seen(remote) }
func (m monitor) Begin(remote string, total int64) server.Transfer {
	return m.t.Begin(remote, total)
}

func runShare(ctx context.Context, o *shareOptions, args []string, stdout, stderr io.Writer) error {
	if o.port < 0 || o.port > 65535 {
		return fmt.Errorf("invalid --port %d", o.port)
	}
	share, err := server.NewShare(args, o.zip, time.Now())
	if err != nil {
		return err
	}
	addr, err := netinfo.LANIP(o.iface)
	if err != nil {
		return err
	}
	// Bind to the LAN address only, not 0.0.0.0, so other interfaces stay closed.
	ln, err := net.Listen("tcp", net.JoinHostPort(addr.IP.String(), strconv.Itoa(o.port)))
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr.IP, err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	tok := token.New()
	link := fmt.Sprintf("http://%s/%s/", ln.Addr(), tok)

	tracker := progress.New(stderr, o.quiet)
	done := make(chan struct{})
	var doneOnce sync.Once
	srv := &http.Server{
		Handler: server.NewHandler(share, tok, server.Options{
			Monitor: monitor{tracker},
			OnComplete: func() {
				if o.once {
					doneOnce.Do(func() { close(done) })
				}
			},
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0), // client disconnects are normal
	}

	if o.qrPNG != "" {
		if err := qr.WritePNG(o.qrPNG, link); err != nil {
			ln.Close()
			return fmt.Errorf("write QR PNG: %w", err)
		}
	}
	if o.quiet {
		fmt.Fprintln(stdout, link)
	} else {
		printIntro(stdout, stderr, share, addr, link, o)
	}

	if o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	go tracker.Run(ctx)
	hint := time.AfterFunc(hintDelay, func() {
		if !tracker.SeenAny() {
			tracker.Println("\n  " + strings.Join(hints.NoConnection(runtime.GOOS, port, hints.IsWSL()), "\n  ") + "\n")
		}
	})
	defer hint.Stop()

	var reason string
	select {
	case <-done:
		reason = "Download complete. Stopped."
	case <-ctx.Done():
		reason = "Stopped."
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = "Timeout reached. Stopped."
		}
	case err := <-serveErr:
		tracker.Stop()
		return err
	}
	tracker.Stop()

	// Give a response that is just finishing a moment to flush, then force close.
	sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		_ = srv.Close()
	}
	if !o.quiet {
		fmt.Fprintln(stderr, "  "+reason)
	}
	return nil
}

func printIntro(stdout, stderr io.Writer, s *server.Share, addr netinfo.Addr, link string, o *shareOptions) {
	detail := units.Bytes(s.Size)
	if s.IsZip {
		detail = fmt.Sprintf("%d file%s, %s, zipped on the fly", s.FileCount, plural(s.FileCount), detail)
	}
	banner.Print(stderr, o.version)
	fmt.Fprintf(stderr, "\n  sharing %s (%s)\n", s.Name, detail)
	fmt.Fprintf(stderr, "  on %s · %s\n", addr.Iface, addr.IP)
	if n := len(s.Skipped()); n > 0 {
		fmt.Fprintf(stderr, "  skipped %d symlink%s or special file%s\n", n, plural(n), plural(n))
	}
	fmt.Fprintln(stderr)

	if !o.noQR {
		if err := qr.Terminal(stderr, link, "  "); err == nil {
			fmt.Fprintln(stderr)
		}
	}
	fmt.Fprint(stderr, "  ")
	fmt.Fprintln(stdout, link) // stdout, so `yoink file | pbcopy` grabs just the link
	fmt.Fprintln(stderr)

	fmt.Fprintln(stderr, "  Scan with your phone camera, or open the link on any device on this Wi-Fi.")
	fmt.Fprintln(stderr, "  Plain HTTP: others on this network could see the file in transit.")
	if o.qrPNG != "" {
		fmt.Fprintf(stderr, "  QR code saved to %s\n", o.qrPNG)
	}
	if o.once {
		fmt.Fprintln(stderr, "  Stops after the first completed download.")
	}
	if o.timeout > 0 {
		fmt.Fprintf(stderr, "  Stops after %s.\n", o.timeout)
	}
	fmt.Fprintln(stderr)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
