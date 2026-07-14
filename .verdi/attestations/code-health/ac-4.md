---
id: attestation/code-health--ac-4
kind: attestation
title: "AC-4 attested: witnessed honesty gaps fail loud"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/code-health }
frozen: { at: 2026-07-14, commit: 49b779af64f9584f55cd3f0940e6c38fda544ed8 }
---
# AC-4 outcome attestation

Operator attests (round 6, 2026-07-14): cascadecheck surfaces an
unreadable spec as exit 2 instead of a clean pass; the four sentinel
comparisons use errors.Is; runtimeprobe's transcription semantic is
stated and pinned by a fail-verdict test; mcpserve refuses a typo'd
tool argument by name under a ledgered posture and logs dropped socket
connections; boardio's doc states the caller-lock contract; the stale
fourteen-rules counts match the registry; and VL-019 carries its
ratified 02 rule-table row with the 08-revision-notes entry. Proven by
spec/fail-loud's archived closure (jira:VERDI-QH-1).
