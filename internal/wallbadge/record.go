package wallbadge

// InputRecord is one pinned input a derivation record cites (spec/badge-
// computes dc-2). Name identifies the input's role in the compute that
// read it (e.g. "spec", "deviation-report", "candidate:<mr-id>"); Path is
// the store-relative file it was read from; Revision is its content
// digest ("sha256:<hex>" over the exact bytes read) or an already-pinned
// digest/sha field the compute carries verbatim (dc-5) — NEVER wall-clock
// time, and never a bare mutable ref like an MR id (that belongs in
// Records instead).
type InputRecord struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Revision string `json:"revision"`
}

// DerivationRecord is the canonical badge derivation schema (spec/badge-
// computes dc-2), load-bearing for every sibling wall-receipts story
// (evidence-slot, case-file-flags add their own computes through this
// same shape, dc-1). Source is a namespaced rule id ("lint:VL-006",
// "ladder:spec-stale", "ladder:pending-supersession", and, reserved for
// sibling stories, "fold:empty-slot"/"observe:size-smell"). Label is the
// chip's short text. Target is the object id a card badge anchors to, or
// "" for a case-file badge. Inputs are every pinned input the compute
// read. Records are one entry per firing record (finding ids/messages, MR
// ids, touched object ids) — receipts, not verdicts. Disclosures name any
// input the compute could not prove (three-valued honesty: named, never
// silent) — a record with only Disclosures and no Records is legitimate
// (ac-3's disclosed-unproven outcome renders as a disclosure, never a
// badge, so in practice such a record is never attached to a card; see
// ComputeBadges).
//
// Every compute in this package builds a DerivationRecord with fully
// sorted Inputs/Records — ac-4's byte-identical-across-renders
// requirement holds because construction order never depends on Go map
// iteration order, not because of any serialization-time canonicalization
// step.
type DerivationRecord struct {
	Source      string        `json:"source"`
	Label       string        `json:"label"`
	Target      string        `json:"target,omitempty"`
	Inputs      []InputRecord `json:"inputs"`
	Records     []string      `json:"records"`
	Disclosures []string      `json:"disclosures,omitempty"`

	// Provenance is an optional pinned-provenance block the derivation
	// drawer stamps once at its head (spec/derivation-drawer ac-3/dc-2) —
	// added HERE, at the record schema's one defining seam, per that
	// story's co-3 (the drawer never side-channels a datum the record
	// lacks). Each line is built ONLY from already-pinned fields of a
	// compute's own inputs (the judged sweep's covers sha,
	// adr_corpus_digest, decisions_scanned) — never wall-clock time, never
	// a recomputation. Most computes supply none (the zero value): a
	// record without a provenance block renders a drawer without one.
	Provenance []string `json:"provenance,omitempty"`
}
