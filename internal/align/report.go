package align

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/upstream"
)

const (
	generatorName    = "verdi-align"
	generatorVersion = "v0"
)

// Input is Generate's full input: the build head plus everything injectable
// for hermetic testing (upstream.Runner, JudgeRunner).
type Input struct {
	// Root is the store root.
	Root string
	// Runner execs flowmap/groundwork (nil is an operational error — see
	// Compute).
	Runner upstream.Runner
	// JudgeRunner execs the judge command; nil defaults to ExecJudgeRunner{}
	// (production). Tests inject a fake.
	JudgeRunner JudgeRunner
	// Spec is the build-head feature spec (already resolved/decoded by the
	// caller, e.g. via storyresolve.ResolveBuildSpec).
	Spec *artifact.SpecFrontmatter
	// Covers is the build head commit sha (`covers`).
	Covers string
	// JudgeCmd/JudgeRequired/JudgeTimeout mirror verdi.yaml's align: block
	// (JudgeTimeout overrides DefaultJudgeTimeout; tests only).
	JudgeCmd      []string
	JudgeRequired bool
	JudgeTimeout  time.Duration
	// Wait mirrors JudgedInput.Wait (spec/judge-ergonomics ac-2): a judge
	// that does not complete within JudgeTimeout makes Generate return
	// *ErrJudgeWaitExpired instead of a Report carrying a synthetic absence
	// finding — cmd/verdi/align.go's --wait flag sets this.
	Wait bool
	// Freeze requests the closure edition (`verdi align --freeze`): a
	// Frozen stamp is added, FrozenAt (a YYYY-MM-DD date, per Frozen.At —
	// the caller derives it from Covers's own commit date, never wall
	// clock) must be non-empty.
	Freeze   bool
	FrozenAt string
	// ExistingFindings are a prior report's findings (nil/empty for a first
	// run), the disposition-preservation source (identity.go).
	ExistingFindings []artifact.Finding
	// ExistingNotResurfaced is a prior report's own not-resurfaced: section
	// (nil/empty for a first run, or a prior report predating this story) —
	// spec/finding-identity ac-3's other reconciliation source, threaded
	// straight to ReconcileJudged so an already-persisted entry stays
	// persisted across any number of further non-reproducing regenerations.
	ExistingNotResurfaced []artifact.Finding
	// ModelDigest is the resolved operating model's canonical-JSON sha256
	// digest (model.Model.Digest(), spec/model-digest ledger L-M5) — the
	// caller (cmd/verdi/align.go) resolves it once via
	// store.Open(Root).Model.Digest() ahead of calling Generate, since this
	// package never imports internal/model itself (that package already
	// imports internal/artifact, so the reverse import would cycle).
	// Threaded straight to artifact.StampProvenance; empty reaches
	// StampProvenance's own panic.
	ModelDigest string
}

// Report is Generate's output: the decoded, self-validated frontmatter, the
// rendered body, and the full file content ready to write to
// deviation-report.md.
type Report struct {
	Frontmatter *artifact.DeviationFrontmatter
	Body        string
	Markdown    []byte
	// JudgedTally is spec/finding-identity's CONTROLLER DIRECTIVE (chronicle
	// P2-9): a carried-vs-new tally over THIS run's judged findings, so a
	// dry LOOP-UNTIL-DRY round (P2-9's own policy) is mechanically legible
	// as New == 0 — cmd/verdi/align.go prints it verbatim on every run.
	JudgedTally JudgedTally
}

// JudgedTally is Report.JudgedTally's shape: Total judged findings in the
// final report (every bucket below, plus any already-settled exact-identity
// carry that needs no human action and so is not itself named); Candidates
// is the count awaiting reaffirmation (ReconcileJudged's Candidates map);
// New is every remaining undispositioned judged finding — a brand-new
// finding or a disclosed slug-collision violation, neither of which has any
// prior ruling to pre-fill against.
type JudgedTally struct {
	Total      int
	Candidates int
	New        int
}

// Generate runs the whole `verdi align` pipeline: regenerate the computed
// section (Compute), run the judged section (RunJudged), preserve
// dispositions for unchanged findings across regeneration
// (PreserveDispositions), and render deviation-report.md.
//
// Returns *ErrJudgeRequiredAbsent (never wrapped further, so
// errors.As(err, &target) sees through Generate) when JudgeRequired is true
// and no judge produced a judged section — cmd/verdi/align.go maps this to
// exit 1; every other error here is operational (exit 2).
func Generate(ctx context.Context, in Input) (*Report, error) {
	if in.Spec == nil {
		return nil, fmt.Errorf("align: Generate: Spec is required")
	}
	if in.Covers == "" {
		return nil, fmt.Errorf("align: Generate: Covers must not be empty")
	}
	if in.Freeze && in.FrozenAt == "" {
		return nil, fmt.Errorf("align: Generate: --freeze requires FrozenAt")
	}

	judgeRunner := in.JudgeRunner
	if judgeRunner == nil {
		judgeRunner = ExecJudgeRunner{}
	}

	computed, err := Compute(ctx, ComputedInput{Root: in.Root, Runner: in.Runner, Spec: in.Spec, Covers: in.Covers})
	if err != nil {
		return nil, err
	}

	prompt := BuildPrompt(in.Spec, computed.Findings)
	judged, err := RunJudged(ctx, judgeRunner, JudgedInput{
		JudgeCmd:      in.JudgeCmd,
		JudgeRequired: in.JudgeRequired,
		Timeout:       in.JudgeTimeout,
		Prompt:        prompt,
		Wait:          in.Wait,
	})
	if err != nil {
		return nil, err // *ErrJudgeRequiredAbsent or *ErrJudgeWaitExpired, propagated as-is
	}

	digest, err := ComputeDigest(in.Covers, computed.Findings, computed.BaselineDiffs)
	if err != nil {
		return nil, err
	}

	// spec/finding-identity (L-N2): computed findings keep using
	// PreserveDispositions UNCHANGED — Identity's frozen rule, byte-for-byte
	// (ac-2). Judged findings route through ReconcileJudged instead, scoped
	// to Kind == FindingJudged only; identity.go's own Identity function is
	// still what ReconcileJudged calls for its exact-match carry-forward, so
	// the ONE identity rule is never duplicated.
	preservedComputed := PreserveDispositions(computed.Findings, findingsOfKind(in.ExistingFindings, artifact.FindingComputed))
	judgedRecon := ReconcileJudged(judged.Findings, findingsOfKind(in.ExistingFindings, artifact.FindingJudged), in.ExistingNotResurfaced)

	preserved := make([]artifact.Finding, 0, len(preservedComputed)+len(judgedRecon.Findings))
	preserved = append(preserved, preservedComputed...)
	preserved = append(preserved, judgedRecon.Findings...)

	prov := &artifact.Provenance{
		Generator: generatorName,
		Version:   generatorVersion,
		Inputs:    buildProvenanceInputs(in.Spec, in.Covers),
		Digest:    digest,
		Integrity: judged.Integrity,
	}
	artifact.StampProvenance(prov, in.ModelDigest)

	fm := &artifact.DeviationFrontmatter{
		Schema:         "verdi.deviation/v1",
		Covers:         in.Covers,
		Findings:       preserved,
		NotResurfaced:  judgedRecon.NotResurfaced,
		Digest:         digest,
		Integrity:      judged.Integrity,
		JudgeIntegrity: judged.JudgeIntegrity,
		Provenance:     prov,
	}
	if in.Freeze {
		frozen := artifact.NewFrozen(in.FrozenAt, in.Covers)
		fm.Frozen = &frozen
	}

	// Never fake success (CLAUDE.md): self-validate what Generate is about
	// to hand back before returning it as though it were a valid report.
	if err := fm.Validate(); err != nil {
		return nil, fmt.Errorf("align: internal error: generated frontmatter failed self-validation: %w", err)
	}

	body := RenderBody(preserved, judgedRecon.Candidates, judgedRecon.NotResurfaced, computed.BaselineDiffs, computed.DiagramProposals, computed.IllustrativeDiagrams)
	return &Report{
		Frontmatter: fm,
		Body:        body,
		Markdown:    RenderMarkdown(fm, body),
		JudgedTally: tallyJudged(judgedRecon),
	}, nil
}

// tallyJudged computes Report.JudgedTally from a JudgedReconciliation:
// Total counts every judged finding in the final report (exact-carried,
// candidate, new, and disclosed collision alike); Candidates counts entries
// awaiting reaffirmation; New is the remainder — undispositioned findings
// with no Candidate pairing (a genuinely new finding, or a disclosed
// slug-collision violation).
func tallyJudged(r JudgedReconciliation) JudgedTally {
	var t JudgedTally
	for _, f := range r.Findings {
		t.Total++
		if f.Dispositioned() {
			continue
		}
		if _, ok := r.Candidates[f.ID]; ok {
			t.Candidates++
			continue
		}
		t.New++
	}
	return t
}

// buildProvenanceInputs lists the pinned inputs the computed digest is
// recomputable from: the spec as it stands at the build head, and — when
// the spec carries an acceptance stamp — the spec as it stood at
// acceptance, which anchors the boundary-diff baseline (computed.go).
// Provenance.Validate requires at least one entry; a spec ref is always
// present.
func buildProvenanceInputs(spec *artifact.SpecFrontmatter, covers string) []string {
	inputs := []string{spec.ID + "@" + covers}
	if spec.Frozen != nil {
		inputs = append(inputs, spec.ID+"@"+spec.Frozen.Commit)
	}
	return inputs
}

// BuildPrompt renders the judge's stdin prompt (S5: "the rendered prompt
// goes to the child's stdin only") deterministically from spec and the
// computed section's findings — a pure function of already-deterministic
// inputs, so two Generate calls against the same tree/commit send the judge
// byte-identical prompts.
//
// spec/finding-identity's judge contract (ledger L-N2): the prompt is
// tightened toward stable, rule/boundary-derived ids ("the id" text below) —
// but this is a request to the judge, NEVER a trusted identity key
// (identity.go's own doc comment has the full rationale). A judge that
// ignores the request produces a drifting or colliding slug; ReconcileJudged
// (reaffirm.go) handles both honestly (a candidate pre-fill, or a disclosed
// contract-violation finding) rather than assuming compliance.
func BuildPrompt(spec *artifact.SpecFrontmatter, computedFindings []artifact.Finding) []byte {
	var b strings.Builder
	// vocab:identity — judge-prompt scaffold speaking corpus schema ids to the agent
	fmt.Fprintf(&b, "You are verdi's alignment judge for %s (story %s).\n\n", spec.ID, spec.Story)
	b.WriteString("Below is this build's mechanically computed alignment section (regenerated ")
	b.WriteString("boundary contracts diffed against the spec's declared boundaries). Read the ")
	b.WriteString("spec's acceptance criteria and report semantic deviations a graph diff cannot ")
	b.WriteString("see: places where the implementation's intent diverges from the spec's, even ")
	b.WriteString("where every mechanical check below holds.\n\n")

	fmt.Fprintf(&b, "## Spec: %s\n\n", spec.Title)
	for _, ac := range spec.AcceptanceCriteria {
		fmt.Fprintf(&b, "- %s: %s\n", ac.ID, ac.Text)
	}

	b.WriteString("\n## Computed findings\n\n")
	if len(computedFindings) == 0 {
		b.WriteString("(none)\n")
	}
	for _, f := range computedFindings {
		fmt.Fprintf(&b, "- %s: %s\n", f.ID, f.Text)
	}

	b.WriteString("\nRespond with ONLY a JSON object of the exact shape ")
	b.WriteString(`{"findings":[{"id":string,"text":string,"confidence":number between 0 and 1}]}. `)
	b.WriteString("Each finding's id must be a short, stable slug derived from the RULE OR BOUNDARY ")
	b.WriteString("the finding is about (e.g. \"retry-semantics\", \"boundary-loansvc-notification-svc\") ")
	b.WriteString("— never from your own prose or wording. If, on a later run over an unchanged issue, ")
	b.WriteString("you would describe it differently, still reuse the IDENTICAL id: verdi tracks each ")
	b.WriteString("finding's disposition by this id across runs, so a changed id for the same underlying ")
	b.WriteString("issue is read as a brand-new, never-reviewed finding. Never reuse the same id for two ")
	b.WriteString("genuinely different findings in this same response — each id must be unique within ")
	b.WriteString("this response. ")
	b.WriteString("No prose outside the JSON.\n")
	return []byte(b.String())
}
