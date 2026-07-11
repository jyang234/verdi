package lint

import (
	"testing"
)

const grandfatherBadSpec = `---
id: spec/grandfather-bad
kind: spec
class: feature
title: "grandfather: bad decode"
status: draft
owners: [platform-team]
story: jira:LOAN-0099
bogus_field: "would fail VL-001"
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [] }
---
# grandfather: bad
`

// TestGrandfatherArchive_OQ3 proves Options.GrandfatherArchive (OQ-3: "skip
// VL-001..006 under specs/archive/ on import") is implemented but off by
// default (dormant): the same badly-shaped file under specs/archive/ fires
// VL-001 (and would fire VL-006) with the option off, and fires nothing
// with it on.
func TestGrandfatherArchive_OQ3(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/specs/archive/grandfather-bad/spec.md", grandfatherBadSpec)
	repo := buildLintRepo(t, dir)

	t.Run("off by default: fires", func(t *testing.T) {
		findings := runLint(t, repo.Dir, Context{}, Options{})
		found := false
		for _, f := range findings {
			if f.Rule == "VL-001" && f.Path == ".verdi/specs/archive/grandfather-bad/spec.md" {
				found = true
			}
		}
		if !found {
			t.Fatalf("VL-001 did not fire on the archived bad spec with GrandfatherArchive off:\n%s", findingsString(findings))
		}
	})

	t.Run("on: dormant, does not fire", func(t *testing.T) {
		findings := runLint(t, repo.Dir, Context{}, Options{GrandfatherArchive: true})
		for _, f := range findings {
			if f.Path == ".verdi/specs/archive/grandfather-bad/spec.md" {
				t.Fatalf("finding fired on a grandfathered archive file: %s", f.String())
			}
		}
	})
}
