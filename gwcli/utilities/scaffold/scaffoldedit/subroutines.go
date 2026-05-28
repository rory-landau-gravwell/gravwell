/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffoldedit

import (
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/spf13/pflag"
)

// SelectSubroutine fetches a specific, edit-able struct by its ID.
// Used both in interactive mode (when an explicit --id is given) and in non-interactive mode.
type SelectSubroutine[I scaffold.Id_t, S any] func(id I) (
	item S, err error,
)

// FetchAllSubroutine returns all edit-able data for the list. Not used in non-interactive mode.
type FetchAllSubroutine[S any] func() (
	items []S, err error,
)

// GetTitleSubroutine returns the display title for an item in the selection list.
type GetTitleSubroutine[S any] func(item S) string

// GetDescriptionSubroutine returns the display description for an item in the selection list.
type GetDescriptionSubroutine[S any] func(item S) string

// PrepopulateSubroutine populates the edit fields with the current values of a selected item.
// It is called after an item is selected (just before the user sees the edit form) and
// should call Provider.Set() for each field to pre-fill it with the item's current value.
type PrepopulateSubroutine[S any] func(item S, fields map[string]scaffold.Field)

// EditFuncT is the function called when the user submits the edit form.
// It receives a pointer to the selected item, the populated fields, and the current flag set.
// The implementor should read field values via fields[key].Provider.Get() and apply them to the item,
// then push the changes to the server.
//
// Returns:
//   - identifier: a human-readable identifier for the updated item (e.g., its name), shown on success.
//   - invalid: a reason the edit attempt was invalid (or empty string). Shown to the user but does not kill the action.
//   - err: an unrecoverable error (or nil).
type EditFuncT[S any] func(item *S, fields map[string]scaffold.Field, fs *pflag.FlagSet) (identifier, invalid string, err error)

// SubroutineSet defines the complete set of subroutines required by a scaffoldedit implementation.
//
// ! NewEditAction will panic if any required subroutine is nil.
type SubroutineSet[I scaffold.Id_t, S any] struct {
	SelectSub         SelectSubroutine[I, S]       // fetch a specific editable struct
	FetchSub          FetchAllSubroutine[S]         // fetch all editable structs for the list
	GetTitleSub       GetTitleSubroutine[S]         // get display title for list
	GetDescriptionSub GetDescriptionSubroutine[S]   // get display description for list
	PrepopulateSub    PrepopulateSubroutine[S]      // pre-fill fields with item's current values
	EditSub           EditFuncT[S]                  // submit the edited item
}

// guarantee validates that all required subroutines are set.
// Panics if any are missing.
func (funcs *SubroutineSet[I, S]) guarantee() {
	if funcs.SelectSub == nil {
		panic("select subroutine is required")
	}
	if funcs.FetchSub == nil {
		panic("fetch all subroutine is required")
	}
	if funcs.GetTitleSub == nil {
		panic("get title subroutine is required")
	}
	if funcs.GetDescriptionSub == nil {
		panic("get description subroutine is required")
	}
	if funcs.PrepopulateSub == nil {
		panic("prepopulate subroutine is required")
	}
	if funcs.EditSub == nil {
		panic("edit subroutine is required")
	}
}

