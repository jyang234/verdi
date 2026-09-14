package specimport

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// isEvidenceOnlyMapping reports whether m has the evidence-only shape
// (spec-import-contract.md: "no SourceID, Text, Transform or nonzero
// offsets"). Request.Validate already enforces this shape's internal
// consistency; this helper just re-derives the same predicate for
// Normalize's content-dependent processing.
func isEvidenceOnlyMapping(m Mapping) bool {
	return m.SourceID == "" && m.Text == nil && m.Transform == "" && m.Start == 0 && m.End == 0 && len(m.Evidence) > 0
}

// reconcileFields overlays req.Mappings, in request order, onto the
// automatically-extracted baseline fields, producing the final ordered
// Field list (spec-import-contract.md, "Deterministic structural
// mapping" / "Map order: statements, then objects in source/explicit
// insertion order; no map-iteration dependence"). selectedBySourceID
// supplies each source's SELECTED bytes (post StartLine/EndLine
// resolution) for source-backed mapping application; every offset a
// Mapping declares is validated against that source's selected bytes
// here, never trusted from the request alone.
func reconcileFields(req Request, baseline []Field, selectedBySourceID map[string][]byte) ([]Field, error) {
	order := make([]string, 0, len(baseline)+len(req.Mappings))
	byTarget := make(map[string]Field, len(baseline)+len(req.Mappings))
	for _, f := range baseline {
		order = append(order, f.Target)
		byTarget[f.Target] = f
	}

	// One structural parse per source that an explicit list-item mapping
	// actually names, reused across that source's mappings.
	itemsBySourceID := make(map[string][]bulletItem)
	listItemsOf := func(sourceID string, selected []byte) ([]bulletItem, error) {
		if items, ok := itemsBySourceID[sourceID]; ok {
			return items, nil
		}
		items, err := supportedBulletItems(selected)
		if err != nil {
			return nil, err
		}
		itemsBySourceID[sourceID] = items
		return items, nil
	}

	for _, m := range req.Mappings {
		switch {
		case isEvidenceOnlyMapping(m):
			existing, ok := byTarget[m.Target]
			if !ok || !strings.HasPrefix(m.Target, "ac-") {
				return nil, fmt.Errorf("%w: evidence-only mapping targets %q, which is not an existing automatic acceptance criterion", ErrInvalidRequest, m.Target)
			}
			existing.Evidence = append([]string(nil), m.Evidence...)
			byTarget[m.Target] = existing

		case m.SourceID != "":
			selected, ok := selectedBySourceID[m.SourceID]
			if !ok {
				return nil, fmt.Errorf("%w: mapping targets unknown source_id %q", ErrInvalidRequest, m.SourceID)
			}
			if m.Start < 0 || m.End > len(selected) || m.End < m.Start {
				return nil, fmt.Errorf("%w: mapping span [%d,%d) is out of range for source %q (%d selected bytes)", ErrInvalidSource, m.Start, m.End, m.SourceID, len(selected))
			}
			if !utf8RuneBoundary(selected, m.Start) || !utf8RuneBoundary(selected, m.End) {
				return nil, fmt.Errorf("%w: mapping span [%d,%d) does not land on a UTF-8 rune boundary in source %q", ErrInvalidSource, m.Start, m.End, m.SourceID)
			}
			var transformed string
			if m.Transform == TransformListItem {
				items, err := listItemsOf(m.SourceID, selected)
				if err != nil {
					return nil, err
				}
				item, err := resolveBulletItem(items, m)
				if err != nil {
					return nil, err
				}
				transformed = item.text
			} else {
				t, err := applyTransform(m.Transform, selected[m.Start:m.End])
				if err != nil {
					return nil, err
				}
				transformed = t
			}
			origin, text := OriginCopiedSource, transformed
			if m.Text != nil && *m.Text != transformed {
				origin, text = OriginUserEditedSrc, *m.Text
			}
			if _, existed := byTarget[m.Target]; !existed {
				order = append(order, m.Target)
			}
			byTarget[m.Target] = Field{
				Target:   m.Target,
				Text:     text,
				Origin:   origin,
				Spans:    []Span{{SourceID: m.SourceID, Start: m.Start, End: m.End, Transform: m.Transform}},
				Evidence: append([]string(nil), m.Evidence...),
			}

		case m.Text != nil:
			if _, existed := byTarget[m.Target]; !existed {
				order = append(order, m.Target)
			}
			byTarget[m.Target] = Field{
				Target:   m.Target,
				Text:     *m.Text,
				Origin:   OriginUserAdded,
				Evidence: append([]string(nil), m.Evidence...),
			}

		default:
			// Request.Validate already refuses any mapping matching none
			// of the three shapes; unreachable in practice.
			return nil, fmt.Errorf("%w: mapping for %q matches no recognized shape", ErrInvalidRequest, m.Target)
		}
	}

	fields := make([]Field, 0, len(order))
	for _, target := range order {
		fields = append(fields, byTarget[target])
	}
	return fields, nil
}

// resolveBulletItem binds an explicit list-item Mapping to a real direct
// bullet list item of its own source (spec-import-contract.md: "list-item
// is valid only for an actual supported direct list item span"). The
// mapping's span must name exactly one item — either its content span (the
// span automatic extraction records) or that same item including its own
// bullet marker, which is what the "strip only the bullet marker and its
// following space" transform describes. Arbitrary prose, a fenced block, an
// ordered item, a nested item, a partial selection and a whole-source
// selection all fail, so none of them can be published as a copied list
// item. The returned text comes from the shared extraction, so the declared
// transform is reproducible across the automatic and explicit paths.
func resolveBulletItem(items []bulletItem, m Mapping) (bulletItem, error) {
	for _, item := range items {
		if spansItem(m, item) {
			return item, nil
		}
	}
	return bulletItem{}, fmt.Errorf("%w: mapping for %q uses the list-item transform over [%d,%d) of source %q, which is not a supported direct bullet list item span", ErrInvalidRequest, m.Target, m.Start, m.End, m.SourceID)
}

// applyTransform applies one of the three byte-local Mapping.Transform
// values to raw, the exact selected bytes named by a source-backed
// Mapping's span (spec-import-contract.md, "Deterministic structural
// mapping"). list-item is not byte-local: it is resolved against the
// source's real list structure by resolveBulletItem, so it is refused here
// rather than silently falling back to a marker strip over arbitrary bytes.
func applyTransform(transform string, raw []byte) (string, error) {
	switch transform {
	case TransformIdentity:
		return string(raw), nil
	case TransformTrimBlankLines:
		start, end := trimBodyRange(raw, 0, len(raw))
		return string(raw[start:end]), nil
	case TransformCollapseWS:
		return collapseWhitespace(raw), nil
	case TransformListItem:
		return "", fmt.Errorf("%w: the list-item transform is resolved against the source's real list structure, never applied to arbitrary bytes", ErrInvalidRequest)
	default:
		return "", fmt.Errorf("%w: unknown transform %q", ErrInvalidRequest, transform)
	}
}

var whitespaceRunRe = regexp.MustCompile(`\s+`)

// collapseWhitespace collapses every run of whitespace to one ASCII space
// and trims the result, with no word or punctuation change — the F13
// reference profile's declared transform (mechanical-field-map.json:
// "collapse source whitespace runs to one ASCII space; no word or
// punctuation changes"), reused here for an explicit collapse-whitespace
// Mapping.
func collapseWhitespace(raw []byte) string {
	collapsed := whitespaceRunRe.ReplaceAll(raw, []byte(" "))
	return string(bytes.TrimSpace(collapsed))
}

// utf8RuneBoundary reports whether i is a valid UTF-8 rune boundary within
// b (spec-import-contract.md: "must land on rune boundaries"): the start
// or end of b, or a byte that is not itself a UTF-8 continuation byte.
func utf8RuneBoundary(b []byte, i int) bool {
	if i <= 0 || i >= len(b) {
		return i == 0 || i == len(b)
	}
	return utf8.RuneStart(b[i])
}

// emptyFieldFindings reports one blocking empty-field Finding per final
// Field whose resolved value is empty or blank (spec-import-contract.md:
// "Duplicate aliases for one field, empty fields, unsupported nesting or
// multiple targets are reported"). Automatic recognition already reports an
// empty SECTION before it can become a Field; this is the same rule applied
// to what an explicit mapping actually resolved to, since a present mapping
// target is not a resolved value. It is computed once, after every
// automatic and explicit Field is finalized, so it covers statements and
// objects alike. Task 2 still performs all canonical requiredness checks.
func emptyFieldFindings(fields []Field) []Finding {
	var findings []Finding
	for _, f := range fields {
		if nonBlankUTF8(f.Text) {
			continue
		}
		findings = append(findings, Finding{
			Code:     FindingEmptyField,
			Target:   f.Target,
			Message:  fmt.Sprintf("%s resolved to an empty value; a present mapping target is not a resolved value", f.Target),
			Blocking: true,
		})
	}
	return findings
}

// missingEvidenceFindings reports one missing-evidence Finding per
// acceptance-criterion Field with no declared Evidence
// (spec-import-contract.md: "The preview has a missing-evidence finding
// until an explicit Mapping supplies kinds"). This is computed once,
// after every automatic and explicit Field is finalized, so it applies
// uniformly to markdown-v1 and f13-reference-v1 acceptance criteria alike
// rather than being duplicated per format.
func missingEvidenceFindings(fields []Field) []Finding {
	var findings []Finding
	for _, f := range fields {
		if strings.HasPrefix(f.Target, "ac-") && len(f.Evidence) == 0 {
			findings = append(findings, Finding{
				Code:     FindingMissingEvidence,
				Target:   f.Target,
				Message:  fmt.Sprintf("%s has no evidence kind declared; an explicit mapping must supply one before creation", f.Target),
				Blocking: true,
			})
		}
	}
	return findings
}

// resolveSourceIDFindings removes a source-id-requires-mapping Finding only
// when an explicit Mapping names that source-declared id AND selects the
// item that declared it, in the source that declared it (the closed
// contract review's residual 4: "An explicit mapping to the source-declared
// ID OVER THAT ITEM resolves the blocker").
//
// Target equality alone is not resolution. Without the span binding, a
// spanless user-added mapping cleared the blocker and published invented
// text under the source's own declared identifier with no span at all,
// while the source's real item was disposed of as ordinary retained-only —
// exactly the "silently present a generated ID as the source's identity"
// outcome the finding exists to prevent. An unrelated item, a partial
// selection, a whole-source selection and the same id in a different source
// are all equally insufficient.
func resolveSourceIDFindings(findings []Finding, mappings []Mapping, blocked []blockedSourceID) []Finding {
	resolved := make(map[string]bool, len(blocked))
	for _, b := range blocked {
		for _, m := range mappings {
			if m.Target == b.id && m.SourceID == b.sourceID && spansItem(m, b.item) {
				resolved[b.id] = true
			}
		}
	}
	out := findings[:0:0]
	for _, f := range findings {
		if f.Code == FindingSourceIDRequiresMap && resolved[f.Target] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// demotableGapCodes are the findings that report a missing or unusable
// VALUE for one specific field target. Every other code reports something
// an explicit mapping does not resolve — a peer/higher heading
// (multiple-targets), a primary with no heading at all
// (unsupported-structure), an unresolved object SECTION (ambiguous-field on
// a section display name, which is never a Mapping target), a
// source-declared id still needing its own item mapping, or bytes with no
// disposition (unresolved-coverage on a source id) — so none is ever
// demoted, whatever its target string happens to be.
var demotableGapCodes = map[string]bool{
	FindingMissingStatement: true,
	FindingAmbiguousField:   true,
	FindingEmptyField:       true,
}

// demoteResolvedFieldGaps turns a field-value gap into a truthful
// NONBLOCKING disclosure when an explicit Mapping actually resolved that
// exact target to a nonblank value.
//
// Without this, Normalize's own output contradicted itself: a Plan carried
// a resolved Fields entry for problem and, at the same time, a blocking
// finding asserting problem was missing, so an explicitly mapped statement
// could never become ready (spec-import-contract.md: "Explicit text/span
// Mappings override the automatic mapping for the same target"). The source
// fact is preserved rather than erased — the source really does lack, or
// ambiguously declare, that section — it simply no longer blocks a value
// the user supplied. A mapping that resolved to nothing demotes nothing.
func demoteResolvedFieldGaps(findings []Finding, fields []Field, mappings []Mapping) []Finding {
	resolved := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		if isEvidenceOnlyMapping(m) {
			continue // evidence-only changes evidence, never a field's value.
		}
		resolved[m.Target] = true
	}
	for _, f := range fields {
		if !nonBlankUTF8(f.Text) {
			delete(resolved, f.Target)
		}
	}

	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		if resolved[f.Target] && demotableGapCodes[f.Code] {
			f.Blocking = false
			f.Message += "; an explicit mapping supplied this value, so this source-structure gap is disclosed rather than blocking"
		}
		out = append(out, f)
	}
	return out
}

// spansItem reports whether m's span names exactly item: either its content
// span (the span automatic extraction records) or that same item including
// its own bullet marker.
func spansItem(m Mapping, item bulletItem) bool {
	return m.End == item.end && (m.Start == item.start || m.Start == item.markerStart)
}
