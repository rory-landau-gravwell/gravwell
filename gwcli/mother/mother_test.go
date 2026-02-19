package mother_test

import (
	"io"
	"os"
	"path"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/internal/testsupport"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Test_SuggestionCompletion_TeaTest(t *testing.T) {
	t.Run("completion on empty input completes to help", func(t *testing.T) {
		// initialize singletons
		logpath := path.Join(t.TempDir(), "log.txt")
		t.Log("logging to", logpath)
		clilog.InitializeFromArgs([]string{"--log=" + logpath, "--loglevel=debug"})
		t.Cleanup(func() {
			if t.Failed() {
				if b, err := os.ReadFile(logpath); err != nil {
					t.Log(err)
				} else {
					t.Log("Log Output:\n", string(b))
				}

			}
		})
		stylesheet.Cur = stylesheet.Plain()

		// build up some example commands
		nav1Action1 := scaffold.NewBasicAction("action1", "action1 short", "action1 long",
			func(cmd *cobra.Command, fs *pflag.FlagSet) (string, tea.Cmd) { return "", nil }, scaffold.BasicOptions{})
		nav1 := treeutils.GenerateNav("nav1", "nav1 short", "nav1 long", nil, nil, []action.Pair{nav1Action1})
		nav2 := treeutils.GenerateNav("nav2", "nav2 short", "nav2 long", nil, nil, nil)

		root := treeutils.GenerateNav("root", "root short", "root long", nil,
			[]*cobra.Command{nav1, nav2}, nil)

		mthr := mother.New(root, root, nil, nil)
		tm := teatest.NewTestModel(t, mthr, teatest.WithInitialTermSize(100, 80))
		testsupport.TTSendSpecial(tm, tea.KeyTab)
		o := tm.Output()
		b, err := io.ReadAll(o)
		if err != nil {
			t.Fatal(err)
		}
		// should contain help exactly twice; once for the prompt, onces for the suggestion bars
		if count := strings.Count(string(b), "help"); count != 2 {
			t.Errorf("incorrect \"help\" count: %v", testsupport.ExpectedActual(2, count))
		}
		teatest.RequireEqualOutput(t, b)
	})

	// TODO
}
