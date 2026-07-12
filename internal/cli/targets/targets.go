package targets

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the targets command group
func NewCommand() *cobra.Command {
	targetsCmd := &cobra.Command{
		Use:   "targets",
		Short: "Target management commands",
		Long:  `Commands for listing and querying file type targets, UTIs, and URL schemes.`,
	}

	targetsCmd.AddCommand(newShowCommand())
	targetsCmd.AddCommand(newListCommand())

	return targetsCmd
}
