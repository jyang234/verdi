package artifact

import (
	"fmt"
	"strings"
)

// ObligationFrontmatter is the frontmatter schema for kind "obligation"
// (spec/obligation-artifact DC-1): a first-class evidence-obligation
// artifact, decoded through internal/artifact exactly like an attestation —
// markdown frontmatter + prose body, no `schema:` line (that is for JSON
// artifacts), frozen unconditionally ("existence is the record", mirroring
// AttestationFrontmatter's own posture — see attestation.go). ForKind is
// the one EvidenceKind this obligation states what that evidence must
// specifically show for.
//
// The obligation's single `verifies` edge (carried in the embedded Base's
// Links, per DC-1: "reuse the existing link types" rather than inventing a
// new frontmatter key) names the WHOLE STORY SPEC it backs — a bare
// spec/<story-name> ref with no object fragment — exactly as an
// attestation's own `verifies` edge does (attestation.go; e.g.
// `ref: spec/stale-decline`). The acceptance criterion itself is NOT carried
// on that edge: like an attestation, the obligation encodes its AC in its
// own id and on-disk path (obligation/<story-slug>--<ac-id>--<for-kind>, at
// .verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md), so a fragment
// on the verifies ref would be redundant — and is rejected, since 02 §Link
// taxonomy's closed spec-object edge vocabulary already forbids a `verifies`
// edge from targeting a fragment (the base Link.Validate, common.go).
//
// Whether that whole-spec target is actually a STORY (as opposed to a
// feature) that genuinely declares the id's own <ac-id> as one of its
// acceptance criteria needs the corpus/index to resolve, which a bare
// frontmatter decode cannot see. VL-019 (internal/lint) owns that
// classification (spec/obligation-artifact AC-2/DC-3). This type's own
// Validate is deliberately narrower: it confirms exactly one verifies link
// is present and its ref is well-formed (a whole-spec ref, per the base
// link vocabulary), nothing about what class of thing it resolves to.
type ObligationFrontmatter struct {
	Base    `yaml:",inline"`
	ForKind EvidenceKind `yaml:"for_kind"`
}

// DecodeObligation strict-decodes and validates obligation frontmatter.
func DecodeObligation(data []byte) (*ObligationFrontmatter, error) {
	var fm ObligationFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields (including, via validateBase, that the
// id parses as an obligation ref whose compound name matches
// obligationNameRe's <story-slug>--<ac-id>--<for-kind> shape — a malformed
// id fails here, and every link's ref shape is checked by the base
// Link.Validate(), so a `verifies` edge carrying an object fragment is
// rejected there — obligations verify a WHOLE story spec, never a
// fragment), that ForKind is a known evidence kind and agrees with the id's
// own <for-kind> segment (DC-2's id/for_kind agreement), and that exactly
// one `verifies` link is present (a missing verifies link, or one of any
// other type, is rejected). Frozen is required unconditionally (DC-1:
// "existence is the record", attestation's own posture) — see requireFrozen.
//
// Path/id agreement (DC-2's other half: on-disk home
// .verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md) is NOT
// checked here: DecodeObligation, like DecodeAttestation, takes only raw
// frontmatter bytes and has no path to compare against. That half is
// internal/lint's VL-011 (extended for the obligation kind alongside its
// existing attestation/waiver/reaffirmation coverage), which walks the
// committed zone and does have both the id and the file's real path.
//
// Similarly NOT checked here: whether the whole-spec verifies target is a
// STORY (vs a feature or an unresolvable spec) that genuinely declares the
// id's own <ac-id> as one of its acceptance criteria — that classification
// needs the corpus/index and is VL-019's job (spec/obligation-artifact
// AC-2).
func (fm ObligationFrontmatter) Validate() error {
	if err := fm.validateBase(KindObligation); err != nil {
		return err
	}

	if !validEvidenceKinds[fm.ForKind] {
		return fmt.Errorf("artifact: obligation for_kind %q is not a known evidence kind", fm.ForKind)
	}

	ref, err := ParseRef(fm.ID)
	if err != nil {
		// Unreachable in practice: validateBase already parsed and
		// validated fm.ID above. Fail closed rather than panic below if
		// that invariant is ever broken by a future refactor.
		return fmt.Errorf("artifact: id: %w", err)
	}
	_, _, idForKind, ok := SplitObligationName(ref.Name)
	if !ok {
		// Unreachable in practice: validateBase's Ref.Validate call already
		// enforced obligationNameRe's three-segment shape for KindObligation
		// above. Guarded rather than assumed for the same reason.
		return fmt.Errorf("artifact: obligation id %q does not split into <story-slug>--<ac-id>--<for-kind>", fm.ID)
	}
	if idForKind != string(fm.ForKind) {
		return fmt.Errorf("artifact: obligation id %q names for_kind segment %q, but frontmatter for_kind is %q (spec/obligation-artifact DC-2: id/for_kind agreement)", fm.ID, idForKind, fm.ForKind)
	}

	if len(fm.Links) != 1 || fm.Links[0].Type != LinkVerifies {
		return fmt.Errorf("artifact: obligation must carry exactly one links entry of type verifies, got %d", len(fm.Links))
	}

	return requireFrozen(fm.Frozen, true, "obligation", "")
}

// SplitObligationName splits an obligation ref's compound name
// "<story-slug>--<ac-id>--<for-kind>" into its three parts
// (spec/obligation-artifact DC-2: id and on-disk path are two views of the
// same (story, ac, for-kind) triple). ok is false when name does not have
// exactly three "--"-joined segments. Exported so internal/lint's VL-011
// (path/id agreement) shares this exact parse rather than re-deriving it —
// callers that already know name comes from a successfully-Validate()'d Ref
// (obligationNameRe already enforced the shape at decode time) use ok only
// as a defensive guard, mirroring VL-011's own "shape already enforced at
// decode" posture for attestation/waiver/reaffirmation's two-segment split.
func SplitObligationName(name string) (storySlug, acID, forKind string, ok bool) {
	parts := strings.Split(name, "--")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
