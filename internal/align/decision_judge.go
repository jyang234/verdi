// Decision-conflict report: judged section (03 §Decision-conflict gate,
// "Judged section — the undeclared-conflict sweep"). Reuses this package's
// existing judge plumbing (judge.go: JudgeRunner, ExecJudgeRunner,
// execJudgeEnvelope, computeIntegrity) — only the prompt content and the
// inner-JSON contract differ from the build-branch report's judged section
// (judged.go).
package align

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/store"
)

// DecisionAbsenceFindingID is the design-branch mode's synthetic
// "judged coverage absent" finding id — decision_computed.go's judged
// analogue of AbsenceFindingID (judged.go), kept as a distinct constant
// (rather than reused verbatim) since the two live in sibling but
// independently-decoded report files and a shared constant would invite a
// caller to conflate them.
const DecisionAbsenceFindingID = "judged-decision-coverage-absent"

// adrCorpusEntry is one ADR read for the sweep: its decoded frontmatter
// plus the raw bytes the corpus digest hashes over.
type adrCorpusEntry struct {
	ref string
	fm  *artifact.ADRFrontmatter
	raw []byte
}

// loadADRCorpus reads every committed .verdi/adr/*.md, strict-decoding
// each, and returns them sorted by ref alongside a deterministic corpus
// digest (SweepProvenance.ADRCorpusDigest) — a canonjson hash over the
// sorted (ref, raw-content-sha256) pairs, recomputable from the same
// pinned tree and therefore able to detect a stale sweep (03 §Decision-
// conflict gate: "records its inputs — ADR corpus revision ... so a
// partial or stale sweep is detectable").
func loadADRCorpus(root string) ([]adrCorpusEntry, string, error) {
	dir := filepath.Join(root, ".verdi", "adr")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			digest, derr := adrCorpusDigest(nil)
			return nil, digest, derr
		}
		return nil, "", fmt.Errorf("align: reading %s: %w", dir, err)
	}

	var out []adrCorpusEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("align: reading %s: %w", path, err)
		}
		fmBytes, _, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			return nil, "", fmt.Errorf("align: %s: %w", path, err)
		}
		adr, err := artifact.DecodeADR(fmBytes)
		if err != nil {
			return nil, "", fmt.Errorf("align: %s: %w", path, err)
		}
		out = append(out, adrCorpusEntry{ref: adr.ID, fm: adr, raw: raw})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ref < out[j].ref })

	digest, err := adrCorpusDigest(out)
	if err != nil {
		return nil, "", err
	}
	return out, digest, nil
}

type adrDigestEntry struct {
	Ref     string `json:"ref"`
	Content string `json:"content_sha256"`
}

// adrCorpusDigest hashes the sorted (ref, content-sha256) list via
// canonjson.Digest (spec/shared-homes ac-2), mirroring verify.go's own
// ComputeDigest formula. The per-entry content-sha256 stays a raw
// crypto/sha256 sum (it identifies one ADR file's bytes, not the digest
// tail itself); only the outer hash-of-the-list collapses to Digest.
func adrCorpusDigest(entries []adrCorpusEntry) (string, error) {
	digestEntries := make([]adrDigestEntry, 0, len(entries))
	for _, e := range entries {
		sum := sha256.Sum256(e.raw)
		digestEntries = append(digestEntries, adrDigestEntry{Ref: e.ref, Content: hex.EncodeToString(sum[:])})
	}
	digest, err := canonjson.Digest(digestEntries)
	if err != nil {
		return "", fmt.Errorf("align: marshaling ADR corpus digest input: %w", err)
	}
	return digest, nil
}

// resolveFeatureSpec finds story's owning feature spec by scanning its own
// `implements` links for a spec ref (02 §Link taxonomy: a story's
// `implements` edges target its feature's AC fragments,
// `spec/<feature>#ac-N>`) and reading that feature spec from the working
// tree. Returns (nil, nil) when story carries no implements link at all
// (a spike story, which resolves rather than implements — 02 §Kind
// registry) — the judged sweep then compares against the ADR corpus only,
// the same as a feature-class spec.
func resolveFeatureSpec(root string, story *artifact.SpecFrontmatter) (*artifact.SpecFrontmatter, error) {
	for _, l := range story.Links {
		if l.Type != artifact.LinkImplements {
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

// decisionSweepContext bundles a design-branch sweep's inputs: the branch's
// own spec, the ADR corpus (always read), and — for a story-class spec
// only — its resolved parent feature (03 §Decision-conflict gate: "story
// decisions against their feature's decisions").
type decisionSweepContext struct {
	Spec        *artifact.SpecFrontmatter
	ADRCorpus   []adrCorpusEntry
	FeatureSpec *artifact.SpecFrontmatter
}

// scannedDecisionIDs lists every decision id the sweep prompt actually
// included, qualified by owning spec ref, sorted — SweepProvenance's
// "decision set scanned".
func (c decisionSweepContext) scannedDecisionIDs() []string {
	var ids []string
	for _, dc := range c.Spec.Decisions {
		ids = append(ids, c.Spec.ID+"#"+dc.ID)
	}
	if c.FeatureSpec != nil {
		for _, dc := range c.FeatureSpec.Decisions {
			ids = append(ids, c.FeatureSpec.ID+"#"+dc.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// BuildDecisionSweepPrompt renders the judged sweep's stdin prompt (mirrors
// report.go's BuildPrompt: a pure function of already-deterministic
// inputs, so two runs against the same tree send byte-identical prompts).
// Feature-class specs (and spike/no-implements story specs) sweep against
// the ADR corpus only; a story spec with a resolved parent feature also
// sweeps against that feature's own decisions (03 §Decision-conflict
// gate).
func BuildDecisionSweepPrompt(ctx decisionSweepContext) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "You are verdi's decision-conflict judge for %s.\n\n", ctx.Spec.ID)
	b.WriteString("Below are this spec's own declared decisions, the org ADR corpus, and — when this is a ")
	b.WriteString("story spec — its parent feature's decisions. Hunt for UNDECLARED conflicts: places where a ")
	b.WriteString("decision below contradicts an ADR or a parent-feature decision that nobody declared a ")
	b.WriteString("supersedes/exempts edge against.\n\n")

	fmt.Fprintf(&b, "## %s's own decisions\n\n", ctx.Spec.ID)
	if len(ctx.Spec.Decisions) == 0 {
		b.WriteString("(none)\n")
	}
	for _, dc := range ctx.Spec.Decisions {
		fmt.Fprintf(&b, "- %s: %s\n", dc.ID, dc.Text)
	}

	b.WriteString("\n## ADR corpus\n\n")
	if len(ctx.ADRCorpus) == 0 {
		b.WriteString("(none)\n")
	}
	for _, e := range ctx.ADRCorpus {
		fmt.Fprintf(&b, "- %s (%s): %s\n", e.ref, e.fm.Status, e.fm.Title)
	}

	if ctx.FeatureSpec != nil {
		fmt.Fprintf(&b, "\n## Parent feature %s's decisions\n\n", ctx.FeatureSpec.ID)
		if len(ctx.FeatureSpec.Decisions) == 0 {
			b.WriteString("(none)\n")
		}
		for _, dc := range ctx.FeatureSpec.Decisions {
			fmt.Fprintf(&b, "- %s: %s\n", dc.ID, dc.Text)
		}
	}

	b.WriteString("\nRespond with ONLY a JSON object of the exact shape ")
	b.WriteString(`{"findings":[{"id":string,"text":string,"confidence":number between 0 and 1,"target":string}]}. `)
	b.WriteString("\"target\" MUST be the unpinned ref (adr/<name> or spec/<name>) of the ADR or decision-owning ")
	b.WriteString("spec the finding is about, so verdi can compute CODEOWNERS routing. No prose outside the JSON.\n")
	return []byte(b.String())
}

// decisionInnerResult is the decision-conflict judge's own findings
// contract — S5's shape plus a required "target" field this mode adds so
// verdi can compute CODEOWNERS routing on an exempt/no-conflict
// disposition (03 §Decision-conflict gate) without inventing a second
// judge round-trip.
type decisionInnerResult struct {
	Findings []decisionInnerFinding `json:"findings"`
}

type decisionInnerFinding struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Target     string  `json:"target"`
}

// DecisionJudgedResult is RunDecisionSweep's output.
type DecisionJudgedResult struct {
	Findings       []artifact.ConflictFinding
	Integrity      string
	JudgeIntegrity *artifact.JudgeIntegrity
}

// DecisionJudgedInput is RunDecisionSweep's input, mirroring JudgedInput
// (judged.go).
type DecisionJudgedInput struct {
	JudgeCmd      []string
	JudgeRequired bool
	Timeout       time.Duration
	Prompt        []byte
}

// RunDecisionSweep is the judged section's entry point — RunJudged's
// design-branch analogue. Same contract: a non-nil error ONLY when
// JudgeRequired is true and no judged section could be produced; every
// other failure mode degrades to the synthetic DecisionAbsenceFindingID
// finding (undispositioned — 03 §Decision-conflict gate's status table
// treats a skipped sweep as "disclosed-unproven-complete" rather than
// requiring a human disposition first, unlike the build-branch report's
// own absence finding; see decision_report.go's DecisionGateStatuses).
func RunDecisionSweep(ctx context.Context, runner JudgeRunner, in DecisionJudgedInput) (*DecisionJudgedResult, error) {
	if len(in.JudgeCmd) == 0 {
		return decisionAbsentResult(in.JudgeRequired, &JudgeFailure{
			Stage:  StageNotConfigured,
			Detail: "no align.judge_cmd configured in verdi.yaml (align: { judge_cmd: [...] })",
		})
	}

	if runner == nil {
		runner = ExecJudgeRunner{}
	}
	rawResult, failure := execJudgeEnvelope(ctx, runner, in.JudgeCmd, in.Timeout, in.Prompt)
	if failure != nil {
		return decisionAbsentResult(in.JudgeRequired, failure)
	}

	inner, err := decodeDecisionInnerResult(rawResult)
	if err != nil {
		return decisionAbsentResult(in.JudgeRequired, &JudgeFailure{
			Stage:       StageInnerParse,
			CmdTemplate: strings.Join(in.JudgeCmd, " "),
			Detail:      fmt.Sprintf("decoding inner findings JSON: %v", err),
		})
	}

	findings := make([]artifact.ConflictFinding, 0, len(inner.Findings))
	for _, jf := range inner.Findings {
		findings = append(findings, artifact.ConflictFinding{
			ID:        "judged-" + store.RefSlug(jf.ID),
			Kind:      artifact.FindingJudged,
			Text:      fmt.Sprintf("%s (confidence %.2f)", jf.Text, jf.Confidence),
			TargetRef: jf.Target,
		})
	}

	return &DecisionJudgedResult{
		Findings:  findings,
		Integrity: computeIntegrity(in.Prompt, rawResult),
		JudgeIntegrity: &artifact.JudgeIntegrity{
			StdinB64:  base64.StdEncoding.EncodeToString(in.Prompt),
			RawResult: rawResult,
		},
	}, nil
}

// ErrDecisionJudgeRequiredAbsent mirrors ErrJudgeRequiredAbsent for the
// design-branch sweep.
type ErrDecisionJudgeRequiredAbsent struct{ Failure *JudgeFailure }

func (e *ErrDecisionJudgeRequiredAbsent) Error() string {
	return fmt.Sprintf("align: align.judge_required is true but no decision-conflict judged section was produced (stage=%s: %s)", e.Failure.Stage, e.Failure.Detail)
}

func decisionAbsentResult(required bool, failure *JudgeFailure) (*DecisionJudgedResult, error) {
	if required {
		return nil, &ErrDecisionJudgeRequiredAbsent{Failure: failure}
	}
	return &DecisionJudgedResult{Findings: []artifact.ConflictFinding{decisionAbsenceFinding(failure)}}, nil
}

// decisionAbsenceFinding mirrors judged.go's absenceFinding for the
// decision-conflict report's own synthetic finding.
func decisionAbsenceFinding(f *JudgeFailure) artifact.ConflictFinding {
	text := fmt.Sprintf("judged decision-conflict coverage absent: %s", f.Detail)
	if f.Stage != StageNotConfigured {
		text += fmt.Sprintf(" (stage=%s, exit=%d, cmd=%q", f.Stage, f.ExitCode, f.CmdTemplate)
		if f.StderrSnippet != "" {
			text += fmt.Sprintf(", stderr=%q", f.StderrSnippet)
		}
		text += ")"
	}
	return artifact.ConflictFinding{ID: DecisionAbsenceFindingID, Kind: artifact.FindingJudged, Text: text}
}

// decodeDecisionInnerResult mirrors judge.go's decodeInnerResult (trim,
// strip a defensive markdown fence, strict-decode).
func decodeDecisionInnerResult(raw string) (*decisionInnerResult, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var inner decisionInnerResult
	if err := artifact.DecodeStrictJSON([]byte(s), &inner); err != nil {
		return nil, err
	}
	return &inner, nil
}
