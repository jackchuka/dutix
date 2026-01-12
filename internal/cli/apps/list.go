package apps

import (
	"fmt"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	var filterQuery string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed applications",
		Long: `Scans and lists all applications found in standard macOS directories:
  - /Applications
  - /System/Applications
  - ~/Applications`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			// Scan apps
			apps, err := ctx.Bridge.ListAllApplications()
			if err != nil {
				return fmt.Errorf("failed to scan apps: %w", err)
			}
			domainApps := domain.FromMacOSAppInfos(apps)

			// Filter if requested
			if filterQuery != "" {
				domainApps = domain.FilterApps(domainApps, filterQuery)
			}

			// Format output
			f := formatter.New(ctx.OutputMode, nil)
			return f.FormatApps(domainApps)
		},
	}

	cmd.Flags().StringVar(&filterQuery, "filter", "", "Filter applications by name or path (case-insensitive)")

	return cmd
}
