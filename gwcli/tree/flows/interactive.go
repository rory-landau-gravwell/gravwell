package flows

import (
	"errors"
	"fmt"
	"slices"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/confirmation"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
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

// This file implements interactive versions of flow actions: cancel, toggle-backfill, and clear-results.

// listFlowItems returns flow items for use in a list.Model.
func listFlowItems() ([]list.Item, error) {
	baseList, err := connection.Client.ListFlows(nil)
	if err != nil {
		return nil, err
	}
	itms := make([]list.Item, 0, len(baseList.Results))
	for _, f := range baseList.Results {
		itms = append(itms, &listitem.Generic{
			ID_:          f.ID,
			Name:         f.Name,
			SecondLine:   fmt.Sprintf("[%s] %s", f.Schedule, f.Description),
			ShowDisabled: true,
			Enabled:      !f.Disabled,
		})
	}
	return slices.Clip(itms), nil
}

//#region cancel

func cancel() action.Pair {
	cmd := treeutils.GenerateAction("cancel", "cancel a running flow",
		"Cancel a currently-executing flow by its ID or GUID.",
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
				return errors.New(phrases.Exactly1ArgRequired("flow ID"))
			}
			id := c.Flags().Arg(0)
			if err := connection.Client.CancelFlow(id); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "successfully cancelled flow %s\n", id)
			return nil
		},
	)

	m := &cancelModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type cancelModel struct {
	selecting bool

	fList   list.Model
	confirm confirmation.Model

	done bool
}

func (c *cancelModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.fList, cmd = c.fList.Update(msg)
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

	itm, err := listitem.GetGeneric(&c.fList)
	if err != nil {
		return tea.Println(err)
	}
	c.done = true
	if err := connection.Client.CancelFlow(itm.ID()); err != nil {
		return tea.Printf("failed to cancel flow '%s': %v", itm.Name, err)
	}
	return tea.Printf("successfully cancelled flow '%s' (ID: %s)", itm.Name, itm.ID())
}

func (c *cancelModel) View() string {
	if c.selecting {
		return c.fList.View()
	}
	return c.confirm.View()
}

func (c *cancelModel) Done() bool { return c.done }
func (c *cancelModel) Reset() error {
	c.selecting = true
	c.fList = list.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *cancelModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := listFlowItems()
	if err != nil {
		clilog.Writer.Error("failed to list flows", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list flows")
	}
	if len(itms) == 0 {
		return "there are no flows", nil, nil
	}
	c.fList = stylesheet.NewList(itms, width, height, "flow", "flows")
	c.confirm.Init([]string{"flow selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion cancel

//#region toggle-backfill

func backfillToggle() action.Pair {
	cmd := treeutils.GenerateAction("toggle-backfill", "toggle flow backfill",
		"Toggle backfill for a flow. Use --enable or --disable to set explicitly.\nBackfill causes the automation to run for missed time periods.",
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
				return errors.New(phrases.Exactly1ArgRequired("flow ID"))
			}
			id := c.Flags().Arg(0)
			flow, err := connection.Client.GetFlow(id)
			if err != nil {
				return err
			}
			flow.BackfillEnabled = !flow.BackfillEnabled

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
				flow.BackfillEnabled = true
			} else if disable {
				flow.BackfillEnabled = false
			}

			if err := connection.Client.UpdateFlow(flow); err != nil {
				return err
			}
			state := "enabled"
			if !flow.BackfillEnabled {
				state = "disabled"
			}
			fmt.Fprintf(c.OutOrStdout(), "flow '%s' backfill %s\n", id, state)
			return nil
		},
	)
	cmd.Flags().Bool("enable", false, "enable backfill")
	cmd.Flags().Bool("disable", false, "disable backfill")

	m := &backfillToggleModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type backfillToggleModel struct {
	selecting bool

	fList   list.Model
	confirm confirmation.Model

	done bool
}

func (c *backfillToggleModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.fList, cmd = c.fList.Update(msg)
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

	itm, err := listitem.GetGeneric(&c.fList)
	if err != nil {
		return tea.Println(err)
	}
	flow, err := connection.Client.GetFlow(itm.ID())
	if err != nil {
		c.done = true
		return tea.Printf("failed to get flow '%s': %v", itm.Name, err)
	}
	flow.BackfillEnabled = !flow.BackfillEnabled
	if err := connection.Client.UpdateFlow(flow); err != nil {
		c.done = true
		return tea.Printf("failed to toggle backfill for flow '%s': %v", itm.Name, err)
	}
	state := "enabled"
	if !flow.BackfillEnabled {
		state = "disabled"
	}
	c.done = true
	return tea.Printf("flow '%s' backfill %s", itm.Name, state)
}

func (c *backfillToggleModel) View() string {
	if c.selecting {
		return c.fList.View()
	}
	return c.confirm.View()
}

func (c *backfillToggleModel) Done() bool { return c.done }
func (c *backfillToggleModel) Reset() error {
	c.selecting = true
	c.fList = list.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *backfillToggleModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := listFlowItems()
	if err != nil {
		clilog.Writer.Error("failed to list flows", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list flows")
	}
	if len(itms) == 0 {
		return "there are no flows", nil, nil
	}
	c.fList = stylesheet.NewList(itms, width, height, "flow", "flows")
	c.confirm.Init([]string{"flow selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion toggle-backfill

//#region clear-results

func clearResults() action.Pair {
	cmd := treeutils.GenerateAction("clear-results", "clear results for a flow",
		"Clear the execution results (including errors and state) for a flow.",
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
				return errors.New(phrases.Exactly1ArgRequired("flow ID"))
			}
			id := c.Flags().Arg(0)
			if err := connection.Client.ClearFlowResults(id); err != nil {
				return err
			}
			fmt.Fprintf(c.OutOrStdout(), "successfully cleared results for flow %s\n", id)
			return nil
		},
	)

	m := &clearResultsModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

type clearResultsModel struct {
	selecting bool

	fList   list.Model
	confirm confirmation.Model

	done bool
}

func (c *clearResultsModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.fList, cmd = c.fList.Update(msg)
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

	itm, err := listitem.GetGeneric(&c.fList)
	if err != nil {
		return tea.Println(err)
	}
	c.done = true
	if err := connection.Client.ClearFlowResults(itm.ID()); err != nil {
		return tea.Printf("failed to clear results for flow '%s': %v", itm.Name, err)
	}
	return tea.Printf("successfully cleared results for flow '%s' (ID: %s)", itm.Name, itm.ID())
}

func (c *clearResultsModel) View() string {
	if c.selecting {
		return c.fList.View()
	}
	return c.confirm.View()
}

func (c *clearResultsModel) Done() bool { return c.done }
func (c *clearResultsModel) Reset() error {
	c.selecting = true
	c.fList = list.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *clearResultsModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := listFlowItems()
	if err != nil {
		clilog.Writer.Error("failed to list flows", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list flows")
	}
	if len(itms) == 0 {
		return "there are no flows", nil, nil
	}
	c.fList = stylesheet.NewList(itms, width, height, "flow", "flows")
	c.confirm.Init([]string{"flow selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion clear-results
