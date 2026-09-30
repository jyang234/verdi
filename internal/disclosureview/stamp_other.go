//go:build !darwin && !linux

package disclosureview

import (
	"io/fs"
	"time"
)

// sysStamp reports no stamp on platforms whose stat result this package
// does not read, so the cache key is uncomputable there and every call
// enumerates afresh.
func sysStamp(fs.FileInfo) (ctime time.Time, ino, dev uint64, ok bool) {
	return time.Time{}, 0, 0, false
}
