package align

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// fakeJudgeRewordedScript emits the SAME judge-side id ("j-1", so the
// rendered finding id is still judged-j-1 — store.RefSlug is a no-op for
// this simple input) but different, reworded text and a different
// confidence — the exact "same slug, reworded prose" shape spec/finding-
// identity ac-1 is driven against.
const fakeJudgeRewordedScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-1\",\"text\":\"retry semantics reworded, same underlying issue\",\"confidence\":0.91}]}"}
EOF
`

// fakeJudgeDriftedScript emits a DIFFERENT judge-side id ("j-2") — the fresh
// run simply does not re-emit j-1 at all (a drifting slug, ac-3's case).
const fakeJudgeDriftedScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"j-2\",\"text\":\"a different finding entirely\",\"confidence\":0.6}]}"}
EOF
`

// fakeJudgeCollidingScript emits TWO findings sharing the SAME judge-side id
// ("dup") within one exchange — ac-4's contract-violation case.
const fakeJudgeCollidingScript = `cat <<'EOF'
{"is_error":false,"subtype":"success","result":"{\"findings\":[{\"id\":\"dup\",\"text\":\"first reading\",\"confidence\":0.5},{\"id\":\"dup\",\"text\":\"second reading\",\"confidence\":0.5}]}"}
EOF
`

// firstDispositioned dispositions every finding in a Generate result as
// accepted-deviation, returning the updated slice — the same "a human
// dispositions everything from round 1" fixture-building step
// TestGenerate_PreservesDispositionsAcrossRegeneration already uses for
// FindingFixed.
func firstDispositioned(findings []artifact.Finding, disposition artifact.FindingDisposition, note string) []artifact.Finding {
	out := make([]artifact.Finding, len(findings))
	for i, f := range findings {
		f.Disposition = disposition
		f.Note = note
		out[i] = f
	}
	return out
}

// TestGenerate_JudgedSlugMatch_BecomesCandidate_AllDispositionedFalse is
// spec/finding-identity ac-1's headline case driven through the FULL
// Generate pipeline (not just ReconcileJudged in isolation): round 1's
// judged finding is dispositioned; round 2's judge rewords the same slug's
// text. The regenerated report must present it as a candidate — never
// silently carried, never silently discarded — and AllDispositioned must
// read false until a human confirms it, exactly X-16's discipline for fresh
// findings.
func TestGenerate_JudgedSlugMatch_BecomesCandidate_AllDispositionedFalse(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	first, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}
	dispositioned := firstDispositioned(first.Frontmatter.Findings, artifact.FindingAcceptedDeviation, "owner-ratified: intentional deviation")

	second, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeRewordedScript)}, JudgeTimeout: time.Second,
		ExistingFindings: dispositioned,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate (second): %v", err)
	}

	var judgedFinding *artifact.Finding
	for i := range second.Frontmatter.Findings {
		if second.Frontmatter.Findings[i].ID == "judged-j-1" {
			judgedFinding = &second.Frontmatter.Findings[i]
		}
	}
	if judgedFinding == nil {
		t.Fatalf("no judged-j-1 among %+v", second.Frontmatter.Findings)
	}
	if judgedFinding.Dispositioned() {
		t.Fatalf("judged-j-1 = %+v, want UNDISPOSITIONED (a candidate, never silently carried)", judgedFinding)
	}
	if judgedFinding.Text != "retry semantics reworded, same underlying issue (confidence 0.91)" {
		t.Fatalf("judged-j-1.Text = %q, want the fresh judge's own new text", judgedFinding.Text)
	}
	if artifact.AllDispositioned(second.Frontmatter.Findings) {
		t.Fatal("AllDispositioned() = true, want false — a candidate is not a disposition (ac-1)")
	}

	// The pre-fill context (old ruling + old text beside new text) is
	// rendered in the body for a human to review before confirming.
	if !containsLine(second.Body, "- **judged-j-1** CANDIDATE — new text: \"retry semantics reworded, same underlying issue (confidence 0.91)\"") {
		t.Fatalf("body does not render the candidate pre-fill line; body:\n%s", second.Body)
	}

	// The tally (P2-9's own tooling, chronicle P2-9): one carried candidate
	// awaiting reaffirmation, zero new.
	if second.JudgedTally.Candidates != 1 || second.JudgedTally.New != 0 {
		t.Fatalf("JudgedTally = %+v, want {Candidates:1 New:0}", second.JudgedTally)
	}
}

// TestGenerate_JudgedDriftingSlug_NotResurfaced_TallyStillDry proves ac-3's
// core through the full pipeline: a judge re-roll that does not re-emit a
// prior dispositioned finding's slug at all lands that finding in
// NotResurfaced, not silently dropped, and the new finding (a genuinely
// different slug) counts as New in the tally.
func TestGenerate_JudgedDriftingSlug_NotResurfaced_TallyStillDry(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	first, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}
	dispositioned := firstDispositioned(first.Frontmatter.Findings, artifact.FindingAcceptedDeviation, "n")

	second, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeDriftedScript)}, JudgeTimeout: time.Second,
		ExistingFindings: dispositioned,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate (second): %v", err)
	}

	var notResurfacedJ1 *artifact.Finding
	for i := range second.Frontmatter.NotResurfaced {
		if second.Frontmatter.NotResurfaced[i].ID == "judged-j-1" {
			notResurfacedJ1 = &second.Frontmatter.NotResurfaced[i]
		}
	}
	if notResurfacedJ1 == nil {
		t.Fatalf("NotResurfaced = %+v, want judged-j-1 preserved (drifting slug)", second.Frontmatter.NotResurfaced)
	}
	if notResurfacedJ1.Disposition != artifact.FindingAcceptedDeviation {
		t.Fatalf("NotResurfaced judged-j-1 disposition = %q, want accepted-deviation preserved verbatim", notResurfacedJ1.Disposition)
	}

	for _, f := range second.Frontmatter.Findings {
		if f.ID == "judged-j-1" {
			t.Fatalf("judged-j-1 must not ALSO appear in findings: once it has drifted to not-resurfaced, got %+v", f)
		}
	}

	if second.JudgedTally.New != 1 || second.JudgedTally.Candidates != 0 {
		t.Fatalf("JudgedTally = %+v, want {New:1 Candidates:0} (judged-j-2 is genuinely new)", second.JudgedTally)
	}
}

// TestGenerate_CollidingJudgeSlugs_DisclosedNeverDeduped proves ac-4's
// collision rule through the full pipeline: a single judge exchange that
// emits two findings sharing one id produces both findings verbatim plus a
// disclosed, undispositioned contract-violation finding — never a silent
// dedupe that would hide which of the two a human actually dispositioned.
func TestGenerate_CollidingJudgeSlugs_DisclosedNeverDeduped(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	report, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeCollidingScript)}, JudgeTimeout: time.Second,
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var dupCount, violationCount int
	for _, f := range report.Frontmatter.Findings {
		switch {
		case f.ID == "judged-dup" || f.ID == "judged-dup-collision-2":
			dupCount++
		case strings.HasPrefix(f.ID, "judged-contract-violation-"):
			violationCount++
			if f.Dispositioned() {
				t.Fatalf("contract-violation finding must be undispositioned fresh, got %+v", f)
			}
		}
	}
	if dupCount != 2 {
		t.Fatalf("dupCount = %d, want 2 (both colliding findings kept, never deduped, ids disambiguated)", dupCount)
	}
	if violationCount != 1 {
		t.Fatalf("violationCount = %d, want 1 disclosed contract-violation finding", violationCount)
	}
}

// TestGenerate_ComputedHoldsToViolated_StillVoids is ac-2's explicit
// regression pin, re-driven at the Generate level (identity_test.go already
// pins PreserveDispositions directly): a computed finding's verdict flip
// (holds -> violated) still starts fresh, undispositioned — computed
// findings never route through ReconcileJudged's slug-matching at all.
func TestGenerate_ComputedHoldsToViolated_StillVoids(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	first, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}
	dispositioned := firstDispositioned(first.Frontmatter.Findings, artifact.FindingFixed, "")
	// FindingFixed allows an empty note (artifact.Finding.Validate) — but
	// firstDispositioned's signature always sets Note; clear it back out to
	// match the plain "fixed, no note" shape identity_test.go's own fixture
	// uses.
	for i := range dispositioned {
		dispositioned[i].Note = ""
	}

	changedSpec := testSpec(repo.Head)
	changedSpec.Declares.Boundaries = []artifact.Boundary{
		{From: "loansvc", To: "notification-svc", Via: "queue"},
	}
	second, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: changedSpec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeOKScript)}, JudgeTimeout: time.Second,
		ExistingFindings: dispositioned,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate (second): %v", err)
	}
	for _, id := range []string{"boundary-loansvc-notification-svc-queue", "boundary-loansvc-notification-svc-events"} {
		found := false
		for _, f := range second.Frontmatter.Findings {
			if f.ID == id {
				found = true
				if f.Dispositioned() {
					t.Fatalf("changed computed finding %s must be undispositioned, got %+v", id, f)
				}
			}
		}
		if !found {
			t.Fatalf("expected finding %s among %+v", id, second.Frontmatter.Findings)
		}
	}
}

// TestGenerate_CarriedFrom_DigestPurity_ExistingFrozenArchive is spec/
// finding-identity ac-2's fixture-level digest-purity proof, driven through
// the REAL pipeline: generate a report, disposition its judged finding with
// carried-from set (exactly the shape cmd/verdi's disposition verb produces
// on a confirmed reaffirmation — this package cannot import cmd/verdi, so
// the stamp is applied here the same way ExistingFindings simulates any
// other prior human edit), freeze it as an "existing frozen archive" would
// be, and prove VerifyDigest — recomputed from an INDEPENDENT, fresh
// align.Compute call — is byte-for-byte unaffected by the new field, on
// every existing frozen archive this shape represents.
func TestGenerate_CarriedFrom_DigestPurity_ExistingFrozenArchive(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	first, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}

	// Simulate a confirmed reaffirmation: the judged finding carries both a
	// disposition AND carried-from — the exact per-finding shape
	// cmd/verdi's disposition verb produces (proven end to end at that
	// layer, cmd/verdi/disposition_test.go).
	dispositioned := make([]artifact.Finding, len(first.Frontmatter.Findings))
	for i, f := range first.Frontmatter.Findings {
		f.Disposition = artifact.FindingAcceptedDeviation
		f.Note = "owner-ratified"
		if f.Kind == artifact.FindingJudged {
			f.CarriedFrom = repo.Head
		}
		dispositioned[i] = f
	}

	frozenAt, err := gitx.CommitDateOnly(context.Background(), repo.Dir, repo.Head)
	if err != nil {
		t.Fatalf("gitx.CommitDateOnly: %v", err)
	}
	frozen, err := FreezeInPlace(&artifact.DeviationFrontmatter{
		Schema:         first.Frontmatter.Schema,
		Covers:         first.Frontmatter.Covers,
		Findings:       dispositioned,
		Digest:         first.Frontmatter.Digest,
		Integrity:      first.Frontmatter.Integrity,
		JudgeIntegrity: first.Frontmatter.JudgeIntegrity,
		Provenance:     first.Frontmatter.Provenance,
	}, first.Body, frozenAt)
	if err != nil {
		t.Fatalf("FreezeInPlace: %v", err)
	}

	var sawCarriedFrom bool
	for _, f := range frozen.Frontmatter.Findings {
		if f.CarriedFrom != "" {
			sawCarriedFrom = true
		}
	}
	if !sawCarriedFrom {
		t.Fatal("test setup: no finding in the frozen fixture carries carried-from — the property under test never engaged")
	}

	// The independent verification path: re-run Compute fresh (exactly what
	// a verifier does — never trusting the stored digest, recomputing it)
	// and confirm it still matches, unaffected by carried-from's presence.
	computed, err := Compute(context.Background(), ComputedInput{Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head})
	if err != nil {
		t.Fatalf("independent Compute (for verification): %v", err)
	}
	if err := VerifyDigest(frozen.Frontmatter, computed); err != nil {
		t.Fatalf("VerifyDigest on a frozen archive carrying carried-from: %v — carried-from must be excluded from the digest (ac-2)", err)
	}
}

// TestBuildPrompt_TightensTowardStableSlugs proves the judge contract's
// prompt-text half (L-N2: "the judge contract tightens toward stable
// rule/boundary-derived slugs"): the rendered prompt instructs the judge to
// derive each finding's id from the rule/boundary it concerns, to reuse the
// identical id across runs for the same underlying issue, and never to
// reuse one id for two different findings in the same response — the
// NEVER-TRUST-IT half is identity.go's own job (Identity unchanged,
// ReconcileJudged's slug matching is pre-fill only), not this prompt's.
func TestBuildPrompt_TightensTowardStableSlugs(t *testing.T) {
	spec := testSpec("")
	prompt := string(BuildPrompt(spec, nil))

	for _, want := range []string{
		"RULE OR BOUNDARY",
		"never from your own prose",
		"reuse the IDENTICAL id",
		"each id must be unique within",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q; prompt:\n%s", want, prompt)
		}
	}
}
