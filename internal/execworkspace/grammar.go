package execworkspace

import (
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

// executionRoot is the data-zone directory every execution-workspace unit
// and its siblings live under: root/.verdi/data/execution/ (spec §Workspace
// naming; landed by the verdi-store-layout amendment, OD-12).
func executionRoot(root string) string {
	return filepath.Join(root, ".verdi", "data", "execution")
}

// ExecutionRoot is executionRoot exported read-only, mirroring
// internal/wtmanager's WorktreesRoot/WorktreePath exported-wrapper shape
// (CLAUDE.md: shared path assemblers live in one place, never copy-pasted):
// later lanes (materialization, gc) address this root without hardcoding a
// second .verdi/data/execution/ literal.
func ExecutionRoot(root string) string {
	return executionRoot(root)
}

// UnitPath returns workspaceID's unit directory path under root — the
// worktree materialization target itself.
func UnitPath(root, workspaceID string) string {
	return filepath.Join(executionRoot(root), workspaceID)
}

// RequestPath returns workspaceID's immutable request-identity sidecar
// path, the materialization's completion witness (spec §Exact workspace
// materialization).
func RequestPath(root, workspaceID string) string {
	return filepath.Join(executionRoot(root), workspaceID+suffixRequest)
}

// RequestStagingPath returns workspaceID's request-sidecar staging path
// (spec §Workspace naming: "staged inside the OWNING UNIT'S SIBLING
// NAMESPACE, at exactly data/execution/<workspace-id>.request.staging").
func RequestStagingPath(root, workspaceID string) string {
	return filepath.Join(executionRoot(root), workspaceID+suffixRequestStaging)
}

// ReleasedPath returns workspaceID's release marker path (spec §GC slice):
// a zero-byte regular file created O_CREATE|O_EXCL, present-or-absent as
// its entire record.
func ReleasedPath(root, workspaceID string) string {
	return filepath.Join(executionRoot(root), workspaceID+suffixReleased)
}

// LockPath returns workspaceID's per-operation lock path (spec §Workspace
// naming; internal/filelock's Acquire/Release body).
func LockPath(root, workspaceID string) string {
	return filepath.Join(executionRoot(root), workspaceID+suffixLock)
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

// String renders EntryForm for diagnostics.
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
		return "unknown"
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
// "grammar-external" — rather than an error or a panic: a bare sibling
// suffix with an empty base id (".request", ".request.staging",
// ".released", ".lock"), an empty name, or a name containing a path
// separator (never a plausible unit id) are all grammar-external. A name
// ending in one of the four known sibling suffixes maps to its base id and
// that sibling's Form; any other plausible id is a candidate unit
// directory name (FormUnit) — ClassifyEntry does not check whether that
// directory actually exists or is well-formed, only whether its name fits
// the grammar.
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
	if !plausibleUnitID(base) {
		return ClassifiedEntry{}, false
	}
	return ClassifiedEntry{WorkspaceID: base, Form: form}, true
}

func classifiedUnit(name string) (ClassifiedEntry, bool) {
	if !plausibleUnitID(name) {
		return ClassifiedEntry{}, false
	}
	return ClassifiedEntry{WorkspaceID: name, Form: FormUnit}, true
}

// plausibleUnitID reports whether id could possibly be a <workspace-id>:
// non-empty, and free of path separators (a raw directory-entry name from
// ReadDir never legitimately carries one; this guards a classifier fed
// something else by mistake — a nested location or a full path — so it
// fails closed rather than misclassifying).
func plausibleUnitID(id string) bool {
	if id == "" {
		return false
	}
	return !strings.ContainsAny(id, "/\\")
}
