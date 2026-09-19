package readinessload

import (
	"context"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// JudgeMode selects how a supplied --context-request's policy-conflict
// evaluation may reach a semantic judgment (spec/readiness-recovery ac-3).
type JudgeMode string

const (
	// JudgeCacheOnly answers only from the D4 judgment cache
	// (policyconflict.NewCacheOnlyJudge): a request never launches the
	// judge command. It is Options' useful zero value — every per-request
	// Load call defaults to it even when Judge is left unset.
	JudgeCacheOnly JudgeMode = "cache-only"
	// JudgeRun may launch the manifest's configured judge command on a
	// cache miss (policyconflict.JudgeAdapter), publishing its result to
	// the D4 cache. Reserved for `verdi serve`'s own startup pre-run and
	// the `verdi context conflict` CLI verb — never a per-request Load
	// call a server makes on a caller's behalf.
	JudgeRun JudgeMode = "run"
)

// normalize treats every value but the exact JudgeRun spelling — including
// the zero value — as the safe JudgeCacheOnly default, so a caller can
// never accidentally enable process execution by leaving Options.Judge
// unset.
func (m JudgeMode) normalize() JudgeMode {
	if m == JudgeRun {
		return JudgeRun
	}
	return JudgeCacheOnly
}

// ActorsResolver resolves the store's opt-in local-operator actor claim
// against a governance profile (2026-09-05 local-operator disposition
// design §2.1-§2.2, ledger SI-183). cmd/verdi's actorlocal.go owns the only
// implementation in this build: it depends on policy-adopt/disposition
// helpers (cmd/verdi/policy.go) that stay in cmd/verdi, so this package
// treats actor resolution as a caller-supplied port rather than
// reimplementing or importing that identity logic. nil resolves no actors
// — the same posture a store whose profile declares no local-operator
// trust source already has today.
type ActorsResolver func(ctx context.Context, root string) ([]governanceprincipal.PrincipalResolution, error)

// Options is every optional posture Load needs beyond (root, ref).
type Options struct {
	// ContextRequestPath is the optional --context-request file naming an
	// acceptance-candidate policy-conflict evaluation for this ref
	// (R-RR1-5). Empty means no conflict request is synthesized — inventing
	// adapter, grants, or scope would be authority the store never
	// declared — and the check-context area carries exactly one unproven
	// context/verdict concern instead.
	ContextRequestPath string
	// BoardHref computes a design branch's board href for an unresolved
	// shape concern's destination (R-RR1-6: this package never imports
	// internal/workbench, so it never calls workbench.BranchBoardHref
	// itself). nil leaves Derive's own required CLI fallback vector in
	// place instead of rewriting it to a board path.
	BoardHref func(branch, name string) string
	// Judge selects how a supplied context request's conflict evaluation
	// may reach a semantic judgment. The zero value behaves as
	// JudgeCacheOnly.
	Judge JudgeMode
	// Actors resolves the store's opt-in local-operator actor claim. nil
	// resolves no actors.
	Actors ActorsResolver
}
