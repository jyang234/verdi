package matrixprojection

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// Projection contains the one canonical matrix record plus the legacy
// text-only detail computed in the same projection pass.
type Projection struct {
	Record          Record
	Spec            *artifact.SpecFrontmatter
	EffectiveStatus artifact.Status
	Model           *model.Model
	Feature         *FeatureDetail
}

// FeatureDetail contains feature-only legacy text facts outside the public
// matrix wire contract.
type FeatureDetail struct {
	Reconciliation evidence.StubReconciliation
	Stories        []ImplementingStory
	SupersededByAC map[string][]string
}

// ImplementingStory is one non-superseded story's legacy feature-matrix
// rendering detail.
type ImplementingStory struct {
	SpecRef string
	ACIDs   []string
	Closed  bool
	Slug    string
}

// StateResolver is the Git-derived state capability needed to classify all
// implementing stories in one batch.
type StateResolver interface {
	ResolveMany(ctx context.Context, root string, candidates []specstate.Candidate) ([]specstate.Result, error)
}

// Project resolves and folds one story or real feature, then assembles the
// only matrix record consumed by CLI text, CLI JSON, and MCP.
func Project(ctx context.Context, root, ref string, preview bool, mdl *model.Model) (Projection, error) {
	gitx.ResetShallowCache()
	commit, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: resolving HEAD: %w", err)
	}
	spec, err := storyresolve.Resolve(root, ref)
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: resolving target: %w", err)
	}
	state, err := resolveEffectiveState(ctx, root, spec)
	if err != nil {
		return Projection{}, err
	}
	targetState := EffectiveState(state.State)
	if !validEffectiveStates[targetState] {
		return Projection{}, fmt.Errorf("matrix projection: effective state %q is unknown", state.State)
	}
	base := Projection{Spec: spec, EffectiveStatus: state.ArtifactStatus(), Model: mdl}

	if spec.Class == artifact.ClassFeature && spec.Problem != nil {
		return projectFeature(ctx, root, commit, preview, mdl, spec, targetState, base)
	}
	return projectStory(ctx, root, commit, preview, spec, state, targetState, base)
}

func projectStory(ctx context.Context, root, commit string, preview bool, spec *artifact.SpecFrontmatter, state specstate.Result, targetState EffectiveState, base Projection) (Projection, error) {
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: loading target evidence: %w", err)
	}
	parsed, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: parsing resolved spec ref %q: %w", spec.ID, err)
	}
	landing := ""
	if state.Baseline != nil {
		landing = state.Baseline.LandingCommit
	}
	result, err := evidence.Fold(evidence.Input{
		Context:           ctx,
		Spec:              spec,
		Records:           records,
		Preview:           preview,
		StoreRoot:         root,
		StorySlug:         store.RefSlug(spec.Story),
		EvaluationCommit:  commit,
		SpecLandingCommit: landing,
		Git:               ancestryReader{},
	})
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: folding target %s: %w", parsed.Name, err)
	}
	record, err := NewStory(Target{Class: ClassStory, SpecRef: spec.ID, EffectiveState: targetState}, preview, result)
	if err != nil {
		return Projection{}, err
	}
	base.Record = record
	return base, nil
}

func projectFeature(ctx context.Context, root, commit string, preview bool, mdl *model.Model, spec *artifact.SpecFrontmatter, targetState EffectiveState, base Projection) (Projection, error) {
	ref, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: parsing target ref %q: %w", spec.ID, err)
	}
	ix, err := index.Build(root)
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: building index: %w", err)
	}
	stories, storiesByAC, supersededByAC, err := DiscoverImplementingStories(ctx, root, commit, ref.Name, spec, ix, specstate.NewProjector())
	if err != nil {
		return Projection{}, err
	}
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: loading target evidence: %w", err)
	}
	result, err := evidence.FoldFeature(evidence.FeatureInput{
		Spec: spec, Stories: storiesByAC, Records: records, Preview: preview,
		StoreRoot: root, FeatureSlug: ref.Name, Model: mdl,
	})
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: folding target: %w", err)
	}
	stubStories := make([]evidence.StubStory, 0, len(stories))
	for _, story := range stories {
		stubStories = append(stubStories, evidence.StubStory{SpecRef: story.SpecRef, ACIDs: story.ACIDs, Closed: story.Closed})
	}
	reconciliation, err := evidence.ReconcileStubs(evidence.StubReconcileInput{Spec: spec, Stories: stubStories, Model: mdl})
	if err != nil {
		return Projection{}, fmt.Errorf("matrix projection: reconciling target stubs: %w", err)
	}
	record, err := NewFeature(Target{Class: ClassFeature, SpecRef: spec.ID, EffectiveState: targetState}, preview, result)
	if err != nil {
		return Projection{}, err
	}
	base.Record = record
	base.Feature = &FeatureDetail{Reconciliation: reconciliation, Stories: stories, SupersededByAC: supersededByAC}
	return base, nil
}

func resolveEffectiveState(ctx context.Context, root string, spec *artifact.SpecFrontmatter) (specstate.Result, error) {
	ref, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return specstate.Result{}, fmt.Errorf("matrix projection: parsing resolved spec ref %q: %w", spec.ID, err)
	}
	_, relPath, content, err := loadSpecBytesWithZone(root, ref.Name)
	if err != nil {
		return specstate.Result{}, err
	}
	if content == nil {
		return specstate.Result{}, fmt.Errorf("matrix projection: resolved spec %s not found in specs/active/ or specs/archive/", spec.ID)
	}
	result, err := specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: content})
	if err != nil {
		return specstate.Result{}, fmt.Errorf("matrix projection: resolving effective state: %w", err)
	}
	return result, nil
}

// DiscoverImplementingStories computes the one feature AC-to-story mapping
// shared by matrix projection and lifecycle consumers.
func DiscoverImplementingStories(ctx context.Context, root, commit, featureName string, spec *artifact.SpecFrontmatter, ix *index.Index, resolver StateResolver) ([]ImplementingStory, map[string][]evidence.ImplementingStory, map[string][]string, error) {
	if resolver == nil {
		return nil, nil, nil, fmt.Errorf("matrix projection: implementer state resolver is required")
	}
	order := make([]string, 0)
	acsByStory := make(map[string][]string)
	seen := make(map[string]map[string]bool)
	for _, ac := range spec.AcceptanceCriteria {
		key := fmt.Sprintf("spec/%s#%s", featureName, ac.ID)
		for _, backlink := range ix.Backlinks(key) {
			if backlink.Type != "implemented-by" {
				continue
			}
			if seen[backlink.From] == nil {
				seen[backlink.From] = make(map[string]bool)
				order = append(order, backlink.From)
			}
			if !seen[backlink.From][ac.ID] {
				seen[backlink.From][ac.ID] = true
				acsByStory[backlink.From] = append(acsByStory[backlink.From], ac.ID)
			}
		}
	}
	sort.Strings(order)

	type loadedStory struct {
		spec *artifact.SpecFrontmatter
	}
	loaded := make(map[string]loadedStory, len(order))
	candidates := make([]specstate.Candidate, 0, len(order))
	for _, storyRef := range order {
		parsed, err := artifact.ParseRef(storyRef)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("matrix projection: implementer ref %q: %w", storyRef, err)
		}
		storySpec, relPath, content, err := loadSpecBytesWithZone(root, parsed.Name)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("matrix projection: loading implementer %s: %w", storyRef, err)
		}
		if storySpec == nil {
			return nil, nil, nil, fmt.Errorf("matrix projection: implementer %s not found in specs/active/ or specs/archive/", storyRef)
		}
		loaded[storyRef] = loadedStory{spec: storySpec}
		candidates = append(candidates, specstate.Candidate{Path: relPath, Content: content})
	}
	states, err := resolver.ResolveMany(ctx, root, candidates)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("matrix projection: resolving implementer states for spec/%s: %w", featureName, err)
	}
	if len(states) != len(order) {
		return nil, nil, nil, fmt.Errorf("matrix projection: resolving implementer states returned %d results for %d candidates", len(states), len(order))
	}

	stories := make([]ImplementingStory, 0, len(order))
	byAC := make(map[string][]evidence.ImplementingStory)
	supersededByAC := make(map[string][]string)
	for i, storyRef := range order {
		state := states[i]
		acIDs := append([]string(nil), acsByStory[storyRef]...)
		sort.Strings(acIDs)
		if state.State == specstate.Unproven {
			return nil, nil, nil, fmt.Errorf("matrix projection: implementer %s effective state cannot be proven: %s", storyRef, strings.Join(state.Disclosures, "; "))
		}
		if state.State == specstate.Superseded {
			for _, acID := range acIDs {
				supersededByAC[acID] = append(supersededByAC[acID], storyRef)
			}
			continue
		}
		folded, err := foldImplementingStory(ctx, root, commit, loaded[storyRef].spec, state)
		if err != nil {
			return nil, nil, nil, err
		}
		closed := state.State == specstate.Closed
		stories = append(stories, ImplementingStory{SpecRef: storyRef, ACIDs: acIDs, Closed: closed, Slug: store.RefSlug(loaded[storyRef].spec.Title)})
		for _, acID := range acIDs {
			byAC[acID] = append(byAC[acID], evidence.ImplementingStory{
				SpecRef: storyRef, ACIDs: acIDs, Closed: closed,
				Eligible: folded.Eligible, Violated: folded.Violated,
			})
		}
	}
	return stories, byAC, supersededByAC, nil
}

func foldImplementingStory(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, state specstate.Result) (evidence.StoryResult, error) {
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		return evidence.StoryResult{}, fmt.Errorf("matrix projection: loading evidence for implementer %s: %w", spec.ID, err)
	}
	landing := ""
	if state.Baseline != nil {
		landing = state.Baseline.LandingCommit
	}
	result, err := evidence.Fold(evidence.Input{
		Context: ctx, Spec: spec, Records: records, StoreRoot: root,
		StorySlug: store.RefSlug(spec.Story), EvaluationCommit: commit,
		SpecLandingCommit: landing, Git: ancestryReader{},
	})
	if err != nil {
		return evidence.StoryResult{}, fmt.Errorf("matrix projection: folding implementer %s: %w", spec.ID, err)
	}
	return result, nil
}

func loadSpecBytesWithZone(root, name string) (*artifact.SpecFrontmatter, string, []byte, error) {
	for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
		path := store.SpecPath(root, zone, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", nil, fmt.Errorf("matrix projection: reading %s: %w", path, err)
		}
		frontmatter, _, err := artifact.SplitFrontmatter(data)
		if err != nil {
			return nil, "", nil, fmt.Errorf("matrix projection: %s: %w", path, err)
		}
		spec, err := artifact.DecodeSpec(frontmatter)
		if err != nil {
			return nil, "", nil, fmt.Errorf("matrix projection: %s: %w", path, err)
		}
		return spec, store.SpecRelPath(zone, name), data, nil
	}
	return nil, "", nil, nil
}

type ancestryReader struct{}

func (ancestryReader) IsAncestor(ctx context.Context, dir, ancestor, descendant string) (bool, error) {
	return gitx.IsAncestor(ctx, dir, ancestor, descendant)
}
