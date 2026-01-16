//go:build windows

/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

package fs

import (
	"os"
	"path/filepath"
)

const (
	temporaryDirFallBack string = `C:\ProgramData\`
)

var tempDir string

func tempDirImpl() string {
	if tempDir != "" {
		return tempDir
	}

	// Use the ProgramData environment variable (typically C:\ProgramData)
	if pd := os.Getenv("ProgramData"); pd != "" {
		tempDir = filepath.Clean(pd) + string(filepath.Separator)
	} else {
		tempDir = temporaryDirFallBack
	}

	return tempDir
}
