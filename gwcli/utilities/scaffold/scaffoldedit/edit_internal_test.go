//go:build ci

/*************************************************************************
 * Copyright 2025 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffoldedit

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	. "github.com/gravwell/gravwell/v4/gwcli/internal/testsupport"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldcreate"
	"github.com/spf13/pflag"
)

func TestMain(m *testing.M) {
	logPath := path.Join(os.TempDir(), "gwcli_edit_internal_test", "dev.log")
	if err := os.MkdirAll(path.Dir(logPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create directory for clilog: %v", err)
		os.Exit(1)
	}
	if err := clilog.Init(logPath, "debug"); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize clilog: %v", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// val is a simple struct used as the edit target in Test_Full.
type val struct {
	title string
	value string
}

// Test_Full is an E2E test for the interactive edit flow.
// It runs the dummy action three times to verify that Reset and SetArgs work correctly back-to-back.
//
// Does not use teatest because editModel is not a full tea.Model.
// Input and output are handled manually; View() output is not inspected.
func Test_Full(t *testing.T) {
	allItems := map[int]val{
		0: {title: "zero", value: "z"},
		5: {title: "five", value: "f"},
	}

	var editCalled bool
	pair := NewEditAction("bauble", "baubles",
		Config{
			"value": scaffold.Field{
				Required: false,
				Title:    "Value",
				Flag:     scaffold.FlagConfig{Name: "value", Usage: "sets the value of the item"},
				Order:    10,
				Provider: &scaffoldcreate.TextProvider{},
			},
		},
		SubroutineSet[int, val]{
			SelectSub: func(id int) (val, error) {
				return allItems[id], nil
			},
			FetchSub: func() ([]val, error) {
				result := make([]val, 0, len(allItems))
				for _, v := range allItems {
					result = append(result, v)
				}
				return result, nil
			},
			GetTitleSub: func(item val) string {
				for k, v := range allItems {
					if v == item {
						return fmt.Sprintf("%d", k)
					}
				}
				return "unknown"
			},
			GetDescriptionSub: func(item val) string {
				return fmt.Sprintf("%v -> %v", item.title, item.value)
			},
			PrepopulateSub: func(item val, fields map[string]scaffold.Field) {
				fields["value"].Provider.Set(item.value)
			},
			EditSub: func(item *val, fields map[string]scaffold.Field, _ *pflag.FlagSet) (string, string, error) {
				item.value = fields["value"].Provider.Get()
				editCalled = true
				return item.title, "", nil
			},
		},
	)

	em, ok := pair.Model.(*editModel[int, val])
	if !ok {
		t.Fatal("failed to type assert result to edit model")
	}

	fauxMother(t, em, &editCalled, -1)
	fauxMother(t, em, &editCalled, 5)
	fauxMother(t, em, &editCalled, -1)
}

// fauxMother simulates a Mother-like run of the edit action.
// If id is -1, interactive item selection is used; otherwise the item is selected by --id.
func fauxMother(t *testing.T, em *editModel[int, val], editCalled *bool, id int) {
	t.Helper()
	*editCalled = false

	var args []string
	if id != -1 {
		args = append(args, fmt.Sprintf("--id=%d", id))
	}

	CheckSetArgs(t, em.SetArgs, nil, args, 80, 50, false, nil, false)
	em.Update(tea.WindowSizeMsg{Width: 80, Height: 50})
	time.Sleep(50 * time.Millisecond)

	if id == -1 {
		// select the first item from the list
		em.Update(SendHotkey(hotkeys.Invoke))
		time.Sleep(50 * time.Millisecond)
	}

	// verify we entered edit mode
	if em.mode != editing {
		t.Fatal("incorrect mode", ExpectedActual(editing, em.mode))
	}

	// sanity-check initial edit state
	if em.editing.selected != 0 {
		t.Error("first field should be focused on entry. Got", em.editing.selected)
	}
	if em.editing.err != "" {
		t.Error("unexpected error on entry:", em.editing.err)
	}
	if em.editing.longestTitleLength < 1 {
		t.Errorf("longestTitleLength too small (%v)", em.editing.longestTitleLength)
	}

	// navigate up — should wrap to the submit button
	em.Update(SendHotkey(hotkeys.CursorUp))
	time.Sleep(50 * time.Millisecond)

	if !em.editing.submitSelected() {
		t.Fatal("keyUp on first field did not cycle to submit.",
			ExpectedActual(uint(len(em.editing.ordered)), em.editing.selected))
	}

	// navigate back to top
	em.Update(SendHotkey(hotkeys.CursorDown))
	time.Sleep(50 * time.Millisecond)

	// navigate down through every field to reach the submit button
	for i := 0; i < len(em.cfg); i++ {
		em.Update(SendHotkey(hotkeys.CursorDown))
		time.Sleep(50 * time.Millisecond)
	}

	if !em.editing.submitSelected() {
		t.Fatal("traversing all fields did not reach submit.",
			ExpectedActual(uint(len(em.editing.ordered)), em.editing.selected))
	}

	// invoke the submit button
	em.Update(SendHotkey(hotkeys.Invoke))
	time.Sleep(50 * time.Millisecond)

	if !*editCalled {
		t.Fatal("the EditSub was not called")
	}
	if !em.Done() {
		t.Fatal("triggering EditSub did not mark the action as Done")
	}

	if err := em.Reset(); err != nil {
		t.Fatal(err)
	}
}

// TestNonInteractive validates the non-interactive (flag-driven) edit path.
func TestNonInteractive(t *testing.T) {
	pair, items, _, sbErr := generateTestPair()

	pair.Action.SetArgs([]string{"--" + ft.NoInteractive.Name(), "--id=eb8b5cb2-7cb6-4586-a2d6-665e662ad976", "--note=baby girl"})
	if err := pair.Action.Execute(); err != nil {
		t.Fatal(err)
	}
	outErr := strings.TrimSpace(sbErr.String())
	if outErr != "" {
		t.Fatal("unexpected stderr:", outErr)
	}
	// verify the note was set on Bee
	bee := items[uuid.MustParse("eb8b5cb2-7cb6-4586-a2d6-665e662ad976")]
	if bee.note == "" {
		t.Fatal("expected note to be set on Bee but it was empty")
	}
	// verify Mozzie was untouched
	mozzie := items[uuid.MustParse("65d7e5a3-9be4-43e0-9fce-887052753661")]
	if mozzie.color != "" {
		t.Fatal("did not expect color to be set on Mozzie")
	}
}

// cat is a simple struct used as the edit target in generateTestPair / TestNonInteractive.
type cat struct {
	name      string
	color     string
	furLength string
	note      string
}

// generateTestPair builds an edit action pair for testing.
// Returns the pair, the data map, and redirected stdout/stderr writers.
func generateTestPair() (pair action.Pair, data map[uuid.UUID]*cat, sbOut, sbErr strings.Builder) {
	items := map[uuid.UUID]*cat{
		uuid.MustParse("eb8b5cb2-7cb6-4586-a2d6-665e662ad976"): {name: "Bee", color: "tortie"},
		uuid.MustParse("eb8b5cb2-7cb6-4586-a2d6-8f3308fafb52"): {name: "Coco", note: "adventure buddy"},
		uuid.MustParse("65d7e5a3-9be4-43e0-9fce-887052753661"): {name: "Mozzie", note: "little grey girl"},
	}

	pair = NewEditAction("cat", "cats",
		Config{
			"fur color": scaffold.Field{
				Required: true,
				Title:    "Fur Color",
				Flag:     scaffold.FlagConfig{Name: "fur-color", Usage: "set the fur color of your feline"},
				Order:    80,
				Provider: &scaffoldcreate.TextProvider{},
			},
			"fur length": scaffold.Field{
				Required: false,
				Title:    "Fur Length",
				Flag:     scaffold.FlagConfig{Name: "fur-length", Usage: "set the fur length of your feline (hairless, short, medium, long)"},
				Order:    100,
				Provider: &scaffoldcreate.TextProvider{},
			},
			"note": scaffold.Field{
				Required: false,
				Title:    "note",
				Flag:     scaffold.FlagConfig{Name: "note", Usage: "add a note to the kitty description"},
				Order:    20,
				Provider: &scaffoldcreate.TextProvider{},
			},
		},
		SubroutineSet[uuid.UUID, *cat]{
			SelectSub: func(id uuid.UUID) (*cat, error) {
				itm, found := items[id]
				if !found {
					return &cat{}, ErrUnknownID(id)
				}
				return itm, nil
			},
			FetchSub: func() ([]*cat, error) {
				result := make([]*cat, 0, len(items))
				for _, v := range items {
					result = append(result, v)
				}
				return result, nil
			},
			GetTitleSub: func(i *cat) string {
				return i.name
			},
			GetDescriptionSub: func(i *cat) string {
				return "some description"
			},
			PrepopulateSub: func(item *cat, fields map[string]scaffold.Field) {
				fields["fur color"].Provider.Set(item.color)
				fields["fur length"].Provider.Set(item.furLength)
				fields["note"].Provider.Set(item.note)
			},
			EditSub: func(item **cat, fields map[string]scaffold.Field, _ *pflag.FlagSet) (string, string, error) {
				(*item).color = fields["fur color"].Provider.Get()
				(*item).furLength = fields["fur length"].Provider.Get()
				(*item).note = fields["note"].Provider.Get()
				return (*item).name, "", nil
			},
		},
	)

	// bolt on the --no-interactive flag (normally added by Mother)
	pair.Action.Flags().Bool(ft.NoInteractive.Name(), false, "run without interactive UI")
	// capture output
	pair.Action.SetOut(&sbOut)
	pair.Action.SetErr(&sbErr)
	return pair, items, sbOut, sbErr
}

