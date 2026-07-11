// Package decisionsweep implements 03 §Exemption audit: per-ADR exemption
// backlinks computed over every live `exempts` edge in the corpus,
// deterministic threshold-triggered conflict auto-filing (a fold over
// committed records, "no judgment, no LLM"), and the orchestration behind
// `verdi audit` (05 §CLI, R4-I-10), which also surfaces V1-P3's
// internal/evidence.SpecStale counts against
// audit.deviations_stale_threshold.
//
// cmd/verdi/audit.go is the verb entry point; this package owns no CLI
// parsing or exit codes, only the scan/plan/file pipeline and its own file
// I/O for auto-filing (§Challenging closed decisions: filing a conflict
// record IS this package's job, not merely computing one).
package decisionsweep
