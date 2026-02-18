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
	"strings"
	"time"

	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/tree/queries/attach"
	"github.com/gravwell/gravwell/v4/gwcli/tree/queries/scheduled"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/uniques"

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
		[]*cobra.Command{scheduled.NewScheduledNav()},
		[]action.Pair{past(), attach.NewAttachAction()})
}

// #region past queries

type prettyPastQuery struct {
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

	UserQuery      string
	EffectiveQuery string
	Launched       time.Time
}

func pretty(m types.SearchHistoryEntry) prettyPastQuery {
	return prettyPastQuery{
		Type:             m.Type,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		DeletedAt:        m.DeletedAt,
		ID:               m.ID,
		ParentID:         m.ParentID,
		OwnerID:          m.OwnerID,
		Owner:            m.Owner,
		Readers:          scaffold.FormatACL(m.Readers),
		Writers:          scaffold.FormatACL(m.Readers),
		LastModifiedByID: m.LastModifiedByID,
		LastModifiedBy:   m.LastModifiedBy,
		Name:             m.Name,
		Description:      m.Description,
		Labels:           m.Labels,
		Version:          m.Version,
		Can:              m.Can,

		UserQuery:      m.UserQuery,
		EffectiveQuery: m.EffectiveQuery,
		Launched:       m.Launched,
	}
}

func past() action.Pair {
	const (
		pastUse string = "past"
		short   string = "display search history"
		long    string = "display past searches made by your user"
	)

	return scaffoldlist.NewListAction(
		short, long,
		prettyPastQuery{},
		func(fs *pflag.FlagSet) ([]prettyPastQuery, error) {
			opts := &types.QueryOptions{}
			if count, e := fs.GetInt("count"); e != nil {
				return nil, uniques.ErrGetFlag(pastUse, e)
			} else if count > 0 {
				opts.Limit = count
			}

			resp, err := connection.Client.ListSearchHistory(opts)
			if err != nil {
				// check for explicit no records error
				if strings.Contains(err.Error(), "No record") {
					clilog.Writer.Debugf("no records error: %v", err)
					return []prettyPastQuery{}, nil
				}
				return nil, err
			}

			prettyResults := make([]prettyPastQuery, len(resp.Results))
			clilog.Writer.Debugf("found %v prior searches", len(resp.Results))
			for i, result := range resp.Results {
				prettyResults[i] = pretty(result)
			}
			return prettyResults, nil
		},
		scaffoldlist.Options{
			Use: pastUse, AddtlFlags: flags,
			DefaultColumns: []string{
				"ID",
				"UserQuery",
				"EffectiveQuery",
				"Launched",
			},
		})
}

func flags() pflag.FlagSet {
	addtlFlags := pflag.FlagSet{}
	addtlFlags.Int("count", 0, "the number of past searches to display.\n"+
		"If negative or 0, fetches entire history")
	return addtlFlags
}

//#endregion past queries
