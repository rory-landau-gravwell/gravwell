package scheduled

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	blist "github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/confirmation"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/ingest/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// This file implements interactive versions of scheduled search actions.

//#region shared scheduled search item

type ssItem struct {
	id       string
	name     string
	desc     string
	query    string
	schedule string
	disabled bool
}

func (i ssItem) FilterValue() string { return i.name + i.query + i.desc }
func (i ssItem) Title() string       { return i.name }
func (i ssItem) Description() string {
	state := "enabled"
	if i.disabled {
		state = "disabled"
	}
	return fmt.Sprintf("(%s) [%s] %s", state, i.schedule, i.query)
}

func fetchScheduledItems() ([]blist.Item, error) {
	l, err := connection.Client.ListScheduledSearches(nil)
	if err != nil {
		return nil, err
	}
	itms := make([]blist.Item, 0, len(l.Results))
	for _, ss := range l.Results {
		itms = append(itms, &ssItem{
			id:       ss.ID,
			name:     ss.Name,
			desc:     ss.Description,
			query:    ss.SearchString,
			schedule: ss.Schedule,
			disabled: ss.Disabled,
		})
	}
	return slices.Clip(itms), nil
}

// getScheduledSearch asserts that the currently selected item in l is a *ssItem and returns it.
func getScheduledSearch(l *blist.Model) (*ssItem, error) {
	ss, ok := l.SelectedItem().(*ssItem)
	if !ok {
		return nil, clilog.TypeAssert(l.SelectedItem(), &ssItem{})
	}
	return ss, nil
}

//#endregion shared scheduled search item

//#region cancel

func cancelAction() action.Pair {
	cmd := treeutils.GenerateAction("cancel", "cancel a running scheduled search",
		"Cancel a currently-executing scheduled search by its ID.",
		nil,
		func(c *cobra.Command, args []string) error {
			if c.Flags().NArg() == 0 {
				ni, err := c.Flags().GetBool(ft.NoInteractive.Name())
				if err != nil {
					clilog.GetFlag(err)
					ni = true
				}
				if !ni {
					return mother.Spawn(c.Root(), c, args)
				}
				return errors.New(phrases.Exactly1ArgRequired("scheduled search ID"))
			}
			id := c.Flags().Arg(0)
			if err := connection.Client.CancelScheduledSearch(id); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "successfully cancelled scheduled search %s\n", id)
			return nil
		},
	)

	m := &ssCancelModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type ssCancelModel struct {
	selecting bool

	ssList  blist.Model
	confirm confirmation.Model

	done bool
}

func (c *ssCancelModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.ssList, cmd = c.ssList.Update(msg)
		}
		return cmd
	}

	var (
		selectionMade, confirmed bool
		ret                      uint
	)
	c.confirm, cmd, selectionMade, confirmed, ret = c.confirm.Update(msg)
	if !selectionMade {
		return cmd
	}

	if !confirmed {
		if ret != 0 {
			clilog.Writer.Error("user selected non-0 choice", log.KV("choice", ret), log.KV("confirmation view", c.confirm.View()))
		}
		c.selecting = true
		return nil
	}

	ss, err := getScheduledSearch(&c.ssList)
	if err != nil {
		return tea.Println(err)
	}
	c.done = true
	if err := connection.Client.CancelScheduledSearch(ss.id); err != nil {
		return tea.Printf("failed to cancel scheduled search '%s': %v", ss.name, err)
	}
	return tea.Printf("successfully cancelled scheduled search '%s'", ss.name)
}

func (c *ssCancelModel) View() string {
	if c.selecting {
		return c.ssList.View()
	}
	return c.confirm.View()
}

func (c *ssCancelModel) Done() bool { return c.done }
func (c *ssCancelModel) Reset() error {
	c.selecting = true
	c.ssList = blist.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *ssCancelModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.ssList = stylesheet.NewList(itms, width, height, "scheduled search", "scheduled searches")
	c.confirm.Init([]string{"scheduled search selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion cancel

//#region toggle-backfill

func backfillToggle() action.Pair {
	cmd := treeutils.GenerateAction("toggle-backfill", "toggle scheduled search backfill",
		"Toggle backfill for a scheduled search. Use --enable or --disable to set explicitly.",
		nil,
		func(c *cobra.Command, args []string) error {
			if c.Flags().NArg() == 0 {
				ni, err := c.Flags().GetBool(ft.NoInteractive.Name())
				if err != nil {
					clilog.GetFlag(err)
					ni = true
				}
				if !ni {
					return mother.Spawn(c.Root(), c, args)
				}
				return errors.New(phrases.Exactly1ArgRequired("scheduled search ID"))
			}
			id := c.Flags().Arg(0)
			ss, err := connection.Client.GetScheduledSearch(id)
			if err != nil {
				return err
			}
			ss.BackfillEnabled = !ss.BackfillEnabled

			enable, err := c.Flags().GetBool("enable")
			if err != nil {
				clilog.GetFlag(err)
			}
			disable, err := c.Flags().GetBool("disable")
			if err != nil {
				clilog.GetFlag(err)
			}
			if enable && disable {
				clilog.Writer.Warn("both enable and disable were set, failing out...")
				return clilog.ErrInternal{}
			}
			if enable {
				ss.BackfillEnabled = true
			} else if disable {
				ss.BackfillEnabled = false
			}

			if err := connection.Client.UpdateScheduledSearch(ss); err != nil {
				return err
			}
			state := "enabled"
			if !ss.BackfillEnabled {
				state = "disabled"
			}
			fmt.Fprintf(c.OutOrStdout(), "scheduled search '%s' backfill %s\n", id, state)
			return nil
		},
	)
	cmd.Flags().Bool("enable", false, "enable backfill")
	cmd.Flags().Bool("disable", false, "disable backfill")

	m := &ssBackfillModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type ssBackfillModel struct {
	selecting bool

	ssList  blist.Model
	confirm confirmation.Model

	done bool
}

func (c *ssBackfillModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.ssList, cmd = c.ssList.Update(msg)
		}
		return cmd
	}

	var (
		selectionMade, confirmed bool
		ret                      uint
	)
	c.confirm, cmd, selectionMade, confirmed, ret = c.confirm.Update(msg)
	if !selectionMade {
		return cmd
	}

	if !confirmed {
		if ret != 0 {
			clilog.Writer.Error("user selected non-0 choice", log.KV("choice", ret), log.KV("confirmation view", c.confirm.View()))
		}
		c.selecting = true
		return nil
	}

	item, err := getScheduledSearch(&c.ssList)
	if err != nil {
		return tea.Println(err)
	}
	ss, err := connection.Client.GetScheduledSearch(item.id)
	if err != nil {
		c.done = true
		return tea.Printf("failed to get scheduled search '%s': %v", item.name, err)
	}
	ss.BackfillEnabled = !ss.BackfillEnabled
	if err := connection.Client.UpdateScheduledSearch(ss); err != nil {
		c.done = true
		return tea.Printf("failed to toggle backfill for '%s': %v", item.name, err)
	}
	state := "enabled"
	if !ss.BackfillEnabled {
		state = "disabled"
	}
	c.done = true
	return tea.Printf("scheduled search '%s' backfill %s", item.name, state)
}

func (c *ssBackfillModel) View() string {
	if c.selecting {
		return c.ssList.View()
	}
	return c.confirm.View()
}

func (c *ssBackfillModel) Done() bool { return c.done }
func (c *ssBackfillModel) Reset() error {
	c.selecting = true
	c.ssList = blist.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *ssBackfillModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.ssList = stylesheet.NewList(itms, width, height, "scheduled search", "scheduled searches")
	c.confirm.Init([]string{"scheduled search selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion toggle-backfill

//#region set-offset

type soStage uint

const (
	soStgSelect soStage = iota
	soStgInput
	soStgDone
)

func setOffset() action.Pair {
	cmd := treeutils.GenerateAction("set-offset", "set the time offset for a scheduled search",
		"Set the time offset (in seconds, must be <= 0) for a scheduled search.\nNegative values represent seconds in the past from the scheduled execution time, controlling how far back the search's time window starts.",
		nil,
		func(c *cobra.Command, args []string) error {
			if c.Flags().NArg() < 2 {
				ni, err := c.Flags().GetBool(ft.NoInteractive.Name())
				if err != nil {
					clilog.GetFlag(err)
					ni = true
				}
				if !ni {
					return mother.Spawn(c.Root(), c, args)
				}
				return errors.New("exactly 2 arguments required: scheduled search ID and offset seconds")
			}
			id := c.Flags().Arg(0)
			offsetStr := c.Flags().Arg(1)
			offset, err := strconv.ParseInt(offsetStr, 10, 64)
			if err != nil {
				return fmt.Errorf("%s is not a valid integer", offsetStr)
			}
			if offset > 0 {
				return errors.New("offset must be <= 0")
			}
			ss, err := connection.Client.GetScheduledSearch(id)
			if err != nil {
				return err
			}
			ss.TimeframeOffset = offset
			if err := connection.Client.UpdateScheduledSearch(ss); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "successfully set offset for scheduled search %s to %d\n", id, offset)
			return nil
		},
	)

	m := &setOffsetModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type setOffsetModel struct {
	ssList blist.Model
	ti     textinput.Model
	stage  soStage

	selectedID   string
	selectedName string
}

func (c *setOffsetModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	switch c.stage {
	case soStgSelect:
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			ss, err := getScheduledSearch(&c.ssList)
			if err != nil {
				c.stage = soStgDone
				return tea.Println(err)
			}
			c.selectedID = ss.id
			c.selectedName = ss.name
			c.stage = soStgInput
			c.ti.Focus()
			return textinput.Blink
		}
		c.ssList, cmd = c.ssList.Update(msg)
	case soStgInput:
		if hotkeys.Match(msg, hotkeys.Invoke) {
			val := c.ti.Value()
			if val == "" {
				return nil
			}
			offset, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return nil // ignore non-numeric input
			}
			if offset > 0 {
				return nil // ignore positive offsets
			}
			ss, err := connection.Client.GetScheduledSearch(c.selectedID)
			if err != nil {
				c.stage = soStgDone
				return tea.Printf("failed to get scheduled search '%s': %v", c.selectedName, err)
			}
			ss.TimeframeOffset = offset
			if err := connection.Client.UpdateScheduledSearch(ss); err != nil {
				c.stage = soStgDone
				return tea.Printf("failed to set offset for '%s': %v", c.selectedName, err)
			}
			c.stage = soStgDone
			return tea.Printf("successfully set offset for scheduled search '%s' to %d", c.selectedName, offset)
		}
		if hotkeys.Match(msg, hotkeys.SoftQuit) {
			c.stage = soStgDone
			return tea.Println("cancelled")
		}
		c.ti, cmd = c.ti.Update(msg)
	}
	return cmd
}

func (c *setOffsetModel) View() string {
	switch c.stage {
	case soStgSelect:
		return c.ssList.View()
	case soStgInput:
		return fmt.Sprintf("Offset (seconds, <= 0) for '%s':\n%s\n\n  %s",
			c.selectedName,
			c.ti.View(),
			stylesheet.Cur.DisabledText.Render("↲ submit • esc cancel"))
	}
	return ""
}

func (c *setOffsetModel) Done() bool { return c.stage == soStgDone }

func (c *setOffsetModel) Reset() error {
	c.ssList = blist.Model{}
	c.ti = textinput.Model{}
	c.stage = soStgSelect
	c.selectedID = ""
	c.selectedName = ""
	return nil
}

func (c *setOffsetModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.ssList = stylesheet.NewList(itms, width, height, "scheduled search", "scheduled searches")

	c.ti = stylesheet.NewTI("", false)
	c.ti.Placeholder = "-3600"
	c.ti.Width = 20
	c.ti.Blur()

	return "", nil, nil
}

//#endregion set-offset

//#region clear-results

func clearResults() action.Pair {
	cmd := treeutils.GenerateAction("clear-results", "clear results for a scheduled search",
		"Clear the execution results (including errors and state) for a scheduled search.",
		[]string{"clear-error", "clear-state"},
		func(c *cobra.Command, args []string) error {
			if c.Flags().NArg() == 0 {
				ni, err := c.Flags().GetBool(ft.NoInteractive.Name())
				if err != nil {
					clilog.GetFlag(err)
					ni = true
				}
				if !ni {
					return mother.Spawn(c.Root(), c, args)
				}
				return errors.New(phrases.Exactly1ArgRequired("scheduled search ID"))
			}
			id := c.Flags().Arg(0)
			if err := connection.Client.ClearScheduledSearchResults(id); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "successfully cleared results for scheduled search %s\n", id)
			return nil
		},
	)

	m := &ssClearResultsModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type ssClearResultsModel struct {
	selecting bool

	ssList  blist.Model
	confirm confirmation.Model

	done bool
}

func (c *ssClearResultsModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.ssList, cmd = c.ssList.Update(msg)
		}
		return cmd
	}

	var (
		selectionMade, confirmed bool
		ret                      uint
	)
	c.confirm, cmd, selectionMade, confirmed, ret = c.confirm.Update(msg)
	if !selectionMade {
		return cmd
	}

	if !confirmed {
		if ret != 0 {
			clilog.Writer.Error("user selected non-0 choice", log.KV("choice", ret), log.KV("confirmation view", c.confirm.View()))
		}
		c.selecting = true
		return nil
	}

	ss, err := getScheduledSearch(&c.ssList)
	if err != nil {
		return tea.Println(err)
	}
	c.done = true
	if err := connection.Client.ClearScheduledSearchResults(ss.id); err != nil {
		return tea.Printf("failed to clear results for '%s': %v", ss.name, err)
	}
	return tea.Printf("successfully cleared results for scheduled search '%s'", ss.name)
}

func (c *ssClearResultsModel) View() string {
	if c.selecting {
		return c.ssList.View()
	}
	return c.confirm.View()
}

func (c *ssClearResultsModel) Done() bool { return c.done }
func (c *ssClearResultsModel) Reset() error {
	c.selecting = true
	c.ssList = blist.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *ssClearResultsModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.ssList = stylesheet.NewList(itms, width, height, "scheduled search", "scheduled searches")
	c.confirm.Init([]string{"scheduled search selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion clear-results
