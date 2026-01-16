/*************************************************************************
 * Copyright 2026 Gravwell, Inc. All rights reserved.
 * Contact: <legal@gravwell.io>
 *
 * This software may be modified and distributed under the terms of the
 * BSD 2-clause license. See the LICENSE file for details.
 **************************************************************************/

// Package fs provides utilities related to the OS file system.
package fs

// TempDir returns a consistent platform-specific temporary directory for Gravwell.
// The returned path is guaranteed to be the same across multiple runs on the same system.
//
// On Linux, this returns /run/ (or /dev/shm/ as a fallback if /run/ doesn't exist).
// Linux systems often mount /run/ and /dev/shm/ as RAM-backed tmpfs filesystems
// for better performance.
//
// On macOS, this returns /tmp/. 
//
// On Windows, this returns the ProgramData directory (typically C:\ProgramData).
// See: https://learn.microsoft.com/en-us/windows/win32/shell/knownfolderid#FOLDERID_ProgramData
//
// Windows and mac don't have RAM-backed temporary directories like Linux's /run/ or /dev/shm/.
func TempDir() string {
	return tempDirImpl()
}
