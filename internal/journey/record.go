// Package journey defines Verdi's canonical journey record: GLG AC-1's
// stable output contract for `verdi journey`, its workbench projection,
// MCP, and dex (.verdi/specs/active/guided-lifecycle-governance-v3/
// spec.md, AC-1, DC-2/DC-3/DC-4, CO-4). The record is a read-only
// projection's output, never lifecycle state itself (DC-1): removing the
// projection leaves every canonical artifact, gate, transition, and
// recovery fact intact, and a journey record may be cached by input
// digest but the cache is never authority.
//
// This package holds only the schema, its stable reason-code and
// blocker-class vocabulary, and deterministic canonical-encode/decode/
// validate behavior. It never gathers facts, calls git, or shells out —
// no wall-clock, no randomness, no I/O. A later lane populates a Record's
// fields from real repository, lifecycle, blocker, principal, and action
// evidence.
package journey

import (
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// SchemaID is the only accepted Record.Schema value.
const SchemaID = "verdi.journey/v1"

var (
	idRe         = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	blockerIDRe  = regexp.MustCompile(`^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)*$`)
	obligationRe = regexp.MustCompile(`^[a-z][a-z0-9-]*/[a-z][a-z0-9-]*$`)
	argumentRe   = regexp.MustCompile(`^[A-Za-z0-9._/@-]+$`)
	digestRe     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// Record is one canonical journey record (AC-1): repository identity and
// relationship, lifecycle facts, current and eventual blockers,
// authenticated principals and required roles, and safe actions. Digest
// is the record's own content address (Canonical), never wall-clock- or
// randomness-derived.
type Record struct {
	Schema      string          `json:"schema"`
	Target      Target          `json:"target"`
	Repository  RepositoryFacts `json:"repository"`
	Lifecycle   LifecycleFacts  `json:"lifecycle"`
	Evidence    EvidenceFacts   `json:"evidence"`
	Blockers    Blockers        `json:"blockers"`
	Principals  PrincipalFacts  `json:"principals"`
	Actions     Actions         `json:"actions"`
	Disclosures []string        `json:"disclosures"`
	Digest      string          `json:"digest"`
}

// Target identifies the spec/story the record was computed for.
type Target struct {
	Ref   string `json:"ref"`
	Class string `json:"class"`
	Path  string `json:"path"`
}

// StringFact is a string-valued fact whose presence is itself proven or
// unproven: Value must be "" whenever Known is false, so an unproven fact
// can never smuggle a stale or guessed value through.
type StringFact struct {
	Known bool   `json:"known"`
	Value string `json:"value"`
}

// BoolFact is a boolean-valued fact with the same known/unproven
// discipline as StringFact: Value must be false whenever Known is false.
type BoolFact struct {
	Known bool `json:"known"`
	Value bool `json:"value"`
}

// DefaultBranchFact is the configured default branch's identity: Name,
// Ref, and Head must all be "" whenever Known is false.
type DefaultBranchFact struct {
	Known bool   `json:"known"`
	Name  string `json:"name"`
	Ref   string `json:"ref"`
	Head  string `json:"head"`
}

// WorktreeFact is the active worktree's identity: Name must be ""
// whenever Managed is false.
type WorktreeFact struct {
	Managed bool   `json:"managed"`
	Name    string `json:"name"`
}

// RepositoryFacts is the record's repository-identity section (AC-1): the
// canonical repository identity, branch, HEAD, default-branch HEAD, and
// their relationship stay visible because hiding them recreates the
// wrong-checkout ambiguity the feature exists to eliminate (DC-2).
//
// RemoteOrigin is the CANONICAL repository identity — "host[:port]/path",
// no scheme and no userinfo (gitx.CanonicalRemoteIdentity) — never the raw
// origin URL: a raw URL may carry credentials, which this record must
// never contain, and its ssh and https spellings of one repository differ,
// which would make identity and every digest over it checkout-dependent.
type RepositoryFacts struct {
	RemoteOrigin  StringFact        `json:"remote_origin"`
	Branch        StringFact        `json:"branch"`
	Head          StringFact        `json:"head"`
	DefaultBranch DefaultBranchFact `json:"default_branch"`
	Relationship  string            `json:"relationship"`
	Dirty         BoolFact          `json:"dirty"`
	Staged        BoolFact          `json:"staged"`
	Worktree      WorktreeFact      `json:"worktree"`
	Source        string            `json:"source"`
}

// Baseline is the accepted-baseline identity: path, blob, and landing
// commit. It mirrors internal/specstate.Baseline's JSON shape but is this
// package's own type — the record schema is self-contained and never
// imports internal/specstate.
type Baseline struct {
	Path          string `json:"path"`
	Blob          string `json:"blob"`
	LandingCommit string `json:"commit"`
}

// FrozenRevision is the active frozen build/design revision, if any.
type FrozenRevision struct {
	At     string `json:"at"`
	Commit string `json:"commit"`
}

// LifecycleFacts is the record's lifecycle section: class and state,
// accepted and frozen revisions, the active build/design branch,
// authoritative-versus-advisory posture, and disclosed gaps in deriving
// any of them.
type LifecycleFacts struct {
	Class            string          `json:"class"`
	State            string          `json:"state"`
	Relation         string          `json:"relation"`
	Posture          string          `json:"posture"`
	AcceptedBaseline *Baseline       `json:"accepted_baseline"`
	Frozen           *FrozenRevision `json:"frozen"`
	ActiveBranch     StringFact      `json:"active_branch"`
	Disclosures      []string        `json:"disclosures"`
}

// EvidenceContributor is one evidence source the target's acceptance
// criteria declare, and this projection's three-valued reading of it.
// Kind is drawn from the closed evidence-kind catalog (parity-tested
// against internal/artifact.EvidenceKind); Resolution is proven,
// violated-with-witness, or unproven — silence is never a pass (CO-1), so
// there is no fourth "not looked at" value: a source nobody evaluated is
// unproven WITH a witness saying why.
type EvidenceContributor struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Resolution string `json:"resolution"`
	Witness    string `json:"witness"`
}

// EvidenceFacts is the record's evidence section. DC-2 makes evidence
// AUTHORITY and FRESHNESS always-visible operands of every journey
// response, alongside repository, HEAD, and default-branch relationship —
// they are typed fields here rather than free-text disclosures precisely
// because a consumer must be able to read them without parsing prose.
//
// Authority is authoritative, advisory, or unknown; Freshness is fresh,
// stale, or unknown. "unknown" is a real, legal posture — but never a
// silent one: CO-1's rule is enforced structurally, so an unknown
// operand with an empty Disclosures list fails Validate.
type EvidenceFacts struct {
	Authority    string                `json:"authority"`
	Freshness    string                `json:"freshness"`
	Contributors []EvidenceContributor `json:"contributors"`
	Disclosures  []string              `json:"disclosures"`
}

// Owner is a blocker's advisory owner: a display projection of the spec's
// own owners frontmatter (Declared, which may be "") plus the kernel's
// Attribution record. Attribution is never a bare string presented as
// identity (DC-19) — Owner.Validate calls Attribution.Validate.
type Owner struct {
	Declared    string                          `json:"declared"`
	Attribution governanceprincipal.Attribution `json:"attribution"`
}

// BlockerClass and ReasonCode live in reason.go.

// Blocker is one current or eventual blocker: its stable reason code,
// fixed class, witnesses, owner, clearing condition, and the transition
// it affects (or the literal "unknown"). ID is slash-segmented
// (^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)*$) so a blocker can compose its
// reason with the transition it blocks, e.g.
// "obligation-countersign-unproven/close".
type Blocker struct {
	ID                string       `json:"id"`
	Reason            ReasonCode   `json:"reason"`
	Class             BlockerClass `json:"class"`
	Witnesses         []string     `json:"witnesses"`
	Owner             Owner        `json:"owner"`
	ClearingCondition string       `json:"clearing_condition"`
	Transition        string       `json:"transition"`
}

// EventualBlockers is the closure-blocker section. An underived section
// must disclose itself (CO-1): when Derived is false, Items must be empty
// and Disclosures must be non-empty.
type EventualBlockers struct {
	Derived     bool      `json:"derived"`
	Items       []Blocker `json:"items"`
	Disclosures []string  `json:"disclosures"`
}

// Blockers is the record's blockers section: current blockers plus
// eventual closure blockers. Blocker IDs are unique across both.
type Blockers struct {
	Current  []Blocker        `json:"current"`
	Eventual EventualBlockers `json:"eventual"`
}

// RequiredRole is one role required for the next governance action, and
// its resolution: authenticated, violated-with-witness, or unproven —
// silence is never a pass (CO-1).
type RequiredRole struct {
	Transition string `json:"transition"`
	Obligation string `json:"obligation"`
	Count      int    `json:"count"`
	Resolution string `json:"resolution"`
}

// PrincipalFacts is the record's principals section: whether a governance
// profile was adopted, the roles required for the next action, and any
// disclosed absence or unavailable separation of duties.
type PrincipalFacts struct {
	ProfileAdopted        bool           `json:"profile_adopted"`
	SelectedProfileID     string         `json:"selected_profile_id"`
	SelectedProfileDigest string         `json:"selected_profile_digest"`
	Required              []RequiredRole `json:"required"`
	Disclosures           []string       `json:"disclosures"`
}

// Precondition is one proven precondition of a safe action. There is
// deliberately no State field: a precondition may appear on a safe action
// only when proven (DC-3), so the type itself cannot express an unproven
// precondition.
type Precondition struct {
	ID      string `json:"id"`
	Witness string `json:"witness"`
}

// Action is one safe action: the existing Verdi verb or forge transition
// that performs it, its bound arguments, its declared state transition,
// its confirmation posture, its proven preconditions, and any
// principal-bearing authority it itself requires (Authority — e.g. a
// countersign obligation on the same verb; empty when none is known-
// required). An action is never free text or generated shell (DC-3):
// Arguments are bare tokens, never shell text.
//
// FromState and ToState are the declared effect of a registered
// operating-model catalog transition, in the operating model's own
// state vocabulary (e.g. "draft") — never internal/specstate's
// spec-lifecycle vocabulary ("proposed", ...), and this schema does not
// import specstate. Proving that Verb/FromState/ToState actually name a
// catalog transition is the deriving layer's obligation; this schema
// enforces only their lexical grammar, deliberately not their catalog
// membership — the record is a projection's output, not the operating
// model itself (DC-1).
type Action struct {
	ID            string         `json:"id"`
	Verb          string         `json:"verb"`
	Arguments     []string       `json:"arguments"`
	FromState     string         `json:"from_state"`
	ToState       string         `json:"to_state"`
	Confirmation  string         `json:"confirmation"`
	Preconditions []Precondition `json:"preconditions"`
	Authority     []RequiredRole `json:"authority"`
}

// Actions is the record's actions section: the safe actions whose
// preconditions hold now, plus the facts still needed to unblock more.
type Actions struct {
	Safe        []Action `json:"safe"`
	NeededFacts []string `json:"needed_facts"`
}

// --- Validate ---------------------------------------------------------

var (
	validTargetClass = map[string]bool{"feature": true, "story": true}
	validState       = map[string]bool{
		"proposed": true, "accepted-pending-build": true, "superseded": true,
		"closed": true, "unproven": true,
	}
	validLifecycleRelation = map[string]bool{"new": true, "exact": true, "diverged": true, "unproven": true}
	validPosture           = map[string]bool{"authoritative": true, "advisory": true, "unknown": true}
	validRelationship      = map[string]bool{"equal": true, "ahead": true, "behind": true, "diverged": true, "unknown": true}
	validSource            = map[string]bool{"head": true, "working-tree": true, "remote-ref": true, "receipt-bound": true}
	validConfirmation      = map[string]bool{"none": true, "explicit-confirmation": true}
	validResolution        = map[string]bool{"authenticated": true, "violated-with-witness": true, "unproven": true}

	validEvidenceAuthority = map[string]bool{"authoritative": true, "advisory": true, "unknown": true}
	validEvidenceFreshness = map[string]bool{"fresh": true, "stale": true, "unknown": true}
	// validEvidenceKind is internal/artifact.EvidenceKind's constant set,
	// all four of it (parity-tested, TestEvidenceKindParityWithArtifact).
	// Omitting "runtime" — the one kind this delivery unit's own fixtures
	// never exercise — would make a spec that legally declares
	// `evidence: [runtime]` abort the whole projection on a fail-closed
	// enum, so the closed set here is the ARTIFACT's closed set, not this
	// unit's convenience subset.
	validEvidenceKind = map[string]bool{
		"static": true, "behavioral": true, "runtime": true, "attestation": true,
	}
	// validEvidenceResolution is deliberately NOT validResolution: a
	// principal is authenticated or not, while an evidence source is
	// PROVEN or not — different questions, different closed vocabularies.
	validEvidenceResolution = map[string]bool{"proven": true, "violated-with-witness": true, "unproven": true}
)

// Validate reports the first rule the record violates, fail-closed: an
// unknown enum value, a malformed identifier, an inconsistent known/value
// fact, a blocker class contradicting its reason code's fixed class, a
// duplicate ID, an unsorted or duplicated disclosure/witness list, a nil
// slice where the schema requires an explicit empty one, or a malformed
// digest.
func (r Record) Validate() error {
	if r.Schema != SchemaID {
		return fmt.Errorf("journey: record: schema %q, want %q", r.Schema, SchemaID)
	}
	if err := r.Target.validate(); err != nil {
		return err
	}
	if err := r.Repository.validate(); err != nil {
		return err
	}
	if err := r.Lifecycle.validate(r.Target.Class); err != nil {
		return err
	}
	if err := r.Evidence.validate(); err != nil {
		return err
	}
	if err := r.Blockers.validate(); err != nil {
		return err
	}
	if err := r.Principals.validate(); err != nil {
		return err
	}
	if err := r.Actions.validate(); err != nil {
		return err
	}
	if r.Disclosures == nil {
		return fmt.Errorf("journey: record: disclosures must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(r.Disclosures) {
		return fmt.Errorf("journey: record: disclosures must be sorted and deduplicated")
	}
	if r.Digest != "" && !digestRe.MatchString(r.Digest) {
		return fmt.Errorf("journey: record: digest %q is not empty or a valid sha256:<hex> digest", r.Digest)
	}
	return nil
}

func (t Target) validate() error {
	if !validTargetClass[t.Class] {
		return fmt.Errorf("journey: target: unknown class %q", t.Class)
	}
	if t.Ref == "" {
		return fmt.Errorf("journey: target: ref must be non-empty")
	}
	if t.Path == "" {
		return fmt.Errorf("journey: target: path must be non-empty")
	}
	return nil
}

func (f StringFact) validate(field string) error {
	if f.Known && f.Value == "" {
		return fmt.Errorf("journey: %s: known is true but value is empty", field)
	}
	if !f.Known && f.Value != "" {
		return fmt.Errorf("journey: %s: known is false but value %q is non-empty", field, f.Value)
	}
	return nil
}

func (f BoolFact) validate(field string) error {
	if !f.Known && f.Value {
		return fmt.Errorf("journey: %s: known is false but value is true", field)
	}
	return nil
}

func (f DefaultBranchFact) validate(field string) error {
	if f.Known && (f.Name == "" || f.Ref == "" || f.Head == "") {
		return fmt.Errorf("journey: %s: known is true but name/ref/head are not all non-empty", field)
	}
	if !f.Known && (f.Name != "" || f.Ref != "" || f.Head != "") {
		return fmt.Errorf("journey: %s: known is false but name/ref/head are non-empty", field)
	}
	return nil
}

func (f WorktreeFact) validate(field string) error {
	if f.Managed && f.Name == "" {
		return fmt.Errorf("journey: %s: managed is true but name is empty", field)
	}
	if !f.Managed && f.Name != "" {
		return fmt.Errorf("journey: %s: managed is false but name %q is non-empty", field, f.Name)
	}
	return nil
}

func (rf RepositoryFacts) validate() error {
	if err := rf.RemoteOrigin.validate("repository.remote_origin"); err != nil {
		return err
	}
	if err := rf.Branch.validate("repository.branch"); err != nil {
		return err
	}
	if err := rf.Head.validate("repository.head"); err != nil {
		return err
	}
	if err := rf.DefaultBranch.validate("repository.default_branch"); err != nil {
		return err
	}
	if !validRelationship[rf.Relationship] {
		return fmt.Errorf("journey: repository: unknown relationship %q", rf.Relationship)
	}
	if err := rf.Dirty.validate("repository.dirty"); err != nil {
		return err
	}
	if err := rf.Staged.validate("repository.staged"); err != nil {
		return err
	}
	if err := rf.Worktree.validate("repository.worktree"); err != nil {
		return err
	}
	if !validSource[rf.Source] {
		return fmt.Errorf("journey: repository: unknown source %q", rf.Source)
	}
	return nil
}

func (lf LifecycleFacts) validate(targetClass string) error {
	if !validTargetClass[lf.Class] {
		return fmt.Errorf("journey: lifecycle: unknown class %q", lf.Class)
	}
	if lf.Class != targetClass {
		return fmt.Errorf("journey: lifecycle: class %q does not match target class %q", lf.Class, targetClass)
	}
	if !validState[lf.State] {
		return fmt.Errorf("journey: lifecycle: unknown state %q", lf.State)
	}
	if !validLifecycleRelation[lf.Relation] {
		return fmt.Errorf("journey: lifecycle: unknown relation %q", lf.Relation)
	}
	if !validPosture[lf.Posture] {
		return fmt.Errorf("journey: lifecycle: unknown posture %q", lf.Posture)
	}
	if lf.AcceptedBaseline != nil {
		b := lf.AcceptedBaseline
		if b.Path == "" || b.Blob == "" || b.LandingCommit == "" {
			return fmt.Errorf("journey: lifecycle.accepted_baseline: path, blob, and commit must all be non-empty")
		}
	}
	if lf.Frozen != nil {
		fz := lf.Frozen
		if fz.At == "" || fz.Commit == "" {
			return fmt.Errorf("journey: lifecycle.frozen: at and commit must both be non-empty")
		}
	}
	if err := lf.ActiveBranch.validate("lifecycle.active_branch"); err != nil {
		return err
	}
	if lf.Disclosures == nil {
		return fmt.Errorf("journey: lifecycle: disclosures must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(lf.Disclosures) {
		return fmt.Errorf("journey: lifecycle: disclosures must be sorted and deduplicated")
	}
	return nil
}

func (c EvidenceContributor) validate(field string) error {
	if !idRe.MatchString(c.ID) {
		return fmt.Errorf("journey: %s: id %q must match ^[a-z][a-z0-9-]*$", field, c.ID)
	}
	if !validEvidenceKind[c.Kind] {
		return fmt.Errorf("journey: %s: unknown evidence kind %q", field, c.Kind)
	}
	if !validEvidenceResolution[c.Resolution] {
		return fmt.Errorf("journey: %s: unknown resolution %q", field, c.Resolution)
	}
	if c.Witness == "" {
		return fmt.Errorf("journey: %s: witness must be non-empty (a resolution with no witness is silence)", field)
	}
	return nil
}

func (ef EvidenceFacts) validate() error {
	if !validEvidenceAuthority[ef.Authority] {
		return fmt.Errorf("journey: evidence: unknown authority %q", ef.Authority)
	}
	if !validEvidenceFreshness[ef.Freshness] {
		return fmt.Errorf("journey: evidence: unknown freshness %q", ef.Freshness)
	}
	if ef.Contributors == nil {
		return fmt.Errorf("journey: evidence.contributors: must be non-nil (an explicitly empty set is [])")
	}
	for i, c := range ef.Contributors {
		if err := c.validate(fmt.Sprintf("evidence.contributors[%d]", i)); err != nil {
			return err
		}
	}
	if !isSortedDeduped(mapStrings(ef.Contributors, func(c EvidenceContributor) string { return c.ID })) {
		return fmt.Errorf("journey: evidence.contributors: must be strictly ascending by id (unique and ordered)")
	}
	if ef.Disclosures == nil {
		return fmt.Errorf("journey: evidence.disclosures: must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(ef.Disclosures) {
		return fmt.Errorf("journey: evidence.disclosures: must be sorted and deduplicated")
	}
	// CO-1, structurally: an unknown operand must disclose itself. Both
	// operands are checked against the SAME list, so a record whose
	// authority is unknown for one reason and freshness for another must
	// still carry at least one disclosure explaining the gap.
	if (ef.Authority == "unknown" || ef.Freshness == "unknown") && len(ef.Disclosures) == 0 {
		return fmt.Errorf("journey: evidence: authority or freshness is unknown but disclosures is empty: an unproven operand must disclose itself (CO-1)")
	}
	return nil
}

func (o Owner) validate(field string) error {
	if err := o.Attribution.Validate(); err != nil {
		return fmt.Errorf("journey: %s.attribution: %w", field, err)
	}
	return nil
}

func (b Blocker) validate(field string) error {
	if !blockerIDRe.MatchString(b.ID) {
		return fmt.Errorf("journey: %s: id %q must match ^[a-z][a-z0-9-]*(/[a-z][a-z0-9-]*)*$", field, b.ID)
	}
	fixedClass, err := b.Reason.Class()
	if err != nil {
		return fmt.Errorf("journey: %s: reason: %w", field, err)
	}
	if !validBlockerClasses[b.Class] {
		return fmt.Errorf("journey: %s: unknown class %q", field, b.Class)
	}
	if b.Class != fixedClass {
		return fmt.Errorf("journey: %s: class %q contradicts reason %q's fixed class %q", field, b.Class, b.Reason, fixedClass)
	}
	if b.Witnesses == nil {
		return fmt.Errorf("journey: %s: witnesses must be non-nil (an explicitly empty set is [])", field)
	}
	if !isSortedDeduped(b.Witnesses) {
		return fmt.Errorf("journey: %s: witnesses must be sorted and deduplicated", field)
	}
	if err := b.Owner.validate(field + ".owner"); err != nil {
		return err
	}
	if b.ClearingCondition == "" {
		return fmt.Errorf("journey: %s: clearing_condition must be non-empty", field)
	}
	if !idRe.MatchString(b.Transition) {
		return fmt.Errorf("journey: %s: transition %q must be a lowercase-kebab token (a catalog verb or the literal \"unknown\")", field, b.Transition)
	}
	return nil
}

func (eb EventualBlockers) validate() error {
	if eb.Items == nil {
		return fmt.Errorf("journey: blockers.eventual.items: must be non-nil (an explicitly empty set is [])")
	}
	for i, blk := range eb.Items {
		if err := blk.validate(fmt.Sprintf("blockers.eventual.items[%d]", i)); err != nil {
			return err
		}
	}
	if !isSortedDeduped(mapStrings(eb.Items, func(b Blocker) string { return b.ID })) {
		return fmt.Errorf("journey: blockers.eventual.items: must be strictly ascending by id (unique and ordered)")
	}
	if eb.Disclosures == nil {
		return fmt.Errorf("journey: blockers.eventual.disclosures: must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(eb.Disclosures) {
		return fmt.Errorf("journey: blockers.eventual.disclosures: must be sorted and deduplicated")
	}
	if !eb.Derived {
		if len(eb.Items) != 0 {
			return fmt.Errorf("journey: blockers.eventual: derived is false but items is non-empty")
		}
		if len(eb.Disclosures) == 0 {
			return fmt.Errorf("journey: blockers.eventual: derived is false but disclosures is empty: an underived section must disclose itself")
		}
	}
	return nil
}

func (bs Blockers) validate() error {
	if bs.Current == nil {
		return fmt.Errorf("journey: blockers.current: must be non-nil (an explicitly empty set is [])")
	}
	seen := make(map[string]bool, len(bs.Current))
	for i, blk := range bs.Current {
		field := fmt.Sprintf("blockers.current[%d]", i)
		if err := blk.validate(field); err != nil {
			return err
		}
		if seen[blk.ID] {
			return fmt.Errorf("journey: blockers: duplicate blocker id %q", blk.ID)
		}
		seen[blk.ID] = true
	}
	if !isSortedDeduped(mapStrings(bs.Current, func(b Blocker) string { return b.ID })) {
		return fmt.Errorf("journey: blockers.current: must be strictly ascending by id (unique and ordered)")
	}
	if err := bs.Eventual.validate(); err != nil {
		return err
	}
	for _, blk := range bs.Eventual.Items {
		if seen[blk.ID] {
			return fmt.Errorf("journey: blockers: duplicate blocker id %q", blk.ID)
		}
		seen[blk.ID] = true
	}
	return nil
}

func (rr RequiredRole) validate(field string) error {
	if !idRe.MatchString(rr.Transition) {
		return fmt.Errorf("journey: %s: transition %q must match ^[a-z][a-z0-9-]*$", field, rr.Transition)
	}
	if !obligationRe.MatchString(rr.Obligation) {
		return fmt.Errorf("journey: %s: obligation %q must match <kind>/<obligation>", field, rr.Obligation)
	}
	if rr.Count < 1 {
		return fmt.Errorf("journey: %s: count must be >= 1, got %d", field, rr.Count)
	}
	if !validResolution[rr.Resolution] {
		return fmt.Errorf("journey: %s: unknown resolution %q", field, rr.Resolution)
	}
	return nil
}

// requiredRoleKey is the (transition, obligation) ordering key shared by
// PrincipalFacts.Required and Action.Authority: both must be strictly
// ascending by this tuple (CO-4). \x00 cannot appear in either field
// (both are restricted to idRe/obligationRe's grammar), so lexicographic
// order over the joined key matches tuple order exactly.
func requiredRoleKey(rr RequiredRole) string {
	return rr.Transition + "\x00" + rr.Obligation
}

func (pf PrincipalFacts) validate() error {
	if pf.ProfileAdopted {
		if err := governanceprincipal.ValidateID(pf.SelectedProfileID); err != nil {
			return fmt.Errorf("journey: principals.selected_profile_id: %w", err)
		}
		if !digestRe.MatchString(pf.SelectedProfileDigest) {
			return fmt.Errorf("journey: principals.selected_profile_digest: %q must match sha256:<64 lowercase hex>", pf.SelectedProfileDigest)
		}
	} else {
		if pf.SelectedProfileID != "" {
			return fmt.Errorf("journey: principals.selected_profile_id: must be empty when profile_adopted is false")
		}
		if pf.SelectedProfileDigest != "" {
			return fmt.Errorf("journey: principals.selected_profile_digest: must be empty when profile_adopted is false")
		}
	}
	if pf.Required == nil {
		return fmt.Errorf("journey: principals.required: must be non-nil (an explicitly empty set is [])")
	}
	for i, rr := range pf.Required {
		if err := rr.validate(fmt.Sprintf("principals.required[%d]", i)); err != nil {
			return err
		}
	}
	if !isSortedDeduped(mapStrings(pf.Required, requiredRoleKey)) {
		return fmt.Errorf("journey: principals.required: must be strictly ascending by (transition, obligation)")
	}
	if pf.Disclosures == nil {
		return fmt.Errorf("journey: principals.disclosures: must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(pf.Disclosures) {
		return fmt.Errorf("journey: principals.disclosures: must be sorted and deduplicated")
	}
	if !pf.ProfileAdopted && len(pf.Disclosures) == 0 {
		return fmt.Errorf("journey: principals: profile_adopted is false but disclosures is empty: absence must be disclosed (CO-1)")
	}
	return nil
}

func (p Precondition) validate(field string) error {
	if !idRe.MatchString(p.ID) {
		return fmt.Errorf("journey: %s: precondition id %q must match ^[a-z][a-z0-9-]*$", field, p.ID)
	}
	if p.Witness == "" {
		return fmt.Errorf("journey: %s: witness must be non-empty", field)
	}
	return nil
}

func (a Action) validate(field string) error {
	if !idRe.MatchString(a.ID) {
		return fmt.Errorf("journey: %s: id %q must match ^[a-z][a-z0-9-]*$", field, a.ID)
	}
	if !idRe.MatchString(a.Verb) {
		return fmt.Errorf("journey: %s: verb %q must match ^[a-z][a-z0-9-]*$", field, a.Verb)
	}
	if a.Arguments == nil {
		return fmt.Errorf("journey: %s: arguments must be non-nil (an explicitly empty set is [])", field)
	}
	for i, arg := range a.Arguments {
		if !argumentRe.MatchString(arg) {
			return fmt.Errorf("journey: %s: argument[%d] %q is not a bare token (never free shell text)", field, i, arg)
		}
	}
	if !idRe.MatchString(a.FromState) {
		return fmt.Errorf("journey: %s: from_state %q must match ^[a-z][a-z0-9-]*$", field, a.FromState)
	}
	if !idRe.MatchString(a.ToState) {
		return fmt.Errorf("journey: %s: to_state %q must match ^[a-z][a-z0-9-]*$", field, a.ToState)
	}
	if !validConfirmation[a.Confirmation] {
		return fmt.Errorf("journey: %s: unknown confirmation %q", field, a.Confirmation)
	}
	if len(a.Preconditions) == 0 {
		return fmt.Errorf("journey: %s: preconditions must be non-empty: an action with no stated preconditions is invalid", field)
	}
	for i, p := range a.Preconditions {
		if err := p.validate(fmt.Sprintf("%s.preconditions[%d]", field, i)); err != nil {
			return err
		}
	}
	if !isSortedDeduped(mapStrings(a.Preconditions, func(p Precondition) string { return p.ID })) {
		return fmt.Errorf("journey: %s.preconditions: must be strictly ascending by id (unique and ordered)", field)
	}
	if a.Authority == nil {
		return fmt.Errorf("journey: %s.authority: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, ar := range a.Authority {
		if err := ar.validate(fmt.Sprintf("%s.authority[%d]", field, i)); err != nil {
			return err
		}
		if ar.Transition != a.Verb {
			return fmt.Errorf("journey: %s.authority[%d]: transition %q must equal the action's verb %q", field, i, ar.Transition, a.Verb)
		}
	}
	if !isSortedDeduped(mapStrings(a.Authority, requiredRoleKey)) {
		return fmt.Errorf("journey: %s.authority: must be strictly ascending by (transition, obligation)", field)
	}
	return nil
}

func (as Actions) validate() error {
	if as.Safe == nil {
		return fmt.Errorf("journey: actions.safe: must be non-nil (an explicitly empty set is [])")
	}
	seen := make(map[string]bool, len(as.Safe))
	for i, a := range as.Safe {
		field := fmt.Sprintf("actions.safe[%d]", i)
		if err := a.validate(field); err != nil {
			return err
		}
		if seen[a.ID] {
			return fmt.Errorf("journey: actions.safe: duplicate action id %q", a.ID)
		}
		seen[a.ID] = true
	}
	if !isSortedDeduped(mapStrings(as.Safe, func(a Action) string { return a.ID })) {
		return fmt.Errorf("journey: actions.safe: must be strictly ascending by id (unique and ordered)")
	}
	if as.NeededFacts == nil {
		return fmt.Errorf("journey: actions.needed_facts: must be non-nil (an explicitly empty set is [])")
	}
	if !isSortedDeduped(as.NeededFacts) {
		return fmt.Errorf("journey: actions.needed_facts: must be sorted and deduplicated")
	}
	return nil
}

// isSortedDeduped reports whether ss is in strict ascending order: sorted
// and free of adjacent duplicates in one pass.
func isSortedDeduped(ss []string) bool {
	for i := 1; i < len(ss); i++ {
		if ss[i] <= ss[i-1] {
			return false
		}
	}
	return true
}

// mapStrings projects each element of items to a string ordering/identity
// key, preserving order — the shared building block for this package's
// ascending-order and dedup checks over non-string element types.
func mapStrings[T any](items []T, key func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = key(it)
	}
	return out
}
