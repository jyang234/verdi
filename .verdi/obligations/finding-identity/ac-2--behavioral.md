---
id: obligation/finding-identity--ac-2--behavioral
kind: obligation
title: "identity.go's Kind+ID+Text rule is unchanged byte-for-byte; stable-slug escalation shows both texts; carried-from is digest-excluded and omitempty"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/finding-identity" }
frozen: { at: 2026-07-20, commit: fb6fb43180469f29545ca99f4d649930222b91a0 }
---
# identity.go's Kind+ID+Text rule is unchanged byte-for-byte; stable-slug escalation shows both texts; carried-from is digest-excluded and omitempty

The behavioral evidence must show three separate proofs. First, the
existing holds-to-violated negative test in `internal/align/
identity_test.go` must keep passing completely unmodified — the test
file's own diff for this change must not touch that test's assertions,
proving the computed/conflict identity path is untouched. Second, a new
escalation-under-stable-slug case: a canned judge emits a low-confidence
(e.g. 0.35) cosmetic-sounding finding at a given slug, it is
dispositioned, then a later regeneration emits a high-confidence (e.g.
0.93) finding describing a real regression at the *identical* slug — the
test must assert the resulting candidate surfaces BOTH texts (never
silently inheriting or hiding the earlier, wrong-in-hindsight ruling).
Third, a frozen-report fixture test proving `VerifyDigest` succeeds
unchanged on an archive predating the `carried-from` field, and a
round-trip test proving a confirmed reaffirmation's `carried-from:
<covers-sha>` is written on `internal/artifact/deviation.go`'s type,
excluded from the digest computation, and `omitempty` on decode. Green in
CI's test step, as part of `make verify`.
