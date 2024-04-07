package update

import (
	"bytes"
	"testing"

	"github.com/cli/cli/v2/pkg/cmdutil"
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
