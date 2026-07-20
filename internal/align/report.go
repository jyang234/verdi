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
	// Freeze requests the closure edition (`verdi align --freeze`): a
	// Frozen stamp is added, FrozenAt (a YYYY-MM-DD date, per Frozen.At —
	// the caller derives it from Covers's own commit date, never wall
	// clock) must be non-empty.
	Freeze   bool
	FrozenAt string
	// ExistingFindings are a prior report's findings (nil/empty for a first
	// run), the disposition-preservation source (identity.go).
	ExistingFindings []artifact.Finding
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
	})
	if err != nil {
		return nil, err // *ErrJudgeRequiredAbsent, propagated as-is
	}

	digest, err := ComputeDigest(in.Covers, computed.Findings, computed.BaselineDiffs)
	if err != nil {
		return nil, err
	}

	allFindings := make([]artifact.Finding, 0, len(computed.Findings)+len(judged.Findings))
	allFindings = append(allFindings, computed.Findings...)
	allFindings = append(allFindings, judged.Findings...)
	preserved := PreserveDispositions(allFindings, in.ExistingFindings)

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

	body := RenderBody(preserved, computed.BaselineDiffs, computed.DiagramProposals, computed.IllustrativeDiagrams)
	return &Report{Frontmatter: fm, Body: body, Markdown: RenderMarkdown(fm, body)}, nil
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
	b.WriteString("No prose outside the JSON.\n")
	return []byte(b.String())
}
