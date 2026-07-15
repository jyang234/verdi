package lint

import (
	"path/filepath"
	"testing"
)

func TestVL008_UngatedGeneratedProvenance(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-008"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-008")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL008_AllowlistedGeneratedProvenance_NoFinding is the happy path the
// overlay test above cannot exercise (its manifest has an empty
// lint.gated_generated): the identical ungated-provenance shape, but with
// the artifact's ref on the allowlist, must not fire.
func TestVL008_AllowlistedGeneratedProvenance_NoFinding(t *testing.T) {
	overlayDir := adHocOverlayDir(t, ".verdi/verdi.yaml", vl008AllowlistManifest)
	writeTestFile(t, filepath.Join(overlayDir, ".verdi", "specs", "active", "ungated", "spec.md"), vl008UngatedSpecBody)

	repo := buildLintRepo(t, overlayDir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-008" {
			t.Fatalf("VL-008 fired on an allowlisted ref: %s", f.String())
		}
	}
}

const vl008AllowlistManifest = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
lint:
  gated_generated: [spec/ungated]
derived:
  retention_days: 14
services:
  discovery: flowmap
`

const vl008UngatedSpecBody = `---
id: spec/ungated
kind: spec
class: component
title: "VL-008 overlay: allowlisted generated provenance"
status: active
owners: [platform-team]
provenance:
  generator: some-generator
  version: v0
  inputs: [spec/store-layout-notes@2f230011b192c5ac1c0ed5442be76fc401c4cbca]
  digest: sha256:1111111111111111111111111111111111111111111111111111111111111111
---
# VL-008 overlay: allowlisted generated provenance
`
