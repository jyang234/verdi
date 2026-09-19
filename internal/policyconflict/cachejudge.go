// cachejudge.go is the AC-3 cache-only judge (spec/readiness-recovery ac-3,
// plan R-RR1-4): a Judge that answers exclusively from the D4 immutable
// judgment cache (cache.go) and never runs a judge process. It reuses
// CachedJudge's own key/path/symlink-refusal/witness-cross-check machinery
// (judgeCacheKeyDigest, store.PolicyConflictCachePath,
// refuseManagedCacheSymlinks, loadCachedJudgment, ValidateJudgeResult) for
// its hit-check, and never touches the D3 writer lock: a miss has nothing
// to publish.
//
// runValidatedJudge (service.go) recognizes the concrete cacheOnlyJudge type
// exactly as it already recognizes JudgeAdapter/*JudgeAdapter, and supplies
// it the SAME tree-hash/profile/authority axes a real run's CachedJudge call
// uses (cache.treeHash, view.Profile.ID, view.Snapshot.ProfileDigest,
// view.Snapshot.EffectivePolicyDigest) — this is what makes R-RR1-4's claim
// true in practice: "the cache key is computed by the same
// judgeCacheKeyDigest path a real run uses, so a judgment cached by `verdi
// context conflict` or by `serve --context-request` is found by a later
// request." The plain two-argument Judge method below is reached only by a
// caller with no resolved ConflictView of its own (this package's own
// direct unit test); it uses a fixed, inert tree hash and empty
// profile/authority axes, documented at cacheOnlyJudge and never reached by
// a Service-driven evaluation.
package policyconflict

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/store"
)

// ErrJudgeCacheMiss reports that no cached judgment exists for this exact
// (adapter, prompt, input, tree-hash, profile, authority) input.
// NewCacheOnlyJudge never runs adapter.Argv to produce one; the service
// treats a miss exactly like a nil judge (authority design's
// judge-unavailable posture), never as an operational failure.
var ErrJudgeCacheMiss = errors.New("policyconflict: no cached judgment for this input")

// standaloneTreeHash/standaloneProfileID/standaloneProfileDigest/
// standaloneAuthorityDigest are the fixed, inert placeholder axes
// cacheOnlyJudge.Judge uses when reached directly, with no Service-resolved
// D4 tree hash or governance profile of its own. Each is well-formed for
// its field's own grammar (bare 64-hex, a legal governanceprincipal ID,
// full sha256:<64 hex>) so a seeded Judgment record still encodes/decodes
// through this package's ordinary strict codec; none can collide with a
// real resolved axis in a way that matters, because a Service-driven
// evaluation never reaches these values (see the file doc comment above).
var (
	standaloneTreeHash        = strings.Repeat("0", 64)
	standaloneProfileID       = "unresolved"
	standaloneProfileDigest   = "sha256:" + strings.Repeat("0", 64)
	standaloneAuthorityDigest = "sha256:" + strings.Repeat("0", 64)
)

// cacheOnlyJudge is the "one wrapper" R-RR1-4 costs: a Judge answering
// exclusively from the D4 cache adapter.Root holds.
type cacheOnlyJudge struct{ adapter JudgeAdapter }

// NewCacheOnlyJudge answers only from the judge cache adapter.Root holds: a
// hit returns the validated exchange exactly as CachedJudge would; a miss
// returns ErrJudgeCacheMiss and never runs adapter.Argv. The service treats
// a miss as "no judgment" (unproven, judge-unavailable).
func NewCacheOnlyJudge(adapter JudgeAdapter) Judge {
	return cacheOnlyJudge{adapter: adapter}
}

// Judge implements the plain Judge port for a caller with no resolved
// ConflictView of its own — see the file doc comment for why a
// Service-driven evaluation never reaches this method and instead gets the
// real tree-hash/profile/authority axes through runValidatedJudge's own
// cacheOnlyJudge case.
func (j cacheOnlyJudge) Judge(ctx context.Context, prompt, input []byte) (JudgmentExchange, error) {
	if err := ctx.Err(); err != nil {
		return JudgmentExchange{}, err
	}
	semInput, err := semanticInputFromNormalizedBytes(prompt, input)
	if err != nil {
		return JudgmentExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	validated, err := cacheOnlyLookup(j.adapter, semInput, standaloneTreeHash, standaloneProfileID, standaloneProfileDigest, standaloneAuthorityDigest)
	if err != nil {
		return JudgmentExchange{}, err
	}
	return validated.Exchange, nil
}

// semanticInputFromNormalizedBytes reconstructs enough of a SemanticInput
// for cacheOnlyLookup's witness cross-check from the exact bytes the Judge
// port receives: prompt is carried as-is, and input is decoded back through
// the same semanticInputWitnessDoc shape CachedJudge/runValidatedJudge
// encode it with (a plain, undecorated JSON shape — see semantic.go).
// Decoded strictly (artifact.DecodeExactJSON: unknown fields, trailing
// data, and duplicate keys all refused) per CLAUDE.md's "JSON via
// DisallowUnknownFields + trailing-data rejection" — stated without a
// test-only-path exception, and this reconstruction feeds a witness
// cross-check, so a permissively decoded extra or duplicate field could
// otherwise pass silently instead of failing closed.
func semanticInputFromNormalizedBytes(prompt, input []byte) (SemanticInput, error) {
	var shown semanticInputWitnessDoc
	if err := artifact.DecodeExactJSON(input, &shown); err != nil {
		return SemanticInput{}, fmt.Errorf("decoding normalized semantic input: %w", err)
	}
	return SemanticInput{
		Prompt: prompt, Claims: shown.Claims,
		UnknownMechanicals: shown.UnknownMechanicals, Exemptions: shown.Exemptions,
	}, nil
}

// cacheOnlyLookup performs CachedJudge's own hit-check ONLY (authority
// design §7's read path: strict-decode, canonical-reencode, path-key
// verification, witness cross-check), reusing its exact key/path/symlink-
// refusal machinery, and NEVER falls through to adapter.Judge on a miss —
// it reports ErrJudgeCacheMiss instead. It never touches the D3 writer
// lock: there is nothing to publish on a miss.
//
// Unlike CachedJudge, it does NOT validate adapter's own operands (role,
// adapter.id/version/model, argv, or the profile id/digests) before
// computing the key — those checks exist to fail malformed CONFIGURATION
// loudly before a real judge process would otherwise run, and cache-only
// mode never runs one. A malformed adapter here simply computes a key that
// cannot match any real entry, so it degrades to loadCachedJudgment's own
// "not found" path: ErrJudgeCacheMiss, and from there the same unproven/
// judge-unavailable posture a genuinely absent cache entry gets — never an
// operational error, and never a fabricated pass, where CachedJudge (a real
// run) would report ErrCacheOperational instead. Key parity with a real
// run (R-RR1-4) is unaffected either way, since judgeCacheKeyDigest itself
// performs no validation.
func cacheOnlyLookup(adapter JudgeAdapter, input SemanticInput, treeHash, profileID, profileDigest, authorityDigest string) (ValidatedExchange, error) {
	inputBytes, err := canonjson.Marshal(semanticInputWitnessDoc{
		Claims: input.Claims, UnknownMechanicals: input.UnknownMechanicals, Exemptions: input.Exemptions,
	})
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: encoding normalized semantic input: %v", ErrCacheOperational, err)
	}
	bareKeyDigest, err := judgeCacheKeyDigest(adapter, input.Prompt, inputBytes, profileID, profileDigest, authorityDigest)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	path, err := store.PolicyConflictCachePath(adapter.Root, treeHash, bareKeyDigest)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	if err := refuseManagedCacheSymlinks(adapter.Root, path); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}

	judgment, found, err := loadCachedJudgment(path, treeHash, bareKeyDigest)
	if err != nil {
		return ValidatedExchange{}, err
	}
	if !found {
		return ValidatedExchange{}, ErrJudgeCacheMiss
	}
	if _, err := ValidateJudgeResult(input, judgment.Exchange.Result); err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: cached judgment result is invalid for the exact semantic input: %v", ErrCacheOperational, err)
	}
	recordDigest, err := semanticInputDigest(input)
	if err != nil {
		return ValidatedExchange{}, fmt.Errorf("%w: %v", ErrCacheOperational, err)
	}
	return ValidatedExchange{Exchange: judgment.Exchange, RecordDigest: recordDigest}, nil
}
