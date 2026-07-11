// Package render holds the markdown-and-code rendering machinery shared by
// every HTML-producing surface (internal/dex's static site, internal/
// workbench's server-rendered pages): goldmark (GFM) plus chroma
// syntax-highlighting at render time, and the I-5 dispositions-block table
// renderer ("workbench and dex render the block as a table so humans never
// read raw YAML" — 05 §Workbench, I-5).
//
// This package used to be two copies, one inside internal/dex (phase 12)
// and a second about to be written inside internal/workbench (phase 10).
// CLAUDE.md's "anything used by two or more packages lives in a shared
// internal/ package" rule applies directly, so the phase-12 code moved
// here verbatim (same goldmark/chroma configuration, same output bytes —
// dex's golden-fragment tests are the regression guard) and both surfaces
// now depend on this package instead of each other.
package render
