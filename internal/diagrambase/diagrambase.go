// Package diagrambase is the digest-verified base-recovery seam a derived
// proposal's mechanical before-peek and reset stand on (spec/board-editor
// ac-4, dc-5; spec/diagram-proposals ac-3): recover the pinned base's
// source from git history at derived_from.ref's pinned commit, compute
// sha256 over the base's canonical graph JSON, and return the base bytes
// EXACTLY — or a typed, disclosed error and NO bytes. Both affordances
// are pure functions of the artifact's provenance fields: this package
// keeps no cache, no history, no state of its own.
//
// The source-digest formula (CanonicalGraphDigest) deserves its own
// honesty note. 02 §Diagram proposals defines derived_from.digest as
// "sha256 of the base's canonical graph JSON at that commit" — flowmap's
// own graph — and the verification extractor's stale-base check
// (internal/diagramverify's StaleBase) realizes "at current HEAD" by
// re-running the pinned flowmap CLI. At a HISTORICAL commit that graph
// JSON is not recoverable: 01 §Store layout rules "generated views are
// never committed", and re-running flowmap against a past commit from a
// live checkout is neither hermetic nor a pure function of the provenance
// fields dc-5 requires. ADJ-16 resolves this: peek/reset gate on the
// SEPARATE, optional derived_from.source_digest — the one derivation that
// IS recoverable from the pinned commit alone: the extractor's own
// one-way extraction of the base's committed mermaid body. This package
// defines that source digest as the canonical JSON (internal/canonjson,
// the 02 §Generated artifacts and digests formula) of the node/edge graph
// diagramverify.Parse extracts from the recovered source. It CONSUMES the
// extractor's exported grammar; it never reimplements graph semantics
// (co-3), and any deriving writer that stamps derived_from.source_digest
// for editor use stamps it with this same exported function
// (CanonicalGraphDigest) — derived_from.digest keeps the flowmap
// stale-base semantics and is NOT what these affordances verify against.
// Recorded as this story's disclosed formula decision in its deviation
// report.
package diagrambase

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/diagramverify"
	"github.com/jyang234/verdi/internal/gitx"
)

// NotDerivedError: the artifact carries no derived_from at all — the
// affordances do not exist for a from-scratch proposal (ac-4).
type NotDerivedError struct{}

func (*NotDerivedError) Error() string {
	return "diagrambase: this proposal has no derived_from provenance; before-peek and reset apply to derived proposals only"
}

// UnpinnedRefError: derived_from.ref carries no @commit pin, so there is
// no pinned commit to recover the base from (dc-5's inputs are the
// PINNED provenance fields; an unpinned ref cannot be one).
type UnpinnedRefError struct{ Ref string }

func (e *UnpinnedRefError) Error() string {
	return fmt.Sprintf("diagrambase: derived_from.ref %q is not commit-pinned; the base cannot be recovered from history without a pin", e.Ref)
}

// UnavailableError: the pinned commit or the base's file at it could not
// be read from git history (unresolvable pin, rewritten history, a base
// path absent at that commit).
type UnavailableError struct {
	Ref string
	Err error
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("diagrambase: base %s could not be recovered from git history: %v", e.Ref, e.Err)
}

func (e *UnavailableError) Unwrap() error { return e.Err }

// NoSourceDigestError: the artifact carries derived_from but no
// derived_from.source_digest (ADJ-16). The peek/reset affordances gate on
// source_digest — recomputable from git history alone — not on
// derived_from.digest (which carries the flowmap stale-base semantics and
// cannot be hermetically regenerated at a historical commit). Absent the
// source digest, the affordances render disclosed-unavailable: the state
// is never guessed and never silently gated on the wrong digest.
type NoSourceDigestError struct{ Ref string }

func (e *NoSourceDigestError) Error() string {
	return fmt.Sprintf("diagrambase: base %s carries no derived_from.source_digest; before-peek and reset are disclosed unavailable — they gate on source_digest, recomputable from history, never on derived_from.digest", e.Ref)
}

// DigestMismatchError: the recovered inputs do not hash to the pinned
// derived_from.source_digest — rewritten history, a wrong pin, or
// corrupted provenance. The affordances fail visible and write nothing
// (ac-4: "a wrong base silently peeked or reset would be worse than no
// affordance"); both digests are carried so the disclosure is concrete.
type DigestMismatchError struct {
	Ref    string
	Pinned string // derived_from.source_digest, the claim
	Got    string // the recovered base's actual source digest
}

func (e *DigestMismatchError) Error() string {
	return fmt.Sprintf("diagrambase: digest mismatch for base %s: derived_from.source_digest is %s but the recovered base hashes to %s — refusing to peek or reset from a base that does not verify", e.Ref, e.Pinned, e.Got)
}

// canonicalGraph is the graph-JSON projection CanonicalGraphDigest
// hashes: the extractor grammar's node identities and ordered edge
// pairs, in source order. Field names are part of the digest formula —
// changing them changes every digest — so they are pinned here once.
type canonicalGraph struct {
	Nodes []string        `json:"nodes"`
	Edges []canonicalEdge `json:"edges"`
}

type canonicalEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// CanonicalGraphDigest computes the SOURCE digest (ADJ-16): the sha256
// (canonjson.Digest — the shared 02 §Generated artifacts and digests
// formula) of the canonical graph JSON extracted one-way from a diagram
// body's mermaid source. This is the value derived_from.source_digest
// carries, the digest peek/reset gate on — NOT derived_from.digest, which
// keeps the flowmap stale-base semantics. A pure function of the body
// bytes: same source, same digest, always. See the package doc for why
// the graph is extracted from the source rather than regenerated by
// flowmap at a historical commit.
func CanonicalGraphDigest(body []byte) (string, error) {
	ext := diagramverify.Parse(string(body), nil)
	g := canonicalGraph{Nodes: []string{}, Edges: []canonicalEdge{}}
	for _, n := range ext.Nodes {
		g.Nodes = append(g.Nodes, n.RawID)
	}
	for _, e := range ext.Edges {
		g.Edges = append(g.Edges, canonicalEdge{From: e.From, To: e.To})
	}
	d, err := canonjson.Digest(g)
	if err != nil {
		return "", fmt.Errorf("diagrambase: digesting canonical graph: %w", err)
	}
	return d, nil
}

// Recover reproduces a derived proposal's pinned base from its
// digest-verified inputs (dc-5, ADJ-16): read the base diagram's file
// from git history at derived_from.ref's pinned commit, split off its
// frontmatter, verify sha256 over the body's canonical graph JSON equals
// derived_from.source_digest, and return the base's BODY bytes exactly.
// The gate is source_digest, not derived_from.digest: a derived proposal
// without source_digest yields NoSourceDigestError (disclosed
// unavailable), never a guess. Every failure is one of the typed errors
// above and returns no bytes — the affordances must have nothing to
// render or write from on failure.
func Recover(ctx context.Context, root string, df *artifact.DiagramDerivedFrom) ([]byte, error) {
	if df == nil {
		return nil, &NotDerivedError{}
	}
	if df.SourceDigest == "" {
		return nil, &NoSourceDigestError{Ref: df.Ref}
	}
	ref, err := artifact.ParseRef(df.Ref)
	if err != nil {
		return nil, &UnavailableError{Ref: df.Ref, Err: err}
	}
	if ref.Kind != artifact.KindDiagram {
		return nil, &UnavailableError{Ref: df.Ref, Err: fmt.Errorf("derived_from.ref names a %s, not a diagram", ref.Kind)}
	}
	if !ref.Pinned() {
		return nil, &UnpinnedRefError{Ref: df.Ref}
	}

	raw, err := gitx.Show(ctx, root, ref.Commit, ".verdi/diagrams/"+ref.Name+".mermaid")
	if err != nil {
		return nil, &UnavailableError{Ref: df.Ref, Err: err}
	}
	_, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, &UnavailableError{Ref: df.Ref, Err: err}
	}
	got, err := CanonicalGraphDigest(body)
	if err != nil {
		return nil, &UnavailableError{Ref: df.Ref, Err: err}
	}
	if got != df.SourceDigest {
		return nil, &DigestMismatchError{Ref: df.Ref, Pinned: df.SourceDigest, Got: got}
	}
	return body, nil
}
