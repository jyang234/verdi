package align

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

func baseGenerateInput(t *testing.T, repoDir, svcDir, covers string, spec *artifact.SpecFrontmatter) Input {
	t.Helper()
	return Input{
		Root:         repoDir,
		Runner:       seedComputeRunner(svcDir),
		Spec:         spec,
		Covers:       covers,
		JudgeCmd:     []string{writeFakeJudge(t, fakeJudgeOKScript)},
		JudgeTimeout: time.Second,
		ModelDigest:  testModelDigest(t),
	}
}

// testModelDigest is this package's test-fixture stand-in for
// store.Open(root).Model.Digest() (cmd/verdi/align.go's real source of
// Input.ModelDigest): none of this package's own test fixtures
// (buildComputeRepo, t.TempDir() roots in decision_report_test.go/
// diagram_report_test.go) ever write a .verdi/verdi.yaml, so there is no
// real store for store.Open to resolve here — but none of them ever write
// a .verdi/model.yaml either, so the resolved model IS model.Canonical()
// in every case, which is exactly the digest store.Open would produce for
// them. Shared across this package's *_test.go files (same package,
// internal tests). See fixtureModelDigest (decision_report_test.go) for
// the deliberately-distinct-model case ac-1 also requires.
func testModelDigest(t *testing.T) string {
	t.Helper()
	digest, err := model.Canonical().Digest()
	if err != nil {
		t.Fatalf("model.Canonical().Digest(): %v", err)
	}
	return digest
}

// fixtureModelYAML is internal/model/testdata/vocab-rename.yaml's own
// content verbatim (already proven frontier-legal by that package's own
// tests): structurally identical to the embedded canonical model, but with
// vocabulary renames and different per-class template filenames — the
// frontier's two named exceptions — so its Digest() differs from
// model.Canonical().Digest(). Used across this package's ac-1
// "tracks a distinct model" tests.
const fixtureModelYAML = `schema: verdi.model/v1

classes:
  feature:
    display: Feature
    decomposes: stubs
    template: custom-feature.md
  story:
    display: Story
    parent: feature
    template: custom-story.md

lifecycle:
  feature:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }
  story:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }

vocabulary:
  verbs:
    accept: "Sign off"
  states:
    accepted-pending-build: "Ready to build"
  classes:
    feature: "Initiative"
`

// fixtureModelDigest decodes fixtureModelYAML and returns its Digest() —
// this package's shared stand-in for "a store.Open resolution against a
// distinct .verdi/model.yaml", used by every mint site's "tracks a
// distinct model" test.
func fixtureModelDigest(t *testing.T) string {
	t.Helper()
	m, err := model.DecodeModel([]byte(fixtureModelYAML))
	if err != nil {
		t.Fatalf("model.DecodeModel(fixtureModelYAML): %v", err)
	}
	digest, err := m.Digest()
	if err != nil {
		t.Fatalf("fixture model Digest(): %v", err)
	}
	return digest
}

// TestGenerate_RoundTripsThroughDecodeDeviation proves the rendered
// markdown's frontmatter is exactly what DecodeDeviation (internal/artifact's
// strict decode seam) accepts — the schema round-trip a hand-rendered
// frontmatter template risks breaking silently.
func TestGenerate_RoundTripsThroughDecodeDeviation(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	report, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	fmBytes, body, err := artifact.SplitFrontmatter(report.Markdown)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fmBytes)
	if err != nil {
		t.Fatalf("DecodeDeviation(rendered markdown): %v\n---\n%s", err, report.Markdown)
	}
	if decoded.Covers != repo.Head {
		t.Fatalf("decoded.Covers = %q, want %q", decoded.Covers, repo.Head)
	}
	if len(decoded.Findings) != len(report.Frontmatter.Findings) {
		t.Fatalf("decoded %d findings, generated %d", len(decoded.Findings), len(report.Frontmatter.Findings))
	}
	if len(bytes.TrimSpace(body)) == 0 {
		t.Fatal("rendered body is empty")
	}
}

// TestGenerate_ByteIdenticalAcrossRuns proves the exit criteria's "computed
// section deterministic; judged section injected" property: two Generate
// calls against the same tree/commit with the same fake judge produce
// byte-identical markdown.
func TestGenerate_ByteIdenticalAcrossRuns(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)
	judgeScript := writeFakeJudge(t, fakeJudgeOKScript)

	run := func() []byte {
		report, err := Generate(context.Background(), Input{
			Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
			JudgeCmd: []string{judgeScript}, JudgeTimeout: time.Second,
			ModelDigest: testModelDigest(t),
		})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		return report.Markdown
	}

	first := run()
	second := run()
	if !bytes.Equal(first, second) {
		t.Fatalf("Generate not byte-identical across runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestGenerate_DigestRecomputes proves the computed digest recomputes from
// pinned inputs via VerifyDigest — independent of the judged section.
func TestGenerate_DigestRecomputes(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	report, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	computed, err := Compute(context.Background(), ComputedInput{Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head})
	if err != nil {
		t.Fatalf("Compute (for verification): %v", err)
	}
	if err := VerifyDigest(report.Frontmatter, computed); err != nil {
		t.Fatalf("VerifyDigest: %v", err)
	}
}

// TestGenerate_TamperedIntegrityFails proves tampering with the persisted
// judged text (the raw_result field VerifyIntegrity recomputes from) breaks
// the integrity check, without touching the stored hash itself.
func TestGenerate_TamperedIntegrityFails(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	report, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if err := VerifyIntegrity(report.Frontmatter); err != nil {
		t.Fatalf("VerifyIntegrity(untampered): %v", err)
	}

	tampered := *report.Frontmatter
	tamperedJI := *report.Frontmatter.JudgeIntegrity
	tamperedJI.RawResult = tamperedJI.RawResult + " tampered"
	tampered.JudgeIntegrity = &tamperedJI

	if err := VerifyIntegrity(&tampered); err == nil {
		t.Fatal("VerifyIntegrity(tampered raw_result): want error, got nil")
	}
}

// TestGenerate_NoJudgeConfigured_AbsenceFinding proves the no-judge path
// emits the synthetic absence finding, undispositioned, with no Integrity.
func TestGenerate_NoJudgeConfigured_AbsenceFinding(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	report, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		ModelDigest: testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if report.Frontmatter.Integrity != "" {
		t.Fatalf("Integrity = %q, want empty (no real judge exchange)", report.Frontmatter.Integrity)
	}
	var absence *artifact.Finding
	for i := range report.Frontmatter.Findings {
		if report.Frontmatter.Findings[i].ID == AbsenceFindingID {
			absence = &report.Frontmatter.Findings[i]
		}
	}
	if absence == nil {
		t.Fatalf("no absence finding among %+v", report.Frontmatter.Findings)
	}
	if absence.Dispositioned() {
		t.Fatalf("absence finding must be undispositioned fresh, got %+v", absence)
	}
}

// TestGenerate_JudgeRequiredAndAbsent_ReturnsSentinelError proves
// judge_required: true with no judge produces *ErrJudgeRequiredAbsent
// (cmd/verdi/align.go's exit-1 signal), not a plain operational error.
func TestGenerate_JudgeRequiredAndAbsent_ReturnsSentinelError(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	_, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		JudgeRequired: true,
	})
	if err == nil {
		t.Fatal("Generate(judge_required, no judge): want error, got nil")
	}
	var target *ErrJudgeRequiredAbsent
	if !errors.As(err, &target) {
		t.Fatalf("Generate error = %v (%T), want *ErrJudgeRequiredAbsent", err, err)
	}
}

// TestGenerate_PreservesDispositionsAcrossRegeneration proves the holds
// finding's disposition survives an unchanged regeneration, while a
// finding whose content changed (violated -> a different service set)
// starts fresh, undispositioned.
func TestGenerate_PreservesDispositionsAcrossRegeneration(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	first, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate (first): %v", err)
	}

	// A human dispositions every finding from the first report.
	dispositioned := make([]artifact.Finding, len(first.Frontmatter.Findings))
	for i, f := range first.Frontmatter.Findings {
		f.Disposition = artifact.FindingFixed
		dispositioned[i] = f
	}

	// Regenerate against the SAME tree/commit: content is unchanged, so
	// every finding's identity matches and every disposition must survive.
	second, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: spec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeOKScript)}, JudgeTimeout: time.Second,
		ExistingFindings: dispositioned,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate (second): %v", err)
	}
	if len(second.Frontmatter.Findings) != len(dispositioned) {
		t.Fatalf("second run findings = %d, want %d", len(second.Frontmatter.Findings), len(dispositioned))
	}
	for _, f := range second.Frontmatter.Findings {
		if !f.Dispositioned() {
			t.Fatalf("finding %s lost its disposition across an unchanged regeneration: %+v", f.ID, f)
		}
	}

	// Now change the spec's declares.boundaries: `via: queue` instead of
	// `via: events` for notification-svc. The notification-svc(events)
	// finding's TEXT flips from "holds" to "UNDECLARED" (a different claim
	// entirely, even though its id happens to be the same one the diff
	// scan would assign) and a brand new declared-violated finding appears
	// for notification-svc(queue) — both must come back undispositioned
	// despite the stale ExistingFindings. audit-svc's undeclared finding is
	// UNCHANGED by this edit (it was never declared, either before or
	// after) and its disposition correctly survives — that is not a bug,
	// it is Identity doing its job.
	changedSpec := testSpec(repo.Head)
	changedSpec.Declares.Boundaries = []artifact.Boundary{
		{From: "loansvc", To: "notification-svc", Via: "queue"},
	}
	third, err := Generate(context.Background(), Input{
		Root: repo.Dir, Runner: seedComputeRunner(svcDir), Spec: changedSpec, Covers: repo.Head,
		JudgeCmd: []string{writeFakeJudge(t, fakeJudgeOKScript)}, JudgeTimeout: time.Second,
		ExistingFindings: dispositioned,
		ModelDigest:      testModelDigest(t),
	})
	if err != nil {
		t.Fatalf("Generate (third): %v", err)
	}
	byID := make(map[string]artifact.Finding, len(third.Frontmatter.Findings))
	for _, f := range third.Frontmatter.Findings {
		byID[f.ID] = f
	}
	changedIDs := []string{"boundary-loansvc-notification-svc-queue", "boundary-loansvc-notification-svc-events"}
	for _, id := range changedIDs {
		f, ok := byID[id]
		if !ok {
			t.Fatalf("expected finding %s among %+v", id, byID)
		}
		if f.Dispositioned() {
			t.Fatalf("changed finding %s (%s) must be undispositioned, got %+v", f.ID, f.Text, f)
		}
	}
	unchanged, ok := byID["boundary-loansvc-audit-svc-http"]
	if !ok || !unchanged.Dispositioned() {
		t.Fatalf("unchanged finding boundary-loansvc-audit-svc-http should keep its disposition, got %+v (present=%v)", unchanged, ok)
	}
}

// TestGenerate_Freeze proves --freeze adds a Frozen stamp at Covers.
func TestGenerate_Freeze(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	in := baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec)
	in.Freeze = true
	in.FrozenAt = "2026-07-10"

	report, err := Generate(context.Background(), in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if report.Frontmatter.Frozen == nil || report.Frontmatter.Frozen.Commit != repo.Head || report.Frontmatter.Frozen.At != "2026-07-10" {
		t.Fatalf("Frozen = %+v, want {at: 2026-07-10, commit: %s}", report.Frontmatter.Frozen, repo.Head)
	}

	t.Run("freeze without FrozenAt is an operational error", func(t *testing.T) {
		bad := baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec)
		bad.Freeze = true
		if _, err := Generate(context.Background(), bad); err == nil {
			t.Fatal("Generate(Freeze, no FrozenAt): want error, got nil")
		}
	})
}

// TestGenerate_ModelDigestStamped is spec/model-digest ac-1's headline
// case: a deviation-report.md Generate produces carries provenance.model
// equal to the resolved model's own Digest() (canonical here — no
// model.yaml in this fixture), both on the in-memory Frontmatter and,
// crucially, in the actually-RENDERED and re-decoded markdown (render.go's
// renderFrontmatter hand-renders provenance: field by field, so this also
// proves the new model: clause was wired into that hand-renderer, not just
// computed and silently dropped).
func TestGenerate_ModelDigestStamped(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	report, err := Generate(context.Background(), baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	wantDigest := testModelDigest(t)
	if report.Frontmatter.Provenance == nil || report.Frontmatter.Provenance.Model != wantDigest {
		t.Fatalf("Frontmatter.Provenance.Model = %+v, want %q", report.Frontmatter.Provenance, wantDigest)
	}

	fmBytes, _, err := artifact.SplitFrontmatter(report.Markdown)
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	decoded, err := artifact.DecodeDeviation(fmBytes)
	if err != nil {
		t.Fatalf("DecodeDeviation(rendered markdown): %v\n---\n%s", err, report.Markdown)
	}
	if decoded.Provenance == nil || decoded.Provenance.Model != wantDigest {
		t.Fatalf("decoded rendered markdown's Provenance.Model = %+v, want %q — render.go's renderFrontmatter must render the model: clause, not just compute it in memory:\n%s", decoded.Provenance, wantDigest, report.Markdown)
	}
}

// TestGenerate_ModelDigestTracksFixtureModel is ac-1's distinguishing case:
// a DIFFERENT resolved model (never the embedded canonical) produces a
// provenance.model equal to THAT model's own digest, proving the stamped
// value tracks the actual resolved model rather than a constant that
// happens to match the one model every other test fixture uses.
func TestGenerate_ModelDigestTracksFixtureModel(t *testing.T) {
	repo := buildComputeRepo(t)
	svcDir := filepath.Join(repo.Dir, "loansvc")
	spec := testSpec(repo.Head)

	fixtureDigest := fixtureModelDigest(t)
	canonicalDigest := testModelDigest(t)
	if fixtureDigest == canonicalDigest {
		t.Fatalf("fixture model digest %q equals the canonical digest — the fixture is not actually distinct", fixtureDigest)
	}

	in := baseGenerateInput(t, repo.Dir, svcDir, repo.Head, spec)
	in.ModelDigest = fixtureDigest

	report, err := Generate(context.Background(), in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if report.Frontmatter.Provenance == nil || report.Frontmatter.Provenance.Model != fixtureDigest {
		t.Fatalf("Provenance.Model = %+v, want %q (the fixture model's own digest)", report.Frontmatter.Provenance, fixtureDigest)
	}
}
