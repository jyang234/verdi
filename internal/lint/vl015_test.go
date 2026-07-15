package lint

import (
	"fmt"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// vl015LoanWorkflowV1DraftYAML and vl015LoanWorkflowV1FrozenTmpl mirror
// internal/artifact/v2fixture_test.go's own dedicated rung-4 supersession
// pair fixture content (loan-workflow / loan-workflow-v2): a small,
// independent fixturegit history built fresh per test (SHA_A computed
// dynamically, not the literal golden SHAs baked into examples/showcase's
// v2 fixtures — this package's own fixturegit corpus is a separate git
// history, so those literal SHAs would not be real history here; VL-015
// needs the predecessor's frozen commit to be real in *this* repo).
const vl015LoanWorkflowV1DraftYAML = `---
id: spec/loan-workflow
kind: spec
class: feature
title: "Loan workflow (VL-015 fixture)"
status: draft
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within one minute", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within one minute", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "workflow history is queryable by loan id", evidence: [static, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
---
# Loan workflow (VL-015 fixture)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within one minute of the change.

## AC-1

Workflow status changes are visible within one minute.

## AC-2

Workflow history is queryable by loan id.

## CO-1

Must not add new synchronous cross-service calls.
`

const vl015LoanWorkflowV1FrozenTmpl = `---
id: spec/loan-workflow
kind: spec
class: feature
title: "Loan workflow (VL-015 fixture)"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within one minute", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within one minute", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "workflow history is queryable by loan id", evidence: [static, attestation], anchor: "#ac-2" }
constraints:
  - { id: co-1, text: "must not add new synchronous cross-service calls", anchor: "#co-1" }
frozen: { at: 2026-06-01, commit: %s }
---
# Loan workflow (VL-015 fixture)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within one minute of the change.

## AC-1

Workflow status changes are visible within one minute.

## AC-2

Workflow history is queryable by loan id.

## CO-1

Must not add new synchronous cross-service calls.
`

// vl015LoanWorkflowV2Tmpl takes the frozen predecessor commit and a
// caller-supplied supersession: block body, producing the superseding
// revision. co-1's own text is fixed to the predecessor's exact text
// ("must not add new synchronous cross-service calls") in every case
// except the carried-byte-drift table row, which overrides it below.
const vl015LoanWorkflowV2Tmpl = `---
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Loan workflow v2 (VL-015 fixture, supersedes v1)"
status: draft
owners: [platform-team]
problem: { text: "loan officers cannot see workflow status changes in real time", anchor: "#problem" }
outcome: { text: "loan officers see workflow status changes within thirty seconds", anchor: "#outcome" }
links:
  - { type: supersedes, ref: spec/loan-workflow }
acceptance_criteria:
  - { id: ac-1, text: "workflow status changes are visible within thirty seconds", evidence: [runtime, attestation], anchor: "#ac-1" }
  - { id: ac-3, text: "workflow status changes emit an audit event", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: %q, anchor: "#co-1" }
supersession:
%s
---
# Loan workflow v2 (VL-015 fixture, supersedes v1)

## Problem

Loan officers only see workflow status changes on their next manual refresh.

## Outcome

Loan officers see workflow status changes within thirty seconds of the change.

## AC-1

Workflow status changes are visible within thirty seconds.

## AC-3

Workflow status changes emit an audit event.

## CO-1

Must not add new synchronous cross-service calls.
`

const vl015PredecessorCO1Text = "must not add new synchronous cross-service calls"

// buildVL015Repo builds the two-layer predecessor history (v1 draft, then
// v1 frozen at its own SHA_A) and a third layer adding the superseding
// revision, with co1Text and supersessionBody plugged into the template
// above.
func buildVL015Repo(t *testing.T, co1Text, supersessionBody string) *fixturegit.Repo {
	t.Helper()

	layer1 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                         setupManifestYAML,
			".gitattributes":                            setupGitAttributes,
			".verdi/specs/active/loan-workflow/spec.md": vl015LoanWorkflowV1DraftYAML,
		},
		Message: "vl015 layer 1: loan-workflow v1 draft",
	}
	repo1 := fixturegit.Build(t, []fixturegit.Layer{layer1})
	shaA := repo1.Head

	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                            setupManifestYAML,
			".gitattributes":                               setupGitAttributes,
			".verdi/specs/active/loan-workflow/spec.md":    fmt.Sprintf(vl015LoanWorkflowV1FrozenTmpl, shaA),
			".verdi/specs/active/loan-workflow-v2/spec.md": fmt.Sprintf(vl015LoanWorkflowV2Tmpl, co1Text, supersessionBody),
		},
		Message: "vl015 layer 2: loan-workflow v1 frozen + loan-workflow-v2",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2})
	provisionMutableZone(t, repo.Dir)
	return repo
}

func TestVL015_TableDriven(t *testing.T) {
	cases := []struct {
		name             string
		co1Text          string
		supersessionBody string
		wantRule         string // "" means clean (no VL-015 finding)
	}{
		{
			name:    "happy: every object classified exactly once, carried byte-identical",
			co1Text: vl015PredecessorCO1Text,
			supersessionBody: `  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`,
			wantRule: "",
		},
		{
			name:    "carried-byte-drift: co-1 carried but text differs from predecessor",
			co1Text: "must not add new SYNCHRONOUS cross-service calls (drifted text)",
			supersessionBody: `  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`,
			wantRule: "VL-015",
		},
		{
			name:    "unclassified-object: co-1 not named in any bucket",
			co1Text: vl015PredecessorCO1Text,
			supersessionBody: `  carried: []
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`,
			wantRule: "VL-015",
		},
		{
			name:    "double-classified: co-1 named in two buckets",
			co1Text: vl015PredecessorCO1Text,
			supersessionBody: `  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: [ { id: co-1, note: "also (wrongly) listed here" } ]
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`,
			wantRule: "VL-015",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildVL015Repo(t, tc.co1Text, tc.supersessionBody)
			findings := runLint(t, repo.Dir, Context{}, Options{})

			if tc.wantRule == "" {
				for _, f := range findings {
					if f.Rule == "VL-015" {
						t.Fatalf("VL-015 fired on the happy-path supersession: %s", f.String())
					}
				}
				return
			}

			onlyRule(t, findings, tc.wantRule)
			if len(findings) != 1 {
				t.Fatalf("got %d findings, want 1:\n%s", len(findings), findingsString(findings))
			}
		})
	}
}
