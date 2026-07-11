// Package align implements `verdi align` (PLAN.md Phase 8, 05 §CLI; 03
// §Alignment report): generating deviation-report.md's two sections —
// computed (regenerated graph/boundary contract vs the spec's declares:
// block, digest-locked) and judged (a configurable judge command's semantic
// reading, integrity-hashed per spike S5) — folding findings from both into
// one report with per-finding dispositions preserved across regeneration.
//
// cmd/verdi/align.go and cmd/verdi/gate.go are the verb entry points; this
// package owns no CLI parsing or exit codes, only report generation and the
// digest/integrity verification `verdi gate` and a future `verdi
// verify-artifact` both need.
package align
