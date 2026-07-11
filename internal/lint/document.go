package lint

import "github.com/OWNER/verdi/internal/artifact"

// Document is one committed-zone artifact file, tolerantly decoded: even a
// file that fails VL-001 appears here (DecodeErr set, every other field
// zero) so the walk always produces a complete inventory — VL-002's
// duplicate-ref and VL-007's unknown-entry checks need to see every file
// regardless of whether it decodes.
type Document struct {
	// Kind is the artifact kind implied by the file's location: "spec",
	// "adr", "diagram", "attestation", "waiver", or "conflict".
	Kind string
	// Path is the file's absolute filesystem path.
	Path string
	// RelPath is Path relative to the store root, slash-separated (e.g.
	// ".verdi/adr/0001-outbox-events.md") — used for Finding.Path and for
	// VL-002's path-derivation checks.
	RelPath string
	// Grandfathered is true when this document sits under
	// .verdi/specs/archive/ and Options.GrandfatherArchive is set (OQ-3):
	// VL-001..VL-006 must skip it.
	Grandfathered bool

	// DecodeErr is non-nil when artifact.SplitFrontmatter or
	// artifact.DecodeStrict failed — VL-001's finding. Every field below is
	// zero when DecodeErr != nil.
	DecodeErr error

	// Base is the common frontmatter, valid whenever DecodeErr == nil.
	Base artifact.Base
	// Status is the raw, kind-scoped status string.
	Status string
	// Body is the markdown body (post-frontmatter).
	Body string

	// Exactly one of the following is non-nil, matching Kind, whenever
	// DecodeErr == nil — the raw decoded struct, for rules that need
	// kind-specific fields (e.g. VL-005/006/014 need Spec).
	Spec        *artifact.SpecFrontmatter
	ADR         *artifact.ADRFrontmatter
	Diagram     *artifact.DiagramFrontmatter
	Attestation *artifact.AttestationFrontmatter
	Waiver      *artifact.WaiverFrontmatter
	Conflict    *artifact.ConflictFrontmatter
}
