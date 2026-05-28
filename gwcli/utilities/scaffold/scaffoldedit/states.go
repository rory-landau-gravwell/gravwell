/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffoldedit

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/spf13/pflag"
)

// stateEdit is the collection of state required to display and manage the fields of an item being edited.
// It is prepared by editModel.enterEditMode().
type stateEdit[S any] struct {
	// err holds the most recent validation or update error message.
	err string
	// updateErr holds an error from the EditSub call itself (not field validation).
	updateErr string
	// item is the struct being edited.
	item S

	// selected is the index of the currently focused field (len(ordered) == submit button).
	selected uint
	// ordered is the sorted list of field keys (descending by Order, then alpha by Title).
	ordered []string
	// longestTitleLength is the width of the longest field title, used for alignment.
	longestTitleLength int
	// takeover is the key of the field currently asserting full-pane control.
	takeover string

	width int
}

// submitSelected returns true if the submit button is currently focused.
func (se *stateEdit[S]) submitSelected() bool {
	return se.selected == uint(len(se.ordered))
}

// update handles a tea.Msg during edit mode.
// Returns:
//   - cmd: any tea.Cmd to issue
//   - identifier: a non-empty string means the update was submitted and succeeded
func (se *stateEdit[S]) update(
	msg tea.Msg,
	fields map[string]scaffold.Field,
	editSub EditFuncT[S],
	fs *pflag.FlagSet,
) (_ tea.Cmd, identifier string) {
	// clear errors on any key
	if _, ok := msg.(tea.KeyMsg); ok {
		se.err = ""
		se.updateErr = ""
	}

	// handle takeover mode: all updates go to the takeover provider
	if se.takeover != "" {
		cmd, tko := fields[se.takeover].Provider.Update(true, msg)
		if !tko {
			se.takeover = ""
		}
		return cmd, ""
	}

	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		se.width = wsm.Width
		// forward to all fields
		var cmds []tea.Cmd
		for i, key := range se.ordered {
			if cmd, _ := fields[key].Provider.Update(i == int(se.selected), msg); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if len(cmds) > 0 {
			return tea.Batch(cmds...), ""
		}
		return nil, ""
	}

	if hotkeys.Match(msg, hotkeys.CursorUp) {
		se.focusPrevious(fields)
		return textinput.Blink, ""
	} else if hotkeys.Match(msg, hotkeys.CursorDown) {
		se.focusNext(fields)
		return textinput.Blink, ""
	}

	if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) && se.submitSelected() {
		// validate required fields
		se.checkSatisfaction(fields, false)
		if se.err != "" {
			return nil, ""
		}
		// call the edit function
		ident, invalid, err := editSub(&se.item, fields, fs)
		if err != nil {
			se.updateErr = err.Error()
			return nil, ""
		} else if invalid != "" {
			se.err = invalid
			return nil, ""
		}
		return nil, ident
	}

	if se.submitSelected() {
		return nil, ""
	}

	// pass the message to the currently selected provider
	p := fields[se.ordered[se.selected]].Provider
	cmd, tko := p.Update(true, msg)
	if tko {
		se.takeover = se.ordered[se.selected]
	}
	se.checkSatisfaction(fields, false)
	return cmd, ""
}

// view renders the edit form for the currently selected item.
func (se *stateEdit[S]) view(fields map[string]scaffold.Field) string {
	if se.takeover != "" {
		_, v, _ := fields[se.takeover].Provider.View(true, se.width)
		return v
	}

	views, setWidth := se.collectViewValues(fields)
	if setWidth == 0 {
		setWidth = se.width
	}

	centerSty := lipgloss.NewStyle().Width(setWidth).AlignHorizontal(lipgloss.Center)
	lines := make([]string, 0, len(views))
	for _, v := range views {
		if v.toCenter {
			lines = append(lines, centerSty.MaxHeight(2).Render(v.content))
		} else {
			lines = append(lines, v.content)
		}
	}

	mainView := lipgloss.JoinVertical(lipgloss.Left, lines...)
	sbtn := stylesheet.ViewSubmitButton(se.submitSelected(), setWidth, se.err, se.updateErr)
	return lipgloss.NewStyle().AlignHorizontal(lipgloss.Left).Render(mainView) + "\n" + sbtn
}

// material is a view component, matching the analogous type in scaffoldcreate.
type material struct {
	content  string
	toCenter bool
}

// collectViewValues gathers the material views from the provider fields.
// Returns the views (ordered as se.ordered) and the widest TitleValue line width.
func (se *stateEdit[S]) collectViewValues(fields map[string]scaffold.Field) (views []material, setWidth int) {
	views = make([]material, 0, len(se.ordered))
	for i, key := range se.ordered {
		field := fields[key]
		kind, value, secondLine := fields[key].Provider.View(i == int(se.selected), se.width)

		switch kind {
		case scaffold.TitleValue:
			padding := strings.Repeat(" ", se.longestTitleLength-len(field.Title))
			pip := stylesheet.Pip(se.selected, uint(i))
			var styledTitle string
			if field.Required {
				styledTitle = stylesheet.RequiredTitle(field.Title)
			} else {
				styledTitle = stylesheet.OptionalTitle(field.Title)
			}
			line := padding + pip + styledTitle + value
			views = append(views, material{content: line, toCenter: false})
			if w := lipgloss.Width(line); w > setWidth {
				setWidth = w
			}
		case scaffold.Line:
			views = append(views, material{
				content:  stylesheet.Pip(se.selected, uint(i)) + value,
				toCenter: true,
			})
		}
		if secondLine != "" {
			views = append(views, material{content: secondLine, toCenter: true})
		}
	}
	return views, setWidth
}

// checkSatisfaction validates fields and sets se.err on the first invalid field.
// Clears se.err if all fields are satisfied.
// If selectedOnly is true, only the currently selected field is checked.
func (se *stateEdit[S]) checkSatisfaction(fields map[string]scaffold.Field, selectedOnly bool) {
	check := func(f scaffold.Field) bool {
		if f.Required && f.Provider.Get() == "" {
			se.err = phrases.MissingRequiredFields([]string{f.Title})
			return true
		}
		if invalid := f.Provider.Satisfied(); invalid != "" {
			se.err = invalid
			return true
		}
		se.err = ""
		return false
	}
	if selectedOnly {
		if !se.submitSelected() {
			check(fields[se.ordered[se.selected]])
		}
		return
	}
	for _, key := range se.ordered {
		if check(fields[key]) {
			return
		}
	}
}

// focusNext blurs the current field and focuses the next one.
// Wraps from the last field to the submit button, and from the submit button to the first field.
func (se *stateEdit[S]) focusNext(fields map[string]scaffold.Field) {
	se.focusInput(fields, false)
	se.selected += 1
	if se.selected > uint(len(se.ordered)) {
		se.selected = 0
	}
	se.focusInput(fields, true)
}

// focusPrevious blurs the current field and focuses the previous one.
// Wraps from the first field to the submit button.
func (se *stateEdit[S]) focusPrevious(fields map[string]scaffold.Field) {
	se.focusInput(fields, false)
	if se.selected == 0 {
		se.selected = uint(len(se.ordered))
	} else {
		se.selected -= 1
	}
	se.focusInput(fields, true)
}

// focusInput toggles focus on the currently selected field (no-op if submit is selected).
func (se *stateEdit[S]) focusInput(fields map[string]scaffold.Field, focus bool) {
	if se.submitSelected() {
		return
	}
	key := se.ordered[se.selected]
	fields[key].Provider.ToggleFocus(focus)
}


// reset returns the stateEdit to its zero value.
func (se *stateEdit[S]) reset() {
	var zero S
	se.ordered = nil
	se.selected = 0
	se.item = zero
	se.err = ""
	se.updateErr = ""
	se.takeover = ""
	se.longestTitleLength = 0
}

