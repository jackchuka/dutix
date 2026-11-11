package cli

import (
	"os"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/spf13/cobra"
)

// NewExportCommand creates the export command
func NewExportCommand() *cobra.Command {
	var extensions, utis, schemes []string
	var opts domain.ExportOptions

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export default applications to a defaults file",
		Long: `Writes the current default applications as a defaults file to stdout,
for "dutix apply" to restore them, e.g. on another Mac.

Without target flags, it exports the defaults you chose: the overrides
stored in LaunchServices, as set in Finder ("Change All"), System Settings,
or dutix. URL schemes and UTIs that only one installed app can handle are
left out, since apps claim those for themselves; --all includes them.
Overrides for apps that aren't installed are left out.

With --extensions, --utis or --schemes, it exports the current default of
each given target instead.

Apps are written by bundle ID, with their names as comments.

Examples:
  dutix export > defaults.yaml
  dutix export --all
  dutix export --extensions md,txt --schemes http,https`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			exporter := domain.NewExporter(ctx.Bridge)
			var file *domain.DefaultsFile
			if len(extensions)+len(utis)+len(schemes) > 0 {
				if opts.All {
					return shared.Usagef("--all only applies without target flags")
				}
				targets, err := shared.ParseTargetSpecs(extensions, utis, schemes)
				if err != nil {
					return err
				}
				file, err = exporter.ExportTargets(targets)
				if err != nil {
					return err
				}
			} else {
				file, err = exporter.ExportUserDefaults(opts)
				if err != nil {
					return err
				}
			}

			if !ctx.Table() {
				ctx.SetData(file)
				return nil
			}
			return file.Encode(os.Stdout)
		},
	}

	cmd.Flags().StringSliceVar(&extensions, "extensions", nil, "File extensions (comma-separated, e.g., txt,md,json)")
	cmd.Flags().StringSliceVar(&utis, "utis", nil, "UTI identifiers (comma-separated)")
	cmd.Flags().StringSliceVar(&schemes, "schemes", nil, "URL schemes (comma-separated, e.g., http,https)")
	cmd.Flags().BoolVar(&opts.All, "all", false, "Include URL schemes and UTIs that only one installed app can handle")

	return cmd
}
