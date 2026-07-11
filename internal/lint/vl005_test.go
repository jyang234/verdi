package lint

import (
	"path/filepath"
	"testing"
)

func TestVL005_MultipleStoryLinks(t *testing.T) {
	repo := buildLintRepo(t, filepath.Join(violationsDir, "VL-005"))
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-005")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

// adHocOverlayDir writes a single file (relPath store-root-relative, e.g.
// ".verdi/specs/active/foo/spec.md") into a fresh temp directory laid out
// so overlayLayer can walk it like any testdata overlay — used for VL-005
// scenarios beyond the one testdata/violations overlay covers.
func adHocOverlayDir(t *testing.T, relPath, content string) string {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, filepath.FromSlash(relPath)), content)
	return dir
}

const vl005SchemeNotConfiguredSpec = `---
id: spec/vl-005-bad-scheme
kind: spec
class: feature
title: "VL-005: unconfigured scheme"
status: draft
owners: [platform-team]
story: confluence:PAGE-1
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-005: unconfigured scheme
`

// TestVL005_SchemeNotConfigured proves "a configured scheme" is checked,
// not just "well-formed scheme:key shape": verdi.yaml's providers: block
// (the lint harness's setup layer) configures only jira.
func TestVL005_SchemeNotConfigured(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-bad-scheme/spec.md", vl005SchemeNotConfiguredSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-005")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

const vl005LinksDisagreeSpec = `---
id: spec/vl-005-links-disagree
kind: spec
class: feature
title: "VL-005: links[] disagrees with scalar story"
status: draft
owners: [platform-team]
story: jira:LOAN-0010
links:
  - { type: story, ref: jira:LOAN-9999 }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-005: links disagree
`

// TestVL005_LinksMirrorDisagreesWithScalar proves I-24: the scalar story:
// field is canonical, and a links[] story mirror must agree with it
// exactly, even when both are individually well-formed and both name a
// configured scheme.
func TestVL005_LinksMirrorDisagreesWithScalar(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-links-disagree/spec.md", vl005LinksDisagreeSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-005")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

const vl005LinksAgreeSpec = `---
id: spec/vl-005-links-agree
kind: spec
class: feature
title: "VL-005: links[] agrees with scalar story"
status: draft
owners: [platform-team]
story: jira:LOAN-0011
links:
  - { type: story, ref: jira:LOAN-0011 }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-005: links agree
`

// TestVL005_LinksMirrorAgreesWithScalar is the happy path I-24 explicitly
// allows: an optional mirroring links[] entry that agrees exactly.
func TestVL005_LinksMirrorAgreesWithScalar(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-links-agree/spec.md", vl005LinksAgreeSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-005" {
			t.Fatalf("VL-005 fired on an agreeing links[] mirror: %s", f.String())
		}
	}
}
