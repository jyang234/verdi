package journey

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// The two eventual sources a feature gathers — stub reconciliation and
// the outcome-floor fold — both start from the same implementing-story
// discovery: the feature's ref, the store's index, and one specstate
// resolution of every story that implements it. Run separately, that
// discovery resolved every implementing story twice per gather (lane P1,
// ledger SI-352). The production adapters therefore also accept one
// discovery that gatherEventualFeatureFacts runs once and hands to both.
// Each adapter's own Reconcile/Fold still runs the discovery itself, so a
// caller that uses only one of them, and every test double, is unchanged.

// sourceStubs and sourceFold name the two consumers in their errors,
// exactly as each adapter always worded them.
const (
	sourceStubs = "stub reconciliation"
	sourceFold  = "feature fold" // vocab:identity — machinery diagnostic naming the fold source (exit-2 machinery, not verdict prose)
)

// discoveryStage is how far one discovery got before it failed.
type discoveryStage int

const (
	discoveryComplete discoveryStage = iota
	discoveryParsingSpecID
	discoveryBuildingIndex
	discoveryFindingStories
)

// implementers is one feature's implementing-story discovery: its bare
// name, every non-superseded implementing story, and the stories per
// acceptance criterion — or the stage at which discovery failed and why.
type implementers struct {
	name    string
	stories []matrixprojection.ImplementingStory
	byAC    map[string][]evidence.ImplementingStory
	stage   discoveryStage
	err     error
}

// discoverImplementers runs the discovery both production adapters
// share, in the order each always ran it: parse the spec id, build the
// index, then discover and resolve the implementing stories.
func discoverImplementers(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter) implementers {
	ref, err := artifact.ParseRef(spec.ID)
	if err != nil {
		return implementers{stage: discoveryParsingSpecID, err: err}
	}
	ix, err := index.Build(root)
	if err != nil {
		return implementers{stage: discoveryBuildingIndex, err: err}
	}
	stories, byAC, _, err := matrixprojection.DiscoverImplementingStories(ctx, root, commit, ref.Name, spec, ix, specstate.NewProjector())
	if err != nil {
		return implementers{stage: discoveryFindingStories, err: err}
	}
	return implementers{name: ref.Name, stories: stories, byAC: byAC}
}

// failure is a failed discovery's error in source's own words — the bytes
// each adapter returned when it ran its own discovery — or nil.
func (d implementers) failure(source, specID string) error {
	switch d.stage {
	case discoveryParsingSpecID:
		return fmt.Errorf("journey: parsing spec id %q for %s: %w", specID, source, d.err)
	case discoveryBuildingIndex:
		return fmt.Errorf("journey: building index for %s: %w", source, d.err)
	case discoveryFindingStories:
		// vocab:identity — operational diagnostic naming ids (exit-2 machinery, not verdict prose)
		return fmt.Errorf("journey: discovering implementing stories for %s: %w", source, d.err)
	default:
		return nil
	}
}

// discoveredReconciler is a StubReconciler that can reconcile over a
// discovery already run for this gather.
type discoveredReconciler interface {
	reconcileDiscovered(spec *artifact.SpecFrontmatter, mdl *model.Model, found implementers) (evidence.StubReconciliation, error)
}

// discoveredFolder is a FeatureFolder that can fold over a discovery
// already run for this gather.
type discoveredFolder interface {
	foldDiscovered(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model, found implementers) (evidence.FeatureResult, error)
}

var (
	_ discoveredReconciler = stubReconciler{}
	_ discoveredFolder     = featureFolder{}
)

func (stubReconciler) reconcileDiscovered(spec *artifact.SpecFrontmatter, mdl *model.Model, found implementers) (evidence.StubReconciliation, error) {
	if err := found.failure(sourceStubs, spec.ID); err != nil {
		return evidence.StubReconciliation{}, err
	}
	stubStories := make([]evidence.StubStory, 0, len(found.stories))
	for _, story := range found.stories {
		stubStories = append(stubStories, evidence.StubStory{SpecRef: story.SpecRef, ACIDs: story.ACIDs, Closed: story.Closed})
	}
	return evidence.ReconcileStubs(evidence.StubReconcileInput{Spec: spec, Stories: stubStories, Model: mdl})
}

func (featureFolder) foldDiscovered(ctx context.Context, root, commit string, spec *artifact.SpecFrontmatter, mdl *model.Model, found implementers) (evidence.FeatureResult, error) {
	if err := found.failure(sourceFold, spec.ID); err != nil {
		return evidence.FeatureResult{}, err
	}
	derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(ctx, root, derivedRoot, commit)
	if err != nil {
		// vocab:identity — operational diagnostic naming ids (exit-2 machinery, not verdict prose)
		return evidence.FeatureResult{}, fmt.Errorf("journey: loading feature evidence records for the outcome floor: %w", err)
	}
	return evidence.FoldFeature(evidence.FeatureInput{
		Spec:    spec,
		Stories: found.byAC,
		Records: records,
		// R-RRF-2: closure folds source: ci only — the same authoritative-
		// only posture cmd/verdi/closefeature.go's foldFeature enforces.
		Preview:     false,
		StoreRoot:   root,
		FeatureSlug: found.name,
		Model:       mdl,
	})
}
