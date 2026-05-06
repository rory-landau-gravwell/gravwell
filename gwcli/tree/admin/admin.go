// Package admin provides actions reserved for admins.
// It should be hidden to non-admin users.
package admin

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/client/types"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/phrases"
	"github.com/gravwell/gravwell/v4/gwcli/tree/admin/email"
	"github.com/gravwell/gravwell/v4/gwcli/tree/admin/groups"
	"github.com/gravwell/gravwell/v4/gwcli/tree/admin/license"
	admin_users "github.com/gravwell/gravwell/v4/gwcli/tree/admin/users"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/scaffold/scaffoldlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewNav() *cobra.Command {
	const (
		use   string = "admin"
		short string = "actions reserved for use by admin users"
		long  string = "Admin contains actions that require elevated privileges." +
			" These actions span a variety of categories and have some overlap with general-use actions."
	)
	return treeutils.GenerateNav(use, short, long, []string{"administrator"},
		[]*cobra.Command{
			groups.NewNav(),
			admin_users.NewNav(),
			license.NewNav(),
			email.NewNav(),
		},
		[]action.Pair{
			cleanup(),
			logLevel(),
			addIndexer(),
			backup(),
			restore(),
			listQueries(),
			deleteQuery(),
		},
	)
}

// does not include "all"
var targets = []string{
	"macros",
	"resources",
	"search_history",
	"secrets",
	"templates",
	"tokens",
	"user_preferences",
}

// getTarget returns the cleanup function associated to a given target in the targets list.
// We have to use this over making targets a map because Client will be nil until all actions have been built.
// Therefore, we cannot cache the cleanup functions.
//
// Returns nil if the target is unknown
func getTarget(target string) func() error {
	switch target {
	case "macros":
		return connection.Client.CleanupMacros
	case "resources":
		return connection.Client.CleanupResources
	case "search_history":
		return connection.Client.CleanupSearchHistory
	case "secrets":
		return connection.Client.CleanupSecrets
	case "templates":
		return connection.Client.CleanupTemplates
	case "tokens":
		return connection.Client.CleanupTokens
	case "user_preferences":
		return connection.Client.CleanupUserPreferences
	default:
		return nil
	}
}

// clean up is responsible for calling all specified cleanup functions, thus purging the respective type/resource/asset/entity
func cleanup() action.Pair {
	slices.Sort(targets)
	return scaffold.NewBasicAction(
		"cleanup",
		"purges deleted items from the system",
		"Purges deleted items of the given type, rendered them unable to be restored.\n"+
			"Available targets:\n"+
			"- all\n- "+
			strings.Join(targets, "\n- "),
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			// compact the list of items to clean so we don't make duplicate m
			var (
				m   = map[string]bool{}
				all = false
			)
			for _, arg := range fs.Args() {
				// sanitize text
				arg = strings.ToLower(strings.TrimSpace(arg))
				m[arg] = true
				if arg == "all" {
					all = true
				}
			}
			if all {
				var out string
				if len(m) > 1 {
					out = "\"all\" specified; other targets are redundant\n"
				}

				return out + strings.Join(runCleanup(targets), "\n"), nil
			}

			// validate all cleanups before calling *any*
			requested := slices.Collect(maps.Keys(m))
			slices.Sort(requested)
			invalid := []string{}
			for _, req := range requested {
				if f := getTarget(req); f == nil {
					invalid = append(invalid, req)
				}
			}
			if len(invalid) > 0 {
				return "unknown cleanup targets: " + strings.Join(invalid, ", "), nil
			}

			return strings.Join(runCleanup(requested), "\n"), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				Aliases: []string{"clean", "tidy", "purge", "burninate"},
				Usage:   fmt.Sprintf("cleanup %v %v ...", ft.Mandatory("TARGET1"), ft.Optional("TARGET2")),
				Example: "cleanup macros secrets",
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() < 1 {
					return "you must specify at least one item to clean up or \"all\"", nil
				}
				return "", nil
			},
		})
}

// helper function for cleanup.
// msgs can contain a mix of success and error messages
func runCleanup(targetsToRun []string) (msgs []string) {
	for _, target := range targetsToRun {
		f := getTarget(target)
		if f == nil {
			msgs = append(msgs, target+" is not a valid target")
			continue
		}
		if err := f(); err != nil {
			msgs = append(msgs, "failed to clean up "+target+": "+err.Error())
			continue
		}
		msgs = append(msgs, "successfully purged "+target)
	}
	return
}

func logLevel() action.Pair {
	return scaffold.NewBasicAction("log-level", "get or set the server log level",
		"Display the current server log level. Use --set to change it.\nValid levels are typically: OFF, ERROR, WARN, INFO, DEBUG",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			if fs.Changed("set") {
				level, err := fs.GetString("set")
				if err != nil {
					clilog.LogFlagFailedGet("set", err)
					return "failed to get set flag", nil
				}
				if err := connection.Client.SetLogLevel(level); err != nil {
					return err.Error(), nil
				}
				return "log level set to " + level, nil
			}
			level, err := connection.Client.GetLogLevel()
			if err != nil {
				return err.Error(), nil
			}
			return "current log level: " + level, nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.String("set", "", "log level to set (empty = display current)")
					return fs
				},
			},
		})
}

func addIndexer() action.Pair {
	return scaffold.NewBasicAction("add-indexer", "add an indexer to the system",
		"Add a remote indexer using its dial string (e.g. host:port).",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			dialstring := fs.Arg(0)
			result, err := connection.Client.AddIndexer(dialstring)
			if err != nil {
				return err.Error(), nil
			}
			var sb strings.Builder
			for k, v := range result {
				sb.WriteString(k + ": " + v + "\n")
			}
			out := strings.TrimRight(sb.String(), "\n")
			if out == "" {
				return "indexer added successfully", nil
			}
			return out, nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("dial string"), nil
				}
				return "", nil
			},
		})
}

func backup() action.Pair {
	return scaffold.NewBasicAction("backup", "backup the system",
		"Download a backup of the Gravwell system to a file.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			output, err := fs.GetString("output")
			if err != nil {
				clilog.LogFlagFailedGet("output", err)
				return "failed to get output flag", nil
			}
			f, err := os.Create(output)
			if err != nil {
				return err.Error(), nil
			}
			defer f.Close()
			cfg := types.BackupConfig{}
			if noHistory, err := fs.GetBool("no-search-history"); err != nil {
				clilog.LogFlagFailedGet("no-search-history", err)
			} else if noHistory {
				cfg.OmitSensitive = true
			}
			if err := connection.Client.BackupWithConfig(f, cfg); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("backup written to %s", output), nil
		},
		scaffold.BasicOptions{
			CommonOptions: scaffold.CommonOptions{
				AddtlFlags: func() *pflag.FlagSet {
					fs := &pflag.FlagSet{}
					fs.String("output", "", "path to write backup to")
					fs.Bool("no-search-history", false, "exclude search history from backup")
					return fs
				},
			},
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				output, err := fs.GetString("output")
				if err != nil {
					clilog.LogFlagFailedGet("output", err)
				}
				if output == "" {
					return "--output must be non-empty", nil
				}
				return "", nil
			},
		})
}

func restore() action.Pair {
	return scaffold.NewBasicAction("restore", "restore the system from a backup",
		"Restore the Gravwell system from a backup file.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			path := fs.Arg(0)
			f, err := os.Open(path)
			if err != nil {
				return err.Error(), nil
			}
			defer f.Close()
			if err := connection.Client.Restore(f); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully restored from %s", path), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("backup file path"), nil
				}
				if _, err := os.Stat(fs.Arg(0)); err != nil {
					return "file " + fs.Arg(0) + " does not exist or is not accessible", nil
				}
				return "", nil
			},
		})
}

func listQueries() action.Pair {
	return scaffoldlist.NewListAction("list active searches", "List currently active/recent searches on the system.",
		types.SearchCtrlStatus{},
		func(fs *pflag.FlagSet) ([]types.SearchCtrlStatus, error) {
			if connection.Client.AdminMode() {
				return connection.Client.ListAllSearchStatuses()
			}
			return connection.Client.ListSearchStatuses()
		},
		nil,
		scaffoldlist.Options{
			CommonOptions: scaffold.CommonOptions{
				Use:     "list-queries",
				Aliases: []string{"list-searches"},
			},
			DefaultColumns: []string{"ID", "UserQuery", "State"},
		})
}

func deleteQuery() action.Pair {
	return scaffold.NewBasicAction("delete-query", "delete an active search",
		"Delete an active search by its ID.",
		func(fs *pflag.FlagSet) (string, tea.Cmd) {
			sid := fs.Arg(0)
			if err := connection.Client.DeleteSearch(sid); err != nil {
				return err.Error(), nil
			}
			return fmt.Sprintf("successfully deleted search %s", sid), nil
		},
		scaffold.BasicOptions{
			ValidateArgs: func(fs *pflag.FlagSet) (invalid string, err error) {
				if fs.NArg() != 1 {
					return phrases.Exactly1ArgRequired("search ID"), nil
				}
				return "", nil
			},
		})
}
