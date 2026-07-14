// Package runtime implements spec/runtime-evidence's scheduled-probe
// mechanism (dc-1), resolving true-closure's oq-1 and delivering its own
// ac-3: every evidence kind a spec can declare — runtime included — needs a
// producing mechanism queryable by (story, AC) at close time (03 §Runtime
// evidence residence).
//
// The package is deliberately small (dc-1: "no OTel pipeline or trace-ingest
// infrastructure is stood up" — those stay open as future producers behind
// the same binding seam, 03 §Pluggable evidence):
//
//   - Emit builds one well-formed kind: runtime artifact.Evidence record for
//     a (story, AC) pair, stamping provenance.source: ci only in genuine CI
//     (dc-3, the same D6-10 discipline cmd/verdi/sync.go's --produce already
//     applies) and provenance.source: local otherwise.
//   - Query filters an already-loaded record set down to the ones bound to
//     one (story, AC) pair — the "queryable by (story, AC)" hard constraint
//     (co-2; true-closure oq-1: "what does its (story, AC)-queryable store
//     look like?").
//
// No artifact.Evidence schema change was needed or made (co-1's contract:
// "internal/artifact/evidence.go already decodes kind: runtime + source: ci
// — no schema change needed"): the (story, AC) key rides entirely on two
// EXISTING fields — EvidenceFor (the AC half) and Producer (the story half,
// via CheckID's deterministic derivation) — so a runtime record is, on the
// wire, indistinguishable in shape from any other verdi.evidence/v1 record;
// only Kind and the Producer naming convention mark it as this mechanism's
// output. This is also why Query needs no new store or index: it is a pure
// filter over whatever internal/evidence.LoadRecords already loaded (once
// LoadRecords also reads runtime.json — see that package's records.go).
//
// HONEST SCOPE (dc-3). verdi itself has no live service to probe, so this
// package's mechanism produces nothing meaningful for verdi's own specs:
// there is no verdi.bindings.yaml entry and no CLI invocation anywhere in
// this repo's own CI that feeds Emit a real (story, AC) verdict for a verdi
// spec, and none should ever be added purely to make a story's fold go
// green — that would be exactly the fabricated "passing" record dc-3
// forbids. The mechanism's correctness is instead proven on a FIXTURE story
// (cmd/verdi/runtimeprobe_test.go's end-to-end test, and internal/evidence's
// fold tests). A real service with a real check plugs its own probe in
// behind this seam by invoking `verdi sync --produce-runtime` with its own
// genuine verdict (cmd/verdi/runtimeprobe.go) — mirroring 03 §Pluggable
// evidence's precedent for the static/behavioral kinds: "No adapter design
// is specified here — this section states the seam and the principle only;
// the producer side is out of scope for this contract."
package runtime
