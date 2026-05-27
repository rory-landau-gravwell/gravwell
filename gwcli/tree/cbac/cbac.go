/*************************************************************************
 * Copyright 2024 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package cbac provides actions for inspecting and managing Capability-Based
// Access Control (CBAC) rules.
//
// # CBAC overview
//
// Gravwell's CBAC system controls which operations a user may perform by
// granting or denying named capabilities.  Capabilities can be granted
// directly to a user or inherited through group membership.
//
// # Client-library methods available
//
//   - [client.CapabilityList] – returns every capability the system knows
//     about, including its name, description, category, and flags such as
//     TokenOnly / AdminOnly.
//   - [client.CapabilityTemplateList] – returns named templates (e.g.
//     "Full User", "Read Only") that bundle common sets of capabilities.
//   - [client.CurrentUserCapabilities] – the calling user's effective
//     capability set (as a slice of CapabilityDesc).
//   - [client.CurrentUserCapabilityExplanations] – same as above but
//     includes why each capability was granted (direct grant vs. group).
//   - [client.HasCapability] – boolean check for a single capability.
//   - [client.GetUserCapabilities(uid)] (admin) – get capabilities for any user.
//   - [client.SetUserCapabilities(uid, CapabilityState)] (admin) – overwrite
//     the capability grant list for a user.
//   - [client.GetUserCapabilityExplanations(uid)] (admin) – explanations for
//     any user's capabilities.
//   - [client.GetGroupCapabilities(gid)] (admin) – get capabilities for a group.
//   - [client.SetGroupCapabilities(gid, CapabilityState)] (admin) – overwrite
//     the capability grant list for a group.
//   - [client.GetUserTagAccess(uid)] / [client.SetUserTagAccess(uid, TagAccess)] (admin)
//     – read or write the tag-access filter for a specific user.
//   - [client.GetGroupTagAccess(gid)] / [client.SetGroupTagAccess(gid, TagAccess)] (admin)
//     – read or write the tag-access filter for a specific group.
//
// # Types
//
//   - types.CapabilityDesc  – {Cap Capability, Name string, Desc string, Category, TokenOnly, AdminOnly}
//   - types.CapabilityState – {Grants []string}  (list of capability names to grant)
//   - types.CapabilityExplanation – embeds CapabilityDesc; adds Granted bool,
//     UserGrant bool, GroupGrants []Group.
//   - types.CapabilityTemplate – {Name, Desc string, Caps []Capability}
//   - types.TagAccess – {Grants []string}  (glob patterns for allowed tags)
package cbac

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// NewCBACNav returns a nav with children relating to capability-based access
// control.
func NewCBACNav() *cobra.Command {
	return treeutils.GenerateNav(
		"cbac", "manage capability-based access control",
		"Inspect and manage CBAC rules that govern which operations users and groups may perform.",
		[]string{"capabilities"},
		[]*cobra.Command{},
		[]action.Pair{
			listCapabilities(),
			listTemplates(),
			myCapabilities(),
			explainMyCapabilities(),
			getUserCapabilities(),
			setUserCapabilities(),
			explainUserCapabilities(),
			getGroupCapabilities(),
			setGroupCapabilities(),
			getUserTagAccess(),
			setUserTagAccess(),
			getGroupTagAccess(),
			setGroupTagAccess(),
		})
}

// listCapabilities lists every capability the system knows about.
func listCapabilities() action.Pair {
	return scaffoldlist.NewListAction(
		"list all known capabilities",
		"List every capability recognised by this Gravwell instance, including its name, description, and flags.",
		types.CapabilityDesc{},
		func(_ *pflag.FlagSet) ([]types.CapabilityDesc, error) {
			return connection.Client.CapabilityList()
		},
		nil,
		scaffoldlist.Options{})
}

// listTemplates lists named capability templates.
func listTemplates() action.Pair {
	return scaffoldlist.NewListAction(
		"list capability templates",
		"List the built-in capability templates (e.g. 'Full User', 'Read Only') that bundle common sets of capabilities.",
		types.CapabilityTemplate{},
		func(_ *pflag.FlagSet) ([]types.CapabilityTemplate, error) {
			return connection.Client.CapabilityTemplateList()
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{Use: "templates"},
		})
}

// myCapabilities shows the calling user's effective capabilities.
func myCapabilities() action.Pair {
	return scaffoldlist.NewListAction(
		"show your effective capabilities",
		"Display the capabilities that are currently granted to your user account.",
		types.CapabilityDesc{},
		func(_ *pflag.FlagSet) ([]types.CapabilityDesc, error) {
			return connection.Client.CurrentUserCapabilities()
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{Use: "my-capabilities"},
		})
}

// explainMyCapabilities shows how the calling user obtained each capability.
func explainMyCapabilities() action.Pair {
	return scaffoldlist.NewListAction(
		"explain how your capabilities were granted",
		"Display each capability along with whether it was granted directly to your user or inherited through group membership.",
		types.CapabilityExplanation{},
		func(_ *pflag.FlagSet) ([]types.CapabilityExplanation, error) {
			return connection.Client.CurrentUserCapabilityExplanations()
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{Use: "explain"},
		})
}

// getUserCapabilities shows a specific user's capability grants (admin only).
func getUserCapabilities() action.Pair {
	return scaffold.NewBasicAction(
		"get-user-capabilities", "show capabilities granted to a user (admin)",
		"Display the capability grant list for a specific user.\n"+
			"Usage: cbac get-user-capabilities <user-ID>",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			var uid int32
			if _, err := fmt.Sscan(fs.Arg(0), &uid); err != nil {
				return fmt.Sprintf("invalid user ID %q: %v", fs.Arg(0), err), nil
			}
			cs, err := connection.Client.GetUserCapabilities(uid)
			if err != nil {
				return err.Error(), nil
			}
			if len(cs.Grants) == 0 {
				return "no capabilities explicitly granted", nil
			}
			return strings.Join(cs.Grants, "\n"), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("user ID"), nil
				}
				return "", nil
			},
		})
}

// setUserCapabilities overwrites the capability grants for a user (admin only).
func setUserCapabilities() action.Pair {
	return scaffold.NewBasicAction(
		"set-user-capabilities", "overwrite capabilities granted to a user (admin)",
		"Overwrite the capability grant list for a specific user.\n"+
			"Usage: cbac set-user-capabilities <user-ID> [capability ...]\n\n"+
			"Pass zero capability names to clear all grants.\n"+
			"Run 'cbac list' to see valid capability names.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			args := fs.Args()
			var uid int32
			if _, err := fmt.Sscan(args[0], &uid); err != nil {
				return fmt.Sprintf("invalid user ID %q: %v", args[0], err), nil
			}
			cs := types.CapabilityState{Grants: args[1:]}
			if err := connection.Client.SetUserCapabilities(uid, cs); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("capabilities updated for user %d", uid), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() < 1 {
					return "user ID is required", nil
				}
				return "", nil
			},
		})
}

// explainUserCapabilities shows how a specific user obtained each capability (admin only).
func explainUserCapabilities() action.Pair {
	return scaffold.NewBasicAction(
		"explain-user-capabilities", "explain how a user's capabilities were granted (admin)",
		"Display each capability for a given user along with how it was obtained.\n"+
			"Usage: cbac explain-user-capabilities <user-ID>",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			var uid int32
			if _, err := fmt.Sscan(fs.Arg(0), &uid); err != nil {
				return fmt.Sprintf("invalid user ID %q: %v", fs.Arg(0), err), nil
			}
			exps, err := connection.Client.GetUserCapabilityExplanations(uid)
			if err != nil {
				return err.Error(), nil
			}
			if len(exps) == 0 {
				return "no capabilities found for this user", nil
			}
			var sb strings.Builder
			for _, e := range exps {
				granted := "no"
				if e.Granted {
					granted = "yes"
				}
				groups := make([]string, 0, len(e.GroupGrants))
				for _, g := range e.GroupGrants {
					groups = append(groups, g.Name)
				}
				groupStr := "(none)"
				if len(groups) > 0 {
					groupStr = strings.Join(groups, ", ")
				}
				sb.WriteString(fmt.Sprintf("%s: granted=%s, via groups=[%s]\n",
					e.Name, granted, groupStr))
			}
			return strings.TrimRight(sb.String(), "\n"), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("user ID"), nil
				}
				return "", nil
			},
		})
}

// getGroupCapabilities shows a group's capability grants (admin only).
func getGroupCapabilities() action.Pair {
	return scaffold.NewBasicAction(
		"get-group-capabilities", "show capabilities granted to a group (admin)",
		"Display the capability grant list for a specific group.\n"+
			"Usage: cbac get-group-capabilities <group-ID>",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			var gid int32
			if _, err := fmt.Sscan(fs.Arg(0), &gid); err != nil {
				return fmt.Sprintf("invalid group ID %q: %v", fs.Arg(0), err), nil
			}
			cs, err := connection.Client.GetGroupCapabilities(gid)
			if err != nil {
				return err.Error(), nil
			}
			if len(cs.Grants) == 0 {
				return "no capabilities explicitly granted", nil
			}
			return strings.Join(cs.Grants, "\n"), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("group ID"), nil
				}
				return "", nil
			},
		})
}

// setGroupCapabilities overwrites the capability grants for a group (admin only).
func setGroupCapabilities() action.Pair {
	return scaffold.NewBasicAction(
		"set-group-capabilities", "overwrite capabilities granted to a group (admin)",
		"Overwrite the capability grant list for a specific group.\n"+
			"Usage: cbac set-group-capabilities <group-ID> [capability ...]\n\n"+
			"Pass zero capability names to clear all grants.\n"+
			"Run 'cbac list' to see valid capability names.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			args := fs.Args()
			var gid int32
			if _, err := fmt.Sscan(args[0], &gid); err != nil {
				return fmt.Sprintf("invalid group ID %q: %v", args[0], err), nil
			}
			cs := types.CapabilityState{Grants: args[1:]}
			if err := connection.Client.SetGroupCapabilities(gid, cs); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("capabilities updated for group %d", gid), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() < 1 {
					return "group ID is required", nil
				}
				return "", nil
			},
		})
}

// getUserTagAccess shows the tag-access filter for a user (admin only).
func getUserTagAccess() action.Pair {
	return scaffold.NewBasicAction(
		"get-user-tags", "show tag-access filter for a user (admin)",
		"Display which tag patterns a specific user is allowed to access.\n"+
			"Usage: cbac get-user-tags <user-ID>",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			var uid int32
			if _, err := fmt.Sscan(fs.Arg(0), &uid); err != nil {
				return fmt.Sprintf("invalid user ID %q: %v", fs.Arg(0), err), nil
			}
			ta, err := connection.Client.GetUserTagAccess(uid)
			if err != nil {
				return err.Error(), nil
			}
			if len(ta.Grants) == 0 {
				return "no tag restrictions (all tags accessible)", nil
			}
			return strings.Join(ta.Grants, "\n"), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("user ID"), nil
				}
				return "", nil
			},
		})
}

// setUserTagAccess overwrites the tag-access filter for a user (admin only).
func setUserTagAccess() action.Pair {
	return scaffold.NewBasicAction(
		"set-user-tags", "overwrite tag-access filter for a user (admin)",
		"Overwrite the tag-access filter for a specific user.\n"+
			"Usage: cbac set-user-tags <user-ID> [tag-pattern ...]\n\n"+
			"Tag patterns may include glob wildcards, e.g. 'syslog*'.\n"+
			"Pass zero patterns to remove all tag restrictions.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			args := fs.Args()
			var uid int32
			if _, err := fmt.Sscan(args[0], &uid); err != nil {
				return fmt.Sprintf("invalid user ID %q: %v", args[0], err), nil
			}
			ta := types.TagAccess{Grants: args[1:]}
			if err := connection.Client.SetUserTagAccess(uid, ta); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("tag access updated for user %d", uid), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() < 1 {
					return "user ID is required", nil
				}
				return "", nil
			},
		})
}

// getGroupTagAccess shows the tag-access filter for a group (admin only).
func getGroupTagAccess() action.Pair {
	return scaffold.NewBasicAction(
		"get-group-tags", "show tag-access filter for a group (admin)",
		"Display which tag patterns a specific group is allowed to access.\n"+
			"Usage: cbac get-group-tags <group-ID>",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			var gid int32
			if _, err := fmt.Sscan(fs.Arg(0), &gid); err != nil {
				return fmt.Sprintf("invalid group ID %q: %v", fs.Arg(0), err), nil
			}
			ta, err := connection.Client.GetGroupTagAccess(gid)
			if err != nil {
				return err.Error(), nil
			}
			if len(ta.Grants) == 0 {
				return "no tag restrictions (all tags accessible)", nil
			}
			return strings.Join(ta.Grants, "\n"), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("group ID"), nil
				}
				return "", nil
			},
		})
}

// setGroupTagAccess overwrites the tag-access filter for a group (admin only).
func setGroupTagAccess() action.Pair {
	return scaffold.NewBasicAction(
		"set-group-tags", "overwrite tag-access filter for a group (admin)",
		"Overwrite the tag-access filter for a specific group.\n"+
			"Usage: cbac set-group-tags <group-ID> [tag-pattern ...]\n\n"+
			"Tag patterns may include glob wildcards, e.g. 'syslog*'.\n"+
			"Pass zero patterns to remove all tag restrictions.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			args := fs.Args()
			var gid int32
			if _, err := fmt.Sscan(args[0], &gid); err != nil {
				return fmt.Sprintf("invalid group ID %q: %v", args[0], err), nil
			}
			ta := types.TagAccess{Grants: args[1:]}
			if err := connection.Client.SetGroupTagAccess(gid, ta); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("tag access updated for group %d", gid), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (string, error) {
				if fs.NArg() < 1 {
					return "group ID is required", nil
				}
				return "", nil
			},
		})
}
