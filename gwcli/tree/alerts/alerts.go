/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package alerts provides actions for interacting with your alerts.
package alerts

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	alertscreate "github.com/gravwell/gravwell/v4/gwcli/tree/alerts/create"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldselect"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewAlertsNav() *cobra.Command {
	const (
		use   string = "alerts"
		short string = "manage alerts"
		long  string = "Alerts allow you to tie sources of intelligence (such as periodic scheduled searches) to actions (such as a flow that files a ticket)." +
			" This can make it much simpler to take automatic action when something of interest occurs."
	)
	return treeutils.GenerateNav(use, short, long, []string{"alert"}, []*cobra.Command{},
		[]action.Pair{
			alertsList(),
			toggle(),
			delete(),
			alertscreate.Action(),
			setDispatchers(),
			setSave(),
		})
}

// set and unset by list's ValidateArgs
var (
	listConsumerID   string
	listDispatcherID string
)

func alertsList() action.Pair {
	const (
		short string = "list your alerts"
		long  string = "lists alerts associated to your user. If admin mode is active, returns all alerts for all users."
	)

	return scaffoldlist.NewListAction(short, long, types.Alert{},
		func(fs *pflag.FlagSet) ([]types.Alert, error) {
			if listConsumerID != "" {
				resp, err := connection.Client.ListAlerts(&types.QueryOptions{
					Filters: []types.Filter{
						{
							Key:       "Consumers.ID",
							Operation: "=",
							Values:    []any{listConsumerID},
						},
					},
				})
				return resp.Results, err

			} else if listDispatcherID != "" {
				resp, err := connection.Client.ListAlerts(&types.QueryOptions{
					Filters: []types.Filter{
						{
							Key:       "Dispatchers.ID",
							Operation: "=",
							Values:    []any{listDispatcherID},
						},
					},
				})
				return resp.Results, err
			}

			resp, err := connection.Client.ListAlerts(nil)
			return resp.Results, err
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.String("consumer", "", "Filter to alerts that refer to this consumer. Should be the ID of the a flow. Used to answer: which alerts will launch this specific flow")
					fs.String("dispatcher", "", "Filter to alerts that refer to this dispatcher. Should be the ID of the a scheduled search. Used to answer: which alerts will be invoked by this specific scheduled search")
					return fs
				},
			},
			DefaultColumns: []string{
				"CommonFields.ID",
				"CommonFields.Name",
				"CommonFields.Description",
				"Disabled",
				"Consumers",
				"Dispatchers",
				"TargetTag",
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, _ error) {
				if listConsumerID, invalid = validateListID("consumer", fs); invalid != "" {
					return invalid, nil
				}
				if listDispatcherID, invalid = validateListID("dispatcher", fs); invalid != "" {
					return invalid, nil
				}

				if listConsumerID != "" && listDispatcherID != "" {
					return ft.ErrMutuallyExclusive("consumer", "dispatcher").Error(), nil
				}
				return "", nil
			},
		})
}

// helper function for list's ValidateArgs.
func validateListID(flagName string, fs *pflag.FlagSet) (id string, invalid string) {
	s, err := fs.GetString(flagName)
	if err != nil {
		clilog.GetFlag(err)
	}
	return s, ""
}

func delete() action.Pair {
	return scaffolddelete.NewDeleteAction("alert", "alerts",
		func(dryrun bool, id string) error {
			if dryrun {
				_, err := connection.Client.GetAlert(id)
				return err
			}
			return connection.Client.DeleteAlert(id)
		},
		func() ([]multiselectlist.SelectableItem[string], error) {
			alerts, err := connection.Client.ListAlerts(nil)
			if err != nil {
				return nil, err
			}
			// sort on name
			slices.SortStableFunc(alerts.Results,
				func(a, b types.Alert) int {
					return strings.Compare(a.Name, b.Name)
				})
			var items = make([]multiselectlist.SelectableItem[string], len(alerts.Results))
			for i, a := range alerts.Results {
				items[i] = &listitem.Generic{
					Selected_:  false,
					ID_:        a.ID,
					Name:       a.Name,
					SecondLine: a.Description,

					ShowDisabled: true,
					Enabled:      !a.Disabled,
				}
			}
			return items, nil
		}, scaffolddelete.Options{})
}

var toggleEnable, toggleDisable bool

func toggle() action.Pair {
	return scaffoldselect.NewSelectAction("enable or disable an alert",
		"Toggle the enabled state of an alert. Optionally use --enable or --disable to set explicitly.",
		"alert", "alerts",
		func(addtlFlags *pflag.FlagSet) ([]multiselectlist.SelectableItem[string], error) {
			lr, err := connection.Client.ListAlerts(nil)
			if err != nil {
				return nil, err
			}

			// if a flag was specified, hide alerts already in the state of the flag
			var enable, disable bool
			if enable, err = addtlFlags.GetBool("enable"); err != nil {
				clilog.GetFlag(err)
			}
			if disable, err = addtlFlags.GetBool("disable"); err != nil {
				clilog.GetFlag(err)
			}

			items := make([]multiselectlist.SelectableItem[string], 0, len(lr.Results))
			for _, alert := range lr.Results {
				if enable && !alert.Disabled {
					continue
				} else if disable && alert.Disabled {
					continue
				}
				items = append(items, &listitem.Generic{
					Selected_:    false,
					ID_:          alert.ID,
					Name:         alert.Name,
					SecondLine:   alert.Description,
					ShowDisabled: !enable && !disable, // only if it wasn't explicit
					Enabled:      !alert.Disabled,
				})
			}
			return items, nil
		},
		func(ID string, addtlFlags *pflag.FlagSet) (success string, _ error) {
			// read flags directly so we get the correct values regardless of call path
			enable, err := addtlFlags.GetBool("enable")
			if err != nil {
				clilog.GetFlag(err)
			}
			disable, err := addtlFlags.GetBool("disable")
			if err != nil {
				clilog.GetFlag(err)
			}

			alert, err := connection.Client.GetAlert(ID)
			if err != nil {
				return "", err
			}
			alert.Disabled = !alert.Disabled
			if enable {
				alert.Disabled = false
			} else if disable {
				alert.Disabled = true
			}
			if _, err := connection.Client.UpdateAlert(alert); err != nil {
				return "", err
			}
			verb := "enabled"
			if alert.Disabled {
				verb = "disabled"
			}
			return fmt.Sprintf("Alert \"%s\" %s", alert.Name, verb), nil
		},
		scaffoldselect.Options{
			CommonOptions: scaffold.CommonOptions{
				Use: "toggle",
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.Bool("enable", false, "explicitly enable selected alerts. No-op on alerts already enabled. Mutually exclusive with --disable")
					fs.Bool("disable", false, "explicitly disable selected alerts. No-op on alerts already disabled. Mutually exclusive with --enable")
					return fs
				},
			},
			NoItemsError: func(fs *pflag.FlagSet) string {
				enable, _ := fs.GetBool("enable")
				disable, _ := fs.GetBool("disable")
				if enable {
					return "You have no alerts that can be enabled."
				} else if disable {
					return "You have no alerts that can be disabled."
				}
				return "You have no alerts that can be toggled."
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				// ensure !(enable && disable)
				en, err := fs.GetBool("enable")
				if err != nil {
					clilog.GetFlag(err)
				}
				dis, err := fs.GetBool("disable")
				if err != nil {
					clilog.GetFlag(err)
				}
				if en && dis {
					return ft.ErrMutuallyExclusive("enable", "disable").Error(), nil
				}
				return "", nil
			},
		})
}

//#region set-dispatchers

// setDispatchers lets the user add or remove scheduled search dispatchers from an alert.
func setDispatchers() action.Pair {
return scaffold.NewBasicAction("set-dispatchers",
"set the scheduled search dispatchers for an alert",
"Set which scheduled searches act as dispatchers (triggers) for an alert.\n"+
"Pass one or more scheduled-search IDs as arguments.\n"+
"Use --add to add dispatchers, --remove to remove them, or neither to replace the entire list.\n\n"+
"Example: alerts set-dispatchers --add <alert-ID> <ss-ID1> <ss-ID2>",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
alertID, err := fs.GetString("alert")
if err != nil {
return err.Error(), nil
}
add, err := fs.GetBool("add")
if err != nil {
return err.Error(), nil
}
remove, err := fs.GetBool("remove")
if err != nil {
return err.Error(), nil
}

a, err := connection.Client.GetAlert(alertID)
if err != nil {
return err.Error(), nil
}

// build new dispatchers from positional args
newIDs := fs.Args()
switch {
case add:
// append only IDs not already present
existing := make(map[string]bool, len(a.Dispatchers))
for _, d := range a.Dispatchers {
existing[d.ID] = true
}
for _, id := range newIDs {
if !existing[id] {
a.Dispatchers = append(a.Dispatchers, types.AlertDispatcher{
ID:   id,
Type: types.ALERTDISPATCHERTYPE_SCHEDULEDSEARCH,
})
}
}
case remove:
remove := make(map[string]bool, len(newIDs))
for _, id := range newIDs {
remove[id] = true
}
filtered := a.Dispatchers[:0]
for _, d := range a.Dispatchers {
if !remove[d.ID] {
filtered = append(filtered, d)
}
}
a.Dispatchers = filtered
default:
// replace entirely
a.Dispatchers = make([]types.AlertDispatcher, 0, len(newIDs))
for _, id := range newIDs {
a.Dispatchers = append(a.Dispatchers, types.AlertDispatcher{
ID:   id,
Type: types.ALERTDISPATCHERTYPE_SCHEDULEDSEARCH,
})
}
}

if _, err := connection.Client.UpdateAlert(a); err != nil {
return err.Error(), nil
}
ids := make([]string, len(a.Dispatchers))
for i, d := range a.Dispatchers {
ids[i] = d.ID
}
return fmt.Sprintf("alert '%s' dispatchers set to: [%s]", a.Name, strings.Join(ids, ", ")), nil
},
scaffold.BasicOptions{
CommonOptions: scaffold.CommonOptions{
AddtlFlags: func() *pflag.FlagSet {
fs := &pflag.FlagSet{}
fs.String("alert", "", "ID of the alert to modify (required)")
fs.Bool("add", false, "add the given dispatcher IDs instead of replacing the list. Mutually exclusive with --remove")
fs.Bool("remove", false, "remove the given dispatcher IDs instead of replacing the list. Mutually exclusive with --add")
return fs
},
},
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
alertID, err := fs.GetString("alert")
if err != nil {
clilog.GetFlag(err)
return "", err
}
if strings.TrimSpace(alertID) == "" {
return "--alert (alert ID) is required", nil
}
add, err := fs.GetBool("add")
if err != nil {
clilog.GetFlag(err)
return "", err
}
remove, err := fs.GetBool("remove")
if err != nil {
clilog.GetFlag(err)
return "", err
}
if add && remove {
return ft.ErrMutuallyExclusive("add", "remove").Error(), nil
}
return "", nil
},
})
}

//#endregion set-dispatchers

//#region set-save

// setSave lets the user configure whether triggered searches should be saved and for how long.
func setSave() action.Pair {
return scaffold.NewBasicAction("set-save",
"configure save-search settings for an alert",
"Configure whether searches that trigger an alert should be automatically saved,\n"+
"and for how long (in seconds).\n\n"+
"Examples:\n"+
"  alerts set-save <alert-ID> --enable --duration 86400\n"+
"  alerts set-save <alert-ID> --disable",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
if fs.NArg() < 1 {
return "alert ID is required as first argument", nil
}
alertID := fs.Arg(0)

enable, err := fs.GetBool("enable")
if err != nil {
return err.Error(), nil
}
disable, err := fs.GetBool("disable")
if err != nil {
return err.Error(), nil
}
duration, err := fs.GetInt("duration")
if err != nil {
return err.Error(), nil
}

a, err := connection.Client.GetAlert(alertID)
if err != nil {
return err.Error(), nil
}

if enable {
a.SaveSearchEnabled = true
}
if disable {
a.SaveSearchEnabled = false
}
if fs.Changed("duration") {
a.SaveSearchDuration = int32(duration)
}

if _, err := connection.Client.UpdateAlert(a); err != nil {
return err.Error(), nil
}

state := "disabled"
if a.SaveSearchEnabled {
state = fmt.Sprintf("enabled (duration: %ds)", a.SaveSearchDuration)
}
return fmt.Sprintf("alert '%s' save-search: %s", a.Name, state), nil
},
scaffold.BasicOptions{
CommonOptions: scaffold.CommonOptions{
AddtlFlags: func() *pflag.FlagSet {
fs := &pflag.FlagSet{}
fs.Bool("enable", false, "enable save-search for this alert. Mutually exclusive with --disable")
fs.Bool("disable", false, "disable save-search for this alert. Mutually exclusive with --enable")
fs.Int("duration", 0, "how long (in seconds) triggered searches should be saved. Only meaningful when save-search is enabled")
return fs
},
},
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() < 1 {
return "alert ID is required as first argument", nil
}
enable, err := fs.GetBool("enable")
if err != nil {
clilog.GetFlag(err)
return "", err
}
disable, err := fs.GetBool("disable")
if err != nil {
clilog.GetFlag(err)
return "", err
}
if enable && disable {
return ft.ErrMutuallyExclusive("enable", "disable").Error(), nil
}
dur, err := fs.GetInt("duration")
if err != nil {
clilog.GetFlag(err)
return "", err
}
if dur < 0 {
return fmt.Sprintf("--duration must be >= 0, got %d", dur), nil
}
_ = strconv.Itoa(dur) // suppress unused import warning; already validated above
return "", nil
},
})
}

//#endregion set-save
