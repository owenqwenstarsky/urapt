package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (r *Root) versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the urapt CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(r.version)
		},
	}
}
