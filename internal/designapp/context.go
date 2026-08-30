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
	"github.com/jyang234/verdi/internal/specstate"
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
	Ref     string          `json:"ref"`
	Content *SpecContent    `json:"content"`
	State   specstate.State `json:"state"`
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
	Ref     string       `json:"ref"`
	Content *SpecContent `json:"content"`
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

// ApplicablePolicy is AC-5's "applicable project policies" content item:
// the resolved design_assistance policy's own CONTENT, not merely its
// digest. AC-5 lists "applicable project policies and ratified decisions"
// as a content item DISTINCT from its later "the context and policy
// digests" item, so a digest alone would satisfy the second bullet while
// silently dropping the first — an agent cannot honor a posture it can
// only fingerprint. Mode and Layout are the two fields
// policyartifact.DesignAssistancePayload actually carries; PolicyID names
// the policy that carried them, so a caller can trace the posture back to
// its governing artifact ("name the governing policy", CO-1).
type ApplicablePolicy struct {
	PolicyID string `json:"policy_id"`
	Mode     string `json:"mode"`
	Layout   bool   `json:"layout"`
}

// The closed vocabulary of reasons RatifiedDecisions holds what it holds.
const (
	// RatifiedFromAcceptedParent: the draft's explicitly linked parent
	// feature is accepted on the default branch, so its decisions are
	// ratified authority and are included.
	RatifiedFromAcceptedParent = "accepted-parent-feature"
	// RatifiedNoParentFeature: the draft declares no parent feature, so
	// there is no ratified decision source at all.
	RatifiedNoParentFeature = "no-parent-feature"
	// RatifiedParentNotAccepted: a parent feature resolved, but its
	// Git-derived state is not accepted — its decisions are proposals,
	// not ratified authority, so none are included.
	RatifiedParentNotAccepted = "parent-feature-not-accepted"
	// RatifiedParentDeclaredMissing: the draft DECLARES a parent feature
	// whose target is genuinely absent from the active zone. Deliberately
	// distinct from RatifiedNoParentFeature: "this draft names no parent"
	// and "this draft names a parent that is not there" are different
	// facts, and collapsing the second into the first would report an
	// inconsistent draft as a consistent one with nothing to resolve
	// (AC-5/CO-1: a fact Verdi could not read is never the favorable
	// answer). Source carries the exact declared ref; the read itself
	// stays clean, because an honest read of an inconsistent draft is not
	// an operational fault.
	RatifiedParentDeclaredMissing = "parent-declared-missing"
)

// RatifiedDecisionsPosture discloses WHY RatifiedDecisions holds what it
// holds, so an empty list is never an ambiguous silence (CO-1). Source
// names the parent ref consulted, when there was one, and ParentState is
// that parent's Git-derived state — the fact the inclusion decision turns
// on.
//
// It is also where the read's PARENT posture is reported, because the
// parent edge is what ratification turns on: Reason separates "this draft
// declares no parent" (RatifiedNoParentFeature) from "this draft declares
// a parent that is not in the active zone" (RatifiedParentDeclaredMissing,
// with Source naming the exact declared ref). ParentFeature above is nil
// in both cases — there is no content to report either way — so the
// posture is the only place the difference between them survives.
type RatifiedDecisionsPosture struct {
	Reason      string          `json:"reason"`
	Source      string          `json:"source,omitempty"`
	ParentState specstate.State `json:"parent_state,omitempty"`
}

// DesignContextResult is get_design_context's exact bounded content
// (AC-5): the current draft; an explicitly selected parent feature or
// child stories; the applicable project policy's content AND its ratified
// decisions; the spec's declared pinned context references;
// Verdi-go-derived findings; and the context/policy digests. Provenance
// is deliberately absent — get_design_provenance is its own tool, on
// explicit request only (AC-4/AC-5).
//
// RatifiedDecisions carries ONLY decisions that are ratified authority:
// the resolved parent feature's own decisions, and only while that
// parent's Git-derived state is accepted. The current draft's own
// decisions are deliberately excluded — they are the very proposals under
// design, already fully visible inside CurrentDraft, and echoing them into
// a list named "ratified" would let an agent read its own unaccepted
// proposal back as project authority (AC-5's normal-build-context
// paragraph: "applicable RATIFIED project decisions and policies"). A
// proposed parent's decisions are excluded for the same reason.
type DesignContextResult struct {
	Schema                   string                   `json:"schema"`
	Identity                 draftmutation.Identity   `json:"identity"`
	CurrentDraft             *SpecContent             `json:"current_draft"`
	ParentFeature            *ParentFeature           `json:"parent_feature,omitempty"`
	ChildStories             []ChildStory             `json:"child_stories,omitempty"`
	ApplicablePolicy         ApplicablePolicy         `json:"applicable_policy"`
	RatifiedDecisions        []Decision               `json:"ratified_decisions"`
	RatifiedDecisionsPosture RatifiedDecisionsPosture `json:"ratified_decisions_posture"`
	PinnedContext            []PinnedContextItem      `json:"pinned_context"`
	VerdiGoFindings          VerdiGoFindings          `json:"verdi_go_findings"`
	PolicyDigest             string                   `json:"policy_digest"`
	ContextDigest            string                   `json:"context_digest"`
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

	parent, declaredMissing, parentErr := s.resolveParentFeature(ctx, identity.Checkout, current.Links)
	if parentErr != nil {
		return nil, parentErr
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
		children = append(children, ChildStory{Ref: childSpec, Content: projectSpec(content)})
	}

	decisions, posture := ratifiedDecisions(parent, declaredMissing)

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
		Schema:                   ContextResultSchema,
		Identity:                 identity,
		CurrentDraft:             projectSpec(current),
		ParentFeature:            parent,
		ChildStories:             children,
		ApplicablePolicy:         ApplicablePolicy{PolicyID: grant.PolicyID, Mode: grant.Mode, Layout: grant.Layout},
		RatifiedDecisions:        decisions,
		RatifiedDecisionsPosture: posture,
		PinnedContext:            pinned,
		VerdiGoFindings:          findings,
		PolicyDigest:             grant.Digest,
	}
	digest, digestErr := contextDigest(result)
	if digestErr != nil {
		return nil, operational("result-invalid", "digesting design context", digestErr)
	}
	result.ContextDigest = digest
	return result, nil
}

// resolveParentFeature resolves the draft's OWN declared document-level
// parent-feature edge (see ParentFeature's doc comment for the scoping
// rule). It returns exactly one of three outcomes, and the caller must be
// able to tell them apart:
//
//   - a resolved parent (declaredMissing empty);
//   - no parent resolved because the declared edge's target is genuinely
//     ABSENT from the active zone — declaredMissing names that exact
//     declared ref, and the read stays clean, because an honest read of an
//     inconsistent draft is not an operational fault;
//   - an operational failure, when the target IS there but could not be
//     turned into a parent: undecodable frontmatter, an unreadable path,
//     or a state projection that could not answer. Reporting any of those
//     as "no parent" would report an unanswered question as a proven
//     absence, which is exactly what CO-1 forbids.
//
// The FIRST spec-kind implements edge is the declared parent, whatever its
// fate: scanning past an absent one to a later resolvable edge would
// silently drop the very fact this function exists to disclose. A
// non-spec-kind implements target (an ADR, a service boundary) is not a
// parent-feature edge at all and is skipped — that is an unrelated,
// genuinely inapplicable link, not a missing fact.
func (s Service) resolveParentFeature(ctx context.Context, root string, links []artifact.Link) (*ParentFeature, string, *Error) {
	for _, link := range links {
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
		content, loadErr := storyresolve.LoadActiveSpec(root, parentRef.Name)
		if loadErr != nil {
			if errors.Is(loadErr, os.ErrNotExist) {
				return nil, link.Ref, nil
			}
			// vocab:identity — operational diagnostic naming AC-5's own declared parent-feature edge, identity
			return nil, "", operational("authority-invalid", "loading declared parent feature "+link.Ref, loadErr)
		}
		state, stateErr := s.resolveSpecState(ctx, root, parentRef.Name)
		if stateErr != nil {
			return nil, "", stateErr
		}
		return &ParentFeature{Ref: link.Ref, Content: projectSpec(content), State: state}, "", nil
	}
	return nil, "", nil
}

// ratifiedDecisions applies AC-5's ratification rule: a decision is
// ratified authority only when it lives on an ACCEPTED parent feature.
// specstate.AcceptedPendingBuild is the accepted state (the exact
// active-zone revision is reachable from the default branch and is neither
// closed nor superseded) — a proposed, superseded, closed, or unproven
// parent all yield an empty list plus the posture naming why, never a
// silent empty (CO-1).
//
// declaredMissing is resolveParentFeature's second outcome: the exact ref
// a draft declared as its parent and that is not in the active zone. It
// carries its own posture reason precisely so it is never reported as the
// draft declaring no parent at all.
func ratifiedDecisions(parent *ParentFeature, declaredMissing string) ([]Decision, RatifiedDecisionsPosture) {
	empty := []Decision{}
	if parent == nil {
		if declaredMissing != "" {
			return empty, RatifiedDecisionsPosture{Reason: RatifiedParentDeclaredMissing, Source: declaredMissing}
		}
		return empty, RatifiedDecisionsPosture{Reason: RatifiedNoParentFeature}
	}
	posture := RatifiedDecisionsPosture{Source: parent.Ref, ParentState: parent.State}
	if parent.State != specstate.AcceptedPendingBuild {
		posture.Reason = RatifiedParentNotAccepted
		return empty, posture
	}
	posture.Reason = RatifiedFromAcceptedParent
	return append(empty, parent.Content.Decisions...), posture
}

// resolveSpecState projects one named active spec's Git-derived lifecycle
// state through the SAME draftmutation.StateProjector port every other
// operation in this package uses (capabilities.go, and the port
// draftmutation.AuthorizeState itself consumes) — never a second state
// derivation. A projection failure is operational, never a silent
// "not accepted".
func (s Service) resolveSpecState(ctx context.Context, root, name string) (specstate.State, *Error) {
	if s.State == nil {
		return "", operational("state-projector-unavailable", "state projector is not configured", nil)
	}
	content, err := os.ReadFile(store.SpecPath(root, store.ZoneActive, name))
	if err != nil {
		return "", operational("io-failure", "reading spec for state projection", err)
	}
	candidate := specstate.Candidate{Path: store.SpecRelPath(store.ZoneActive, name), Content: content}
	result, err := s.State.ResolveState(ctx, root, candidate)
	if err != nil {
		return "", operational("authority-invalid", "projecting Git-derived spec state", err)
	}
	return result.State, nil
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
// result's own CONTENT, excluding the digest field itself and the WHOLE
// Identity — the same self-referential digest shape
// internal/designprovenance.Entry.Seal already uses for its own committed
// records.
//
// Excluding all four identity fields is deliberate and each is excluded
// for its own reason, none of them "it is covered elsewhere":
//
//   - Checkout is a purely local, per-worktree filesystem path with no
//     bearing on content.
//   - Branch and HEAD are NOT folded into the digested content anywhere.
//     They are genuinely outside what this digest covers: the digest
//     answers "did the agent and I read the same design context?", and
//     the same bounded context can legitimately be read from two
//     different branch names or two different HEAD commits (an unrelated
//     commit elsewhere in the checkout moves HEAD without changing one
//     byte of this response's content). Including them would make the
//     digest change when the context did not, defeating the comparison it
//     exists for.
//   - Spec is redundant with CurrentDraft.ID, which IS digested.
//
// The consequence a caller must know: two context digests being equal
// proves the CONTENT matched, not that it was read on the same branch or
// commit. AC-8's "Branch and worktree identity" is therefore reported
// honestly and uncompressed via the separate Identity field, which a
// caller comparing digests must compare alongside it — and which the
// mutation contract, not this digest, is what actually pins a write to an
// exact HEAD (draftmutation.ExpectedIdentity).
func contextDigest(result *DesignContextResult) (string, error) {
	projection := *result
	projection.ContextDigest = ""
	projection.Identity = draftmutation.Identity{}
	return canonjson.Digest(projection)
}
