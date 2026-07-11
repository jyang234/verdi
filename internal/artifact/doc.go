// Package artifact is the contract package (02-artifact-contract.md): ref
// grammar, common and per-kind frontmatter schemas, and the record schemas
// (verdi.annotation/v1, verdi.board/v1, verdi.evidence/v1, verdi.rollup/v1,
// verdi.deviation/v1). Every consumer of the store's semantics — lint,
// index, fold, workbench, MCP, dex — depends on this package rather than
// re-deriving its own reading of the contract.
//
// internal/artifact is the sole importer of gopkg.in/yaml.v3 in this module
// (CLAUDE.md: "single import seam"; PLAN.md I-1). Frontmatter is decoded
// exclusively through DecodeStrict in decode.go, which enforces both
// KnownFields(true) and the restricted YAML dialect (no anchors, aliases,
// or custom tags). Record schemas that live in plain JSON files
// (board.json, verdicts.json, rollup.json) are decoded with the stdlib
// encoding/json package instead — see decode.go's DecodeStrictJSON.
package artifact
