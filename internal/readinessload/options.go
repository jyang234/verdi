package readinessload

import (
	"context"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
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

// ConflictProviderFunc constructs a policy-conflict VerdictProvider for one
// request — the same shape NewConflictProvider itself has, minus the
// mode/actors parameters a caller-supplied provider has already baked in
// (or has no use for). It is the seam Options.ConflictProvider carries.
type ConflictProviderFunc func(ctx context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error)

// PredecodedRequest bundles an already-read, already-decoded
// --context-request's exact canonical bytes and decoded value, for
// Options.PredecodedRequest — the "exactly one canonical read" seam a
// caller that has already called ContextRequestSpec (to learn Load's own
// required ref parameter) uses to avoid a second read of the same file.
// Supplying it skips Load's own read AND its ValidatedContextRequestPath
// safety check (the ".."/symlink refusal): the path check is the
// caller's too, already performed by the ContextRequestSpec call that
// produced this bundle, exactly once, before Load ever runs.
type PredecodedRequest struct {
	// Bytes is the exact byte sequence RequestDigest is computed from —
	// never re-encoded or reformatted.
	Bytes []byte
	// Request is Bytes decoded through contextcompile.DecodeRequest.
	Request contextcompile.Request
}

// Options is every optional posture Load needs beyond (root, ref).
type Options struct {
	// ContextRequestPath is the optional --context-request file naming an
	// acceptance-candidate policy-conflict evaluation for ONE spec
	// (R-RR1-5). Empty means no conflict request is synthesized — inventing
	// adapter, grants, or scope would be authority the store never
	// declared — and the check-context area carries exactly one unproven
	// context/verdict concern instead. A supplied request binds to its own
	// spec only: a Load for any OTHER ref derives exactly as if this field
	// were empty (R-RR1-15), so the ONE loader `verdi serve
	// --context-request` threads into the board, the Document tab and MCP
	// still serves every active spec, and each of those specs reads
	// identically through it and through a caller carrying no request at
	// all (ac-4). Still required (for its own destination text and identity
	// checks) even when PredecodedRequest is also set.
	ContextRequestPath string
	// PredecodedRequest, when non-nil, is the exact bytes and decoded value
	// of the file ContextRequestPath names — supplied by a caller that
	// already read it once (ContextRequestSpec returns this bundle) so Load
	// does not read it a second time. Load still validates phase/spec
	// identity against it exactly as it would a freshly read request; only
	// the read itself is skipped. Leave nil for the ordinary case (Load
	// reads ContextRequestPath itself).
	PredecodedRequest *PredecodedRequest
	// RequireExpectedMatch turns a supplied request's optional `expected`
	// branch/HEAD claim that no longer matches the checkout back into an
	// operational error, instead of R-RRF-3's (SI-216) disclosure. It is
	// the one knob that separates the two postures, and it is explicit
	// precisely because the two callers need opposite answers:
	//
	//   false (every PER-REQUEST load — the useful zero value): the
	//   mismatch is the ac-3 ConflictUnavailable posture. `verdi serve`
	//   keeps deriving against the request bundle it validated at startup
	//   (R-RR1-17), so one ordinary commit or branch change makes that
	//   pinned claim stale; refusing there returned a 503 readiness page
	//   and dropped the Readiness section from the Document tab and MCP for
	//   the startup spec until the server was restarted, even though every
	//   area except check-context still derived perfectly. False derives
	//   all of them and discloses the context area unproven with a fixed
	//   witness naming the request's expected repository and the
	//   repository's current one; `expected` is never rebound.
	//
	//   true (ONLY `verdi serve`'s startup warm-up, cmd/verdi/serve.go's
	//   readinessLoadBuilder.Build): a request that is ALREADY stale when
	//   the server starts is a misconfiguration the operator must see at
	//   once, so the warm-up still refuses it and serve exits 2 rather than
	//   starting against a request that can never be honoured.
	RequireExpectedMatch bool
	// BoardHref computes a design branch's board href for an unresolved
	// shape concern's destination (R-RR1-6: this package never imports
	// internal/workbench, so it never calls workbench.BranchBoardHref
	// itself). nil leaves Derive's own required CLI fallback vector in
	// place instead of rewriting it to a board path.
	BoardHref func(branch, name string) string
	// Judge selects how a supplied context request's conflict evaluation
	// may reach a semantic judgment. The zero value behaves as
	// JudgeCacheOnly. Ignored when ConflictProvider is set (the caller's
	// own provider owns that decision).
	Judge JudgeMode
	// Actors resolves the store's opt-in local-operator actor claim. nil
	// resolves no actors. Ignored when ConflictProvider is set.
	Actors ActorsResolver
	// ConflictProvider, when non-nil, replaces NewConflictProvider as the
	// policy-conflict VerdictProvider construction Load uses for a
	// supplied context request — Load's own hermetic test seam, exported
	// so a consumer (cmd/verdi's serve.go, and Task 3's workbench/MCP
	// wiring) and ITS OWN tests can inject a fake or in-process-only
	// provider instead of exercising the real align.judge_cmd process
	// transport (fix round 1, Important 2: readinessload previously had no
	// hermetic seam of its own, forcing every consumer test that needed a
	// specific conflict outcome to either replicate cmd/verdi's deleted
	// fake-provider machinery or spawn a real judge subprocess). nil (the
	// ordinary case) uses NewConflictProvider under Judge/Actors above. A
	// PRODUCTION caller that sets this (as opposed to a test) takes over
	// ac-3's no-launch guarantee for its own provider: Load itself never
	// launches a judge through this seam, so a production ConflictProvider
	// that does so owns that decision and its consequences, not this
	// package.
	ConflictProvider ConflictProviderFunc
}
