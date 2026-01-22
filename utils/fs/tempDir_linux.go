//go:build linux

/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package fs provides utilities related to the OS file system.
package fs

import (
	"os"
)

const (
	temporaryDir         string = `/run/`
	temporaryDirFallBack string = `/dev/shm/`
)

var tempDir = temporaryDir

func init() {
	if f, err := os.Stat(tempDir); err != nil || !f.IsDir() {
		tempDir = temporaryDirFallBack
	}
}

func tempDirImpl() string {
	return tempDir
}
