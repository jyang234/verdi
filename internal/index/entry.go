package index

import "github.com/OWNER/verdi/internal/artifact"

// Entry is one indexed artifact — either a committed-zone kind (spec, adr,
// diagram, attestation, waiver, conflict) or an index-minted external ref
// (02 §External refs: Kind "external").
type Entry struct {
	// Ref is the canonical ref string: "<kind>/<name>" for committed-zone
	// kinds (taken verbatim from the decoded frontmatter's `id:` field —
	// 01 §D2: "The canonical id lives in frontmatter"), or
	// "svc/<service>/<artifact>[/<name>]" for external refs (02 §External
	// refs).
	Ref string
	// Kind is one of "spec", "adr", "diagram", "attestation", "waiver",
	// "conflict", or "external".
	Kind string
	// Title is the artifact's title (committed-zone) or a synthesized
	// human-readable label (external).
	Title string
	// Status is the per-kind status string, or "" where the kind has none
	// (attestation) or the entry is external.
	Status string
	// Path is the absolute filesystem path backing this entry: the
	// artifact's own file for committed-zone kinds, or the discovered
	// upstream file for external refs (boundary-contract.json,
	// .flowmap.yaml for a minted obligation, or the OpenAPI doc).
	Path string
	// Body is the markdown body (committed-zone kinds) or raw file content
	// / a synthesized label (external), used only for full-text search.
	Body string
	// Links are the artifact's outgoing typed links (02 §Link taxonomy).
	// Always empty for external entries — index-minted refs carry no
	// frontmatter of their own.
	Links []artifact.Link
}
