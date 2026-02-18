/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

/*
Package resources defines the resources nav, which holds data related to persistent data.
*/
package resources

import (
	"slices"
	"strings"
	"time"

	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffolddelete"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/uniques"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewResourcesNav() *cobra.Command {
	const (
		use   string = "resources"
		short string = "manage persistent search data"
		long  string = "Resources store persistent data for use in searches." +
			" Resources can be manually uploaded by a user or automatically created by search modules." +
			" Resources are used by a number of modules for things such as storing lookup tables," +
			" scripts, and more. A resource is simply a stream of bytes."
	)
	return treeutils.GenerateNav(use, short, long, nil,
		[]*cobra.Command{},
		[]action.Pair{
			list(),
			delete(),
		})
}

type prettyResource struct {
	// Common Fields

	Type             types.AssetType
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        time.Time
	ID               string
	ParentID         string
	OwnerID          int32
	Owner            types.User
	Readers          string
	Writers          string
	LastModifiedByID int32
	LastModifiedBy   types.User
	Name             string
	Description      string
	Labels           []string
	Version          int
	Can              types.Actions

	// Other Fields

	SizeBytes   uint64
	Hash        string
	ContentType string // Guessed at update time if possible
}

func pretty(r types.Resource) prettyResource {
	return prettyResource{
		Type:             r.Type,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		DeletedAt:        r.DeletedAt,
		ID:               r.ID,
		ParentID:         r.ParentID,
		OwnerID:          r.OwnerID,
		Owner:            r.Owner,
		Readers:          scaffold.FormatACL(r.Readers),
		Writers:          scaffold.FormatACL(r.Readers),
		LastModifiedByID: r.LastModifiedByID,
		LastModifiedBy:   r.LastModifiedBy,
		Name:             r.Name,
		Description:      r.Description,
		Labels:           r.Labels,
		Version:          r.Version,
		Can:              r.Can,

		SizeBytes:   r.Size,
		Hash:        r.Hash,
		ContentType: r.ContentType,
	}
}

func list() action.Pair {
	const (
		short string = "list resources on the system"
		long  string = "view resources available to your user and the system"
	)
	return scaffoldlist.NewListAction(short, long,
		prettyResource{}, func(fs *pflag.FlagSet) ([]prettyResource, error) {
			var rawResults []types.Resource
			if all, err := fs.GetBool("all"); err != nil {
				uniques.ErrGetFlag("resources list", err)
			} else if all {
				resp, err := connection.Client.ListAllResources(nil)
				if err != nil {
					return nil, err
				}
				rawResults = resp.Results
			} else {
				resp, err := connection.Client.ListResources(nil)
				if err != nil {
					return nil, err
				}
				rawResults = resp.Results
			}
			convertedResults := make([]prettyResource, len(rawResults))
			for i, result := range rawResults {
				convertedResults[i] = pretty(result)
			}
			return convertedResults, nil
		},
		scaffoldlist.Options{
			AddtlFlags: flags,
		})
}

func flags() pflag.FlagSet {
	addtlFlags := pflag.FlagSet{}
	addtlFlags.Bool("all", false, "ADMIN ONLY. Lists all resources on the system")
	return addtlFlags
}

func delete() action.Pair {
	return scaffolddelete.NewDeleteAction("resource", "resources",
		func(dryrun bool, id string) error {
			if dryrun {
				_, err := connection.Client.GetResourceMetadata(id)
				return err
			}
			return connection.Client.DeleteResource(id)
		},
		func() ([]scaffolddelete.Item[string], error) {
			resources, err := connection.Client.ListResources(nil)
			if err != nil {
				return nil, err
			}
			slices.SortStableFunc(resources.Results,
				func(a, b types.Resource) int {
					return strings.Compare(a.Name, b.Name)
				})
			var items = make([]scaffolddelete.Item[string], len(resources.Results))
			for i, r := range resources.Results {
				items[i] = scaffolddelete.NewItem(r.Name, r.Description, r.ID)
			}
			return items, nil
		})
}
