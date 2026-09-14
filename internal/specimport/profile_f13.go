package specimport

import "fmt"

// f13PrimarySHA256 is the pinned SHA-256 of the selected primary bytes
// this named, versioned reference profile is bound to
// (spec-import-contract.md: "f13-reference-v1 binds the selected primary
// SHA-256 7c6d95d4... and the eight exact selectors in the reviewed
// mechanical-field-map.json. This is a named, versioned reference profile,
// not a recognizer for all ATC plans. Changed primary bytes refuse that
// profile with guidance to use manual mapping or labeled Markdown.").
const f13PrimarySHA256 = "7c6d95d4aa516cf682e6d4a33d1c260f1c861d938d46559241a7c764bffca577"

// f13Selector is one fixed, reviewed selector from
// docs/superpowers/proposals/2026-09-14-spec-import-f13/mechanical-field-map.json.
type f13Selector struct {
	target string
	start  int
	end    int
}

// f13Selectors are the eight exact selectors the reviewed field map
// declares, shipped as fixed package data interpreted by fixed code
// (spec-import-contract.md: "Profiles are shipped data interpreted by
// fixed code, never uploaded executables"). Values are pinned directly
// from mechanical-field-map.json's "cards" array; profile_f13_test.go
// checks each against the field map's own source_text_sha256/display_text.
var f13Selectors = []f13Selector{
	{"ac-1", 1485, 1549},
	{"ac-2", 1556, 1681},
	{"ac-3", 1683, 1724},
	{"ac-4", 1726, 1775},
	{"ac-5", 1777, 1850},
	{"ac-6", 1852, 1924},
	{"ac-7", 1981, 2115},
	{"ac-8", 2116, 2200},
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
