package targets

import (
	"fmt"
	"strings"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/domain"
	"github.com/jackchuka/dutix/internal/formatter"
	"github.com/spf13/cobra"
)

func newShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <target>",
		Short: "Show current default handler for a target",
		Long: `Shows the current default application for a file extension, UTI, or URL scheme.

Examples:
  dutix targets show txt
  dutix targets show public.plain-text
  dutix targets show http`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}

			target := args[0]

			// Determine target type based on input
			targetKind, identifier := inferTargetKind(target)

			// Get current default
			var defaultApp string
			var availableApps []string
			var utis []string

			switch targetKind {
			case domain.TargetKindExtension:
				// For extensions, resolve to UTI first to get the default
				utis, err = ctx.Bridge.ResolveUTIsForExtension(identifier)
				if err != nil {
					return fmt.Errorf("failed to resolve UTIs for extension %s: %w", identifier, err)
				}
				if len(utis) > 0 {
					defaultApp, err = ctx.Bridge.GetDefaultAppForUTI(utis[0])
					if err != nil {
						return fmt.Errorf("failed to get default for UTI %s: %w", utis[0], err)
					}
					availableApps, err = ctx.Bridge.ListAppsForUTI(utis[0])
					if err != nil {
						return fmt.Errorf("failed to list apps for UTI %s: %w", utis[0], err)
					}
				}
			case domain.TargetKindUTI:
				defaultApp, err = ctx.Bridge.GetDefaultAppForUTI(identifier)
				if err != nil {
					return fmt.Errorf("failed to get default for UTI %s: %w", identifier, err)
				}
				availableApps, err = ctx.Bridge.ListAppsForUTI(identifier)
				if err != nil {
					return fmt.Errorf("failed to list apps for UTI %s: %w", identifier, err)
				}

			case domain.TargetKindScheme:
				defaultApp, err = ctx.Bridge.GetDefaultAppForScheme(identifier)
				if err != nil {
					return fmt.Errorf("failed to get default for scheme %s: %w", identifier, err)
				}
				availableApps, err = ctx.Bridge.ListAppsForScheme(identifier)
				if err != nil {
					return fmt.Errorf("failed to list apps for scheme %s: %w", identifier, err)
				}
			default:
				return fmt.Errorf("unknown target kind: %s", targetKind)
			}

			// Create target for formatter
			targetObj := domain.Target{
				Kind:       targetKind,
				Identifier: identifier,
			}

			// Display using formatter
			if !ctx.QuietMode {
				f := formatter.New(ctx.OutputMode, nil)
				return f.FormatTargetDetails(targetObj, defaultApp, utis, availableApps)
			}

			return nil
		},
	}

	return cmd
}

// inferTargetKind attempts to determine the target kind from the input
func inferTargetKind(target string) (domain.TargetKind, string) {
	// Remove leading dot if present (e.g., ".txt" -> "txt")
	target = strings.TrimPrefix(target, ".")

	// If it contains dots, likely a UTI (e.g., "public.plain-text")
	if strings.Contains(target, ".") {
		return domain.TargetKindUTI, target
	}

	// Common URL schemes
	schemes := []string{"http", "https", "ftp", "ftps", "mailto", "ssh"}
	for _, scheme := range schemes {
		if strings.EqualFold(target, scheme) {
			return domain.TargetKindScheme, strings.ToLower(target)
		}
	}

	// Default to extension
	return domain.TargetKindExtension, strings.ToLower(target)
}
