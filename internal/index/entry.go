package index

import "github.com/jyang234/verdi/internal/artifact"

// Entry is one indexed artifact — either a committed-zone kind (spec, adr,
// diagram, attestation, waiver, conflict, obligation) or an index-minted
// external ref (02 §External refs: Kind "external").
type Entry struct {
	// Ref is the canonical ref string: "<kind>/<name>" for committed-zone
	// kinds (taken verbatim from the decoded frontmatter's `id:` field —
	// 01 §D2: "The canonical id lives in frontmatter"), or
	// "svc/<service>/<artifact>[/<name>]" for external refs (02 §External
	// refs).
	Ref string
	// Kind is one of "spec", "adr", "diagram", "attestation", "waiver",
	// "conflict", "obligation", or "external".
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
	// DiagramClass is the diagram kind's class discriminator
	// (02 §Diagram proposals): "" for an incumbent diagram — and for
	// every non-diagram kind — or artifact.DiagramClassProposal for a
	// future-state proposal. Carried so body-rendering surfaces (dex
	// artifact pages, the workbench corpus page and reference peek) can
	// dispatch the diagram tier badge at internal/render's shared seam
	// (spec/illustrative-class ac-2) without re-reading frontmatter.
	DiagramClass string
	// ObjectIDs is the set of frontmatter-declared object ids a SPEC entry
	// carries (its acceptance criteria, constraints, decisions, and open
	// questions — artifact.DeclaredObjectIDs), computed once during the walk
	// where the frontmatter is already decoded. nil for every non-spec kind
	// and for external refs. Carried so a fragment-bearing ref
	// (spec/<name>#<object-id>) can be resolved to the AC/object level
	// without re-reading the target's file — the same DeclaredObjectIDs set
	// lint's VL-003 uses to resolve fragments.
	ObjectIDs map[string]bool
}
