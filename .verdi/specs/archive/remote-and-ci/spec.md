---
id: spec/remote-and-ci
kind: spec
title: "Remote and CI"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-1
problem: { text: "verdi has never had a remote: no origin, no forge, no CI job producing anything, so `source: ci` evidence — the only kind a gate may consume authoritatively (constitution 4, dc-1) — has never existed for any story here, and the module path is still the `github.com/OWNER/verdi` placeholder across the tree. The trust root true-closure#ac-1 depends on does not exist yet.", anchor: "#problem" }
outcome: { text: "the module identity settles to `github.com/jyang234/verdi` end to end, a `verdi-evidence` CI workflow produces the authoritative derived bundle on the real remote, and `verdi sync` fetches `source: ci` evidence by (ref, commit) — the trust root exercised for real.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the module identity is `github.com/jyang234/verdi` end to end — go.mod, every import, the MCP shims, and .mcp.json — and the tree builds and `make verify` passes under the new path", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "a `verdi-evidence` GitHub Actions workflow assembles and uploads the `data/derived/<ref-slug>/<commit>/` tree (03 §Evidence records) with provenance.source: ci, and runs `make verify` as the CI gate (CI runs exactly `make verify` — trust parity)", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "`verdi sync` fetches the authoritative bundle by (ref, commit) through the forge port and a gate consumes only `source: ci` records produced by the repo's own CI, never local regeneration", evidence: [behavioral, attestation], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/true-closure#ac-1" }
decisions:
  - { id: dc-1, text: "the CI-provenance producer is a `verdi sync --produce` path invoked only by the workflow; its output is authoritative solely because it is fetched from the forge artifact store by (ref, commit), never because the flag was passed — a local `--produce` bundle never reaches a gate", anchor: "#dc-1" }
  - { id: dc-2, text: "the module-path rename is one mechanical change (I-4's one-MR precedent for pin/shim churn): go.mod + all imports + the verdi-mcp/groundwork-mcp shims + .mcp.json, plus verdi.yaml forge:github and the .gitattributes forge token, land together", anchor: "#dc-2" }
constraints:
  - { id: co-1, text: "no network in any test: the producer and forge fetch are exercised hermetically (fixturegit, httptest forge doubles, canned upstream JSON); only the real-remote proof run exercises the live artifact round-trip, disclosed as such", anchor: "#co-1" }
frozen: { at: 2026-07-12, commit: 6b7b6afcf54b2fb6882076455a67a0fae99be435, stub_matched: true }
---
# Remote and CI

## Problem

verdi has never had a remote — no `origin`, no forge, no pull request, and
no CI job producing anything. `source: ci` evidence, the only kind a gate
may consume authoritatively (constitution 4; true-closure dc-1), has never
existed for any story in this store; every gate to date ran on
`source: local` or fixture bundles. And the module path is still the
`github.com/OWNER/verdi` placeholder across the codebase, go.mod, the MCP
shims, and `.mcp.json`. The trust root that true-closure#ac-1 ("a true,
archived closure on authoritative CI-produced evidence alone") depends on
does not exist yet. This story builds it and exercises it for real.

## Outcome

The module identity settles to `github.com/jyang234/verdi` end to end; a
`verdi-evidence` GitHub Actions workflow produces the authoritative
`data/derived/<ref-slug>/<commit>/` bundle (03 §Evidence records) on the
real remote; and `verdi sync` fetches `source: ci` evidence by (ref,
commit) through the forge port. The trust root is not merely wired — it is
observed producing one real authoritative bundle, so everything downstream
(close-verb, runtime-evidence) can honestly claim to consume it.

## AC-1

The module identity is `github.com/jyang234/verdi` end to end. The
placeholder `github.com/OWNER/verdi` is replaced in go.mod and every import
across the tree, in the `verdi-mcp` and `groundwork-mcp` shims, and in
`.mcp.json`; the tree builds and `make verify` passes under the new path.
Evidence: static (no `OWNER` placeholder survives in any import) and
behavioral (the built binary and full gate pass under the settled path).

## AC-2

A `verdi-evidence` GitHub Actions workflow assembles and uploads the
`data/derived/<ref-slug>/<commit>/` tree as its artifact under the fixed
I-8 convention (03 §Evidence records), each record stamped
`provenance.source: ci`, and runs `make verify` as the CI gate — CI runs
exactly `make verify`, trust parity with the local gate. Evidence:
behavioral (the workflow produces a real artifact on a real run) and
attestation (an operator affirms the uploaded tree matches the convention).

## AC-3

`verdi sync` fetches the authoritative bundle by (ref, commit) through the
forge port (the way it already fetches evidence bundles), and a gate
consumes only `source: ci` records — never local regeneration (dc-1). A
locally produced bundle is advisory and never load-bearing in a gate
decision. Evidence: behavioral (an exerciser pulls a real `verdi-evidence`
artifact and confirms `source: ci`) and attestation (an operator affirms no
local record gated).

## DC-1

The CI-provenance producer is a `verdi sync --produce` path invoked only by
the `verdi-evidence` workflow. Its output is authoritative solely because it
is fetched back from the forge artifact store by (ref, commit) — not because
the flag was passed. A local `verdi sync --produce` writes a bundle that no
gate ever consumes (it never enters the committed zone, VL-013, and the gate
path fetches via the forge). This keeps dc-1's "no local shortcut" true even
though the producer binary is the same one an author could run.

## DC-2

The module-path rename lands as one mechanical change, following I-4's
one-MR precedent for pin/shim churn: go.mod, every import, the `verdi-mcp`
and `groundwork-mcp` shims, and `.mcp.json` together, plus `verdi.yaml`'s
`forge: github` (or the auto-detect from the `origin` URL) and the
`.gitattributes` forge-generated token flipped from the GitLab form. Split
across MRs it would leave the tree un-buildable between them; it is one
atomic settle.

## CO-1

No network in any test. The producer, the forge fetch, and the fold are
exercised hermetically — fixturegit stores, httptest forge doubles, canned
upstream JSON (`testdata/svcfix-canned/`) — so `make verify` proves the
mechanism with no network. Only the real-remote proof run exercises the
live artifact round-trip (a real Actions run, a real `verdi sync` pull),
and that step is disclosed as the one thing the hermetic gate structurally
cannot cover.
