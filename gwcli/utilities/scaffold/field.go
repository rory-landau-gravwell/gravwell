/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// This file contains types and functions shared between scaffoldcreate and scaffoldedit.

package scaffold

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/pflag"
)

// ViewKind indicates how a FieldProvider's view should be rendered.
type ViewKind uint

const (
	// TitleValue is the default view kind. The view is bifurcated into a title column and a value column.
	TitleValue ViewKind = iota
	// Line is displayed as a single block, centered relative to other fields.
	Line
	// Takeover means this provider is asserting control over the entire pane.
	// View processing stops on the first takeover.
	Takeover
)

// FieldProvider defines the contract that all interactive field providers must satisfy.
// It is used by both scaffoldcreate and scaffoldedit to manage field input and display.
type FieldProvider interface {
	// Initialize the instance, fetching required data.
	// This is only called once, at tree-construction time.
	Initialize(defaultValue string, required bool)
	// Reset the instance back to its initial, ready-for-use state.
	// Called after the action's invocation completes.
	Reset()
	// SetArgs is called BEFORE flags are parsed into their fields,
	// allowing the provider to alter/set data based on context.
	SetArgs(width, height int)
	// Update handles a tea.Msg for this provider.
	// takeover tells the parent scaffold that this provider wants exclusive control.
	// In takeover mode, all updates are forwarded directly to this provider.
	Update(selected bool, msg tea.Msg) (_ tea.Cmd, takeover bool)
	// View renders this provider.
	// kind indicates how the parent scaffold should compose the view.
	// secondLine, if non-empty, is displayed below the primary view content.
	View(selected bool, width int) (_ ViewKind, value, secondLine string)
	// Satisfied returns a non-empty invalid reason if this field is not ready to be submitted.
	Satisfied() (invalid string)
	// Set attempts to assign val to this provider. Returns a non-empty invalid reason on failure.
	Set(val string) (invalid string)
	// Get returns the current value of the field as a string.
	Get() string
	// ToggleFocus focuses or blurs this provider.
	ToggleFocus(focus bool)
	// AsBoolFlag returns true if this provider's flag should be registered as a bool flag.
	// Only BoolProvider returns true; all other providers return false.
	AsBoolFlag() bool
}

// FlagConfig defines settings for customizing how the flag for a Field is displayed and handled.
// All flag configuration is optional.
type FlagConfig struct {
	Name      string // Longform flag (ex: --flagname). Derived from Title if empty.
	Usage     string // Description displayed with -h.
	Shorthand rune   // Shortform flag (ex: -f). Omitted if zero.
}

// Field defines a single data point for create or edit scaffolds.
type Field struct {
	// User-facing identifier of this field.
	Title string
	// This field must be populated prior to calling the create/edit func.
	// Ineffectual for BoolProviders.
	Required     bool
	Flag         FlagConfig // OPTIONAL. Control how this field's flag is handled.
	DefaultValue string     // OPTIONAL. Default flag and provider value.
	Order        int        // OPTIONAL. Top-Down (highest to lowest) display order.

	Provider FieldProvider
}

// NewField composes a Field from the required parameters.
func NewField(title string, required bool, provider FieldProvider) Field {
	return Field{Title: title, Required: required, Provider: provider}
}

// SortFieldKeys returns a sorted slice of keys from fields.
// Fields are sorted descending by Order, then alphabetically by Title on ties.
func SortFieldKeys(fields map[string]Field) []string {
	keys := slices.Collect(maps.Keys(fields))
	slices.SortStableFunc(keys, func(aKey, bKey string) int {
		switch {
		case fields[aKey].Order < fields[bKey].Order:
			return 1
		case fields[aKey].Order > fields[bKey].Order:
			return -1
		}
		return strings.Compare(fields[aKey].Title, fields[bKey].Title)
	})
	return keys
}

// LongestTitleLen returns the length of the longest Title string across all fields.
func LongestTitleLen(fields map[string]Field) int {
	var longest int
	for _, f := range fields {
		if l := len(f.Title); l > longest {
			longest = l
		}
	}
	return longest
}

// InstallFlagsFromFields returns a FlagSet built from the given fields.
// If Flag.Name is empty for a field it is derived from Title.
// BoolProvider fields are registered as bool flags; all others as strings.
func InstallFlagsFromFields(fields map[string]Field) pflag.FlagSet {
	var flags pflag.FlagSet
	for key, f := range fields {
		if f.Flag.Name == "" {
			f.Flag.Name = ft.DeriveFlagName(f.Title) // sanitize
		} else {
			f.Flag.Name = ft.DeriveFlagName(f.Flag.Name) // sanitize
		}
		fields[key] = f

		if f.Provider != nil && f.Provider.AsBoolFlag() {
			if f.Flag.Shorthand != 0 {
				flags.BoolP(f.Flag.Name, string(f.Flag.Shorthand), false, f.Flag.Usage)
			} else {
				flags.Bool(f.Flag.Name, false, f.Flag.Usage)
			}
		} else {
			if f.Flag.Shorthand != 0 {
				flags.StringP(f.Flag.Name, string(f.Flag.Shorthand), f.DefaultValue, f.Flag.Usage)
			} else {
				flags.String(f.Flag.Name, f.DefaultValue, f.Flag.Usage)
			}
		}
	}
	return flags
}

// SetValuesFromFlags attempts to set flag values into their respective fields.
// Returns a list of required fields that did not receive values (flag was not changed).
// This is intended for use in scaffoldcreate, where required fields must be provided.
func SetValuesFromFlags(fs *pflag.FlagSet, fields map[string]Field) (missingRequireds []string, err error) {
	if !fs.Parsed() {
		clilog.Writer.Errorf("attempted to set values from unparsed flagset")
		return nil, clilog.ErrInternal{}
	}
	for key := range fields {
		flagName := fields[key].Flag.Name
		changed := fs.Changed(flagName)
		isBool := fields[key].Provider != nil && fields[key].Provider.AsBoolFlag()
		// if this value is required, but unset, add it to the list and move on.
		// NOTE: this uses fs.Changed(), which will fail if the value was set as a default.
		if fields[key].Required && !isBool && !changed {
			missingRequireds = append(missingRequireds, fields[key].Flag.Name)
			continue
		}

		var v string
		if isBool {
			if changed {
				b, err := fs.GetBool(flagName)
				if err != nil {
					return nil, err
				}
				v = strconv.FormatBool(b)
			}
		} else if v, err = fs.GetString(flagName); err != nil {
			return nil, err
		}

		if invalid := fields[key].Provider.Set(v); invalid != "" {
			return nil, fmt.Errorf("%s is not a valid input to --%s: %s", v, fields[key].Flag.Name, invalid)
		}
	}
	return missingRequireds, nil
}

// ApplyChangedFlags applies only flag values that were explicitly set by the user.
// Returns true if at least one flag was changed, false otherwise.
// This is intended for use in scaffoldedit's non-interactive mode.
func ApplyChangedFlags(fs *pflag.FlagSet, fields map[string]Field) (anyChanged bool, err error) {
	if !fs.Parsed() {
		clilog.Writer.Errorf("attempted to apply changed flags from unparsed flagset")
		return false, clilog.ErrInternal{}
	}
	for key := range fields {
		flagName := fields[key].Flag.Name
		if !fs.Changed(flagName) {
			continue
		}
		anyChanged = true
		isBool := fields[key].Provider != nil && fields[key].Provider.AsBoolFlag()

		var v string
		if isBool {
			b, ferr := fs.GetBool(flagName)
			if ferr != nil {
				return false, ferr
			}
			v = strconv.FormatBool(b)
		} else {
			v, err = fs.GetString(flagName)
			if err != nil {
				return false, err
			}
		}

		if invalid := fields[key].Provider.Set(v); invalid != "" {
			return false, fmt.Errorf("%s is not a valid input to --%s: %s", v, fields[key].Flag.Name, invalid)
		}
	}
	return anyChanged, nil
}
