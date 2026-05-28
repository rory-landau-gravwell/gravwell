/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

/*
Package scaffoldedit provides a template for building actions that modify existing data.

An edit action allows the user to select an entity from a list of all available entities, modify its
fields, and reflect the changes to the server.

Implementors must provide a SubroutineSet and a Config (map of scaffold.Field) to be displayed
after an item is selected for editing. Each Field must have a Provider set (e.g. a TextProvider).

The editing phase operates like scaffoldcreate: fields are navigated with arrow keys, providers
render their own inputs, and submission is confirmed via enter/space on the submit button.

The subroutines provide methods for scaffoldedit to find and manipulate data.

Example implementation: see macro edit action, which is fairly minimal.
*/
package scaffoldedit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	listHeightMax   = 40 // lines
	successStringF  = "successfully updated %v %v"
	initialMinWidth = 20 // lower clamp for startup width
)

// NewEditAction composes a usable edit action, returning its action pair.
// This is the function that implementors should call to create an edit action.
// Panics if any required parameters are missing.
func NewEditAction[I scaffold.Id_t, S any](singular, plural string, cfg Config, funcs SubroutineSet[I, S]) action.Pair {
	funcs.guarantee()
	if len(cfg) < 1 {
		panic("cannot edit with no fields defined")
	}
	if strings.TrimSpace(singular) == "" {
		panic("singular form of the noun cannot be empty")
	} else if strings.TrimSpace(plural) == "" {
		panic("plural form of the noun cannot be empty")
	}

	fs := generateFlagSet(cfg, singular)

	cmd := treeutils.GenerateAction(
		"edit",
		"edit a "+singular,
		"edit/alter an existing "+singular,
		[]string{"e"},
		func(cmd *cobra.Command, args []string) error {
			noInteractive, err := cmd.Flags().GetBool(ft.NoInteractive.Name())
			if err != nil {
				return err
			}
			if noInteractive {
				return runNonInteractive(cmd, cfg, funcs, singular)
			}
			return runInteractive(cmd, args)
		})

	cmd.Flags().AddFlagSet(&fs)

	return action.NewPair(cmd,
		newEditModel(cfg, singular, plural, funcs, fs),
	)
}

// generateFlagSet builds a pflag.FlagSet from the Config fields plus the native --id flag.
func generateFlagSet(cfg Config, singular string) pflag.FlagSet {
	fs := scaffold.InstallFlagsFromFields(cfg)
	fs.StringP("id", "i", "", fmt.Sprintf("id of the %v to edit", singular))
	return fs
}

// runNonInteractive handles --no-interactive mode:
//  1. Requires --id to identify the target item.
//  2. Calls PrepopulateSub to fill providers with current values.
//  3. Calls ApplyChangedFlags to override providers with any explicitly-set flags.
//  4. Calls EditSub to submit the updated item.
func runNonInteractive[I scaffold.Id_t, S any](cmd *cobra.Command, cfg Config, funcs SubroutineSet[I, S], singular string) error {
	var (
		id   I
		zero I
	)
	strid, err := cmd.Flags().GetString("id")
	if err != nil {
		return err
	}
	id, err = scaffold.FromString[I](strid)
	if err != nil {
		return err
	}
	if id == zero {
		return errors.New("--id is required in no-interactive mode")
	}

	itm, err := funcs.SelectSub(id)
	if err != nil {
		return fmt.Errorf("failed to select %s (id: %v): %w", singular, id, err)
	}

	// pre-fill providers with current values
	funcs.PrepopulateSub(itm, cfg)

	// apply any flags that were explicitly set
	fs := cmd.Flags()
	anyChanged, err := scaffold.ApplyChangedFlags(fs, cfg)
	if err != nil {
		return err
	}
	if !anyChanged {
		return errors.New("no field would be updated; quitting...")
	}

	identifier, invalid, err := funcs.EditSub(&itm, cfg, fs)
	if err != nil {
		return err
	}
	if invalid != "" {
		return errors.New(invalid)
	}
	fmt.Fprintf(cmd.OutOrStdout(), successStringF+"\n", singular, identifier)
	return nil
}

// runInteractive boots Mother to handle the interactive edit session.
func runInteractive(cmd *cobra.Command, args []string) error {
	return mother.Spawn(cmd.Root(), cmd, args)
}

//#region interactive mode (model)

type mode = uint8

const (
	quitting  mode = iota // done; mother should reassert
	selecting             // picking an item from the list
	editing               // item selected; user is editing fields
	idle                  // inactive
)

type editModel[I scaffold.Id_t, S any] struct {
	mode             mode
	fs               pflag.FlagSet
	singular, plural string
	width, height    int
	funcs            SubroutineSet[I, S]

	// cfg holds the fields and their providers.
	// Providers are mutated in-place as values are set.
	cfg Config

	data []S

	list            list.Model
	listInitialized bool

	editing stateEdit[S]
}

func newEditModel[I scaffold.Id_t, S any](cfg Config, singular, plural string,
	funcs SubroutineSet[I, S], initialFS pflag.FlagSet) *editModel[I, S] {
	return &editModel[I, S]{
		mode:     idle,
		fs:       initialFS,
		singular: singular,
		plural:   plural,
		cfg:      cfg,
		funcs:    funcs,
	}
}

// SetArgs parses tokens into the model's own flag set.
// The outer *pflag.FlagSet (fs) is intentionally unused: the model maintains
// em.fs internally to persist flag state across multiple Update cycles.
func (em *editModel[I, S]) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (
	invalid string, onStart tea.Cmd, err error) {

	if err := em.fs.Parse(tokens); err != nil {
		return err.Error(), nil, nil
	}

	// if --id was given, jump directly to editing mode
	if em.fs.Changed("id") {
		strid, err := em.fs.GetString("id")
		if err != nil {
			return "", nil, err
		}
		id, err := scaffold.FromString[I](strid)
		if err != nil {
			return "failed to parse id from " + strid, nil, nil
		}
		itm, err := em.funcs.SelectSub(id)
		if err != nil {
			return fmt.Sprintf("failed to fetch %s by id (%v): %v", em.singular, id, err), nil, nil
		}
		if err := em.enterEditMode(itm, width); err != nil {
			em.mode = quitting
			clilog.Writer.Errorf("%v", err)
			return "", nil, err
		}
		return "", nil, nil
	}

	// fetch all editable items for the list
	em.data, err = em.funcs.FetchSub()
	if err != nil {
		return
	}
	dataCount := len(em.data)
	if dataCount < 1 {
		em.mode = quitting
		return "", tea.Printf("You have no %v that can be edited", em.plural), nil
	}

	itms := make([]list.Item, dataCount)
	for i, s := range em.data {
		itms[i] = listItem{em.funcs.GetTitleSub(s), em.funcs.GetDescriptionSub(s)}
	}

	em.width = max(width, initialMinWidth)
	em.height = height
	em.list = stylesheet.NewList(itms, em.width, em.height, em.singular, em.plural)
	hotkeys.ApplyToList(&em.list.KeyMap)
	em.listInitialized = true
	em.mode = selecting

	return "", nil, nil
}

func (em *editModel[I, S]) Update(msg tea.Msg) tea.Cmd {
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		em.width = wsMsg.Width
		em.height = wsMsg.Height
		if em.listInitialized {
			em.list.SetHeight(min(wsMsg.Height-6, listHeightMax))
			em.list.SetWidth(em.width)
		}
	}

	switch em.mode {
	case quitting:
		return nil
	case selecting:
		return em.updateSelecting(msg)
	case editing:
		cmd, identifier := em.editing.update(msg, em.cfg, em.funcs.EditSub, &em.fs)
		if identifier != "" {
			em.mode = quitting
			return tea.Printf(successStringF, em.singular, identifier)
		}
		return cmd
	default:
		clilog.Writer.Criticalf("unknown edit mode %v.", em.mode)
		em.mode = quitting
		return textinput.Blink
	}
}

func (em *editModel[I, S]) updateSelecting(msg tea.Msg) tea.Cmd {
	if hotkeys.Match(msg, hotkeys.Invoke) {
		itm := em.data[em.list.GlobalIndex()]
		if err := em.enterEditMode(itm, em.width); err != nil {
			em.mode = quitting
			clilog.Writer.Errorf("%v", err)
			return tea.Println(err.Error())
		}
		return textinput.Blink
	}
	var cmd tea.Cmd
	em.list, cmd = em.list.Update(msg)
	return cmd
}

func (em *editModel[I, S]) View() string {
	switch em.mode {
	case quitting:
		return ""
	case selecting:
		return em.list.View() + "\n" +
			stylesheet.Cur.ExampleText.
				AlignHorizontal(lipgloss.Center).
				Width(em.width).
				Render("Press space or enter to select")
	case editing:
		return em.editing.view(em.cfg)
	default:
		clilog.Writer.Errorf("unknown mode %v", em.mode)
		em.mode = quitting
		return ""
	}
}

func (em *editModel[I, S]) Done() bool {
	return em.mode == quitting
}

func (em *editModel[I, S]) Reset() error {
	em.mode = idle
	em.data = nil
	em.fs = generateFlagSet(em.cfg, em.singular)
	em.list = list.Model{}
	em.listInitialized = false
	em.editing.reset()
	// reset all providers
	for _, key := range scaffold.SortFieldKeys(em.cfg) {
		em.cfg[key].Provider.Reset()
	}
	return nil
}

// enterEditMode transitions the model to editing mode for the given item.
// It calls PrepopulateSub to pre-fill providers with the item's current values,
// then applies any flags that were explicitly set.
func (em *editModel[I, S]) enterEditMode(item S, width int) error {
	// reset providers from any previous edit
	for _, key := range scaffold.SortFieldKeys(em.cfg) {
		em.cfg[key].Provider.Reset()
	}

	// pre-fill providers with the item's current values
	em.funcs.PrepopulateSub(item, em.cfg)

	// apply any flags that were explicitly set
	if _, err := scaffold.ApplyChangedFlags(&em.fs, em.cfg); err != nil {
		return err
	}

	// call SetArgs hooks on providers
	for _, key := range scaffold.SortFieldKeys(em.cfg) {
		em.cfg[key].Provider.SetArgs(width, 0)
	}

	ordered := scaffold.SortFieldKeys(em.cfg)
	if len(ordered) == 0 {
		return errors.New("no fields available to edit")
	}

	em.editing = stateEdit[S]{
		item:               item,
		ordered:            ordered,
		longestTitleLength: scaffold.LongestTitleLen(em.cfg),
		width:              width,
	}

	// focus the first field
	em.editing.focusInput(em.cfg, true)
	em.mode = editing
	return nil
}

//#endregion interactive mode (model)

// listItem is the list.Item implementation used in the selecting state.
type listItem struct {
	title       string
	description string
}

var _ stylesheet.ListItem = listItem{}

func (i listItem) Title() string       { return i.title }
func (i listItem) Description() string { return i.description }
func (i listItem) FilterValue() string { return i.title }

