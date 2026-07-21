package artifact

import (
	"fmt"
	"strings"
)

const deviationSchema = "verdi.deviation/v1"

// CollisionInfix is the reserved id infix ReconcileJudged (internal/align)
// appends when it disambiguates a slug that 2+ fresh judged findings shared
// within one run: a later member becomes "<slug><CollisionInfix><n>". It lives
// here, at the shared schema seam, because it is a schema-level fact about the
// id shapes a deviation report may legitimately carry — so the reserved literal
// never drifts across the readers that now key off it: ReconcileJudged
// (reaffirm.go, collisionMemberIDs) mints it, Validate below relaxes its
// same-kind not-resurfaced overlap for it (the sanctioned confirmed-collision-
// member shape, judged-collision-suffixed-backing-shadow), and cmd/verdi's
// disposition verb withholds live-path reaffirmation for it.
const CollisionInfix = "-collision-"

// ContractViolationIDPrefix is the reserved id prefix ReconcileJudged mints for
// the synthetic per-slug judge-contract-violation finding it appends when a
// slug collides (contractViolationFinding). Reserved at the shared schema seam
// alongside CollisionInfix for the same reason: a report may legitimately carry
// this id shape and more than one reader must agree on it byte-for-byte.
const ContractViolationIDPrefix = "judged-contract-violation-"

// IsCollisionMachineryID reports whether id was MINTED by ReconcileJudged's
// within-run slug-collision machinery — either a suffixed collision member
// ("<slug><CollisionInfix><n>") or the synthetic per-slug contract-violation
// finding ("<ContractViolationIDPrefix><slug>"). Such an id NEVER carried a
// rendered reaffirmation Candidate (ReconcileJudged pre-fills a Candidate only
// for a single fresh finding under a bare slug, never for any collision output,
// ac-4), so the disposition verb must never resolve+stamp a same-id backing
// record through the live path for it (L-N13, judged-collision-suffixed-backing-
// shadow), and Validate legitimately permits such an id to be dispositioned in
// findings: while a distinct-content backing record persists in not-resurfaced:.
// The ContractViolationIDPrefix arm treats that prefix as reserved: a (rare,
// prompt-discouraged) real judged slug of that literal shape fails CLOSED here
// (no live-path reaffirmation), the safe direction — it never mints unearned
// provenance.
func IsCollisionMachineryID(id string) bool {
	return strings.Contains(id, CollisionInfix) || strings.HasPrefix(id, ContractViolationIDPrefix)
}

// FindingKind tags a deviation finding as computed (regenerated graph/
// contract diff) or judged (the alignment subagent's semantic reading)
// (03 §Alignment report).
type FindingKind string

const (
	FindingComputed FindingKind = "computed"
	FindingJudged   FindingKind = "judged"
)

var validFindingKinds = map[FindingKind]bool{
	FindingComputed: true,
	FindingJudged:   true,
}

// FindingDisposition is a deviation finding's pre-merge disposition
// (03 §Gates: "every finding ... carries a disposition: fixed or
// accepted-deviation with a note").
type FindingDisposition string

const (
	FindingFixed             FindingDisposition = "fixed"
	FindingAcceptedDeviation FindingDisposition = "accepted-deviation"
)

var validFindingDispositions = map[FindingDisposition]bool{
	FindingFixed:             true,
	FindingAcceptedDeviation: true,
}

// Finding is one entry in a deviation report's `findings:` block.
type Finding struct {
	ID          string             `yaml:"id"`
	Kind        FindingKind        `yaml:"kind"`
	Text        string             `yaml:"text"`
	Disposition FindingDisposition `yaml:"disposition"`
	Note        string             `yaml:"note,omitempty"`
	// CarriedFrom is spec/finding-identity ac-2's reaffirmation provenance:
	// the covering commit sha at which a human confirmed this disposition as
	// a REAFFIRMATION of a prior ruling under the same judged slug (the same
	// decision, not a fresh escalation) — set only by `verdi disposition`
	// when the finding it is confirming matches a not-resurfaced: entry's
	// own disposition exactly. Deliberately excluded from ComputeDigest's
	// inputs (digestInput only ever reads computed-kind finding id/kind/text
	// — never Disposition/Note/CarriedFrom, all human state) so VerifyDigest
	// is unaffected on every existing frozen archive, and omitempty so every
	// pre-story fixture keeps decoding unchanged. Schema-additive,
	// ac-2/L-N2.
	CarriedFrom string `yaml:"carried-from,omitempty"`
}

// Validate checks ID/Text are present, Kind is a known enum, Disposition is
// either empty (**undispositioned** — a living report's normal state for a
// new or changed finding before human review, PLAN.md Phase 8: "align ...
// marks new/changed findings undispositioned") or a known disposition
// value, and accepted-deviation carries a note (03 §Alignment report: "the
// sanctioned record of how the build diverged from the accepted design").
// An empty Disposition is legal at THIS decode seam deliberately: the merge
// gate — not schema decode — is what enforces "every finding carries a
// disposition" (03 §Gates condition 3), via Dispositioned/AllDispositioned
// below, since a living, mid-build report is a legitimate, decodable
// artifact even while findings remain open.
func (f Finding) Validate() error {
	if f.ID == "" {
		return fmt.Errorf("artifact: finding has no id")
	}
	if f.Text == "" {
		return fmt.Errorf("artifact: finding %s has no text", f.ID)
	}
	if !validFindingKinds[f.Kind] {
		return fmt.Errorf("artifact: finding %s: kind %q is not computed or judged", f.ID, f.Kind)
	}
	if f.Disposition != "" && !validFindingDispositions[f.Disposition] {
		return fmt.Errorf("artifact: finding %s: disposition %q is not a known value", f.ID, f.Disposition)
	}
	if f.Disposition == FindingAcceptedDeviation && f.Note == "" {
		return fmt.Errorf("artifact: finding %s: accepted-deviation requires a note", f.ID)
	}
	if f.CarriedFrom != "" {
		if f.Disposition == "" {
			return fmt.Errorf("artifact: finding %s: carried-from is set but the finding carries no disposition — a candidate awaiting confirmation must never itself carry provenance for a decision not yet made", f.ID)
		}
		if !commitRe.MatchString(f.CarriedFrom) {
			return fmt.Errorf("artifact: finding %s: carried-from %q is not a valid commit sha", f.ID, f.CarriedFrom)
		}
	}
	return nil
}

// Dispositioned reports whether f carries a disposition at all — false is
// the "undispositioned" state Validate legally permits.
func (f Finding) Dispositioned() bool { return f.Disposition != "" }

// AllDispositioned reports whether every finding in fs carries a disposition —
// the merge/closure gate's condition 3 ("every finding ... carries a
// disposition", 03 §Gates) in bool form. An empty slice is trivially all-
// dispositioned. Callers that need to name the offenders (the merge gate's
// user-facing message) iterate Dispositioned themselves; callers that only
// need the yes/no verdict (freeze-in-place eligibility) use this.
func AllDispositioned(fs []Finding) bool {
	for _, f := range fs {
		if !f.Dispositioned() {
			return false
		}
	}
	return true
}

// JudgeIntegrity is the persisted judge exchange a deviation report's
// Integrity hash needs to be self-verifiable (PLAN.md Phase 8, spike S5:
// "Integrity hash = hash of the exact stdin bytes + the raw result
// string"): the exact stdin bytes (base64 — a YAML frontmatter value must
// be a well-formed scalar; the real prompt can run to ~100KB per S5) and
// the raw, untouched judge `result` string. A verifier recomputes the hash
// straight from these two fields plus DeviationFrontmatter.Integrity —
// tamper-evident (editing either field or the rendered judged text without
// updating the hash breaks verification) without needing to re-run the
// judge, which 03 §Alignment report is explicit is never reproducible.
//
// Present iff a genuine judge exchange succeeded — never for the synthetic
// "judged coverage absent" finding, whose content is itself a
// deterministic, computed fact (config presence, failure stage/exit/stderr)
// rather than judge-authored text; see internal/align's doc comment on why
// that finding is digest-, not integrity-, covered despite being tagged
// kind: judged.
type JudgeIntegrity struct {
	StdinB64  string `yaml:"stdin_b64"`
	RawResult string `yaml:"raw_result"`
}

// DeviationFrontmatter is the frontmatter schema for deviation-report.md,
// schema verdi.deviation/v1 (03 §Alignment report). It is decoded via the
// YAML frontmatter seam (the file is markdown, not plain JSON), unlike
// board/evidence/rollup which live in plain JSON files.
type DeviationFrontmatter struct {
	Schema   string    `yaml:"schema"`
	Covers   string    `yaml:"covers"`
	Findings []Finding `yaml:"findings"`
	// NotResurfaced is spec/finding-identity ac-3's persisted archive: a
	// finding dispositioned in a prior report that a fresh regeneration
	// simply does not re-emit (never treated as resolved — a non-
	// reproducible judge failing to re-emit a finding proves nothing about
	// whether the underlying issue is fixed) lands here and stays here
	// across further regenerations until a human explicitly marks it fixed.
	// Every entry here is already dispositioned (Validate enforces it) and
	// disjoint from Findings by id (a finding is either live or archived,
	// never both at once). Exactly two consumers: the disposition pre-fill
	// UI (internal/align.ReconcileJudged reads it back to pair a resurfacing
	// finding with its old ruling) and the spec-stale deviations counterweight
	// (internal/evidence.SpecStale unions it with findings: so a finding
	// that stops reproducing never drains out of the accepted-deviation
	// budget — the X-18 laundering drain). Schema-additive, omitempty so
	// every pre-story fixture keeps decoding unchanged. Renamed from an
	// earlier `resolved:` working name during the design wave (L-N2) — a
	// non-reproducing judge proves nothing resolved.
	NotResurfaced  []Finding       `yaml:"not-resurfaced,omitempty"`
	Digest         string          `yaml:"digest,omitempty"`
	Integrity      string          `yaml:"integrity,omitempty"`
	JudgeIntegrity *JudgeIntegrity `yaml:"judge_integrity,omitempty"`
	Frozen         *Frozen         `yaml:"frozen,omitempty"`
	Provenance     *Provenance     `yaml:"provenance,omitempty"`
}

// DecodeDeviation strict-decodes and validates deviation-report.md
// frontmatter.
func DecodeDeviation(data []byte) (*DeviationFrontmatter, error) {
	var fm DeviationFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the schema literal, Covers is a valid commit sha, every
// finding is individually valid with a unique id, every not-resurfaced:
// entry is individually valid, already dispositioned, unique among
// themselves, and never shares an id with a SAME-KIND dispositioned findings:
// entry (the judged↔judged backing relationship — a cross-kind slug collision
// is legal, judged-reaffirm-judged-kind-scope), Digest/Integrity (if present)
// are well-formed, and Frozen (if present) is well-formed.
func (fm DeviationFrontmatter) Validate() error {
	if fm.Schema != deviationSchema {
		return fmt.Errorf("artifact: deviation schema %q, want %q", fm.Schema, deviationSchema)
	}
	if !commitRe.MatchString(fm.Covers) {
		return fmt.Errorf("artifact: deviation covers %q is not a valid sha", fm.Covers)
	}
	seen := make(map[string]bool, len(fm.Findings))
	dispositionedFindingKind := make(map[string]FindingKind, len(fm.Findings))
	for i, f := range fm.Findings {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("artifact: findings[%d]: %w", i, err)
		}
		if seen[f.ID] {
			return fmt.Errorf("artifact: findings[%d]: duplicate id %q", i, f.ID)
		}
		seen[f.ID] = true
		if f.Dispositioned() {
			dispositionedFindingKind[f.ID] = f.Kind
		}
	}
	// not-resurfaced: (spec/finding-identity ac-3): every entry is already
	// dispositioned (it exists only because a PRIOR report dispositioned it —
	// an undispositioned entry has no prior ruling to persist) and unique
	// among themselves. An id here MAY also appear in findings: — that is
	// exactly the "live candidate + its backing record" shape
	// (align.ReconcileJudged's own doc comment): a slug-only match pre-fills
	// an UNDISPOSITIONED candidate in findings: while its old ruling stays
	// here, verbatim, until a human confirms it. What is never legal is an id
	// that is BOTH dispositioned in findings: AS THE SAME KIND and still
	// present here — a confirmed finding must have had its not-resurfaced
	// backing record removed (cmd/verdi's disposition verb does this), so that
	// shape can only mean a hand-edited or otherwise malformed report. The
	// SAME-KIND scope is load-bearing (judged-reaffirm-judged-kind-scope): the
	// backing relationship is judged↔judged, so a DISPOSITIONED COMPUTED
	// finding sharing an id with a judged not-resurfaced entry is a legitimate
	// cross-namespace slug collision (computed boundary ids and judged boundary
	// slugs share the same shape), not an unremoved backing record — it must
	// decode, never be rejected.
	//
	// The rejection is unconditional within the same kind EXCEPT for a
	// collision-machinery id (IsCollisionMachineryID: a suffixed member or the
	// synthetic contract-violation finding). For a bare, non-collision slug the
	// bare-slug invariant (judged-collision-backing-regeneration-drain) still
	// holds — every member of a colliding slug that owns a backing record is
	// suffixed, so a live member never shadows the backing record's BARE id, and
	// a same-kind overlap there can only mean a hand-edited/malformed report. But
	// that invariant holds ONLY for the bare slug: a prior dispositioned member at
	// a SUFFIXED id whose recurrence reworded away from its frozen text lands in
	// not-resurfaced under that suffixed id while a fresh member re-occupies it
	// live (judged-collision-suffixed-backing-shadow). Confirming that live member
	// legitimately leaves the distinct-content backing record standing for its
	// exit ramp (cmd/verdi's disposition verb, presentation-predicated per L-N13),
	// so a dispositioned collision-machinery findings: entry MAY coexist with a
	// same-id not-resurfaced entry — the sanctioned confirmed-collision-member
	// shape, permitted here from the other side.
	seenNotResurfaced := make(map[string]bool, len(fm.NotResurfaced))
	for i, f := range fm.NotResurfaced {
		if err := f.Validate(); err != nil {
			return fmt.Errorf("artifact: not-resurfaced[%d]: %w", i, err)
		}
		if !f.Dispositioned() {
			return fmt.Errorf("artifact: not-resurfaced[%d]: finding %s carries no disposition — only a previously-dispositioned finding belongs in not-resurfaced", i, f.ID)
		}
		if seenNotResurfaced[f.ID] {
			return fmt.Errorf("artifact: not-resurfaced[%d]: duplicate id %q", i, f.ID)
		}
		seenNotResurfaced[f.ID] = true
		if k, ok := dispositionedFindingKind[f.ID]; ok && k == f.Kind && !IsCollisionMachineryID(f.ID) {
			return fmt.Errorf("artifact: not-resurfaced[%d]: id %q is already dispositioned as a %s finding in findings — a confirmed finding's not-resurfaced backing record must be removed", i, f.ID, f.Kind)
		}
	}
	if fm.Digest != "" && !sha256Re.MatchString(fm.Digest) {
		return fmt.Errorf("artifact: deviation digest %q is not sha256:<64 hex> form", fm.Digest)
	}
	if fm.Integrity != "" && !sha256Re.MatchString(fm.Integrity) {
		return fmt.Errorf("artifact: deviation integrity %q is not sha256:<64 hex> form", fm.Integrity)
	}
	// One-directional only: judge_integrity requires integrity (it exists to
	// let integrity be recomputed), but integrity may legally stand alone —
	// an older or hand-authored frozen report that predates this
	// self-verification record is still a legally decodable artifact; it is
	// simply unverifiable (VerifyIntegrity reports that explicitly rather
	// than silently accepting or rejecting it).
	if fm.JudgeIntegrity != nil && fm.Integrity == "" {
		return fmt.Errorf("artifact: deviation judge_integrity is present but integrity is empty")
	}
	if fm.Frozen != nil {
		if err := fm.Frozen.Validate(); err != nil {
			return fmt.Errorf("artifact: deviation frozen: %w", err)
		}
	}
	if fm.Provenance != nil {
		if err := fm.Provenance.Validate(); err != nil {
			return fmt.Errorf("artifact: deviation provenance: %w", err)
		}
	}
	return nil
}
