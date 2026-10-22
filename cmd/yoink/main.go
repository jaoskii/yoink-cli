// Command yoink shares files over the local network with a QR code.
package main

import (
	"os"

	"github.com/jaoskii/yoink-cli/internal/cli"
)

// version is set at build time: -ldflags "-X main.version=v0.1.0".
var version = "dev"

func main() {
	if err := cli.Execute(version); err != nil {
		os.Exit(1)
	}
}
