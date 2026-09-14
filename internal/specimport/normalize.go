package specimport

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// nativeCoverageTarget is the pseudo-target label a native-format primary's
// single whole-document coverage interval carries. It is not a real Field
// target: native content is not decomposed into problem/outcome/ac-N spans
// at all (strict frontmatter decode owns that), so this exists purely so
// the interval's Disposition can honestly read "mapped" — the opposite of
// retained-only — rather than a real field-target claim.
//
// Semantic decision (see the lane report): native primary coverage reports
// one mapped interval spanning the whole selected primary. The
// alternative considered was reporting no Coverage entry at all for
// native's primary; "mapped" was chosen because native content is
// unambiguously canonical spec content, never sidecar-only retained
// material, and Coverage's disposition vocabulary has no third option for
// "this content is authoritative but not target-decomposed."
const nativeCoverageTarget = "native"

// Normalize performs pure source selection, mechanical structural
// recognition or profile application, explicit-mapping overlay and byte-
// coverage accounting (spec-import-contract.md, "Shared internal
// interfaces": "Normalize(Request) (Plan, error) performs pure source
// selection/mapping"). It calls Request.Validate unconditionally — even
// for a Request built directly as a Go struct literal — so no caller can
// reach source-derived logic without passing the same gate a decoded
// request does.
//
// The same source bytes, format/options and target always normalize to
// the same Plan: Normalize makes no network, model, filesystem or clock
// call, and every map it builds is walked in a fixed, declared order
// (statements, then objects in source/explicit order) rather than Go map
// iteration order.
func Normalize(req Request) (Plan, error) {
	if err := req.Validate(); err != nil {
		return Plan{}, err
	}

	selectedBySourceID := make(map[string][]byte, len(req.Sources))
	snapshots := make([]Snapshot, 0, len(req.Sources))
	for _, s := range req.Sources {
		selected, err := selectLineRange(s.Data, s.StartLine, s.EndLine)
		if err != nil {
			// Validate already checked this; defensive, unreachable in
			// practice, but Normalize never trusts prior validation alone.
			return Plan{}, fmt.Errorf("%w: sources: %v", ErrInvalidSource, err)
		}
		selectedBySourceID[s.ID] = selected
		snapshots = append(snapshots, Snapshot{
			ID:             s.ID,
			Label:          s.Label,
			Data:           s.Data,
			OriginalDigest: sha256Hex(s.Data),
			Digest:         sha256Hex(selected),
			StartLine:      s.StartLine,
			EndLine:        s.EndLine,
		})
	}
	primarySelected := selectedBySourceID[req.Primary]

	var baseline []Field
	var findings []Finding
	var native []byte

	switch req.Format {
	case FormatNative:
		n, err := normalizeNative(req, primarySelected)
		if err != nil {
			return Plan{}, err
		}
		native = n

	case FormatMarkdownV1:
		result, err := recognizeMarkdown(req.Primary, primarySelected)
		if err != nil {
			return Plan{}, err
		}
		baseline = result.fields
		findings = append(findings, result.findings...)

	case FormatF13Reference:
		fields, f13findings, err := applyF13Profile(req.Primary, primarySelected)
		if err != nil {
			return Plan{}, err
		}
		baseline = fields
		findings = append(findings, f13findings...)

	case FormatManualV1:
		// manual-v1 supplies no automatic mappings at all
		// (spec-import-contract.md): baseline stays empty.
	}

	fields, err := reconcileFields(req, baseline, selectedBySourceID)
	if err != nil {
		return Plan{}, err
	}
	findings = append(findings, missingEvidenceFindings(fields)...)
	findings = suppressResolvedSourceIDFindings(findings, req.Mappings)
	if req.Format == FormatManualV1 {
		findings = append(findings, manualMissingStatementFindings(fields)...)
	}

	mappedBySource := make(map[string][]mappedSpan, len(req.Sources))
	for _, f := range fields {
		for _, sp := range f.Spans {
			mappedBySource[sp.SourceID] = append(mappedBySource[sp.SourceID], mappedSpan{start: sp.Start, end: sp.End, target: f.Target})
		}
	}
	if req.Format == FormatNative {
		mappedBySource[req.Primary] = []mappedSpan{{start: 0, end: len(primarySelected), target: nativeCoverageTarget}}
	}

	coverages := make([]Coverage, 0, len(snapshots))
	for _, snap := range snapshots {
		selected := selectedBySourceID[snap.ID]
		cov := buildCoverage(snap.ID, len(selected), mappedBySource[snap.ID], req.RetainUnmapped)
		if !req.RetainUnmapped && cov.UnresolvedBytes > 0 {
			findings = append(findings, Finding{
				Code:     FindingUnresolvedCoverage,
				Target:   snap.ID,
				Message:  fmt.Sprintf("source %q has %d unmapped byte(s) and retain_unmapped is false; creation is blocked until every selected byte is mapped or explicitly retained", snap.ID, cov.UnresolvedBytes),
				Blocking: true,
			})
		}
		coverages = append(coverages, cov)
	}

	return Plan{Sources: snapshots, Fields: fields, Coverage: coverages, Findings: findings, Native: native}, nil
}

// manualMissingStatementFindings reports problem/outcome as missing
// whenever manual-v1 (which performs no automatic recognition of any
// kind) leaves them unmapped. markdown-v1 and f13-reference-v1 report
// this themselves, format-specifically, because they can distinguish "no
// such heading was found" from "found but ambiguous/empty" — a
// distinction manual-v1 has no concept of at all.
func manualMissingStatementFindings(fields []Field) []Finding {
	has := make(map[string]bool, len(fields))
	for _, f := range fields {
		has[f.Target] = true
	}
	var findings []Finding
	for _, key := range []string{"problem", "outcome"} {
		if !has[key] {
			findings = append(findings, Finding{
				Code:     FindingMissingStatement,
				Target:   key,
				Message:  fmt.Sprintf("manual-v1 performs no automatic recognition; no mapping supplied the %s statement", key),
				Blocking: true,
			})
		}
	}
	return findings
}

// normalizeNative strict-decodes a native primary's frontmatter and checks
// it structurally against Request.Target, returning the primary's selected
// bytes unchanged on success (spec-import-contract.md, "Candidate and
// validation": "Native mode requires one selected native primary whose
// valid ID/class/title/story match Target, status absent or draft, and no
// frozen/supersession/lifecycle-migration metadata").
//
// This is deliberately a SMALL slice of native validation: anchors
// (ResolveObjectAnchors), evidence-kind/model-compatibility and project
// checks all require the shared candidate/lint seam Task 2 owns, and are
// never duplicated here (spec-import-contract.md: "no parallel canonical
// spec parser"; plan.md Task 2 owns "existing class template/scaffold and
// typed splice operations").
func normalizeNative(req Request, selectedPrimary []byte) ([]byte, error) {
	frontmatter, _, err := artifact.SplitFrontmatter(selectedPrimary)
	if err != nil {
		return nil, fmt.Errorf("%w: native primary: %v", ErrInvalidSource, err)
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("%w: native primary: %v", ErrInvalidRequest, err)
	}

	wantID := "spec/" + req.Target.Slug
	if spec.ID != wantID {
		return nil, fmt.Errorf("%w: native primary id %q does not match target slug %q", ErrInvalidRequest, spec.ID, wantID)
	}
	if string(spec.Class) != req.Target.Class {
		return nil, fmt.Errorf("%w: native primary class %q does not match target class %q", ErrInvalidRequest, spec.Class, req.Target.Class)
	}
	if spec.Title != req.Target.Title {
		return nil, fmt.Errorf("%w: native primary title %q does not match target title %q", ErrInvalidRequest, spec.Title, req.Target.Title)
	}
	if spec.Story != req.Target.Story {
		return nil, fmt.Errorf("%w: native primary story %q does not match target story %q", ErrInvalidRequest, spec.Story, req.Target.Story)
	}
	if spec.Status != "" && spec.Status != artifact.Status("draft") {
		return nil, fmt.Errorf("%w: native primary status %q must be absent or draft", ErrInvalidRequest, spec.Status)
	}
	if spec.Frozen != nil {
		return nil, fmt.Errorf("%w: native primary carries frozen authority; only a draft/proposed native spec may be imported", ErrInvalidRequest)
	}
	if spec.Supersession != nil {
		return nil, fmt.Errorf("%w: native primary carries supersession metadata; only a draft/proposed native spec may be imported", ErrInvalidRequest)
	}

	return selectedPrimary, nil
}
