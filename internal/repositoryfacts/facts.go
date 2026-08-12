// Package repositoryfacts defines the shared, canonically encoded
// repository-identity fact shape SI-85 extracts from internal/journey:
// remote origin, branch, HEAD, configured default branch, their
// relationship, dirty/staged posture, managed-worktree identity, and
// which checkout state an evaluated target's bytes came from
// (context-compiler authority design §4). internal/journey and a later
// internal/contextcompile task both consume this package so the two
// consumers cannot silently diverge in fact shape or canonical
// remote-identity rule.
//
// This file holds only the schema and its known/value invariants;
// gather.go is the one place these facts are actually computed, from a
// real or faked checkout.
package repositoryfacts

import "fmt"

// StringFact is a string-valued fact whose presence is itself proven or
// unproven: Value must be "" whenever Known is false, so an unproven fact
// can never smuggle a stale or guessed value through.
type StringFact struct {
	Known bool   `json:"known"`
	Value string `json:"value"`
}

// Validate reports whether f's known/value pair is internally consistent.
func (f StringFact) Validate() error {
	if f.Known && f.Value == "" {
		return fmt.Errorf("repositoryfacts: known is true but value is empty")
	}
	if !f.Known && f.Value != "" {
		return fmt.Errorf("repositoryfacts: known is false but value %q is non-empty", f.Value)
	}
	return nil
}

// BoolFact is a boolean-valued fact with the same known/unproven
// discipline as StringFact: Value must be false whenever Known is false.
type BoolFact struct {
	Known bool `json:"known"`
	Value bool `json:"value"`
}

// Validate reports whether f's known/value pair is internally consistent.
func (f BoolFact) Validate() error {
	if !f.Known && f.Value {
		return fmt.Errorf("repositoryfacts: known is false but value is true")
	}
	return nil
}

// DefaultBranchFact is the configured default branch's identity: Name,
// Ref, and Head must all be "" whenever Known is false, and all
// non-empty whenever Known is true.
type DefaultBranchFact struct {
	Known bool   `json:"known"`
	Name  string `json:"name"`
	Ref   string `json:"ref"`
	Head  string `json:"head"`
}

// Validate reports whether f's known/value fields are internally
// consistent.
func (f DefaultBranchFact) Validate() error {
	if f.Known && (f.Name == "" || f.Ref == "" || f.Head == "") {
		return fmt.Errorf("repositoryfacts: known is true but name/ref/head are not all non-empty")
	}
	if !f.Known && (f.Name != "" || f.Ref != "" || f.Head != "") {
		return fmt.Errorf("repositoryfacts: known is false but name/ref/head are non-empty")
	}
	return nil
}

// WorktreeFact is the active worktree's identity: Name must be ""
// whenever Managed is false, and non-empty whenever Managed is true.
type WorktreeFact struct {
	Managed bool   `json:"managed"`
	Name    string `json:"name"`
}

// Validate reports whether f's managed/name pair is internally
// consistent.
func (f WorktreeFact) Validate() error {
	if f.Managed && f.Name == "" {
		return fmt.Errorf("repositoryfacts: managed is true but name is empty")
	}
	if !f.Managed && f.Name != "" {
		return fmt.Errorf("repositoryfacts: managed is false but name %q is non-empty", f.Name)
	}
	return nil
}

// Source is the closed provenance vocabulary naming which checkout state
// a Facts snapshot's evaluated target bytes came from.
type Source string

const (
	SourceHead         Source = "head"
	SourceWorkingTree  Source = "working-tree"
	SourceRemoteRef    Source = "remote-ref"
	SourceReceiptBound Source = "receipt-bound"
)

var validSource = map[Source]bool{
	SourceHead:         true,
	SourceWorkingTree:  true,
	SourceRemoteRef:    true,
	SourceReceiptBound: true,
}

// Validate reports whether s is one of the closed Source values.
func (s Source) Validate() error {
	if !validSource[s] {
		return fmt.Errorf("repositoryfacts: unknown source %q", string(s))
	}
	return nil
}

// The five closed values Facts.Relationship may carry: HEAD classified
// against the configured default branch's HEAD.
const (
	RelationshipEqual    = "equal"
	RelationshipAhead    = "ahead"
	RelationshipBehind   = "behind"
	RelationshipDiverged = "diverged"
	RelationshipUnknown  = "unknown"
)

var validRelationship = map[string]bool{
	RelationshipEqual:    true,
	RelationshipAhead:    true,
	RelationshipBehind:   true,
	RelationshipDiverged: true,
	RelationshipUnknown:  true,
}

// Facts is the shared repository-identity fact shape (context-compiler
// authority design §4, §8.2; SI-85): canonical repository identity,
// branch, HEAD, configured default branch, their relationship,
// dirty/staged posture, managed-worktree identity, and the source the
// evaluated target bytes came from.
//
// RemoteOrigin is the CANONICAL repository identity — "host[:port]/path",
// no scheme and no userinfo (gitx.CanonicalRemoteIdentity) — never the
// raw origin URL: a raw URL may carry credentials, which this shape must
// never contain, and its ssh and https spellings of one repository
// differ, which would make identity and every digest over it
// checkout-dependent.
//
// Every field's JSON tag matches internal/journey's pre-extraction
// RepositoryFacts exactly (SI-85: "the shared fact shape canonically
// encodes exactly like the current journey repository section") — a
// consumer embeds Facts by type alias, so canonical output is byte
// identical to before the extraction.
type Facts struct {
	RemoteOrigin  StringFact        `json:"remote_origin"`
	Branch        StringFact        `json:"branch"`
	Head          StringFact        `json:"head"`
	DefaultBranch DefaultBranchFact `json:"default_branch"`
	Relationship  string            `json:"relationship"`
	Dirty         BoolFact          `json:"dirty"`
	Staged        BoolFact          `json:"staged"`
	Worktree      WorktreeFact      `json:"worktree"`
	Source        Source            `json:"source"`
}

// Validate reports the first rule f violates: an inconsistent known/value
// fact, or an unknown relationship or source.
func (f Facts) Validate() error {
	if err := f.RemoteOrigin.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: remote_origin: %w", err)
	}
	if err := f.Branch.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: branch: %w", err)
	}
	if err := f.Head.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: head: %w", err)
	}
	if err := f.DefaultBranch.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: default_branch: %w", err)
	}
	if !validRelationship[f.Relationship] {
		return fmt.Errorf("repositoryfacts: unknown relationship %q", f.Relationship)
	}
	if err := f.Dirty.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: dirty: %w", err)
	}
	if err := f.Staged.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: staged: %w", err)
	}
	if err := f.Worktree.Validate(); err != nil {
		return fmt.Errorf("repositoryfacts: worktree: %w", err)
	}
	if err := f.Source.Validate(); err != nil {
		return err
	}
	return nil
}

// DisclosureCode is the closed, machine-stable, per-cause vocabulary
// Gather emits when a fact could not be established. It is deliberately
// FINER-grained than the context-compiler manifest's own closed
// disclosure vocabulary (authority design §8.2): every prose variant
// internal/journey's pre-extraction gatherRepositoryFacts distinguished —
// including the three distinct remote-origin causes — keeps its own
// stable code here, so a consumer maps this code to its own prose
// (journey) or to the coarser manifest code (a later contextcompile task)
// without the two consumers silently drifting apart on WHY a fact is
// unknown. Prose itself is never part of this package's protocol.
type DisclosureCode string

const (
	// DisclosureRemoteOriginUncanonicalizable: the origin remote URL was
	// read but could not be reduced to a canonical repository identity.
	DisclosureRemoteOriginUncanonicalizable DisclosureCode = "remote-origin-uncanonicalizable"
	// DisclosureRemoteOriginNotConfigured: the checkout has no "origin"
	// remote configured at all (gitx.ErrNoSuchRemote).
	DisclosureRemoteOriginNotConfigured DisclosureCode = "remote-origin-not-configured"
	// DisclosureRemoteOriginReadFailed: reading the origin remote URL
	// failed for a reason other than "no such remote".
	DisclosureRemoteOriginReadFailed DisclosureCode = "remote-origin-read-failed"
	// DisclosureBranchUnresolved: the current branch could not be
	// determined from this checkout.
	DisclosureBranchUnresolved DisclosureCode = "branch-unresolved"
	// DisclosureBranchDetached: the checkout is in a detached HEAD state,
	// so it has no current branch name.
	DisclosureBranchDetached DisclosureCode = "branch-detached"
	// DisclosureHeadUnresolved: HEAD could not be resolved to a commit.
	DisclosureHeadUnresolved DisclosureCode = "head-unresolved"
	// DisclosureDefaultBranchRefUnresolved: the configured default
	// branch's NAME resolved, but its ref could not be turned into a
	// commit — distinct from "no default branch resolves at all", which
	// this package deliberately does not disclose (a caller that also
	// needs default-branch state derives that fact from
	// Facts.DefaultBranch.Known itself; the "no default branch resolves"
	// case is a lifecycle/blocker-level concern outside this leaf's
	// scope, exactly mirroring the pre-extraction journey behavior it
	// ports).
	DisclosureDefaultBranchRefUnresolved DisclosureCode = "default-branch-ref-unresolved"
	// DisclosureDirtyUnknown: working-tree dirty state could not be
	// determined from this checkout.
	DisclosureDirtyUnknown DisclosureCode = "dirty-unknown"
	// DisclosureStagedUnknown: staged paths could not be determined from
	// this checkout.
	DisclosureStagedUnknown DisclosureCode = "staged-unknown"
)

var validDisclosureCode = map[DisclosureCode]bool{
	DisclosureRemoteOriginUncanonicalizable: true,
	DisclosureRemoteOriginNotConfigured:     true,
	DisclosureRemoteOriginReadFailed:        true,
	DisclosureBranchUnresolved:              true,
	DisclosureBranchDetached:                true,
	DisclosureHeadUnresolved:                true,
	DisclosureDefaultBranchRefUnresolved:    true,
	DisclosureDirtyUnknown:                  true,
	DisclosureStagedUnknown:                 true,
}

// Validate reports whether c is one of the closed DisclosureCode values.
func (c DisclosureCode) Validate() error {
	if !validDisclosureCode[c] {
		return fmt.Errorf("repositoryfacts: unknown disclosure code %q", string(c))
	}
	return nil
}

// Snapshot is Gather's complete result: the fact shape plus every
// disclosure naming why a fact could not be established. Disclosures is
// sorted and duplicate-free (CLAUDE.md: "every set-like slice is sorted
// and duplicate rejected"); a consumer that additionally merges these
// codes with disclosures from other sources still re-sorts the merged
// set itself.
type Snapshot struct {
	Facts       Facts            `json:"facts"`
	Disclosures []DisclosureCode `json:"disclosures"`
}

// Validate reports the first rule s violates: an invalid Facts value, a
// nil Disclosures slice, an unknown disclosure code, or a Disclosures
// slice that is not strictly ascending (sorted, duplicate-free).
func (s Snapshot) Validate() error {
	if err := s.Facts.Validate(); err != nil {
		return err
	}
	if s.Disclosures == nil {
		return fmt.Errorf("repositoryfacts: disclosures must be non-nil (an explicitly empty set is [])")
	}
	var last DisclosureCode
	for i, c := range s.Disclosures {
		if err := c.Validate(); err != nil {
			return err
		}
		if i > 0 && c <= last {
			return fmt.Errorf("repositoryfacts: disclosures must be sorted and duplicate-free")
		}
		last = c
	}
	return nil
}
