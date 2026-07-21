// Diagram sweep: judged section (spec/judged-sweep ac-2, dc-3). Reuses this
// package's existing judge plumbing (judge.go: JudgeRunner, ExecJudgeRunner,
// execJudgeEnvelope, computeIntegrity) and decision_judge.go's own shape
// (prompt builder, inner-result decode, entry point) — only the prompt
// content, the corpus this mode reads against (a diagram's mermaid body
// plus its own linked spec, rather than a spec's declared decisions), and
// the persistence home differ from decision_judge.go's design-branch
// sweep.
package align

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// DiagramAbsenceFindingID is the diagram-sweep mode's synthetic "judged
// coverage absent" finding id — decision_judge.go's DecisionAbsenceFindingID
// analogue, kept as its own distinct constant for the same reason: this
// report is independently decoded (DecodeDiagramSweep), so a shared
// constant would invite a caller to conflate the two.
const DiagramAbsenceFindingID = "judged-diagram-sweep-coverage-absent"

// resolveDiagramSpec finds a diagram's owning spec by scanning its own
// links for a derived-from edge targeting a spec ref — the same
// spec-tying convention this codebase's own diagram fixtures already use
// (e.g. examples/showcase/.verdi/diagrams/loansvc-topology.mermaid's
// `links: [{ type: derived-from, ref: spec/store-layout-notes }]`),
// mirroring decision_judge.go's resolveFeatureSpec exactly but walking a
// diagram's links instead of a story spec's implements link. Returns
// (nil, nil) when the diagram carries no such link (a from-scratch
// proposal with no spec tie, or an unlinked one): the sweep then compares
// against the ADR corpus only, exactly like a spike/no-implements story
// spec does in the design-branch sweep.
func resolveDiagramSpec(root string, diagram *artifact.DiagramFrontmatter) (*artifact.SpecFrontmatter, error) {
	for _, l := range diagram.Links {
		if l.Type != artifact.LinkDerivedFrom {
			continue
		}
		ref, err := artifact.ParseRef(l.Ref)
		if err != nil || ref.Kind != artifact.KindSpec {
			continue
		}
		spec, _, err := readSpecByName(root, ref.Name)
		if err != nil {
			return nil, err
		}
		if spec != nil {
			return spec, nil
		}
	}
	return nil, nil
}

// diagramSweepContext bundles a diagram sweep's inputs: the target
// diagram's own ref and already-read mermaid body, the ADR corpus (always
// read), and — when the diagram resolves to an owning spec via a
// derived-from link — that spec's own declared constraints and decisions
// (spec/judged-sweep ac-2: "the corpus's declared constraints/decisions").
type diagramSweepContext struct {
	DiagramRef string
	Body       []byte
	ADRCorpus  []adrCorpusEntry
	Spec       *artifact.SpecFrontmatter
}

// scannedIDs lists every constraint/decision id the sweep prompt actually
// included, qualified by owning spec ref, sorted — SweepProvenance's "decision
// set scanned", extended here to also cover constraints (spec/judged-sweep
// ac-2 names both).
func (c diagramSweepContext) scannedIDs() []string {
	if c.Spec == nil {
		return nil
	}
	var ids []string
	for _, co := range c.Spec.Constraints {
		ids = append(ids, c.Spec.ID+"#"+co.ID)
	}
	for _, dc := range c.Spec.Decisions {
		ids = append(ids, c.Spec.ID+"#"+dc.ID)
	}
	sort.Strings(ids)
	return ids
}

// BuildDiagramSweepPrompt renders the judged sweep's stdin prompt (mirrors
// BuildDecisionSweepPrompt: a pure function of already-deterministic
// inputs, so two runs against the same diagram body and corpus send
// byte-identical prompts). A diagram with no resolved owning spec sweeps
// against the ADR corpus only.
func BuildDiagramSweepPrompt(ctx diagramSweepContext) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "You are verdi's diagram-sweep judge for %s.\n\n", ctx.DiagramRef)
	b.WriteString("Below is a proposed future-state diagram's mermaid body, the org ADR corpus, and — when this ")
	b.WriteString("proposal is tied to a spec — that spec's own declared constraints and decisions. Hunt for ")
	b.WriteString("places where the proposal's structure collides with a constraint, decision, or ADR that ")
	b.WriteString("nobody has declared an exempts/supersedes edge against: a new node or edge a reviewer would ")
	b.WriteString("flag against something already decided.\n\n")

	b.WriteString("## Proposal mermaid body\n\n")
	b.Write(ctx.Body)
	b.WriteString("\n")

	b.WriteString("\n## ADR corpus\n\n")
	if len(ctx.ADRCorpus) == 0 {
		b.WriteString("(none)\n")
	}
	for _, e := range ctx.ADRCorpus {
		fmt.Fprintf(&b, "- %s (%s): %s\n", e.ref, e.fm.Status, e.fm.Title)
	}

	if ctx.Spec != nil {
		fmt.Fprintf(&b, "\n## %s's declared constraints\n\n", ctx.Spec.ID)
		if len(ctx.Spec.Constraints) == 0 {
			b.WriteString("(none)\n")
		}
		for _, co := range ctx.Spec.Constraints {
			fmt.Fprintf(&b, "- %s: %s\n", co.ID, co.Text)
		}

		fmt.Fprintf(&b, "\n## %s's declared decisions\n\n", ctx.Spec.ID)
		if len(ctx.Spec.Decisions) == 0 {
			b.WriteString("(none)\n")
		}
		for _, dc := range ctx.Spec.Decisions {
			fmt.Fprintf(&b, "- %s: %s\n", dc.ID, dc.Text)
		}
	}

	b.WriteString("\nRespond with ONLY a JSON object of the exact shape ")
	b.WriteString(`{"findings":[{"id":string,"text":string,"confidence":number between 0 and 1,"target":string}]}. `)
	b.WriteString("\"target\" MUST be the unpinned ref (adr/<name> or spec/<name>) of the ADR or constraint/decision-owning ")
	b.WriteString("spec the finding is about, so verdi can compute CODEOWNERS routing. No prose outside the JSON.\n")
	return []byte(b.String())
}

// diagramInnerResult / diagramInnerFinding mirror decision_judge.go's
// decisionInnerResult / decisionInnerFinding exactly (the same wire
// shape); kept as this mode's own type, decoded independently, rather than
// reused verbatim — mirroring decision_judge.go's own choice not to reuse
// judge.go's judgeInnerResult for its mode either.
type diagramInnerResult struct {
	Findings []diagramInnerFinding `json:"findings"`
}

type diagramInnerFinding struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Target     string  `json:"target"`
}

// DiagramJudgedResult is RunDiagramSweep's output.
type DiagramJudgedResult struct {
	Findings       []artifact.ConflictFinding
	Integrity      string
	JudgeIntegrity *artifact.JudgeIntegrity
}

// DiagramJudgedInput is RunDiagramSweep's input. Body carries the target
// diagram's ALREADY-READ mermaid body bytes — a value parameter, never a
// file path, *os.File, or any other writable handle to the diagram itself
// (spec/judged-sweep ac-4/dc-5): the type system makes an edit-back to the
// diagram impossible from within this function by construction, since it
// is never given anything it could open, let alone write.
type DiagramJudgedInput struct {
	DiagramRef    string
	Body          []byte
	ADRCorpus     []adrCorpusEntry
	Spec          *artifact.SpecFrontmatter
	JudgeCmd      []string
	JudgeRequired bool
	Timeout       time.Duration
}

// RunDiagramSweep is the diagram sweep's entry point — RunDecisionSweep's
// spec/judged-sweep analogue. Builds the sweep prompt from in's
// already-read inputs (BuildDiagramSweepPrompt), execs the judge through
// the SAME execJudgeEnvelope seam every judged mode in this package shares
// (no second exec path), and decodes its findings into
// []artifact.ConflictFinding — the existing four-value disposition
// machinery, unchanged. Returns a non-nil error ONLY when JudgeRequired is
// true and no judged section could be produced; every other failure mode
// degrades to the synthetic DiagramAbsenceFindingID finding, mirroring
// decisionAbsentResult/decisionAbsenceFinding exactly.
func RunDiagramSweep(ctx context.Context, runner JudgeRunner, in DiagramJudgedInput) (*DiagramJudgedResult, error) {
	prompt := BuildDiagramSweepPrompt(diagramSweepContext{
		DiagramRef: in.DiagramRef,
		Body:       in.Body,
		ADRCorpus:  in.ADRCorpus,
		Spec:       in.Spec,
	})

	if len(in.JudgeCmd) == 0 {
		return diagramAbsentResult(in.JudgeRequired, &JudgeFailure{
			Stage:  StageNotConfigured,
			Detail: "no align.judge_cmd configured in verdi.yaml (align: { judge_cmd: [...] })",
		})
	}

	if runner == nil {
		runner = ExecJudgeRunner{}
	}
	rawResult, failure := execJudgeEnvelope(ctx, runner, in.JudgeCmd, in.Timeout, prompt)
	if failure != nil {
		return diagramAbsentResult(in.JudgeRequired, failure)
	}

	inner, err := decodeDiagramInnerResult(rawResult)
	if err != nil {
		return diagramAbsentResult(in.JudgeRequired, &JudgeFailure{
			Stage:       StageInnerParse,
			CmdTemplate: strings.Join(in.JudgeCmd, " "),
			Detail:      fmt.Sprintf("decoding inner findings JSON: %v", err),
		})
	}

	findings := make([]artifact.ConflictFinding, 0, len(inner.Findings))
	for _, jf := range inner.Findings {
		findings = append(findings, artifact.ConflictFinding{
			ID:        judgedFindingID(jf.ID),
			Kind:      artifact.FindingJudged,
			Text:      fmt.Sprintf("%s (confidence %.2f)", jf.Text, jf.Confidence),
			TargetRef: jf.Target,
		})
	}

	return &DiagramJudgedResult{
		Findings:  findings,
		Integrity: computeIntegrity(prompt, rawResult),
		JudgeIntegrity: &artifact.JudgeIntegrity{
			StdinB64:  base64.StdEncoding.EncodeToString(prompt),
			RawResult: rawResult,
		},
	}, nil
}

// ErrDiagramJudgeRequiredAbsent mirrors ErrDecisionJudgeRequiredAbsent for
// the diagram sweep.
type ErrDiagramJudgeRequiredAbsent struct{ Failure *JudgeFailure }

func (e *ErrDiagramJudgeRequiredAbsent) Error() string {
	return fmt.Sprintf("align: align.judge_required is true but no diagram-sweep judged section was produced (stage=%s: %s)", e.Failure.Stage, e.Failure.Detail)
}

func diagramAbsentResult(required bool, failure *JudgeFailure) (*DiagramJudgedResult, error) {
	if required {
		return nil, &ErrDiagramJudgeRequiredAbsent{Failure: failure}
	}
	return &DiagramJudgedResult{Findings: []artifact.ConflictFinding{diagramAbsenceFinding(failure)}}, nil
}

// diagramAbsenceFinding mirrors decision_judge.go's decisionAbsenceFinding
// for the diagram-sweep report's own synthetic finding.
func diagramAbsenceFinding(f *JudgeFailure) artifact.ConflictFinding {
	text := fmt.Sprintf("judged diagram-sweep coverage absent: %s", f.Detail)
	if f.Stage != StageNotConfigured {
		text += fmt.Sprintf(" (stage=%s, exit=%d, cmd=%q", f.Stage, f.ExitCode, f.CmdTemplate)
		if f.StderrSnippet != "" {
			text += fmt.Sprintf(", stderr=%q", f.StderrSnippet)
		}
		text += ")"
	}
	return artifact.ConflictFinding{ID: DiagramAbsenceFindingID, Kind: artifact.FindingJudged, Text: text}
}

// decodeDiagramInnerResult mirrors decision_judge.go's
// decodeDecisionInnerResult (trim, strip a defensive markdown fence,
// strict-decode).
func decodeDiagramInnerResult(raw string) (*diagramInnerResult, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var inner diagramInnerResult
	if err := artifact.DecodeStrictJSON([]byte(s), &inner); err != nil {
		return nil, err
	}
	return &inner, nil
}
