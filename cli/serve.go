package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tamnd/shirabe/pkg/server"
)

func newServeCmd() *cobra.Command {
	var addr string
	var open, dev bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the search page",
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, reg := newContext()
			srv := server.New(eng, reg, Version)
			srv.Dev = dev
			url := "http://" + displayAddr(addr)
			fmt.Println("shirabe listening on", url)
			if open {
				browse(url)
			}
			return srv.ListenAndServe(cmd.Context(), addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8879", "listen address")
	cmd.Flags().BoolVar(&open, "open", false, "open the page in a browser")
	cmd.Flags().BoolVar(&dev, "dev", false, "serve web/ from disk and enable /dev/cards")
	return cmd
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

func browse(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
