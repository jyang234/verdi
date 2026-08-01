// Package specstate derives a specification revision's effective lifecycle
// state — proposed, accepted-pending-build, superseded, closed, or
// unproven — from candidate bytes plus Git history on the configured
// default branch (docs/superpowers/specs/2026-08-01-merge-signals-spec-
// acceptance-design.md, "Merge-Signaled Specification Acceptance"). It is
// the ONE place every later consumer — lint, the CLI, the workbench,
// refindex, residue — routes lifecycle decisions through: no adapter
// reimplements reachability, and none trusts a persisted `status:` field
// alone (the design's "Command behavior" section).
//
// The package speaks only in Git facts and this package's own vocabulary.
// It never decides what a caller should DO with a state — that judgment
// (block a build, render a badge, refuse an edit) belongs entirely to the
// consumer.
package specstate

import "github.com/jyang234/verdi/internal/artifact"

// State is a spec revision's git-derived effective lifecycle state (the
// design's "Authority and derived state" section).
type State string

const (
	// Proposed: the candidate's path, or its exact revision, is not
	// reachable from the configured default branch.
	Proposed State = "proposed"
	// AcceptedPendingBuild: the exact active-zone candidate revision is
	// reachable from the configured default branch and is not closed or
	// superseded.
	AcceptedPendingBuild State = "accepted-pending-build"
	// Superseded: a validated successor on the default branch names this
	// revision as its predecessor.
	Superseded State = "superseded"
	// Closed: the exact revision is reachable from the default branch at
	// its archive-zone path.
	Closed State = "closed"
	// Unproven: the configured default branch, or required Git ancestry,
	// could not be resolved. Disclosures name the missing witness.
	Unproven State = "unproven"
)

// Relation classifies how a candidate's bytes compare to whatever the
// configured default branch currently holds at the candidate's path.
type Relation string

const (
	// RelationNew: the path does not exist on the default branch at all.
	RelationNew Relation = "new"
	// RelationExact: the default branch holds byte-identical content.
	RelationExact Relation = "exact"
	// RelationDiverged: the default branch holds the path, but with
	// different bytes.
	RelationDiverged Relation = "diverged"
	// RelationUnproven: the comparison itself could not be completed.
	RelationUnproven Relation = "unproven"
)

// Branch names the resolved default branch. Name is the branch's short
// name (e.g. "main") — for display and for legacy short-name comparisons
// (the internal/lint.ResolveDefaultBranch compatibility wrapper returns
// exactly this field). Ref is the git-resolvable ref this package's own
// Show/BlobAt/FirstParentBlobLanding/LsTree calls use — a local branch
// name when a matching local branch exists, otherwise an
// "origin/<name>" remote-tracking ref. The two differ whenever the
// process has never checked the default branch out locally (see
// defaultbranch.go).
type Branch struct {
	Name string
	Ref  string
}

// Baseline is the accepted-baseline identity the ratified design names
// (the design's "Authority and derived state" section): the
// specification's path, its blob object id at the landing revision, and
// the first-parent default-branch commit that landed that blob. This
// identity is stable across merge commits, squash merges, and rebase
// merges without consulting the forge.
type Baseline struct {
	Path          string `json:"path"`
	Blob          string `json:"blob"`
	LandingCommit string `json:"commit"`
}

// Candidate is a caller's in-hand revision of a spec: a repo-relative path
// and its exact bytes (working-tree content, a PR head's content, or
// anything else a caller wants classified). Content is never trusted on
// its own — it is always exact-byte compared against what the default
// branch holds at Path, never against a working-tree status field.
type Candidate struct {
	Path    string
	Content []byte
}

// Result is one candidate's projected effective state. Disclosures is
// always sorted for deterministic output; it is non-empty exactly when
// State could not be fully proven, or when a legacy artifact needed a
// compatibility note.
type Result struct {
	State       State     `json:"state"`
	Relation    Relation  `json:"relation"`
	Baseline    *Baseline `json:"baseline"`
	Disclosures []string  `json:"disclosures"`
}

// ArtifactStatus maps Result to the legacy per-kind artifact.Status
// vocabulary spec frontmatter has always carried, for display-only
// consumers (e.g. internal/refindex) that still speak that vocabulary. It
// is a read-only projection, never a decision seam: every acceptance
// decision in this module — and every consumer's — reads Result.State,
// never this string. Unproven has no legacy equivalent; it maps to the
// literal string "unproven" rather than to any of the four vocabulary
// values a caller might mistake for a proven verdict.
func (r Result) ArtifactStatus() artifact.Status {
	switch r.State {
	case AcceptedPendingBuild:
		return artifact.Status("accepted-pending-build")
	case Superseded:
		return artifact.Status("superseded")
	case Closed:
		return artifact.Status("closed")
	case Proposed:
		return artifact.Status("draft")
	default: // Unproven
		return artifact.Status("unproven")
	}
}
