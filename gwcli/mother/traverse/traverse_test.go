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
	var root *cobra.Command
	{
		navA := treeutils.GenerateNav("nav_a", "nav_a short", "nav_a long", []string{"nav_a_alias", "AAlias"},
			nil, // subnavs
			[]action.Pair{scaffold.NewBasicAction("action_a_1", "action_a_1 short", "action_a_1 long", dummyActionFunc, scaffold.BasicOptions{})}, // sub-actions
		)
		action1 := scaffold.NewBasicAction("action1", "action1 short", "action1 long", dummyActionFunc, scaffold.BasicOptions{})
		navB := treeutils.GenerateNav("nav_b", "nav_b short", "nav_b long", nil,
			[]*cobra.Command{ // subnavs
				treeutils.GenerateNav("nav_ba", "nav_ba short", "nav_ba long", nil,
					nil, // subnavs
					[]action.Pair{
						scaffold.NewBasicAction("action_ba_1", "action_ba_1 short", "action_ba_1 long", dummyActionFunc, scaffold.BasicOptions{}),
						scaffold.NewBasicAction("action_ba_2", "action_ba_2 short", "action_ba_2 long", dummyActionFunc, scaffold.BasicOptions{Aliases: []string{"aBA2"}}),
					}, // sub-actions
				)},
			nil, // sub-actions
		)
		root = treeutils.GenerateNav("root", "root short", "root long", nil,
			[]*cobra.Command{navA, navB},
			[]action.Pair{action1})
	}

	tests := []struct {
		curInput                string
		startingWD              *cobra.Command
		builtins                []string
		expectedWalkSuggestions []traverse.WalkSuggestions
		expectedBISuggestions   []string
	}{
		{"nav", root, []string{}, []traverse.WalkSuggestions{
			{Name: "nav_a", Aliases: []string{"nav_a_alias"}},
			{Name: "nav_b"},
		}, nil},
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

			if !slices.EqualFunc(actualWalk, tt.expectedWalkSuggestions, func(a, b traverse.WalkSuggestions) bool {
				if a.Name != b.Name {
					return false
				}
				return slices.Equal(a.Aliases, b.Aliases)
			}) {
				t.Error(testsupport.ExpectedActual(tt.expectedWalkSuggestions, actualWalk))
			}
			if !slices.Equal(actualBI, tt.expectedBISuggestions) {
				t.Error(testsupport.ExpectedActual(tt.expectedWalkSuggestions, actualBI))
			}
		})
	}
}
