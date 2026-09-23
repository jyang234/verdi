package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
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
				".verdi/verdi.yaml":                   "schema: verdi.layout/v1\nforge: gitlab\n",
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
	recs, err := readExistingEvidenceRecords(path)
	if err != nil {
		t.Fatalf("readExistingEvidenceRecords(%s): %v", path, err)
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

// evRec is a minimal valid record for producer, attesting ac, told apart from
// its siblings by witness.
func evRec(producer, ac, witness string, verdict artifact.EvidenceVerdict) artifact.Evidence {
	return artifact.Evidence{
		Schema: "verdi.evidence/v1", EvidenceFor: []string{ac}, Kind: artifact.EvidenceBehavioral,
		Verdict: verdict, Witness: witness, Producer: producer,
		Provenance: artifact.EvidenceProvenance{Source: artifact.SourceCI, Commit: "c0ffee0"},
		Digest:     "sha256:" + strings.Repeat("0", 64),
	}
}

// witnesses lists recs' witnesses in order, for comparing record sequences.
func witnesses(recs []artifact.Evidence) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.Witness
	}
	return out
}

// TestMergeEvidenceByProducer pins the merge helper runtimeprobe.go's
// writeRuntimeRecord uses, unchanged by SI-238: an existing record is replaced
// only by an incoming record with its producer; one whose producer has no
// incoming record is kept, in order, ahead of the incoming records.
func TestMergeEvidenceByProducer(t *testing.T) {
	pass := artifact.VerdictPass
	cases := []struct {
		name               string
		existing, incoming []artifact.Evidence
		want               []string
	}{
		{"nothing existing", nil, []artifact.Evidence{evRec("p1", "ac-1", "new-p1", pass)}, []string{"new-p1"}},
		{"same producer replaced", []artifact.Evidence{evRec("p1", "ac-1", "old-p1", pass)}, []artifact.Evidence{evRec("p1", "ac-1", "new-p1", pass)}, []string{"new-p1"}},
		{"a producer with no incoming record is kept", []artifact.Evidence{evRec("p1", "ac-1", "old-p1", pass), evRec("p2", "ac-2", "old-p2", pass)}, []artifact.Evidence{evRec("p2", "ac-2", "new-p2", pass)}, []string{"old-p1", "new-p2"}},
		{"nothing incoming keeps everything", []artifact.Evidence{evRec("p1", "ac-1", "old-p1", pass)}, nil, []string{"old-p1"}},
		{"every existing record of a replaced producer goes", []artifact.Evidence{evRec("p1", "ac-1", "old-p1a", pass), evRec("p3", "ac-3", "old-p3", pass), evRec("p1", "ac-2", "old-p1b", pass)}, []artifact.Evidence{evRec("p1", "ac-1", "new-p1", pass)}, []string{"old-p3", "new-p1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := witnesses(mergeEvidenceByProducer(c.existing, c.incoming)); !reflect.DeepEqual(got, c.want) {
				t.Errorf("mergeEvidenceByProducer = %q, want %q", got, c.want)
			}
		})
	}
}

// TestRuntimeProbe_WriteRuntimeRecordKeepsOtherProducers proves the runtime
// producer's write is unchanged by SI-238: a record for one check replaces
// only that check's earlier record in runtime.json, and another check's
// record, which this write did not mention, stays.
func TestRuntimeProbe_WriteRuntimeRecordKeepsOtherProducers(t *testing.T) {
	root := t.TempDir()
	const commit = "c0ffee0"
	rt := func(producer, ac, witness string) artifact.Evidence {
		r := evRec(producer, ac, witness, artifact.VerdictPass)
		r.Kind = artifact.EvidenceRuntime
		return r
	}
	for _, rec := range []artifact.Evidence{rt("probe:ac-1", "ac-1", "first ac-1"), rt("probe:ac-2", "ac-2", "only ac-2"), rt("probe:ac-1", "ac-1", "second ac-1")} {
		if err := writeRuntimeRecord(root, commit, "spec/story-r", rec); err != nil {
			t.Fatalf("writeRuntimeRecord: %v", err)
		}
	}
	recs, err := readExistingEvidenceRecords(filepath.Join(store.DerivedSpecDir(root, store.RefSlug("spec/story-r")), commit, "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := witnesses(recs), []string{"only ac-2", "second ac-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("runtime.json witnesses = %q, want %q", got, want)
	}
}

// TestReplaceManagedEvidence proves SI-238's per-spec replacement: every
// existing record whose producer is in the managed subset, or is an incoming
// record's producer, is dropped; every other existing record is kept in order
// ahead of the incoming records; and a dropped record whose producer has no
// incoming record is reported withdrawn. The result is never nil, so an
// emptied file is written as an empty array.
func TestReplaceManagedEvidence(t *testing.T) {
	pass, fail := artifact.VerdictPass, artifact.VerdictFail
	managed := func(ps ...string) map[string]bool {
		m := map[string]bool{}
		for _, p := range ps {
			m[p] = true
		}
		return m
	}
	cases := []struct {
		name               string
		existing           []artifact.Evidence
		managed            map[string]bool
		incoming           []artifact.Evidence
		want, wantWithdraw []string
	}{
		{"no managed subset is the merge helper's rule",
			[]artifact.Evidence{evRec("p1", "ac-1", "old-p1", pass), evRec("p2", "ac-2", "old-p2", pass)}, nil,
			[]artifact.Evidence{evRec("p1", "ac-1", "new-p1", fail)},
			[]string{"old-p2", "new-p1"}, nil},
		{"a managed producer with no incoming record is withdrawn",
			[]artifact.Evidence{evRec("coarse", "ac-1", "coarse", pass), evRec("pA", "ac-1", "old-pA", pass)}, managed("pA"), nil,
			[]string{"coarse"}, []string{"old-pA"}},
		{"a replaced managed producer is not withdrawn",
			[]artifact.Evidence{evRec("pA", "ac-1", "old-pA", pass)}, managed("pA"),
			[]artifact.Evidence{evRec("pA", "ac-1", "new-pA", fail)},
			[]string{"new-pA"}, nil},
		{"mixed: one replaced, one withdrawn, unmanaged kept in order",
			[]artifact.Evidence{evRec("c1", "ac-9", "c1", pass), evRec("pA", "ac-1", "old-pA", pass), evRec("c2", "ac-9", "c2", fail), evRec("pB", "ac-2", "old-pB", pass)}, managed("pA", "pB"),
			[]artifact.Evidence{evRec("pA", "ac-1", "new-pA", pass)},
			[]string{"c1", "c2", "new-pA"}, []string{"old-pB"}},
		{"every earlier record of a withdrawn producer",
			[]artifact.Evidence{evRec("pA", "ac-1", "old-pA-1", pass), evRec("pA", "ac-2", "old-pA-2", fail)}, managed("pA"), nil,
			[]string{}, []string{"old-pA-1", "old-pA-2"}},
		{"a managed producer with no earlier record withdraws nothing",
			[]artifact.Evidence{evRec("coarse", "ac-1", "coarse", pass)}, managed("pA"), nil,
			[]string{"coarse"}, nil},
		{"nothing existing and nothing incoming", nil, managed("pA"), nil, []string{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, withdrawn := replaceManagedEvidence(c.existing, c.managed, c.incoming)
			if out == nil {
				t.Fatal("replaceManagedEvidence returned a nil record set, which would marshal as null")
			}
			if got := witnesses(out); !reflect.DeepEqual(got, c.want) {
				t.Errorf("records = %q, want %q", got, c.want)
			}
			if got := witnesses(withdrawn); len(got) != len(c.wantWithdraw) || (len(got) > 0 && !reflect.DeepEqual(got, c.wantWithdraw)) {
				t.Errorf("withdrawn = %q, want %q", got, c.wantWithdraw)
			}
		})
	}
}

// TestWriteManagedEvidence proves the writer around replaceManagedEvidence:
// it reads every affected spec's verdicts.json before writing any, so an
// undecodable file writes nothing anywhere; it creates nothing for a spec with
// no file and nothing to add; it leaves a file it would not change untouched;
// it writes an emptied file as an empty array; and it reports each spec's
// withdrawn records.
func TestWriteManagedEvidence(t *testing.T) {
	const commit = "c0ffee0"
	pass := artifact.VerdictPass
	pathOf := func(root, spec string) string {
		return filepath.Join(store.DerivedSpecDir(root, store.RefSlug(spec)), commit, "verdicts.json")
	}
	put := func(t *testing.T, root, spec, content string) {
		t.Helper()
		p := pathOf(root, spec)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	canon := func(t *testing.T, recs ...artifact.Evidence) string {
		t.Helper()
		b, err := canonjson.Marshal(recs)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	t.Run("no file and nothing to add creates nothing", func(t *testing.T) {
		root := t.TempDir()
		withdrawn, err := writeManagedEvidence(root, commit, map[string]map[string]bool{"spec/a": {"pA": true}}, nil)
		if err != nil || len(withdrawn) != 0 {
			t.Fatalf("writeManagedEvidence = (%v, %v), want nothing withdrawn and no error", withdrawn, err)
		}
		if _, err := os.Stat(filepath.Dir(pathOf(root, "spec/a"))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("stat err = %v, want no directory created", err)
		}
	})
	t.Run("an unchanged file is not rewritten", func(t *testing.T) {
		root := t.TempDir()
		const hand = "[ {\"schema\":\"verdi.evidence/v1\",\"evidence_for\":[\"ac-1\"],\"kind\":\"behavioral\",\"verdict\":\"pass\",\"witness\":\"w\",\"producer\":\"coarse\",\"provenance\":{\"source\":\"ci\",\"commit\":\"c0ffee0\"},\"digest\":\"sha256:0000000000000000000000000000000000000000000000000000000000000000\"} ]"
		put(t, root, "spec/a", hand)
		if _, err := writeManagedEvidence(root, commit, map[string]map[string]bool{"spec/a": {"pA": true}}, nil); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(pathOf(root, "spec/a")); string(got) != hand {
			t.Errorf("file = %q, want it untouched: %q", got, hand)
		}
	})
	t.Run("an emptied file is an empty array and its records are withdrawn", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "spec/a", canon(t, evRec("pA", "ac-1", "old-pA", pass)))
		withdrawn, err := writeManagedEvidence(root, commit, map[string]map[string]bool{"spec/a": {"pA": true}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := witnesses(withdrawn["spec/a"]); !reflect.DeepEqual(got, []string{"old-pA"}) {
			t.Errorf("withdrawn[spec/a] = %q, want [old-pA]", got)
		}
		if got, _ := os.ReadFile(pathOf(root, "spec/a")); string(got) != "[]\n" {
			t.Errorf("file = %q, want %q", got, "[]\n")
		}
	})
	t.Run("an undecodable file writes nothing anywhere", func(t *testing.T) {
		root := t.TempDir()
		aBefore := canon(t, evRec("pA", "ac-1", "old-pA", pass))
		put(t, root, "spec/a", aBefore)
		put(t, root, "spec/b", "not json")
		_, err := writeManagedEvidence(root, commit,
			map[string]map[string]bool{"spec/a": {"pA": true}, "spec/b": {"pB": true}},
			map[string][]artifact.Evidence{"spec/a": {evRec("pA", "ac-1", "new-pA", pass)}})
		if err == nil || !strings.Contains(err.Error(), "verdicts.json") {
			t.Fatalf("err = %v, want one naming the undecodable verdicts.json", err)
		}
		if got, _ := os.ReadFile(pathOf(root, "spec/a")); string(got) != aBefore {
			t.Errorf("spec/a = %q, want it untouched: %q", got, aBefore)
		}
	})
	t.Run("specs are written per their own subset", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "spec/a", canon(t, evRec("pA", "ac-1", "old-a-pA", pass), evRec("coarse", "ac-1", "coarse", pass)))
		bBefore := canon(t, evRec("pA", "ac-1", "old-b-pA", pass))
		put(t, root, "spec/b", bBefore)
		withdrawn, err := writeManagedEvidence(root, commit, map[string]map[string]bool{"spec/a": {"pA": true}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(withdrawn) != 1 || len(withdrawn["spec/a"]) != 1 {
			t.Errorf("withdrawn = %+v, want only spec/a's pA record", withdrawn)
		}
		if got := witnesses(readVerdicts(t, root, "spec/a", commit)); !reflect.DeepEqual(got, []string{"coarse"}) {
			t.Errorf("spec/a = %q, want [coarse]", got)
		}
		if got, _ := os.ReadFile(pathOf(root, "spec/b")); string(got) != bBefore {
			t.Errorf("spec/b = %q, want it untouched", got)
		}
	})
}
