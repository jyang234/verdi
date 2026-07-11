// Package index implements the semantics 01 §Scale and 02 §Link taxonomy /
// §External refs own: a walk of the committed zone that decodes every
// artifact through internal/artifact, the in-memory index built from that
// walk plus internal/store's service discovery (external-ref minting for
// boundary contracts, obligations, and OpenAPI docs), backlink inversion of
// the link taxonomy, and a stdlib full-text search over id/title/body.
//
// There is no persistence here: rebuilding is the only cache-miss path
// (01 §Scale envelope: "no database"); internal/store's TreeHash and
// CacheKey are what a later phase uses to decide whether a rebuild is
// needed at all.
package index
