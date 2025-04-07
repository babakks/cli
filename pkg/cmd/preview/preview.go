package preview

import (
	cmdPrompter "github.com/cli/cli/v2/pkg/cmd/preview/prompter"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/spf13/cobra"
)

func NewCmdPreview(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview <command>",
		Short: "Execute previews for gh features",
	}

	cmdutil.DisableAuthCheck(cmd)

	cmd.AddCommand(cmdPrompter.NewCmdPrompter(f, nil))

	return cmd
}
