package align

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// fakeDiagramJudgeOKScript mirrors fakeDecisionJudgeOKScript
// (decision_judge_test.go) for the diagram-sweep mode's own inner-result
// shape — the SAME judge exchange envelope, a different (but structurally
// identical) findings contract.
const fakeDiagramJudgeOKScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"dsj-1\",\"text\":\"new sync edge collides with the outbox mandate\",\"confidence\":0.72,\"target\":\"adr/outbox-mandate\"}]}"}
EOF
`

// TestRunDiagramSweep_Success is spec/judged-sweep ac-2's behavioral
// obligation, first half: a fake judge response producing a finding,
// decoded into an artifact.ConflictFinding.
func TestRunDiagramSweep_Success(t *testing.T) {
	script := writeFakeJudge(t, fakeDiagramJudgeOKScript)
	res, err := RunDiagramSweep(context.Background(), ExecJudgeRunner{}, DiagramJudgedInput{
		DiagramRef: "diagram/loansvc-future",
		Body:       []byte("graph TD\n  a --> b\n"),
		JudgeCmd:   []string{script},
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunDiagramSweep: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", res.Findings)
	}
	f := res.Findings[0]
	if f.Kind != artifact.FindingJudged {
		t.Fatalf("Kind = %q, want judged", f.Kind)
	}
	if f.ID != "judged-dsj-1" {
		t.Fatalf("ID = %q, want judged-dsj-1", f.ID)
	}
	if !strings.Contains(f.Text, "outbox mandate") {
		t.Fatalf("Text = %q, want it to mention the outbox mandate", f.Text)
	}
	if f.TargetRef != "adr/outbox-mandate" {
		t.Fatalf("TargetRef = %q, want adr/outbox-mandate", f.TargetRef)
	}
	if res.Integrity == "" || res.JudgeIntegrity == nil {
		t.Fatal("Integrity/JudgeIntegrity must be populated on a real judge exchange")
	}
}

// TestRunDiagramSweep_NotConfiguredDegradesToAbsence is spec/judged-sweep
// ac-2's behavioral obligation, second half: no judge command configured
// degrades to the synthetic absence finding rather than erroring out or
// silently returning zero findings.
func TestRunDiagramSweep_NotConfiguredDegradesToAbsence(t *testing.T) {
	res, err := RunDiagramSweep(context.Background(), ExecJudgeRunner{}, DiagramJudgedInput{
		DiagramRef: "diagram/loansvc-future",
		Body:       []byte("graph TD\n  a --> b\n"),
	})
	if err != nil {
		t.Fatalf("RunDiagramSweep: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].ID != DiagramAbsenceFindingID {
		t.Fatalf("Findings = %+v, want exactly one %s", res.Findings, DiagramAbsenceFindingID)
	}
	if res.Findings[0].Dispositioned() {
		t.Fatal("synthetic absence finding must start undispositioned")
	}
	if res.Integrity != "" || res.JudgeIntegrity != nil {
		t.Fatal("absence result must carry no integrity record")
	}
}

// TestRunDiagramSweep_FailingJudgeDegradesToAbsence proves a configured but
// failing judge command (non-zero exit) ALSO degrades to the synthetic
// absence finding rather than erroring out — the same disclosed-failure
// posture as the not-configured case, exercised over a different
// JudgeFailure stage.
func TestRunDiagramSweep_FailingJudgeDegradesToAbsence(t *testing.T) {
	script := writeFakeJudge(t, "echo boom >&2\nexit 1\n")
	res, err := RunDiagramSweep(context.Background(), ExecJudgeRunner{}, DiagramJudgedInput{
		DiagramRef: "diagram/loansvc-future",
		Body:       []byte("graph TD\n  a --> b\n"),
		JudgeCmd:   []string{script},
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunDiagramSweep: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].ID != DiagramAbsenceFindingID {
		t.Fatalf("Findings = %+v, want exactly one %s", res.Findings, DiagramAbsenceFindingID)
	}
	if !strings.Contains(res.Findings[0].Text, "boom") {
		t.Fatalf("absence finding text = %q, want it to carry the stderr snippet", res.Findings[0].Text)
	}
}

// TestRunDiagramSweep_JudgeRequiredAndAbsent proves the judge_required
// posture mirrors RunDecisionSweep's own (an *ErrDiagramJudgeRequiredAbsent,
// not a swallowed absence finding, when the caller declared the judge
// mandatory).
func TestRunDiagramSweep_JudgeRequiredAndAbsent(t *testing.T) {
	_, err := RunDiagramSweep(context.Background(), ExecJudgeRunner{}, DiagramJudgedInput{
		DiagramRef:    "diagram/loansvc-future",
		Body:          []byte("graph TD\n"),
		JudgeRequired: true,
	})
	if err == nil {
		t.Fatal("RunDiagramSweep: want error when judge_required and no judge configured")
	}
	var reqAbsent *ErrDiagramJudgeRequiredAbsent
	if !errors.As(err, &reqAbsent) {
		t.Fatalf("err = %v, want *ErrDiagramJudgeRequiredAbsent", err)
	}
}

// TestBuildDiagramSweepPrompt_Deterministic proves the prompt builder is a
// pure function of already-deterministic inputs (spec/judged-sweep dc-3's
// "BuildDiagramSweepPrompt's own render" doc comment).
func TestBuildDiagramSweepPrompt_Deterministic(t *testing.T) {
	ctx := diagramSweepContext{DiagramRef: "diagram/foo", Body: []byte("graph TD\n  a --> b\n")}
	p1 := BuildDiagramSweepPrompt(ctx)
	p2 := BuildDiagramSweepPrompt(ctx)
	if string(p1) != string(p2) {
		t.Fatal("BuildDiagramSweepPrompt is not deterministic over identical inputs")
	}
	if len(p1) == 0 {
		t.Fatal("BuildDiagramSweepPrompt: empty prompt")
	}
	if !strings.Contains(string(p1), "graph TD") {
		t.Fatal("BuildDiagramSweepPrompt: prompt does not include the diagram's mermaid body")
	}
}

// TestBuildDiagramSweepPrompt_IncludesSpecConstraintsAndDecisions proves
// ac-2's "reading ... the corpus's declared constraints/decisions" — not
// merely the diagram's mermaid text alone (the obligation's own wording).
func TestBuildDiagramSweepPrompt_IncludesSpecConstraintsAndDecisions(t *testing.T) {
	spec := &artifact.SpecFrontmatter{
		Base:        artifact.Base{ID: "spec/store-layout-notes"},
		Constraints: []artifact.Constraint{{ID: "co-1", Text: "constraint text marker"}},
		Decisions:   []artifact.Decision{{ID: "dc-1", Text: "decision text marker"}},
	}
	prompt := string(BuildDiagramSweepPrompt(diagramSweepContext{
		DiagramRef: "diagram/foo",
		Body:       []byte("graph TD\n"),
		Spec:       spec,
	}))
	if !strings.Contains(prompt, "constraint text marker") {
		t.Fatalf("prompt = %q, want it to include the resolved spec's constraint text", prompt)
	}
	if !strings.Contains(prompt, "decision text marker") {
		t.Fatalf("prompt = %q, want it to include the resolved spec's decision text", prompt)
	}
}

// writeFeatureSpecWithDecision writes a minimal, Validate-legal
// feature-class spec.md carrying one constraint and one decision — unlike
// writeActiveSpec (decision_computed_test.go), which writes a
// component-class spec that a real DecodeSpec call actually rejects for
// carrying feature/story-only fields (harmless there, since
// computeOneEdge/resolveDecisionTarget swallows that error into an
// undispositioned "could not resolve" finding rather than propagating it;
// resolveDiagramSpec, by contrast, propagates readSpecByName's error
// directly, so its own test needs a target that genuinely decodes).
func writeFeatureSpecWithDecision(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", "active", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nid: spec/" + name + "\nkind: spec\ntitle: \"" + name + "\"\nclass: feature\nstatus: draft\nowners: [platform-team]\n" +
		"acceptance_criteria:\n  - { id: ac-1, text: \"t\", evidence: [static] }\n" +
		"constraints:\n  - { id: co-1, text: \"constraint text marker\", anchor: \"#co-1\" }\n" +
		"decisions:\n  - { id: dc-1, text: \"decision text marker\", anchor: \"#dc-1\" }\n" +
		"---\n# body\n"
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveDiagramSpec proves a diagram carrying a derived-from link to a
// spec resolves it, and a diagram with no such link resolves to nil (ADR
// corpus only sweep), mirroring resolveFeatureSpec's own two cases.
func TestResolveDiagramSpec(t *testing.T) {
	root := t.TempDir()
	writeFeatureSpecWithDecision(t, root, "store-layout-notes")

	t.Run("resolves via derived-from link", func(t *testing.T) {
		diag := &artifact.DiagramFrontmatter{
			Base: artifact.Base{ID: "diagram/loansvc-topology", Links: []artifact.Link{
				{Type: artifact.LinkDerivedFrom, Ref: "spec/store-layout-notes"},
			}},
		}
		spec, err := resolveDiagramSpec(root, diag)
		if err != nil {
			t.Fatalf("resolveDiagramSpec: %v", err)
		}
		if spec == nil || spec.ID != "spec/store-layout-notes" {
			t.Fatalf("resolveDiagramSpec = %+v, want spec/store-layout-notes", spec)
		}
	})

	t.Run("no derived-from link resolves to nil", func(t *testing.T) {
		diag := &artifact.DiagramFrontmatter{Base: artifact.Base{ID: "diagram/from-scratch"}}
		spec, err := resolveDiagramSpec(root, diag)
		if err != nil {
			t.Fatalf("resolveDiagramSpec: %v", err)
		}
		if spec != nil {
			t.Fatalf("resolveDiagramSpec = %+v, want nil (no owning spec)", spec)
		}
	})
}

// TestScannedIDs proves scannedIDs lists both constraints and decisions,
// qualified by owning spec ref, sorted.
func TestScannedIDs(t *testing.T) {
	spec := &artifact.SpecFrontmatter{
		Base:        artifact.Base{ID: "spec/foo"},
		Constraints: []artifact.Constraint{{ID: "co-1"}},
		Decisions:   []artifact.Decision{{ID: "dc-1"}},
	}
	ids := diagramSweepContext{Spec: spec}.scannedIDs()
	want := []string{"spec/foo#co-1", "spec/foo#dc-1"}
	if len(ids) != 2 || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("scannedIDs = %v, want %v (sorted)", ids, want)
	}
}

// TestScannedIDs_NilSpec proves a nil Spec yields no scanned ids rather than
// panicking (the from-scratch/unlinked proposal case).
func TestScannedIDs_NilSpec(t *testing.T) {
	ids := diagramSweepContext{}.scannedIDs()
	if len(ids) != 0 {
		t.Fatalf("scannedIDs = %v, want empty for a nil Spec", ids)
	}
}

// fakeDiagramJudgePreambleScript is the diagram-sweep analogue of
// judge_test.go's fakeJudgePreambleScript / decision_judge_test.go's
// fakeDecisionJudgePreambleScript: a natural-language preamble precedes the
// findings object. Proves the shared prose-tolerant inner-parse (innerparse.go)
// covers the diagram-sweep site too.
const fakeDiagramJudgePreambleScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"Reviewing the proposed diagram against the corpus:\n\n{\"findings\":[{\"id\":\"dsj-pre\",\"text\":\"new sync edge collides with the outbox mandate\",\"confidence\":0.72,\"target\":\"adr/outbox-mandate\"}]}"}
EOF
`

// TestRunDiagramSweep_PreambleTolerated is the diagram-sweep red-first
// reproduction. Pre-fix, decodeDiagramInnerResult fails on the preamble and
// RunDiagramSweep degrades to the synthetic DiagramAbsenceFindingID; post-fix
// the buried object parses into the judged finding.
func TestRunDiagramSweep_PreambleTolerated(t *testing.T) {
	script := writeFakeJudge(t, fakeDiagramJudgePreambleScript)
	res, err := RunDiagramSweep(context.Background(), ExecJudgeRunner{}, DiagramJudgedInput{
		DiagramRef: "diagram/loansvc-future",
		Body:       []byte("graph TD\n  a --> b\n"),
		JudgeCmd:   []string{script},
		Timeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("RunDiagramSweep: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", res.Findings)
	}
	f := res.Findings[0]
	if f.ID != "judged-dsj-pre" {
		t.Fatalf("ID = %q, want judged-dsj-pre (the judged finding, not the synthetic absence finding — a preamble must not degrade the sweep)", f.ID)
	}
	if f.TargetRef != "adr/outbox-mandate" {
		t.Fatalf("TargetRef = %q, want adr/outbox-mandate", f.TargetRef)
	}
}
