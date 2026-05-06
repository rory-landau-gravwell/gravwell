// Package groups introduces actions to managing groups.
//
// Only available to admins.
package groups

import (
	"fmt"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldcreate"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldedit"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewNav() *cobra.Command {
	const (
		use   string = "groups"
		short string = "manage groups"
		long  string = "View and edit groups"
	)

	return treeutils.GenerateNav(use, short, long, []string{"group"},
		nil,
		[]action.Pair{
			list(),
			create(),
			delete(),
			edit(),
			listUsers(),
			addUser(),
			removeUser(),
		})
}

// lists all groups the current user is able to see
func list() action.Pair {
	return scaffoldlist.NewListAction("list groups", "Retrieves a list of groups available on the system",
		types.Group{},
		func(fs *pflag.FlagSet) ([]types.Group, error) {
			resp, err := connection.Client.ListGroups(nil)
			return resp.Results, err
		},
		nil,
		scaffoldlist.Options{})
}

func create() action.Pair {
	return scaffoldcreate.NewCreateAction("group",
		map[string]scaffoldcreate.Field{
			"name": scaffoldcreate.FieldName("group"),
			"desc": scaffoldcreate.FieldDescription("group"),
		},

		func(fields map[string]scaffoldcreate.Field, fs *pflag.FlagSet) (id any, invalid string, err error) {
			result, err := connection.Client.CreateGroup(types.Group{
				Name:        fields["name"].Provider.Get(),
				Description: fields["desc"].Provider.Get(),
			})
			return result.Name, "", err
		}, scaffoldcreate.Options{})
}

func delete() action.Pair {
	return scaffolddelete.NewDeleteAction("group", "groups",
		func(dryrun bool, id int32) error {
			if dryrun {
				_, err := connection.Client.GetGroup(id)
				return err
			}
			return connection.Client.DeleteGroup(id)
		},
		func() ([]scaffolddelete.Item[int32], error) {
			resp, err := connection.Client.ListGroups(nil)
			if err != nil {
				return nil, err
			}
			var items = make([]scaffolddelete.Item[int32], len(resp.Results))
			for i, g := range resp.Results {
				items[i] = scaffolddelete.NewItem(g.Name, g.Description, g.ID)
			}
			return items, nil
		})
}

func edit() action.Pair {
	cfg := scaffoldedit.Config{
		"name":        scaffoldedit.FieldName("group"),
		"description": scaffoldedit.FieldDescription("group"),
	}
	funcs := scaffoldedit.SubroutineSet[int32, types.Group]{
		SelectSub: func(id int32) (types.Group, error) {
			gwcbac, err := connection.Client.GetGroup(id)
			if err != nil {
				return types.Group{}, err
			}
			return gwcbac.Group, nil
		},
		FetchSub: func() ([]types.Group, error) {
			resp, err := connection.Client.ListGroups(nil)
			return resp.Results, err
		},
		GetFieldSub: func(item types.Group, fieldKey string) (string, error) {
			switch fieldKey {
			case "name":
				return item.Name, nil
			case "description":
				return item.Description, nil
			}
			return "", fmt.Errorf("unknown field key: %v", fieldKey)
		},
		SetFieldSub: func(item *types.Group, fieldKey, val string) (string, error) {
			switch fieldKey {
			case "name":
				item.Name = val
			case "description":
				item.Description = val
			default:
				return "", fmt.Errorf("unknown field key: %v", fieldKey)
			}
			return "", nil
		},
		GetTitleSub:       func(item types.Group) string { return item.Name },
		GetDescriptionSub: func(item types.Group) string { return item.Description },
		UpdateSub: func(data *types.Group) (string, error) {
			return data.Name, connection.Client.UpdateGroup(*data)
		},
	}
	return scaffoldedit.NewEditAction("group", "groups", cfg, funcs)
}

var listUsersGID int32

func listUsers() action.Pair {
	return scaffoldlist.NewListAction("list users in a group", "Display the users that are members of a given group.",
		types.User{},
		func(fs *pflag.FlagSet) ([]types.User, error) {
			return connection.Client.GetGroupUsers(listUsersGID)
		},
		nil,
		scaffoldlist.Options{
			CommonOptions:  scaffold.CommonOptions{Use: "list-users"},
			DefaultColumns: []string{"ID", "Username", "Name", "Email", "Admin"},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("group ID"), nil
				}
				gid, err := strconv.ParseInt(fs.Arg(0), 10, 32)
				if err != nil {
					return fs.Arg(0) + " is not a valid group ID", nil
				}
				listUsersGID = int32(gid)
				return "", nil
			},
		})
}

func addUser() action.Pair {
	return scaffold.NewBasicAction("add-user", "add a user to a group", "Add a user to a group by providing the user ID and group ID.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			uid, err := fs.GetInt32("uid")
			if err != nil {
				clilog.LogFlagFailedGet("uid", err)
				return "failed to get uid flag", nil
			}
			gid, err := fs.GetInt32("gid")
			if err != nil {
				clilog.LogFlagFailedGet("gid", err)
				return "failed to get gid flag", nil
			}
			if err := connection.Client.AddUserToGroup(uid, gid); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully added user %d to group %d", uid, gid), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.Int32("uid", 0, "user ID")
					fs.Int32("gid", 0, "group ID")
					return fs
				},
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				uid, err := fs.GetInt32("uid")
				if err != nil {
					clilog.LogFlagFailedGet("uid", err)
				}
				gid, err := fs.GetInt32("gid")
				if err != nil {
					clilog.LogFlagFailedGet("gid", err)
				}
				if uid == 0 || gid == 0 {
					return "both --uid and --gid must be set and nonzero", nil
				}
				return "", nil
			},
		})
}

func removeUser() action.Pair {
	return scaffold.NewBasicAction("remove-user", "remove a user from a group", "Remove a user from a group by providing the user ID and group ID.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			uid, err := fs.GetInt32("uid")
			if err != nil {
				clilog.LogFlagFailedGet("uid", err)
				return "failed to get uid flag", nil
			}
			gid, err := fs.GetInt32("gid")
			if err != nil {
				clilog.LogFlagFailedGet("gid", err)
				return "failed to get gid flag", nil
			}
			if err := connection.Client.DeleteUserFromGroup(uid, gid); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully removed user %d from group %d", uid, gid), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.Int32("uid", 0, "user ID")
					fs.Int32("gid", 0, "group ID")
					return fs
				},
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				uid, err := fs.GetInt32("uid")
				if err != nil {
					clilog.LogFlagFailedGet("uid", err)
				}
				gid, err := fs.GetInt32("gid")
				if err != nil {
					clilog.LogFlagFailedGet("gid", err)
				}
				if uid == 0 || gid == 0 {
					return "both --uid and --gid must be set and nonzero", nil
				}
				return "", nil
			},
		})
}

// TODO this probably requires a custom action to ensure it is as usable as possible
/*func addUser_old() action.Pair {
	return scaffold.NewBasicAction("adduser", "add a user to a group", "Add a user to a group",
		func(cmd *cobra.Command, fs *pflag.FlagSet) (string, tea.Cmd) {
			var (
				uid, gid int32
				err      error
			)

			uid, err = fs.GetInt32("uid")
			if err != nil {
				clilog.LogFlagFailedGet("uid", err)
			}
			gid, err = fs.GetInt32("gid")
			if err != nil {
				clilog.LogFlagFailedGet("gid", err)
			}
			if err := connection.Client.AddUserToGroup(uid, gid); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("Successfully added user %d to group %d", uid, gid), nil
		},
		scaffold.BasicOptions{
			AddtlFlagFunc: func() pflag.FlagSet {
				fs := pflag.FlagSet{}
				fs.Int32("uid", 0, "id of the user to add")
				fs.Int32("gid", 0, "id of the group to add the user to")
				return fs
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if !connection.CurrentUser().Admin {
					return "you must be an admin to use this function", nil
				}
				uid, err := fs.GetInt32("uid")
				if err != nil {
					clilog.LogFlagFailedGet("uid", err)
				}
				gid, err := fs.GetInt32("gid")
				if err != nil {
					clilog.LogFlagFailedGet("gid", err)
				}
				if gid == 0 || uid == 0 {
					return "you must specify both --uid and --gid", nil
				}
				return "", nil
			},
		})
}*/
