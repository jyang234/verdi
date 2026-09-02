package constitutionapp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path custody for the ONE file Propose writes.
//
// Why a component-by-component Lstat walk rather than trusting the path
// grammar: Propose derives its destination lexically (constitutionPolicyDir
// + the kind's directory + the request's own name, all cross-checked through
// policyartifact.ClassifyPolicyPath), but the FILESYSTEM decides where that
// spelling leads. filepath.Join never resolves links, os.MkdirAll and
// internal/atomicfile's temp-then-rename both traverse them without
// complaint, and os.ReadFile reads through one — so a single symlinked
// directory component turns a governed, reviewable, Git-tracked proposal
// into a write somewhere else entirely: outside the checkout, outside Git,
// outside review. The proof therefore has to be taken against what is
// actually on disk.
//
// This mirrors, deliberately and exactly, the discipline already ratified in
// internal/draftmutation (validateDirectoryChain/validateRegularDestination/
// ensureDirectory, the SI-177-adjudicated pattern) and in
// internal/instructionprojection (checkNoSymlinkedComponent). Neither
// exports its own copy, and both live in packages this unit does not own, so
// the discipline is restated here rather than reached into; the extraction of
// one shared seam across all three is a standing candidate, not this unit's
// work.
//
// root itself is deliberately NOT judged: the caller chose it, and on macOS
// an ordinary temp or /var checkout is reached through a symlinked ancestor,
// so judging root would refuse legitimate stores while proving nothing about
// the path the request itself named.

// errUnsafePathComponent classifies every refusal this file produces, so
// Propose can map them all to one typed verdict without string matching.
var errUnsafePathComponent = errors.New("constitutionapp: unsafe proposal path")

func unsafePath(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUnsafePathComponent, fmt.Sprintf(format, args...))
}

// splitRepoRelative splits a repository-relative, slash-spelled path into its
// components, refusing any spelling that is empty, absolute, or carries a
// "."/".." element — such a spelling could name a destination outside the
// checkout before any filesystem question is even asked.
func splitRepoRelative(rel string) ([]string, error) {
	if rel == "" || strings.HasPrefix(rel, "/") {
		return nil, unsafePath("%q is not a repository-relative path", rel)
	}
	segments := strings.Split(rel, "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return nil, unsafePath("path component %q in %q is not a plain name", segment, rel)
		}
	}
	return segments, nil
}

// checkNoSymlinkedComponent proves that no EXISTING component of the
// repository-relative path rel under root is a symlink, and that rel's own
// final component — when it exists at all — is a regular file rather than a
// directory, a device, or anything else Propose must never replace.
//
// A component that does not exist ends the walk cleanly: nothing below a
// missing directory can exist either, and Propose legitimately creates the
// missing ancestors itself (through ensureDirectoryChain, which re-proves
// each component it creates).
func checkNoSymlinkedComponent(root, rel string) error {
	segments, err := splitRepoRelative(rel)
	if err != nil {
		return err
	}
	current := root
	for i, segment := range segments {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			return nil
		}
		if statErr != nil {
			return unsafePath("inspecting path component %q of %q: %v", segment, rel, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return unsafePath("path component %q of %q is a symlink; a constitution artifact is never written or read through a link", segment, rel)
		}
		if i == len(segments)-1 {
			if !info.Mode().IsRegular() {
				return unsafePath("destination %q already exists and is not a regular file (mode %s)", rel, info.Mode())
			}
			return nil
		}
		if !info.IsDir() {
			return unsafePath("path component %q of %q is not a directory", segment, rel)
		}
	}
	return nil
}

// ensureDirectoryChain creates every missing directory component of the
// repository-relative directory path relDir under root ONE COMPONENT AT A
// TIME, re-Lstat'ing each one after creating it. os.MkdirAll cannot be used
// here: it happily traverses a symlinked component, and Mkdir success or
// EEXIST alone is never sufficient authority — a racing process could have
// installed a link between the check and the create, so what is actually on
// disk is re-read and re-judged after every step.
func ensureDirectoryChain(root, relDir string) error {
	segments, err := splitRepoRelative(relDir)
	if err != nil {
		return err
	}
	current := root
	for _, segment := range segments {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkErr := os.Mkdir(current, 0o755); mkErr != nil && !errors.Is(mkErr, os.ErrExist) {
				return unsafePath("creating directory component %q of %q: %v", segment, relDir, mkErr)
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil {
			return unsafePath("inspecting path component %q of %q: %v", segment, relDir, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return unsafePath("path component %q of %q is a symlink or not a directory", segment, relDir)
		}
	}
	return nil
}

// readRegularFile reads path when — and only when — it is an existing
// regular file, reporting (nil, false, nil) for a clean absence. It never
// follows a symlink: checkNoSymlinkedComponent has already refused one at
// this path, and this second Lstat keeps that guarantee local to the read
// rather than inherited from a caller's ordering.
func readRegularFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, unsafePath("%s is a symlink or not a regular file", filepath.Base(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
