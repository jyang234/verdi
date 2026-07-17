package refindex

import "github.com/jyang234/verdi/internal/disclosure"

// Source is a closed, always-populated four-value enum naming where an
// Entry's ref was read from (dc-3). It is itself the mechanism that
// satisfies parent spec/workbench-directory dc-5's "each entry discloses
// its source" — a plain field, never omitted or defaulted away, so a
// remote-only or default-branch entry's sourcing is disclosed simply by a
// consumer rendering this value; no disclosure.Disclosure wrapper is
// needed or created for that ordinary case (dc-3's "two distinct
// disclosures").
type Source string

const (
	// SourceDefault names an entry read from the default branch's own tree
	// (dc-4) — neither a local nor a remote-tracking design ref.
	SourceDefault Source = "default"
	// SourceLocal names a design-branch entry that exists only as a local
	// refs/heads/design/* ref.
	SourceLocal Source = "local"
	// SourceRemote names a design-branch entry that exists only as a
	// remote-tracking refs/remotes/origin/design/* ref (never fetched down
	// to a local branch, or a teammate's still-open draft).
	SourceRemote Source = "remote"
	// SourceBoth names a design-branch entry whose branch exists both
	// locally and as a remote-tracking ref — a single entry, never two
	// (ac-2). Reserved for a genuinely still-open (unmerged) branch; a
	// merged leftover contributes no design-branch entry at all (dc-5).
	SourceBoth Source = "both"
)

// StatusGroup is the closed, four-value grouping vocabulary parent spec
// workbench-directory dc-2 ratifies: "drafts in progress / accepted-
// pending-build / active components / terminal — the status is the
// distinction, never the address."
type StatusGroup string

const (
	StatusGroupDraftsInProgress     StatusGroup = "drafts-in-progress"
	StatusGroupAcceptedPendingBuild StatusGroup = "accepted-pending-build"
	StatusGroupActiveComponents     StatusGroup = "active-components"
	StatusGroupTerminal             StatusGroup = "terminal"
)

// Zone names which of the default branch's two spec zones an Entry's
// content was read from (spec/home-status-glance dc-2, ADJ-32's computed,
// in-memory zone distinction): ZoneActive for .verdi/specs/active/*,
// ZoneArchive for .verdi/specs/archive/*. Every design-branch entry —
// ordinary or disclosed alike — is unconditionally ZoneActive: a design
// branch's draft spec is read only from .verdi/specs/active/ (dc-4's own
// computeDesignBranchEntries specPath), never from an archive zone of its
// own, so this field is never derived from that branch's content any more
// than StatusGroup is (ac-3's identical override).
//
// This is a purely additive, in-memory computed signal — never persisted,
// never a frontmatter field (home-status-glance dc-1 upheld) — and it is
// consumed today ONLY by the home page's status-glance section. Every
// other refindex consumer (internal/workbench/directory.go's exhaustive
// render; this package's own tests) reads none of it and is unaffected by
// its presence; ComputeIndex's production code paths set it explicitly on
// every Entry they construct, so the zero value below never appears on a
// real entry. A test fixture built elsewhere that leaves Zone unset gets
// the zero value, which the glance's own fail-closed reading (CLAUDE.md:
// "unknown enum values fail closed") treats as "not active" — excluded
// from the glance rather than assumed servable, the same conservative
// posture ADJ-32 already chose for a genuine archive-zone entry.
type Zone string

const (
	ZoneActive  Zone = "active"
	ZoneArchive Zone = "archive"
)

// Entry is one row of the computed directory index (dc-3): a default-branch
// spec, or a design branch's draft (ordinary or degraded).
type Entry struct {
	// Ref is the canonical kind/name identity ("spec/<name>"). For a
	// design-branch entry it is always derived from the branch's own name
	// (the part after "design/"), never from the draft's own frontmatter —
	// the only derivation that is defined even when no content is readable
	// at all (ac-4's degraded case).
	Ref string
	// Source names where this entry's ref was read from — always
	// populated, never a zero value (dc-3).
	Source Source
	// StatusGroup is the four-bucket group this entry renders under.
	// Default-sourced entries derive it from their own frontmatter status
	// field (mapStatusGroup); every design-branch entry (ordinary or
	// degraded) is unconditionally StatusGroupDraftsInProgress (ac-3) —
	// never derived from that branch's content, readable or not.
	StatusGroup StatusGroup
	// SpecStatus is the raw frontmatter status field, where a spec was
	// readable at this entry's ref; empty otherwise (dc-3). Always empty
	// for a Disclosed (degraded) entry, since there was no content to read.
	SpecStatus string
	// Disclosed is non-nil only for a degraded entry whose content could
	// not be read at all (ac-4's no-draft-spec case) — never used for an
	// ordinary remote-only or default-branch entry, whose content is
	// present, just sourced from a particular place (dc-3).
	Disclosed *disclosure.Disclosure
	// Zone names which default-branch zone this entry's content was read
	// from — see the Zone type's own doc comment. Always populated by
	// ComputeIndex's production code paths, for every Entry kind alike.
	Zone Zone
}
