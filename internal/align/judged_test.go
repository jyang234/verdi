package align

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// guide-claim: 11-configurable-alignment-judge
func TestRunJudged_NotConfigured(t *testing.T) {
	t.Run("not required: absence finding", func(t *testing.T) {
		result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{})
		if err != nil {
			t.Fatalf("RunJudged: %v", err)
		}
		if len(result.Findings) != 1 || result.Findings[0].ID != AbsenceFindingID {
			t.Fatalf("Findings = %+v, want one absence finding", result.Findings)
		}
		if result.Findings[0].Dispositioned() {
			t.Fatalf("absence finding must be undispositioned fresh")
		}
		if result.Integrity != "" || result.JudgeIntegrity != nil {
			t.Fatalf("not-configured path must carry no Integrity/JudgeIntegrity, got %+v", result)
		}
	})

	t.Run("required: align fails outright", func(t *testing.T) {
		_, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeRequired: true})
		if err == nil {
			t.Fatal("RunJudged(judge_required=true, no judge_cmd): want error, got nil")
		}
	})
}

func TestRunJudged_ExecutionFailure(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeNonZeroExitScript)

	t.Run("not required: absence finding names the failure", func(t *testing.T) {
		result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeCmd: []string{script}, Timeout: time.Second})
		if err != nil {
			t.Fatalf("RunJudged: %v", err)
		}
		if len(result.Findings) != 1 || result.Findings[0].ID != AbsenceFindingID {
			t.Fatalf("Findings = %+v, want one absence finding", result.Findings)
		}
		if !strings.Contains(result.Findings[0].Text, "exit=3") {
			t.Fatalf("absence finding text = %q, want it to name the exit code", result.Findings[0].Text)
		}
	})

	t.Run("required: align fails outright", func(t *testing.T) {
		_, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeCmd: []string{script}, JudgeRequired: true, Timeout: time.Second})
		if err == nil {
			t.Fatal("RunJudged(judge_required=true, failing judge): want error, got nil")
		}
	})
}

func TestRunJudged_Success(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeOKScript)
	result, err := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{JudgeCmd: []string{script}, Timeout: time.Second, Prompt: []byte("prompt")})
	if err != nil {
		t.Fatalf("RunJudged: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].ID != "judged-j-1" {
		t.Fatalf("Findings = %+v, want one judged-j-1", result.Findings)
	}
	if result.Integrity == "" {
		t.Fatal("Integrity must be set on a successful judge exchange")
	}
	if result.JudgeIntegrity == nil || result.JudgeIntegrity.RawResult == "" || result.JudgeIntegrity.StdinB64 == "" {
		t.Fatalf("JudgeIntegrity = %+v, want both fields populated", result.JudgeIntegrity)
	}
	stdin, err := base64.StdEncoding.DecodeString(result.JudgeIntegrity.StdinB64)
	if err != nil {
		t.Fatalf("decoding StdinB64: %v", err)
	}
	if got := computeIntegrity(stdin, result.JudgeIntegrity.RawResult); got != result.Integrity {
		t.Fatalf("recomputed integrity %q != stored %q", got, result.Integrity)
	}
}

func TestRunJudged_DeterministicAbsenceText(t *testing.T) {
	// Same not-configured input twice -> byte-identical absence finding
	// text, the property identity.go's disposition preservation across
	// regeneration depends on for the no-judge path.
	r1, _ := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{})
	r2, _ := RunJudged(context.Background(), ExecJudgeRunner{}, JudgedInput{})
	if r1.Findings[0].Text != r2.Findings[0].Text {
		t.Fatalf("absence text not deterministic: %q vs %q", r1.Findings[0].Text, r2.Findings[0].Text)
	}
}
