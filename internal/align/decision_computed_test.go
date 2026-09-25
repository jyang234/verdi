package align

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// writeADR writes .verdi/adr/<name>.md with the given status (proposed,
// accepted, or superseded), a minimal but Validate-legal ADR document.
func writeADR(t *testing.T, root, name, status string) {
	t.Helper()
	writeTreeFile(t, root, ".verdi/adr/"+name+".md", adrDoc(name, status))
}

func adrDoc(name, status string) string {
	var extra string
	switch status {
	case "accepted", "superseded":
		extra = "decided: 2026-01-01\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n"
	}
	return "---\nid: adr/" + name + "\nkind: adr\ntitle: \"" + name + "\"\nstatus: " + status + "\nowners: [platform-team]\n" + extra + "---\nbody\n"
}

// writeActiveSpec writes .verdi/specs/active/<name>/spec.md, a Validate-legal
// draft feature spec with one decision object of the given id — a legal
// supersedes/exempts target (a decision fragment) for another spec's edges.
func writeActiveSpec(t *testing.T, root, name, decisionID string) {
	t.Helper()
	writeTreeFile(t, root, ".verdi/specs/active/"+name+"/spec.md", "---\nid: spec/"+name+"\nkind: spec\ntitle: \""+name+"\"\nclass: feature\nstatus: draft\nowners: [platform-team]\n"+
		"acceptance_criteria:\n  - { id: ac-1, text: \"t\", evidence: [static] }\n"+
		"decisions:\n  - { id: "+decisionID+", text: \"some decision\", anchor: \"#"+decisionID+"\" }\n---\nbody\n")
}

func writeTreeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeEstablisher answers every establishment in force on 2024-02-15 and
// counts the calls it receives.
type fakeEstablisher struct{ calls int }

func (f *fakeEstablisher) Establishment(context.Context, string, artifact.Ref) objsupersede.Establishment {
	f.calls++
	return objsupersede.Establishment{Commit: "c0ffee", Date: "2024-02-15"}
}

func computeEdges(t *testing.T, tr objsupersede.TreeReader, spec string, est objsupersede.Establisher) []artifact.ConflictFinding {
	t.Helper()
	findings, err := ComputeDecisionEdges(context.Background(), tr, spec, est)
	if err != nil {
		t.Fatalf("ComputeDecisionEdges: %v", err)
	}
	for _, f := range findings {
		if err := f.Validate(); err != nil {
			t.Fatalf("finding %+v failed its own Validate: %v", f, err)
		}
	}
	return findings
}

// TestComputeDecisionEdges_EarlierComputationUnchanged pins ADR `supersedes`
// edges, `exempts` edges, and whole-spec `supersedes` targets to their
// earlier computation now that the section reads through a TreeReader; an
// exempts edge to an ADR carries the ADR's owners (the routing the gate's
// recompute compares), and a supersedes edge to an object of a spec that is
// not closed is the core's, never "cannot be computed-resolved".
func TestComputeDecisionEdges_EarlierComputationUnchanged(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, root string)
		links []artifact.Link
		want  []artifact.ConflictFinding
	}{
		{"exempts with a reason resolves and is routed",
			func(t *testing.T, root string) { writeADR(t, root, "retry-policy", "accepted") },
			[]artifact.Link{{Type: artifact.LinkExempts, Ref: "adr/retry-policy", Note: "documented exception"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-exempts-adr--retry-policy", Kind: artifact.FindingComputed, Text: "decision dc-1 exempts adr/retry-policy: resolved (EXEMPT)",
				Disposition: artifact.ConflictExempt, Note: "documented exception", TargetRef: "adr/retry-policy", RoutedOwners: []string{"platform-team"}}}},
		{"exempts without a reason is unresolved",
			func(t *testing.T, root string) { writeADR(t, root, "retry-policy", "accepted") },
			[]artifact.Link{{Type: artifact.LinkExempts, Ref: "adr/retry-policy"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-exempts-adr--retry-policy", Kind: artifact.FindingComputed,
				Text: "decision dc-1 exempts adr/retry-policy: unresolved — an exempts edge requires a reason (link note)", TargetRef: "adr/retry-policy"}}},
		{"supersedes resolves once the target ADR is superseded",
			func(t *testing.T, root string) { writeADR(t, root, "old-policy", "superseded") },
			[]artifact.Link{{Type: artifact.LinkSupersedes, Ref: "adr/old-policy"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-supersedes-adr--old-policy", Kind: artifact.FindingComputed, Text: "decision dc-1 supersedes adr/old-policy: resolved (SUPERSEDED)",
				Disposition: artifact.ConflictSuperseded, Note: "target ADR status is superseded", TargetRef: "adr/old-policy"}}},
		{"supersedes is unresolved while the target ADR is not superseded",
			func(t *testing.T, root string) { writeADR(t, root, "current-policy", "accepted") },
			[]artifact.Link{{Type: artifact.LinkSupersedes, Ref: "adr/current-policy"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-supersedes-adr--current-policy", Kind: artifact.FindingComputed,
				Text: `decision dc-1 supersedes adr/current-policy: unresolved — target ADR status is "accepted", want "superseded" (the supersession has not landed)`, TargetRef: "adr/current-policy"}}},
		{"dangling edges fail closed, even with a reason",
			func(*testing.T, string) {},
			[]artifact.Link{{Type: artifact.LinkSupersedes, Ref: "adr/does-not-exist"}, {Type: artifact.LinkExempts, Ref: "adr/also-missing", Note: "reason present but target dangling"}},
			[]artifact.ConflictFinding{
				{ID: "edge-dc-1-supersedes-adr--does-not-exist", Kind: artifact.FindingComputed, Text: "decision dc-1 supersedes adr/does-not-exist: dangling — target does not exist in the committed corpus", TargetRef: "adr/does-not-exist"},
				{ID: "edge-dc-1-exempts-adr--also-missing", Kind: artifact.FindingComputed, Text: "decision dc-1 exempts adr/also-missing: dangling — target does not exist in the committed corpus", TargetRef: "adr/also-missing"}}},
		{"an undecodable ADR is its finding's unresolved text",
			func(t *testing.T, root string) {
				writeTreeFile(t, root, ".verdi/adr/broken.md", "---\nid: adr/broken\n---\n")
			},
			[]artifact.Link{{Type: artifact.LinkSupersedes, Ref: "adr/broken"}},
			nil},
		{"a whole-spec supersedes target never computed-resolves",
			func(t *testing.T, root string) { writeActiveSpec(t, root, "other-feature", "dc-9") },
			[]artifact.Link{{Type: artifact.LinkSupersedes, Ref: "spec/other-feature"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-supersedes-spec--other-feature", Kind: artifact.FindingComputed, TargetRef: "spec/other-feature",
				Text: "decision dc-1 supersedes spec/other-feature: unresolved — supersedes edges targeting a non-ADR decision cannot be computed-resolved (no independent status field, 02 §Kind registry); resolve via the judged section or file a conflict directly (03 §Challenging closed decisions)"}}},
		{"exempts to a declared spec object resolves",
			func(t *testing.T, root string) { writeActiveSpec(t, root, "other-feature", "dc-9") },
			[]artifact.Link{{Type: artifact.LinkExempts, Ref: "spec/other-feature#dc-9", Note: "excused"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-exempts-spec--other-feature-dc-9", Kind: artifact.FindingComputed, Text: "decision dc-1 exempts spec/other-feature: resolved (EXEMPT)",
				Disposition: artifact.ConflictExempt, Note: "excused", TargetRef: "spec/other-feature"}}},
		{"supersedes to an object of a spec that is not closed is the core's",
			func(t *testing.T, root string) { writeActiveSpec(t, root, "other-feature", "dc-9") },
			[]artifact.Link{{Type: artifact.LinkSupersedes, Ref: "spec/other-feature#dc-9"}},
			[]artifact.ConflictFinding{{ID: "edge-dc-1-supersedes-spec--other-feature-dc-9", Kind: artifact.FindingComputed, Text: "the target spec spec/other-feature is not closed", TargetRef: "spec/other-feature"}}},
		{"no declared edges, no findings", func(*testing.T, string) {}, nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			writeDecisionSpec(t, root, "my-feature", tc.links...)
			est := &fakeEstablisher{}
			got := computeEdges(t, objsupersede.WorkTree{Root: root}, "my-feature", est)
			if tc.name == "an undecodable ADR is its finding's unresolved text" {
				if len(got) != 1 || got[0].Dispositioned() || !strings.HasPrefix(got[0].Text, "decision dc-1 supersedes adr/broken: could not resolve target: .verdi/adr/broken.md:") {
					t.Fatalf("got %+v, want one unresolved could-not-resolve finding naming the repo-relative path", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("findings:\n got %+v\nwant %+v", got, tc.want)
			}
			if est.calls != 0 {
				t.Fatalf("establisher calls = %d, want 0 (no carried edge)", est.calls)
			}
		})
	}
}

// TestComputeDecisionEdges_Negative pins the operational errors: a missing
// reader or establisher, an empty spec name, and a spec that is not a
// decodable spec of the tree (ErrSpecNotInTree, naming undecodable records).
func TestComputeDecisionEdges_Negative(t *testing.T) {
	root := t.TempDir()
	writeDecisionSpec(t, root, "my-feature")
	broken := t.TempDir()
	writeTreeFile(t, broken, ".verdi/specs/active/my-feature/spec.md", "---\nid: spec/my-feature\nkind: spec\n---\n")
	tests := []struct {
		name    string
		tr      objsupersede.TreeReader
		spec    string
		est     objsupersede.Establisher
		wantErr string
		notIn   bool
	}{
		{"no tree reader", nil, "my-feature", &fakeEstablisher{}, "needs a tree reader", false},
		{"no establisher", objsupersede.WorkTree{Root: root}, "my-feature", nil, "needs a tree reader and an establisher", false},
		{"empty spec name", objsupersede.WorkTree{Root: root}, "", &fakeEstablisher{}, "spec name must not be empty", false},
		{"spec absent from the tree", objsupersede.WorkTree{Root: root}, "missing", &fakeEstablisher{}, "spec/missing", true},
		{"spec undecodable in the tree", objsupersede.WorkTree{Root: broken}, "my-feature", &fakeEstablisher{}, "records that do not decode: .verdi/specs/active/my-feature/spec.md", true},
		{"a commit that does not exist", objsupersede.CommitTree{Root: root, Commit: "HEAD"}, "my-feature", &fakeEstablisher{}, "align:", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ComputeDecisionEdges(context.Background(), tc.tr, tc.spec, tc.est)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want one containing %q", err, tc.wantErr)
			}
			if errors.Is(err, ErrSpecNotInTree) != tc.notIn {
				t.Fatalf("errors.Is(err, ErrSpecNotInTree) = %v, want %v (%v)", !tc.notIn, tc.notIn, err)
			}
		})
	}
}

// TestComputeDecisionEdges_UndecodableRecords proves SI-274(6) reaches the
// report: with a record that fails strict decode, every closed-spec object
// edge and the completeness check are unresolved, naming the path, while an
// ADR edge keeps its own computation.
func TestComputeDecisionEdges_UndecodableRecords(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "old-policy", "superseded")
	writeTreeFile(t, root, ".verdi/conflicts/broken.md", "---\nid: conflict/broken\n---\n")
	writeDecisionSpec(t, root, "my-feature",
		artifact.Link{Type: artifact.LinkSupersedes, Ref: "adr/old-policy"},
		artifact.Link{Type: artifact.LinkSupersedes, Ref: "spec/closed-feature#dc-1"})
	got := computeEdges(t, objsupersede.WorkTree{Root: root}, "my-feature", &fakeEstablisher{})
	if len(got) != 3 || got[0].Disposition != artifact.ConflictSuperseded {
		t.Fatalf("findings = %+v, want the resolved ADR edge, the object edge, and one completeness finding", got)
	}
	for i, id := range []string{"edge-dc-1-supersedes-spec--closed-feature-dc-1", "completeness-records-undecodable"} {
		f := got[i+1]
		if f.ID != id || f.Dispositioned() || !strings.HasPrefix(f.Text, "records do not decode: .verdi/conflicts/broken.md:") {
			t.Fatalf("finding %d = %+v, want undispositioned %s naming the undecodable conflict", i+1, f, id)
		}
	}
}

// TestComputeDecisionEdges_CommitTreeMatchesWorkTree proves the gate's
// recompute and align share one computation: over a clean checkout, the
// head commit's records and the working tree's compute identical sections
// (closed-spec object edges, ADR edges, routing), and the commit reader
// reads the commit — an uncommitted edit changes only the working tree's.
func TestComputeDecisionEdges_CommitTreeMatchesWorkTree(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	ctx := context.Background()
	adrRepo := fixturegit.Build(t, []fixturegit.Layer{{Message: "adr edges", Files: map[string]string{
		".verdi/adr/retry-policy.md":             adrDoc("retry-policy", "accepted"),
		".verdi/adr/old-policy.md":               adrDoc("old-policy", "superseded"),
		".verdi/specs/active/my-feature/spec.md": "---\nid: spec/my-feature\nkind: spec\ntitle: \"f\"\nclass: feature\nstatus: draft\nowners: [platform-team]\nacceptance_criteria:\n  - { id: ac-1, text: \"t\", evidence: [static] }\ndecisions:\n  - { id: dc-1, text: \"d\", anchor: \"#dc-1\", links: [ { type: exempts, ref: adr/retry-policy, note: \"n\" }, { type: supersedes, ref: adr/old-policy } ] }\n---\nbody\n",
	}}})
	proposed := scenario.Build(t, "proposed")
	for _, tc := range []struct {
		name, dir, spec string
		want            int
	}{{"ADR edges", adrRepo.Dir, "my-feature", 2}, {"closed-spec object edges", proposed.Dir, "successor", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			work := computeEdges(t, objsupersede.WorkTree{Root: tc.dir}, tc.spec, objsupersede.NewHistory(ctx, tc.dir))
			head := computeEdges(t, objsupersede.CommitTree{Root: tc.dir, Commit: "HEAD"}, tc.spec, objsupersede.NewHistory(ctx, tc.dir))
			if len(work) != tc.want || !reflect.DeepEqual(work, head) {
				t.Fatalf("working tree %+v\nhead commit %+v\nwant %d identical findings", work, head, tc.want)
			}
			for _, f := range work {
				if !f.Dispositioned() {
					t.Fatalf("finding %+v is unresolved; want every edge resolved in this fixture", f)
				}
			}
		})
	}
	if err := os.Remove(filepath.Join(proposed.Dir, ".verdi", "conflicts", "successor-closed-feature.md")); err != nil {
		t.Fatal(err)
	}
	work := computeEdges(t, objsupersede.WorkTree{Root: proposed.Dir}, "successor", objsupersede.NewHistory(ctx, proposed.Dir))
	head := computeEdges(t, objsupersede.CommitTree{Root: proposed.Dir, Commit: "HEAD"}, "successor", objsupersede.NewHistory(ctx, proposed.Dir))
	if work[0].Text != "no conflict challenges spec/closed-feature#dc-1" || head[0].Text != cssNewText {
		t.Fatalf("after an uncommitted delete: working tree %q, head commit %q; want the delete seen only in the working tree", work[0].Text, head[0].Text)
	}
}

// TestComputeDecisionEdges_CompletenessDeduplicated proves completeness
// finding ids are unique (L4 review a, I-1): a conflict that lists the same
// challenged fragment more than once yields ONE completeness finding for
// that (conflict, fragment), so the generated report validates and align
// writes it, while a distinct unmatched fragment keeps its own finding.
func TestComputeDecisionEdges_CompletenessDeduplicated(t *testing.T) {
	const (
		ac1   = "  - { type: challenges, ref: \"spec/closed-feature#ac-1\" }\n"
		co1   = "  - { type: challenges, ref: \"spec/closed-feature#co-1\" }\n"
		idAC1 = "completeness-conflict--successor-closed-feature-spec--closed-feature-ac-1"
		idCO1 = "completeness-conflict--successor-closed-feature-spec--closed-feature-co-1"
	)
	texts := map[string]string{
		idAC1: "conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge",
		idCO1: "conflict/successor-closed-feature challenges spec/closed-feature#co-1, but spec/successor carries no matching edge",
	}
	tests := []struct {
		name  string
		extra string // challenges links added after the fixture's own ac-1 link
		want  []string
	}{
		{"listed once", "", []string{idAC1}},
		{"listed twice", ac1, []string{idAC1}},
		{"listed three times", ac1 + ac1, []string{idAC1}},
		{"a distinct unmatched fragment keeps its own finding", co1 + ac1, []string{idAC1, idCO1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := scenario.Build(t, "unmatched-challenge").Dir
			p := filepath.Join(dir, ".verdi", "conflicts", "successor-closed-feature.md")
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), ac1) {
				t.Fatalf("test setup: the fixture conflict has no %q link", ac1)
			}
			if err := os.WriteFile(p, []byte(strings.Replace(string(b), ac1, ac1+tc.extra, 1)), 0o644); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range computeEdges(t, objsupersede.WorkTree{Root: dir}, "successor", &fakeEstablisher{}) {
				if !strings.HasPrefix(f.ID, "completeness-") {
					continue
				}
				if f.Dispositioned() || f.Text != texts[f.ID] {
					t.Fatalf("completeness finding %+v, want undispositioned with text %q", f, texts[f.ID])
				}
				got = append(got, f.ID)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("completeness finding ids = %v, want %v", got, tc.want)
			}
			if _, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{Root: dir, Spec: loadTreeSpec(t, dir, "successor"), Covers: "abc1234", ModelDigest: testModelDigest(t)}); err != nil {
				t.Fatalf("GenerateDecisionConflict: %v", err)
			}
		})
	}
}

// TestComputeDecisionEdges_MemoizesEstablishment proves one computation asks
// the establisher once per (successor, object): two carried edges to the
// same object read the establishing successor's acceptance once (L3
// re-review OB-3).
func TestComputeDecisionEdges_MemoizesEstablishment(t *testing.T) {
	root := t.TempDir()
	closed, err := os.ReadFile(filepath.Join(scenario.Dir(), "records", "specs", "closed-feature.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeTreeFile(t, root, ".verdi/specs/archive/closed-feature/spec.md", string(closed))
	decisions := "decisions:\n" +
		"  - { id: dc-1, text: \"a\", anchor: \"#dc-1\", links: [ { type: supersedes, ref: \"spec/closed-feature#dc-1\" } ] }\n" +
		"  - { id: dc-2, text: \"b\", anchor: \"#dc-2\", links: [ { type: supersedes, ref: \"spec/closed-feature#dc-1\" } ] }\n"
	head := "kind: spec\nclass: feature\nstatus: draft\nowners: [platform-team]\nacceptance_criteria:\n  - { id: ac-1, text: \"t\", evidence: [static] }\n"
	writeTreeFile(t, root, ".verdi/specs/active/s1/spec.md", "---\nid: spec/s1\ntitle: \"s1\"\n"+head+decisions+"---\nbody\n")
	writeTreeFile(t, root, ".verdi/specs/active/s2/spec.md", "---\nid: spec/s2\ntitle: \"s2\"\n"+head+decisions+
		"links:\n  - { type: supersedes, ref: \"spec/s1\" }\nsupersession:\n  carried: [ac-1, dc-1, dc-2]\n---\nbody\n")
	writeTreeFile(t, root, ".verdi/conflicts/c1.md", "---\nid: conflict/c1\nkind: conflict\ntitle: \"c1\"\nowners: [platform-team]\nstatus: superseded\nresolved_by: spec/s1\n"+
		"links:\n  - { type: challenges, ref: \"spec/closed-feature#dc-1\" }\nfrozen: { at: 2024-02-01, commit: d49dd630388ff05fe4cd7d4084c785045ba15689 }\n---\nbody\n")

	est := &fakeEstablisher{}
	got := computeEdges(t, objsupersede.WorkTree{Root: root}, "s2", est)
	want := "carries the replacement established by spec/s1 (conflict/c1, since 2024-02-15)"
	if len(got) != 2 || got[0].Text != want || got[1].Text != want || got[1].Disposition != artifact.ConflictSuperseded {
		t.Fatalf("findings = %+v, want two carried findings", got)
	}
	if est.calls != 1 {
		t.Fatalf("establisher calls = %d, want 1 (memoized per successor and object)", est.calls)
	}
}
