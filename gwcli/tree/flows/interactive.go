package flows

import (
	"errors"
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/ingest/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// This file implements interactive versions of flow actions: cancel, toggle-backfill, and clear-results.

//#region shared flow item

type flowItem struct {
	Selected_ bool
	ID_       string
	name      string
	desc      string
	schedule  string
	disabled  bool
}

func (i flowItem) FilterValue() string { return i.name + i.desc }
func (i flowItem) Title() string       { return i.name }
func (i flowItem) ID() string          { return i.ID_ }
func (i flowItem) Description() string {
	state := "enabled"
	if i.disabled {
		state = "disabled"
	}
	return fmt.Sprintf("(%s) [%s] %s", state, i.schedule, i.desc)
}
func (i *flowItem) SetSelected(selected bool) { i.Selected_ = selected }
func (i flowItem) Selected() bool             { return i.Selected_ }

// fetchFlowItems returns flow items for the multiselect list.
func fetchFlowItems() ([]multiselectlist.SelectableItem[string], error) {
	baseList, err := connection.Client.ListFlows(nil)
	if err != nil {
		return nil, err
	}
	var itms = make([]multiselectlist.SelectableItem[string], 0, len(baseList.Results))
	for _, f := range baseList.Results {
		itms = append(itms, &flowItem{
			ID_:      f.ID,
			name:     f.Name,
			desc:     f.Description,
			schedule: f.Schedule,
			disabled: f.Disabled,
		})
	}
	return slices.Clip(itms), nil
}

//#endregion shared flow item

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
	return action.NewPair(cmd, &cancelModel{})
}

type cancelModel struct {
	m multiselectlist.Model[string]
}

func (c *cancelModel) Init() tea.Cmd { return nil }

func (c *cancelModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	c.m, cmd = c.m.Update(msg)
	if c.m.Done() {
		selected := c.m.GetSelectedItems()
		if len(selected) == 0 {
			c.m.Undone()
			return c.m.NewStatusMessage("select at least 1 flow")
		}
		var cmds []tea.Cmd
		for _, li := range selected {
			if err := connection.Client.CancelFlow(li.ID()); err != nil {
				cmds = append(cmds, tea.Printf("failed to cancel flow '%s': %v", li.Title(), err))
				continue
			}
			cmds = append(cmds, tea.Printf("successfully cancelled flow '%s' (ID: %s)", li.Title(), li.ID()))
		}
		cmd = tea.Sequence(cmds...)
	}
	return cmd
}

func (c *cancelModel) View() string  { return c.m.View() }
func (c *cancelModel) Done() bool    { return c.m.Done() }
func (c *cancelModel) Reset() error  { c.m = multiselectlist.Model[string]{}; return nil }

func (c *cancelModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchFlowItems()
	if err != nil {
		clilog.Writer.Error("failed to list flows", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list flows")
	}
	if len(itms) == 0 {
		return "there are no flows", nil, nil
	}
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select flows to cancel"
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

			if enable, err := c.Flags().GetBool("enable"); err != nil {
				clilog.GetFlag(err)
			} else if enable {
				flow.BackfillEnabled = true
			}
			if disable, err := c.Flags().GetBool("disable"); err != nil {
				clilog.GetFlag(err)
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

	return action.NewPair(cmd, &backfillToggleModel{})
}

type backfillToggleModel struct {
	m multiselectlist.Model[string]
}

func (c *backfillToggleModel) Init() tea.Cmd { return nil }

func (c *backfillToggleModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	c.m, cmd = c.m.Update(msg)
	if c.m.Done() {
		selected := c.m.GetSelectedItems()
		if len(selected) == 0 {
			c.m.Undone()
			return c.m.NewStatusMessage("select at least 1 flow")
		}
		var cmds []tea.Cmd
		for _, li := range selected {
			flow, err := connection.Client.GetFlow(li.ID())
			if err != nil {
				cmds = append(cmds, tea.Printf("failed to get flow '%s': %v", li.Title(), err))
				continue
			}
			flow.BackfillEnabled = !flow.BackfillEnabled
			if err := connection.Client.UpdateFlow(flow); err != nil {
				cmds = append(cmds, tea.Printf("failed to toggle backfill for flow '%s': %v", li.Title(), err))
				continue
			}
			state := "enabled"
			if !flow.BackfillEnabled {
				state = "disabled"
			}
			cmds = append(cmds, tea.Printf("flow '%s' backfill %s", li.Title(), state))
		}
		cmd = tea.Sequence(cmds...)
	}
	return cmd
}

func (c *backfillToggleModel) View() string  { return c.m.View() }
func (c *backfillToggleModel) Done() bool    { return c.m.Done() }
func (c *backfillToggleModel) Reset() error  { c.m = multiselectlist.Model[string]{}; return nil }

func (c *backfillToggleModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchFlowItems()
	if err != nil {
		clilog.Writer.Error("failed to list flows", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list flows")
	}
	if len(itms) == 0 {
		return "there are no flows", nil, nil
	}
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select flows to toggle backfill"
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
	return action.NewPair(cmd, &clearResultsModel{})
}

type clearResultsModel struct {
	m multiselectlist.Model[string]
}

func (c *clearResultsModel) Init() tea.Cmd { return nil }

func (c *clearResultsModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	c.m, cmd = c.m.Update(msg)
	if c.m.Done() {
		selected := c.m.GetSelectedItems()
		if len(selected) == 0 {
			c.m.Undone()
			return c.m.NewStatusMessage("select at least 1 flow")
		}
		var cmds []tea.Cmd
		for _, li := range selected {
			if err := connection.Client.ClearFlowResults(li.ID()); err != nil {
				cmds = append(cmds, tea.Printf("failed to clear results for flow '%s': %v", li.Title(), err))
				continue
			}
			cmds = append(cmds, tea.Printf("successfully cleared results for flow '%s' (ID: %s)", li.Title(), li.ID()))
		}
		cmd = tea.Sequence(cmds...)
	}
	return cmd
}

func (c *clearResultsModel) View() string  { return c.m.View() }
func (c *clearResultsModel) Done() bool    { return c.m.Done() }
func (c *clearResultsModel) Reset() error  { c.m = multiselectlist.Model[string]{}; return nil }

func (c *clearResultsModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	itms, err := fetchFlowItems()
	if err != nil {
		clilog.Writer.Error("failed to list flows", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to list flows")
	}
	if len(itms) == 0 {
		return "there are no flows", nil, nil
	}
	c.m = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	c.m.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	c.m.StatusMessageOnSelect = true
	c.m.Title = "Select flows to clear results"
	return "", nil, nil
}

//#endregion clear-results
