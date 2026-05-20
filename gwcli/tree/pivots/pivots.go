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
	"github.com/google/uuid"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewNav() *cobra.Command {
	return treeutils.GenerateNav("pivots", "manage pivots",
		"Pivots are actionable items that appear when hovering over data in the Gravwell web interface.",
		[]string{"pivot"}, nil,
		[]action.Pair{
			list(),
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
		func() ([]scaffolddelete.Item[string], error) {
			pivots, err := connection.Client.ListPivots()
			if err != nil {
				return nil, err
			}
			var items = make([]scaffolddelete.Item[string], len(pivots))
			for i, p := range pivots {
				items[i] = scaffolddelete.NewItem(p.Name, p.Description, p.GUID.String())
			}
			return items, nil
		}, scaffolddelete.Options{})
}
