---
id: obligation/obligation-seam--ac-1--behavioral
kind: obligation
title: "accept scaffolds every missing declared obligation before the lint gate runs, stamped preFlipHead, staged into the accept commit"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-seam" }
frozen: { at: 2026-07-21, commit: af0edd77237b6c52cffda3bc344c020ff5fad58e }
---
# accept scaffolds every missing declared obligation before the lint gate runs, stamped preFlipHead, staged into the accept commit

The behavioral evidence must show a built-binary test (mirroring
`cmd/verdi/accept_test.go`'s existing `scaffoldAndDesign` style) that runs
`verdi accept` against a draft story spec declaring one or more `(ac,
kind)` pairs with no obligation anywhere on disk, and asserts: the accept
commit succeeds (exit 0); a decodable obligation file now exists at
`.verdi/obligations/<spec>/<ac>--<kind>.md` for every declared pair; each
scaffolded file's `frozen.commit` equals the CAPTURED pre-flip HEAD (the
commit accept ran against, not the newly-created accept commit itself) and
`frozen.at` equals that commit's own committer date, independently
recomputed via `gitx.CommitDate` rather than compared against wall-clock
`time.Now()`; and the accept commit's own tree diff (`gitx.DiffNameStatus`
between the pre-accept HEAD and the post-accept HEAD) contains every
scaffolded obligation path alongside the spec's own flip — proving the
pairing lands in one atomic commit, never a second, separate write a
retry or a crash between the two could pull apart. A companion test (or
subtest) must further prove the ORDERING: instrument or otherwise observe
that the obligation files exist on disk by the time the store lint runs
(the quartet gate accept.go's `lintQuartetOrRefuse` call performs), not
merely by the time the accept commit lands — the two are different claims
and O-1 requires the first.
