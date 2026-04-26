package internal

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ErrWebDAVHandledByDesktopClient is returned when the local data file
// lives inside a Nextcloud desktop-client sync folder, so the CLI should
// not also push to WebDAV — doing both would race and create "conflicted
// copy" files.
var ErrWebDAVHandledByDesktopClient = errors.New("local data file is inside a Nextcloud sync folder; desktop client handles sync")

// LocalPathInNextcloudSyncFolder reports whether dataPath resolves to a
// location inside the user's Nextcloud desktop-client sync folder
// (~/Nextcloud). When true, the desktop client already propagates writes
// to the server, and the CLI should not perform its own WebDAV PUTs
// against the same file.
//
// This is a Nextcloud-with-desktop-client-specific quirk and lives
// outside the Backend abstraction.
func LocalPathInNextcloudSyncFolder(dataPath string) bool {
	if dataPath == "" {
		return false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(dataPath)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(filepath.Join(home, "Nextcloud"), abs)
	if err != nil {
		return false
	}
	return rel != "." && !strings.HasPrefix(rel, "..")
}
