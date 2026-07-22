---
id: obligation/obligation-seam--ac-5--behavioral
kind: obligation
title: "verdi obligation author creates, regenerates, and refuses on an already-frozen obligation"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-seam" }
frozen: { at: 2026-07-21, commit: af0edd77237b6c52cffda3bc344c020ff5fad58e }
---
# verdi obligation author creates, regenerates, and refuses on an already-frozen obligation

The behavioral evidence must show built-binary tests (`cmd/verdi/
obligation_test.go`) covering three cases. CREATE: given a declared
`(story, ac)` pair and a kind the AC actually declares, with no obligation
yet on disk, `verdi obligation author <story-ref> <ac-id> <kind>` exits 0
and writes a decodable obligation at the convention path. REGENERATE:
running it again against the SAME pair, now that a file exists there but
is not yet reachable from `merge-base(HEAD, default branch)` (still
local/unmerged), exits 0 and overwrites the file — proving pre-freeze
authoring is never a one-shot, "already exists" refusal the way the
board's own sticky-graduate action is. REFUSE-ON-FROZEN: given a fixture
where the target obligation IS reachable from the merge-base (a commit on
the configured default branch already carries a decodable obligation at
that exact path — mirroring how `internal/lint/vl010_test.go` constructs
its own frozen-file fixtures), the verb exits 2, names the path in its
message, and leaves the working tree untouched — proving the frozen
check mirrors VL-010's own scoping (reachability from the merge-base, not
mere presence on the current branch) rather than a weaker or stronger
predicate invented independently. A fourth, negative case must show an
unknown kind, an unresolvable story ref, or an AC the story does not
declare each refuse in plain language naming the offending input, writing
nothing.
