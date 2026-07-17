package artifact

// This file is the stamp seam (L-M4, extensibility-phase1 plan Task 4):
// the single place a frozen-artifact mint routes through, replacing what
// had been five-going-on-six hand-rolled `artifact.Frozen{...}` literals
// scattered across align, commitdesign, workbench, and cmd/verdi — one of
// which (internal/workbench/obligationauthor.go) had silently drifted into
// a wall-clock determinism violation (CLAUDE.md: "no wall-clock or
// randomness in generated artifacts except declared stamps"). See
// stamp_test.go.

// NewFrozen constructs a Frozen stamp — the seam every frozen-artifact mint
// now routes through. at and commit must already be resolved by the
// caller: at is typically a YYYY-MM-DD date derived from the covered
// commit's own committer date (internal/gitx.CommitDateOnly), never wall
// clock. The one documented exception is the attestation scaffold's
// deliberately-mutable "today" convenience (internal/evidence's
// AttestationScaffold.Frozen doc comment, dc-2/ADJ-30): an unauthored,
// pre-first-commit convenience stamp the operator is expected to correct
// before their first commit, not a permanent frozen record, so wall-clock
// "now" there is a documented design choice rather than the determinism bug
// this seam otherwise closes.
//
// NewFrozen panics if at or commit is empty: a structurally empty stamp is
// always a caller bug, never a runtime condition, and this constructor's
// signature — a bare Frozen, no error return — has no other way to fail
// closed (CLAUDE.md: "never fake success"). Format validation (date shape,
// commit hex shape) stays Frozen.Validate's job, invoked downstream at
// self-validation time exactly as it always has been; this constructor only
// guards against an uninitialized stamp.
func NewFrozen(at, commit string) Frozen {
	if at == "" {
		panic("artifact: NewFrozen: at must not be empty")
	}
	if commit == "" {
		panic("artifact: NewFrozen: commit must not be empty")
	}
	return Frozen{At: at, Commit: commit}
}

// StampProvenance stamps p with the resolved operating model's digest
// (L-M5, extensibility-phase1 plan Task 10): once Provenance gains a Model
// field, every Provenance-minting call site is meant to write it through
// this one seam, so that later task changes only the value each caller
// passes — never a call site's structure.
//
// Provenance carries no Model field yet, so modelDigest is unused and
// StampProvenance is a documented no-op today: it exists so the signature
// is fixed once, ahead of Task 10 wiring it into the Provenance-minting
// call sites and passing store.Config.Model.Digest() (until then, any
// caller that does adopt this seam passes ""). p must be non-nil —
// StampProvenance panics otherwise, the same fail-closed posture NewFrozen
// takes on an empty stamp.
func StampProvenance(p *Provenance, modelDigest string) {
	if p == nil {
		panic("artifact: StampProvenance: p must not be nil")
	}
}
