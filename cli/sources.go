package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newSourcesCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "List registered sources with capabilities and availability",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, reg := newContext()
			statuses := reg.Statuses()
			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(statuses)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tSEARCH\tRESOLVE\tHOSTS\tINTENTS\tSTATUS")
			for _, s := range statuses {
				status := "ok"
				if !s.Available {
					status = "missing"
				}
				_, _ = fmt.Fprintf(w, "%s\t%v\t%v\t%s\t%s\t%s\n",
					s.Name, s.Search, s.Resolve,
					strings.Join(s.Hosts, ","), strings.Join(s.Intents, ","), status)
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print as JSON")
	return cmd
}
