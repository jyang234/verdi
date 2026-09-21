// Package readinessload derives the readiness projection
// (internal/readinesspilot) for any active feature or story spec, on any
// branch, at the checkout's current HEAD (spec/readiness-recovery ac-2).
// Load is the per-request successor to cmd/verdi's old startup-only
// readiness_snapshot.go adapter: the same ordered pipeline — resolve the
// target spec, gather provenance/mutation/board facts, project the journey,
// optionally evaluate an already-authored --context-request file's
// acceptance-candidate policy conflict — now runnable for any target, not
// only the one spec `verdi serve --context-request` named at startup on its
// own design branch.
//
// A supplied --context-request's conflict evaluation never launches the
// judge command from a request (ac-3): it answers exclusively from the D4
// judgment cache through policyconflict.NewCacheOnlyJudge. An uncached
// semantic candidate resolves the context area's concern unproven,
// disclosing judge-unavailable, with the `verdi context conflict --request`
// destination — never a synthesized pass and never a launched process.
// Only `verdi serve`'s own startup pre-run and the `verdi context conflict`
// CLI verb use JudgeRun, the mode that may execute the configured judge on
// a miss and publish its result.
//
// NewConflictProvider is the moved policy-conflict provider construction
// (cmd/verdi/context_conflict.go's old newLocalContextConflictProvider):
// cmd/verdi's context-conflict-evaluating verbs and this package's own Load
// both call it, so there is exactly one home for that wiring.
// ValidatedContextRequestPath is the moved --context-request path safety
// check (cmd/verdi/conflictgate.go's old validatedConflictRequestPath),
// shared the same way.
//
// Loader is the consumer-side port every surface (workbench, MCP, cmd/verdi
// itself) defines for itself; it is the production value closing over one
// checkout root and one Options posture.
package readinessload
