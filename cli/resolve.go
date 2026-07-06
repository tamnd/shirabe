package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newResolveCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "resolve <url>",
		Short: "Dereference a URL into structured cards",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, _ := newContext()
			res := eng.Run(cmd.Context(), args[0])
			if len(res.Cards) == 0 {
				for _, e := range res.Errors {
					fmt.Fprintf(os.Stderr, "%s: %s\n", e.Source, e.Message)
				}
				return fmt.Errorf("no source could resolve %s", args[0])
			}
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
				if c.Snippet != "" {
					fmt.Println("   ", c.Snippet)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print cards as JSONL")
	return cmd
}
