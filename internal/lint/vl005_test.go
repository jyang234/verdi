package lint

import (
	"fmt"
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

// storyPreambleTmpl is the common problem/outcome/implements boilerplate
// every ad hoc VL-005 story fixture below needs to satisfy the story
// class's own decode-time requirements (02 §Kind registry) without
// tripping an unrelated VL-003 finding: the implements edge targets a real
// corpus AC fragment (spec/stale-decline#ac-1). %s is an insertion point
// inside the still-open links: block for an extra type:story mirror entry.
const storyPreambleTmpl = `links:
  - { type: implements, ref: "spec/stale-decline#ac-1" }
%sproblem: { text: "placeholder problem", anchor: "#problem" }
outcome: { text: "placeholder outcome", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static], anchor: "#ac-1" }
`

func storyPreamble(extraLink string) string {
	return fmt.Sprintf(storyPreambleTmpl, extraLink)
}

const storyBody = `
## Problem

Placeholder problem.

## Outcome

Placeholder outcome.

## AC-1

Placeholder.
`

var vl005SchemeNotConfiguredSpec = `---
id: spec/vl-005-bad-scheme
kind: spec
class: story
title: "VL-005: unconfigured scheme"
status: draft
owners: [platform-team]
story: confluence:PAGE-1
` + storyPreamble("") + `---
# VL-005: unconfigured scheme
` + storyBody

// TestVL005_SchemeNotConfigured proves "a configured scheme" is checked,
// not just "well-formed scheme:key shape": verdi.yaml's providers: block
// (the lint harness's setup layer) configures only jira. Story class
// (rescoped from feature, R4-I-2): the story: scalar is required and
// canonical here.
func TestVL005_SchemeNotConfigured(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-bad-scheme/spec.md", vl005SchemeNotConfiguredSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-005")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

var vl005LinksDisagreeSpec = `---
id: spec/vl-005-links-disagree
kind: spec
class: story
title: "VL-005: links[] disagrees with scalar story"
status: draft
owners: [platform-team]
story: jira:LOAN-0010
` + storyPreamble("  - { type: story, ref: jira:LOAN-9999 }\n") + `---
# VL-005: links disagree
` + storyBody

// TestVL005_LinksMirrorDisagreesWithScalar proves I-24: the scalar story:
// field is canonical, and a links[] story mirror must agree with it
// exactly, even when both are individually well-formed and both name a
// configured scheme. Story class (rescoped from feature, R4-I-2).
func TestVL005_LinksMirrorDisagreesWithScalar(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-links-disagree/spec.md", vl005LinksDisagreeSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-005")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

var vl005LinksAgreeSpec = `---
id: spec/vl-005-links-agree
kind: spec
class: story
title: "VL-005: links[] agrees with scalar story"
status: draft
owners: [platform-team]
story: jira:LOAN-0011
` + storyPreamble("  - { type: story, ref: jira:LOAN-0011 }\n") + `---
# VL-005: links agree
` + storyBody

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

// --- Feature class: the lighter, post-rescope epic-ref check ---

const vl005FeatureEpicRefBadSchemeSpec = `---
id: spec/vl-005-feature-bad-epic-scheme
kind: spec
class: feature
title: "VL-005: feature epic ref, unconfigured scheme"
status: draft
owners: [platform-team]
story: confluence:EPIC-1
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-005: feature epic ref, unconfigured scheme
`

// TestVL005_FeatureEpicRef_SchemeNotConfigured proves the feature class's
// post-rescope check (02 §Lint rules, amended): the optional story: epic
// ref, when present, is still validated against the configured schemes —
// even though the feature class no longer owns the "exactly one" /
// links[]-mirror-agreement machinery (moved to the story class, R4-I-2).
func TestVL005_FeatureEpicRef_SchemeNotConfigured(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-feature-bad-epic-scheme/spec.md", vl005FeatureEpicRefBadSchemeSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-005")
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
	}
}

const vl005FeatureNoEpicRefSpec = `---
id: spec/vl-005-feature-no-epic-ref
kind: spec
class: feature
title: "VL-005: feature with no story: epic ref at all"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-005: feature with no story: epic ref at all
`

// TestVL005_FeatureNoEpicRef_Clean proves the feature class's story: epic
// ref is genuinely optional (R4-I-2): its total absence never fires VL-005.
func TestVL005_FeatureNoEpicRef_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/active/vl-005-feature-no-epic-ref/spec.md", vl005FeatureNoEpicRefSpec)
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-005" {
			t.Fatalf("VL-005 fired on a feature spec with no story: epic ref at all: %s", f.String())
		}
	}
}
