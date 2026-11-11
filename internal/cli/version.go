package cli

import (
	"fmt"
	"runtime"

	"github.com/jackchuka/dutix/internal/cli/shared"
	"github.com/jackchuka/dutix/internal/version"
	"github.com/spf13/cobra"
)

// versionData is the data of version for the JSON or YAML output
type versionData struct {
	Version   string `json:"version" yaml:"version"`
	Commit    string `json:"commit" yaml:"commit"`
	Date      string `json:"date" yaml:"date"`
	GoVersion string `json:"go_version" yaml:"go_version"`
}

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  `Display the version, git commit, and build date of dutix.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := shared.GetContext(cmd)
			if err != nil {
				return err
			}
			if !ctx.Table() {
				ctx.SetData(versionData{
					Version:   version.Version,
					Commit:    version.Commit,
					Date:      version.Date,
					GoVersion: runtime.Version(),
				})
				return nil
			}
			fmt.Println(version.Info())
			return nil
		},
	}
}
