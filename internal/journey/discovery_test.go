package journey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// separateReconciler and separateFolder wrap the production adapters but
// expose only the public port methods, so a projector built over them
// runs each adapter's own discovery: the pre-P1 behaviour the shared
// discovery must reproduce byte for byte.
type separateReconciler struct{ StubReconciler }

type separateFolder struct{ FeatureFolder }

func separateDiscoveryProjector() Projector {
	p := NewProjector()
	p.stubs = separateReconciler{NewStubReconciler()}
	p.folder = separateFolder{NewFeatureFolder()}
	return p
}

// TestGatherEventualFeatureFacts_SharedDiscoveryMatchesSeparate (ledger
// SI-352, lane P1 (c)): one discovery handed to both production adapters
// gathers exactly what each adapter's own discovery gathered — the stub
// reconciliation, the outcome-floor fold, and every unavailable-source
// sentence, at every stage a discovery can fail.
func TestGatherEventualFeatureFacts_SharedDiscoveryMatchesSeparate(t *testing.T) {
	for _, tc := range []struct {
		name string
		root func(*testing.T) string
		spec *artifact.SpecFrontmatter
	}{
		{name: "a feature with closed, candidate and proposed implementers", root: func(t *testing.T) string { return buildEventualFixtureRepo(t).Dir }, spec: eventualFixtureFeature(t)},
		{name: "an unparseable spec id", root: func(t *testing.T) string { return t.TempDir() }, spec: unparseableRefSpec()},
		{name: "a root whose index cannot be built", root: func(t *testing.T) string { return t.TempDir() }, spec: checkoutSpec()},
		{name: "an implementer that cannot be resolved", root: ghostImplementerStoreRoot, spec: checkoutSpec()},
		{name: "evidence records that cannot be loaded", root: unreadableDerivedRoot, spec: checkoutSpec()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.root(t)
			repo := RepositoryFacts{Head: StringFact{Known: true, Value: headOrEmpty(t, root)}}
			shared := NewProjector()
			gotStubs, gotFold, gotUnavailable := shared.gatherEventualFeatureFacts(context.Background(), root, "checkout", tc.spec, model.Canonical(), repo)
			wantStubs, wantFold, wantUnavailable := separateDiscoveryProjector().gatherEventualFeatureFacts(context.Background(), root, "checkout", tc.spec, model.Canonical(), repo)
			if !reflect.DeepEqual(gotStubs, wantStubs) || !reflect.DeepEqual(gotFold, wantFold) || !reflect.DeepEqual(gotUnavailable, wantUnavailable) {
				t.Fatalf("shared discovery gathered\n  stubs %+v\n  fold %+v\n  unavailable %q\nwant\n  stubs %+v\n  fold %+v\n  unavailable %q", gotStubs, gotFold, gotUnavailable, wantStubs, wantFold, wantUnavailable)
			}
		})
	}
}

// landingWalks counts the first-parent landing walks (`git rev-list
// --first-parent --reverse <ref> -- <path>`) a context launches, per path.
type landingWalks struct {
	mu     sync.Mutex
	byPath map[string]int
}

func (w *landingWalks) Observe(_ string, args []string) {
	if len(args) < 5 || args[0] != "rev-list" || args[1] != "--first-parent" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.byPath[args[len(args)-1]]++
}

// TestGatherFacts_ResolvesEachImplementerOnce (ledger SI-352, lane P1 (c)):
// gathering a feature's facts resolves each implementing story's
// lifecycle — its first-parent landing walk — once, where the stub
// reconciliation and the outcome-floor fold each used to resolve it.
func TestGatherFacts_ResolvesEachImplementerOnce(t *testing.T) {
	repo := buildEventualFixtureRepo(t)
	cfg := openConfig(t, repo.Dir)
	walks := &landingWalks{byPath: map[string]int{}}
	if _, err := NewProjector().GatherFacts(gitx.WithObserver(context.Background(), walks), cfg, "spec/checkout"); err != nil {
		t.Fatalf("GatherFacts: %v", err)
	}
	story := ".verdi/specs/active/checkout-story-two/spec.md"
	if walks.byPath[story] == 0 {
		t.Fatalf("landing walks per path = %v, want one for the landed implementer %s", walks.byPath, story)
	}
	for path, n := range walks.byPath {
		if n != 1 {
			t.Errorf("%s was walked %d times in one gather, want once", path, n)
		}
	}
}

// eventualFixtureFeature decodes the eventual fixture's feature spec.
func eventualFixtureFeature(t *testing.T) *artifact.SpecFrontmatter {
	t.Helper()
	fm, _, err := artifact.SplitFrontmatter([]byte(eventualFixtureFeatureSpecMD))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

// headOrEmpty is root's HEAD, or "" when root is not a repository.
func headOrEmpty(t *testing.T, root string) string {
	t.Helper()
	head, err := gitx.RevParse(context.Background(), root, "HEAD")
	if err != nil {
		return ""
	}
	return head
}

// unreadableDerivedRoot is loneFeatureStoreRoot with a regular file where
// the feature's derived-evidence directory belongs, so discovery succeeds
// and loading the feature's own evidence records fails.
func unreadableDerivedRoot(t *testing.T) string {
	t.Helper()
	root := loneFeatureStoreRoot(t)
	derived := store.DerivedSpecDir(root, store.RefSlug("spec/checkout"))
	if err := os.MkdirAll(filepath.Dir(derived), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(derived, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestImplementersFailure pins, per discovery stage and per source, the
// exact sentence each production adapter returned when it ran its own
// discovery: these become blockers.eventual.unavailable entries, part of
// the record's bytes. A complete discovery has no failure.
func TestImplementersFailure(t *testing.T) {
	cause := errors.New("boom")
	for _, tc := range []struct {
		stage  discoveryStage
		source string
		want   string
	}{
		{discoveryParsingSpecID, sourceStubs, `journey: parsing spec id "spec/x" for stub reconciliation: boom`},
		{discoveryParsingSpecID, sourceFold, `journey: parsing spec id "spec/x" for feature fold: boom`},
		{discoveryBuildingIndex, sourceStubs, "journey: building index for stub reconciliation: boom"},
		{discoveryBuildingIndex, sourceFold, "journey: building index for feature fold: boom"},
		{discoveryFindingStories, sourceStubs, "journey: discovering implementing stories for stub reconciliation: boom"},
		{discoveryFindingStories, sourceFold, "journey: discovering implementing stories for feature fold: boom"},
	} {
		got := implementers{stage: tc.stage, err: cause}.failure(tc.source, "spec/x")
		if got == nil || got.Error() != tc.want || !errors.Is(got, cause) {
			t.Errorf("failure(%v, %q) = %v, want %q wrapping its cause", tc.stage, tc.source, got, tc.want)
		}
	}
	if err := (implementers{}).failure(sourceStubs, "spec/x"); err != nil {
		t.Errorf("a complete discovery's failure = %v, want nil", err)
	}
}
