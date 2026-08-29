package designapp

import (
	"context"
	"errors"
	"os"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// PrepareDesignReviewRequest names the one spec whose semantic review
// packet to derive.
type PrepareDesignReviewRequest struct {
	Spec string
}

func (r PrepareDesignReviewRequest) validate() error {
	ref, err := artifact.ParseRef(r.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return errors.New("designapp: prepare_design_review spec must be an unpinned whole spec ref")
	}
	return nil
}

// ReviewBaseline names the review base prepare_design_review diffed
// against: the configured default branch's content at this spec's path,
// at the exact commit consulted (AC-6: "since the review base"). A new
// draft that the default branch has never held is Available:false with
// Reason — a disclosed omission of the diff section only; every other
// packet field remains fully populated (CO-1: silence is never a pass,
// but an honestly absent baseline is not a fault).
type ReviewBaseline struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Commit    string `json:"commit,omitempty"`
}

// ExcerptFlag names one semantic object that has EVER carried an
// ai-inferred or unresolved provenance excerpt (AC-6: "objects classified
// as ai-inferred or unresolved"). "Ever", not "most recently": a later
// human-stated excerpt on the same target adds a human's own account
// alongside the earlier one — the sidecar is append-only and no entry
// retracts a prior classification — so a target that was once inferred
// stays flagged for the reviewer. Classification is the LAST
// ai-inferred/unresolved value recorded for that target, in sidecar file
// order.
type ExcerptFlag struct {
	Target         string `json:"target"`
	Classification string `json:"classification"`
}

// UnclassifiedEdit names one direct-Markdown discontinuity in the
// provenance chain — either a historical gap an entry already recorded,
// or the open gap between the last recorded typed digest and the current
// spec bytes (AC-6: "direct edits whose origin is unclassified").
type UnclassifiedEdit struct {
	FromDigest string `json:"from_digest"`
	ToDigest   string `json:"to_digest"`
}

// ReviewResult is AC-6's exact semantic review packet: a view, never a
// persisted approval artifact or second source of truth. The agent may
// prepare or explain it but cannot mark it approved — this package has no
// method that could (doc.go: no accept/approve/merge operation exists
// anywhere in designapp).
type ReviewResult struct {
	Schema               string                  `json:"schema"`
	Identity             draftmutation.Identity  `json:"identity"`
	Problem              string                  `json:"problem"`
	Outcome              string                  `json:"outcome"`
	AcceptanceCriteria   []AcceptanceCriterion   `json:"acceptance_criteria"`
	Constraints          []Constraint            `json:"constraints"`
	Decisions            []Decision              `json:"decisions"`
	OpenQuestions        []OpenQuestion          `json:"open_questions"`
	Links                []artifact.Link         `json:"links"`
	Stubs                []Stub                  `json:"stubs"`
	Baseline             ReviewBaseline          `json:"baseline"`
	Changes              []draftmutation.Change  `json:"changes"`
	Warnings             []draftmutation.Warning `json:"warnings"`
	InferredOrUnresolved []ExcerptFlag           `json:"inferred_or_unresolved"`
	UnclassifiedEdits    []UnclassifiedEdit      `json:"unclassified_edits"`
	PolicyDigest         string                  `json:"policy_digest"`
	PolicyMode           string                  `json:"policy_mode"`
}

// PrepareDesignReview derives the semantic review packet without changing
// governance state (AC-8: "prepare_design_review ... derive[s] the
// semantic review packet without changing governance state"). It never
// persists anything and never dispositions, accepts, or approves — the
// human's PR-merge event remains the sole acceptance decision (AC-6).
func (s Service) PrepareDesignReview(ctx context.Context, start string, req PrepareDesignReviewRequest) (*ReviewResult, *Error) {
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

	specPath := store.SpecPath(identity.Checkout, store.ZoneActive, ref.Name)
	current, err := os.ReadFile(specPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, notFound("spec-not-found", "no such active spec: "+identity.Spec)
	}
	if err != nil {
		return nil, operational("io-failure", "reading current spec", err)
	}
	frontmatter, _, err := artifact.SplitFrontmatter(current)
	if err != nil {
		return nil, operational("authority-invalid", "splitting current spec frontmatter", err)
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return nil, operational("authority-invalid", "decoding current spec", err)
	}

	baseline, changes, warnings, baselineErr := s.resolveReviewBaseline(ctx, identity.Checkout, ref.Name, current)
	if baselineErr != nil {
		return nil, baselineErr
	}

	inferredOrUnresolved, unclassifiedEdits, provErr := s.resolveReviewProvenance(identity.Checkout, ref.Name, current)
	if provErr != nil {
		return nil, provErr
	}

	if s.Policy == nil {
		return nil, operational("policy-source-unavailable", "policy source is not configured", nil)
	}
	grant, policyErr := draftmutation.ResolvePolicyGrant(ctx, identity.Checkout, identity, s.Policy)
	if policyErr != nil {
		return nil, translateDraftmutationError(policyErr)
	}

	problem, outcome := "", ""
	if spec.Problem != nil {
		problem = spec.Problem.Text
	}
	if spec.Outcome != nil {
		outcome = spec.Outcome.Text
	}

	return &ReviewResult{
		Schema:               ReviewResultSchema,
		Identity:             identity,
		Problem:              problem,
		Outcome:              outcome,
		AcceptanceCriteria:   projectAcceptanceCriteria(spec.AcceptanceCriteria),
		Constraints:          projectConstraints(spec.Constraints),
		Decisions:            projectDecisions(spec.Decisions),
		OpenQuestions:        projectOpenQuestions(spec.OpenQuestions),
		Links:                projectLinks(spec.Links),
		Stubs:                projectStubs(spec.Stubs),
		Baseline:             baseline,
		Changes:              changes,
		Warnings:             warnings,
		InferredOrUnresolved: inferredOrUnresolved,
		UnclassifiedEdits:    unclassifiedEdits,
		PolicyDigest:         grant.Digest,
		PolicyMode:           grant.Mode,
	}, nil
}

// resolveReviewBaseline resolves "the review base" (AC-6): the configured
// default branch's own content at this spec's path, via the exact same
// specstate.ResolveDefaultBranch + internal/gitx primitives
// internal/specstate itself uses internally — never a second default-
// branch resolution. A path absent from the default branch (a spec that
// has never yet been accepted) is a disclosed, non-fatal "no baseline"
// rather than an error: draftmutation.Diff requires two decodable spec
// documents on both sides, and an empty byte string is not one.
//
// Absence is PROVEN, never inferred from a failure. `git show` collapses
// "path genuinely absent at a resolvable commit" and "git could not answer
// at all" into one indistinguishable non-zero exit, so a caller that read
// every Show error as absence would report a verdict-free "not yet
// accepted" on a broken repository — a silent pass exactly where CO-1
// forbids one. gitx.PathExistsAt draws that three-way instead (its own doc
// comment records the distinction): an absent path at a resolvable commit
// is empty output and exit 0, and only a real git failure is an error,
// which propagates operationally.
func (s Service) resolveReviewBaseline(ctx context.Context, root, name string, current []byte) (ReviewBaseline, []draftmutation.Change, []draftmutation.Warning, *Error) {
	empty := []draftmutation.Change{}
	emptyWarnings := []draftmutation.Warning{}

	branch, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		return ReviewBaseline{Available: false, Reason: "default branch is unresolved"}, empty, emptyWarnings, nil
	}
	relPath := store.SpecRelPath(store.ZoneActive, name)
	present, err := gitx.PathExistsAt(ctx, root, branch.Ref, relPath)
	if err != nil {
		return ReviewBaseline{}, nil, nil, operational("io-failure", "reading the default branch's review base", err)
	}
	if !present {
		return ReviewBaseline{Available: false, Reason: "spec is not yet present on the default branch", Branch: branch.Name}, empty, emptyWarnings, nil
	}
	baseBytes, err := gitx.Show(ctx, root, branch.Ref, relPath)
	if err != nil {
		return ReviewBaseline{}, nil, nil, operational("io-failure", "reading the default branch's review base", err)
	}
	commit, err := gitx.RevParse(ctx, root, branch.Ref)
	if err != nil {
		return ReviewBaseline{}, nil, nil, operational("io-failure", "resolving default branch commit", err)
	}
	changes, warnings, err := draftmutation.Diff(baseBytes, current)
	if err != nil {
		return ReviewBaseline{}, nil, nil, operational("authority-invalid", "computing semantic diff since review base", err)
	}
	if changes == nil {
		changes = empty
	}
	if warnings == nil {
		warnings = emptyWarnings
	}
	return ReviewBaseline{Available: true, Branch: branch.Name, Commit: commit}, changes, warnings, nil
}

// resolveReviewProvenance scans the committed provenance sidecar (never
// re-parsed or re-classified — internal/designprovenance owns its own
// codec) for AC-6's two provenance-derived disclosures: objects that ever
// carried an ai-inferred/unresolved excerpt (see ExcerptFlag for why
// "ever" and not "most recent"), and every unclassified direct-edit gap,
// including the open one between the sidecar's last recorded typed digest
// and the current spec bytes.
func (s Service) resolveReviewProvenance(root, name string, current []byte) ([]ExcerptFlag, []UnclassifiedEdit, *Error) {
	path := store.DesignProvenancePath(root, store.ZoneActive, name)
	raw, err := os.ReadFile(path)
	var entries []designprovenance.Entry
	switch {
	case errors.Is(err, os.ErrNotExist):
		entries = nil
	case err != nil:
		return nil, nil, operational("io-failure", "reading design provenance", err)
	default:
		entries, err = designprovenance.DecodeLog(raw)
		if err != nil {
			return nil, nil, operational("authority-invalid", "decoding design provenance", err)
		}
	}

	classifications := map[string]designprovenance.ExcerptClassification{}
	unclassifiedEdits := []UnclassifiedEdit{}
	for _, entry := range entries {
		if entry.UnclassifiedGap != nil {
			unclassifiedEdits = append(unclassifiedEdits, UnclassifiedEdit{
				FromDigest: entry.UnclassifiedGap.FromDigest, ToDigest: entry.UnclassifiedGap.ToDigest,
			})
		}
		for _, excerpt := range entry.Excerpts {
			switch excerpt.Classification {
			case designprovenance.ClassificationAIInferred, designprovenance.ClassificationUnresolved:
				classifications[excerpt.Target] = excerpt.Classification
			}
		}
	}
	if len(entries) > 0 {
		currentDigest := draftmutation.DigestBytes(current)
		last := entries[len(entries)-1]
		if last.ResultDigest != currentDigest {
			unclassifiedEdits = append(unclassifiedEdits, UnclassifiedEdit{FromDigest: last.ResultDigest, ToDigest: currentDigest})
		}
	}

	flags := make([]ExcerptFlag, 0, len(classifications))
	for target, classification := range classifications {
		flags = append(flags, ExcerptFlag{Target: target, Classification: string(classification)})
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].Target < flags[j].Target })
	return flags, unclassifiedEdits, nil
}
