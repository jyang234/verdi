package specimport

import "fmt"

// f13PrimarySHA256 is the pinned SHA-256 of the selected primary bytes
// this named, versioned reference profile is bound to
// (spec-import-contract.md: "f13-reference-v1 binds the selected primary
// SHA-256 7c6d95d4... and the eight exact selectors in the reviewed
// mechanical-field-map.json. This is a named, versioned reference profile,
// not a recognizer for all ATC plans. Changed primary bytes refuse that
// profile with guidance to use manual mapping or labeled Markdown.").
// The primary bytes and this digest are unchanged by spec/uat-round-1
// ac-7/dc-9; only the selector COUNT the quoted sentence above names is
// historical — dc-9 revised the reviewed mechanical-field-map.json from
// eight selectors (several spanning more than one claim or starting
// mid-clause) to twelve, one per complete claim. See f13Selectors below.
const f13PrimarySHA256 = "7c6d95d4aa516cf682e6d4a33d1c260f1c861d938d46559241a7c764bffca577"

// f13Selector is one fixed, reviewed selector from
// docs/superpowers/proposals/2026-09-14-spec-import-f13/mechanical-field-map.json.
type f13Selector struct {
	target string
	start  int
	end    int
}

// f13Selectors are the twelve exact selectors the reviewed field map
// declares, shipped as fixed package data interpreted by fixed code
// (spec-import-contract.md: "Profiles are shipped data interpreted by
// fixed code, never uploaded executables"). Values are pinned directly
// from mechanical-field-map.json's "cards" array;
// TestF13Selectors_MatchMechanicalFieldMapJSON (profile_f13_test.go)
// decodes that JSON file directly at test time and checks these
// selectors, and profile_f13_test.go's own f13PinnedCards, against it —
// id, byte range, sha256, and display_text — so the three hand-copies
// cannot silently diverge.
//
// Revised under spec/uat-round-1 ac-7/dc-9: the source states its
// criteria as one comma-joined sentence led by the instruction word
// "Prove", so a selector covers exactly one complete claim (its own
// subject and verb) and starts and ends at a clause or sentence boundary,
// never joining two claims and never beginning mid-clause. "Prove " and
// the imperative "Enumerate every allowed edge and representative
// forbidden edges." sentence stay retained-only; ac-1..ac-9 below cover
// the nine Prove-sentence claims in source order; ac-10 is the
// (unchanged, already whole) blocking-finding sentence; the former
// two-sentence ac-8 splits into one selector per sentence (ac-11, ac-12).
var f13Selectors = []f13Selector{
	{"ac-1", 1556, 1572},
	{"ac-2", 1574, 1600},
	{"ac-3", 1602, 1633},
	{"ac-4", 1635, 1681},
	{"ac-5", 1683, 1724},
	{"ac-6", 1726, 1775},
	{"ac-7", 1777, 1850},
	{"ac-8", 1852, 1891},
	{"ac-9", 1897, 1924},
	{"ac-10", 1981, 2115},
	{"ac-11", 2116, 2157},
	{"ac-12", 2158, 2200},
}

// applyF13Profile applies the f13-reference-v1 fixed profile to the
// primary source's selected bytes. It refuses with ErrUnsupportedFormat
// unless selected's SHA-256 exactly equals the pinned primary this
// profile is bound to; every selector's byte range must be well-formed
// only for that one pinned length, so applying it to any other bytes
// would otherwise silently select the wrong content.
//
// Problem/outcome are permanently absent from this profile
// (spec-import-contract.md, "F13 prototype and validation": "the selected
// primary source has no explicit named pair"), so this always reports
// both as missing-statement — never format-generic detection, since F13
// never has a Problem/Outcome heading to look for in the first place.
func applyF13Profile(sourceID string, selected []byte) ([]Field, []Finding, error) {
	if sha256Hex(selected) != f13PrimarySHA256 {
		return nil, nil, fmt.Errorf("%w: f13-reference-v1 is bound to primary sha256:%s; this selection's bytes do not match, use manual-v1 or a labeled markdown-v1 mapping instead", ErrUnsupportedFormat, f13PrimarySHA256)
	}
	fields := make([]Field, 0, len(f13Selectors))
	for _, sel := range f13Selectors {
		raw := selected[sel.start:sel.end]
		fields = append(fields, Field{
			Target: sel.target,
			Text:   collapseWhitespace(raw),
			Origin: OriginCopiedSource,
			Spans: []Span{{
				SourceID:  sourceID,
				Start:     sel.start,
				End:       sel.end,
				Transform: TransformCollapseWS,
			}},
		})
	}
	findings := []Finding{
		{Code: FindingMissingStatement, Target: "problem", Message: "the f13-reference-v1 profile's bound primary has no explicitly labeled Problem section", Blocking: true},
		{Code: FindingMissingStatement, Target: "outcome", Message: "the f13-reference-v1 profile's bound primary has no explicitly labeled Outcome section", Blocking: true},
	}
	return fields, findings, nil
}
