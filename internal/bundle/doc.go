// Package bundle assembles a derived evidence bundle — the four files at
// data/derived/<ref-slug>/<commit>/ (01 §Directory layout) — from already
// strict-decoded upstream artifacts (internal/upstream) and a service's
// verdi.bindings.yaml sidecar (internal/artifact). It never execs anything
// itself and never talks to a forge; callers (cmd/verdi/sync.go) wire it to
// internal/upstream (regeneration) or internal/forge (a pulled CI bundle).
//
// PLAN.md I-3, as revised by spike S1:
//
//   - verdicts.json: verdi.evidence/v1 records synthesized from a graph's
//     obligations[] joined against a service's bindings (static kind), plus
//     coarse behavioral records from a go test -json suite summary. A
//     binding naming an unknown producer or an AC the bound spec does not
//     declare is a hard error (03 §Declarations: "dangling bindings are
//     errors, not empty cells"); an UNMATCHED obligation is likewise a hard
//     error, never a silent abstain.
//   - tests.json: a small verdi-owned schema (verdi.tests/v1, not an
//     upstream shape) summarizing `go test -json` output, per 03
//     §Declarations's explicit design choice to keep unit-test evidence
//     coarse (suite pass/fail) rather than inventing a per-test-to-AC join
//     that would rot.
//   - review.json: the strict-decoded upstream Review record(s), verbatim
//     (every field preserved unchanged) — an array to accommodate more
//     than one impacted service, each entry self-identifying via its own
//     embedded Service field.
//   - boundary-diff.json: internal/upstream.ComputeBoundaryDiff's output,
//     concatenated across every impacted service (PLAN.md I-3's
//     {op,surface,name,breaking} shape carries no service field, so v0
//     does not attribute diff entries to a service when more than one is
//     impacted — disclosed simplification, adequate for the single-service
//     fixture this build targets).
//
// Every file is written via internal/canonjson for byte-identical, sorted
// output.
package bundle
