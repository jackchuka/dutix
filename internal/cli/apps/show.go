package apps

import (
	"fmt"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <app|bundle-path|bundle-id>",
		Short: "Show application details and supported content types",
		Long: `Displays information about an application including:
  - Application path and bundle ID
  - Supported content types (UTIs)
  - Supported URL schemes

Examples:
  dutix apps show "Visual Studio Code"
  dutix apps show Safari
  dutix apps show TextEdit
  dutix apps show com.microsoft.VSCode
  dutix apps show ~/Applications/Foo.app`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			appQuery := args[0]

			// Find the application
			matchedAppPtr, err := domain.ResolveApp(ctx.Bridge, appQuery)
			if err != nil {
				return err
			}
			matchedApp := *matchedAppPtr

			// Get supported types
			docTypes, err := ctx.Bridge.ListSupportedDocumentTypes(matchedApp.Path)
			if err != nil {
				return fmt.Errorf("failed to get supported types: %w", err)
			}
			defaultTypes, err := ctx.Bridge.ListDefaultDocumentTypesForApp(matchedApp.Path)
			if err != nil {
				return fmt.Errorf("failed to get default types: %w", err)
			}

			// Display information using formatter
			if !ctx.QuietMode {
				f := formatter.New(ctx.OutputMode, nil)
				return f.FormatAppDetails(
					matchedApp,
					domain.FromMacOSDocumentTypes(docTypes),
					domain.FromMacOSDocumentTypes(defaultTypes),
				)
			}

			return nil
		},
	}

	return cmd
}
