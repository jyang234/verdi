// Decision-conflict report orchestration (03 §Decision-conflict gate; 05
// §CLI's `align` design-branch mode row): `verdi align`'s design-branch
// analogue of report.go's Generate, wiring decision_computed.go's computed
// section and decision_judge.go's judged section into one
// decision-conflict-report.md, reusing this package's disposition-
// preservation (identity.go) and digest (verify.go's formula, reapplied
// here) machinery.
package align

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

const decisionGeneratorVersion = "v1"

// GateStatus is one of 03 §Decision-conflict gate's three-valued gate
// status labels. An empty GateStatus is deliberately not one of the three
// — it means "not yet earned any of the three honest labels" (a computed
// section with an unresolved edge, or a judged section that found real
// conflicts not all dispositioned yet): the table describes end states,
// never a bare pass, so this package never invents a fourth label for the
// in-between (see DecisionGateStatuses' doc comment).
type GateStatus string

const (
	StatusProven                    GateStatus = "proven"
	StatusFoundAndDispositioned     GateStatus = "found-and-dispositioned"
	StatusDisclosedUnprovenComplete GateStatus = "disclosed-unproven-complete"
)

// DecisionConflictInput is GenerateDecisionConflict's input, mirroring
// Input (report.go) for the design-branch mode.
type DecisionConflictInput struct {
	// Root is the store root.
	Root string
	// Runner/JudgeRunner mirror Input; JudgeRunner nil defaults to
	// ExecJudgeRunner{}.
	JudgeRunner JudgeRunner
	// Spec is the design branch's own spec (feature or story class).
	Spec *artifact.SpecFrontmatter
	// Covers is the design branch head commit.
	Covers string
	// JudgeCmd/JudgeRequired/JudgeTimeout mirror verdi.yaml's align: block.
	JudgeCmd      []string
	JudgeRequired bool
	JudgeTimeout  time.Duration
	// Freeze/FrozenAt mirror Input.
	Freeze   bool
	FrozenAt string
	// ExistingFindings are a prior report's findings, the disposition-
	// preservation source.
	ExistingFindings []artifact.ConflictFinding
	// ModelDigest mirrors Input.ModelDigest (report.go) — the resolved
	// operating model's digest, resolved once by the caller and threaded
	// straight to artifact.StampProvenance.
	ModelDigest string
}

// DecisionConflictReport is GenerateDecisionConflict's output.
type DecisionConflictReport struct {
	Frontmatter *artifact.DecisionConflictFrontmatter
	Body        string
	Markdown    []byte
}

// GenerateDecisionConflict runs the design-branch decision-conflict-report
// pipeline: compute the computed section (declared-edge completeness),
// load the ADR corpus and (for a story spec) the parent feature, run the
// judged sweep, preserve dispositions and compute CODEOWNERS routing, and
// render decision-conflict-report.md.
//
// Returns *ErrDecisionJudgeRequiredAbsent (never wrapped further) when
// JudgeRequired is true and no judge produced a judged section — the
// cmd/verdi/align.go caller maps this to exit 1, the same convention
// report.go's Generate uses for the build-branch report.
func GenerateDecisionConflict(ctx context.Context, in DecisionConflictInput) (*DecisionConflictReport, error) {
	if in.Spec == nil {
		return nil, fmt.Errorf("align: GenerateDecisionConflict: Spec is required")
	}
	if in.Covers == "" {
		return nil, fmt.Errorf("align: GenerateDecisionConflict: Covers must not be empty")
	}
	if in.Root == "" {
		return nil, fmt.Errorf("align: GenerateDecisionConflict: Root must not be empty")
	}
	if in.Freeze && in.FrozenAt == "" {
		return nil, fmt.Errorf("align: GenerateDecisionConflict: --freeze requires FrozenAt")
	}

	judgeRunner := in.JudgeRunner
	if judgeRunner == nil {
		judgeRunner = ExecJudgeRunner{}
	}

	computedFindings, err := ComputeDecisionEdges(in.Root, in.Spec)
	if err != nil {
		return nil, err
	}

	adrCorpus, adrDigest, err := loadADRCorpus(in.Root)
	if err != nil {
		return nil, err
	}

	var featureSpec *artifact.SpecFrontmatter
	if in.Spec.Class == artifact.ClassStory {
		featureSpec, err = resolveFeatureSpec(in.Root, in.Spec)
		if err != nil {
			return nil, err
		}
	}

	swCtx := decisionSweepContext{Spec: in.Spec, ADRCorpus: adrCorpus, FeatureSpec: featureSpec}
	prompt := BuildDecisionSweepPrompt(swCtx)

	judged, err := RunDecisionSweep(ctx, judgeRunner, DecisionJudgedInput{
		JudgeCmd:      in.JudgeCmd,
		JudgeRequired: in.JudgeRequired,
		Timeout:       in.JudgeTimeout,
		Prompt:        prompt,
	})
	if err != nil {
		return nil, err // *ErrDecisionJudgeRequiredAbsent, propagated as-is
	}

	allFindings := make([]artifact.ConflictFinding, 0, len(computedFindings)+len(judged.Findings))
	allFindings = append(allFindings, computedFindings...)
	allFindings = append(allFindings, judged.Findings...)
	preserved := PreserveConflictDispositions(allFindings, in.ExistingFindings)
	preserved = computeRouting(preserved, adrCorpus)

	scanned := swCtx.scannedDecisionIDs()
	digest, err := ComputeDecisionDigest(in.Covers, computedFindings, adrDigest, scanned)
	if err != nil {
		return nil, err
	}

	prov := &artifact.Provenance{
		Generator: generatorName,
		Version:   decisionGeneratorVersion,
		Inputs:    buildProvenanceInputs(in.Spec, in.Covers),
		Digest:    digest,
		Integrity: judged.Integrity,
	}
	artifact.StampProvenance(prov, in.ModelDigest)

	fm := &artifact.DecisionConflictFrontmatter{
		Schema:   "verdi.decisionconflict/v1",
		Covers:   in.Covers,
		Findings: preserved,
		SweepProvenance: &artifact.SweepProvenance{
			ADRCorpusDigest:  adrDigest,
			DecisionsScanned: scanned,
		},
		Digest:         digest,
		Integrity:      judged.Integrity,
		JudgeIntegrity: judged.JudgeIntegrity,
		Provenance:     prov,
	}
	if in.Freeze {
		frozen := artifact.NewFrozen(in.FrozenAt, in.Covers)
		fm.Frozen = &frozen
	}

	if err := fm.Validate(); err != nil {
		return nil, fmt.Errorf("align: internal error: generated decision-conflict frontmatter failed self-validation: %w", err)
	}

	body := RenderDecisionBody(preserved)
	return &DecisionConflictReport{Frontmatter: fm, Body: body, Markdown: RenderDecisionMarkdown(fm, body)}, nil
}

// adrOwnersByRef indexes an ADR corpus by unpinned ref for CODEOWNERS
// routing lookups.
func adrOwnersByRef(corpus []adrCorpusEntry) map[string][]string {
	m := make(map[string][]string, len(corpus))
	for _, e := range corpus {
		m[e.ref] = e.fm.Owners
	}
	return m
}

// computeRouting fills RoutedOwners on every finding whose Disposition is
// exempt or no-conflict and whose TargetRef resolves to an ADR in corpus
// (03 §Decision-conflict gate: "An EXEMPT or no-conflict disposition of a
// judged finding that targets an ADR is CODEOWNERS-routed to that ADR's
// owners" — computed and disclosed here, never enforced: this function
// only annotates the finding with the ADR's own Owners field, it never
// calls a forge API or blocks anything). Applies uniformly to computed and
// judged findings alike, since a computed exempts edge against an ADR is
// exactly as CODEOWNERS-relevant as a judged one.
func computeRouting(findings []artifact.ConflictFinding, corpus []adrCorpusEntry) []artifact.ConflictFinding {
	owners := adrOwnersByRef(corpus)
	out := make([]artifact.ConflictFinding, len(findings))
	for i, f := range findings {
		if (f.Disposition == artifact.ConflictExempt || f.Disposition == artifact.ConflictNoConflict) && f.TargetRef != "" {
			if o, ok := owners[f.TargetRef]; ok {
				f.RoutedOwners = append([]string(nil), o...)
			}
		}
		out[i] = f
	}
	return out
}

// decisionDigestInput mirrors verify.go's digestInput for the
// decision-conflict report: the computed section's finding identity plus
// the sweep's own pinned provenance (ADR corpus digest, decisions
// scanned) — Disposition/Note/RoutedOwners are human/derived state,
// excluded from the hash, the same principle ComputeDigest applies.
type decisionDigestInput struct {
	Covers           string                `json:"covers"`
	ComputedFindings []findingIdentityOnly `json:"computed_findings"`
	ADRCorpusDigest  string                `json:"adr_corpus_digest"`
	DecisionsScanned []string              `json:"decisions_scanned"`
}

// ComputeDecisionDigest hashes the decision-conflict report's computed
// section content, mirroring verify.go's ComputeDigest formula
// (canonjson.Digest, spec/shared-homes ac-2) so both reports share one
// digest convention.
func ComputeDecisionDigest(covers string, computedFindings []artifact.ConflictFinding, adrCorpusDigest string, decisionsScanned []string) (string, error) {
	in := decisionDigestInput{Covers: covers, ADRCorpusDigest: adrCorpusDigest, DecisionsScanned: decisionsScanned}
	for _, f := range computedFindings {
		in.ComputedFindings = append(in.ComputedFindings, findingIdentityOnly{ID: f.ID, Kind: string(f.Kind), Text: f.Text})
	}
	digest, err := canonjson.Digest(in)
	if err != nil {
		return "", fmt.Errorf("align: marshaling decision-conflict digest input: %w", err)
	}
	return digest, nil
}

// DecisionGateStatuses computes the report's two status labels — computed
// (StatusProven once every computed finding is dispositioned, i.e. every
// declared edge resolved; "" otherwise, never a bare pass) and judged
// (StatusFoundAndDispositioned once at least one real, non-absence judged
// finding exists and every judged finding — real and the synthetic
// absence finding alike — is dispositioned; StatusDisclosedUnprovenComplete
// when the sweep found no real conflicts at all, including a fully-skipped
// sweep whose only judged finding is the synthetic absence finding,
// dispositioned or not; "" when real conflicts were found but are not yet
// fully dispositioned — the table names three END states, not an interim
// one, so this package invents no fourth label; see GateStatus's own doc
// comment).
//
// A documented reading of 03's table (a judgment call — the spec names
// three status values without spelling out which report state maps to
// which combination; see the phase report): "proven" and the judged pair
// are read as answering two SEPARATE questions (the computed half's own
// completeness, and the judged half's own completeness-of-disposition),
// not as three mutually exclusive values of one field — directly mirroring
// 03's own framing, "reusing the incumbent verifiability split" between
// computed (reproducible) and judged (never reproducible, only disclosed).
func DecisionGateStatuses(fm *artifact.DecisionConflictFrontmatter) (computed, judged GateStatus) {
	computedOK := true
	for _, f := range fm.Findings {
		if f.Kind != artifact.FindingComputed {
			continue
		}
		if !f.Dispositioned() {
			computedOK = false
		}
	}
	if computedOK {
		computed = StatusProven
	}

	var real []artifact.ConflictFinding
	allJudgedDispositioned := true
	for _, f := range fm.Findings {
		if f.Kind != artifact.FindingJudged {
			continue
		}
		if !f.Dispositioned() {
			allJudgedDispositioned = false
		}
		if f.ID != DecisionAbsenceFindingID {
			real = append(real, f)
		}
	}
	switch {
	case len(real) == 0:
		judged = StatusDisclosedUnprovenComplete
	case allJudgedDispositioned:
		judged = StatusFoundAndDispositioned
	default:
		judged = "" // real conflicts found, not yet fully dispositioned
	}
	return computed, judged
}

// DecisionReviewReady implements 03's spec-MR merge-blocking condition:
// "All declared conflicts resolved and all judged findings dispositioned"
// — every finding, computed and judged alike, must be dispositioned.
// Returns the sorted ids of every undispositioned finding as the reason
// list (empty when ok).
func DecisionReviewReady(fm *artifact.DecisionConflictFrontmatter) (ok bool, undispositioned []string) {
	for _, f := range fm.Findings {
		if !f.Dispositioned() {
			undispositioned = append(undispositioned, f.ID)
		}
	}
	sort.Strings(undispositioned)
	return len(undispositioned) == 0, undispositioned
}
