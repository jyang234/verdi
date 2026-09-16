// Package specimport implements Task 1 of the mechanical spec-import
// contract (docs/superpowers/specs/2026-09-14-spec-import-contract.md):
// pure, deterministic normalization of explicitly selected native or
// Markdown source bytes into the shared Request/Plan shapes that later
// tasks compose into a candidate spec, preview and publish.
//
// This package performs no network, model, provider or Git I/O, and it
// never writes a file. ReadSource is the sole filesystem entry point, and
// it only reads a single caller-selected regular file under a caller-
// selected import root. Everything else — Request decoding, Markdown
// structural recognition, mechanical profile application, explicit-mapping
// overlay and byte-coverage accounting — is a pure function of its
// arguments: the same bytes, options and target always normalize to the
// same Plan.
//
// Candidate composition (template/scaffold rendering, splice mutation,
// shared lint), Git publication, import-record persistence and the CLI/
// browser adapters are later tasks (2-4) and are deliberately absent here.
package specimport
