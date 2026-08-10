package execworkspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Sibling-form suffixes recognized under data/execution/ (spec §Workspace
// naming, §GC slice). Checked longest-and-most-specific first in
// ClassifyEntry, though ".request.staging" never actually shares a
// trailing-suffix collision with ".request" (its own suffix is
// ".staging"): the ordering is kept explicit for legibility, not because it
// changes any classification.
const (
	suffixRequestStaging = ".request.staging"
	suffixRequest        = ".request"
	suffixReleased       = ".released"
	suffixLock           = ".lock"
)

// ExecutionRoot is the data-zone directory every execution-workspace unit
// and its siblings live under: root/.verdi/data/execution/ (spec
// /execution-workspace §Workspace naming, addressing
// `data/execution/<workspace-id>`; the directory itself is landed by the
// verdi-store-layout amendment's `execution/<workspace-id>/` and
// `execution/<workspace-id>.lock` rows, which trace to OD-6 — "naming owned
// by spec/execution-workspace").
//
// It is exported as the ONE home for this path (CLAUDE.md: shared path
// assemblers live in one place, never copy-pasted), so later lanes
// (materialization, gc) address this root without hardcoding a second
// .verdi/data/execution/ literal, and every assembler below is built on it.
func ExecutionRoot(root string) string {
	return filepath.Join(root, ".verdi", "data", "execution")
}

// The five assemblers below (UnitPath, RequestPath, RequestStagingPath,
// ReleasedPath, LockPath) are PURE GRAMMAR HELPERS: they join whatever
// workspaceID they are handed and validate NOTHING, so an id carrying a
// path segment ("../writer") resolves out of data/execution/ entirely.
// Every caller that turns one of these paths into a FILESYSTEM EFFECT must
// therefore gate its raw id on ValidWorkspaceID first — Releaser.Release
// does so on entry, and GC and Materializer only ever pass ids that are
// already ValidWorkspaceID-proven (ClassifyEntry /
// classifyResolvedAdminPath for the former, Identity.WorkspaceID for the
// latter).

// UnitPath returns workspaceID's unit directory path under root — the
// worktree materialization target itself.
func UnitPath(root, workspaceID string) string {
	return filepath.Join(ExecutionRoot(root), workspaceID)
}

// RequestPath returns workspaceID's immutable request-identity sidecar
// path, the materialization's completion witness (spec §Exact workspace
// materialization).
func RequestPath(root, workspaceID string) string {
	return filepath.Join(ExecutionRoot(root), workspaceID+suffixRequest)
}

// RequestStagingPath returns workspaceID's request-sidecar staging path
// (spec §Workspace naming: "staged inside the OWNING UNIT'S SIBLING
// NAMESPACE, at exactly data/execution/<workspace-id>.request.staging").
func RequestStagingPath(root, workspaceID string) string {
	return filepath.Join(ExecutionRoot(root), workspaceID+suffixRequestStaging)
}

// ReleasedPath returns workspaceID's release marker path (spec §GC slice):
// a zero-byte regular file created O_CREATE|O_EXCL, present-or-absent as
// its entire record.
func ReleasedPath(root, workspaceID string) string {
	return filepath.Join(ExecutionRoot(root), workspaceID+suffixReleased)
}

// LockPath returns workspaceID's per-operation lock path (spec §Workspace
// naming; internal/filelock's Acquire/Release body).
func LockPath(root, workspaceID string) string {
	return filepath.Join(ExecutionRoot(root), workspaceID+suffixLock)
}

// EntryForm identifies which unit-grammar form a classified directory
// entry under data/execution/ is.
type EntryForm int

const (
	// FormUnit is the unit directory itself: <workspace-id>/.
	FormUnit EntryForm = iota
	// FormRequest is the immutable request-identity sidecar:
	// <workspace-id>.request.
	FormRequest
	// FormRequestStaging is the sidecar's staging temporary:
	// <workspace-id>.request.staging.
	FormRequestStaging
	// FormReleased is the release marker: <workspace-id>.released.
	FormReleased
	// FormLock is the per-operation lock file: <workspace-id>.lock.
	FormLock
)

// String renders EntryForm for diagnostics, with a SELF-NAMING fallback for
// a value outside the closed set — the same shape PathKind.String and
// GCOutcome.String already use in this package, so a diagnostic can never
// print a bare, unattributable "unknown".
func (f EntryForm) String() string {
	switch f {
	case FormUnit:
		return "unit"
	case FormRequest:
		return "request"
	case FormRequestStaging:
		return "request-staging"
	case FormReleased:
		return "released"
	case FormLock:
		return "lock"
	default:
		return fmt.Sprintf("execworkspace.EntryForm(%d)", int(f))
	}
}

// ClassifiedEntry is ClassifyEntry's result: the unit id a directory entry
// under data/execution/ belongs to, and which grammar form it is.
type ClassifiedEntry struct {
	WorkspaceID string
	Form        EntryForm
}

// ClassifyEntry classifies name — a single raw directory-entry NAME
// (never a path) taken directly from a data/execution/ listing — against
// the unit grammar (spec §Workspace naming, §GC slice). It returns
// (ClassifiedEntry{}, false) for anything the grammar does not recognize —
// "grammar-external", the scan-level state the GC slice DISCLOSES AND KEEPS
// ("The slice never deletes what it cannot classify") — rather than an
// error or a panic.
//
// A name ending in one of the four known sibling suffixes maps to its base
// id and that sibling's Form; any other name is a candidate unit directory
// name (FormUnit). In BOTH cases the id part must satisfy ValidWorkspaceID:
// an ordinary file ("README", ".DS_Store", "notes.txt"), a bare sibling
// suffix with an empty base (".request", ".lock"), a dot entry (".", ".."),
// a name carrying a path separator, and any id that does not end in the
// normative --<sha12>[-p<patch12>] group are all grammar-external. Nothing
// else in this package or a later lane may re-derive that shape test.
//
// ClassifyEntry does not check whether the named object exists or is
// well-formed on disk, only whether its NAME fits the grammar.
func ClassifyEntry(name string) (ClassifiedEntry, bool) {
	switch {
	case strings.HasSuffix(name, suffixRequestStaging):
		return classifiedSibling(name, suffixRequestStaging, FormRequestStaging)
	case strings.HasSuffix(name, suffixReleased):
		return classifiedSibling(name, suffixReleased, FormReleased)
	case strings.HasSuffix(name, suffixLock):
		return classifiedSibling(name, suffixLock, FormLock)
	case strings.HasSuffix(name, suffixRequest):
		return classifiedSibling(name, suffixRequest, FormRequest)
	default:
		return classifiedUnit(name)
	}
}

func classifiedSibling(name, suffix string, form EntryForm) (ClassifiedEntry, bool) {
	base := strings.TrimSuffix(name, suffix)
	if !ValidWorkspaceID(base) {
		return ClassifiedEntry{}, false
	}
	return ClassifiedEntry{WorkspaceID: base, Form: form}, true
}

func classifiedUnit(name string) (ClassifiedEntry, bool) {
	if !ValidWorkspaceID(name) {
		return ClassifiedEntry{}, false
	}
	return ClassifiedEntry{WorkspaceID: name, Form: FormUnit}, true
}

// ValidWorkspaceID reports whether id has the normative <workspace-id> shape
// (spec §Workspace naming): `<run-slug>--<sha12>` for the exact-SHA shape and
// `<run-slug>--<sha12>-p<patch12>` for the base-plus-patch shape, where
// <run-slug> is non-empty, <sha12> and <patch12> are exactly 12 LOWERCASE hex
// digits, and every byte of id lies in the store's normative slug alphabet
// [a-z0-9._-] (verdi-store-layout §Directory layout notes, "Ref slugging is
// normative" — the alphabet store.RefSlug maps into, plus the "--" and "-p"
// separators this scheme adds, all of which are already inside it).
//
// It is the ONE shape test for a <workspace-id>, used by ClassifyEntry to
// separate grammar-external entries from unit-grammar ones and satisfied by
// every id Identity.WorkspaceID returns.
//
// PARSING NOTE. RefSlug maps "/" to "--", so a run slug may itself contain
// "--" ("a--b--abcdef012345" is a valid id whose slug is "a--b"). The sha
// group is therefore the LAST "--<12 hex>" group, found by SUFFIX parsing —
// never by splitting on the first "--".
func ValidWorkspaceID(id string) bool {
	if !slugAlphabetOnly(id) {
		return false
	}
	// Base-plus-patch first: strip a trailing "-p<patch12>", then require
	// the remainder to still end in "--<sha12>" with a non-empty slug.
	if rest, ok := trimHexGroup(id, "-p"); ok {
		if slug, ok := trimHexGroup(rest, "--"); ok && slug != "" {
			return true
		}
	}
	slug, ok := trimHexGroup(id, "--")
	return ok && slug != ""
}

// hexGroupLen is the width of both truncated hex groups in a
// <workspace-id>: the 12-hex commit abbreviation and the 12-hex patch
// abbreviation (spec §Workspace naming).
const hexGroupLen = 12

// trimHexGroup reports whether s ends with sep immediately followed by
// exactly hexGroupLen lowercase hex digits, returning the part of s that
// precedes sep. It examines only the tail of s, so a separator appearing
// earlier in the slug never confuses it.
func trimHexGroup(s, sep string) (string, bool) {
	if len(s) < len(sep)+hexGroupLen {
		return "", false
	}
	hexAt := len(s) - hexGroupLen
	if s[hexAt-len(sep):hexAt] != sep {
		return "", false
	}
	if !lowerHexOnly(s[hexAt:]) {
		return "", false
	}
	return s[:hexAt-len(sep)], true
}

// lowerHexOnly reports whether every byte of s is a lowercase hex digit.
// Uppercase is rejected: a <workspace-id>'s digests are canonical lowercase,
// matching validateCommitSHA's discipline for the full form.
func lowerHexOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

// slugAlphabetOnly reports whether every byte of s lies in the slug alphabet
// [a-z0-9._-]. This rejects an empty string, every path separator ("/" and
// "\" — a raw ReadDir name never legitimately carries one, so a classifier
// handed a nested location or a full path fails closed), uppercase, spaces,
// "_", and every other byte RefSlug would itself have mapped away.
func slugAlphabetOnly(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '-':
		default:
			return false
		}
	}
	return true
}
