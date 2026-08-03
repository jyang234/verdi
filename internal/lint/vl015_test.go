package lint

import (
	"fmt"
	"strings"
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

// vl015RateLockYAML is a second, otherwise-unrelated predecessor spec the
// predecessor-cardinality rows point their EXTRA supersedes edge at, so the
// edge resolves (no VL-003 dangling-ref storm) and the only thing under
// test is the cardinality of whole-spec supersedes edges themselves.
const vl015RateLockYAML = `---
id: spec/rate-lock
kind: spec
class: feature
title: "Rate lock (VL-015 second-predecessor fixture)"
status: draft
owners: [platform-team]
problem: { text: "borrowers lose a good quoted rate the moment they pause", anchor: "#problem" }
outcome: { text: "borrowers can lock a quoted rate and finish later", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-9, text: "a borrower can lock a quoted rate for a fixed window", evidence: [static, attestation], anchor: "#ac-9" }
---
# Rate lock (VL-015 second-predecessor fixture)

## Problem

Borrowers lose a good quoted rate the moment they pause the application.

## Outcome

Borrowers can lock a quoted rate and finish later.

## AC-9

A borrower can lock a quoted rate for a fixed window.
`

// buildVL015LinksRepo is buildVL015Repo with the superseding revision's
// links: block rewritten wholesale (and spec/rate-lock added to the store
// so a second whole-spec edge resolves).
func buildVL015LinksRepo(t *testing.T, linksBody string) *fixturegit.Repo {
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

	v2 := fmt.Sprintf(vl015LoanWorkflowV2Tmpl, vl015PredecessorCO1Text, `  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`)
	const templateLinks = "links:\n  - { type: supersedes, ref: spec/loan-workflow }\n"
	if !strings.Contains(v2, templateLinks) {
		t.Fatal("test setup: vl015LoanWorkflowV2Tmpl's links: block no longer matches — this helper cannot rewrite it")
	}
	v2 = strings.Replace(v2, templateLinks, linksBody, 1)

	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                            setupManifestYAML,
			".gitattributes":                               setupGitAttributes,
			".verdi/specs/active/loan-workflow/spec.md":    fmt.Sprintf(vl015LoanWorkflowV1FrozenTmpl, shaA),
			".verdi/specs/active/rate-lock/spec.md":        vl015RateLockYAML,
			".verdi/specs/active/loan-workflow-v2/spec.md": v2,
		},
		Message: "vl015 layer 2: loan-workflow v1 frozen + loan-workflow-v2 + rate-lock",
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2})
	provisionMutableZone(t, repo.Dir)
	return repo
}

// TestVL015_PredecessorCardinality proves the lint surface of the
// exactly-one-whole-spec-predecessor invariant: a supersession: manifest is
// ABOUT one named predecessor's objects, so a revision carrying two
// whole-spec supersedes edges (the P1 defect: the ONE manifest was credited
// to BOTH) is reported, not silently validated against whichever edge came
// first. A fragment supersedes edge alongside the single whole-spec one is
// a decision-level override and must stay clean.
func TestVL015_PredecessorCardinality(t *testing.T) {
	cases := []struct {
		name      string
		linksBody string
		wantVL015 bool
	}{
		{
			name:      "exactly one whole-spec predecessor: clean",
			linksBody: "links:\n  - { type: supersedes, ref: spec/loan-workflow }\n",
			wantVL015: false,
		},
		{
			name: "one whole-spec predecessor plus a fragment override edge: clean",
			linksBody: "links:\n" +
				"  - { type: supersedes, ref: spec/loan-workflow }\n" +
				"  - { type: supersedes, ref: \"spec/rate-lock#ac-9\" }\n",
			wantVL015: false,
		},
		{
			name: "TWO whole-spec predecessors: reported",
			linksBody: "links:\n" +
				"  - { type: supersedes, ref: spec/loan-workflow }\n" +
				"  - { type: supersedes, ref: spec/rate-lock }\n",
			wantVL015: true,
		},
		{
			name:      "NO whole-spec predecessor, only a fragment edge: reported",
			linksBody: "links:\n  - { type: supersedes, ref: \"spec/rate-lock#ac-9\" }\n",
			wantVL015: true,
		},
		{
			name:      "no supersedes edge at all: reported",
			linksBody: "",
			wantVL015: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildVL015LinksRepo(t, tc.linksBody)
			findings := runLint(t, repo.Dir, Context{}, Options{})

			var vl015 []Finding
			for _, f := range findings {
				if f.Rule == "VL-015" {
					vl015 = append(vl015, f)
				}
			}
			if !tc.wantVL015 {
				if len(vl015) != 0 {
					t.Fatalf("VL-015 fired on a legal predecessor shape:\n%s", findingsString(vl015))
				}
				return
			}
			if len(vl015) == 0 {
				t.Fatalf("no VL-015 finding, want one naming the whole-spec predecessor cardinality violation.\nall findings:\n%s", findingsString(findings))
			}
			if !strings.Contains(vl015[0].Message, "exactly one") {
				t.Fatalf("VL-015 message = %q, want it to state the exactly-one-whole-spec-predecessor rule", vl015[0].Message)
			}
		})
	}
}

// TestVL015_FrozenPredecessor_ReadsFrozenCommit_NotLaterEditedBytes proves
// the LEGACY frozen.commit path — unchanged by the merge-signaled addition
// — still reads the predecessor's manifest at ITS FROZEN COMMIT
// specifically, never whatever bytes the predecessor file happens to carry
// now: a third layer edits the already-frozen predecessor's own committed
// text (without touching its frozen: stamp — mechanically possible even
// though never legitimate in practice) after freezing, and the successor's
// ORIGINAL-text carried claim (matching the frozen commit, not the later
// edit) still reads clean. A regression that started reading the
// predecessor's current committed bytes instead of frozen.commit would red
// this on the drifted text.
func TestVL015_FrozenPredecessor_ReadsFrozenCommit_NotLaterEditedBytes(t *testing.T) {
	layer1 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                         setupManifestYAML,
			".gitattributes":                            setupGitAttributes,
			".verdi/specs/active/loan-workflow/spec.md": vl015LoanWorkflowV1DraftYAML,
		},
		Message: "vl015 frozen-read layer 1: loan-workflow v1 draft",
	}
	repo1 := fixturegit.Build(t, []fixturegit.Layer{layer1})
	shaA := repo1.Head

	supersessionBody := `  carried: [co-1]
  amended: [ { id: ac-1, note: "tightened the visibility threshold" } ]
  amended_advisory: []
  removed: [ { id: ac-2, note: "moved to a separate reporting feature" } ]
  added: [ac-3]`

	layer2 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                            setupManifestYAML,
			".gitattributes":                               setupGitAttributes,
			".verdi/specs/active/loan-workflow/spec.md":    fmt.Sprintf(vl015LoanWorkflowV1FrozenTmpl, shaA),
			".verdi/specs/active/loan-workflow-v2/spec.md": fmt.Sprintf(vl015LoanWorkflowV2Tmpl, vl015PredecessorCO1Text, supersessionBody),
		},
		Message: "vl015 frozen-read layer 2: loan-workflow v1 frozen + loan-workflow-v2",
	}

	frozenAtLayer2 := fmt.Sprintf(vl015LoanWorkflowV1FrozenTmpl, shaA)
	if !strings.Contains(frozenAtLayer2, vl015PredecessorCO1Text) {
		t.Fatal("test setup: frozen predecessor template's co-1 text no longer matches vl015PredecessorCO1Text — cannot inject post-freeze drift")
	}
	driftedAfterFreeze := strings.Replace(frozenAtLayer2, vl015PredecessorCO1Text, "must not add new SYNCHRONOUS cross-service calls (edited after freeze)", 1)

	layer3 := fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml":                            setupManifestYAML,
			".gitattributes":                               setupGitAttributes,
			".verdi/specs/active/loan-workflow/spec.md":    driftedAfterFreeze,
			".verdi/specs/active/loan-workflow-v2/spec.md": fmt.Sprintf(vl015LoanWorkflowV2Tmpl, vl015PredecessorCO1Text, supersessionBody),
		},
		Message: "vl015 frozen-read layer 3: edit the frozen predecessor's own bytes post-freeze",
	}

	repo := fixturegit.Build(t, []fixturegit.Layer{layer1, layer2, layer3})
	provisionMutableZone(t, repo.Dir)

	findings := runLint(t, repo.Dir, Context{}, Options{})
	for _, f := range findings {
		if f.Rule == "VL-015" {
			t.Fatalf("VL-015 fired even though the successor's carried claim matches the FROZEN COMMIT's original text (the predecessor file's own committed bytes drifted AFTER freezing, which the frozen.commit read must be immune to): %s", f.String())
		}
	}
}
