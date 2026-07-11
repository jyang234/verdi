package lint

import (
	"path/filepath"
	"testing"
)

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
