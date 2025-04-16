package prompter

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type prompterOptions struct {
	IO     *iostreams.IOStreams
	Config func() (gh.Config, error)

	PromptsToRun []func(prompter.Prompter) error
}

func NewCmdPrompter(f *cmdutil.Factory, runF func(*prompterOptions) error) *cobra.Command {
	opts := &prompterOptions{
		IO:     f.IOStreams,
		Config: f.Config,
	}

	const (
		selectPrompt          = "select"
		multiSelectPrompt     = "multi-select"
		inputPrompt           = "input"
		passwordPrompt        = "password"
		confirmPrompt         = "confirm"
		authTokenPrompt       = "auth-token"
		confirmDeletionPrompt = "confirm-deletion"
		inputHostnamePrompt   = "input-hostname"
		markdownEditorPrompt  = "markdown-editor"
	)

	prompterTypeFuncMap := map[string]func(prompter.Prompter) error{
		selectPrompt:          runSelect,
		multiSelectPrompt:     runMultiSelect,
		inputPrompt:           runInput,
		passwordPrompt:        runPassword,
		confirmPrompt:         runConfirm,
		authTokenPrompt:       runAuthToken,
		confirmDeletionPrompt: runConfirmDeletion,
		inputHostnamePrompt:   runInputHostname,
		markdownEditorPrompt:  runMarkdownEditor,
	}

	allPromptsOrder := []string{
		selectPrompt,
		multiSelectPrompt,
		inputPrompt,
		passwordPrompt,
		confirmPrompt,
		authTokenPrompt,
		confirmDeletionPrompt,
		inputHostnamePrompt,
		markdownEditorPrompt,
	}

	cmd := &cobra.Command{
		Use:   "prompter [prompt type]",
		Short: "Execute a test program to preview the prompter",
		Long: heredoc.Doc(`
			Execute a test program to preview the prompter.
			Without an argument, all prompts will be run.

			Available prompt types:
			- select
			- multi-select
			- input
			- password
			- confirm
			- auth-token
			- confirm-deletion
			- input-hostname
			- markdown-editor
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if runF != nil {
				return runF(opts)
			}

			if len(args) == 0 {
				// All prompts, in a fixed order
				for _, promptType := range allPromptsOrder {
					f := prompterTypeFuncMap[promptType]
					opts.PromptsToRun = append(opts.PromptsToRun, f)
				}
			} else {
				// Only the one specified
				for _, arg := range args {
					f, ok := prompterTypeFuncMap[arg]
					if !ok {
						return fmt.Errorf("unknown prompter type: %q", arg)
					}
					opts.PromptsToRun = append(opts.PromptsToRun, f)
				}
			}

			return prompterRun(opts)
		},
	}

	return cmd
}

func prompterRun(opts *prompterOptions) error {
	editor, err := cmdutil.DetermineEditor(opts.Config)
	if err != nil {
		return err
	}

	p := prompter.New(editor, opts.IO.In, opts.IO.Out, opts.IO.ErrOut)

	for _, f := range opts.PromptsToRun {
		if err := f(p); err != nil {
			return err
		}
		// Newline for readability
		fmt.Println()
	}

	return nil
}

func runSelect(p prompter.Prompter) error {
	fmt.Println("Demonstrating Single Select")
	cuisines := []string{"Italian", "Greek", "Indian", "Japanese", "American"}
	favorite, err := p.Select("Favorite cuisine?", "Italian", cuisines)
	if err != nil {
		return err
	}
	fmt.Printf("Favorite cuisine: %s\n", cuisines[favorite])
	return nil
}

func runMultiSelect(p prompter.Prompter) error {
	fmt.Println("Demonstrating Multi Select")
	cuisines := []string{"Italian", "Greek", "Indian", "Japanese", "American"}
	favorites, err := p.MultiSelect("Favorite cuisines?", []string{}, cuisines)
	if err != nil {
		return err
	}
	for _, f := range favorites {
		fmt.Printf("Favorite cuisine: %s\n", cuisines[f])
	}
	return nil
}

func runInput(p prompter.Prompter) error {
	fmt.Println("Demonstrating Text Input")
	text, err := p.Input("Favorite meal?", "Breakfast")
	if err != nil {
		return err
	}
	fmt.Printf("You typed: %s\n", text)
	return nil
}

func runPassword(p prompter.Prompter) error {
	fmt.Println("Demonstrating Password Input")
	safeword, err := p.Password("Safe word?")
	if err != nil {
		return err
	}
	fmt.Printf("Safe word: %s\n", safeword)
	return nil
}

func runConfirm(p prompter.Prompter) error {
	fmt.Println("Demonstrating Confirmation")
	confirmation, err := p.Confirm("Are you sure?", true)
	if err != nil {
		return err
	}
	fmt.Printf("Confirmation: %t\n", confirmation)
	return nil
}

func runAuthToken(p prompter.Prompter) error {
	fmt.Println("Demonstrating Auth Token (can't be blank)")
	token, err := p.AuthToken()
	if err != nil {
		return err
	}
	fmt.Printf("Auth token: %s\n", token)
	return nil
}

func runConfirmDeletion(p prompter.Prompter) error {
	fmt.Println("Demonstrating Deletion Confirmation")
	err := p.ConfirmDeletion("delete-me")
	if err != nil {
		return err
	}
	fmt.Println("Item deleted")
	return nil
}

func runInputHostname(p prompter.Prompter) error {
	fmt.Println("Demonstrating Hostname")
	hostname, err := p.InputHostname()
	if err != nil {
		return err
	}
	fmt.Printf("Hostname: %s\n", hostname)
	return nil
}

func runMarkdownEditor(p prompter.Prompter) error {
	defaultText := "default text value"

	fmt.Println("Demonstrating Markdown Editor with blanks allowed and default text")
	editorText, err := p.MarkdownEditor("Edit your text:", defaultText, true)
	if err != nil {
		return err
	}
	fmt.Printf("Returned text: %s\n\n", editorText)

	fmt.Println("Demonstrating Markdown Editor with blanks disallowed and default text")
	editorText2, err := p.MarkdownEditor("Edit your text:", defaultText, false)
	if err != nil {
		return err
	}
	fmt.Printf("Returned text: %s\n\n", editorText2)

	fmt.Println("Demonstrating Markdown Editor with blanks disallowed and no default text")
	editorText3, err := p.MarkdownEditor("Edit your text:", "", false)
	if err != nil {
		return err
	}
	fmt.Printf("Returned text: %s\n", editorText3)
	return nil
}
