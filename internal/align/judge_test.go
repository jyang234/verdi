package align

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeFakeJudge writes a tiny shell script honoring S5's `claude -p
// --output-format json` envelope shape (or one of its failure modes) and
// returns its path. A real script executed via os/exec — not a mocked
// interface — so ExecJudgeRunner itself (the production exec path) is what
// gets exercised; only the far end (the "claude" binary) is fake, matching
// PLAN.md Phase 8's "fake judge binary in tests keeps the phase hermetic".
func writeFakeJudge(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fakejudge.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("writing fake judge script: %v", err)
	}
	return path
}

const fakeJudgeOKScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-1\",\"text\":\"retry semantics match spec intent\",\"confidence\":0.87}]}"}
EOF
`

// fakeJudgeOKFencedScript wraps the result string's JSON in a markdown code
// fence — S5: "trim/strip fences defensively".
const fakeJudgeFencedScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"` + "```json\\n{\\\"findings\\\":[{\\\"id\\\":\\\"j-2\\\",\\\"text\\\":\\\"fenced ok\\\",\\\"confidence\\\":0.5}]}\\n```" + `"}
EOF
`

const fakeJudgeNonZeroExitScript = `echo "simulated crash" >&2
exit 3
`

const fakeJudgeGarbageStdoutScript = `echo "not json at all"
`

const fakeJudgeIsErrorScript = `cat <<'EOF'
{"is_error":true,"subtype":"error_during_execution","result":""}
EOF
`

const fakeJudgeInnerGarbageScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"this is not findings json"}
EOF
`

const fakeJudgeTimeoutScript = `sleep 5
echo "should never get here"
`

// fakeJudgeAlreadyPrefixedIDScript emits a finding whose raw id already
// carries a "judged-" prefix — the shape a regeneration/carry path
// (spec/finding-identity) can feed back to the judge as context (e.g. a
// prior finding's own already-minted id) and have the judge echo back
// verbatim. Reproduces spec/ritual-traps ac-2's exact defect: minting
// unconditionally re-prefixed this into "judged-judged-...".
const fakeJudgeAlreadyPrefixedIDScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"judged-retry-semantics-drift\",\"text\":\"retry semantics match spec intent\",\"confidence\":0.87}]}"}
EOF
`

// fakeJudgeNewlineTextScript emits a finding whose text carries an embedded
// newline (ADJ-53's j-4 fixture): a judge is free-text, nothing in S5's own
// contract constrains it to a single line, so this is a legitimate — if
// rare — judge response shape the ingestion layer must handle safely
// rather than pass through raw. The literal `\\n` here is JSON's own
// newline escape at the OUTER envelope's re-escaping layer (S5's two-layer
// parse: this decodes, at the inner layer, to a `text` value containing a
// real newline character, exactly as a real judge emitting a
// multi-paragraph answer would produce).
const fakeJudgeNewlineTextScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-newline\",\"text\":\"line one\\nline two\",\"confidence\":0.4}]}"}
EOF
`

// fakeJudgeEchoedConfidenceSuffixScript emits a finding whose text ALREADY
// ends with one " (confidence N.NN)" suffix — the shape a regeneration/carry
// path (spec/finding-identity) re-presents to the judge (a prior report's own
// already-decorated text) and the judge echoes back verbatim. The echoed
// suffix's value (0.30) is deliberately DIFFERENT from this run's confidence
// field (0.87) so the test can prove the retained suffix reflects the CURRENT
// run, not the stale echo. Reproduces spec/ritual-traps finding
// judged-ac2-confidence-suffix-doubling-survives-in-finding-text (the text-half
// sibling of the "judged-judged-" id defect): minting unconditionally
// re-appends, producing a doubled "... (confidence 0.30) (confidence 0.87)".
const fakeJudgeEchoedConfidenceSuffixScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-echo\",\"text\":\"retry semantics match spec intent (confidence 0.30)\",\"confidence\":0.87}]}"}
EOF
`

// fakeJudgeDoubledConfidenceSuffixScript emits a finding whose text already
// carries TWO stacked suffixes — an already-doubled archived echo (the shape
// this build's own prior deviation-report.md witnessed) — which the
// prospective fix must collapse to exactly one fresh suffix.
const fakeJudgeDoubledConfidenceSuffixScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-doubled\",\"text\":\"retry semantics match spec intent (confidence 0.30) (confidence 0.30)\",\"confidence\":0.45}]}"}
EOF
`

// judgeTestBudget bounds every runJudgeOnce call in this file that is NOT
// itself testing the timeout stage (TestRunJudgeOnce_Timeout, below, keeps
// its own short, deliberately tight timeout — that IS the behavior under
// test there).
//
// This is a fix for a proven flake, not a guess: a flat 1-second injected
// timeout on these calls was observed failing StageTimeout under
// full-module `go test -race` load three times across this build (always
// passing standalone). Reproduced on demand by running this package's
// tests concurrently with the rest of the module under -race (see the
// phase report for the captured run) — every failure showed
// Stage:timeout, Detail:"no result within 1s", never a wrong finding or a
// wrong OTHER stage. The fake judge scripts here do near-zero real work
// (a `cat <<EOF` heredoc or an `echo`); the time that blows through 1s
// under load is the OS scheduling this test's own os/exec fork+exec of
// /bin/sh, competing with the rest of the module's `-race`-instrumented
// goroutines for CPU — not the judge logic being slow. Sizing the budget
// as a `context.WithTimeout` deadline handed to runJudgeOnce as ctx (with
// the `timeout` parameter itself left at its zero-value default, so
// runJudgeOnce's OWN timeout-selection logic — "timeout<=0 uses
// DefaultJudgeTimeout" — stays exercised exactly as production calls it)
// is deadline-from-context, not a blind widening of the injected
// duration: the deadline these tests actually race against is expressed
// once, here, reasoned about explicitly, and stays two orders of
// magnitude below DefaultJudgeTimeout's 120s ceiling — a genuine
// timeout-logic regression (e.g. the real timeout silently not firing at
// all) still fails this test suite within judgeTestBudget, not by hanging
// for the full 120s or forever.
const judgeTestBudget = 10 * time.Second

// judgeTestContext returns a context bounded by judgeTestBudget, cleaned
// up automatically at the end of the calling test.
func judgeTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), judgeTestBudget)
	t.Cleanup(cancel)
	return ctx
}

func TestRunJudgeOnce_Success(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeOKScript)
	success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("prompt bytes"))
	if failure != nil {
		t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
	}
	if len(success.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", success.Findings)
	}
	f := success.Findings[0]
	if f.ID != "judged-j-1" || f.Kind != "judged" {
		t.Fatalf("finding = %+v, want id judged-j-1 kind judged", f)
	}
	if !strings.Contains(f.Text, "retry semantics match spec intent") || !strings.Contains(f.Text, "0.87") {
		t.Fatalf("finding text = %q, want it to carry the judge's text and confidence", f.Text)
	}
	if string(success.Stdin) != "prompt bytes" {
		t.Fatalf("Stdin = %q, want the exact prompt bytes", success.Stdin)
	}
	if !strings.Contains(success.RawResult, "j-1") {
		t.Fatalf("RawResult = %q, want the raw result string preserved", success.RawResult)
	}
}

// TestRunJudgeOnce_AlreadyPrefixedRawID_NeverDoubles is spec/ritual-traps
// ac-2's genuine regression reproduction: when the judge's raw finding id
// already carries a "judged-" prefix (fakeJudgeAlreadyPrefixedIDScript,
// modeling a regeneration path that fed a prior finding's own id back to
// the judge as context and got it echoed back verbatim), the minted
// Finding.ID must carry exactly ONE "judged-" prefix — never
// "judged-judged-...". Pre-fix, this test fails with
// ID == "judged-judged-retry-semantics-drift" (the doubled-prefix defect
// itself), proving this is a real reproduction, not a vacuous assertion.
func TestRunJudgeOnce_AlreadyPrefixedRawID_NeverDoubles(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeAlreadyPrefixedIDScript)
	success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("prompt bytes"))
	if failure != nil {
		t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
	}
	if len(success.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", success.Findings)
	}
	got := success.Findings[0].ID
	if got != "judged-retry-semantics-drift" {
		t.Fatalf("Findings[0].ID = %q, want exactly one judged- prefix (judged-retry-semantics-drift)", got)
	}
	if n := strings.Count(got, "judged-"); n != 1 {
		t.Fatalf("Findings[0].ID = %q carries %d occurrences of \"judged-\", want exactly 1", got, n)
	}
}

// TestJudgedFindingID unit-tests the shared minting helper directly:
// ordinary raw ids get prefixed exactly once, an already-prefixed raw id
// (any case) is used as its own slug rather than re-prefixed, and the slug
// transform (store.RefSlug) still applies underneath either way.
func TestJudgedFindingID(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"j-1", "judged-j-1"},
		{"judged-j-1", "judged-j-1"},
		{"Judged-Foo", "judged-foo"},
		{"retry semantics drift", "judged-retry-semantics-drift"},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			if got := judgedFindingID(tc.raw); got != tc.want {
				t.Fatalf("judgedFindingID(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestJudgedFindingID_EchoedDoubledID_PreservedVerbatim pins the adjudication
// recorded on judgedFindingID's doc comment (spec/ritual-traps ac-2, finding
// judged-ac2-echoed-doubled-id-still-doubles): an echoed, ALREADY-DOUBLED
// archived id ("judged-judged-x") is preserved VERBATIM — never normalized to
// "judged-x". Normalizing would sever the id-join to any archived disposition
// recorded against the doubled form (dispositions reference ids exactly as
// originally minted — the W4 precedent), orphaning it. This is a ratified
// behavior pin, not a change: the HasPrefix short-circuit already preserves
// it. Its value is guarding against a future "helpful" normalization that
// would silently renumber an archived id.
func TestJudgedFindingID_EchoedDoubledID_PreservedVerbatim(t *testing.T) {
	const doubled = "judged-judged-x"
	if got := judgedFindingID(doubled); got != doubled {
		t.Fatalf("judgedFindingID(%q) = %q, want it preserved verbatim (%q) — an echoed archived doubled id must never be normalized (e.g. to \"judged-x\"), which would orphan archived dispositions that reference the doubled id exactly as minted", doubled, got, doubled)
	}
}

// TestRunJudgeOnce_NewlineInTextIsNormalized is ADJ-53's j-4 fix proof: a
// judge-emitted finding text carrying an embedded newline must never
// survive into Finding.Text raw — every downstream consumer
// (align.RenderFindingLine's single-line bullet, the disposition verb's
// whole-line matcher, Identity's content-hash finding equality) assumes
// one line, and this pre-existing latent gap is exactly what the
// disposition verb converted into a permanent brick (judged-j-4). The
// RawResult (the persisted judge-integrity input) must still carry the
// judge's own raw, UNnormalized text verbatim — normalization is a
// Finding.Text presentation concern, never a tamper with the integrity
// hash's own input bytes.
func TestRunJudgeOnce_NewlineInTextIsNormalized(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeNewlineTextScript)
	success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("prompt"))
	if failure != nil {
		t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
	}
	if len(success.Findings) != 1 {
		t.Fatalf("Findings = %+v, want 1", success.Findings)
	}
	text := success.Findings[0].Text
	if strings.ContainsAny(text, "\n\r") {
		t.Fatalf("finding text = %q, contains a raw newline/CR — must be normalized to a single line", text)
	}
	if !strings.Contains(text, "line one") || !strings.Contains(text, "line two") {
		t.Fatalf("finding text = %q, want it to still carry both halves of the judge's text", text)
	}
	// RawResult is JSON SOURCE TEXT (one decode layer short of
	// judgeInnerFinding.Text) — its embedded newline is still the two-byte
	// `\n` JSON escape sequence at this layer, never an actual newline
	// byte, so the raw (backtick) string below is deliberate, not a typo.
	if !strings.Contains(success.RawResult, `line one\nline two`) {
		t.Fatalf("RawResult = %q, want the judge's raw, UNnormalized text preserved verbatim (integrity hash input)", success.RawResult)
	}
}

// TestRunJudgeOnce_EchoedConfidenceSuffix_NeverDoubles is spec/ritual-traps
// finding judged-ac2-confidence-suffix-doubling-survives-in-finding-text's
// genuine regression reproduction — the text-half sibling of
// TestRunJudgeOnce_AlreadyPrefixedRawID_NeverDoubles. When the judge's text
// already carries the " (confidence N.NN)" suffix runJudgeOnce itself mints (a
// prior report's decorated text echoed back on a carry path), the minted
// Finding.Text must carry EXACTLY ONE suffix, reflecting THIS run's confidence.
// Pre-fix this fails with a doubled "... (confidence 0.30) (confidence 0.87)",
// proving a real reproduction rather than a vacuous assertion. A separately-fed
// ALREADY-DOUBLED echo must collapse to one. RawResult (the persisted
// judge-integrity input) must still carry the judge's raw text verbatim —
// idempotence is a Finding.Text presentation concern, never a tamper with the
// integrity hash's own bytes (mirrors TestRunJudgeOnce_NewlineInTextIsNormalized).
func TestRunJudgeOnce_EchoedConfidenceSuffix_NeverDoubles(t *testing.T) {
	t.Run("one echoed suffix collapses to the current run's single suffix", func(t *testing.T) {
		script := writeFakeJudge(t, fakeJudgeEchoedConfidenceSuffixScript)
		success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("prompt"))
		if failure != nil {
			t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
		}
		if len(success.Findings) != 1 {
			t.Fatalf("Findings = %+v, want 1", success.Findings)
		}
		text := success.Findings[0].Text
		if n := strings.Count(text, "(confidence "); n != 1 {
			t.Fatalf("Finding.Text = %q carries %d confidence suffixes, want exactly 1 (the doubling defect)", text, n)
		}
		if !strings.HasSuffix(text, "(confidence 0.87)") {
			t.Fatalf("Finding.Text = %q, want it to end with THIS run's confidence (confidence 0.87), not the stale echoed 0.30", text)
		}
		if !strings.Contains(text, "retry semantics match spec intent") {
			t.Fatalf("Finding.Text = %q, want the judge's base text preserved", text)
		}
		// RawResult is the integrity input — the judge's raw echoed text
		// (including its stale suffix) must survive verbatim, untouched by the
		// presentation-layer idempotence.
		if !strings.Contains(success.RawResult, `spec intent (confidence 0.30)`) {
			t.Fatalf("RawResult = %q, want the judge's raw echoed text preserved verbatim (integrity input)", success.RawResult)
		}
	})

	t.Run("an already-doubled archived echo collapses to one fresh suffix", func(t *testing.T) {
		script := writeFakeJudge(t, fakeJudgeDoubledConfidenceSuffixScript)
		success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("prompt"))
		if failure != nil {
			t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
		}
		if len(success.Findings) != 1 {
			t.Fatalf("Findings = %+v, want 1", success.Findings)
		}
		text := success.Findings[0].Text
		if n := strings.Count(text, "(confidence "); n != 1 {
			t.Fatalf("Finding.Text = %q carries %d confidence suffixes, want exactly 1 after collapsing a doubled echo", text, n)
		}
		if !strings.HasSuffix(text, "(confidence 0.45)") {
			t.Fatalf("Finding.Text = %q, want it to end with THIS run's single confidence (confidence 0.45)", text)
		}
	})
}

// TestDecorateConfidence unit-tests the text-half idempotence helper directly
// (table-driven, happy + negative paths): bare text gains exactly one suffix;
// an echoed suffix (any value) is stripped and replaced by THIS run's;
// stacked/doubled suffixes collapse to one; a mint-shaped occurrence that is
// NOT at the tail (a witness the finding itself quotes mid-text) is preserved;
// and a tail that only resembles the mint shape (one fractional digit,
// non-numeric) is deliberately left alone.
func TestDecorateConfidence(t *testing.T) {
	cases := []struct {
		name string
		text string
		conf float64
		want string
	}{
		{"bare text gains one suffix", "retry semantics match spec intent", 0.87, "retry semantics match spec intent (confidence 0.87)"},
		{"one echoed suffix replaced by current", "foo (confidence 0.30)", 0.87, "foo (confidence 0.87)"},
		{"doubled echo collapses to one", "foo (confidence 0.30) (confidence 0.30)", 0.45, "foo (confidence 0.45)"},
		{"mixed-value stack collapses to current", "foo (confidence 0.30) (confidence 0.87)", 0.12, "foo (confidence 0.12)"},
		{"integer-part and zero confidence", "foo (confidence 1.00)", 0.0, "foo (confidence 0.00)"},
		{"mid-text mint-shaped witness preserved", "ended with (confidence 0.30) as noted", 0.50, "ended with (confidence 0.30) as noted (confidence 0.50)"},
		{"one-fractional-digit is not the mint shape", "foo (confidence 0.3)", 0.50, "foo (confidence 0.3) (confidence 0.50)"},
		{"non-numeric parenthetical is not stripped", "foo (confidence high)", 0.50, "foo (confidence high) (confidence 0.50)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := decorateConfidence(tc.text, tc.conf); got != tc.want {
				t.Fatalf("decorateConfidence(%q, %.2f) = %q, want %q", tc.text, tc.conf, got, tc.want)
			}
		})
	}
}

func TestRunJudgeOnce_FencedResult(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeFencedScript)
	success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("p"))
	if failure != nil {
		t.Fatalf("runJudgeOnce: unexpected failure %+v", failure)
	}
	if len(success.Findings) != 1 || success.Findings[0].ID != "judged-j-2" {
		t.Fatalf("Findings = %+v, want one judged-j-2", success.Findings)
	}
}

func TestRunJudgeOnce_FailureModes(t *testing.T) {
	cases := []struct {
		name   string
		script string
		stage  JudgeFailureStage
	}{
		{"missing binary", "", StageExec}, // handled specially below
		{"non-zero exit", fakeJudgeNonZeroExitScript, StageExit},
		{"garbage stdout (outer-parse)", fakeJudgeGarbageStdoutScript, StageOuterParse},
		{"is_error true (outer-parse)", fakeJudgeIsErrorScript, StageOuterParse},
		{"inner garbage (inner-parse)", fakeJudgeInnerGarbageScript, StageInnerParse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var argv []string
			if tc.name == "missing binary" {
				argv = []string{filepath.Join(t.TempDir(), "does-not-exist")}
			} else {
				argv = []string{writeFakeJudge(t, tc.script)}
			}
			success, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, argv, 0, []byte("p"))
			if success != nil {
				t.Fatalf("runJudgeOnce(%s): unexpected success %+v", tc.name, success)
			}
			if failure == nil {
				t.Fatalf("runJudgeOnce(%s): want a JudgeFailure, got nil", tc.name)
			}
			if failure.Stage != tc.stage {
				t.Fatalf("runJudgeOnce(%s): Stage = %q, want %q (failure: %+v)", tc.name, failure.Stage, tc.stage, failure)
			}
			if failure.CmdTemplate == "" {
				t.Fatalf("runJudgeOnce(%s): CmdTemplate must be recorded", tc.name)
			}
		})
	}

	t.Run("non-zero exit records exit code and stderr", func(t *testing.T) {
		script := writeFakeJudge(t, fakeJudgeNonZeroExitScript)
		_, failure := runJudgeOnce(judgeTestContext(t), ExecJudgeRunner{}, []string{script}, 0, []byte("p"))
		if failure.ExitCode != 3 {
			t.Fatalf("ExitCode = %d, want 3", failure.ExitCode)
		}
		if !strings.Contains(failure.StderrSnippet, "simulated crash") {
			t.Fatalf("StderrSnippet = %q, want it to carry the child's stderr", failure.StderrSnippet)
		}
	})
}

// TestRunJudgeOnce_Timeout exercises the timeout stage with a short
// injected Timeout, never S5's real ~120s ceiling. Unlike every other test
// in this file, this one is DELIBERATELY tight (100ms) — it is the
// regression detector for the timeout mechanism itself, so it must stay
// sensitive rather than getting the judgeTestBudget treatment above. It is
// not part of the proven flake (judgeTestBudget's doc comment) and does
// not need to be: fakeJudgeTimeoutScript always sleeps 5s regardless of
// scheduling load, an interval no plausible fork/exec scheduling jitter
// approaches, so a 100ms timeout fires deterministically either way.
func TestRunJudgeOnce_Timeout(t *testing.T) {
	script := writeFakeJudge(t, fakeJudgeTimeoutScript)
	start := time.Now()
	success, failure := runJudgeOnce(context.Background(), ExecJudgeRunner{}, []string{script}, 100*time.Millisecond, []byte("p"))
	elapsed := time.Since(start)
	if success != nil {
		t.Fatalf("unexpected success %+v", success)
	}
	if failure == nil || failure.Stage != StageTimeout {
		t.Fatalf("failure = %+v, want stage timeout", failure)
	}
	if elapsed > 4*time.Second {
		t.Fatalf("runJudgeOnce took %s, want it to return promptly after the injected 100ms timeout, not wait for the sleep 5", elapsed)
	}
}
