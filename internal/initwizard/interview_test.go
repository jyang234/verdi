package initwizard

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// scriptedInterview runs RunInterview against a fixed answer script (one
// line per prompt, newline-joined) — the stdin-script harness this
// story's spec discloses, exercised here at the package level (the
// built-binary twin lives in cmd/verdi/init_test.go, driving the real
// compiled binary with the same shaped scripts over a real OS pipe).
// seed is passed straight through to RunInterview (R-W4-6); every test
// written before the seed parameter existed passes model.Vocabulary{},
// reproducing its own prior, unseeded behavior exactly.
func scriptedInterview(t *testing.T, script string, seed model.Vocabulary) (InterviewResult, string, error) {
	t.Helper()
	var out strings.Builder
	result, err := RunInterview(strings.NewReader(script), &out, seed)
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
	result, _, err := scriptedInterview(t, allDefaultsScript(), model.Vocabulary{})
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
	// closed, draft, superseded], verbs [close, merge].
	script := "Epic\n\nTask\n" + // feature->Epic, spike default, story->Task
		"\n\n\n\n" + // all four states default
		"\nSign off\n" + // close default, merge->"Sign off"
		"y\n" + // copy templates: yes
		"n\n" + // structural probe: no
		"y\n" // confirm write

	result, _, err := scriptedInterview(t, script, model.Vocabulary{})
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
	if got := result.Vocabulary.Verbs["merge"]; got != "Sign off" {
		t.Fatalf("Vocabulary.Verbs[merge] = %q, want %q", got, "Sign off")
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

	result, out, err := scriptedInterview(t, script, model.Vocabulary{})
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

	_, _, err := scriptedInterview(t, script, model.Vocabulary{})
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
			_, _, err := scriptedInterview(t, truncated, model.Vocabulary{})
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
	_, outRenamed, err := scriptedInterview(t, scriptRenamed, model.Vocabulary{})
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}

	_, outDefault, err := scriptedInterview(t, allDefaultsScript(), model.Vocabulary{})
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

// TestRunInterview_SeedIsTheDefault is R-W4-6's own behavioral pin:
// RunInterview's seed model.Vocabulary parameter supplies each rename
// prompt's Enter-default (the seed's own value for that id when present,
// else the id unchanged — today's fallback, now just the zero-seed special
// case), and the interview's own result STARTS as a deep copy of the seed
// rather than the zero value, so an untouched (Enter-only) prompt leaves
// the seeded entry in place rather than silently dropping it.
func TestRunInterview_SeedIsTheDefault(t *testing.T) {
	seed := PlainPreset()

	t.Run("all defaults reproduces the seed exactly", func(t *testing.T) {
		result, out, err := scriptedInterview(t, allDefaultsScript(), seed)
		if err != nil {
			t.Fatalf("RunInterview(seeded, all defaults) = %v, want nil error", err)
		}
		if !reflect.DeepEqual(result.Vocabulary, seed) {
			t.Fatalf("result.Vocabulary = %+v, want the seed %+v unchanged", result.Vocabulary, seed)
		}
		if !strings.Contains(out, `class "story" [Enter to keep "planned story"]`) {
			t.Fatalf("transcript does not show the seed's own value as the story prompt's default:\n%s", out)
		}
		if !strings.Contains(out, `class "feature" [Enter to keep "feature"]`) {
			t.Fatalf("transcript does not fall back to the id for an id the seed does not carry:\n%s", out)
		}
	})

	t.Run("an explicit answer overrides the seeded default", func(t *testing.T) {
		// Order: classes [feature, spike, story]. Blank, blank, "Task": leave
		// feature absent and spike at its seeded default, rename story.
		script := "\n\nTask\n" + strings.Repeat("\n", 6) + "n\nn\ny\n"
		result, _, err := scriptedInterview(t, script, seed)
		if err != nil {
			t.Fatalf("RunInterview(seeded, story renamed) = %v, want nil error", err)
		}
		want := map[string]string{"story": "Task", "spike": "research spike"}
		if !reflect.DeepEqual(result.Vocabulary.Classes, want) {
			t.Fatalf("result.Vocabulary.Classes = %+v, want %+v", result.Vocabulary.Classes, want)
		}
	})

	t.Run("an empty seed reproduces today's transcript byte-for-byte", func(t *testing.T) {
		_, out, err := scriptedInterview(t, allDefaultsScript(), model.Vocabulary{})
		if err != nil {
			t.Fatalf("RunInterview(empty seed, all defaults) = %v, want nil error", err)
		}
		want := `verdi init --wizard — configuring a store in this directory.
Every answer is written to editable config; nothing here is final.

Vocabulary — class display words (Enter to keep the id unchanged):
  class "feature" [Enter to keep "feature"]:   class "spike" [Enter to keep "spike"]:   class "story" [Enter to keep "story"]: Vocabulary — state display words (Enter to keep the id unchanged):
  state "accepted-pending-build" [Enter to keep "accepted-pending-build"]:   state "closed" [Enter to keep "closed"]:   state "draft" [Enter to keep "draft"]:   state "superseded" [Enter to keep "superseded"]: Vocabulary — verb display words (Enter to keep the id unchanged):
  verb "close" [Enter to keep "close"]:   verb "merge" [Enter to keep "merge"]: Copy the canonical templates into .verdi/templates/ for local customization? [y/N]: Add, remove, or restructure the class hierarchy, lifecycle states, or per-transition obligations? [y/N]: 
Summary:
  vocabulary: unchanged (every rename left at its default)
  templates: unchanged (no local override copies)

Write .verdi/ ? [Y/n]: `
		if out != want {
			t.Fatalf("empty-seed transcript drifted from the pre-seed-parameter baseline:\n--- got ---\n%s\n--- want ---\n%s", out, want)
		}
	})
}
