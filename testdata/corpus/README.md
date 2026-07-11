# testdata/corpus

Sample store exercising every artifact kind and status (PLAN.md §4). Two
zones of content:

- **Committed zone** (`.verdi/`, minus `data/`): built into a deterministic
  git repository by `internal/fixturegit`, per the layer assignments in
  `layers.txt`. The resulting commit SHAs are golden-pinned in
  `internal/corpus/corpus_test.go` — every pinned ref, `frozen:` stamp, and
  record `commit`/`covers` field elsewhere in this corpus names one of
  those three SHAs (layer 1, 2, or 3's head).
- **Mutable/derived zone** (`mutable/`, `derived/`): standalone fixtures,
  never routed through fixturegit and never git-tracked in the real store
  (VL-013). `internal/corpus/corpus_test.go` decodes them directly off
  disk.

Coverage:

- component spec active (`store-layout-notes`) + superseded
  (`legacy-cache-policy`, stays in `specs/active/` per 02's Q9 note)
- feature spec draft (`new-feature-x`)
- feature spec accepted-pending-build (`stale-decline`) with `story:`,
  `context:`, `impacts:`, `declares:`, four ACs spanning all four evidence
  kinds, and a `dispositions:` block covering all three disposition values
- archived closed quartet (`loan-refi-2023`: spec + board.json + rollup.json
  + deviation-report.md, all frozen at the same commit)
- ADR chain: proposed (`0003`) / accepted (`0002`, supersedes `0001`) /
  superseded (`0001`)
- diagram (active)
- attestation, waiver active + expired
- conflict open / superseded / dismissed
- links exercising all nine 02 §Link taxonomy types (see file comments)
- annotations JSONL: one targeted, one board-only, one `agent-task`
- live (unfrozen) board state JSON
- a canned `derived/` bundle: `verdi.evidence/v1` records at two distinct
  commits, one `ci`-provenance and one `local`-provenance

Regenerating the golden SHAs (only needed if corpus content changes): see
the "build once, bake in" procedure described in PLAN.md §4 and the
`goldenHeads` comment in `internal/corpus/corpus_test.go`. In short: edit
layer *N*'s files, build with fixturegit to learn layer *N*'s new SHA,
substitute that SHA into every later layer/file that pins it, repeat for
each subsequent layer, then update `goldenHeads`.
