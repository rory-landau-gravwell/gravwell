/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package mother

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/internal/testsupport"
	"github.com/spf13/cobra"
)

func Test_quoteSplitTokens(t *testing.T) {
	tests := []struct {
		name               string
		oldTokens          []string
		wantStrippedTokens []string
	}{
		{"no alterations",
			[]string{
				"--flag1", "value1",
				"--flag2", "value2",
				"argValue",
				"-a", "value3",
				"argValue2",
			},
			[]string{
				"--flag1", "value1",
				"--flag2", "value2",
				"argValue",
				"-a", "value3",
				"argValue2",
			},
		},
		{"mixed style",
			[]string{
				"--flag1=value1",
				"--flag2", "value2",
				"argValue",
				"-a", "value3",
				"argValue2",
				"-b=value4",
				"argValue3",
			},
			[]string{
				"--flag1", "value1",
				"--flag2", "value2",
				"argValue",
				"-a", "value3",
				"argValue2",
				"-b", "value4",
				"argValue3",
			},
		},
		{"mixed style with boolean flags",
			[]string{
				"--boolFlag1",
				"--flag1=value1",
				"--flag2", "value2",
				"argValue",
				"-a", "value3",
				"argValue2",
				"-n",
				"-b=value4",
				"argValue3",
			},
			[]string{
				"--boolFlag1",
				"--flag1", "value1",
				"--flag2", "value2",
				"argValue",
				"-a", "value3",
				"argValue2",
				"-n",
				"-b", "value4",
				"argValue3",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotStrippedTokens := quoteSplitTokens(tt.oldTokens); !reflect.DeepEqual(gotStrippedTokens, tt.wantStrippedTokens) {
				t.Errorf("quoteSplitTokens() = %v, want %v", gotStrippedTokens, tt.wantStrippedTokens)
			}
		})
	}

	t.Run("no tokens", func(t *testing.T) {
		got := quoteSplitTokens([]string{})
		if len(got) != 0 {
			t.Errorf("quoteSplitTokens() = %v (len: %v), want [] (len: 0)", got, len(got))

		}
	})
}

func Test_generateSuggestionFromCurrentInput(t *testing.T) {
	// initialize required data with constant data
	builtins = map[string]func(m *Mother, endCmd *cobra.Command, excessTokens []string) tea.Cmd{
		"help":    nil,
		"ls":      nil,
		"history": nil,
		"pwd":     nil,
		"quit":    nil,
		"exit":    nil,
		"clear":   nil,
		"tree":    nil,
	}
	builtinKeys = slices.Collect(maps.Keys(builtins))

	{
		biTests := []struct {
			curInput     string
			expectedSgts []string
		}{
			{"h", []string{"help", "history"}},
			{"help", []string{"help"}},
			{"dne", []string{}},
			{"", []string{}},
			{" ", []string{}},
		}
		for _, tt := range biTests {
			t.Run(fmt.Sprintf("in: %v | expects: %v", tt.curInput, tt.expectedSgts), func(t *testing.T) {
				_, biSgt := generateSuggestionFromCurrentInput(tt.curInput, nil)
				if slices.Compare(biSgt, tt.expectedSgts) != 0 {
					t.Fatal(testsupport.ExpectedActual(tt.expectedSgts, biSgt))
				}
			})
		}
	}
}
