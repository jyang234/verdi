package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// selfEvidenceSpecMD renders a minimal story spec at name declaring one AC
// with the given evidence kinds — enough for verifySelfHostedACDeclared to
// resolve it and for evidence.Fold to later prove the fold reaches
// evidenced.
func selfEvidenceSpecMD(name, kindsYAML string) string {
	return `---
id: spec/` + name + `
kind: spec
class: story
title: "` + name + `"
status: accepted-pending-build
owners: [platform-team]
story: jira:` + strings.ToUpper(name) + `-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/some-feature#ac-1" }
acceptance_criteria:
  - { id: ac-1, text: "t", evidence: [` + kindsYAML + `] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# body
## Problem
p
## Outcome
o
`
}

// buildSelfEvidenceRepo builds a fixturegit repo carrying two self-hosted
// story specs and a root verdi.bindings.yaml binding a static+behavioral
// producer across BOTH of them via artifact.ResolveBindingAC's
// fragment-qualified form — mirroring the real root verdi.bindings.yaml
// this phase adds (remote-and-ci#ac-1, close-verb#ac-1/#ac-3).
func buildSelfEvidenceRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	bindingsYAML := `schema: verdi.bindings/v1
spec: spec/story-a
bindings:
  - { producer: verdi-verify-behavioral, kind: behavioral, acs: [ac-1, "spec/story-b#ac-1"] }
  - { producer: verdi-verify-static, kind: static, acs: ["spec/story-b#ac-1"] }
`
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":                  "schema: verdi.layout/v1\nforge: gitlab\n",
				".verdi/specs/active/story-a/spec.md": selfEvidenceSpecMD("story-a", "behavioral"),
				".verdi/specs/active/story-b/spec.md": selfEvidenceSpecMD("story-b", "static, behavioral"),
				"verdi.bindings.yaml":                 bindingsYAML,
			},
			Message: "self-hosted evidence fixture",
		},
	})
}

func readVerdicts(t *testing.T, root, specRef, commit string) []artifact.Evidence {
	t.Helper()
	path := filepath.Join(root, ".verdi", "data", "derived", store.RefSlug(specRef), commit, "verdicts.json")
	recs, err := readExistingSelfHostedVerdicts(path)
	if err != nil {
		t.Fatalf("readExistingSelfHostedVerdicts(%s): %v", path, err)
	}
	return recs
}

// TestProduceSelfHostedEvidence_WritesPerSpecRecords proves the producer
// resolves a cross-spec bindings file into per-spec derived directories:
// spec/story-a (bound via a bare "ac-1") gets exactly its own behavioral
// record; spec/story-b (bound via the "spec/story-b#ac-1" fragment form)
// gets both a static and a behavioral record — each source: ci, verdict:
// pass, written under store.RefSlug(spec.ID) (the convention every fold
// consumer actually reads, not sync's branch-keyed bundle).
func TestProduceSelfHostedEvidence_WritesPerSpecRecords(t *testing.T) {
	repo := buildSelfEvidenceRepo(t)
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}

	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}

	aRecs := readVerdicts(t, repo.Dir, "spec/story-a", repo.Head)
	if len(aRecs) != 1 {
		t.Fatalf("spec/story-a verdicts = %+v, want exactly 1 behavioral record", aRecs)
	}
	if aRecs[0].Kind != artifact.EvidenceBehavioral || aRecs[0].Verdict != artifact.VerdictPass {
		t.Fatalf("spec/story-a record = %+v, want behavioral/pass", aRecs[0])
	}
	if aRecs[0].Provenance.Source != artifact.SourceCI {
		t.Fatalf("spec/story-a record provenance = %+v, want source ci", aRecs[0].Provenance)
	}
	if len(aRecs[0].EvidenceFor) != 1 || aRecs[0].EvidenceFor[0] != "ac-1" {
		t.Fatalf("spec/story-a evidence_for = %v, want [ac-1]", aRecs[0].EvidenceFor)
	}

	bRecs := readVerdicts(t, repo.Dir, "spec/story-b", repo.Head)
	if len(bRecs) != 2 {
		t.Fatalf("spec/story-b verdicts = %+v, want exactly 2 records (static + behavioral)", bRecs)
	}
	var sawStatic, sawBehavioral bool
	for _, r := range bRecs {
		if r.Verdict != artifact.VerdictPass || r.Provenance.Source != artifact.SourceCI {
			t.Fatalf("spec/story-b record %+v, want pass/source-ci", r)
		}
		switch r.Kind {
		case artifact.EvidenceStatic:
			sawStatic = true
		case artifact.EvidenceBehavioral:
			sawBehavioral = true
		}
	}
	if !sawStatic || !sawBehavioral {
		t.Fatalf("spec/story-b records = %+v, want both static and behavioral", bRecs)
	}
}

// TestProduceSelfHostedEvidence_FeedsTheRealFold proves the end-to-end
// point of this producer (spec/close-verb ac-3): a story declaring
// [static, behavioral] evidence, with no other evidence anywhere, folds all
// the way to evidenced once this producer has run — on source: ci alone.
func TestProduceSelfHostedEvidence_FeedsTheRealFold(t *testing.T) {
	repo := buildSelfEvidenceRepo(t)
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence: %v", err)
	}

	spec, err := storyresolve.LoadSpec(repo.Dir, "story-b")
	if err != nil {
		t.Fatal(err)
	}
	if spec == nil {
		t.Fatal("storyresolve.LoadSpec(story-b) = nil, want the fixture spec")
	}

	derivedRoot := filepath.Join(repo.Dir, ".verdi", "data", "derived", store.RefSlug(spec.ID))
	records, err := evidence.LoadRecords(context.Background(), repo.Dir, derivedRoot, repo.Head)
	if err != nil {
		t.Fatalf("evidence.LoadRecords: %v", err)
	}
	result, err := evidence.Fold(evidence.Input{Spec: spec, Records: records, Preview: false, StoreRoot: repo.Dir, StorySlug: store.RefSlug(spec.Story)})
	if err != nil {
		t.Fatalf("evidence.Fold: %v", err)
	}
	if len(result.ACs) != 1 || result.ACs[0].Status != evidence.StatusEvidenced {
		t.Fatalf("spec/story-b fold = %+v, want ac-1 evidenced", result.ACs)
	}
	if !result.Eligible {
		t.Fatalf("spec/story-b eligible = %v, want true", result.Eligible)
	}
}

// TestProduceSelfHostedEvidence_NoRootBindings_NoOp proves a store with no
// root verdi.bindings.yaml is a silent no-op, not an error — most repos ARE
// real flowmap services and never need this producer.
func TestProduceSelfHostedEvidence_NoRootBindings_NoOp(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"},
		Message: "no self-hosted bindings",
	}})
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err != nil {
		t.Fatalf("produceSelfHostedEvidence(no bindings file): %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "data", "derived")); err == nil {
		t.Fatal("derived/ was created despite no root bindings file existing")
	}
}

// TestProduceSelfHostedEvidence_DanglingBindingFailsLoudly proves a binding
// naming an AC its target spec does not declare is a hard error, never a
// silent empty cell (03 §Declarations).
func TestProduceSelfHostedEvidence_DanglingBindingFailsLoudly(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                   "schema: verdi.layout/v1\n",
			".verdi/specs/active/story-a/spec.md": selfEvidenceSpecMD("story-a", "behavioral"),
			"verdi.bindings.yaml": `schema: verdi.bindings/v1
spec: spec/story-a
bindings:
  - { producer: verdi-verify-behavioral, kind: behavioral, acs: [ac-99] }
`,
		},
		Message: "dangling binding fixture",
	}})
	prov := artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov); err == nil {
		t.Fatal("produceSelfHostedEvidence(dangling ac-99): want error, got nil")
	}
}

// TestProduceSelfHostedEvidence_IdempotentAcrossReruns proves re-running the
// producer on the SAME commit (a CI retry) replaces its own prior records
// rather than duplicating them, per producer id.
func TestProduceSelfHostedEvidence_IdempotentAcrossReruns(t *testing.T) {
	repo := buildSelfEvidenceRepo(t)
	prov1 := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "1", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov1); err != nil {
		t.Fatalf("produceSelfHostedEvidence (first): %v", err)
	}
	prov2 := artifact.EvidenceProvenance{Source: artifact.SourceCI, Pipeline: "1", Job: "2", Commit: repo.Head}
	if err := produceSelfHostedEvidence(repo.Dir, repo.Head, prov2); err != nil {
		t.Fatalf("produceSelfHostedEvidence (retry): %v", err)
	}

	aRecs := readVerdicts(t, repo.Dir, "spec/story-a", repo.Head)
	if len(aRecs) != 1 {
		t.Fatalf("spec/story-a verdicts after retry = %+v, want still exactly 1 (replaced, not duplicated)", aRecs)
	}
	if aRecs[0].Provenance.Job != "2" {
		t.Fatalf("spec/story-a record job = %q, want the retry's job %q to have replaced the first", aRecs[0].Provenance.Job, "2")
	}
}
