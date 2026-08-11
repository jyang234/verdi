package journey

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// GitReader is the read-only Git-plumbing surface fact-gathering depends on
// (the 04 §port pattern, mirroring internal/refindex/port.go's identical
// GitRunner shape): a consumer-defined interface, not internal/gitx's free
// functions called directly, so fact-gathering is unit-testable against an
// in-process fake with no real git process at all (see the *_test.go fakes
// beside this file).
//
// Every method reads a ref, a working-tree state, or a remote
// configuration. The method set contains NOTHING capable of moving HEAD,
// writing the index, or touching the working tree — no Checkout, no
// CheckoutNewBranch, no generic Run(args ...string) escape hatch — so a
// checkout- or index-mutating call is impossible to express through this
// interface, not merely undocumented or unused: the journey projection is
// read-only by construction (GLG DC-1: "Journey is a projection, not a
// workflow engine"; CO-2: "No button state ... becomes a lifecycle fact"),
// and this method set is where that guarantee is enforced statically, the
// same static-guarantee posture refindex.GitRunner's own doc comment
// documents for its sibling interface.
//
// RemoteDesignBranches, suggested by the original work order for this
// port, is deliberately NOT part of this method set: ActiveBranch
// resolution (facts.go) only ever needs to test EXISTENCE of two specific,
// already-known candidate branch names (design/<name> and feature/<name>)
// against one target spec, never to enumerate every design/* branch in the
// repository — HasRemoteTrackingBranch is the precise primitive for that,
// and using it avoids an O(all design branches) listing for an O(1)
// existence question.
type GitReader interface {
	// RevParse resolves rev to its object id (gitx.RevParse).
	RevParse(ctx context.Context, dir, rev string) (string, error)
	// CurrentBranch returns dir's checked-out branch short name, or ("",
	// nil) for a detached HEAD (gitx.CurrentBranch's own documented
	// contract — not an error).
	CurrentBranch(ctx context.Context, dir string) (string, error)
	// RemoteURL returns the URL configured for remote name (gitx.RemoteURL).
	// A genuinely-absent remote is gitx.ErrNoSuchRemote (errors.Is-able).
	RemoteURL(ctx context.Context, dir, name string) (string, error)
	// StatusDirty reports whether dir's working tree has any uncommitted
	// change (gitx.StatusDirty).
	StatusDirty(ctx context.Context, dir string) (bool, error)
	// StagedPaths returns the repository-relative paths whose index
	// entries differ from HEAD (gitx.StagedPaths).
	StagedPaths(ctx context.Context, dir string) ([]string, error)
	// Show reads path's content as it existed at ref (gitx.Show).
	Show(ctx context.Context, dir, ref, path string) ([]byte, error)
	// IsAncestor reports whether ancestor is ref itself or a real ancestor
	// of ref (gitx.IsAncestor).
	IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error)
	// HasLocalBranch reports whether dir has a local branch named name
	// (gitx.HasLocalBranch).
	HasLocalBranch(ctx context.Context, dir, name string) (bool, error)
	// HasRemoteTrackingBranch reports whether dir has a local
	// remote-tracking ref for remote/branch (gitx.HasRemoteTrackingBranch)
	// — no network call.
	HasRemoteTrackingBranch(ctx context.Context, dir, remote, branch string) (bool, error)
}

// gitxReader adapts internal/gitx's free functions to GitReader — the
// production implementation, mirroring internal/refindex/port.go's
// gitxRunner.
type gitxReader struct{}

// NewGitReader returns the production GitReader: a thin adapter over
// internal/gitx, execed against the process's system git.
func NewGitReader() GitReader { return gitxReader{} }

func (gitxReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	return gitx.RevParse(ctx, dir, rev)
}

func (gitxReader) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return gitx.CurrentBranch(ctx, dir)
}

func (gitxReader) RemoteURL(ctx context.Context, dir, name string) (string, error) {
	return gitx.RemoteURL(ctx, dir, name)
}

func (gitxReader) StatusDirty(ctx context.Context, dir string) (bool, error) {
	return gitx.StatusDirty(ctx, dir)
}

func (gitxReader) StagedPaths(ctx context.Context, dir string) ([]string, error) {
	return gitx.StagedPaths(ctx, dir)
}

func (gitxReader) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return gitx.Show(ctx, dir, ref, path)
}

func (gitxReader) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return gitx.IsAncestor(ctx, dir, ancestor, ref)
}

func (gitxReader) HasLocalBranch(ctx context.Context, dir, name string) (bool, error) {
	return gitx.HasLocalBranch(ctx, dir, name)
}

func (gitxReader) HasRemoteTrackingBranch(ctx context.Context, dir, remote, branch string) (bool, error) {
	return gitx.HasRemoteTrackingBranch(ctx, dir, remote, branch)
}

// StateResolver is the consumer-defined port (04 §port pattern) over
// internal/specstate's Git-derived lifecycle projection — the SAME
// resolver every other lifecycle decision in this module routes through
// (specstate's own package doc: "no adapter reimplements reachability").
// Mirrors internal/refindex/port.go's StateResolver, narrowed to the
// single-candidate Resolve entry point: unlike refindex (which resolves a
// whole corpus per call and so needs the batched ResolveMany), fact-
// gathering here always evaluates exactly one target per call.
type StateResolver interface {
	Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
}

// specstateResolver adapts specstate.Projector to StateResolver — the
// production implementation.
type specstateResolver struct{ p specstate.Projector }

// NewStateResolver returns the production StateResolver: a thin adapter
// over specstate.NewProjector(), the ONE place lifecycle state is derived.
func NewStateResolver() StateResolver { return specstateResolver{p: specstate.NewProjector()} }

func (r specstateResolver) Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	return r.p.Resolve(ctx, root, candidate)
}

// ProfileSelection identifies the constitution-selected governance profile
// and its kernel-sealed digest. It contains no profile rules: journey records
// the installed authority without interpreting it.
type ProfileSelection struct {
	ID     string
	Digest string
}

// ProfileLoader is journey's consumer-owned port for reading the installed
// policy authority's selected governance profile.
type ProfileLoader interface {
	Load(ctx context.Context, root string) (ProfileSelection, error)
}

type policyAuthorityProfileLoader struct{}

// NewProfileLoader returns the production adapter backed exclusively by
// policyauthority.Load.
func NewProfileLoader() ProfileLoader { return policyAuthorityProfileLoader{} }

func (policyAuthorityProfileLoader) Load(_ context.Context, root string) (ProfileSelection, error) {
	policyStore, err := policyauthority.Load(root)
	if err != nil {
		return ProfileSelection{}, fmt.Errorf("journey: loading policy authority: %w", err)
	}

	selectedID := policyStore.Constitution.SelectedProfile
	selected, ok := policyStore.Profiles[selectedID]
	if !ok || selected == nil {
		return ProfileSelection{}, fmt.Errorf("journey: loading policy authority: selected profile %q is unavailable after validation", selectedID)
	}
	return ProfileSelection{ID: selectedID, Digest: selected.ProfileDigest}, nil
}

// DefaultBranchResolver is the func-value shape of
// specstate.ResolveDefaultBranch: a production Projector wires this field
// directly to that function; tests substitute a fake func, exactly like
// git/state above — the "production func value, fakeable" seam the work
// order calls for, without a third named interface for a single free
// function.
type DefaultBranchResolver func(ctx context.Context, root string) (specstate.Branch, bool)

// ObligationQualityFact is journey's consumer-owned projection of the shared
// evidence assessment for one declared (AC, kind) pair.
type ObligationQualityFact struct {
	ACID       string
	Kind       artifact.EvidenceKind
	Assessment evidence.ObligationAssessment
}

// ObligationQualityReader is journey's read-only port to the shared
// obligation-quality loader and exact record matcher.
type ObligationQualityReader interface {
	Assess(ctx context.Context, root, targetPath, targetClass, targetCommit, evaluationCommit, specLandingCommit string) ([]ObligationQualityFact, error)
}

type obligationQualityGitReader interface {
	evidence.ObligationAncestryReader
	Show(ctx context.Context, dir, ref, path string) ([]byte, error)
}

type evidenceObligationQualityReader struct {
	git obligationQualityGitReader
}

// NewObligationQualityReader returns the production adapter. It performs no
// mutation and delegates all structural and record semantics to evidence.
func NewObligationQualityReader() ObligationQualityReader {
	return evidenceObligationQualityReader{git: gitxReader{}}
}

func (r evidenceObligationQualityReader) Assess(ctx context.Context, root, targetPath, targetClass, targetCommit, evaluationCommit, specLandingCommit string) ([]ObligationQualityFact, error) {
	// Only stories own obligation artifacts. Feature and initiative targets can
	// be projected from a remote ref without a corresponding working-tree file,
	// so reject them at the already-resolved target-class boundary before I/O.
	if targetClass != string(artifact.ClassStory) {
		return []ObligationQualityFact{}, nil
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(targetPath)))
	if errors.Is(err, os.ErrNotExist) && targetCommit != "" && r.git != nil {
		raw, err = r.git.Show(ctx, root, targetCommit, targetPath)
	}
	if err != nil {
		return nil, fmt.Errorf("journey: reading obligation-quality target %s: %w", targetPath, err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("journey: decoding obligation-quality target %s: %w", targetPath, err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return nil, fmt.Errorf("journey: decoding obligation-quality target %s: %w", targetPath, err)
	}
	if spec.Class != artifact.ClassStory {
		return nil, fmt.Errorf("journey: obligation-quality target %s resolved as %q but decoded as %q", targetPath, targetClass, spec.Class)
	}

	var records []artifact.Evidence
	if evaluationCommit != "" {
		derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
		records, err = evidence.LoadRecords(ctx, root, derivedRoot, evaluationCommit)
		if err != nil {
			return nil, fmt.Errorf("journey: loading obligation-quality evidence for %s: %w", spec.ID, err)
		}
	}

	facts := make([]ObligationQualityFact, 0)
	for _, ac := range spec.AcceptanceCriteria {
		current := evidence.Current(evidence.RecordsForAC(records, ac.ID))
		for _, kind := range ac.Evidence {
			assessment, err := evidence.AssessObligation(ctx, evidence.ObligationAssessmentInput{
				StoreRoot: root,
				SpecName:  specNameFromRelPath(targetPath),
				ACID:      ac.ID,
				Kind:      kind,
			})
			if err != nil {
				return nil, err
			}

			fact := ObligationQualityFact{ACID: ac.ID, Kind: kind, Assessment: assessment}
			fact.Assessment, err = selectObligationQualityAssessment(ctx, assessment, evidence.ObligationAssessmentInput{
				StoreRoot:         root,
				SpecName:          specNameFromRelPath(targetPath),
				ACID:              ac.ID,
				Kind:              kind,
				EvaluationCommit:  evaluationCommit,
				SpecLandingCommit: specLandingCommit,
				Git:               r.git,
			}, current)
			if err != nil {
				return nil, err
			}
			facts = append(facts, fact)
		}
	}
	return facts, nil
}

// selectObligationQualityAssessment applies the fold's total evidence
// precedence to every current record for one declared pair. A witnessed
// failure outranks both a matched pass and any unproven candidate regardless
// of record order; a matched pass outranks unproven candidates.
func selectObligationQualityAssessment(ctx context.Context, base evidence.ObligationAssessment, in evidence.ObligationAssessmentInput, current []artifact.Evidence) (evidence.ObligationAssessment, error) {
	selected := base
	for i := range current {
		if current[i].Kind != in.Kind {
			continue
		}
		candidateInput := in
		candidateInput.Record = &current[i]
		candidate, err := evidence.MatchObligation(ctx, base, candidateInput)
		if err != nil {
			return evidence.ObligationAssessment{}, err
		}
		if candidate.MatchState == evidence.ObligationViolatedWithWitness {
			return candidate, nil
		}
		if candidate.MatchState == evidence.ObligationMatched {
			selected = candidate
			continue
		}
		if selected.MatchState != evidence.ObligationMatched && selected.Reason == evidence.ObligationReasonProducerMissing {
			selected = candidate
		}
	}
	return selected, nil
}

// Projector gathers repository and lifecycle facts and (a later stage,
// Project) derives the complete journey Record from them. Its zero value
// is not useful — construct it via NewProjector (production) or the
// package-private newProjector (tests, over fakes), mirroring
// internal/specstate.Projector's own construction discipline.
type Projector struct {
	git                  GitReader
	state                StateResolver
	profiles             ProfileLoader
	obligations          ObligationQualityReader
	resolveDefaultBranch DefaultBranchResolver
}

// NewProjector returns a Projector backed by real git plumbing, the real
// internal/specstate resolver, the policyauthority-backed profile loader,
// and specstate.ResolveDefaultBranch — the only constructor production
// callers may use.
func NewProjector() Projector {
	return Projector{
		git:                  NewGitReader(),
		state:                NewStateResolver(),
		profiles:             NewProfileLoader(),
		obligations:          NewObligationQualityReader(),
		resolveDefaultBranch: specstate.ResolveDefaultBranch,
	}
}

// newProjector is the test-only seam: package tests construct a Projector
// over in-process fakes.
func newProjector(git GitReader, state StateResolver, resolveDefaultBranch DefaultBranchResolver) Projector {
	return Projector{git: git, state: state, profiles: NewProfileLoader(), obligations: evidenceObligationQualityReader{git: git}, resolveDefaultBranch: resolveDefaultBranch}
}
