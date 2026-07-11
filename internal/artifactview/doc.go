// Package artifactview holds the per-kind frontmatter projection every
// artifact-rendering surface needs beyond internal/index.Entry's generic
// view: owners, frozen/provenance stamps, and every feature-spec-only
// field (class, story, declared boundaries, acceptance criteria, the I-5
// dispositions block). internal/index.Entry deliberately stays generic
// (one struct for every committed-zone kind); this package re-decodes an
// artifact's frontmatter through the same internal/artifact seam to get
// the typed view a page's anatomy renders.
//
// Moved here (from internal/dex, phase 12) once internal/workbench (phase
// 10) needed the identical per-kind decode dispatch — CLAUDE.md: "anything
// used by two or more packages lives in a shared internal/ package."
package artifactview
