package instructionprojection

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// checkNoSymlinkedComponent proves that no EXISTING component of rel
// under root is a symlink, walking the path one segment at a time with
// os.Lstat (which never follows a link). includeFinal decides whether
// rel's own last component is part of the proof.
//
// Why the walk, rather than trusting the path grammar: the constitution
// declares a repo-relative, escape-free path, but the FILESYSTEM decides
// where that path leads. internal/atomicfile MkdirAlls and renames
// through a symlinked ancestor without complaint, and os.ReadFile reads
// through one, so a single symlinked directory turns a governed
// projection into a write (and a verification read) somewhere else
// entirely — outside the repository the constitution governs, outside
// review, outside Git. A projection is a one-way output of the
// constitution (AC-1/DC-1); a projection whose location is decided by a
// link nobody reviewed is not that. Fail closed naming the component and
// the path (CO-1).
//
// A component that does not exist is fine and ends the walk: nothing
// below a missing directory can exist either, and Generate legitimately
// creates missing ancestors itself. Only what is already on disk is
// judged.
//
// root itself is deliberately NOT examined: the caller chose it (and on
// macOS a perfectly ordinary temp or /var root is reached through a
// symlink), so judging it would refuse legitimate stores while proving
// nothing about the constitution's own declared paths.
func checkNoSymlinkedComponent(root, rel string, includeFinal bool) error {
	segs := strings.Split(rel, "/")
	if !includeFinal {
		segs = segs[:len(segs)-1]
	}
	cur := root
	for _, seg := range segs {
		cur = filepath.Join(cur, seg)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("instructionprojection: %s: statting path component %q: %w", rel, seg, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("instructionprojection: %s: path component %q is a symlink; a projection is this repository's own generated file and is never written or read through a link", rel, seg)
		}
	}
	return nil
}

// checkProjectionPathsSafe proves every path a projection run touches —
// each adapter's managed files and each adapter's own manifest — is free
// of symlinked components before anything is written or read.
// includeFinal is false for Verify, whose per-file passes classify a
// symlinked FINAL path as their own fail-closed finding (a verdict, with
// the offending path named in the report) rather than an operational
// error; it is true for Generate, which must never write to a path that
// is already a link.
func checkProjectionPathsSafe(root string, adapters []policyartifact.Adapter, includeFinal bool) error {
	for _, a := range adapters {
		for _, rel := range a.Managed {
			if err := checkNoSymlinkedComponent(root, rel, includeFinal); err != nil {
				return fmt.Errorf("adapter %s: %w", a.ID, err)
			}
		}
		if err := checkNoSymlinkedComponent(root, adapterManifestRelPath(a.ID), includeFinal); err != nil {
			return fmt.Errorf("adapter %s: %w", a.ID, err)
		}
	}
	return nil
}
