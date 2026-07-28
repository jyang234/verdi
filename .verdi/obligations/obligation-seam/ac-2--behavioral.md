---
id: obligation/obligation-seam--ac-2--behavioral
kind: obligation
title: "the backstop skips already-covered pairs and never overwrites a pre-existing obligation, keyed on the decode-based coverage predicate"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-seam" }
frozen: { at: 2026-07-21, commit: af0edd77237b6c52cffda3bc344c020ff5fad58e }
---
# the backstop skips already-covered pairs and never overwrites a pre-existing obligation, keyed on the decode-based coverage predicate

The behavioral evidence must show a built-binary test driving a story spec
declaring at least two `(ac, kind)` pairs, one already backed by a
hand-authored obligation file carrying distinctive, recognizable prose and
one with no obligation at all. After `verdi accept` succeeds, the test
must assert: the pre-existing obligation's bytes are byte-for-byte
identical to what the test wrote before accept ran (the distinctive prose
survives verbatim — never replaced by a scaffolded stub); the missing
pair now has a freshly-scaffolded stub; and the accept commit's own diff
names only the newly-scaffolded path, never the pre-existing one (proving
the pre-existing file was never even re-written, not merely re-written
identically).

A second case must prove the predicate is decode-based, not
`os.Stat`-based, on the SKIP side: an obligation file whose `for_kind`
does not match the file's own declared kind segment, or that otherwise
fails `artifact.DecodeObligation`, must never be silently counted as
covering its pair merely because a file sits at the path — reusing
`internal/evidence.Obligations`'s own existing fail-closed contract (it
already surfaces a decode failure as an error rather than treating it as
absence) rather than a second, hand-rolled predicate that could disagree
with it.

A third case must prove the write side is equally conservative: given a
declared pair whose convention path already holds a file that fails to
decode as an obligation, `verdi accept` must refuse operationally (exit
2, naming the path and the decode failure) rather than silently
overwriting whatever is there — a malformed file is exactly the case
where guessing (overwrite, or silently treat as covered) is the dishonest
choice; refusing and asking the operator to resolve it via `verdi
obligation author` or by hand is the only fail-closed option once
something already sits at that path in a shape the shared decoder cannot
read.
