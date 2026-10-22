# yoink

Share files across your Wi-Fi with a QR code. Your phone yoinks it.

```
$ yoink ./trip-photos

  ██╗   ██╗ ██████╗ ██╗███╗   ██╗██╗  ██╗
  ╚██╗ ██╔╝██╔═══██╗██║████╗  ██║██║ ██╔╝
   ╚████╔╝ ██║   ██║██║██╔██╗ ██║█████╔╝
    ╚██╔╝  ██║   ██║██║██║╚██╗██║██╔═██╗
     ██║   ╚██████╔╝██║██║ ╚████║██║  ██╗
     ╚═╝    ╚═════╝ ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝

  share files over Wi-Fi with a QR code · v0.1.0

  sharing trip-photos.zip (42 files, 186.3 MB, zipped on the fly)
  on en0 · 192.168.1.23

  ▄▄▄▄▄▄▄ ▄ ▄▄ ▄▄▄▄▄▄▄
  █ ▄▄▄ █ ▀█▄█ █ ▄▄▄ █      ← scan with your phone camera
  ...

  http://192.168.1.23:53421/k3x9q7.../

  ↓ 192.168.1.40  ████████████░░░░░░░░  62%  115.5 MB / 186.3 MB  24.1 MB/s
```

No app on the phone, no cloud, no account. One static binary for macOS, Linux and Windows.

---

## 1. Requirements

| Need | Version | Install |
|---|---|---|
| **Go** | 1.21 or newer | macOS: `brew install go` · Windows: `winget install GoLang.Go` · Linux: your package manager or [go.dev/dl](https://go.dev/dl/) |
| **Git** | any | to clone the repo |
| make | optional | only for the `make` shortcuts (preinstalled on macOS/Linux) |

The repo pins Go 1.26; any Go 1.21+ downloads that toolchain automatically on first build. Check yours with `go version`.

## 2. Clone

```bash
git clone https://github.com/jaoskii/yoink-cli.git
cd yoink-cli
```

## 3. Install

The install script checks Go, downloads the dependencies, builds `yoink`, installs it into Go's `bin` folder, and offers to add that folder to your `PATH`.

**macOS / Linux**

```bash
./scripts/install.sh
```

**Windows (PowerShell)**

```powershell
powershell -ExecutionPolicy Bypass -File scripts\install.ps1
```

Then open a new terminal and check it works:

```bash
yoink version
```

<details>
<summary>Other ways to install</summary>

**With make** (macOS / Linux):

```bash
make install        # downloads deps, builds, installs to $(go env GOPATH)/bin
```

**Straight from GitHub**, without cloning:

```bash
go install github.com/jaoskii/yoink-cli/cmd/yoink@latest
```

**Without installing**, run from the repo:

```bash
make build && ./bin/yoink ./README.md
```

If `yoink` is "command not found", Go's bin folder isn't on your PATH. Add it (zsh shown):

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

</details>

### Update / uninstall

```bash
git pull && ./scripts/install.sh   # update
make uninstall                     # remove (or delete $(go env GOPATH)/bin/yoink)
```

---

## 4. Usage

```
yoink <path>... [flags]
```

Run it, scan the QR code with your phone camera, tap **Download**. The phone must be on the same Wi-Fi. Press **Ctrl+C** to stop sharing.

### Commands

| Command | What it does |
|---|---|
| `yoink <file>` | Share one file. Downloads can resume if interrupted |
| `yoink <folder>` | Share a folder, zipped on the fly (no temp files) |
| `yoink <path> <path>...` | Share several files/folders as one zip |
| `yoink version` | Print version, OS and architecture |
| `yoink --help` | Show all flags |
| `yoink completion zsh\|bash\|fish\|powershell` | Generate shell completions |

### Flags

| Flag | Default | What it does |
|---|---|---|
| `--once` | off | Stop after the first completed download |
| `-t, --timeout <dur>` | none | Stop after a duration, e.g. `10m`, `1h` |
| `-p, --port <n>` | random | Use a fixed port (handy for firewall rules) |
| `-i, --interface <name>` | auto | Use a specific network interface, e.g. `en0`, `wlan0`, `Wi-Fi` |
| `--qr-png <file>` | — | Also save the QR code as a PNG image |
| `--no-qr` | off | Print the link only |
| `--zip` | off | Zip a single file instead of sending it as-is |
| `-q, --quiet` | off | Print only the link (for scripts) |

### Examples

```bash
yoink ~/Downloads/photo.jpg                 # share a photo
yoink ./dist --once                         # send a build folder, stop when downloaded
yoink report.pdf slides.key --timeout 10m   # two files as one zip, auto-stop after 10 min
yoink ./video.mp4 --port 8080 -i en0        # fixed port on Wi-Fi interface en0
yoink ./file.zip --qr-png qr.png            # save the QR to share in chat
yoink ./file.zip -q | pbcopy                # copy just the link (macOS)
```

---

## How it works

- Picks your Wi-Fi/Ethernet address (skipping VPN, Docker and VM interfaces) and listens **only** on that address.
- The link contains a random 130-bit token; every other path returns 404.
- Single files support HTTP range requests, so interrupted downloads resume. Folders stream as a zip; photos, videos and archives are stored without recompressing.
- The QR code is drawn with explicit colors, so it scans on light and dark terminal themes.

## Security

The link is plain HTTP on your local network. The token stops others guessing the link, but anyone on the same network could see the file in transit. Avoid sensitive files on shared or public Wi-Fi. Encrypted transfer (`yoink send` / `yoink get`) is on the roadmap.

## Troubleshooting

If no device connects within 45 seconds, yoink prints hints for your OS.

| Problem | Fix |
|---|---|
| macOS asks about incoming connections | Click **Allow** (System Settings → Network → Firewall) |
| Phone can't open the link on Windows | Allow `yoink.exe` in Windows Firewall; set the Wi-Fi to a **Private** network |
| Phone can't open the link on Linux | `sudo ufw allow <port>/tcp` (use `--port` for a stable rule) |
| Running inside WSL2 | Phones can't reach WSL's NAT; run the Windows `yoink.exe` instead |
| Guest / office Wi-Fi | These often block devices from reaching each other; try a home network or hotspot |
| Wrong network picked | Pass `--interface`, e.g. `-i en0` (list with `ifconfig` / `ip addr` / `ipconfig`) |
| `yoink: command not found` | Go's bin folder isn't on PATH; see *Other ways to install* above |

---

## Development

```bash
make            # list all targets
make deps       # download dependencies
make build      # build bin/yoink
make run ARGS="./README.md --once"
make test       # unit + integration tests
make race       # tests with the race detector
make vet        # static checks
make lint       # golangci-lint (install separately)
make cross      # static binaries for 6 platforms into dist/
make clean      # remove bin/ and dist/
```

Project layout:

```
cmd/yoink/          entry point
internal/cli/       commands and flags (cobra)
internal/server/    HTTP handler, landing page, downloads
internal/archive/   streaming zip
internal/netinfo/   LAN address detection
internal/qr/        terminal + PNG QR codes
internal/progress/  live progress line
internal/banner/    YOINK wordmark
internal/ansi/      terminal color detection (NO_COLOR, Windows VT)
internal/hints/     OS-specific connection hints
scripts/            install scripts
```

## Roadmap

- [x] Share files and folders over LAN with a QR code
- [ ] `yoink receive`: upload page so the phone can send files to the computer
- [ ] `yoink send` / `yoink get`: encrypted transfer with a short code
- [ ] `yoink relay`: relay server for transfers across networks
- [ ] Homebrew, Scoop and winget packages
