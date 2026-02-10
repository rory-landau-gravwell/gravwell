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
	nav_a := treeutils.GenerateNav("nav_a", "nav_a short", "nav_a long", []string{"nav_a_alias", "AAlias"},
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
	nav_b := treeutils.GenerateNav("nav_b", "nav_b short", "nav_b long", nil,
		[]*cobra.Command{nav_ba}, // subnavs
		nil,                      // sub-actions
	)
	root := treeutils.GenerateNav("root", "root short", "root long", nil,
		[]*cobra.Command{nav_a, nav_b},
		[]action.Pair{action1})

	tests := []struct {
		name                  string // NOTE: curInput is prefixed to test name on run
		curInput              string
		startingWD            *cobra.Command
		builtins              []string
		expectedNavs          []traverse.CmdSuggestion
		expectedActions       []traverse.CmdSuggestion
		expectedBISuggestions []string
	}{
		{"nil working directory",
			"nav", nil, []string{"a", "b", "c"},
			nil,
			nil,
			nil,
		},
		{"empty input should suggest all immediate navs, actions and all builtins.",
			"", root, []string{"bi1", "bi2", "help"},
			[]traverse.CmdSuggestion{
				{CmdName: "nav_a"},
				{CmdName: "nav_b"},
			},
			[]traverse.CmdSuggestion{
				{CmdName: "action1"},
			},
			[]string{"bi1", "bi2", "help"},
		},
		{"whitespace-only input should suggest all immediate navs, actions and all builtins.",
			"       	  ", nav_ba, []string{"bi1", "bi2", "help"},
			nil,
			[]traverse.CmdSuggestion{
				{CmdName: "action_ba_1"},
				{CmdName: "action_ba_2"},
			},
			[]string{"bi1", "bi2", "help"},
		},
		{"against root should match both subnavs and a BI, but not the action",
			"nav", root, []string{"bi1", "bi2", "help", "n", "N", "navigator", "Navigator"},
			[]traverse.CmdSuggestion{
				{CmdName: "nav_a", MatchedNameCharacters: "nav"},
				{CmdName: "nav_b", MatchedNameCharacters: "nav"},
			},
			nil,
			[]string{"navigator"},
		},
		{"against nav_b should match only nav_ba and a BI",
			"nav", nav_b, []string{"bi1", "bi2", "help", "n", "N", "navigator", "Navigator"},
			[]traverse.CmdSuggestion{
				{CmdName: "nav_ba", MatchedNameCharacters: "nav"},
			},
			nil,
			[]string{"navigator"},
		},
		{"against root should traverse to nav_b and match only nav_ba and a BI",
			"nav_b nav", root, []string{"bi1", "bi2", "help", "n", "N", "navigator", "Navigator"},
			[]traverse.CmdSuggestion{
				{CmdName: "nav_ba", MatchedNameCharacters: "nav"},
			},
			nil,
			[]string{"navigator"},
		},
		{"(trailing space) should traverse and then suggest all at new pwd",
			"nav_a ", root, []string{"a", "b"},
			nil,
			[]traverse.CmdSuggestion{
				{CmdName: "action_a_1"},
			},
			[]string{"a", "b"},
		},
		{"alias match, but no trailing space so no traversal and thus no suggestions",
			"AAlias", root, []string{"a", "b"},
			nil,
			nil,
			nil,
		},
		{"(trailing space) traverse nav_a via alias",
			"AAlias ", root, []string{"a", "b"},
			nil,
			[]traverse.CmdSuggestion{
				{CmdName: "action_a_1"},
			},
			[]string{"a", "b"},
		},
		{"no matching suggests, no traversal",
			"z", root, []string{"a", "b"},
			nil,
			nil,
			nil,
		},
		{"traversal, then no matching suggests",
			"nav_b z", root, []string{"a", "b"},
			nil,
			nil,
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("in:\"%s\"|%v", tt.curInput, tt.name), func(t *testing.T) {
			actualNavs, actualActions, actualBI := traverse.DeriveSuggestions(tt.curInput, tt.startingWD, tt.builtins)

			// compare nav suggestions
			if !slices.EqualFunc(actualNavs, tt.expectedNavs, func(a, b traverse.CmdSuggestion) bool {
				return a.Equals(b)
			}) {
				t.Error("incorrect nav suggestions", testsupport.ExpectedActual(tt.expectedNavs, actualNavs))
			}
			// compare action suggestions
			if !slices.EqualFunc(actualActions, tt.expectedActions, func(a, b traverse.CmdSuggestion) bool {
				return a.Equals(b)
			}) {
				t.Error("incorrect action suggestions", testsupport.ExpectedActual(tt.expectedActions, actualActions))
			}
			// compare BI suggestions
			if !slices.Equal(actualBI, tt.expectedBISuggestions) {
				t.Error("incorrect BI suggestions", testsupport.ExpectedActual(tt.expectedBISuggestions, actualBI))
			}
		})
	}
}
