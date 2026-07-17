package evidence

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// AttestationPath returns the on-disk path an attestation for
// (storySlug, acID) lives at under storeRoot's attestations/ directory
// (attestations/<storySlug>/<acID>.md, I-6). It resolves through
// store.AttestationPath — the single .verdi-layout assembler (ADJ-71) — which
// AttestationExists, LoadAttestationState, and any external disclosure that
// must NAME this path (spec/close-preflight ac-1/dc-4) all share, so none
// hand-derives a second, possibly-drifting copy of it. Passing an empty
// storeRoot yields the store-relative display form
// ("`.verdi/attestations/<storySlug>/<acID>.md`", filepath.Join drops an empty
// leading element) a disclosure prints instead of a temp-dir- or
// checkout-rooted absolute path.
func AttestationPath(storeRoot, storySlug, acID string) string {
	return store.AttestationPath(storeRoot, storySlug, acID)
}

// AttestationExists reports whether an attestation file exists for
// (storySlug, acID) under storeRoot's attestations/ directory
// (attestations/<storySlug>/<acID>.md). 03 §Evidence kinds is explicit
// that the attestation kind is "Satisfied by: attestation file exists for
// (story, AC)" and 02 §Kind registry says existence is the record (no
// status field at all) — so this checks existence only, deliberately not
// decoding or validating the file's frontmatter: a malformed attestation
// is still an attestation for the fold's purposes (VL-001 lint is where a
// malformed one gets caught, not the fold).
func AttestationExists(storeRoot, storySlug, acID string) (bool, error) {
	path := AttestationPath(storeRoot, storySlug, acID)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("evidence: checking attestation %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("evidence: attestation path %s is a directory, not a file", path)
	}
	return true, nil
}

// UnauthoredAttestationMarker is the fixed, exported sentinel a scaffold
// `verdi attest` writes carries in its body until an operator authors a
// claim (spec/attest-helper dc-3): a single HTML-comment line, invisible
// under markdown rendering, trivially greppable, and vanishingly unlikely
// to collide with genuine first-person claim prose. Defined once so the
// scaffold writer (cmd/verdi's attest verb) and every fold reader
// (this package, internal/wallbadge) share one literal rather than a
// copy-pasted string (CLAUDE.md).
const UnauthoredAttestationMarker = "<!-- verdi:attestation-unauthored -->"

// AttestationState is an attestation's three-way state at the fold's own
// expected path (spec/attest-helper dc-3):
//
//   - AttestationAbsent: no file at the path at all — nothing has ever
//     been written here.
//   - AttestationUnauthored: the file exists and its whole byte content
//     still contains UnauthoredAttestationMarker — a `verdi attest`
//     scaffold nobody has authored a claim into yet.
//   - AttestationAuthored: the file exists and the marker is gone — a
//     human has replaced it with their own first-person claim.
//
// Only AttestationAuthored satisfies the fold (parent spec/
// closure-ergonomics dc-2: "the scaffold is not foldable until the
// operator has authored the claim"): AttestationUnauthored collapses to
// exactly the same not-satisfied outcome AttestationAbsent already
// produces everywhere the fold computes evidenced/pending/no-signal. The
// only difference is disclosure — a caller such as the sibling
// close-preflight story's own surface can render "scaffolded but not yet
// authored" more precisely than an undifferentiated absent (co-3).
type AttestationState int

const (
	AttestationAbsent AttestationState = iota
	AttestationUnauthored
	AttestationAuthored
)

// String renders the state for disclosure/debugging.
func (s AttestationState) String() string {
	switch s {
	case AttestationAbsent:
		return "absent"
	case AttestationUnauthored:
		return "unauthored"
	case AttestationAuthored:
		return "authored"
	default:
		return fmt.Sprintf("AttestationState(%d)", int(s))
	}
}

// LoadAttestationState reports the three-way AttestationState of the
// attestation file for (storySlug, acID) under storeRoot's attestations/
// directory — the same path AttestationExists checks (spec/attest-helper
// dc-3). Detection is a raw substring check over the file's whole byte
// content — deliberately not a frontmatter/body split, since the marker
// can only ever appear where the scaffold writer put it (in the body) —
// so no coupling to frontmatter-parsing code is needed.
//
// AttestationExists is left untouched, same existence-only semantics, for
// any caller that genuinely only needs raw existence; this is a sibling,
// not a replacement (its own doc comment already disclaims decoding
// content — this function does not weaken that contract).
func LoadAttestationState(storeRoot, storySlug, acID string) (AttestationState, error) {
	path := AttestationPath(storeRoot, storySlug, acID)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AttestationAbsent, nil
		}
		return AttestationAbsent, fmt.Errorf("evidence: loading attestation state %s: %w", path, err)
	}
	if info.IsDir() {
		return AttestationAbsent, fmt.Errorf("evidence: attestation path %s is a directory, not a file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AttestationAbsent, fmt.Errorf("evidence: loading attestation state %s: %w", path, err)
	}
	if strings.Contains(string(data), UnauthoredAttestationMarker) {
		return AttestationUnauthored, nil
	}
	return AttestationAuthored, nil
}

// AttestationScaffold bundles RenderAttestationScaffold's inputs (spec/
// attest-helper ac-1/dc-2): every field here is structure the caller
// (cmd/verdi's attest verb) already resolved or derived —
// RenderAttestationScaffold itself does no resolution and no I/O, and
// never invents a claim (parent spec/closure-ergonomics dc-2).
type AttestationScaffold struct {
	// StorySlug is store.RefSlug(story.Story) — the <storySlug> half of
	// the compound id/path (I-6, the D6-16/D6-18-corrected convention).
	StorySlug string
	// ACID is the acceptance-criterion id the attestation is for (e.g.
	// "ac-2").
	ACID string
	// StoryRefArg is the raw <story-ref> the operator typed at the CLI
	// (either form storyresolve.Resolve accepts — a scheme-prefixed story
	// ref or a spec ref) — echoed into the title and instructional body
	// prose so the scaffold reads back exactly what was invoked.
	StoryRefArg string
	// VerifiesRef is the resolved story spec's own canonical ref (e.g.
	// "spec/borrower-update-api") — the verifies edge's target.
	VerifiesRef string
	// Owners is copied VERBATIM from the resolved story spec's own
	// owners: (dc-2: never invented, never an [unassigned] placeholder).
	Owners []string
	// Frozen is the frozen stamp: At is today (YYYY-MM-DD), Commit is git
	// HEAD at scaffold time (dc-2, ADJ-30: a convenience the operator
	// updates to the tree they actually verified against when authoring
	// the claim — legally mutable until this file's first commit).
	Frozen artifact.Frozen
}

// attestationScaffoldBody is the fixed instructional prose every scaffold
// carries (spec/attest-helper AC-1's own worked example, verbatim): the
// unauthored marker, then prose naming the (story-ref, ac-id) pair and
// explaining the frozen.commit convenience (dc-2, ADJ-30). The two %s
// verbs both take (StoryRefArg, ACID) — the prose names the pair twice,
// exactly as the frozen contract's own example does.
const attestationScaffoldBody = "%s\n" +
	"This attestation was scaffolded by `verdi attest` for %s %s\n" +
	"and has not been authored. Replace this entire paragraph, and delete the\n" +
	"marker comment above, with your own first-person account of what you\n" +
	"verified, how, and why this acceptance criterion is satisfied. Until the\n" +
	"marker above is removed, this file folds as absent, with disclosure — it\n" +
	"is not evidence of anything.\n" +
	"\n" +
	"The `frozen.commit` stamped above is a convenience: it was pre-filled with\n" +
	"the repository HEAD when this scaffold was written. By the store's\n" +
	"attestation convention that field names the tree your claim was verified\n" +
	"against — not this file's own commit — so set it to the exact commit you\n" +
	"actually reviewed when you author your claim. The stamp is yours to\n" +
	"correct: nothing here is frozen until this file's first commit (VL-010\n" +
	"binds only committed frozen artifacts), so updating it in this same\n" +
	"authoring pass is always legitimate.\n"

// RenderAttestationScaffold renders a complete, strict-decodable, unauthored
// attestation file's bytes (spec/attest-helper ac-1): frontmatter carrying
// structure only — id, kind, a mechanically-derived identifier-shaped
// title, owners copied verbatim, schema, a single bare verifies edge, a
// frozen stamp — and a body whose entire content is
// UnauthoredAttestationMarker followed by fixed instructional prose. Never
// generates, defaults, or templates a single claim-shaped sentence (parent
// dc-2) — every word beyond the fixed instructional prose is structure
// derived from identifiers already on hand.
//
// Hand-rendered, never yaml.Marshal'd (the module-wide posture:
// internal/align/render.go, internal/workbench/commitdesign.go's
// renderObligation), so field order and flow-mapping style are pinned
// exactly, byte for byte, rather than left to a library's own formatting.
func RenderAttestationScaffold(in AttestationScaffold) string {
	id := fmt.Sprintf("attestation/%s--%s", in.StorySlug, in.ACID)
	title := fmt.Sprintf("unauthored attestation scaffold: %s %s", in.StoryRefArg, in.ACID)

	quotedOwners := make([]string, len(in.Owners))
	for i, o := range in.Owners {
		quotedOwners[i] = artifact.YAMLDoubleQuote(o)
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	b.WriteString("kind: attestation\n")
	fmt.Fprintf(&b, "title: %s\n", artifact.YAMLDoubleQuote(title))
	fmt.Fprintf(&b, "owners: [%s]\n", strings.Join(quotedOwners, ", "))
	b.WriteString("schema: verdi.attestation/v1\n")
	b.WriteString("links:\n")
	fmt.Fprintf(&b, "  - { type: verifies, ref: %s }\n", artifact.YAMLDoubleQuote(in.VerifiesRef))
	fmt.Fprintf(&b, "frozen: { at: %s, commit: %s }\n", in.Frozen.At, in.Frozen.Commit)
	b.WriteString("---\n")
	fmt.Fprintf(&b, attestationScaffoldBody, UnauthoredAttestationMarker, in.StoryRefArg, in.ACID)
	return b.String()
}
