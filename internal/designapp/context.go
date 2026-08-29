package designapp

import (
	"context"
	"errors"
	"os"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
	"github.com/jyang234/verdi/internal/upstream"
)

// ErrAlignUnavailable is the sentinel AlignFindings implementations return
// when no toolchain is configured (verdi.yaml `toolchain:` block, I-4) —
// the same "no toolchain configured" condition
// cmd/verdi/baseline.go's regenerateBaseline already treats as a
// graceful, disclosed skip rather than a hard failure. get_design_context
// checks for exactly this sentinel to omit its Verdi-go findings section
// honestly instead of failing the whole bounded-context read.
var ErrAlignUnavailable = errors.New("designapp: no toolchain configured; Verdi-go-derived findings are unavailable")

// AlignFindings is the consumer-owned port over internal/align's existing
// Verdi-go-derived service and boundary findings composer (AC-5: "Verdi-go
// service and boundary findings relevant to the scope"). Compute regenerates
// graph/boundary-contract state by executing the pinned flowmap/groundwork
// CLIs (CO-7) — a genuinely slow, toolchain-dependent operation — so
// production and tests inject different adapters here rather than this
// package re-deriving toolchain wiring or findings classification itself.
type AlignFindings interface {
	Findings(ctx context.Context, root string, spec *artifact.SpecFrontmatter, covers string) (*align.ComputedResult, error)
}

// upstreamAlignFindings is the production adapter: it resolves the
// store's configured toolchain (mirroring cmd/verdi/baseline.go's and
// cmd/verdi/design.go's own upstream.RealRunner construction exactly) and
// delegates to align.Compute — the one findings algorithm, never
// reimplemented here.
type upstreamAlignFindings struct{}

func (upstreamAlignFindings) Findings(ctx context.Context, root string, spec *artifact.SpecFrontmatter, covers string) (*align.ComputedResult, error) {
	cfg, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	if cfg.Manifest == nil || cfg.Manifest.Toolchain == nil {
		return nil, ErrAlignUnavailable
	}
	runner := upstream.RealRunner{Module: cfg.Manifest.Toolchain.Module, Commit: cfg.Manifest.Toolchain.Commit, Dir: root}
	return align.Compute(ctx, align.ComputedInput{Root: root, Runner: runner, Spec: spec, Covers: covers})
}

// AlignFinding is a typed, JSON-tagged projection of artifact.Finding's
// identity-bearing fields (that type carries only `yaml` tags — an
// authoring-corpus convention, not a wire-response one; this is a
// re-tagging of existing public data, never a new field or new
// authority). Disposition/Note/CarriedFrom are alignment-report human
// workflow state, out of get_design_context's bounded read scope.
type AlignFinding struct {
	ID   string               `json:"id"`
	Kind artifact.FindingKind `json:"kind"`
	Text string               `json:"text"`
}

// VerdiGoFindings is get_design_context's Verdi-go-derived service and
// boundary findings section (AC-5). Available is false — Findings always
// nil, never an empty-but-present slice — exactly when no toolchain is
// configured for this checkout (Reason names it); this is a disclosed
// omission, never a silent one (CO-1).
type VerdiGoFindings struct {
	Available bool           `json:"available"`
	Reason    string         `json:"reason,omitempty"`
	Findings  []AlignFinding `json:"findings,omitempty"`
}

// ParentFeature is get_design_context's "explicitly selected parent
// feature" content item (AC-5), resolved from the current draft's OWN
// declared document-level `implements` link — never a corpus-wide search
// — mirroring the exact edge-scoping rule
// internal/workbench/familylinks.go's attachParentFeatureLink already
// uses (Type=="implements", From=="spec"). "Explicit" here means the
// draft itself names the relationship; Verdi never infers one from
// proximity or naming convention.
type ParentFeature struct {
	Ref     string                    `json:"ref"`
	Content *artifact.SpecFrontmatter `json:"content"`
}

// ChildStory is get_design_context's "explicitly selected ... child
// stories" content item (AC-5). Unlike ParentFeature, the current draft
// carries no back-index of its own child stories, so the CALLER names
// which already-known story refs to resolve
// (GetDesignContextRequest.ChildStories) — the caller does the
// "explicit[]" selecting, precisely as AC-5's phrase distinguishes this
// from an automatic, unbounded corpus-wide enumeration of every story
// that happens to implement this feature.
type ChildStory struct {
	Ref     string                    `json:"ref"`
	Content *artifact.SpecFrontmatter `json:"content"`
}

// PinnedContextItem is one resolved entry from the spec's own declared
// `context:` field (AC-5's "spec's declared pinned context references"),
// resolved through the exact same internal/index seam get_context_bundle
// already uses — never a second pinned-ref resolution algorithm.
type PinnedContextItem struct {
	Ref   string `json:"ref"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// GetDesignContextRequest names the one spec to compile bounded context
// for, plus the child story refs the caller explicitly wants resolved
// (ParentFeature has no such field — see ChildStory's doc comment for why
// the two directions are asymmetric).
type GetDesignContextRequest struct {
	Spec         string
	ChildStories []string
}

func (r GetDesignContextRequest) validate() error {
	ref, err := artifact.ParseRef(r.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return errors.New("designapp: get_design_context spec must be an unpinned whole spec ref")
	}
	for _, child := range r.ChildStories {
		childRef, err := artifact.ParseRef(child)
		if err != nil || childRef.Kind != artifact.KindSpec || childRef.Pinned() || childRef.Fragment() {
			// vocab:identity — operational diagnostic naming the ChildStories request field, identity
			return errors.New("designapp: get_design_context child story ref " + child + " must be an unpinned whole spec ref")
		}
	}
	return nil
}

// DesignContextResult is get_design_context's exact bounded content
// (AC-5): the current draft; an explicitly selected parent feature or
// child stories; applicable ratified decisions (the current draft's own
// decisions, plus the resolved parent feature's own already-accepted
// decisions — both already visible in typed frontmatter, never a second
// decision ledger); the spec's declared pinned context references;
// Verdi-go-derived findings; and the context/policy digests. Provenance
// is deliberately absent — get_design_provenance is its own tool, on
// explicit request only (AC-4/AC-5).
type DesignContextResult struct {
	Identity          draftmutation.Identity    `json:"identity"`
	CurrentDraft      *artifact.SpecFrontmatter `json:"current_draft"`
	ParentFeature     *ParentFeature            `json:"parent_feature,omitempty"`
	ChildStories      []ChildStory              `json:"child_stories,omitempty"`
	RatifiedDecisions []artifact.Decision       `json:"ratified_decisions"`
	PinnedContext     []PinnedContextItem       `json:"pinned_context"`
	VerdiGoFindings   VerdiGoFindings           `json:"verdi_go_findings"`
	PolicyDigest      string                    `json:"policy_digest"`
	ContextDigest     string                    `json:"context_digest"`
}

// GetDesignContext returns only material relevant to design assistance
// (AC-5). It composes storyresolve.LoadActiveSpec (the current draft and
// any resolved parent/child spec), the shared index/pinned-ref seam
// get_context_bundle already uses, draftmutation's own effective-policy
// resolution, and AlignFindings — never re-deriving any of their
// algorithms.
func (s Service) GetDesignContext(ctx context.Context, start string, req GetDesignContextRequest) (*DesignContextResult, *Error) {
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	identity, typed := s.resolveIdentity(ctx, start, req.Spec)
	if typed != nil {
		return nil, typed
	}
	ref, err := artifact.ParseRef(identity.Spec)
	if err != nil {
		return nil, operational("authority-invalid", "parsing canonical spec identity", err)
	}

	current, err := storyresolve.LoadActiveSpec(identity.Checkout, ref.Name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, notFound("spec-not-found", "no such active spec: "+identity.Spec)
		}
		// vocab:identity — operational diagnostic naming the draft-mutation/design-branch concept, identity
		return nil, operational("io-failure", "loading current draft", err)
	}

	var parent *ParentFeature
	for _, link := range current.Links {
		if link.Type != artifact.LinkImplements {
			continue
		}
		// A story's own implements link ordinarily names one acceptance
		// criterion on the parent feature (cmd/verdi/design.go's own
		// scaffold: "spec/todo-replace-feature-name#ac-1"), so the target
		// ref carries a fragment; Name still names the whole parent spec to
		// load. A document-level implements link with no fragment at all is
		// also accepted (a story that implements a whole feature, not one
		// AC) — either shape resolves the same parent.
		parentRef, parseErr := artifact.ParseRef(link.Ref)
		if parseErr != nil || parentRef.Kind != artifact.KindSpec {
			continue
		}
		content, loadErr := storyresolve.LoadActiveSpec(identity.Checkout, parentRef.Name)
		if loadErr != nil {
			continue
		}
		parent = &ParentFeature{Ref: link.Ref, Content: content}
		break
	}

	children := make([]ChildStory, 0, len(req.ChildStories))
	for _, childSpec := range req.ChildStories {
		childRef, parseErr := artifact.ParseRef(childSpec)
		if parseErr != nil {
			// vocab:identity — operational diagnostic naming the ChildStories request field, identity
			return nil, inputInvalid("input-invalid", "invalid child story ref: "+childSpec)
		}
		content, loadErr := storyresolve.LoadActiveSpec(identity.Checkout, childRef.Name)
		if loadErr != nil {
			return nil, notFound("child-story-not-found", "no such active spec: "+childSpec)
		}
		children = append(children, ChildStory{Ref: childSpec, Content: content})
	}

	decisions := append([]artifact.Decision{}, current.Decisions...)
	if parent != nil {
		decisions = append(decisions, parent.Content.Decisions...)
	}

	pinned, pinErr := s.resolvePinnedContext(ctx, identity.Checkout, current.Context)
	if pinErr != nil {
		return nil, pinErr
	}

	findings := s.resolveVerdiGoFindings(ctx, identity.Checkout, current, identity.Head)

	if s.Policy == nil {
		return nil, operational("policy-source-unavailable", "policy source is not configured", nil)
	}
	grant, policyErr := draftmutation.ResolvePolicyGrant(ctx, identity.Checkout, identity, s.Policy)
	if policyErr != nil {
		return nil, translateDraftmutationError(policyErr)
	}

	result := &DesignContextResult{
		Identity:          identity,
		CurrentDraft:      current,
		ParentFeature:     parent,
		ChildStories:      children,
		RatifiedDecisions: decisions,
		PinnedContext:     pinned,
		VerdiGoFindings:   findings,
		PolicyDigest:      grant.Digest,
	}
	digest, digestErr := contextDigest(result)
	if digestErr != nil {
		return nil, operational("result-invalid", "digesting design context", digestErr)
	}
	result.ContextDigest = digest
	return result, nil
}

func (s Service) resolvePinnedContext(ctx context.Context, root string, refs []string) ([]PinnedContextItem, *Error) {
	items := make([]PinnedContextItem, 0, len(refs))
	if len(refs) == 0 {
		return items, nil
	}
	ix, err := index.Build(root)
	if err != nil {
		return nil, operational("io-failure", "building corpus index", err)
	}
	for _, r := range refs {
		pinned, err := artifact.ParsePinnedRef(r)
		if err != nil {
			return nil, operational("authority-invalid", "invalid pinned context reference "+r, err)
		}
		entry, err := ix.GetPinned(ctx, pinned)
		if err != nil {
			return nil, operational("io-failure", "resolving pinned context reference "+r, err)
		}
		items = append(items, PinnedContextItem{Ref: entry.Ref, Kind: entry.Kind, Title: entry.Title, Body: entry.Body})
	}
	return items, nil
}

func (s Service) resolveVerdiGoFindings(ctx context.Context, root string, spec *artifact.SpecFrontmatter, covers string) VerdiGoFindings {
	if s.Align == nil {
		return VerdiGoFindings{Available: false, Reason: "align findings port is not configured"}
	}
	result, err := s.Align.Findings(ctx, root, spec, covers)
	if err != nil {
		if errors.Is(err, ErrAlignUnavailable) {
			return VerdiGoFindings{Available: false, Reason: "no toolchain configured (verdi.yaml toolchain: block)"}
		}
		return VerdiGoFindings{Available: false, Reason: "computing Verdi-go findings: " + err.Error()}
	}
	findings := make([]AlignFinding, 0, len(result.Findings))
	for _, finding := range result.Findings {
		findings = append(findings, AlignFinding{ID: finding.ID, Kind: finding.Kind, Text: finding.Text})
	}
	return VerdiGoFindings{Available: true, Findings: findings}
}

// contextDigest returns a deterministic canonical-JSON digest over
// result's own CONTENT, excluding the digest field itself and Identity —
// the same self-referential digest shape
// internal/designprovenance.Entry.Seal already uses for its own committed
// records. Identity.Checkout is excluded deliberately: it is a purely
// local, per-worktree filesystem path with no bearing on content, so two
// checkouts of byte-identical repository state (Branch/HEAD/Spec, both
// still folded into content via CurrentDraft/PolicyDigest/etc.) get the
// SAME context digest — the property a caller comparing notes across
// worktrees or adapters (this file's own CLI/MCP conformance pairing)
// actually needs. AC-8's "Branch and worktree identity" is already
// reported honestly via the separate, uncompressed Identity field.
func contextDigest(result *DesignContextResult) (string, error) {
	projection := *result
	projection.ContextDigest = ""
	projection.Identity = draftmutation.Identity{}
	return canonjson.Digest(projection)
}
