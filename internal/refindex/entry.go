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
}
