//go:build linux

package disclosureview

import (
	"io/fs"
	"syscall"
	"time"
)

// sysStamp returns the change time, inode and device of a stat result.
func sysStamp(fi fs.FileInfo) (ctime time.Time, ino, dev uint64, ok bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return time.Time{}, 0, 0, false
	}
	return time.Unix(st.Ctim.Unix()), st.Ino, uint64(st.Dev), true
}
