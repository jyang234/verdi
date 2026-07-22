package commitdesign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// overrideFeatureTemplate is a store's own .verdi/templates/feature.md
// override — the exact file ledger L-M12's witness named as silently
// ignored by commit-to-design. It reshapes the scaffold (a team section,
// a custom: field) while still rendering a valid feature spec, and it
// references the content-carrying fields (Pins, Dispositions) the
// template contract gained for this path (spec/creation-form ac-4).
const overrideFeatureTemplate = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: feature
status: draft
story: {{safe .StoryRef}}
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
{{if .Pins}}context:
{{range .Pins}}  - {{.Ref}}
{{end}}{{end}}acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static] }
{{if .Dispositions}}dispositions:
{{range .Dispositions}}  - { sticky: {{.Sticky}}, disposition: {{.Disposition}} }
{{end}}{{end}}custom:
  rollout_plan: "canary then full rollout"
---
# {{.Title}}

## Rollout Plan

TODO: fill in the rollout plan.
`

// wrongClassFeatureOverride renders class: story under the feature
// class's own template filename — the misconfiguration CheckClass exists
// to catch (K1) on this path too.
const wrongClassFeatureOverride = `---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: [unassigned]
class: story
status: draft
story: {{safe .StoryRef}}
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO", evidence: [static] }
links:
  - { type: implements, ref: "spec/other#ac-1" }
---
# {{.Title}}
`

// TestRun_StoreOverrideHonored proves the L-M12 discharge end to end
// (spec/creation-form ac-4): with a .verdi/templates/feature.md override
// present, commit-to-design's committed spec carries the override's own
// shape — the custom: field, the reshaped body — while still
// self-validating and landing the board content (pins as context:,
// sticky dispositions) through the template's content-carrying fields.
func TestRun_StoreOverrideHonored(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	tmplPath := filepath.Join(repo.Dir, ".verdi", "templates", "feature.md")
	if err := os.MkdirAll(filepath.Dir(tmplPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmplPath, []byte(overrideFeatureTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	res, err := Run(ctx, Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "override-born", StoryRef: "jira:LOAN-1482", ModelDigest: testModelDigest(t, repo.Dir)})
	if err != nil {
		t.Fatalf("Run with feature.md override: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(repo.Dir, res.SpecRelPath))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, "## Rollout Plan") {
		t.Fatalf("committed spec does not carry the override's body section:\n%s", content)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Class != artifact.ClassFeature {
		t.Fatalf("Class = %q, want feature", spec.Class)
	}
	if got := spec.Custom["rollout_plan"]; got != "canary then full rollout" {
		t.Fatalf(`Custom["rollout_plan"] = %#v, want the override's value`, got)
	}
	// The board content still landed through the override's own slots.
	if !strings.Contains(content, "spec/other@") {
		t.Fatalf("override render dropped the board's pinned context:\n%s", content)
	}
	if !strings.Contains(content, "dispositions:") {
		t.Fatalf("override render dropped the dispositions block:\n%s", content)
	}
}

// TestRun_StoreOverrideWrongClassRefuses: an override rendering a
// non-feature class fails closed (CheckClass, stub-instantiate's
// inherited posture) before anything is committed.
func TestRun_StoreOverrideWrongClassRefuses(t *testing.T) {
	repo := buildRepo(t)
	seedBoard(t, repo)
	tmplPath := filepath.Join(repo.Dir, ".verdi", "templates", "feature.md")
	if err := os.MkdirAll(filepath.Dir(tmplPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmplPath, []byte(wrongClassFeatureOverride), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Run(context.Background(), Input{Root: repo.Dir, BoardKey: "STORY-1482", SpecName: "wrong-class", StoryRef: "jira:LOAN-1482", ModelDigest: testModelDigest(t, repo.Dir)})
	if err == nil {
		t.Fatal("Run with a wrong-class override succeeded, want a CheckClass refusal")
	}
	if !strings.Contains(err.Error(), "class") {
		t.Fatalf("error %q does not name the class disagreement", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "wrong-class")); !os.IsNotExist(statErr) {
		t.Fatalf("refused Run left the spec dir behind (statErr=%v)", statErr)
	}
}
