// TestWallCoverageChips_SharedFunction (spec/index-coverage ac-1--
// behavioral) proves the wall's coverage chips come from the shared
// internal/featurecoverage function, texts unchanged: served over the
// scoping-fixture wall, every coverage chip renders byte-identically to
// the golden captured before ACCoverage's stub half was extracted into
// featurecoverage.Compute.
//
// TestBuildProjection_ACCoverageMatchesFeatureCoverage is the
// differential half: over BOTH the plain scoping fixture and the
// duplicate-entry fixture (whose per-stub-vs-per-entry dedup distinction
// is exactly the case a re-implementation could get subtly wrong),
// buildProjection's ACCoverage counts equal featurecoverage.Compute's own
// stub-half counts for the identical frontmatter — not merely
// coincidentally equal totals on one fixture.
package workbench

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/featurecoverage"
	"github.com/jyang234/verdi/internal/fixturegit"
)

var coverageChipRe = regexp.MustCompile(`<span class="coverage-chip[^"]*" data-testid="coverage-[^"]*" data-coverage="[0-9]+">[^<]*</span>`)

func TestWallCoverageChips_SharedFunction(t *testing.T) {
	mainFiles := map[string]string{
		".verdi/specs/active/scoping-fixture/spec.md": scopingProjectionFixtureSpec,
		".verdi/.gitignore":                           "data/\n",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: mainFiles, Message: "seed coverage-chip fixture"}})
	setDefaultBranchSymref(t, repo.Dir)

	h := newBoardTestHandler(repo.Dir)
	rec := getBoard(t, h, "scoping-fixture")
	if rec.Code != 200 {
		t.Fatalf("GET board = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	got := coverageChipRe.FindAllString(body, -1)
	if len(got) == 0 {
		t.Fatalf("no coverage chips found in served body:\n%s", body)
	}
	var gotJoined string
	for _, c := range got {
		gotJoined += c + "\n"
	}

	wantBytes, err := os.ReadFile(filepath.Join("testdata", "wall-coverage-chips.golden"))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if gotJoined != string(wantBytes) {
		t.Fatalf("served coverage chips changed from the pre-extraction golden.\n--- got ---\n%s--- want ---\n%s", gotJoined, string(wantBytes))
	}
}

// featureCoverageStubDecls builds featurecoverage.StubDecl inputs from a
// decoded feature's own stubs, exactly as buildProjection's extraction
// does — the shared conversion both TestBuildProjection_
// ACCoverageMatchesFeatureCoverage and projection.go itself perform.
func featureCoverageStubDecls(fm *artifact.SpecFrontmatter) []featurecoverage.StubDecl {
	var stubs []featurecoverage.StubDecl
	for _, st := range fm.Stubs {
		stubs = append(stubs, featurecoverage.StubDecl{Slug: st.Slug, AcceptanceCriteria: st.AcceptanceCriteria})
	}
	return stubs
}

// TestBuildProjection_ACCoverageMatchesFeatureCoverage is the differential
// half of ac-1--behavioral: buildProjection's ACCoverage must equal
// featurecoverage.Compute's own stub-half count for the SAME frontmatter,
// on both the plain fixture and the duplicate-entry fixture — the case
// that distinguishes "delegates to the shared function" from a
// re-implementation that merely produces the same TOTAL on the simple
// case.
func TestBuildProjection_ACCoverageMatchesFeatureCoverage(t *testing.T) {
	tests := []struct {
		name string
		fm   *artifact.SpecFrontmatter
	}{
		{"plain fixture", mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)},
		{"duplicate-entry fixture", dupEntryStubFrontmatter()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := buildProjectionFM(tt.name, tt.fm, nil, nil, nil, nil, modeReadOnly)
			if err != nil {
				t.Fatalf("buildProjectionFM: %v", err)
			}
			ids := make([]string, len(tt.fm.AcceptanceCriteria))
			for i, ac := range tt.fm.AcceptanceCriteria {
				ids[i] = ac.ID
			}
			cov := featurecoverage.Compute(ids, featureCoverageStubDecls(tt.fm), nil)
			for _, id := range ids {
				want := len(cov[id].Stubs)
				if got := p.ACCoverage[id]; got != want {
					t.Errorf("ACCoverage[%s] = %d, want featurecoverage.Compute's %d", id, got, want)
				}
			}
		})
	}
}
