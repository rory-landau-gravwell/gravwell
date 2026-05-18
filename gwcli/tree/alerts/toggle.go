package alerts

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
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

// This file implements the interactive toggle action for alerts.

func toggle() action.Pair {
	cmd := treeutils.GenerateAction("toggle", "enable or disable an alert",
		"Toggle the state of an alert. You may provide --enable or --disable to ensure the alert is in the respective state.",
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
				return errors.New(phrases.Exactly1ArgRequired("alert ID"))
			}

			id := c.Flags().Arg(0)
			alert, err := connection.Client.GetAlert(id)
			if err != nil {
				return err
			}

			enable, err := c.Flags().GetBool("enable")
			if err != nil {
				clilog.GetFlag(err)
			}
			disable, err := c.Flags().GetBool("disable")
			if err != nil {
				clilog.GetFlag(err)
			}

			success, err := setAlertState(alert, enable, disable)
			if err != nil {
				return err
			}
			fmt.Fprint(c.OutOrStdout(), success)
			return nil
		},
	)
	cmd.Flags().Bool("enable", false, "enable the alert. Does nothing if the alert is already enabled. Mutually exclusive with --disable")
	cmd.Flags().Bool("disable", false, "disable the alert. Does nothing if the alert is already disabled. Mutually exclusive with --enable")

	m := &toggleModel{}
	m.Reset()

	return action.NewPair(cmd, m)
}

// Set the state of the given alert according to enable and disable.
func setAlertState(alert types.Alert, enable, disable bool) (success string, _ error) {
	alert.Disabled = !alert.Disabled
	if enable && disable {
		clilog.Writer.Warn("both enable and disable were set, failing out...")
		return "", clilog.ErrInternal{}
	}
	if enable {
		alert.Disabled = false
	} else if disable {
		alert.Disabled = true
	}
	if _, err := connection.Client.UpdateAlert(alert); err != nil {
		return "", err
	}
	state := "enabled"
	if alert.Disabled {
		state = "disabled"
	}
	return fmt.Sprintf("alert '%s' (ID: %s) %s\n", alert.Name, alert.ID, state), nil
}

//#region interactive

type alertItem struct {
	id       string
	name     string
	desc     string
	disabled bool
}

func (i alertItem) FilterValue() string { return i.name + i.desc }
func (i alertItem) Title() string       { return i.name }
func (i alertItem) Description() string {
	state := "enabled"
	if i.disabled {
		state = "disabled"
	}
	return fmt.Sprintf("(%s) %s", state, i.desc)
}

// toggleModel presents a list of alerts and toggles the selected one after confirmation.
type toggleModel struct {
	selecting bool

	aList   list.Model
	confirm confirmation.Model

	done bool
}

func (c *toggleModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	if c.done {
		return nil
	}

	if c.selecting {
		if hotkeys.Match(msg, hotkeys.Invoke, hotkeys.Select) {
			c.selecting = false
		} else {
			c.aList, cmd = c.aList.Update(msg)
		}
		return cmd
	}

	// confirmation mode
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

	// submit
	item, ok := c.aList.SelectedItem().(*alertItem)
	if !ok {
		return tea.Println(clilog.TypeAssert(c.aList.SelectedItem(), &alertItem{}))
	}
	alert, err := connection.Client.GetAlert(item.id)
	if err != nil {
		c.done = true
		return tea.Printf("failed to get alert '%s': %v", item.name, err)
	}
	success, err := setAlertState(alert, false, false)
	c.done = true
	if err != nil {
		return tea.Println(err)
	}
	return tea.Println(success)
}

func (c *toggleModel) View() string {
	if c.selecting {
		return c.aList.View()
	}
	return c.confirm.View()
}

func (c *toggleModel) Done() bool {
	return c.done
}

func (c *toggleModel) Reset() error {
	c.selecting = true
	c.aList = list.Model{}
	c.confirm = confirmation.Model{}
	c.done = false
	return nil
}

func (c *toggleModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {
	alerts, err := connection.Client.ListAlerts(nil)
	if err != nil {
		clilog.Writer.Error("failed to get the list of alerts", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to get the list of alerts")
	}
	slices.SortStableFunc(alerts.Results, func(a, b types.Alert) int {
		return strings.Compare(a.Name, b.Name)
	})
	itms := make([]list.Item, 0, len(alerts.Results))
	for _, a := range alerts.Results {
		itms = append(itms, &alertItem{
			id:       a.ID,
			name:     a.Name,
			desc:     a.Description,
			disabled: a.Disabled,
		})
	}
	itms = slices.Clip(itms)
	if len(itms) == 0 {
		return "there are no alerts", nil, nil
	}
	c.aList = stylesheet.NewList(itms, width, height, "alert", "alerts")
	c.confirm.Init([]string{"alert selection"}, uint(width), uint(height))
	return "", nil, nil
}

//#endregion interactive
