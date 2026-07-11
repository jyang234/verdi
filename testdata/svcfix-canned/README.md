# svcfix-canned

Canned upstream JSON captured once from the pinned toolchain (spike S1's
binaries, `flowmap`/`groundwork` built at commit `cd38b1a`) against
`testdata/svcfix`'s real, compiled fixture module — not hand-authored. Every
file here is exactly what the real binaries printed or wrote, adapted only
by choosing which branch-state edits to capture (PLAN.md §4).

- `graph.json` — `flowmap graph -stamp deadbeef testdata/svcfix`: the base
  graph, obligation `audit-before-publish` SATISFIED.
- `boundary-contract-base.json` — `flowmap boundary testdata/svcfix`'s
  output at the same commit; byte-identical to the committed
  `testdata/svcfix/.flowmap/boundary-contract.json`.
- `boundary-contract-branch.json` — the same, captured after adding a
  `GET /healthz` route (a real branch-state edit, reverted after capture):
  one route added relative to the base contract.
- `review-structurally-clear.json`, `review-block.json`,
  `review-no-structural-signal.json` — `groundwork review <policy> <base
  graph> <branch graph> -json`, captured against three real branch-state
  edits: adding the healthz route (STRUCTURALLY-CLEAR), a deliberate
  handler→audit layering violation (BLOCK), and a body-only edit with no
  new nodes or edges (NO-STRUCTURAL-SIGNAL).
- `*-unknown-field.json` — one twin per decoder under test (graph, boundary
  contract, review), each the real capture plus one injected top-level
  field, used by strict-decode failure tests (internal/upstream).
- `digests.json` — a sha256 ratchet (schema `verdi.fixture-digests/v1`)
  over every file in this directory except itself and this README;
  `make fixture` verifies it hermetically (no exec, no network). The
  opt-in `make fixture-regen` target re-captures everything from the
  toolchain (via `go run …@pin`, or spike S1's `bin/` if present on
  `$VERDI_S1_BIN`) and recomputes this ratchet; it never runs in
  `make verify`.

`testdata/svcfix/policy.json` was captured the same way, via
`groundwork init` against `graph.json`, and lives with the service fixture
rather than here since it is itself an input to future `groundwork review`
invocations, not an output to strict-decode.
