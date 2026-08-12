package store

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// NonSpecArtifactPath is SI-92's one shared canonical store path table
// (docs/superpowers/invention-ledger.md SI-92: "non-spec kinds use the
// shared canonical store path table"): given a closed-registry artifact
// kind other than spec and its ref's Name, it returns the exact
// store-relative path (root joined with "", the same empty-root display/
// identity convention ObligationPath already documents and
// internal/contextcompile's 7F obligation resolver already relies on) a
// strict decoder reads bytes from.
//
// spec is deliberately excluded: unlike every other kind, a spec ref
// resolves to exactly one matching active- OR archive-zone path searched
// in the PINNED tree (SI-92), not a single fixed path a name alone
// determines — that search belongs to the caller (context-compiler's
// declared-context resolver), not this table.
//
// name is the ref's Name field exactly as artifact.Ref carries it — for
// the compound kinds (attestation, waiver, reaffirmation, obligation) this
// is the whole "<story>--<ac-id>" / "<story-slug>--<ac-id>--<for-kind>"
// string; NonSpecArtifactPath splits it itself rather than requiring the
// caller to pre-split, so a (kind, name) pair from a parsed artifact.Ref is
// always enough on its own. This function does not re-validate name's
// per-kind shape (artifact.Ref.Validate already does, on decode/parse) —
// only its kind.
func NonSpecArtifactPath(kind artifact.Kind, name string) (string, error) {
	switch kind {
	case artifact.KindADR:
		return ADRPath("", name), nil
	case artifact.KindDiagram:
		return DiagramPath("", name), nil
	case artifact.KindConflict:
		return ConflictPath("", name), nil
	case artifact.KindAttestation:
		storySlug, acID, ok := splitCompound2(name)
		if !ok {
			return "", fmt.Errorf("store: attestation name %q is not <story>--<ac-id>", name)
		}
		return AttestationPath("", storySlug, acID), nil
	case artifact.KindWaiver:
		storySlug, acID, ok := splitCompound2(name)
		if !ok {
			return "", fmt.Errorf("store: waiver name %q is not <story>--<ac-id>", name)
		}
		return WaiverPath("", storySlug, acID), nil
	case artifact.KindReaffirmation:
		storySlug, objectID, ok := splitCompound2(name)
		if !ok {
			return "", fmt.Errorf("store: reaffirmation name %q is not <story>--<object-id>", name)
		}
		return ReaffirmationPath("", storySlug, objectID), nil
	case artifact.KindObligation:
		specName, acID, kindSeg, ok := splitCompound3(name)
		if !ok {
			return "", fmt.Errorf("store: obligation name %q is not <story-slug>--<ac-id>--<for-kind>", name)
		}
		return ObligationPath("", specName, acID, kindSeg), nil
	case artifact.KindSpec:
		return "", fmt.Errorf("store: NonSpecArtifactPath does not resolve kind %q; spec resolves via an active/archive pinned-tree zone search", kind)
	default:
		return "", fmt.Errorf("store: NonSpecArtifactPath: unknown artifact kind %q", kind)
	}
}

// splitCompound2 splits a "<a>--<b>" two-segment compound name (the
// attestation/waiver/reaffirmation shape, I-6/R4-I-4) on its single "--"
// separator.
func splitCompound2(name string) (a, b string, ok bool) {
	a, b, found := strings.Cut(name, "--")
	if !found || a == "" || b == "" || strings.Contains(b, "--") {
		return "", "", false
	}
	return a, b, true
}

// splitCompound3 splits a "<a>--<b>--<c>" three-segment compound name (the
// obligation shape, spec/obligation-artifact DC-2) on its two "--"
// separators.
func splitCompound3(name string) (a, b, c string, ok bool) {
	first, rest, found := strings.Cut(name, "--")
	if !found || first == "" {
		return "", "", "", false
	}
	second, third, found := strings.Cut(rest, "--")
	if !found || second == "" || third == "" || strings.Contains(third, "--") {
		return "", "", "", false
	}
	return first, second, third, true
}
