package execworkspace

import (
	"errors"
	"fmt"
	"os"
)

// PathKind is the closed enum LstatType types a filesystem path into,
// never following a symlink (spec: "BOTH PATHS ARE EXAMINED WITH LSTAT,
// never a following stat").
type PathKind int

const (
	// PathUnknown is the zero value. It is returned only alongside a
	// non-nil error and is never a valid, standalone classification — a
	// caller must check the error before interpreting the kind, exactly as
	// with any other Go (value, error) pair. Keeping it distinct from
	// PathAbsent (rather than making PathAbsent the zero value) means a
	// caller that forgets to check the error can never silently observe a
	// "clean absent" result for a real lstat failure.
	PathUnknown PathKind = iota
	// PathAbsent means lstat found nothing at path (a clean not-found).
	PathAbsent
	// PathRegular means path names a regular file.
	PathRegular
	// PathDir means path names a real directory.
	PathDir
	// PathSymlink means path names a symlink — its target is never
	// resolved or reported; the symlink itself is the classified object.
	PathSymlink
	// PathOther means path names something that is none of the above
	// (a device, socket, named pipe, etc.).
	PathOther
)

// String renders PathKind for diagnostics.
func (k PathKind) String() string {
	switch k {
	case PathUnknown:
		return "unknown"
	case PathAbsent:
		return "absent"
	case PathRegular:
		return "regular"
	case PathDir:
		return "dir"
	case PathSymlink:
		return "symlink"
	case PathOther:
		return "other"
	default:
		return fmt.Sprintf("execworkspace.PathKind(%d)", int(k))
	}
}

// LstatType lstat-types path into PathKind, NEVER following a symlink —
// this is the ONLY lstat seam later lanes (materialization, release, gc)
// use for typing a unit path, a marker path, a request path, or a staging
// path (spec: "BOTH PATHS ARE EXAMINED WITH LSTAT, never a following
// stat").
//
// A clean not-found (os.IsNotExist) returns (PathAbsent, nil). Any other
// lstat failure — permission denied, ENOTDIR from a non-directory path
// component, or anything else — returns (PathUnknown, a non-nil error) and
// must NEVER be read as absence: the spec's fail-closed rule for
// malformed/unreadable paths applies to this failure, not the ordinary
// not-found case. Callers must check the error before interpreting the
// returned kind.
func LstatType(path string) (PathKind, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return PathAbsent, nil
		}
		// os.Lstat's error is an *os.PathError already naming both the
		// operation ("lstat") and the path, so this wrap adds only the
		// package context — repeating either would double-print it.
		return PathUnknown, fmt.Errorf("execworkspace: typing path: %w", err)
	}

	mode := fi.Mode()
	switch {
	case mode&os.ModeSymlink != 0:
		return PathSymlink, nil
	case mode.IsDir():
		return PathDir, nil
	case mode.IsRegular():
		return PathRegular, nil
	default:
		return PathOther, nil
	}
}
