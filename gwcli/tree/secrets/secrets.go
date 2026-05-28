// Package secrets introduces actions for managing secrets.
package secrets

/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/bubbles/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
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
		use   string = "secrets"
		short string = "manage secret data that can be fed into other actions"
		long  string = "Gravwell can store secret strings for use in other actions (typically flows)." +
			" Once created, the user cannot read the contents of the secret again, although the value can be updated." +
			" The user may then refer to the secret in certain node types when building a flow."
	)
	return treeutils.GenerateNav(use, short, long, []string{"secret"},
		[]*cobra.Command{},
		[]action.Pair{
			listAction(),
			create(),
			delete(),
			edit(),
			updateValue(),
		})
}

func listAction() action.Pair {
	const (
		short string = "list secrets on the system"
		long  string = "View secrets available to your user."
	)
	return scaffoldlist.NewListAction(short, long,
		types.Secret{}, func(fs *pflag.FlagSet) ([]types.Secret, error) {
			if all, err := fs.GetBool("all"); err != nil {
				clilog.GetFlag(err)
			} else if all {
				resp, err := connection.Client.ListAllSecrets(nil)
				if err != nil {
					return nil, err
				}
				return resp.Results, nil
			}

			resp, err := connection.Client.ListSecrets(nil)
			if err != nil {
				return nil, err
			}
			return resp.Results, nil
		},
		nil,
		scaffoldlist.Options{
			DefaultColumns: []string{
				"CommonFields.ID",
				"CommonFields.Name",
				"CommonFields.Description",
			},
			CommonOptions: scaffold.CommonOptions{AddtlFlags: flags},
		})
}

func flags() *pflag.FlagSet {
	addtlFlags := pflag.FlagSet{}
	ft.GetAll.Register(&addtlFlags, true, "secrets")
	return &addtlFlags
}

func create() action.Pair {
	fields := map[string]scaffoldcreate.Field{
		"name": scaffoldcreate.FieldName("secret"),
		"desc": scaffoldcreate.FieldDescription("secret"),
		"value": scaffoldcreate.Field{
			Required: true,
			Title:    "Value",
			Flag:     scaffoldcreate.FlagConfig{Usage: "the secret itself", Shorthand: 'v'},
			Provider: &scaffoldcreate.TextProvider{},
			Order:    80,
		},
		"labels": scaffoldcreate.FieldLabels(),
	}

	return scaffoldcreate.NewCreateAction("secret", fields,
		func(cfg map[string]scaffoldcreate.Field, fs *pflag.FlagSet) (id any, invalid string, err error) {
			// transmute to resource struct
			var labels []string
			if lbls := cfg["labels"].Provider.Get(); strings.TrimSpace(lbls) != "" {
				labels = strings.Split(strings.TrimSpace(lbls), ",")
			}

			data := types.SecretCreate{
				CommonFields: types.CommonFields{
					Name:        cfg["name"].Provider.Get(),
					Description: cfg["desc"].Provider.Get(),
					Labels:      labels,
				},
				Value: cfg["value"].Provider.Get(),
			}

			resp, err := connection.Client.CreateSecret(data)
			if err != nil {
				return "", "", err
			}

			return resp.ID, "", err
		}, scaffoldcreate.Options{})
}

func delete() action.Pair {
	return scaffolddelete.NewDeleteAction("secret", "secrets",
		func(dryrun bool, id string) error {
			if dryrun {
				_, err := connection.Client.GetSecret(id)
				return err
			}
			return connection.Client.DeleteSecret(id)
		},
		func() ([]multiselectlist.SelectableItem[string], error) {
			lr, err := connection.Client.ListSecrets(&types.QueryOptions{AdminMode: connection.AdminMode()})
			if err != nil {
				return nil, err
			}
			var items = make([]multiselectlist.SelectableItem[string], len(lr.Results))
			for i, s := range lr.Results {
				items[i] = &listitem.Generic{
					Selected_:  false,
					ID_:        s.ID,
					Name:       s.Name,
					SecondLine: s.Description,
				}
			}

			return items, nil
		}, scaffolddelete.Options{})
}

func edit() action.Pair {
	return scaffoldedit.NewEditAction("secret", "secret", scaffoldedit.Config{
		"name":   scaffoldedit.FieldName("secret"),
		"desc":   scaffoldedit.FieldDescription("secret"),
		"labels": scaffoldedit.FieldLabels(),
	}, scaffoldedit.SubroutineSet[string, types.Secret]{
		SelectSub: func(id string) (item types.Secret, err error) {
			return connection.Client.GetSecret(id)
		},
		FetchSub: func() (items []types.Secret, err error) {
			resp, err := connection.Client.ListSecrets(nil)
			if err != nil {
				return nil, err
			}
			return resp.Results, nil
		},
		GetTitleSub: func(item types.Secret) string {
			return item.Name
		},
		GetDescriptionSub: func(item types.Secret) string {
			return item.Description
		},
		PrepopulateSub: func(item types.Secret, fields map[string]scaffold.Field) {
			fields["name"].Provider.Set(item.Name)
			fields["desc"].Provider.Set(item.Description)
			fields["labels"].Provider.Set(strings.Join(item.Labels, ","))
		},
		EditSub: func(item *types.Secret, fields map[string]scaffold.Field, fs *pflag.FlagSet) (string, string, error) {
			name := fields["name"].Provider.Get()
			if strings.Contains(name, " ") {
				return "", "name may not contain spaces", nil
			}
			item.Name = strings.ToUpper(name)
			item.Description = fields["desc"].Provider.Get()
			if lbls := fields["labels"].Provider.Get(); lbls != "" {
				var filtered []string
				for _, l := range strings.Split(lbls, ",") {
					if l != "" {
						filtered = append(filtered, l)
					}
				}
				item.Labels = filtered
			} else {
				item.Labels = nil
			}
			var sc types.SecretCreate
			sc.CommonFields = item.CommonFields
			sc.CommonFields.Name = item.Name
			sc.CommonFields.Description = item.Description
			sc.CommonFields.Labels = item.Labels
			s, err := connection.Client.UpdateSecret(item.ID, sc)
			return s.Name, "", err
		},
	})
}

func updateValue() action.Pair {
	return scaffold.NewBasicAction("update", "update a secret's value",
		"Update the value stored in a secret. The secret is identified by its ID.\n"+
			"Use --value to provide the new value.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			id := fs.Arg(0)
			value, err := fs.GetString("value")
			if err != nil {
				return err.Error(), nil
			}
			s, err := connection.Client.UpdateSecretValue(id, value)
			if err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully updated value for secret '%s'", s.Name), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.String("value", "", "new value for the secret")
					return fs
				},
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("secret ID"), nil
				}
				if !fs.Changed("value") {
					return "--value is required", nil
				}
				return "", nil
			},
		})
}
