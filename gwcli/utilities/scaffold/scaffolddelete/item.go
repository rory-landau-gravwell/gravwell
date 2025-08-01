/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffolddelete

import (
	"fmt"
	"io"

	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

//#region Item implementation

// Item is the base functions a delete action must provide on the type it wants deleted
type Item[I scaffold.Id_t] struct {
	title       string
	description string
	id          I // value passed to the delete function

}

var _ stylesheet.ListItem = Item[uint64]{}

// NewItem returns a new item instance with the given basic information and unique identifier.
func NewItem[I scaffold.Id_t](title, description string, ID I) Item[I] {
	return Item[I]{title: title, description: description, id: ID}
}

// FilterValue returns the element of data that is compare against for filtration.
func (i Item[I]) FilterValue() string {
	return i.title
}

// Title gets the one-line representation of the item.
func (i Item[I]) Title() string {
	return i.title

}

// Description fetches the extra text to be displayed beneath item # and title for additional details.
func (i Item[I]) Description() string {
	return i.description

}

// #endregion
// the item delegate defines display format of an item in the list
type defaultDelegate[I scaffold.Id_t] struct {
	height     int
	spacing    int
	renderFunc func(w io.Writer, m list.Model, index int, listItem list.Item)
}

func (d defaultDelegate[I]) Height() int                           { return d.height }
func (d defaultDelegate[I]) Spacing() int                          { return d.spacing }
func (defaultDelegate[I]) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (dd defaultDelegate[I]) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	dd.renderFunc(w, m, index, listItem)
}

// default renderFunc used by the delegate if not overwritten by WithRender()
func defaultRender[I scaffold.Id_t](w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item[I])
	if !ok {
		return
	}

	str := fmt.Sprintf("%s%s. %s\n%s",
		stylesheet.Pip(uint(index), uint(m.Index())),
		stylesheet.Index(index+1),
		stylesheet.Cur.PrimaryText.Render(i.Title()),
		i.Description())
	fmt.Fprint(w, str)
}

// A DelegateOption is a modifier on the item delegate, typically to change how it is displayed.
type DelegateOption[I scaffold.Id_t] func(*defaultDelegate[I])

// WithHeight alters the number of lines allocated to each item.
// Height should be set equal to 1 + the lipgloss.Height of your Item.Details (1+ for Title) if
// using the default render function.
// Values above or below that can have... unpredictable... results.
func WithHeight[I scaffold.Id_t](h int) DelegateOption[I] {
	return func(dd *defaultDelegate[I]) { dd.height = h }
}

// WithSpacing alters the number of lines between each item.
func WithSpacing[I scaffold.Id_t](s int) DelegateOption[I] {
	return func(dd *defaultDelegate[I]) { dd.spacing = s }
}

// WithRender alters how each item is displayed in the list of delete-able items, using the given function into of the default item renderer.
func WithRender[I scaffold.Id_t](f func(w io.Writer, m list.Model, index int, listItem list.Item)) DelegateOption[I] {
	return func(dd *defaultDelegate[I]) { dd.renderFunc = f }
}
