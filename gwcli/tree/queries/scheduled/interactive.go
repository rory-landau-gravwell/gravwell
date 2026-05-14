package scheduled

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/ingest/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// This file implements interactive versions of scheduled search actions.

//#region shared scheduled search item

type ssItem struct {
	Selected_ bool
	ID_       string
	name      string
	desc      string
	query     string
	schedule  string
	disabled  bool
}

func (i ssItem) FilterValue() string { return i.name + i.query + i.desc }
func (i ssItem) Title() string       { return i.name }
func (i ssItem) ID() string          { return i.ID_ }
func (i ssItem) Description() string {
	state := "enabled"
	if i.disabled {
		state = "disabled"
	}
	return fmt.Sprintf("(%s) [%s] %s", state, i.schedule, i.query)
}
func (i *ssItem) SetSelected(selected bool) { i.Selected_ = selected }
func (i ssItem) Selected() bool             { return i.Selected_ }

func fetchScheduledItems() ([]multiselectlist.SelectableItem[string], error) {
	list, err := connection.Client.ListScheduledSearches(nil)
	if err != nil {
		return nil, err
	}
	var itms = make([]multiselectlist.SelectableItem[string], 0, len(list.Results))
	for _, ss := range list.Results {
		itms = append(itms, &ssItem{
			ID_:      ss.ID,
			name:     ss.Name,
			desc:     ss.Description,
			query:    ss.SearchString,
			schedule: ss.Schedule,
			disabled: ss.Disabled,
		})
	}
	return slices.Clip(itms), nil
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
	return action.NewPair(cmd, &ssCancelModel{})
}

type ssCancelModel struct {
	m multiselectlist.Model[string]
}

func (c *ssCancelModel) Init() tea.Cmd { return nil }

func (c *ssCancelModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	c.m, cmd = c.m.Update(msg)
	if c.m.Done() {
		selected := c.m.GetSelectedItems()
		if len(selected) == 0 {
			c.m.Undone()
			return c.m.NewStatusMessage("select at least 1 scheduled search")
		}
		var cmds []tea.Cmd
		for _, li := range selected {
			if err := connection.Client.CancelScheduledSearch(li.ID()); err != nil {
				cmds = append(cmds, tea.Printf("failed to cancel scheduled search '%s': %v", li.Title(), err))
				continue
			}
			cmds = append(cmds, tea.Printf("successfully cancelled scheduled search '%s'", li.Title()))
		}
		cmd = tea.Sequence(cmds...)
	}
	return cmd
}

func (c *ssCancelModel) View() string { return c.m.View() }
func (c *ssCancelModel) Done() bool   { return c.m.Done() }
func (c *ssCancelModel) Reset() error { c.m = multiselectlist.Model[string]{}; return nil }

func (c *ssCancelModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select scheduled searches to cancel"
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

			if enable, err := c.Flags().GetBool("enable"); err != nil {
				clilog.GetFlag(err)
			} else if enable {
				ss.BackfillEnabled = true
			}
			if disable, err := c.Flags().GetBool("disable"); err != nil {
				clilog.GetFlag(err)
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

	return action.NewPair(cmd, &ssBackfillModel{})
}

type ssBackfillModel struct {
	m multiselectlist.Model[string]
}

func (c *ssBackfillModel) Init() tea.Cmd { return nil }

func (c *ssBackfillModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	c.m, cmd = c.m.Update(msg)
	if c.m.Done() {
		selected := c.m.GetSelectedItems()
		if len(selected) == 0 {
			c.m.Undone()
			return c.m.NewStatusMessage("select at least 1 scheduled search")
		}
		var cmds []tea.Cmd
		for _, li := range selected {
			ss, err := connection.Client.GetScheduledSearch(li.ID())
			if err != nil {
				cmds = append(cmds, tea.Printf("failed to get scheduled search '%s': %v", li.Title(), err))
				continue
			}
			ss.BackfillEnabled = !ss.BackfillEnabled
			if err := connection.Client.UpdateScheduledSearch(ss); err != nil {
				cmds = append(cmds, tea.Printf("failed to toggle backfill for '%s': %v", li.Title(), err))
				continue
			}
			state := "enabled"
			if !ss.BackfillEnabled {
				state = "disabled"
			}
			cmds = append(cmds, tea.Printf("scheduled search '%s' backfill %s", li.Title(), state))
		}
		cmd = tea.Sequence(cmds...)
	}
	return cmd
}

func (c *ssBackfillModel) View() string { return c.m.View() }
func (c *ssBackfillModel) Done() bool   { return c.m.Done() }
func (c *ssBackfillModel) Reset() error { c.m = multiselectlist.Model[string]{}; return nil }

func (c *ssBackfillModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select scheduled searches to toggle backfill"
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
	return action.NewPair(cmd, &setOffsetModel{})
}

type setOffsetModel struct {
	m     multiselectlist.Model[string]
	ti    textinput.Model
	stage soStage

	selectedID   string
	selectedName string
}

func (c *setOffsetModel) Init() tea.Cmd { return nil }

func (c *setOffsetModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	switch c.stage {
	case soStgSelect:
		c.m, cmd = c.m.Update(msg)
		if c.m.Done() {
			selected := c.m.GetSelectedItems()
			if len(selected) != 1 {
				c.m.Undone()
				return c.m.NewStatusMessage("select exactly 1 scheduled search")
			}
			c.selectedID = selected[0].ID()
			c.selectedName = selected[0].Title()
			c.stage = soStgInput
			c.ti.Focus()
			return textinput.Blink
		}
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
		return c.m.View()
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
	c.m = multiselectlist.Model[string]{}
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
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select a scheduled search to set offset"

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
	return action.NewPair(cmd, &ssClearResultsModel{})
}

type ssClearResultsModel struct {
	m multiselectlist.Model[string]
}

func (c *ssClearResultsModel) Init() tea.Cmd { return nil }

func (c *ssClearResultsModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	c.m, cmd = c.m.Update(msg)
	if c.m.Done() {
		selected := c.m.GetSelectedItems()
		if len(selected) == 0 {
			c.m.Undone()
			return c.m.NewStatusMessage("select at least 1 scheduled search")
		}
		var cmds []tea.Cmd
		for _, li := range selected {
			if err := connection.Client.ClearScheduledSearchResults(li.ID()); err != nil {
				cmds = append(cmds, tea.Printf("failed to clear results for '%s': %v", li.Title(), err))
				continue
			}
			cmds = append(cmds, tea.Printf("successfully cleared results for scheduled search '%s'", li.Title()))
		}
		cmd = tea.Sequence(cmds...)
	}
	return cmd
}

func (c *ssClearResultsModel) View() string { return c.m.View() }
func (c *ssClearResultsModel) Done() bool   { return c.m.Done() }
func (c *ssClearResultsModel) Reset() error { c.m = multiselectlist.Model[string]{}; return nil }

func (c *ssClearResultsModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchScheduledItems()
	if err != nil {
		clilog.Writer.Error("failed to list scheduled searches", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list scheduled searches")
	}
	if len(itms) == 0 {
		return "there are no scheduled searches", nil, nil
	}
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select scheduled searches to clear results"
	return "", nil, nil
}

//#endregion clear-results
