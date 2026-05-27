/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

/*
Package queries provides a nav that contains utilities related to interacting with existing or former queries.
All query creation is done at the top-level query action.
*/
package queries

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/tree/queries/attach"
	"github.com/gravwell/gravwell/v4/gwcli/tree/queries/saved"
	"github.com/gravwell/gravwell/v4/gwcli/tree/queries/scheduled"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	use   string = "queries"
	short string = "manage existing and past queries"
	long  string = "Queries contains utilities for managing auxiliary query actions." +
		"Query creation is handled by the top-level `query` action."
)

var aliases []string = []string{"searches"}

func NewQueriesNav() *cobra.Command {
	return treeutils.GenerateNav(use, short, long, aliases,
		[]*cobra.Command{scheduled.NewScheduledNav(), saved.NewSavedNav()},
		[]action.Pair{
			past(),
			attach.NewAttachAction(),
			listActive(),
			deleteQuery(),
			stopQuery(),
			saveQuery(),
			backgroundQuery(),
			queryInfo(),
			setQueryGroup(),
		})
}

// #region past queries

func past() action.Pair {
	const (
		pastUse string = "past"
		short   string = "display search history"
		long    string = "display past searches made by your user"
	)

	return scaffoldlist.NewListAction(
		short, long,
		types.SearchHistoryEntry{},
		func(fs *pflag.FlagSet) ([]types.SearchHistoryEntry, error) {
			opts := &types.QueryOptions{}
			if count, err := fs.GetInt("count"); err != nil {
				clilog.GetFlag(err)
			} else if count > 0 {
				opts.Limit = count
			}

			resp, err := connection.Client.ListSearchHistory(opts)
			if err != nil {
				// check for explicit no records error
				if strings.Contains(err.Error(), "No record") {
					clilog.Writer.Debugf("no records error: %v", err)
					return nil, nil
				}
				return nil, err
			}
			return resp.Results, nil
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{Use: pastUse, AddtlFlags: flags},
			DefaultColumns: []string{
				"CommonFields.ID",
				"EffectiveQuery",
				"Launched",
			},
		})
}

func flags() *pflag.FlagSet {
	addtlFlags := pflag.FlagSet{}
	addtlFlags.Int("count", 0, "the number of past searches to display.\n"+
		"If negative or 0, fetches entire history")
	return &addtlFlags
}

//#endregion past queries

// listActive provides an interface for viewing active searches.
func listActive() action.Pair {
	return scaffoldlist.NewListAction("list active searches", "List currently active/recent searches on the system.",
		types.SearchCtrlStatus{},
		func(fs *pflag.FlagSet) ([]types.SearchCtrlStatus, error) {
			all, err := fs.GetBool(ft.GetAll.Name())
			if err != nil {
				clilog.GetFlag(err)
			}
			if all {
				return connection.Client.ListAllSearchStatuses()
			}
			return connection.Client.ListSearchStatuses()
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{
				Use:     "active",
				Aliases: []string{"list-searches"},
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					ft.GetAll.Register(fs, true, "active queries")
					return fs
				},
			},
			DefaultColumns: []string{"ID", "EffectiveQuery", "State"},
		})
}

func deleteQuery() action.Pair {
	return scaffold.NewBasicAction("delete", "delete an active search",
		"Delete an active search by its ID.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			sid := fs.Arg(0)
			if err := connection.Client.DeleteSearch(sid); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully deleted search %s", sid), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("search ID"), nil
				}
				return "", nil
			},
		})
}

// stopQuery asks the backend to stop (terminate) an active search.
func stopQuery() action.Pair {
return scaffold.NewBasicAction("stop", "stop an active search",
"Request the backend to stop a running search identified by its ID.\n"+
"The search results remain accessible after stopping.",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
sid := fs.Arg(0)
if err := connection.Client.StopSearch(sid); err != nil {
return err.Error(), nil
}
return fmt.Sprintf("successfully stopped search %s", sid), nil
},
scaffold.BasicOptions{
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() != 1 {
return phrases.Exactly1ArgRequired("search ID"), nil
}
return "", nil
},
})
}

// saveQuery marks an active search as saved so it persists beyond its normal expiry.
func saveQuery() action.Pair {
return scaffold.NewBasicAction("save", "save an active search so it persists",
"Save an active search by its ID so the results are retained.\n"+
"Optionally provide a name via --name.",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
sid := fs.Arg(0)
name, err := fs.GetString("name")
if err != nil {
clilog.GetFlag(err)
}
if name != "" {
patch := types.SaveSearchPatch{Name: name}
if err := connection.Client.SaveSearch(sid, patch); err != nil {
return err.Error(), nil
}
} else {
if err := connection.Client.SaveSearch(sid); err != nil {
return err.Error(), nil
}
}
return fmt.Sprintf("successfully saved search %s", sid), nil
},
scaffold.BasicOptions{
CommonOptions: scaffold.CommonOptions{
AddtlFlags: func() *pflag.FlagSet {
fs := &pflag.FlagSet{}
fs.String("name", "", "optional name to give the saved search")
return fs
},
},
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() != 1 {
return phrases.Exactly1ArgRequired("search ID"), nil
}
return "", nil
},
})
}

// backgroundQuery moves an active search to background so it keeps running after the client disconnects.
func backgroundQuery() action.Pair {
return scaffold.NewBasicAction("background", "move an active search to the background",
"Mark an active search as backgrounded so it continues running after you disconnect.\n"+
"Background searches can be re-attached later with `queries attach`.",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
sid := fs.Arg(0)
if err := connection.Client.BackgroundSearch(sid); err != nil {
return err.Error(), nil
}
return fmt.Sprintf("successfully backgrounded search %s", sid), nil
},
scaffold.BasicOptions{
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() != 1 {
return phrases.Exactly1ArgRequired("search ID"), nil
}
return "", nil
},
})
}

// queryInfo displays detailed information about an active search.
func queryInfo() action.Pair {
return scaffold.NewBasicAction("info", "display detailed info for an active search",
"Display detailed information about an active search, including its query, time range, and state.",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
sid := fs.Arg(0)
si, err := connection.Client.SearchInfo(sid)
if err != nil {
return err.Error(), nil
}
return fmt.Sprintf(
"ID: %s\nState: %s\nQuery: %s\nEffective: %s\nStarted: %s\nRange: %s - %s\nBackground: %v",
si.ID,
si.Error,
si.UserQuery,
si.EffectiveQuery,
si.Started.Local().Format("2006-01-02 15:04:05"),
si.StartRange.Local().Format("2006-01-02 15:04:05"),
si.EndRange.Local().Format("2006-01-02 15:04:05"),
si.Background,
), nil
},
scaffold.BasicOptions{
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() != 1 {
return phrases.Exactly1ArgRequired("search ID"), nil
}
return "", nil
},
})
}

// setQueryGroup assigns a group to an active search so group members can view the results.
func setQueryGroup() action.Pair {
return scaffold.NewBasicAction("set-group", "assign a group to an active search",
"Assign one or more groups to an active search so members of those groups can access the results.\n\n"+
"Pass the search ID followed by one or more group IDs.\n"+
"Example: queries set-group <search-ID> <group-ID>",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
sid := fs.Arg(0)
// remaining args are group IDs
rawGIDs := fs.Args()[1:]
gids := make([]int32, 0, len(rawGIDs))
for _, s := range rawGIDs {
var gid int32
if _, err := fmt.Sscan(s, &gid); err != nil {
return fmt.Sprintf("'%s' is not a valid group ID", s), nil
}
gids = append(gids, gid)
}
if len(gids) == 1 {
if err := connection.Client.SetGroup(sid, gids[0]); err != nil {
return err.Error(), nil
}
} else {
if err := connection.Client.SetGroups(sid, gids); err != nil {
return err.Error(), nil
}
}
return fmt.Sprintf("successfully set groups for search %s", sid), nil
},
scaffold.BasicOptions{
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() < 2 {
return "search ID and at least one group ID are required", nil
}
return "", nil
},
})
}
