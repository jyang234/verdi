package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/constitutionapp"
	"github.com/jyang234/verdi/internal/ritualwitness"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// The shared pieces of spec/ritual-effect-witness's two producer tests
// (TestUAT036_RitualsNeverCarryForeignEntries, ac-3, and
// TestBranchCuts_FromResolvedDefaultBranch, ac-4): every ritual runs
// through its real entry point on an internal/ritualwitness fixture whose
// seed commit carries the store the ritual needs, and is judged against
// exactly the declaration internal/writescope holds for it (story dc-1).

// ritualDeclaration returns the registry's declaration of ritual.
func ritualDeclaration(t *testing.T, ritual string) ws.Declaration {
	t.Helper()
	for _, d := range ws.Registry() {
		if d.Ritual == ritual {
			return d
		}
	}
	t.Fatalf("the write-scope registry declares no ritual %q", ritual)
	return ws.Declaration{}
}

// createdCommits returns every commit the run created, sorted.
func createdCommits(res ritualwitness.Result) []string {
	var out []string
	for id := range res.After.Commits {
		if _, old := res.Before.Commits[id]; !old {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// requireIndexCarryWithin fails the test unless the run's one index-carry
// verdict (Evaluate always reports exactly one) holds the declared carry.
func requireIndexCarryWithin(t *testing.T, res ritualwitness.Result) {
	t.Helper()
	var carry []ritualwitness.Verdict
	for _, v := range res.Verdicts {
		if v.Field == "index_carry" {
			carry = append(carry, v)
		}
	}
	if len(carry) != 1 || carry[0].Status != ritualwitness.Within {
		t.Errorf("index carry verdicts = %v, want exactly one within the declaration", carry)
	}
}

// logVerdicts records every verdict in the test's output, so a reader of
// the producer's `go test -json` stream sees the whole judgment.
func logVerdicts(t *testing.T, res ritualwitness.Result) {
	t.Helper()
	for _, v := range res.Verdicts {
		t.Logf("verdict %s", v)
	}
}

// requireCleanRun fails the test unless the ritual ran to completion.
func requireCleanRun(t *testing.T, res ritualwitness.Result) {
	t.Helper()
	if res.Exit != 0 || res.Err != nil {
		t.Fatalf("the ritual exited %d: %v", res.Exit, res.Err)
	}
}

// stagedIn reports whether path is staged (index differs from HEAD) in
// snap.
func stagedIn(snap ritualwitness.Snapshot, path string) bool {
	for _, st := range snap.Status {
		if st.Path == path && st.X != ' ' && st.X != '?' && st.X != '!' {
			return true
		}
	}
	return false
}

// minimalManifestYAML is a store manifest with no toolchain block and no
// provider, so no built-binary ritual reaches an upstream runner or a
// tracker (census §4's hermeticity hazards).
const minimalManifestYAML = supersedeManifestYAML

// proposedDiagramFile is buildAcceptDiagramRepo's proposal diagram, as a
// base file for a ritual fixture.
func proposedDiagramFile() string {
	return "---\n" + proposedDiagramFM + "---\n" + diagramBodyIdiosyncratic
}

// buildStartStoreFiles is a store carrying one accepted story, landed on
// the default branch with its obligation elaborated, so build start's
// preconditions pass (TestRunBuildStart_StatuslessExactDefaultBranch_Starts's
// fixture, without the toolchain block).
func buildStartStoreFiles() map[string]string {
	return map[string]string{
		".verdi/verdi.yaml":                               minimalManifestYAML,
		".verdi/specs/active/widget-story/spec.md":        statuslessBuildStorySpecMD,
		".verdi/obligations/widget-story/ac-1--static.md": buildQualityObligationDocument("widget-story", "ac-1", artifact.EvidenceStatic, buildQualityBlock()),
	}
}

// constitutionProposeRequest writes, outside the fixture, a propose
// request creating branch with the store's overlay retitled, and returns
// its path.
func constitutionProposeRequest(t *testing.T, store map[string]string, branch string) string {
	t.Helper()
	overlay, ok := store[".verdi/policy/overlays/frontend-go-version.md"]
	if !ok {
		t.Fatal("the constitution store carries no frontend-go-version overlay")
	}
	retitled := strings.Replace(overlay, "Frontend Go version overlay", "Frontend Go version overlay (ritual witness)", 1)
	data, err := json.Marshal(map[string]any{
		"schema":   constitutionapp.ProposeRequestSchema,
		"branch":   branch,
		"kind":     "policy-overlay",
		"name":     "frontend-go-version",
		"content":  base64.StdEncoding.EncodeToString([]byte(retitled)),
		"expected": map[string]any{"branch": branch},
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "propose.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
