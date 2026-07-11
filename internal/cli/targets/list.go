package targets

import (
	"fmt"
	"os"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

func newListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [pattern]",
		Short: "List file extensions and their default handlers",
		Long: `Lists file extensions discovered across all installed applications,
along with the UTI each extension resolves to and the current default
application for that UTI.

An optional pattern filters extensions by case-insensitive substring.

Examples:
  dutix targets list
  dutix targets list txt
  dutix targets list --output json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			pattern := ""
			if len(args) == 1 {
				pattern = args[0]
			}

			if !ctx.QuietMode {
				_, _ = fmt.Fprintln(os.Stderr, "Scanning applications...")
			}

			lister := domain.NewExtensionLister(ctx.Bridge)
			rows, err := lister.ListExtensionDefaults(pattern)
			if err != nil {
				return fmt.Errorf("failed to list extensions: %w", err)
			}

			if !ctx.QuietMode {
				_, _ = fmt.Fprintf(os.Stderr, "Resolved %d extension entries.\n", len(rows))
			}

			f := formatter.New(ctx.OutputMode, nil)
			return f.FormatExtensionDefaults(rows)
		},
	}

	return cmd
}
