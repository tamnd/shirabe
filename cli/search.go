package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "search <query>...",
		Short: "Run a query and print the merged cards",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, _ := newContext()
			res := eng.Run(cmd.Context(), strings.Join(args, " "))
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				for _, c := range res.Cards {
					if err := enc.Encode(c); err != nil {
						return err
					}
				}
				return nil
			}
			for _, c := range res.Cards {
				fmt.Printf("[%s/%s] %s\n", c.Source, c.Kind, c.Title)
				if c.URL != "" {
					fmt.Println("   ", c.URL)
				}
				if c.Snippet != "" {
					fmt.Println("   ", c.Snippet)
				}
			}
			for _, e := range res.Errors {
				fmt.Fprintf(os.Stderr, "warning: %s: %s\n", e.Source, e.Message)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print cards as JSONL")
	return cmd
}
