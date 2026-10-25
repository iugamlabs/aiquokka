//go:build !tray || (!linux && !darwin) || (darwin && !cgo)

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newTrayCmd() *cobra.Command {
	return &cobra.Command{Use: "tray", Aliases: []string{"bar", "menu", "systray"}, Short: "Use the optional macOS / Linux tray build", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("tray requires the optional macOS/Linux build: go build -tags tray -o aiquokka-tray . (macOS also requires CGO_ENABLED=1 and Xcode command line tools)")
	}}
}
