---
id: obligation/verification-extractor--ac-2--static
kind: obligation
title: "Truth regeneration execs flowmap graph through the existing upstream seam, extended with scope"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Truth regeneration execs flowmap graph through the existing upstream seam, extended with scope

The static evidence must show the exact call site (naming the function)
that builds an `internal/upstream.Request{Bin: "flowmap", Subcommand:
"graph", ...}` with an `-entry <scope>` flag appended only when a scope
string is non-empty, running it through the existing `Runner` interface
(never a second, parallel exec path), and decoding the result with the
existing `upstream.DecodeGraph`/`RunGraph` — no new JSON schema, no
reimplementation of graph decoding. If `RunGraph` itself is extended
in place (rather than a new function added), the evidence must show the
extension keeps the unscoped call sites (that already exist elsewhere in
this codebase) passing an empty scope and behaving identically to today.
