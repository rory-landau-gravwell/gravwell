package traverse_test

import (
	"fmt"
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/internal/testsupport"
	"github.com/gravwell/gravwell/v4/gwcli/mother/traverse"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestDeriveSuggestions(t *testing.T) {
	dummyActionFunc := func(cmd *cobra.Command, fs *pflag.FlagSet) (string, tea.Cmd) { return "", nil } // actually functionality is irrelevant
	/*
		generate a command tree to test against:
		root/
		├── nav_a/ (aliases: "nav_a_alias","AAlias")
		│   └── action_a_1
		├── action1
		└── nav_b/
		    └── nav_ba/
		        ├── action_ba_1
		        └── action_ba_2 (aliases: "aBA2")
	*/
	navA := treeutils.GenerateNav("nav_a", "nav_a short", "nav_a long", []string{"nav_a_alias", "AAlias"},
		nil, // subnavs
		[]action.Pair{scaffold.NewBasicAction("action_a_1", "action_a_1 short", "action_a_1 long", dummyActionFunc, scaffold.BasicOptions{})}, // sub-actions
	)
	action1 := scaffold.NewBasicAction("action1", "action1 short", "action1 long", dummyActionFunc, scaffold.BasicOptions{})
	nav_ba := treeutils.GenerateNav("nav_ba", "nav_ba short", "nav_ba long", nil,
		nil, // subnavs
		[]action.Pair{
			scaffold.NewBasicAction("action_ba_1", "action_ba_1 short", "action_ba_1 long", dummyActionFunc, scaffold.BasicOptions{}),
			scaffold.NewBasicAction("action_ba_2", "action_ba_2 short", "action_ba_2 long", dummyActionFunc, scaffold.BasicOptions{Aliases: []string{"aBA2"}}),
		}, // sub-actions
	)
	navB := treeutils.GenerateNav("nav_b", "nav_b short", "nav_b long", nil,
		[]*cobra.Command{nav_ba}, // subnavs
		nil,                      // sub-actions
	)
	root := treeutils.GenerateNav("root", "root short", "root long", nil,
		[]*cobra.Command{navA, navB},
		[]action.Pair{action1})

	tests := []struct {
		curInput                string
		startingWD              *cobra.Command
		builtins                []string
		expectedWalkSuggestions []traverse.WalkSuggestions
		expectedBISuggestions   []string
	}{
		{"", root, nil, nil, nil},
		{"nav", root, []string{},
			[]traverse.WalkSuggestions{
				{Name: "nav_a", Aliases: []string{"nav_a_alias"}},
				{Name: "nav_b"},
			},
			nil,
		},
		{"a", nav_ba, []string{},
			[]traverse.WalkSuggestions{
				{Name: "action_ba_1"},
				{Name: "action_ba_2", Aliases: []string{"aBA2"}},
			},
			nil,
		},
		{"a", nav_ba, []string{"a", "abcdef"},
			[]traverse.WalkSuggestions{
				{Name: "action_ba_1"},
				{Name: "action_ba_2", Aliases: []string{"aBA2"}},
			},
			[]string{"a", "abcdef"},
		},
		{"z", nav_ba, []string{"a", "abcdef"},
			[]traverse.WalkSuggestions{},
			nil,
		},
		{"nav_a acti", root, []string{"acting", "Acting", "actiNg"},
			[]traverse.WalkSuggestions{
				{Name: "action_a_1"},
			},
			[]string{"acting", "actiNg"},
		},
		{"nav_b nav_ba hel", root, []string{"help", "history", "History", "ls", "here"},
			nil,
			[]string{"help"},
		},
	}
	for _, tt := range tests {
		var startingWDStr string
		if tt.startingWD == nil {
			startingWDStr = "nil"
		} else {
			startingWDStr = tt.startingWD.Name()
		}
		t.Run(fmt.Sprintf("in %v: | startingWD: %s | expects walk suggestions: %v | expects builtin suggestions: %v", tt.curInput, startingWDStr, tt.expectedWalkSuggestions, tt.expectedWalkSuggestions), func(t *testing.T) {
			actualWalk, actualBI := traverse.DeriveSuggestions(tt.curInput, tt.startingWD, tt.builtins)

			// compare walk suggestions
			if !slices.EqualFunc(actualWalk, tt.expectedWalkSuggestions, func(a, b traverse.WalkSuggestions) bool {
				return a.Name == b.Name && slices.Equal(a.Aliases, b.Aliases)
			}) {
				t.Error("incorrect walk suggestions", testsupport.ExpectedActual(tt.expectedWalkSuggestions, actualWalk))
			}
			// compare BI suggestions
			if !slices.Equal(actualBI, tt.expectedBISuggestions) {
				t.Error("incorrect BI suggestions", testsupport.ExpectedActual(tt.expectedBISuggestions, actualBI))
			}
		})
	}
}
