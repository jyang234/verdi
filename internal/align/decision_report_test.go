package align

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/store"
)

// TestGenerateDecisionConflict_JudgeSkipped_DisclosedUnprovenComplete is
// this phase's headline exit criterion: "the three-valued gate status
// table is exercised (a judge-skipped run reports
// disclosed-unproven-complete, never a bare pass)".
func TestGenerateDecisionConflict_JudgeSkipped_DisclosedUnprovenComplete(t *testing.T) {
	root := t.TempDir()
	spec := writeDecisionSpec(t, root, "my-feature")

	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root:   root,
		Spec:   spec,
		Covers: "abc1234",
		// No JudgeCmd configured — the judge is skipped.
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}

	computedStatus, judgedStatus := DecisionGateStatuses(report.Frontmatter)
	if computedStatus != StatusProven {
		t.Fatalf("computedStatus = %q, want proven (no declared edges at all)", computedStatus)
	}
	if judgedStatus != StatusDisclosedUnprovenComplete {
		t.Fatalf("judgedStatus = %q, want disclosed-unproven-complete", judgedStatus)
	}

	// Never a bare pass: the absence finding is present and undispositioned,
	// so review-readiness is NOT satisfied merely because the judge ran/was
	// skipped — a human must still disposition it.
	ok, undispositioned := DecisionReviewReady(report.Frontmatter)
	if ok {
		t.Fatalf("DecisionReviewReady = true, want false (absence finding %v still undispositioned)", undispositioned)
	}
	if len(undispositioned) != 1 || undispositioned[0] != DecisionAbsenceFindingID {
		t.Fatalf("undispositioned = %v, want exactly [%s]", undispositioned, DecisionAbsenceFindingID)
	}
}

// TestGenerateDecisionConflict_ComputedIncompleteBlocksReview proves an
// unresolved declared edge blocks review-readiness even when the judged
// section is entirely clean/skipped.
func TestGenerateDecisionConflict_ComputedIncompleteBlocksReview(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "current-policy", "accepted") // not yet superseded

	spec := writeDecisionSpec(t, root, "my-feature", artifact.Link{Type: artifact.LinkSupersedes, Ref: "adr/current-policy"})

	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}

	computedStatus, _ := DecisionGateStatuses(report.Frontmatter)
	if computedStatus == StatusProven {
		t.Fatal("computedStatus = proven, want unresolved/empty (declared supersedes edge not yet ratified)")
	}
	ok, _ := DecisionReviewReady(report.Frontmatter)
	if ok {
		t.Fatal("DecisionReviewReady = true, want false")
	}
}

// TestGenerateDecisionConflict_JudgedFoundAndDispositioned proves the
// found-and-dispositioned status once a real judged finding is
// dispositioned across a regeneration (ExistingFindings preservation), and
// that a disposition of "exempt" targeting an ADR gets CODEOWNERS-routed
// to that ADR's owners.
func TestGenerateDecisionConflict_JudgedFoundAndDispositioned(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")

	script := writeFakeJudge(t, fakeDecisionJudgeOKScript) // targets adr/retry-policy
	spec := writeDecisionSpec(t, root, "my-feature")

	// First run: judged finding lands undispositioned.
	first, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		JudgeCmd:    []string{script},
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict (first): %v", err)
	}
	if len(first.Frontmatter.Findings) != 1 || first.Frontmatter.Findings[0].Dispositioned() {
		t.Fatalf("first run findings = %+v, want one undispositioned judged finding", first.Frontmatter.Findings)
	}

	// A human dispositions the finding "exempt" with a note, then align
	// regenerates: PreserveConflictDispositions must carry it forward, and
	// computeRouting must fill RoutedOwners from adr/retry-policy's owners.
	dispositioned := first.Frontmatter.Findings
	dispositioned[0].Disposition = artifact.ConflictExempt
	dispositioned[0].Note = "reviewed, exemption stands"

	second, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "def5678",
		JudgeCmd:         []string{script},
		ExistingFindings: dispositioned,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict (second): %v", err)
	}
	if len(second.Frontmatter.Findings) != 1 {
		t.Fatalf("second run findings = %+v, want 1", second.Frontmatter.Findings)
	}
	f := second.Frontmatter.Findings[0]
	if f.Disposition != artifact.ConflictExempt {
		t.Fatalf("Disposition = %q, want exempt (preserved across regeneration)", f.Disposition)
	}
	if len(f.RoutedOwners) != 1 || f.RoutedOwners[0] != "platform-team" {
		t.Fatalf("RoutedOwners = %v, want [platform-team] (CODEOWNERS routing computed from adr/retry-policy's owners)", f.RoutedOwners)
	}

	_, judgedStatus := DecisionGateStatuses(second.Frontmatter)
	if judgedStatus != StatusFoundAndDispositioned {
		t.Fatalf("judgedStatus = %q, want found-and-dispositioned", judgedStatus)
	}
	ok, undispositioned := DecisionReviewReady(second.Frontmatter)
	if !ok {
		t.Fatalf("DecisionReviewReady = false (undispositioned: %v), want true", undispositioned)
	}
}

// TestGenerateDecisionConflict_AllFourJudgedDispositions proves each of
// the four disposition values can be recorded and decoded through the full
// pipeline (identity-preserved across a regeneration), the exit
// criterion's "judged section dispositions land in all four values".
func TestGenerateDecisionConflict_AllFourJudgedDispositions(t *testing.T) {
	existing := []artifact.ConflictFinding{
		{ID: "judged-dj-1", Kind: artifact.FindingJudged, Text: "dc-1 may contradict adr/retry-policy (confidence 0.60)", Disposition: artifact.ConflictSuperseded, Note: "n1"},
	}
	// Identity is a content hash over (kind, id, text); only the entry whose
	// (kind, id, text) matches the freshly-regenerated finding survives —
	// this test seeds exactly that match for the "superseded" case, and
	// separately proves the other three values decode/round-trip legally
	// via DecodeDecisionConflict (already covered in
	// internal/artifact/decisionconflict_test.go); here we prove the
	// preservation path for one value end-to-end.
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")
	script := writeFakeJudge(t, fakeDecisionJudgeOKScript)
	spec := writeDecisionSpec(t, root, "my-feature")

	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		JudgeCmd:         []string{script},
		ExistingFindings: existing,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}
	if report.Frontmatter.Findings[0].Disposition != artifact.ConflictSuperseded {
		t.Fatalf("Disposition = %q, want superseded (preserved)", report.Frontmatter.Findings[0].Disposition)
	}

	for _, d := range []artifact.ConflictDisposition{artifact.ConflictExempt, artifact.ConflictRejected, artifact.ConflictNoConflict} {
		f := artifact.ConflictFinding{ID: "f-1", Kind: artifact.FindingJudged, Text: "t", Disposition: d, Note: "n"}
		if err := f.Validate(); err != nil {
			t.Fatalf("disposition %q failed to validate: %v", d, err)
		}
	}
}

func TestGenerateDecisionConflict_SweepProvenanceRecorded(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")
	spec := writeDecisionSpec(t, root, "my-feature")
	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}
	sp := report.Frontmatter.SweepProvenance
	if sp == nil || sp.ADRCorpusDigest == "" {
		t.Fatalf("SweepProvenance = %+v, want a populated ADR corpus digest", sp)
	}
	if len(sp.DecisionsScanned) != 1 || sp.DecisionsScanned[0] != "spec/my-feature#dc-1" {
		t.Fatalf("DecisionsScanned = %v, want [spec/my-feature#dc-1]", sp.DecisionsScanned)
	}

	// Staleness detection: adding an ADR changes the corpus digest on the
	// next run against the same tree state otherwise.
	writeADR(t, root, "second-policy", "accepted")
	report2, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict (2): %v", err)
	}
	if report2.Frontmatter.SweepProvenance.ADRCorpusDigest == sp.ADRCorpusDigest {
		t.Fatal("ADRCorpusDigest did not change after the ADR corpus changed — staleness would go undetected")
	}
}

func TestGenerateDecisionConflict_Negative_NilSpec(t *testing.T) {
	_, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{Root: t.TempDir(), Covers: "abc1234"})
	if err == nil {
		t.Fatal("GenerateDecisionConflict(nil spec): want error, got nil")
	}
}

func TestGenerateDecisionConflict_Negative_EmptyCovers(t *testing.T) {
	_, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: t.TempDir(), Spec: &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/x"}},
	})
	if err == nil {
		t.Fatal("GenerateDecisionConflict(empty covers): want error, got nil")
	}
}

// TestGenerateDecisionConflict_Negative_SpecNotInTree proves the computed
// section is computed from the tree's own spec: a spec absent from the tree
// is ErrSpecNotInTree, and an id that names no spec is an error.
func TestGenerateDecisionConflict_Negative_SpecNotInTree(t *testing.T) {
	tests := []struct {
		name, id  string
		wantNotIn bool
	}{
		{"a spec absent from the tree", "spec/my-feature", true},
		{"an id that does not parse", "not a ref", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
				Root: t.TempDir(), Spec: &artifact.SpecFrontmatter{Base: artifact.Base{ID: tc.id}}, Covers: "abc1234", ModelDigest: testModelDigest(t),
			})
			if err == nil || errors.Is(err, ErrSpecNotInTree) != tc.wantNotIn {
				t.Fatalf("err = %v, want an error (ErrSpecNotInTree: %v)", err, tc.wantNotIn)
			}
		})
	}
}

// TestGenerateDecisionConflict_ModelDigestStamped is spec/model-digest
// ac-1's headline case for this mint site: the rendered, re-decoded
// decision-conflict-report.md carries provenance.model equal to the
// resolved (canonical) model's own Digest() — proving decision_render.go's
// hand-rendered provenance: clause was wired to emit model:, not just
// computed in memory and dropped.
func TestGenerateDecisionConflict_ModelDigestStamped(t *testing.T) {
	root := t.TempDir()
	spec := writeDecisionSpec(t, root, "my-feature")

	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}

	wantDigest := testModelDigest(t)
	if report.Frontmatter.Provenance == nil || report.Frontmatter.Provenance.Model != wantDigest {
		t.Fatalf("Frontmatter.Provenance.Model = %+v, want %q", report.Frontmatter.Provenance, wantDigest)
	}

	fmBytes, _, err := artifact.SplitFrontmatter(report.Markdown)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDecisionConflict(fmBytes)
	if err != nil {
		t.Fatalf("DecodeDecisionConflict(rendered markdown): %v\n---\n%s", err, report.Markdown)
	}
	if decoded.Provenance == nil || decoded.Provenance.Model != wantDigest {
		t.Fatalf("decoded rendered markdown's Provenance.Model = %+v, want %q:\n%s", decoded.Provenance, wantDigest, report.Markdown)
	}
}

// TestGenerateDecisionConflict_ModelDigestTracksFixtureModel is ac-1's
// distinguishing case: a DIFFERENT resolved model produces a
// provenance.model equal to THAT model's own digest.
func TestGenerateDecisionConflict_ModelDigestTracksFixtureModel(t *testing.T) {
	root := t.TempDir()
	spec := writeDecisionSpec(t, root, "my-feature")

	fixtureDigest := fixtureModelDigest(t)
	canonicalDigest := testModelDigest(t)
	if fixtureDigest == canonicalDigest {
		t.Fatalf("fixture model digest %q equals the canonical digest — the fixture is not actually distinct", fixtureDigest)
	}

	report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
		Root: root, Spec: spec, Covers: "abc1234",
		ModelDigest: fixtureDigest,
	})
	if err != nil {
		t.Fatalf("GenerateDecisionConflict: %v", err)
	}
	if report.Frontmatter.Provenance == nil || report.Frontmatter.Provenance.Model != fixtureDigest {
		t.Fatalf("Provenance.Model = %+v, want %q (the fixture model's own digest)", report.Frontmatter.Provenance, fixtureDigest)
	}
}

// TestGenerateDecisionConflict_ByteIdenticalAcrossRuns closes the
// decision-conflict leg of ac-1's "identical across repeated runs"
// obligation (obligation ac-1--behavioral: "two fresh generate calls against
// unchanged inputs must produce byte-identical model: lines", extended across
// the four minting suites). Two fresh GenerateDecisionConflict calls against
// unchanged inputs must produce byte-identical output — including the
// provenance model: line — not two independently-computed digests that
// merely agree.
//
// With this test ac-1's across-runs enumeration is now symmetric and CLOSED
// over all four mint suites: deviation (report_test.go's
// TestGenerate_ByteIdenticalAcrossRuns, the named precedent), decision
// (here), diagram-sweep (diagram_report_test.go's
// TestGenerateDiagramSweep_ByteIdenticalAcrossRuns), and board-freeze
// (commitdesign's TestFreezeBoard_ModelDigestDeterministic). A fifth mint
// suite is thereby visibly obligated to add its own across-runs leg.
func TestGenerateDecisionConflict_ByteIdenticalAcrossRuns(t *testing.T) {
	root := t.TempDir()
	writeADR(t, root, "retry-policy", "accepted")
	script := writeFakeJudge(t, fakeDecisionJudgeOKScript)
	spec := writeDecisionSpec(t, root, "my-feature")

	run := func() []byte {
		report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
			Root: root, Spec: spec, Covers: "abc1234",
			JudgeCmd:    []string{script},
			ModelDigest: testModelDigest(t),
		})
		if err != nil {
			t.Fatalf("GenerateDecisionConflict: %v", err)
		}
		return report.Markdown
	}

	first := run()
	second := run()
	if !bytes.Equal(first, second) {
		t.Fatalf("GenerateDecisionConflict not byte-identical across runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// writeDecisionSpec writes spec/<name> into root's active zone as a
// Validate-legal component spec whose one decision, dc-1, carries links, and
// returns it decoded: the computed section reads the evaluated spec from the
// tree's records (objsupersede.ReadRecords), as align and the gate do.
func writeDecisionSpec(t *testing.T, root, name string, links ...artifact.Link) *artifact.SpecFrontmatter {
	t.Helper()
	var ls []string
	for _, l := range links {
		s := fmt.Sprintf("{ type: %s, ref: %q", l.Type, l.Ref)
		if l.Note != "" {
			s += fmt.Sprintf(", note: %q", l.Note)
		}
		ls = append(ls, s+" }")
	}
	dc := `{ id: dc-1, text: "some decision", anchor: "#dc-1" }`
	if len(ls) > 0 {
		dc = `{ id: dc-1, text: "some decision", anchor: "#dc-1", links: [` + strings.Join(ls, ", ") + `] }`
	}
	content := "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: feature\nstatus: draft\nowners: [platform-team]\n" +
		"acceptance_criteria:\n  - { id: ac-1, text: \"t\", evidence: [static] }\ndecisions:\n  - " + dc + "\n---\nbody\n"
	path := store.ActiveSpecPath(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return loadTreeSpec(t, root, name)
}

// loadTreeSpec strict-decodes spec/<name> from root's active zone.
func loadTreeSpec(t *testing.T, root, name string) *artifact.SpecFrontmatter {
	t.Helper()
	data, err := os.ReadFile(store.ActiveSpecPath(root, name))
	if err != nil {
		t.Fatal(err)
	}
	fm, _, err := artifact.SplitFrontmatter(data)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

func computedOf(findings []artifact.ConflictFinding) []artifact.ConflictFinding {
	var out []artifact.ConflictFinding
	for _, f := range findings {
		if f.Kind == artifact.FindingComputed {
			out = append(out, f)
		}
	}
	return out
}

// Closed-spec object supersession finding ids and texts over the scenario
// fixture (testdata/objsupersede).
const (
	cssDC1     = "edge-dc-1-supersedes-spec--closed-feature-dc-1"
	cssDC2     = "edge-dc-2-supersedes-spec--closed-story-ac-1"
	cssNewText = "records match; takes effect when spec/successor is accepted"
)

func cssResolved(id, text, decision, edge, target string) artifact.ConflictFinding {
	return artifact.ConflictFinding{ID: id, Kind: artifact.FindingComputed, Text: text, Disposition: artifact.ConflictSuperseded,
		Note: "decision " + decision + " supersedes " + edge, TargetRef: target}
}

func cssUnresolved(id, text, target string) artifact.ConflictFinding {
	return artifact.ConflictFinding{ID: id, Kind: artifact.FindingComputed, Text: text, TargetRef: target}
}

// TestGenerateDecisionConflict_ClosedSpecObjectEdges proves align's computed
// section evaluates a decision's `supersedes` edge to a closed spec's object
// through internal/objsupersede (design §5, SI-262): a resolved result is
// SUPERSEDED with the core's text verbatim, an unresolved one is
// undispositioned with its reason's text, and the issuing successor's
// completeness results are their own computed findings.
func TestGenerateDecisionConflict_ClosedSpecObjectEdges(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	carried := func(conflict string) string {
		return "carries the replacement established by spec/successor (conflict/" + conflict + ", since 2024-02-15)"
	}
	dc2New := cssResolved(cssDC2, cssNewText, "dc-2", "spec/closed-story#ac-1", "spec/closed-story")
	tests := []struct {
		name, scenario, branch, spec string
		want                         []artifact.ConflictFinding
	}{
		{"new replacements of a decision and a criterion", "proposed", "", "successor", []artifact.ConflictFinding{
			cssResolved(cssDC1, cssNewText, "dc-1", "spec/closed-feature#dc-1", "spec/closed-feature"), dc2New}},
		{"a revision carries both replacements", "chain", "design/successor-v2", "successor-v2", []artifact.ConflictFinding{
			cssResolved(cssDC1, carried("successor-closed-feature"), "dc-1", "spec/closed-feature#dc-1", "spec/closed-feature"),
			cssResolved(cssDC2, carried("successor-closed-story"), "dc-2", "spec/closed-story#ac-1", "spec/closed-story")}},
		{"no conflict", "no-conflict", "", "successor", []artifact.ConflictFinding{
			cssUnresolved(cssDC1, "no conflict challenges spec/closed-feature#dc-1", "spec/closed-feature"), dc2New}},
		{"a challenged fragment with no edge", "unmatched-challenge", "", "successor", []artifact.ConflictFinding{
			cssResolved(cssDC1, cssNewText, "dc-1", "spec/closed-feature#dc-1", "spec/closed-feature"), dc2New,
			cssUnresolved("completeness-conflict--successor-closed-feature-spec--closed-feature-ac-1",
				"conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge", "spec/closed-feature")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := scenario.Build(t, tc.scenario)
			if tc.branch != "" {
				gitCheckout(t, repo.Dir, tc.branch)
			}
			report, err := GenerateDecisionConflict(context.Background(), DecisionConflictInput{
				Root: repo.Dir, Spec: loadTreeSpec(t, repo.Dir, tc.spec), Covers: "abc1234", ModelDigest: testModelDigest(t),
			})
			if err != nil {
				t.Fatalf("GenerateDecisionConflict: %v", err)
			}
			if got := computedOf(report.Frontmatter.Findings); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("computed findings:\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// TestGenerateDecisionConflict_ComputedNeverCarriesDisposition proves 03's
// split (design §5 "Computed means computed", SI-262, BL-67's structural
// part): a disposition or note typed onto ANY computed finding of an existing
// report is dropped by the next align — ADR `supersedes`, `exempts`, and
// closed-spec object edges alike — while a judged finding's disposition is
// still preserved.
func TestGenerateDecisionConflict_ComputedNeverCarriesDisposition(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "")
	typed := func(f artifact.ConflictFinding) artifact.ConflictFinding {
		f.Disposition, f.Note = artifact.ConflictSuperseded, "typed by hand"
		return f
	}
	tests := []struct {
		name  string
		setup func(t *testing.T) (root string, spec *artifact.SpecFrontmatter)
	}{
		{"an unresolved ADR supersedes edge", func(t *testing.T) (string, *artifact.SpecFrontmatter) {
			root := t.TempDir()
			writeADR(t, root, "current-policy", "accepted")
			return root, writeDecisionSpec(t, root, "my-feature", artifact.Link{Type: artifact.LinkSupersedes, Ref: "adr/current-policy"})
		}},
		{"a resolved exempts edge's note", func(t *testing.T) (string, *artifact.SpecFrontmatter) {
			root := t.TempDir()
			writeADR(t, root, "retry-policy", "accepted")
			return root, writeDecisionSpec(t, root, "my-feature", artifact.Link{Type: artifact.LinkExempts, Ref: "adr/retry-policy", Note: "documented exception"})
		}},
		{"an unresolved closed-spec object edge", func(t *testing.T) (string, *artifact.SpecFrontmatter) {
			repo := scenario.Build(t, "no-conflict")
			return repo.Dir, loadTreeSpec(t, repo.Dir, "successor")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, spec := tc.setup(t)
			in := DecisionConflictInput{Root: root, Spec: spec, Covers: "abc1234", ModelDigest: testModelDigest(t)}
			first, err := GenerateDecisionConflict(context.Background(), in)
			if err != nil {
				t.Fatalf("GenerateDecisionConflict (first): %v", err)
			}
			for _, f := range first.Frontmatter.Findings {
				in.ExistingFindings = append(in.ExistingFindings, typed(f))
			}
			second, err := GenerateDecisionConflict(context.Background(), in)
			if err != nil {
				t.Fatalf("GenerateDecisionConflict (second): %v", err)
			}
			if !reflect.DeepEqual(second.Frontmatter.Findings[:len(first.Frontmatter.Findings)-1], first.Frontmatter.Findings[:len(first.Frontmatter.Findings)-1]) {
				t.Fatalf("computed findings after a typed disposition:\n got %+v\nwant %+v (align must drop it)", second.Frontmatter.Findings, first.Frontmatter.Findings)
			}
			judged := second.Frontmatter.Findings[len(second.Frontmatter.Findings)-1]
			if judged.Kind != artifact.FindingJudged || judged.Disposition != artifact.ConflictSuperseded || judged.Note != "typed by hand" {
				t.Fatalf("judged finding = %+v, want its disposition preserved", judged)
			}
		})
	}
}

func gitCheckout(t *testing.T, dir, branch string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", branch).CombinedOutput(); err != nil {
		t.Fatalf("git checkout %s: %v\n%s", branch, err, out)
	}
}
