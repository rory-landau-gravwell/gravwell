/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package kits provides actions for interacting with kits. *jazz hands*
package kits

import (
	"encoding/json"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewKitsNav() *cobra.Command {
	const (
		use   string = "kits"
		short string = "view kits associated to this instance"
		long  string = "Kits bundle up of related items (dashboards, queries, scheduled searches," +
			" autoextractors) for easy installation."
	)
	var aliases = []string{"kit"}
	return treeutils.GenerateNav(use, short, long, aliases,
		[]*cobra.Command{},
		[]action.Pair{
			newKitsListAction(),
			deleteKit(),
			uninstall(),
			install(),
			upload(),
			pull(),
			remote(),
			get(),
			buildKit(),
		})
}

//#region list

func newKitsListAction() action.Pair {
	const short string = "list installed and staged kits"
	var long = "lists kits available to your user" +
		"(or all kits on the system, via the --" + ft.GetAll.Name() + " flag if you are an admin)"

	return scaffoldlist.NewListAction(
		short, long,
		types.IdKitState{}, func(fs *pflag.FlagSet) ([]types.IdKitState, error) {
			// if --all, use the admin version
			if all, err := fs.GetBool(ft.GetAll.Name()); err != nil {
				clilog.GetFlag(err)
			} else if all {
				return connection.Client.AdminListKits()
			}

			return connection.Client.ListKits()
		},
		nil,
		scaffoldlist.Options{CommonOptions: scaffold.CommonOptions{AddtlFlags: flags},
			DefaultColumns: []string{
				"UUID",
				"KitState.Name",
				"KitState.Description",
				"KitState.Version",
			}})
}

func flags() *pflag.FlagSet {
	addtlFlags := pflag.FlagSet{}
	ft.GetAll.Register(&addtlFlags, true, "kits")

	return &addtlFlags
}

//#endregion list

func deleteKit() action.Pair {
	return scaffolddelete.NewDeleteAction("kit", "kits",
		func(dryrun bool, id string) error {
			if dryrun {
				uid, err := uuid.Parse(id)
				if err != nil {
					return err
				}
				_, err = connection.Client.KitInfo(uid)
				return err
			}
			return connection.Client.DeleteKit(id)
		},
		func() ([]multiselectlist.SelectableItem[string], error) {
			pkgs, err := connection.Client.ListKits()
			if err != nil {
				return nil, err
			}
			var items = make([]multiselectlist.SelectableItem[string], len(pkgs))
			for i, kit := range pkgs {
				items[i] = &listitem.Generic{
					Selected_:  false,
					ID_:        kit.ID,
					Name:       kit.Name,
					SecondLine: kit.Description,
				}
			}

			return items, nil
		}, scaffolddelete.Options{})
}

func install() action.Pair {
	return scaffold.NewBasicAction("install", "install a staged kit",
		"Install a kit that has been uploaded/staged, by its UUID.\nThe kit will be installed using the global settings by default.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			id := fs.Arg(0)
			if err := connection.Client.InstallKit(id, types.KitConfig{}); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully installed kit %s", id), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("kit UUID"), nil
				}
				return "", nil
			},
		})
}

func upload() action.Pair {
	return scaffold.NewBasicAction("upload", "upload a kit file",
		"Upload a kit file (.kit) to stage it for installation.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			path := fs.Arg(0)
			pc, err := connection.Client.UploadKit(path)
			if err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("uploaded kit '%s' (UUID: %s)", pc.Name, pc.UUID), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("file path"), nil
				}
				return "", nil
			},
		})
}

func pull() action.Pair {
	return scaffold.NewBasicAction("pull", "pull a kit from a remote repository",
		"Download a kit from the configured remote repository by its UUID.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			id := fs.Arg(0)
			uid, err := uuid.Parse(id)
			if err != nil {
				return id + " is not a valid UUID", nil
			}
			pc, err := connection.Client.PullKit(uid)
			if err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("pulled kit '%s'", pc.Name), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("kit UUID"), nil
				}
				if _, err := uuid.Parse(fs.Arg(0)); err != nil {
					return fs.Arg(0) + " is not a valid UUID", nil
				}
				return "", nil
			},
		})
}

func remote() action.Pair {
	return scaffoldlist.NewListAction("list remote kits", "List kits available in the configured remote repository.",
		types.KitMetadata{},
		func(fs *pflag.FlagSet) ([]types.KitMetadata, error) {
			return connection.Client.ListRemoteKits(false)
		},
		nil,
		scaffoldlist.Options{
			CommonOptions:  scaffold.CommonOptions{Use: "remote"},
			DefaultColumns: []string{"ID", "Name", "Description", "Version"},
		})
}

func get() action.Pair {
	return scaffold.NewBasicAction("get", "get kit information",
		"Display detailed information about an installed kit by its UUID.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			id := fs.Arg(0)
			uid, err := uuid.Parse(id)
			if err != nil {
				return id + " is not a valid UUID", nil
			}
			ki, err := connection.Client.KitInfo(uid)
			if err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("Name: %s\nDescription: %s\nVersion: %d\nUUID: %s",
				ki.KitState.Name, ki.KitState.Description, ki.KitState.Version, ki.UUID.String()), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("kit UUID"), nil
				}
				if _, err := uuid.Parse(fs.Arg(0)); err != nil {
					return fs.Arg(0) + " is not a valid UUID", nil
				}
				return "", nil
			},
		})
}

// uninstall is an alias for deleteKit to match the API naming.
// It registers the same delete logic under the "uninstall" name.
func uninstall() action.Pair {
	inner := deleteKit()
	// Rename so cobra registers it as a distinct sub-command while preserving behaviour
	inner.Action.Use = "uninstall"
	inner.Action.Short = "uninstall (delete) an installed kit"
	inner.Action.Long = inner.Action.Long // keep existing long desc
	inner.Action.Aliases = []string{"remove"}
	return inner
}

// buildKit assembles a kit from a JSON build-request file and returns the resulting kit UUID.
// The JSON must match the types.KitBuildRequest structure.
// See https://docs.gravwell.io/api/kits.html for the spec.
func buildKit() action.Pair {
return scaffold.NewBasicAction("build", "build a kit from a JSON spec file",
"Assemble a new kit from a JSON file that describes its contents.\n"+
"The JSON must conform to the KitBuildRequest schema.\n\n"+
"On success the new kit UUID is printed; the kit will then appear in the staged list\n"+
"and can be downloaded with the 'kits download' action.\n\n"+
"See https://docs.gravwell.io/api/kits.html for the expected JSON structure.",
func(fs *pflag.FlagSet) (string, tea.Cmd) {
path := fs.Arg(0)
raw, err := os.ReadFile(path)
if err != nil {
return fmt.Sprintf("failed to read file '%s': %v", path, err), nil
}
var req types.KitBuildRequest
if err := json.Unmarshal(raw, &req); err != nil {
return fmt.Sprintf("failed to parse build-request JSON: %v", err), nil
}
resp, err := connection.Client.BuildKit(req)
if err != nil {
return err.Error(), nil
}
return fmt.Sprintf("built kit '%s' (UUID: %s, size: %d bytes)", req.Name, resp.UUID, resp.Size), nil
},
scaffold.BasicOptions{
CommonOptions: scaffold.CommonOptions{
Usage: fmt.Sprintf("build %s", ft.Mandatory("build-spec.json")),
},
ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
if fs.NArg() != 1 {
return phrases.Exactly1ArgRequired("path to build-spec JSON file"), nil
}
if _, err := os.Stat(fs.Arg(0)); err != nil {
return fmt.Sprintf("cannot access file '%s': %v", fs.Arg(0), err), nil
}
return "", nil
},
})
}
