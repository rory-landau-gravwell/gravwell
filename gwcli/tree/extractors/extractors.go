/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package extractors provides actions for interacting with autoextractors.
package extractors

import (
	"time"

	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/uniques"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewExtractorsNav returns a nav based around manipulating autoextractors.
func NewExtractorsNav() *cobra.Command {
	const (
		use   string = "extractors"
		short string = "manage your tag autoextractors"
		long  string = "Autoextractors describe how to extract fields from tagged, unstructured data."
	)

	var aliases = []string{"extractor", "ex", "ax", "autoextractor", "autoextractors"}

	return treeutils.GenerateNav(use, short, long, aliases,
		[]*cobra.Command{},
		[]action.Pair{
			newExtractorsListAction(),
			newExtractorsCreateAction(),
			newExtractorDeleteAction()})
}

// #region list

type prettyExtractor struct {
	Type      types.AssetType
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
	ID        string
	ParentID  string // the parent object this was cloned from

	OwnerID int32
	Owner   types.User

	// Permissions
	Readers string
	Writers string

	// Tracks who made the last change to this item
	LastModifiedByID int32
	LastModifiedBy   types.User

	Name        string
	Description string
	Labels      []string
	Version     int

	// Auto-generated for the requesting user based on permissions of this object.
	Can types.Actions

	Module string   `toml:"module"`
	Params string   `toml:"params" json:",omitempty"`
	Args   string   `toml:"args,omitempty" json:",omitempty"`
	Tags   []string `toml:"tags"` // AXs can support multiple tags. For backwards compatibility, we leave Tag and add Tags
}

// Convert maps the underlying AX type to the pretty wrapper used by List.
// Most types are a direct copy.
func Convert(a types.AX) prettyExtractor {
	return prettyExtractor{
		Type:             a.Type,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
		DeletedAt:        a.DeletedAt,
		ID:               a.ID,
		ParentID:         a.ParentID,
		OwnerID:          a.OwnerID,
		Owner:            a.Owner,
		Readers:          scaffold.FormatACL(a.Readers),
		Writers:          scaffold.FormatACL(a.Writers),
		LastModifiedByID: a.LastModifiedByID,
		LastModifiedBy:   a.LastModifiedBy,
		Name:             a.Name,
		Description:      a.Description,
		Labels:           a.Labels,
		Version:          a.Version,
		Can:              a.Can,

		Module: a.Module,
		Params: a.Params,
		Args:   a.Args,
		Tags:   a.Tags,
	}
}

func newExtractorsListAction() action.Pair {
	const (
		short string = "list extractors"
		long  string = "list autoextractions available to you and the system"
	)

	return scaffoldlist.NewListAction(
		short,
		long,
		prettyExtractor{},
		list,
		scaffoldlist.Options{
			AddtlFlags: flags,
			DefaultColumns: []string{
				"ID",
				"Name",
				"Description",
				"Module",
				"Params",
				"Args",
				"Tags",
			},
		})
}

func flags() pflag.FlagSet {
	addtlFlags := pflag.FlagSet{}
	addtlFlags.String("id", "", "Fetch extractor by id")
	return addtlFlags
}

func list(fs *pflag.FlagSet) ([]prettyExtractor, error) {
	if id, err := fs.GetString("id"); err != nil {
		uniques.ErrGetFlag("extractors list", err)
	} else if id != "" {
		clilog.Writer.Infof("Fetching ax with id \"%v\"", id)
		d, err := connection.Client.GetExtraction(id)
		return []prettyExtractor{Convert(d)}, err
	}

	lr, err := connection.Client.ListExtractions(nil)
	converted := make([]prettyExtractor, len(lr.Results))
	for i, result := range lr.Results {
		converted[i] = Convert(result)
	}
	return converted, err
}

//#endregion list
