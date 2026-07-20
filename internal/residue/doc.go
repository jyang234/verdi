// Package residue implements spec/closure-hygiene's scan: `verdi audit`'s
// third, additive report section (dc-1) naming every git-reality-versus-
// spec-status contradiction (AC-1), every stranded `close/<name>` branch
// (AC-2), and a read-only survey of merged-but-undeleted branches and
// worktrees (AC-3) — never a guess where git state cannot decide, never a
// mutation.
//
// cmd/verdi/audit.go is the sole consumer: it resolves root and the
// default branch ref, calls Scan, and renders the result alongside the
// existing exemption/spec-stale sections (internal/decisionsweep, entirely
// untouched by this package — co-2). This package owns no CLI parsing, no
// exit codes, and no display-vocabulary resolution, matching
// internal/decisionsweep's own "no CLI parsing" posture (its doc.go).
//
// Reclamation of any kind (deleting a branch, removing a worktree) is
// explicitly out of scope (spec/closure-hygiene dc-5) — every function in
// this package is read-only, proven by AC-3's exhaustive command-surface
// check (survey_test.go).
package residue
