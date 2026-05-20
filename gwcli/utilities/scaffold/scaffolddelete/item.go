/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffolddelete

import (
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
)

//#region Item implementation

// Item satisfies multiselectlist.SelectableItem and represents a delete-able entity.
type Item[I scaffold.Id_t] struct {
	title       string
	description string
	id          I    // value passed to the delete function
	selected    bool // selection state for multiselectlist
}

var _ multiselectlist.SelectableItem[uint64] = &Item[uint64]{}

// NewItem returns a new item instance with the given basic information and unique identifier.
func NewItem[I scaffold.Id_t](title, description string, ID I) Item[I] {
	return Item[I]{title: title, description: description, id: ID}
}

// FilterValue returns the element of data that is compared against for filtration.
func (i Item[I]) FilterValue() string {
	return i.title + i.description
}

// Title gets the one-line representation of the item.
func (i Item[I]) Title() string {
	return i.title
}

// Description fetches the extra text to be displayed beneath item title for additional details.
func (i Item[I]) Description() string {
	return i.description
}

// ID returns the unique identifier for this item.
func (i Item[I]) ID() I {
	return i.id
}

// Selected returns whether this item is currently selected.
func (i Item[I]) Selected() bool {
	return i.selected
}

// SetSelected sets the selection state of this item.
func (i *Item[I]) SetSelected(selected bool) {
	i.selected = selected
}

// #endregion
