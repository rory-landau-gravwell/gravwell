package users

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravwell/gravwell/v4/gwcli/action"
	"github.com/gravwell/gravwell/v4/gwcli/clilog"
	"github.com/gravwell/gravwell/v4/gwcli/connection"
	"github.com/gravwell/gravwell/v4/gwcli/internal/listitem"
	"github.com/gravwell/gravwell/v4/gwcli/mother"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet"
	ft "github.com/gravwell/gravwell/v4/gwcli/stylesheet/flagtext"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/hotkeys"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/multiselectlist"
	"github.com/gravwell/gravwell/v4/gwcli/utilities/treeutils"
	"github.com/gravwell/gravwell/v4/ingest/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// This file implements the interactive change-password action for admin users.

type cpStage uint

const (
	cpStgSelectUser cpStage = iota // select a user
	cpStgPassword                  // enter new password
	cpStgDone                      // completed
)

func changePassword() action.Pair {
	cmd := treeutils.GenerateAction("change-password", "change a user's password",
		"Change a user's password without requiring their current password. "+
			"Non-interactive mode can take the password in clear as --new-password. "+
			"If you prefer to keep the password out of your history, consider using --new-passfile",
		nil,
		func(c *cobra.Command, args []string) error {
			uid, err := c.Flags().GetInt32("uid")
			if err != nil {
				clilog.GetFlag(err)
			}
			password, err := c.Flags().GetString("password")
			if err != nil {
				clilog.GetFlag(err)
			}

			// if both flags are provided, run non-interactively
			if uid != 0 && password != "" {
				if err := connection.Client.AdminChangePass(uid, password); err != nil {
					return err
				}
				fmt.Fprintf(c.OutOrStdout(), "successfully changed password for user %d\n", uid)
				return nil
			}

			// otherwise, boot interactive mode
			ni, err := c.Flags().GetBool(ft.NoInteractive.Name())
			if err != nil {
				clilog.GetFlag(err)
				ni = true
			}
			if !ni {
				return mother.Spawn(c.Root(), c, args)
			}
			if uid == 0 {
				return errors.New("--uid must be set and nonzero")
			}
			return errors.New("--password must be non-empty")
		},
	)

	cmd.Flags().AddFlagSet(cpFlags())

	return action.NewPair(cmd, &changePasswordModel{})
}

func cpFlags() *pflag.FlagSet {
	fs := &pflag.FlagSet{}
	ft.UID.Register(fs)
	fs.String("new-password", "", "the new password to assign the user. Mutually exclusive with --new-passfile.")
	fs.String("new-passfile", "", "reads the users new password from the given path. Mutually exclusive with --new-password.")
	return fs
}

// getPasswordFromFlags attempts to fetch the new password from --new-password and --new-passfile.
// Returns an error if both are set are passfile was set and failed to read from.
// Return "", nil if neither is set.
func getPasswordFromFlags(fs *pflag.FlagSet) (password string, err error) {
	pass, err := fs.GetString("new-password")
	if err != nil {
		clilog.GetFlag(err)
	}
	pf, err := fs.GetString("new-passfile")
	if err != nil {
		clilog.GetFlag(err)
	}
	// we don't set defaults here, so any value in either means changed
	if pass != "" && pf != "" {
		return "", errors.New("--new-password and --new-passfile are mutually exclusive")
	}
	if pass != "" {
		return pass, nil
	}

	if pf != "" {
		b, err := os.ReadFile(pf)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	return "", nil
}

//#region interactive

type changePasswordModel struct {
	users multiselectlist.Model[int32]
	ti    textinput.Model
	stage cpStage

	selectedUID      int32
	selectedUsername string
}

func (m *changePasswordModel) Update(msg tea.Msg) (cmd tea.Cmd) {
	switch m.stage {
	case cpStgSelectUser:
		m.users, cmd = m.users.Update(msg)
		if m.users.Done() {
			selected := m.users.GetSelectedItems()
			if len(selected) != 1 {
				m.users.Undone()
				return m.users.NewStatusMessage("select exactly 1 user")
			}
			m.selectedUID = selected[0].ID()
			m.selectedUsername = selected[0].Title()
			m.stage = cpStgPassword
			m.ti.Focus()
			return textinput.Blink
		}
	case cpStgPassword:
		// handle enter to submit
		if hotkeys.Match(msg, hotkeys.Invoke) {
			password := m.ti.Value()
			if password == "" {
				return nil // ignore empty submissions
			}
			if err := connection.Client.AdminChangePass(m.selectedUID, password); err != nil {
				clilog.Writer.Error("failed to change password", log.KV("uid", m.selectedUID), log.KVErr(err))
				m.stage = cpStgDone
				return tea.Printf("failed to change password for user '%s': %v", m.selectedUsername, err)
			}
			m.stage = cpStgDone
			return tea.Printf("successfully changed password for user '%s' (ID: %d)", m.selectedUsername, m.selectedUID)
		}
		// handle esc to cancel
		if hotkeys.Match(msg, hotkeys.SoftQuit) {
			m.stage = cpStgDone
			return tea.Println("cancelled")
		}
		m.ti, cmd = m.ti.Update(msg)
	}
	return cmd
}

func (m *changePasswordModel) View() string {
	switch m.stage {
	case cpStgSelectUser:
		return m.users.View()
	case cpStgPassword:
		return fmt.Sprintf("New password for '%s':\n%s\n\n  %s",
			m.selectedUsername,
			m.ti.View(),
			stylesheet.Cur.DisabledText.Render("↲ submit • esc cancel"))
	}
	return ""
}

func (m *changePasswordModel) Done() bool {
	return m.stage == cpStgDone
}

func (m *changePasswordModel) Reset() error {
	m.users = multiselectlist.Model[int32]{}
	m.ti = textinput.Model{}
	m.stage = cpStgSelectUser
	m.selectedUID = 0
	m.selectedUsername = ""
	return nil
}

func (m *changePasswordModel) SetArgs(_ *pflag.FlagSet, tokens []string, width, height int) (invalid string, onStart tea.Cmd, err error) {

	// attach and check flags
	fs := cpFlags()
	if err := fs.Parse(tokens); err != nil {
		return "", nil, err
	}

	// if a password and UID were provided, we can operate without interactive mode.
	uid, err := fs.GetInt32(ft.UID.Name())
	if err != nil {
		clilog.GetFlag(err)
	}
	pass, err := getPasswordFromFlags(fs)
	if err != nil {
		return err.Error(), nil, nil
	}
	if uid != 0 && pass != "" {
		if err := connection.Client.AdminChangePass(m.selectedUID, pass); err != nil {
			return "", nil, err
		}
		m.stage = cpStgDone

		return "", tea.Printf("successfully changed password for user ID: %d", uid), nil
	}

	m.ti.SetValue(pass)

	// fetch all users
	users, err := connection.Client.ListUsers(nil)
	if err != nil {
		clilog.Writer.Error("failed to get the list of users", log.KV("error", err))
		return "", nil, fmt.Errorf("failed to get the list of users")
	}
	var itms = make([]multiselectlist.SelectableItem[int32], 0, len(users.Results))
	for _, user := range users.Results {
		itms = append(itms, &listitem.User{
			ID_:      user.ID,
			Username: user.Username,
			Name:     user.Name,
			Email:    user.Email,
			Admin:    user.Admin,
		})
	}
	itms = slices.Clip(itms)
	if len(itms) == 0 {
		return "there are no users", nil, nil
	}
	m.users = multiselectlist.New(itms, width, height, multiselectlist.Options{})
	m.users.StatusMessageLifetime = stylesheet.StatusMessageLifetime
	m.users.StatusMessageOnSelect = true
	m.users.Title = "Select a user"

	// set up password text input
	m.ti = stylesheet.NewTI("", false)
	m.ti.EchoMode = textinput.EchoPassword
	m.ti.Placeholder = "enter new password"
	m.ti.Width = 40
	m.ti.Blur()

	return "", nil, nil
}

//#endregion interactive
