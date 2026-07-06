// Package cli wires the cobra commands.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tamnd/shirabe/pkg/defaults"
	"github.com/tamnd/shirabe/pkg/engine"
	"github.com/tamnd/shirabe/pkg/source"
)

// Set via ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func newContext() (*engine.Engine, *source.Registry) {
	reg, warnings := defaults.Registry()
	for _, w := range warnings {
		fmt.Println("warning:", w)
	}
	return engine.New(reg), reg
}

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "shirabe",
		Short: "One search box over every source you already have",
		Long: "shirabe (調べ) serves a Google-style answer page over native providers\n" +
			"and any site CLI that prints JSON. Queries fan out, URLs resolve, and\n" +
			"everything renders as rich typed cards.",
		SilenceUsage: true,
	}
	root.AddCommand(newServeCmd(), newSearchCmd(), newResolveCmd(), newSourcesCmd(), newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("shirabe %s (%s, %s)\n", Version, Commit, Date)
		},
	}
}
