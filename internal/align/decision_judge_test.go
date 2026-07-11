package align

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OWNER/verdi/internal/artifact"
)

const fakeDecisionJudgeOKScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"dj-1\",\"text\":\"dc-1 may contradict adr/retry-policy\",\"confidence\":0.6,\"target\":\"adr/retry-policy\"}]}"}
EOF
`

func TestRunDecisionSweep_Success(t *testing.T) {
	script := writeFakeJudge(t, fakeDecisionJudgeOKScript)
	res, err := RunDecisionSweep(context.Background(), ExecJudgeRunner{}, DecisionJudgedInput{
		JudgeCmd: []string{script},
		Timeout:  5 * time.Second,
		Prompt:   []byte("prompt"),
	})
	if err != nil {
		t.Fatalf("RunDecisionSweep: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", res.Findings)
	}
	f := res.Findings[0]
	if f.Kind != artifact.FindingJudged {
		t.Fatalf("Kind = %q, want judged", f.Kind)
	}
	if f.TargetRef != "adr/retry-policy" {
		t.Fatalf("TargetRef = %q, want adr/retry-policy", f.TargetRef)
	}
	if res.Integrity == "" || res.JudgeIntegrity == nil {
		t.Fatal("Integrity/JudgeIntegrity must be populated on a real judge exchange")
	}
}

func TestRunDecisionSweep_NotConfiguredDegradesToAbsence(t *testing.T) {
	res, err := RunDecisionSweep(context.Background(), ExecJudgeRunner{}, DecisionJudgedInput{})
	if err != nil {
		t.Fatalf("RunDecisionSweep: %v", err)
	}
	if len(res.Findings) != 1 || res.Findings[0].ID != DecisionAbsenceFindingID {
		t.Fatalf("Findings = %+v, want exactly one %s", res.Findings, DecisionAbsenceFindingID)
	}
	if res.Findings[0].Dispositioned() {
		t.Fatal("synthetic absence finding must start undispositioned")
	}
	if res.Integrity != "" || res.JudgeIntegrity != nil {
		t.Fatal("absence result must carry no integrity record")
	}
}

func TestRunDecisionSweep_JudgeRequiredAndAbsent(t *testing.T) {
	_, err := RunDecisionSweep(context.Background(), ExecJudgeRunner{}, DecisionJudgedInput{JudgeRequired: true})
	if err == nil {
		t.Fatal("RunDecisionSweep: want error when judge_required and no judge configured")
	}
	var reqAbsent *ErrDecisionJudgeRequiredAbsent
	if !errors.As(err, &reqAbsent) {
		t.Fatalf("err = %v, want *ErrDecisionJudgeRequiredAbsent", err)
	}
}

func TestBuildDecisionSweepPrompt_Deterministic(t *testing.T) {
	spec := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/foo"}, Decisions: []artifact.Decision{{ID: "dc-1", Text: "t"}}}
	ctx := decisionSweepContext{Spec: spec}
	p1 := BuildDecisionSweepPrompt(ctx)
	p2 := BuildDecisionSweepPrompt(ctx)
	if string(p1) != string(p2) {
		t.Fatal("BuildDecisionSweepPrompt is not deterministic over identical inputs")
	}
	if len(p1) == 0 {
		t.Fatal("BuildDecisionSweepPrompt: empty prompt")
	}
}

func TestBuildDecisionSweepPrompt_IncludesFeatureDecisionsForStory(t *testing.T) {
	story := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/my-story"}, Class: artifact.ClassStory}
	feature := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/my-feature"}, Decisions: []artifact.Decision{{ID: "dc-9", Text: "feature decision text"}}}
	prompt := string(BuildDecisionSweepPrompt(decisionSweepContext{Spec: story, FeatureSpec: feature}))
	if !strings.Contains(prompt, "feature decision text") {
		t.Fatalf("prompt = %q, want it to include the parent feature's decision text", prompt)
	}
}

func TestScannedDecisionIDs(t *testing.T) {
	story := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/story"}, Decisions: []artifact.Decision{{ID: "dc-1"}}}
	feature := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/feature"}, Decisions: []artifact.Decision{{ID: "dc-9"}}}
	ids := decisionSweepContext{Spec: story, FeatureSpec: feature}.scannedDecisionIDs()
	want := []string{"spec/feature#dc-9", "spec/story#dc-1"}
	if len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] {
		t.Fatalf("scannedDecisionIDs = %v, want %v (sorted)", ids, want)
	}
}
