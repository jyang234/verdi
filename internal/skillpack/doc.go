// Package skillpack renders and verifies the four verdi skills (specify,
// clarify, plan, tasks) that coding-agent harnesses read from a consuming
// repository (spec/spec-documents ac-7, ac-8; dc-4: "Skills are
// projections, not authority"). Templates are embedded in the binary so a
// skill can never drift from the engine that serves it; every render is
// stamped with the skillpack engine digest, the template digest, and the
// render commit, and marked as generated; Check recomputes each skill from
// the running binary and reports drift or absence (R-W3-2: the render
// commit is provenance and is masked from the comparison).
//
// Both hosts receive the same Agent Skills format (frontmatter `name` +
// `description`, Markdown body): Claude Code at .claude/skills/<name>/
// SKILL.md and Codex at .agents/skills/<name>/SKILL.md (SI-201, the
// codex-prompt-conventions spike). The bytes differ only in the marker's
// host= value.
//
// Every byte written is canonical (no wall clock, username, absolute
// path, or random identifier) except the declared render-commit stamp.
package skillpack
