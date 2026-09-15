package specimport

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// Service wires the application-level spec-import operations
// (spec-import-contract.md, "Shared internal interfaces": "NewService()
// *Service wires the production application dependencies"). Every
// consumer-owned port is an exported field with a safe production default,
// so a hermetic test constructs a Service literal directly rather than
// needing a second constructor.
type Service struct {
	// Engine resolves engine_digest. NewService wires the real running
	// binary; hermetic tests inject a fixed value.
	Engine EngineIdentity
	// Policy resolves design_assistance authority for Apply's actor-policy
	// check, reusing draftmutation's own dispatcher unchanged.
	Policy draftmutation.PolicySource
	// Faults injects deterministic failure points around Git publication
	// for hermetic fault-injection tests (publish.go). The zero value never
	// injects a fault.
	Faults PublishFaults
}

// NewService returns the production Service: the actual running binary's
// identity and the existing constitution-backed policy source.
func NewService() *Service {
	return &Service{
		Engine: binaryEngineIdentity{},
		Policy: draftmutation.ConstitutionPolicySource{},
	}
}

// checkCleanContext refuses a dirty tracked checkout/index or an untracked,
// non-ignored path (spec-import-contract.md: "Prepare requires a clean
// tracked checkout/index and no untracked corpus/config inputs (ignored
// .verdi/data is allowed); it refuses with correction guidance instead of
// reading an uncommitted context that cannot be reproduced. This first
// version may refuse unrelated tracked edits; it never resets them").
// gitx.WorktreeChangedPaths never reports a path .gitignore excludes (no
// --ignored flag), so an ignored .verdi/data entry never reaches here.
func (s *Service) checkCleanContext(ctx context.Context, root string) error {
	changed, err := gitx.WorktreeChangedPaths(ctx, root)
	if err != nil {
		return fmt.Errorf("%w: checking checkout cleanliness: %v", ErrIOFailure, err)
	}
	if len(changed) > 0 {
		return fmt.Errorf("%w: checkout has %d uncommitted or untracked path(s) (e.g. %q); commit, stage, or remove them before preview/apply — nothing is reset automatically", ErrDirtyContext, len(changed), changed[0])
	}
	return nil
}

// resolveConfigDigest binds the committed manifest, model/template
// overrides and dependency corpus used in validation (spec-import-
// contract.md: "config digest binds the committed manifest, model/template
// overrides and dependency corpus used in validation"), reusing the
// existing D4 corpus tree hasher — never a second corpus-hashing algorithm.
func resolveConfigDigest(ctx context.Context, root string) (string, error) {
	services, err := store.DiscoverServices(root)
	if err != nil {
		return "", fmt.Errorf("%w: discovering services for config digest: %v", ErrIOFailure, err)
	}
	digest, err := store.TreeHash(ctx, root, services)
	if err != nil {
		return "", fmt.Errorf("%w: computing config digest: %v", ErrIOFailure, err)
	}
	return digest, nil
}

// previewBindings is every deterministic identity fact Preview and Apply's
// internal preview recomputation share, resolved once behind the clean-
// context gate.
type previewBindings struct {
	baseCommit    string
	modelDigest   string
	configDigest  string
	engineDigest  string
	requestDigest string
}

// resolvePreviewBindings resolves base_commit/model/config/engine/request
// digests (spec-import-contract.md: "The base is current HEAD. Model digest
// uses model.Model.Digest. config digest binds ..."). It must run only
// after Normalize has validated request shape and checkCleanContext has
// passed — never reading model/template/corpus state from a checkout that
// cannot be reproduced.
func (s *Service) resolvePreviewBindings(ctx context.Context, root string, request Request) (previewBindings, error) {
	if s.Engine == nil {
		return previewBindings{}, fmt.Errorf("%w: service has no engine identity configured", ErrIOFailure)
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		return previewBindings{}, fmt.Errorf("%w: resolving HEAD: %v", ErrIdentityUnavailable, err)
	}
	cfg, err := store.Open(root)
	if err != nil {
		return previewBindings{}, fmt.Errorf("%w: opening store configuration: %v", ErrInvalidModel, err)
	}
	modelDigest, err := cfg.Model.Digest()
	if err != nil {
		return previewBindings{}, fmt.Errorf("%w: digesting operating model: %v", ErrInvalidModel, err)
	}
	configDigest, err := resolveConfigDigest(ctx, root)
	if err != nil {
		return previewBindings{}, err
	}
	engineDigest, err := s.Engine.Digest(ctx)
	if err != nil {
		return previewBindings{}, fmt.Errorf("%w: resolving engine identity: %v", ErrIOFailure, err)
	}
	canonicalRequest, err := canonjson.Marshal(request)
	if err != nil {
		return previewBindings{}, fmt.Errorf("%w: canonicalizing request for digest: %v", ErrInvalidRequest, err)
	}
	return previewBindings{
		baseCommit:    head,
		modelDigest:   modelDigest,
		configDigest:  configDigest,
		engineDigest:  engineDigest,
		requestDigest: sha256Hex(canonicalRequest),
	}, nil
}

// Preview is read-only (spec-import-contract.md, "Preview, identity and
// atomic publication"). It Normalizes the request (validating shape first),
// requires a clean context, then builds the digest-bound result. It never
// writes anything.
func (s *Service) Preview(ctx context.Context, root string, request Request) (PreviewResult, error) {
	plan, err := Normalize(request)
	if err != nil {
		return PreviewResult{}, err
	}
	if err := s.checkCleanContext(ctx, root); err != nil {
		return PreviewResult{}, err
	}
	return s.buildPreviewResult(ctx, root, request, plan)
}

// buildPreviewResult is the ONE shared preview-construction algorithm
// Preview and Apply's internal recomputation both call (spec-import-
// contract.md's plan: "production must retain the one shared validation/
// mutation/authority algorithm"), so the two can never diverge on what
// "ready" or a digest means. Callers must already have validated request
// shape (Normalize) and checked a clean context; this resolves the
// deterministic identity bindings, composes the candidate, assembles the
// final fields/coverage/findings, and returns the digest-bound result.
func (s *Service) buildPreviewResult(ctx context.Context, root string, request Request, plan Plan) (PreviewResult, error) {
	bindings, err := s.resolvePreviewBindings(ctx, root, request)
	if err != nil {
		return PreviewResult{}, err
	}

	candidate, composeFindings, err := Compose(ctx, root, request, plan)
	if err != nil {
		return PreviewResult{}, err
	}
	fields, coverage, findings := assemblePreview(request, plan, composeFindings)

	result := PreviewResult{
		Schema:        PreviewResultSchema,
		BaseCommit:    bindings.baseCommit,
		ModelDigest:   bindings.modelDigest,
		ConfigDigest:  bindings.configDigest,
		EngineDigest:  bindings.engineDigest,
		RequestDigest: bindings.requestDigest,
		SpecRef:       specRef(request.Target.Slug),
		Candidate:     candidate,
		Fields:        fields,
		Sources:       plan.Sources,
		Coverage:      coverage,
		Findings:      findings,
		Ready:         !hasBlocking(findings),
	}
	digest, err := computePreviewDigest(result)
	if err != nil {
		return PreviewResult{}, err
	}
	result.Digest = digest
	return result, nil
}

// nonBlockingFindings returns every non-blocking entry of findings, in
// order — Result.Disclosures never includes a blocking finding (Apply
// already refused before publication if one existed).
func nonBlockingFindings(findings []Finding) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if !f.Blocking {
			out = append(out, f)
		}
	}
	return out
}

// canonicalCheckout returns root's clean, absolute, slash-canonical form —
// the shape draftmutation.Identity.Validate requires — without resolving
// symlinks (draftmutation's own Validate does not require that either;
// only its heavier ResolveCanonicalIdentity does, which this package does
// not use).
func canonicalCheckout(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Clean(abs)), nil
}

// buildActorIdentity resolves the draftmutation.Identity Apply's actor-
// policy check reports errors against: the checkout's actual current
// branch (or "DETACHED", mirroring draftmutation.ResolveCanonicalIdentity's
// own convention) and head — never the target design/<slug> branch, which
// does not exist yet at authorization time.
func buildActorIdentity(ctx context.Context, root, head, slug string) (draftmutation.Identity, error) {
	checkout, err := canonicalCheckout(root)
	if err != nil {
		return draftmutation.Identity{}, fmt.Errorf("%w: resolving checkout path: %v", ErrIOFailure, err)
	}
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		return draftmutation.Identity{}, fmt.Errorf("%w: resolving current branch: %v", ErrIdentityUnavailable, err)
	}
	if branch == "" {
		branch = "DETACHED"
	}
	return draftmutation.Identity{Checkout: checkout, Branch: branch, Head: head, Spec: specRef(slug)}, nil
}

// translateDraftmutationError maps the shared actor-policy dispatcher's
// typed refusal onto this package's own closed error codes, without
// inventing a second authorization decision.
func translateDraftmutationError(typed *draftmutation.Error) error {
	switch typed.Code {
	case draftmutation.CodePolicyForbidden:
		return fmt.Errorf("%w: %s", ErrPolicyForbidden, typed.Detail)
	case draftmutation.CodeActorForbidden:
		return fmt.Errorf("%w: %s", ErrActorForbidden, typed.Detail)
	default:
		return fmt.Errorf("%w: %s", ErrAuthorityInvalid, typed.Error())
	}
}
