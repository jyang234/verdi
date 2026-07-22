//go:build linux

package main

import "golang.org/x/sys/unix"

// renameExclusive atomically renames oldpath to newpath, failing — with an
// error that errors.Is(err, os.ErrExist) matches — if newpath already
// exists, via linux's RENAME_NOREPLACE flag to renameat2(2). This is the
// load-bearing arm of verdi init's promotion backstop (spec/init-wizard
// ac-1): on ext4-class POSIX filesystems a bare rename SUCCEEDS over an
// empty destination directory (silently replacing it), so only the
// NOREPLACE flag refuses an empty .verdi/ that raced into the
// check-to-rename window. AT_FDCWD resolves relative or absolute paths
// against the current directory, exactly as os.Rename does.
func renameExclusive(oldpath, newpath string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldpath, unix.AT_FDCWD, newpath, unix.RENAME_NOREPLACE)
}
