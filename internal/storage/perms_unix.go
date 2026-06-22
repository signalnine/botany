//go:build unix

package storage

import (
	"os"
	"syscall"
)

// fileOwnedByUs reports whether the given file is owned by the current uid, used
// to decide whether to relax permissions on shared-host data files.
func fileOwnedByUs(info os.FileInfo) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(st.Uid) == os.Getuid()
}
