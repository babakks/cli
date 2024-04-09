package update

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/context"
	"github.com/cli/cli/v2/git"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/internal/run"
	shared "github.com/cli/cli/v2/pkg/cmd/pr/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/httpmock"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
)

func TestNewCmdUpdate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		noTTY    bool
		output   UpdateOptions
		wantsErr string
	}{
		{
			name:  "no argument",
			input: "",
			output: UpdateOptions{
				Interactive: true,
			},
		},
		{
			name:  "no tty, no argument",
			input: "",
			noTTY: true,
			output: UpdateOptions{
				SelectorArg: "",
			},
		},
		{
			name:  "with argument",
			input: "23",
			output: UpdateOptions{
				SelectorArg: "23",
				Interactive: true,
			},
		},
		{
			name:  "no argument, --update-local",
			input: "--update-local",
			output: UpdateOptions{
				Interactive: true,
				UpdateLocal: true,
			},
		},
		{
			name:  "with argument, --update-local",
			input: "23 --update-local",
			output: UpdateOptions{
				SelectorArg: "23",
				Interactive: true,
				UpdateLocal: true,
			},
		},
		{
			name:  "no argument, --skip-local",
			input: "--skip-local",
			output: UpdateOptions{
				Interactive: true,
				SkipLocal:   true,
			},
		},
		{
			name:  "with argument, --skip-local",
			input: "23 --skip-local",
			output: UpdateOptions{
				SelectorArg: "23",
				Interactive: true,
				SkipLocal:   true,
			},
		},
		{
			name:  "no argument, --rebase",
			input: "--rebase",
			output: UpdateOptions{
				Interactive: true,
				Rebase:      true,
			},
		},
		{
			name:  "with argument, --rebase",
			input: "23 --rebase",
			output: UpdateOptions{
				SelectorArg: "23",
				Interactive: true,
				Rebase:      true,
			},
		},
		{
			name:     "mutually exclusive options: --rebase and --update-local",
			input:    "--rebase --update-local",
			wantsErr: "specify only one of `--rebase` or `--update-local`",
		},
		{
			name:     "mutually exclusive options: --skip-local and --update-local",
			input:    "--skip-local --update-local",
			wantsErr: "specify only one of `--skip-local` or `--update-local`",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			ios.SetStdoutTTY(!tt.noTTY)
			ios.SetStdinTTY(!tt.noTTY)
			ios.SetStderrTTY(!tt.noTTY)

			f := &cmdutil.Factory{
				IOStreams: ios,
			}

			argv, err := shlex.Split(tt.input)
			assert.NoError(t, err)

			var gotOpts *UpdateOptions
			cmd := NewCmdUpdate(f, func(opts *UpdateOptions) error {
				gotOpts = opts
				return nil
			})
			cmd.Flags().BoolP("help", "x", false, "")

			cmd.SetArgs(argv)
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err = cmd.ExecuteC()
			if tt.wantsErr != "" {
				assert.EqualError(t, err, tt.wantsErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.output.SelectorArg, gotOpts.SelectorArg)
			assert.Equal(t, tt.output.Interactive, gotOpts.Interactive)
			assert.Equal(t, tt.output.SkipLocal, gotOpts.SkipLocal)
			assert.Equal(t, tt.output.UpdateLocal, gotOpts.UpdateLocal)
			assert.Equal(t, tt.output.Rebase, gotOpts.Rebase)
		})
	}
}

func Test_updateRun(t *testing.T) {
	defaultInput := func() UpdateOptions {
		return UpdateOptions{
			Remotes: func() (context.Remotes, error) {
				return context.Remotes{
					{
						Remote: &git.Remote{
							Name:     "origin",
							Resolved: "base",
						},
						Repo: ghrepo.New("OWNER", "REPO"),
					},
				}, nil
			},
			Branch: func() (string, error) {
				return "pr-branch", nil
			},
			Finder: shared.NewMockFinder("123", &api.PullRequest{
				ID:          "123",
				HeadRefName: "pr-branch",
				HeadRefOid:  "head-ref-oid",
			}, ghrepo.New("OWNER", "REPO")),
		}
	}

	tests := []struct {
		name      string
		input     *UpdateOptions
		httpStubs func(*testing.T, *httpmock.Registry)
		cmdStubs  func(cs *run.CommandStubber)
		stdout    string
		stderr    string
		wantsErr  string
	}{
		{
			name: "success, tty, no update",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch already up-to-date\n",
		},
		{
			name: "success, tty, no update, on different local branch",
			input: &UpdateOptions{
				Branch: func() (string, error) {
					return "some-other-branch", nil
				},
				SelectorArg: "123",
				Interactive: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch already up-to-date\n",
		},
		{
			name: "success, tty, update (merge), on different local branch",
			input: &UpdateOptions{
				Branch: func() (string, error) {
					return "some-other-branch", nil
				},
				SelectorArg: "123",
				Interactive: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch updated\n",
		},
		{
			name: "success, tty, update (merge), prompt (no)",
			input: &UpdateOptions{
				SelectorArg: "123",
				Prompter: &prompter.PrompterMock{
					ConfirmFunc: func(p string, _ bool) (bool, error) {
						if p == "Update branch locally?" {
							return false, nil
						}
						return false, prompter.NoSuchPromptErr(p)
					},
				},
				Interactive: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch updated\n",
		},
		{
			name: "success, tty, update (merge), prompt (yes)",
			input: &UpdateOptions{
				SelectorArg: "123",
				Prompter: &prompter.PrompterMock{
					ConfirmFunc: func(p string, _ bool) (bool, error) {
						if p == "Update branch locally?" {
							return true, nil
						}
						return false, prompter.NoSuchPromptErr(p)
					},
				},
				Interactive: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			cmdStubs: func(cs *run.CommandStubber) {
				cs.Register(`git pull .+ origin pr-branch`, 0, "")
			},
			stdout: "",
			stderr: "✓ PR branch updated\n✓ local branch updated\n",
		},
		{
			name: "success, tty, --update-local, update (merge)",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
				UpdateLocal: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			cmdStubs: func(cs *run.CommandStubber) {
				cs.Register(`git rev-parse --verify refs/heads/pr-branch`, 0, "0000000000000000000000000000000000000000")
				cs.Register(`git pull .+ origin pr-branch`, 0, "")
			},
			stdout: "",
			stderr: "✓ PR branch updated\n✓ local branch updated\n",
		},
		{
			name: "failure, tty, --update-local, on a different branch",
			input: &UpdateOptions{
				Branch: func() (string, error) {
					return "some-other-branch", nil
				},
				SelectorArg: "123",
				Interactive: true,
				UpdateLocal: true,
			},
			wantsErr: "current branch does not track the PR branch; consider switching to the correct branch or running the command without the `--update-local` option",
		},
		{
			name: "failure, tty, --update-local, no remote pointing at the repo",
			input: &UpdateOptions{
				Remotes: func() (context.Remotes, error) {
					return context.Remotes{
						{
							Remote: &git.Remote{
								Name:     "origin",
								Resolved: "base",
							},
							Repo: ghrepo.New("SOME-OTHER-OWNER", "SOME-OTHER-REPO"),
						},
					}, nil
				},
				SelectorArg: "123",
				Interactive: true,
				UpdateLocal: true,
			},
			wantsErr: "current branch does not track the PR branch; consider setting the branch to track the PR branch or running the command without the `--update-local` option",
		},
		{
			name: "failure, tty, --update-local, update (merge), update local branch fails",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
				UpdateLocal: true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			cmdStubs: func(cs *run.CommandStubber) {
				cs.Register(`git rev-parse --verify refs/heads/pr-branch`, 0, "0000000000000000000000000000000000000000")
				cs.Register(`git pull .+ origin pr-branch`, 1, "some error")
			},
			wantsErr: "cannot update local branch; running git pull failed",
		},
		{
			name: "success, tty, --skip-local, update (merge)",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
				SkipLocal:   true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "MERGE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch updated\n",
		},
		{
			name: "success, tty, --rebase, no update",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
				Rebase:      true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "REBASE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch already up-to-date\n",
		},
		{
			name: "success, tty, --rebase, update (rebase), with a local branch tracking the remote",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
				Rebase:      true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "REBASE", inputs["updateMethod"])
					}))
			},
			cmdStubs: func(cs *run.CommandStubber) {
				cs.Register(`git rev-parse --verify refs/heads/pr-branch`, 0, "0000000000000000000000000000000000000000")
			},
			stdout: "",
			stderr: "✓ PR branch updated\n! warning: due to rebase, you need to manually pull the latest changes to the local branch\n",
		},
		{
			name: "success, tty, --rebase, update (rebase), no local branch tracking the remote",
			input: &UpdateOptions{
				SelectorArg: "123",
				Interactive: true,
				Rebase:      true,
			},
			httpStubs: func(t *testing.T, reg *httpmock.Registry) {
				reg.Register(
					httpmock.GraphQL(`mutation PullRequestUpdateBranch\b`),
					httpmock.GraphQLMutation(`{
						"data": {
							"updatePullRequestBranch": {
								"pullRequest": {
									"id": "123",
									"headRefOid": "new-head-ref-oid"
								}
							}
						}
					}`, func(inputs map[string]interface{}) {
						assert.Equal(t, "123", inputs["pullRequestId"])
						assert.Equal(t, "head-ref-oid", inputs["expectedHeadOid"])
						assert.Equal(t, "REBASE", inputs["updateMethod"])
					}))
			},
			stdout: "",
			stderr: "✓ PR branch updated\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, stdout, stderr := iostreams.Test()
			ios.SetStdoutTTY(true)
			ios.SetStdinTTY(true)
			ios.SetStderrTTY(true)

			reg := &httpmock.Registry{}
			defer reg.Verify(t)
			if tt.httpStubs != nil {
				tt.httpStubs(t, reg)
			}

			if tt.cmdStubs != nil {
				cs, cmdTeardown := run.Stub()
				defer cmdTeardown(t)
				tt.cmdStubs(cs)
			}

			tt.input.GitClient = &git.Client{
				GhPath:  "some/path/gh",
				GitPath: "some/path/git",
			}

			if tt.input.Remotes == nil {
				tt.input.Remotes = defaultInput().Remotes
			}

			if tt.input.Branch == nil {
				tt.input.Branch = defaultInput().Branch
			}

			if tt.input.Finder == nil {
				tt.input.Finder = defaultInput().Finder
			}

			httpClient := func() (*http.Client, error) { return &http.Client{Transport: reg}, nil }

			tt.input.IO = ios
			tt.input.HttpClient = httpClient

			err := updateRun(tt.input)

			if tt.wantsErr != "" {
				assert.EqualError(t, err, tt.wantsErr)
				return
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.stdout, stdout.String())
			assert.Equal(t, tt.stderr, stderr.String())
		})
	}
}
