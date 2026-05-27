/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package pivots provides actions for managing Gravwell pivots (actionable items).
package pivots

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldedit"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewNav() *cobra.Command {
	return treeutils.GenerateNav("pivots", "manage pivots",
		"Pivots are actionable items that appear when hovering over data in the Gravwell web interface.\n"+
			"They allow users to quickly pivot from a data value to a related search or action.\n"+
			"Pivot contents are stored as a JSON blob describing the actionable behaviour.",
		[]string{"pivot"}, nil,
		[]action.Pair{
			list(),
			create(),
			editAction(),
			show(),
			delete(),
		})
}

func list() action.Pair {
	return scaffoldlist.NewListAction("list pivots", "List pivots available to your user.",
		types.WirePivot{},
		func(fs *pflag.FlagSet) ([]types.WirePivot, error) {
			return connection.Client.ListPivots()
		},
		nil,
		scaffoldlist.Options{
			DefaultColumns: []string{"GUID", "Name", "Description"},
		})
}

// create creates a new pivot.  The pivot contents must be provided as a JSON file.
func create() action.Pair {
	return scaffold.NewBasicAction("create", "create a new pivot",
		"Create a pivot (actionable item) from a JSON file describing its behaviour.\n"+
			"Requires --name and --path (path to the JSON content file).\n\n"+
			"See https://docs.gravwell.io/api/pivots.html for the expected JSON structure.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			name, _ := fs.GetString("name")
			desc, _ := fs.GetString("desc")
			path, _ := fs.GetString("path")

			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Sprintf("failed to read file '%s': %v", path, err), nil
			}

			contents := types.RawObject(raw)
			guid, err := connection.Client.NewPivot(uuid.New(), name, desc, contents)
			if err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("created pivot '%s' with GUID %s", name, guid), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.String("name", "", "name for the new pivot (required)")
					fs.StringP("desc", "d", "", "description for the new pivot")
					fs.StringP("path", "p", "", "path to a JSON file containing the pivot contents (required)")
					return fs
				},
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				name, _ := fs.GetString("name")
				if name == "" {
					return "--name is required", nil
				}
				path, _ := fs.GetString("path")
				if path == "" {
					return "--path is required", nil
				}
				if _, err := os.Stat(path); err != nil {
					return fmt.Sprintf("cannot access file '%s': %v", path, err), nil
				}
				return "", nil
			},
		})
}

// editAction updates a pivot's name and/or description.
func editAction() action.Pair {
	cfg := scaffoldedit.Config{
		"name":        scaffoldedit.FieldName("pivot"),
		"description": scaffoldedit.FieldDescription("pivot"),
	}
	funcs := scaffoldedit.SubroutineSet[string, types.WirePivot]{
		SelectSub: func(id string) (types.WirePivot, error) {
			uid, err := uuid.Parse(id)
			if err != nil {
				return types.WirePivot{}, err
			}
			return connection.Client.GetPivot(uid)
		},
		FetchSub: func() ([]types.WirePivot, error) {
			return connection.Client.ListPivots()
		},
		GetFieldSub: func(item types.WirePivot, fieldKey string) (string, error) {
			switch fieldKey {
			case "name":
				return item.Name, nil
			case "description":
				return item.Description, nil
			}
			return "", fmt.Errorf("unknown field key: %v", fieldKey)
		},
		SetFieldSub: func(item *types.WirePivot, fieldKey, val string) (string, error) {
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
		GetTitleSub:       func(item types.WirePivot) string { return item.Name },
		GetDescriptionSub: func(item types.WirePivot) string { return item.Description },
		UpdateSub: func(data *types.WirePivot) (string, error) {
			uid := data.ThingUUID
			_, err := connection.Client.SetPivot(uid, *data)
			return data.Name, err
		},
	}
	return scaffoldedit.NewEditAction("pivot", "pivots", cfg, funcs)
}

// show displays the details of a pivot including its JSON contents.
func show() action.Pair {
	return scaffold.NewBasicAction("show", "display a pivot's details",
		"Display the details of a pivot by its GUID, including its JSON contents.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			id := fs.Arg(0)
			uid, err := uuid.Parse(id)
			if err != nil {
				return fmt.Sprintf("'%s' is not a valid GUID: %v", id, err), nil
			}
			p, err := connection.Client.GetPivot(uid)
			if err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("GUID: %s\nName: %s\nDescription: %s\nDisabled: %v\nContents:\n%s",
				p.GUID, p.Name, p.Description, p.Disabled, string(p.Contents)), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				Aliases: []string{"print", "get"},
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("pivot GUID"), nil
				}
				if _, err := uuid.Parse(fs.Arg(0)); err != nil {
					return fmt.Sprintf("'%s' is not a valid GUID", fs.Arg(0)), nil
				}
				return "", nil
			},
		})
}

func delete() action.Pair {
	return scaffolddelete.NewDeleteAction("pivot", "pivots",
		func(dryrun bool, id string) error {
			uid, err := uuid.Parse(id)
			if err != nil {
				return err
			}
			if dryrun {
				_, err = connection.Client.GetPivot(uid)
				return err
			}
			return connection.Client.DeletePivot(uid)
		},
		func() ([]multiselectlist.SelectableItem[string], error) {
			pivots, err := connection.Client.ListPivots()
			if err != nil {
				return nil, err
			}
			var items = make([]multiselectlist.SelectableItem[string], len(pivots))
			for i, p := range pivots {
				items[i] = &listitem.Generic{
					Selected_:  false,
					ID_:        p.ThingUUID.String(), // TODO replace with p.ID when pivots/actionables are registry-ready
					Name:       p.Name,
					SecondLine: p.Description,
				}
			}

			return items, nil
		}, scaffolddelete.Options{})
}
