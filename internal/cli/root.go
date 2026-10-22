// Package cli wires up the yoink commands.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	"github.com/jaoskii/yoink-cli/internal/banner"
)

// Execute runs the CLI. Ctrl+C cancels the command context for a clean shutdown.
func Execute(version string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := newRootCmd(version).ExecuteContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yoink: %v\n", err)
	}
	return err
}

func newRootCmd(version string) *cobra.Command {
	o := &shareOptions{version: version}
	cmd := &cobra.Command{
		Use:   "yoink <path>...",
		Short: "Share files over your Wi-Fi with a QR code",
		Long: "yoink serves files or folders to devices on your local network.\n" +
			"Scan the QR code with your phone and it yoinks the file. Folders are zipped on the fly.",
		Example: "  yoink ./photo.jpg\n" +
			"  yoink ./dist --once\n" +
			"  yoink report.pdf slides.key --timeout 10m\n" +
			"  yoink ./build.zip --port 8080 --interface en0",
		Args:          cobra.ArbitraryArgs,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runShare(cmd.Context(), o, args, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	o.bind(cmd)
	cmd.SetVersionTemplate("yoink {{.Version}}\n")

	// Banner on the root help (`yoink`, `yoink --help`), not on subcommand help.
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c == cmd {
			banner.Print(c.OutOrStdout(), version)
			fmt.Fprintln(c.OutOrStdout())
		}
		defaultHelp(c, args)
	})
	cmd.AddCommand(newVersionCmd(version))
	return cmd
}
