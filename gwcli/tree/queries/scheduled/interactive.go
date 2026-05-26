package scheduled

import (
	"errors"
	"fmt"
	"strconv"

	blist "github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldselect"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/ingest/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// This file implements interactive versions of scheduled search actions.

func scheduledSecondLine(schedule, query, desc string) string {
	line := fmt.Sprintf("[%s] %s", schedule, query)
	if desc != "" {
		line += " - " + desc
	}
	return line
}

func scheduledItems() ([]*listitem.Generic, error) {
	l, err := connection.Client.ListScheduledSearches(nil)
	if err != nil {
		return nil, err
	}
	itms := make([]*listitem.Generic, len(l.Results))
	for i, ss := range l.Results {
		itms[i] = &listitem.Generic{
			ID_:          ss.ID,
			Name:         ss.Name,
			SecondLine:   scheduledSecondLine(ss.Schedule, ss.SearchString, ss.Description),
			ShowDisabled: true,
			Enabled:      !ss.Disabled,
		}
	}
	return itms, nil
}

func scheduledSelectableItems(_ *pflag.FlagSet) ([]multiselectlist.SelectableItem[string], error) {
	base, err := scheduledItems()
	if err != nil {
		return nil, err
	}
	itms := make([]multiselectlist.SelectableItem[string], len(base))
	for i := range base {
		itms[i] = base[i]
	}
	return itms, nil
}

func scheduledListItems() ([]blist.Item, error) {
	base, err := scheduledItems()
	if err != nil {
		return nil, err
	}
	itms := make([]blist.Item, len(base))
	for i := range base {
		itms[i] = base[i]
	}
	return itms, nil
}

func scheduledBackfillFlags() *pflag.FlagSet {
	fs := &pflag.FlagSet{}
	fs.Bool("enable", false, "enable backfill")
	fs.Bool("disable", false, "disable backfill")
	return fs
}

func getScheduledBackfillFlags(fs *pflag.FlagSet) (enable, disable bool, err error) {
	enable, err = fs.GetBool("enable")
	if err != nil {
		clilog.GetFlag(err)
		return
	}
	disable, err = fs.GetBool("disable")
	if err != nil {
		clilog.GetFlag(err)
		return
	}
	if enable && disable {
		return false, false, ft.ErrMutuallyExclusive("enable", "disable")
	}
	return
}

func cancelAction() action.Pair {
	return scaffoldselect.NewSelectAction("cancel running scheduled searches",
		"Cancel one or several currently-executing scheduled searches by ID.",
		"scheduled search", "scheduled searches",
		scheduledSelectableItems,
		func(id string, _ *pflag.FlagSet) (success string, _ error) {
			if err := connection.Client.CancelScheduledSearch(id); err != nil {
				return "", err
			}
			return fmt.Sprintf("successfully cancelled scheduled search %s", id), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{Use: "cancel"},
		})
}

func backfillToggle() action.Pair {
	return scaffoldselect.NewSelectAction("toggle scheduled search backfill",
		"Toggle backfill for one or several scheduled searches. Use --enable or --disable to set explicitly.",
		"scheduled search", "scheduled searches",
		func(fs *pflag.FlagSet) ([]multiselectlist.SelectableItem[string], error) {
			enable, disable, err := getScheduledBackfillFlags(fs)
			if err != nil {
				return nil, err
			}

			l, err := connection.Client.ListScheduledSearches(nil)
			if err != nil {
				return nil, err
			}
			itms := make([]multiselectlist.SelectableItem[string], 0, len(l.Results))
			for _, ss := range l.Results {
				if enable && ss.BackfillEnabled {
					continue
				} else if disable && !ss.BackfillEnabled {
					continue
				}
				itms = append(itms, &listitem.Generic{
					ID_:          ss.ID,
					Name:         ss.Name,
					SecondLine:   scheduledSecondLine(ss.Schedule, ss.SearchString, ss.Description),
					ShowDisabled: true,
					Enabled:      !ss.Disabled,
				})
			}
			return itms, nil
		},
		func(id string, fs *pflag.FlagSet) (success string, _ error) {
			enable, disable, err := getScheduledBackfillFlags(fs)
			if err != nil {
				return "", err
			}

			ss, err := connection.Client.GetScheduledSearch(id)
			if err != nil {
				return "", err
			}
			ss.BackfillEnabled = !ss.BackfillEnabled
			if enable {
				ss.BackfillEnabled = true
			} else if disable {
				ss.BackfillEnabled = false
			}

			if err := connection.Client.UpdateScheduledSearch(ss); err != nil {
				return "", err
			}
			state := "enabled"
			if !ss.BackfillEnabled {
				state = "disabled"
			}
			return fmt.Sprintf("scheduled search '%s' backfill %s", id, state), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{
				Use:        "toggle-backfill",
				AddtlFlags: scheduledBackfillFlags,
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				_, _, err = getScheduledBackfillFlags(fs)
				return "", err
			},
		})
}

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
		treeutils.GenerateActionOptions{
			Usage: ft.Mandatory("scheduled_search_ID") + " " + ft.Mandatory("offset_seconds"),
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
			ss, err := listitem.GetGeneric(&c.ssList)
			if err != nil {
				c.stage = soStgDone
				return tea.Println(err)
			}
			c.selectedID = ss.ID()
			c.selectedName = ss.Name
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
				return nil
			}
			if offset > 0 {
				return nil
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
	itms, err := scheduledListItems()
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

func clearResults() action.Pair {
	return scaffoldselect.NewSelectAction("clear results for scheduled searches",
		"Clear the execution results (including errors and state) for one or several scheduled searches.",
		"scheduled search", "scheduled searches",
		scheduledSelectableItems,
		func(id string, _ *pflag.FlagSet) (success string, _ error) {
			if err := connection.Client.ClearScheduledSearchResults(id); err != nil {
				return "", err
			}
			return fmt.Sprintf("successfully cleared results for scheduled search %s", id), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{Use: "clear-results"},
		})
}
