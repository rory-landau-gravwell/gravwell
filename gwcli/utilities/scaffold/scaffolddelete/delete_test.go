//go:build ci

/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffolddelete_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/group"
	"github.com/gravwell/gravwell/v4/gwcli/internal/testsupport"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/uniques"
	"github.com/spf13/cobra"
)

func TestMain(m *testing.M) {
	clilog.InitializeFromArgs(nil)
	m.Run()
}

// #region test helpers

// testItems returns a set of items suitable for test usage.
func testItems() []listitem { // TODO generic
	return []scaffolddelete.Item[uint64]{
		scaffolddelete.NewItem("alpha", "first item", uint64(1)),
		scaffolddelete.NewItem("beta", "second item", uint64(2)),
		scaffolddelete.NewItem("gamma", "third item", uint64(3)),
	}
}

// noopDelete never fails.
func noopDelete(_ bool, _ uint64) error { return nil }

// trackingDelete records which IDs were deleted and with what dryrun state.
type trackingDelete struct {
	deleted []uint64
	dryrun  []bool
}

func (td *trackingDelete) delete(dryrun bool, id uint64) error {
	td.deleted = append(td.deleted, id)
	td.dryrun = append(td.dryrun, dryrun)
	return nil
}

func (td *trackingDelete) reset() {
	td.deleted = nil
	td.dryrun = nil
}

// failingDelete always returns an error.
func failingDelete(_ bool, _ uint64) error {
	return errors.New("deletion failed")
}

// newTestCommand creates a rooted delete action cobra command with persistent flags attached.
func newTestCommand(del func(bool, uint64) error, fch func() ([]scaffolddelete.Item[uint64], error)) *cobra.Command {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", del, fch, scaffolddelete.Options{})
	// Wrap in a root to get persistent flags (like --no-interactive) and groups
	root := &cobra.Command{Use: "root"}
	uniques.AttachPersistentFlags(root)
	group.AddActionGroup(root)
	root.AddCommand(pair.Action)
	return root
}

// #endregion

// #region Item tests

func TestItem(t *testing.T) {
	itm := scaffolddelete.NewItem("myTitle", "myDesc", uint64(42))

	t.Run("Title", func(t *testing.T) {
		if itm.Title() != "myTitle" {
			t.Fatal(testsupport.ExpectedActual("myTitle", itm.Title()))
		}
	})
	t.Run("Description", func(t *testing.T) {
		if itm.Description() != "myDesc" {
			t.Fatal(testsupport.ExpectedActual("myDesc", itm.Description()))
		}
	})
	t.Run("ID", func(t *testing.T) {
		if itm.ID() != 42 {
			t.Fatal(testsupport.ExpectedActual(42, itm.ID()))
		}
	})
	t.Run("FilterValue", func(t *testing.T) {
		expected := "myTitle" + "myDesc"
		if itm.FilterValue() != expected {
			t.Fatal(testsupport.ExpectedActual(expected, itm.FilterValue()))
		}
	})
	t.Run("Selected defaults false", func(t *testing.T) {
		if itm.Selected() {
			t.Fatal("new items should not be selected by default")
		}
	})
	t.Run("SetSelected", func(t *testing.T) {
		itm.SetSelected(true)
		if !itm.Selected() {
			t.Fatal("expected Selected() to be true after SetSelected(true)")
		}
		itm.SetSelected(false)
		if itm.Selected() {
			t.Fatal("expected Selected() to be false after SetSelected(false)")
		}
	})
}

// #endregion

// #region Non-interactive (Cobra RunE) tests

func TestNonInteractive_SingleID(t *testing.T) {
	td := &trackingDelete{}
	root := newTestCommand(td.delete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive", "--id=2"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if len(td.deleted) != 1 || td.deleted[0] != 2 {
		t.Fatal(testsupport.ExpectedActual([]uint64{2}, td.deleted))
	}
	if td.dryrun[0] {
		t.Fatal("expected dryrun to be false")
	}
	if !strings.Contains(out.String(), "widget (ID 2) deleted") {
		t.Fatal("expected success message in output, got:", out.String())
	}
}

func TestNonInteractive_MultipleIDs(t *testing.T) {
	td := &trackingDelete{}
	root := newTestCommand(td.delete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive", "--id=1", "--id=3"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if len(td.deleted) != 2 {
		t.Fatal(testsupport.ExpectedActual(2, len(td.deleted)))
	}
	if td.deleted[0] != 1 || td.deleted[1] != 3 {
		t.Fatal(testsupport.ExpectedActual([]uint64{1, 3}, td.deleted))
	}
}

func TestNonInteractive_CommaSeparatedIDs(t *testing.T) {
	td := &trackingDelete{}
	root := newTestCommand(td.delete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive", "--id=1,2,3"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if len(td.deleted) != 3 {
		t.Fatal(testsupport.ExpectedActual(3, len(td.deleted)))
	}
}

func TestNonInteractive_Dryrun(t *testing.T) {
	td := &trackingDelete{}
	root := newTestCommand(td.delete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive", "--id=1", "--dryrun"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	if len(td.deleted) != 1 {
		t.Fatal(testsupport.ExpectedActual(1, len(td.deleted)))
	}
	if !td.dryrun[0] {
		t.Fatal("expected dryrun to be true")
	}
	if !strings.Contains(out.String(), "DRYRUN") {
		t.Fatal("expected DRYRUN in output, got:", out.String())
	}
}

func TestNonInteractive_NoIDRequiresError(t *testing.T) {
	root := newTestCommand(noopDelete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	root.SetOut(&strings.Builder{})
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --id is not provided in no-interactive mode")
	}
	if !strings.Contains(err.Error(), "--id is required") {
		t.Fatal("expected --id required error, got:", err.Error())
	}
}

func TestNonInteractive_DeleteError(t *testing.T) {
	root := newTestCommand(failingDelete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	var out strings.Builder
	root.SetOut(&out)
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive", "--id=1", "--id=2"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when deletion fails")
	}
	// errors.Join produces joined errors for both
	if !strings.Contains(err.Error(), "failed to delete") {
		t.Fatal("expected 'failed to delete' in error, got:", err.Error())
	}
}

func TestNonInteractive_InvalidID(t *testing.T) {
	root := newTestCommand(noopDelete, func() ([]scaffolddelete.Item[uint64], error) {
		return testItems(), nil
	})

	root.SetOut(&strings.Builder{})
	root.SetErr(&strings.Builder{})
	root.SetArgs([]string{"delete", "--no-interactive", "--id=notanumber"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for non-numeric ID")
	}
}

// #endregion

// #region Interactive model (SetArgs) tests

func TestSetArgs_NoItems(t *testing.T) {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return nil, nil // no items
		}, scaffolddelete.Options{})

	inv, cmd, err := pair.Model.SetArgs(nil, []string{}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if inv != "" {
		t.Fatal("unexpected invalid:", inv)
	}
	// should be done immediately with a message about no items
	if !pair.Model.Done() {
		t.Fatal("expected Done() after SetArgs with no items")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd (print message) when no items available")
	}
}

func TestSetArgs_FetchError(t *testing.T) {
	fetchErr := errors.New("network error")
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return nil, fetchErr
		}, scaffolddelete.Options{})

	_, _, err := pair.Model.SetArgs(nil, []string{}, 80, 50)
	if err == nil {
		t.Fatal("expected error from SetArgs when fetch fails")
	}
	if !errors.Is(err, fetchErr) {
		t.Fatal(testsupport.ExpectedActual(fetchErr, err))
	}
}

func TestSetArgs_WithIDFlags(t *testing.T) {
	td := &trackingDelete{}
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", td.delete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	inv, cmd, err := pair.Model.SetArgs(nil, []string{"--id=1", "--id=2"}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if inv != "" {
		t.Fatal("unexpected invalid:", inv)
	}
	// Should have immediately deleted the items
	if !pair.Model.Done() {
		t.Fatal("expected Done() after SetArgs with --id flags")
	}
	if len(td.deleted) != 2 {
		t.Fatal(testsupport.ExpectedActual(2, len(td.deleted)))
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd with result messages")
	}
}

func TestSetArgs_WithDryrunFlag(t *testing.T) {
	td := &trackingDelete{}
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", td.delete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	inv, cmd, err := pair.Model.SetArgs(nil, []string{"--id=3", "--dryrun"}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if inv != "" {
		t.Fatal("unexpected invalid:", inv)
	}
	if !pair.Model.Done() {
		t.Fatal("expected Done()")
	}
	if len(td.deleted) != 1 || !td.dryrun[0] {
		t.Fatal("expected dryrun deletion of id 3")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd with dryrun result messages")
	}
}

func TestSetArgs_NoFlags_Interactive(t *testing.T) {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	inv, cmd, err := pair.Model.SetArgs(nil, []string{}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if inv != "" {
		t.Fatal("unexpected invalid:", inv)
	}
	// Should NOT be done -- waiting for interactive input
	if pair.Model.Done() {
		t.Fatal("should not be Done() without flags or interaction")
	}
	if cmd != nil {
		t.Fatal("expected nil onStart cmd when entering interactive mode")
	}
}

func TestSetArgs_BadFlags(t *testing.T) {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	inv, _, err := pair.Model.SetArgs(nil, []string{"--nonexistent"}, 80, 50)
	if err != nil {
		t.Fatal("expected invalid, not error, for bad flags")
	}
	if inv == "" {
		t.Fatal("expected invalid string for unknown flag")
	}
}

// #endregion

// #region Model lifecycle tests

func TestModel_Reset(t *testing.T) {
	td := &trackingDelete{}
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", td.delete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	// First run: SetArgs with --id to complete
	_, _, err := pair.Model.SetArgs(nil, []string{"--id=1"}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !pair.Model.Done() {
		t.Fatal("expected Done()")
	}

	// Reset should bring it back to a non-done state
	if err := pair.Model.Reset(); err != nil {
		t.Fatal(err)
	}
	if pair.Model.Done() {
		t.Fatal("should not be Done() after Reset()")
	}
}

func TestModel_RepeatedUse(t *testing.T) {
	td := &trackingDelete{}
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", td.delete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	// First invocation
	_, _, err := pair.Model.SetArgs(nil, []string{"--id=1"}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !pair.Model.Done() {
		t.Fatal("expected Done()")
	}
	if err := pair.Model.Reset(); err != nil {
		t.Fatal(err)
	}

	// Second invocation with different args
	td.reset()
	_, _, err = pair.Model.SetArgs(nil, []string{"--id=2", "--id=3"}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !pair.Model.Done() {
		t.Fatal("expected Done()")
	}
	if len(td.deleted) != 2 {
		t.Fatal(testsupport.ExpectedActual(2, len(td.deleted)))
	}
	if td.deleted[0] != 2 || td.deleted[1] != 3 {
		t.Fatal(testsupport.ExpectedActual([]uint64{2, 3}, td.deleted))
	}
}

func TestModel_View_WhenDone(t *testing.T) {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	_, _, err := pair.Model.SetArgs(nil, []string{"--id=1"}, 80, 50)
	if err != nil {
		t.Fatal(err)
	}
	view := pair.Model.View()
	if view == "" {
		t.Fatal("expected non-empty view when done")
	}
}

// #endregion

// #region Options tests

func TestOptions_Apply(t *testing.T) {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	// Verify the command has expected defaults
	if pair.Action.Use != "delete" {
		t.Fatal(testsupport.ExpectedActual("delete", pair.Action.Use))
	}
	if !strings.Contains(pair.Action.Short, "widgets") {
		t.Fatal("expected 'widgets' in Short description")
	}
}

func TestOptions_ApplyWithOverrides(t *testing.T) {
	pair := scaffolddelete.NewDeleteAction("widget", "widgets", noopDelete,
		func() ([]scaffolddelete.Item[uint64], error) {
			return testItems(), nil
		}, scaffolddelete.Options{})

	// Verify flags are registered
	if pair.Action.Flags().Lookup("id") == nil {
		t.Fatal("expected --id flag to be registered")
	}
	if pair.Action.Flags().Lookup("dryrun") == nil {
		t.Fatal("expected --dryrun flag to be registered")
	}
}

// #endregion
