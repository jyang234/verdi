package lint

import (
	"path/filepath"
	"testing"
)

// TestVL016_SpikeBuildBranch_FencesWithoutTouchingSpikeDir is D-5's
// regression: it reproduces Phase B's real spike-build-branch shape, where
// the spike's own spec directory is committed and frozen during the design
// phase — BEFORE the build branch is cut — so the build branch's diff
// (DiffBase..HEAD) never touches it. The old touchesSpike heuristic keyed the
// fence off exactly that (now-absent) signal, so it could never fire on a
// real spike build. The fence must now activate on the current branch being
// feature/<spike-name>, fire on the out-of-fence touch, and stay quiet on the
// in-fence one.
func TestVL016_SpikeBuildBranch_FencesWithoutTouchingSpikeDir(t *testing.T) {
	overlay := t.TempDir()
	writeTestFile(t, filepath.Join(overlay, ".verdi/verdi.yaml"), vl016ManifestWithSpikePaths)
	spikeSpec := readTestdataFile(t, filepath.Join(violationsDir, "VL-016", "only-spike-dir", ".verdi", "specs", "active", "borrower-update-mobile-spike", "spec.md"))
	writeTestFile(t, filepath.Join(overlay, ".verdi/specs/active/borrower-update-mobile-spike/spec.md"), spikeSpec)

	repo := buildLintRepo(t, overlay)
	// The spike-spec commit is the build branch's diff base — the build
	// branch's own diff starts AFTER the spike directory was already frozen.
	branchPoint := repo.Heads[len(repo.Heads)-1]

	gitCheckoutNewBranch(t, repo.Dir, "feature/borrower-update-mobile-spike")
	// The build branch's real diff: one in-fence file (matches spike_paths:
	// spikes/**) and one out-of-fence production touch — NEVER the spike's
	// own spec directory.
	writeTestFile(t, filepath.Join(repo.Dir, "spikes/borrower-update-mobile-spike/findings.md"), "# spike findings\n")
	writeTestFile(t, filepath.Join(repo.Dir, "internal/production/leaked.go"), "package production\n")
	commitPaths(t, repo.Dir, "spike build: in-fence findings + out-of-fence touch",
		"spikes/borrower-update-mobile-spike/findings.md", "internal/production/leaked.go")

	lctx := Context{DiffBase: branchPoint, CurrentBranch: "feature/borrower-update-mobile-spike"}
	findings := runLint(t, repo.Dir, lctx, Options{})

	var vl016 []Finding
	for _, f := range findings {
		if f.Rule == "VL-016" {
			vl016 = append(vl016, f)
		}
	}
	if len(vl016) != 1 {
		t.Fatalf("VL-016 findings = %d, want exactly 1 (the out-of-fence touch), got:\n%s", len(vl016), findingsString(vl016))
	}
	if vl016[0].Path != "internal/production/leaked.go" {
		t.Fatalf("VL-016 finding path = %q, want internal/production/leaked.go (in-fence findings.md must stay quiet)", vl016[0].Path)
	}
}

// TestVL016_TouchesOutsideFence_Fails is the primary negative case: a
// spike build branch's diff (the spike's own spec directory plus a path
// outside both that directory and any spike_paths: allowlist entry) fails
// VL-016 on the disallowed path.
func TestVL016_TouchesOutsideFence_Fails(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-016", "touched-outside-fence"))
	diffBase := repo.Heads[len(repo.Heads)-2] // the commit before this overlay
	findings := runLint(t, repo.Dir, Context{DiffBase: diffBase}, Options{})
	onlyRule(t, findings, "VL-016")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
	if findings[0].Path != "internal/production/should-not-be-touched.go" {
		t.Fatalf("finding path = %q, want internal/production/should-not-be-touched.go", findings[0].Path)
	}
}

// TestVL016_OnlySpikeDirTouched_Clean is the positive complement: a diff
// touching only the spike's own spec directory never fires VL-016,
// regardless of spike_paths: (the spike's own directory is always
// implicitly allowed, vl016.go's withinAnyDir).
func TestVL016_OnlySpikeDirTouched_Clean(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-016", "only-spike-dir"))
	diffBase := repo.Heads[len(repo.Heads)-2]
	findings := runLint(t, repo.Dir, Context{DiffBase: diffBase}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-016" {
			t.Fatalf("VL-016 fired on a diff touching only the spike's own directory: %s", f.String())
		}
	}
}

// TestVL016_NoDiffBase_Silent proves VL-016's "can't prove it" posture
// (mirroring VL-010): with no DiffBase, the rule is silent rather than
// guessing, even over a store containing a spike.
func TestVL016_NoDiffBase_Silent(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-016", "touched-outside-fence"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-016" {
			t.Fatalf("VL-016 fired with no DiffBase set: %s", f.String())
		}
	}
}

// vl016ManifestWithSpikePaths overrides the shared setup layer's
// verdi.yaml with one that additionally configures spike_paths:, so this
// test can prove a path inside the allowlist (but outside the spike's own
// directory) is accepted.
const vl016ManifestWithSpikePaths = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
lint:
  gated_generated: []
derived:
  retention_days: 14
services:
  discovery: flowmap
spike_paths: [spikes/**]
`

// TestVL016_PathWithinConfiguredAllowlist_Clean proves a path outside the
// spike's own directory but matching a configured spike_paths: entry is
// accepted. The manifest override lands in its own, earlier layer (before
// the spike commit) so DiffBase excludes it — otherwise the manifest
// change itself would be "a path outside the fence" and defeat the test.
func TestVL016_PathWithinConfiguredAllowlist_Clean(t *testing.T) {
	manifestDir := t.TempDir()
	writeTestFile(t, filepath.Join(manifestDir, ".verdi/verdi.yaml"), vl016ManifestWithSpikePaths)

	spikeDir := t.TempDir()
	spikeSpec := readTestdataFile(t, filepath.Join(violationsDir, "VL-016", "only-spike-dir", ".verdi", "specs", "active", "borrower-update-mobile-spike", "spec.md"))
	writeTestFile(t, filepath.Join(spikeDir, ".verdi/specs/active/borrower-update-mobile-spike/spec.md"), spikeSpec)
	writeTestFile(t, filepath.Join(spikeDir, "spikes/borrower-update-mobile-spike/notes.md"), "# spike workspace notes\n")

	repo := buildLintRepo(t, manifestDir, spikeDir)
	diffBase := repo.Heads[len(repo.Heads)-2] // the commit right after the manifest override, right before the spike commit
	findings := runLint(t, repo.Dir, Context{DiffBase: diffBase}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-016" {
			t.Fatalf("VL-016 fired on a path matching the configured spike_paths: allowlist: %s", f.String())
		}
	}
}
