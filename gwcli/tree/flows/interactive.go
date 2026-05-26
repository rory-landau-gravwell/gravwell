package flows

import (
	"fmt"

	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldselect"
	"github.com/spf13/pflag"
)

// This file implements interactive versions of flow actions: cancel, toggle-backfill, and clear-results.

func listFlowItems() ([]multiselectlist.SelectableItem[string], error) {
	baseList, err := connection.Client.ListFlows(nil)
	if err != nil {
		return nil, err
	}
	itms := make([]multiselectlist.SelectableItem[string], len(baseList.Results))
	for i, f := range baseList.Results {
		itms[i] = &listitem.Generic{
			ID_:          f.ID,
			Name:         f.Name,
			SecondLine:   fmt.Sprintf("[%s] %s", f.Schedule, f.Description),
			ShowDisabled: true,
			Enabled:      !f.Disabled,
		}
	}
	return itms, nil
}

func backfillFlags() *pflag.FlagSet {
	fs := &pflag.FlagSet{}
	fs.Bool("enable", false, "enable backfill")
	fs.Bool("disable", false, "disable backfill")
	return fs
}

func getBackfillFlags(fs *pflag.FlagSet) (enable, disable bool, err error) {
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

func cancel() action.Pair {
	return scaffoldselect.NewSelectAction("cancel running flows",
		"Cancel one or several currently-executing flows by ID or GUID.",
		"flow", "flows",
		func(_ *pflag.FlagSet) ([]multiselectlist.SelectableItem[string], error) {
			return listFlowItems()
		},
		func(id string, _ *pflag.FlagSet) (success string, err error) {
			if err := connection.Client.CancelFlow(id); err != nil {
				return "", err
			}
			return fmt.Sprintf("successfully cancelled flow %s", id), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{Use: "cancel"},
		})
}

func backfillToggle() action.Pair {
	return scaffoldselect.NewSelectAction("toggle flow backfill",
		"Toggle backfill for one or several flows. Use --enable or --disable to set explicitly.\n"+
			"Backfill causes the automation to run for missed time periods.",
		"flow", "flows",
		func(fs *pflag.FlagSet) ([]multiselectlist.SelectableItem[string], error) {
			enable, disable, err := getBackfillFlags(fs)
			if err != nil {
				return nil, err
			}

			baseList, err := connection.Client.ListFlows(nil)
			if err != nil {
				return nil, err
			}
			itms := make([]multiselectlist.SelectableItem[string], 0, len(baseList.Results))
			for _, f := range baseList.Results {
				if enable && f.BackfillEnabled {
					continue
				} else if disable && !f.BackfillEnabled {
					continue
				}
				itms = append(itms, &listitem.Generic{
					ID_:          f.ID,
					Name:         f.Name,
					SecondLine:   fmt.Sprintf("[%s] %s", f.Schedule, f.Description),
					ShowDisabled: true,
					Enabled:      !f.Disabled,
				})
			}
			return itms, nil
		},
		func(id string, fs *pflag.FlagSet) (success string, err error) {
			enable, disable, err := getBackfillFlags(fs)
			if err != nil {
				return "", err
			}

			flow, err := connection.Client.GetFlow(id)
			if err != nil {
				return "", err
			}
			flow.BackfillEnabled = !flow.BackfillEnabled
			if enable {
				flow.BackfillEnabled = true
			} else if disable {
				flow.BackfillEnabled = false
			}

			if err := connection.Client.UpdateFlow(flow); err != nil {
				return "", err
			}
			state := "enabled"
			if !flow.BackfillEnabled {
				state = "disabled"
			}
			return fmt.Sprintf("flow '%s' backfill %s", id, state), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{
				Use:        "toggle-backfill",
				AddtlFlags: backfillFlags,
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				_, _, err = getBackfillFlags(fs)
				return "", err
			},
		})
}

func clearResults() action.Pair {
	return scaffoldselect.NewSelectAction("clear results for flows",
		"Clear the execution results (including errors and state) for one or several flows.",
		"flow", "flows",
		func(_ *pflag.FlagSet) ([]multiselectlist.SelectableItem[string], error) {
			return listFlowItems()
		},
		func(id string, _ *pflag.FlagSet) (success string, err error) {
			if err := connection.Client.ClearFlowResults(id); err != nil {
				return "", err
			}
			return fmt.Sprintf("successfully cleared results for flow %s", id), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{Use: "clear-results"},
		})
}
