//go:build darwin

package main

import "golang.org/x/sys/unix"

// renameExclusive atomically renames oldpath to newpath, failing — with an
// error that errors.Is(err, os.ErrExist) matches — if newpath already
// exists, via darwin's RENAME_EXCL flag to renamex_np(2). This is the
// darwin arm of verdi init's promotion backstop (spec/init-wizard ac-1):
// unlike a bare os.Rename it refuses ANY existing destination, empty
// directory included; unlike os.Mkdir-then-rename it is a single atomic
// operation that never breaks the happy path on filesystems where a rename
// over an existing empty directory fails with EEXIST (APFS among them).
func renameExclusive(oldpath, newpath string) error {
	return unix.RenamexNp(oldpath, newpath, unix.RENAME_EXCL)
}
