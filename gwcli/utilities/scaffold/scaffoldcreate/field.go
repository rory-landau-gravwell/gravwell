/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package scaffoldcreate

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/validate"
)

// Field, FlagConfig, NewField, InstallFlagsFromFields, SetValuesFromFlags, and ApplyChangedFlags
// are provided by the scaffold package and re-exported here for convenience.

// Field is re-exported from the scaffold package.
type Field = scaffold.Field

// FlagConfig is re-exported from the scaffold package.
type FlagConfig = scaffold.FlagConfig

// NewField is re-exported from the scaffold package.
var NewField = scaffold.NewField

// FieldName returns a struct suited for Name inputs.
// Order == 100.
func FieldName(singular string) Field {
	return Field{
		Title:    ft.Name.Name(),
		Required: true,
		Flag: FlagConfig{
			Name:      ft.Name.Name(),
			Usage:     ft.Name.Usage(singular),
			Shorthand: rune(ft.Name.Shorthand()[0]),
		},
		Order:    100,
		Provider: &TextProvider{},
	}
}

// FieldDescription returns a struct suited for Description inputs.
// Order == 90.
func FieldDescription(singular string) Field {
	return Field{
		Title:    ft.Description.Name(),
		Required: false,
		Flag: FlagConfig{
			Name:      ft.Description.Name(),
			Usage:     ft.Description.Usage(singular),
			Shorthand: rune(ft.Description.Shorthand()[0]),
		},
		Order:    90,
		Provider: &TextProvider{},
	}
}

// FieldPath returns a struct suited for file path specification inputs.
// Order == 80.
func FieldPath(singular string) Field {
	return Field{
		Title:    ft.Path.Name(),
		Required: true,
		Flag: FlagConfig{
			Name:      ft.Path.Name(),
			Usage:     ft.Path.Usage(singular),
			Shorthand: rune(ft.Path.Shorthand()[0]),
		},
		Order:    80,
		Provider: &PathProvider{},
	}
}

// FieldLabels returns a struct suited for taking in labels as "<1>,<2>,<3>".
// Order == 70.
func FieldLabels() Field {
	return Field{
		Title:    "Labels",
		Required: false,
		Flag: FlagConfig{
			Name:  "labels",
			Usage: "comma-separated list of labels to apply",
		},
		Order: 70,
		Provider: &TextProvider{
			CustomInit: func() textinput.Model {
				ti := stylesheet.NewTI("", true)
				ti.Placeholder = "label1,label2,label3,..."
				return ti
			},
		},
	}
}

// FieldFrequency returns a struct suitable for taking in the frequency of something occurring as a cron string.
// Attaches uniques.CronRuneValidator and shorthand -c.
// Order == 50.
func FieldFrequency() Field {
	return Field{
		Title:    "Frequency",
		Required: true,
		Flag: FlagConfig{
			Name:      ft.Frequency.Name(),
			Usage:     ft.Frequency.Usage(),
			Shorthand: rune(ft.Frequency.Shorthand()[0]),
		},
		Order: 50,
		Provider: &TextProvider{
			CustomInit: func() textinput.Model {
				ti := stylesheet.NewTI("", false)
				ti.Placeholder = "* * * * *"
				ti.Validate = validate.CronRuneValidator
				return ti
			},
		},
	}
}

// FieldPassword returns a struct suitable for taking in a password (using the appropriate echo mode).
func FieldPassword(required bool, fc FlagConfig, order int) Field {
	return Field{
		Title:    "Password",
		Required: required,
		Flag:     fc,
		Order:    order,
		Provider: &TextProvider{
			CustomInit: func() textinput.Model {
				ti := stylesheet.NewTI("", !required)
				ti.EchoMode = textinput.EchoPassword
				return ti
			},
		},
	}
}
