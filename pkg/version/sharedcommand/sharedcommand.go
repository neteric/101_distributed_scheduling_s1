package sharedcommand

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/VTeam/k8s-webhook-template/pkg/version"
)

var (
	versionShort   = `Print the version information`
	versionLong    = `Print the version information.`
	versionExample = templates.Examples(`
		# Print %[1]s command version
		%[1]s version`)
)

// NewCmdVersion prints out the release version info for this command binary.
// It is used as a subcommand of a parent command.
func NewCmdVersion(parentCommand string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		Short:   versionShort,
		Long:    versionLong,
		Example: fmt.Sprintf(versionExample, parentCommand),
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(os.Stdout, "%s version: %s\n", parentCommand, version.Get())
		},
	}

	return cmd
}
