package update

import (
	"context"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/api"
	ghContext "github.com/cli/cli/v2/context"
	"github.com/cli/cli/v2/git"
	shared "github.com/cli/cli/v2/pkg/cmd/pr/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/shurcooL/githubv4"
	"github.com/spf13/cobra"
)

type UpdateOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	GitClient  *git.Client
	Remotes    func() (ghContext.Remotes, error)
	Branch     func() (string, error)

	Finder   shared.PRFinder
	Prompter shared.EditPrompter

	SelectorArg string
	Interactive bool
	SkipLocal   bool
	UpdateLocal bool
	Rebase      bool
}

func NewCmdUpdate(f *cmdutil.Factory, runF func(*UpdateOptions) error) *cobra.Command {
	opts := &UpdateOptions{
		IO:         f.IOStreams,
		HttpClient: f.HttpClient,
		GitClient:  f.GitClient,
		Remotes:    f.Remotes,
		Branch:     f.Branch,
		Prompter:   f.Prompter,
	}

	cmd := &cobra.Command{
		Use:   "update [<number> | <url> | <branch>]",
		Short: "Update a pull request branch",
		Long: heredoc.Docf(`
			Update a pull request branch with latest changes of the base branch.

			Without an argument, the pull request that belongs to the current branch is selected.

			The default behavior is to update with a merge (i.e., merging the base branch into the
			the PR's branch). To reconcile the changes with rebasing on top of the base branch the
			%[1]s--rebase%[1]s option should be provided.

			If the current local branch tracks the PR branch, the command will prompt for pulling
			the latest changes. To skip the prompt, either one of %[1]s--update-local%[1]s or
			%[1]s--skip-local%[1]s options should be provided.
			
			In non-interactive mode, the command will not update the local branch.
		`, "`"),
		Example: heredoc.Doc(`
			$ gh pr update 23"
			$ gh pr update 23 --update-local"
			$ gh pr update 23 --skip-local"
			$ gh pr update 23 --rebase"
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Finder = shared.NewFinder(f)

			if len(args) > 0 {
				opts.SelectorArg = args[0]
			}

			if err := cmdutil.MutuallyExclusive(
				"specify only one of `--skip-local` or `--update-local`",
				opts.SkipLocal,
				opts.UpdateLocal,
			); err != nil {
				return err
			}

			if err := cmdutil.MutuallyExclusive(
				"specify only one of `--rebase` or `--update-local`",
				opts.Rebase,
				opts.UpdateLocal,
			); err != nil {
				return err
			}

			if runF != nil {
				return runF(opts)
			}

			return updateRun(opts)
		},
	}

	cmd.Flags().BoolVarP(&opts.SkipLocal, "skip-local", "s", false, "Do not update local branch")
	cmd.Flags().BoolVarP(&opts.UpdateLocal, "update-local", "u", false, "Update local branch")
	cmd.Flags().BoolVar(&opts.Rebase, "rebase", false, "Update PR branch by rebasing on top of latest base branch")

	_ = cmdutil.RegisterBranchCompletionFlags(f.GitClient, cmd, "base")

	return cmd
}

func updateRun(opts *UpdateOptions) error {
	ctx := context.Background()

	findOptions := shared.FindOptions{
		Selector: opts.SelectorArg,
		Fields:   []string{"id", "headRefName", "headRefOid"},
	}
	pr, repo, err := opts.Finder.Find(findOptions)
	if err != nil {
		return err
	}

	currentBranch, err := opts.Branch()
	if err != nil {
		return err
	}

	cs := opts.IO.ColorScheme()

	if opts.UpdateLocal {
		// We need to make sure the command is run on the right branch, so that we
		// can update the local with `git pull`.
		if currentBranch != pr.HeadRefName {
			return fmt.Errorf("current branch does not track the PR branch; consider switching to the correct branch or running the command without the `--update-local` option")
		}
		if !opts.GitClient.HasLocalBranch(ctx, pr.HeadRefName) {
			return fmt.Errorf("current branch does not track the PR branch; consider setting the branch to track the PR branch or running the command without the `--update-local` option")
		}
	}

	updateMethod := githubv4.PullRequestBranchUpdateMethodMerge
	if opts.Rebase {
		updateMethod = githubv4.PullRequestBranchUpdateMethodRebase
	}

	params := githubv4.UpdatePullRequestBranchInput{
		PullRequestID: pr.ID,
		UpdateMethod:  &updateMethod,
	}

	httpClient, err := opts.HttpClient()
	if err != nil {
		return err
	}
	apiClient := api.NewClientFromHTTP(httpClient)

	updatedHeadRefOid, err := api.UpdatePullRequestBranch(apiClient, repo, params)
	if err != nil {
		return err
	}

	if updatedHeadRefOid == pr.HeadRefOid {
		fmt.Fprintf(opts.IO.ErrOut, "%s PR branch already up-to-date\n", cs.SuccessIcon())
		return nil
	}

	fmt.Fprintf(opts.IO.ErrOut, "%s PR branch updated\n", cs.SuccessIcon())

	if opts.Rebase && opts.GitClient.HasLocalBranch(ctx, pr.HeadRefName) {
		fmt.Fprintf(opts.IO.ErrOut, "%s warning: due to rebase, you need to manually pull the latest changes to the local branch\n", cs.WarningIcon())
		return nil
	}

	if opts.Rebase || opts.SkipLocal || !opts.Interactive && !opts.UpdateLocal {
		return nil
	}

	// Check if the branch the command was run on is the same as the pull request branch.
	// If not, we cannot update the local branch, so we should just return.
	if currentBranch != pr.HeadRefName {
		return nil
	}

	if opts.Interactive && !opts.UpdateLocal {
		if !opts.IO.CanPrompt() {
			return nil
		}

		confirm, err := opts.Prompter.Confirm("Update branch locally?", true)
		if err != nil {
			return err
		}
		if !confirm {
			return nil
		}
	}

	remotes, err := opts.Remotes()
	if err != nil {
		return err
	}

	remote, err := remotes.FindByRepo(repo.RepoOwner(), repo.RepoName())
	if err != nil {
		return err
	}

	if err := opts.GitClient.Pull(context.Background(), remote.Name, pr.HeadRefName); err != nil {
		fmt.Fprintf(opts.IO.ErrOut, "%s cannot update local branch\n", cs.FailureIcon())
		return err
	}

	fmt.Fprintf(opts.IO.ErrOut, "%s local branch updated\n", cs.SuccessIcon())

	return nil
}
