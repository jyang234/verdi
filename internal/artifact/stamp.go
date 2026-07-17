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
// (L-M5, spec/model-digest): the ONE seam every production
// Provenance-minting call site routes through to set Model — never inline
// in the struct literal the way Digest/Integrity are set (ac-2's "one
// seam, no surviving copies" static convention). Callers resolve
// modelDigest once, at their own cmd/verdi (or workbench HTTP handler)
// entry point via store.Open(...).Model.Digest(), and thread it down as a
// plain string — never re-derived deep inside internal/align or
// internal/commitdesign, and never by importing internal/model there
// (internal/model already imports internal/artifact, so the reverse
// import would cycle).
//
// Both p and modelDigest must be non-empty — StampProvenance panics
// otherwise, mirroring NewFrozen's own "a structurally empty stamp is
// always a caller bug, never a runtime condition" posture (this file's own
// doc comment above): every production call site already has a real,
// non-nil *store.Config (store.Open's guarantee — Config.Model is never
// nil, model-schema), so an empty modelDigest reaching here can only be a
// caller wiring bug, never a legitimate runtime condition to tolerate
// silently.
func StampProvenance(p *Provenance, modelDigest string) {
	if p == nil {
		panic("artifact: StampProvenance: p must not be nil")
	}
	if modelDigest == "" {
		panic("artifact: StampProvenance: modelDigest must not be empty")
	}
	p.Model = modelDigest
}
