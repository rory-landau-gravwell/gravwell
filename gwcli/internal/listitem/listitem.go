// Package listitem defines common list types so we don't have a bunch of duplicate structs floating around any time list.Model or
// multiselectlist.Model are used.
package listitem

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/gravwell/gravwell/v4/gwcli/stylesheet/multiselectlist"
)

type User struct {
	Selected_ bool
	ID_       int32

	Username string
	Name     string
	Email    string
	Admin    bool
}

var _ multiselectlist.SelectableItem[int32] = &User{}
var _ list.Item = &User{}

// FilterValue filters on the concat of ttl and desc.
func (i User) FilterValue() string {
	var adm string
	if i.Admin {
		adm = "admin"
	}
	return adm + fmt.Sprintf("%d %v %v", i.ID_, i.Username, i.Name)
}

func (i User) Title() string {
	return i.Username
}

func (i User) ID() int32 {
	return i.ID_
}

func (i User) Description() string {
	var sb strings.Builder

	if i.Admin {
		sb.WriteString("(admin) ")
	}
	fmt.Fprintf(&sb, "(ID: %d) %s (%s)", i.ID_, i.Name, i.Email)

	return sb.String()
}

func (i *User) SetSelected(selected bool) {
	i.Selected_ = selected
}

func (i User) Selected() bool {
	return i.Selected_
}
