package initwizard

import (
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// scriptedInterview runs RunInterview against a fixed answer script (one
// line per prompt, newline-joined) — the stdin-script harness this
// story's spec discloses, exercised here at the package level (the
// built-binary twin lives in cmd/verdi/init_test.go, driving the real
// compiled binary with the same shaped scripts over a real OS pipe).
func scriptedInterview(t *testing.T, script string) (InterviewResult, string, error) {
	t.Helper()
	var out strings.Builder
	result, err := RunInterview(strings.NewReader(script), &out)
	return result, out.String(), err
}

// allDefaultsScript answers every one of RunInterview's prompts with the
// bare-minimum script that reaches the end: a blank line for each of the
// 3+4+2=9 vocabulary-rename prompts (RenameableIDs: 3 classes, 4 states,
// 2 verbs), "n" for the template-copy offer, "n" for the structural-
// request probe, and "y" to confirm the write.
func allDefaultsScript() string {
	return strings.Repeat("\n", 9) + "n\nn\ny\n"
}

// TestRunInterview_AllDefaults_EmptyVocabulary proves the zero-divergence
// path: every prompt skipped produces an empty Vocabulary and
// CopyTemplates false — the property that makes "a wizard run with every
// answer defaulted writes the same store as bare init" true.
func TestRunInterview_AllDefaults_EmptyVocabulary(t *testing.T) {
	result, _, err := scriptedInterview(t, allDefaultsScript())
	if err != nil {
		t.Fatalf("RunInterview(all defaults) = %v, want nil error", err)
	}
	if !VocabularyEmpty(result.Vocabulary) {
		t.Fatalf("RunInterview(all defaults).Vocabulary = %+v, want empty", result.Vocabulary)
	}
	if result.CopyTemplates {
		t.Fatal("RunInterview(all defaults).CopyTemplates = true, want false")
	}
}

// TestRunInterview_RenamesAndTemplateCopy proves real answers land in the
// result exactly, and the template-copy choice is threaded through.
func TestRunInterview_RenamesAndTemplateCopy(t *testing.T) {
	// Order: classes [feature, spike, story], states [accepted-pending-build,
	// closed, draft, superseded], verbs [accept, close].
	script := "Epic\n\nTask\n" + // feature->Epic, spike default, story->Task
		"\n\n\n\n" + // all four states default
		"Sign off\n\n" + // accept->"Sign off", close default
		"y\n" + // copy templates: yes
		"n\n" + // structural probe: no
		"y\n" // confirm write

	result, _, err := scriptedInterview(t, script)
	if err != nil {
		t.Fatalf("RunInterview(renames) = %v, want nil error", err)
	}
	wantClasses := map[string]string{"feature": "Epic", "story": "Task"}
	if len(result.Vocabulary.Classes) != len(wantClasses) {
		t.Fatalf("Vocabulary.Classes = %+v, want %+v", result.Vocabulary.Classes, wantClasses)
	}
	for k, v := range wantClasses {
		if result.Vocabulary.Classes[k] != v {
			t.Fatalf("Vocabulary.Classes[%q] = %q, want %q", k, result.Vocabulary.Classes[k], v)
		}
	}
	if len(result.Vocabulary.States) != 0 {
		t.Fatalf("Vocabulary.States = %+v, want empty (every state prompt defaulted)", result.Vocabulary.States)
	}
	if got := result.Vocabulary.Verbs["accept"]; got != "Sign off" {
		t.Fatalf("Vocabulary.Verbs[accept] = %q, want %q", got, "Sign off")
	}
	if _, ok := result.Vocabulary.Verbs["close"]; ok {
		t.Fatalf("Vocabulary.Verbs[close] should be absent (defaulted), got %+v", result.Vocabulary.Verbs)
	}
	if !result.CopyTemplates {
		t.Fatal("CopyTemplates = false, want true (answered y)")
	}
}

// TestRunInterview_StructuralRequest_RefusedButContinues proves the
// frontier-refusal-then-continue pin (spec/init-wizard ac-2): answering
// "y" to the structural-request probe must not abort the interview — the
// output must name the frontier, and the run must still reach its final
// confirmation and succeed.
func TestRunInterview_StructuralRequest_RefusedButContinues(t *testing.T) {
	script := strings.Repeat("\n", 9) + "n\n" + "y\n" /* structural: yes */ + "y\n" /* confirm write */

	result, out, err := scriptedInterview(t, script)
	if err != nil {
		t.Fatalf("RunInterview(structural request) = %v, want nil error (the interview must continue, not abort)", err)
	}
	if !strings.Contains(out, "frontier") {
		t.Fatalf("output does not name the frontier when a structural request is made:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "unlocks per-verb later") {
		t.Fatalf("output does not carry the design doc's own frontier phrase 'unlocks per-verb later':\n%s", out)
	}
	if VocabularyEmpty(result.Vocabulary) && result.CopyTemplates {
		// no-op guard just to use result; the real proof is err == nil above
		// (the interview reached and returned from its final step).
		_ = result
	}
}

// TestRunInterview_DeclinedWrite proves an explicit "n" at the final
// confirmation is distinguished from a mid-interview abort — both must
// leave the caller with instructions to write NOTHING, but they are
// different conditions (a clean decline vs. an unexpectedly-ended
// script) and get distinct sentinel errors so cmd/verdi/init.go can
// disclose which one happened.
func TestRunInterview_DeclinedWrite(t *testing.T) {
	script := allDefaultsScript()
	script = strings.TrimSuffix(script, "y\n") + "n\n" // decline the final confirm instead

	_, _, err := scriptedInterview(t, script)
	if !errors.Is(err, ErrDeclinedWrite) {
		t.Fatalf("RunInterview(declined) = %v, want ErrDeclinedWrite", err)
	}
	if errors.Is(err, ErrAborted) {
		t.Fatal("a clean decline must not also satisfy errors.Is(err, ErrAborted) — they are distinct conditions")
	}
}

// TestRunInterview_Aborted_TruncatedAtEveryPoint proves stdin ending
// before every prompt is answered is reported as ErrAborted, no matter
// which prompt it happens at (the mid-interview-abort pin, ac-3) — a
// truncation after 0, 1, 5, or all 9 rename prompts, and a truncation
// during the trailing y/n prompts, must all abort rather than silently
// treating EOF as a default answer.
func TestRunInterview_Aborted_TruncatedAtEveryPoint(t *testing.T) {
	full := allDefaultsScript()
	lines := strings.Split(strings.TrimSuffix(full, "\n"), "\n")
	for cut := 0; cut < len(lines); cut++ {
		truncated := strings.Join(lines[:cut], "\n")
		if cut > 0 {
			truncated += "\n"
		}
		t.Run("cutAfterLine", func(t *testing.T) {
			_, _, err := scriptedInterview(t, truncated)
			if !errors.Is(err, ErrAborted) {
				t.Fatalf("truncated after %d line(s) (script %q): RunInterview = %v, want ErrAborted", cut, truncated, err)
			}
		})
	}
}

// TestRunInterview_LiveValidationPreview proves each vocabulary answer
// is followed by a validation preview computed against the SAME
// mechanism verdi model check itself uses (model.DecodeModel over the
// hand-rendered candidate) — asserted here by checking the preview
// output changes as soon as a real rename is entered, never a static
// "ok" printed unconditionally before any answer is read.
func TestRunInterview_LiveValidationPreview(t *testing.T) {
	scriptRenamed := "Epic\n" + strings.Repeat("\n", 8) + "n\nn\ny\n"
	_, outRenamed, err := scriptedInterview(t, scriptRenamed)
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}

	_, outDefault, err := scriptedInterview(t, allDefaultsScript())
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}

	wantDigest, derr := CandidateModel(model.Vocabulary{Classes: map[string]string{"feature": "Epic"}}).Digest()
	if derr != nil {
		t.Fatalf("computing want digest: %v", derr)
	}
	if !strings.Contains(outRenamed, wantDigest) {
		t.Fatalf("output after a real rename does not contain the live-validated candidate's own digest %q:\n%s", wantDigest, outRenamed)
	}
	if strings.Contains(outDefault, wantDigest) {
		t.Fatalf("the all-defaults run's output should never contain the renamed candidate's digest")
	}
}
