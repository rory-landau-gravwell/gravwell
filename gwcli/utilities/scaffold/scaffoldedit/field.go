/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffoldedit

import (
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldcreate"

	"github.com/charmbracelet/bubbles/textinput"
)

// Config is the full set of fields available to and required from the implementor.
// It maps each scaffold.Field to a unique key.
// Field providers should be initialized before passing the Config to NewEditAction.
type Config = map[string]scaffold.Field

// FieldName returns a Field suited for Name inputs.
// Order == 100.
func FieldName(singular string) scaffold.Field {
	return scaffold.Field{
		Title:    ft.Name.Name(),
		Required: true,
		Flag: scaffold.FlagConfig{
			Name:      ft.Name.Name(),
			Usage:     ft.Name.Usage(singular),
			Shorthand: rune(ft.Name.Shorthand()[0]),
		},
		Order:    100,
		Provider: &scaffoldcreate.TextProvider{},
	}
}

// FieldDescription returns a Field suited for Description inputs.
// Order == 90.
func FieldDescription(singular string) scaffold.Field {
	return scaffold.Field{
		Title:    ft.Description.Name(),
		Required: false,
		Flag: scaffold.FlagConfig{
			Name:      ft.Description.Name(),
			Usage:     ft.Description.Usage(singular),
			Shorthand: rune(ft.Description.Shorthand()[0]),
		},
		Order:    90,
		Provider: &scaffoldcreate.TextProvider{},
	}
}

// FieldLabels returns a Field suited for taking in labels as "<1>,<2>,<3>".
// Order == 70.
func FieldLabels() scaffold.Field {
	return scaffold.Field{
		Required: false,
		Title:    "Labels",
		Flag: scaffold.FlagConfig{
			Name:  "labels",
			Usage: "comma-separated list of labels to apply",
		},
		Order: 70,
		Provider: &scaffoldcreate.TextProvider{
			CustomInit: func() textinput.Model {
				ti := stylesheet.NewTI("", true)
				ti.Placeholder = "label1,label2,label3,..."
				return ti
			},
		},
	}
}

