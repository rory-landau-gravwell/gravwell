// Code generated by scriggo command. DO NOT EDIT.

package plugin

import (
	"github.com/open2b/scriggo/native"
)

func initLegacy(packages native.Packages) {
	// "github.com/gravwell/gravwell/v3/ingest"
	packages["github.com/gravwell/gravwell/v3/ingest"] = packages["github.com/gravwell/gravwell/v4/ingest"]

	// "github.com/gravwell/gravwell/v3/ingest/config"
	packages["github.com/gravwell/gravwell/v3/ingest/config"] = packages["github.com/gravwell/gravwell/v4/ingest/config"]

	// "github.com/gravwell/gravwell/v3/ingest/entry"
	packages["github.com/gravwell/gravwell/v3/ingest/entry"] = packages["github.com/gravwell/gravwell/v4/ingest/entry"]
}
