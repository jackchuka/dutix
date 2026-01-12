package apps

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the apps command group
func NewCommand() *cobra.Command {
	appsCmd := &cobra.Command{
		Use:   "apps",
		Short: "Application management commands",
		Long:  `Commands for listing and querying installed applications.`,
	}

	appsCmd.AddCommand(newListCommand())
	appsCmd.AddCommand(newShowCommand())
	appsCmd.AddCommand(newMigrateCommand())

	return appsCmd
}
