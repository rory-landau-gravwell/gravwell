/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package playbooks provides actions for managing Gravwell playbooks.
package playbooks

import (
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
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
	return treeutils.GenerateNav("playbooks", "manage playbooks",
		"Playbooks are markdown documents that can be used to guide investigations.",
		[]string{"playbook"}, nil,
		[]action.Pair{
			list(),
			create(),
			delete(),
			edit(),
		})
}

func list() action.Pair {
	return scaffoldlist.NewListAction("list playbooks", "List playbooks available to your user.",
		types.Playbook{},
		func(fs *pflag.FlagSet) ([]types.Playbook, error) {
			resp, err := connection.Client.ListPlaybooks(nil)
			return resp.Results, err
		},
		nil,
		scaffoldlist.Options{
			DefaultColumns: []string{
				"CommonFields.ID",
				"CommonFields.Name",
				"CommonFields.Description",
				"AuthorName",
			},
		})
}

func create() action.Pair {
	return scaffoldcreate.NewCreateAction("playbook",
		map[string]scaffoldcreate.Field{
			"name": scaffoldcreate.FieldName("playbook"),
			"desc": scaffoldcreate.FieldDescription("playbook"),
			"body": {
				Required: true,
				Title:    "body",
				Flag:     scaffoldcreate.FlagConfig{Name: "body", Usage: "markdown body content of the playbook"},
				Provider: &scaffoldcreate.TextProvider{},
				Order:    60,
			},
		},
		func(cfg map[string]scaffoldcreate.Field, fs *pflag.FlagSet) (any, string, error) {
			pb := types.Playbook{
				CommonFields: types.CommonFields{
					Name:        cfg["name"].Provider.Get(),
					Description: cfg["desc"].Provider.Get(),
				},
				Body: cfg["body"].Provider.Get(),
			}
			result, err := connection.Client.CreatePlaybook(pb)
			return result.ID, "", err
		},
		scaffoldcreate.Options{})
}

func delete() action.Pair {
	return scaffolddelete.NewDeleteAction("playbook", "playbooks",
		func(dryrun bool, id string) error {
			if dryrun {
				_, err := connection.Client.GetPlaybook(id)
				return err
			}
			return connection.Client.DeletePlaybook(id)
		},
		func() ([]multiselectlist.SelectableItem[string], error) {
			lr, err := connection.Client.ListPlaybooks(nil)
			if err != nil {
				return nil, err
			}
			var items = make([]multiselectlist.SelectableItem[string], len(lr.Results))
			for i, p := range lr.Results {
				items[i] = &listitem.Generic{
					Selected_:  false,
					ID_:        p.ID,
					Name:       p.Name,
					SecondLine: p.Description,
				}
			}

			return items, nil
		}, scaffolddelete.Options{})
}

func edit() action.Pair {
	cfg := scaffoldedit.Config{
		"name":        scaffoldedit.FieldName("playbook"),
		"description": scaffoldedit.FieldDescription("playbook"),
		"body": scaffold.Field{
			Required: true,
			Title:    "body",
			Flag:     scaffold.FlagConfig{Name: "body", Usage: "markdown body content"},
			Order:    40,
			Provider: &scaffoldcreate.TextProvider{},
		},
	}
	funcs := scaffoldedit.SubroutineSet[string, types.Playbook]{
		SelectSub: func(id string) (types.Playbook, error) {
			return connection.Client.GetPlaybook(id)
		},
		FetchSub: func() ([]types.Playbook, error) {
			resp, err := connection.Client.ListPlaybooks(nil)
			return resp.Results, err
		},
		GetTitleSub:       func(item types.Playbook) string { return item.Name },
		GetDescriptionSub: func(item types.Playbook) string { return item.Description },
		PrepopulateSub: func(item types.Playbook, fields map[string]scaffold.Field) {
			fields["name"].Provider.Set(item.Name)
			fields["description"].Provider.Set(item.Description)
			fields["body"].Provider.Set(item.Body)
		},
		EditSub: func(item *types.Playbook, fields map[string]scaffold.Field, fs *pflag.FlagSet) (string, string, error) {
			item.Name = fields["name"].Provider.Get()
			item.Description = fields["description"].Provider.Get()
			item.Body = fields["body"].Provider.Get()
			_, err := connection.Client.UpdatePlaybook(*item)
			return item.Name, "", err
		},
	}
	return scaffoldedit.NewEditAction("playbook", "playbooks", cfg, funcs)
}
