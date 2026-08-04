---
id: conflict/journey-event-receipt-storage-unspecified
kind: conflict
title: "GLG v2 AC-8's immutable journey event receipts have no ratified storage home"
status: open
owners: [platform-team]
links:
  - { type: challenges, ref: spec/guided-lifecycle-governance-v2 }
---
# Conflict: AC-8's immutable event receipts have no ratified storage home

## What is disputed

`spec/guided-lifecycle-governance-v2` AC-8 requires that "every participating
lifecycle command, confirmed journey action, and governed recovery attempt
emits an immutable event receipt" carrying registered action id, target ref,
input-state digest, result class, blocker ids, authority posture, required
governance principal ids, source commit or forge witness, and a declared
provenance stamp — yet the accepted spec leaves the receipt's storage zone,
path grammar, retention, immutability rule, and writer/reader authority
completely unspecified. Yesterday's accepted truth — an "immutable" record
whose durable home is not named anywhere in landed authority — is contested
as incomplete: a naive data-zone home would put an immutable,
authority-adjacent record in a disposable per-checkout zone, and a naive
committed home would conflict with the committed zone's "mutations are MRs"
writer rule for machine-emitted, per-command output.

## Witness

The owner-merged execution-workspace/store-layout authority plan
(`docs/superpowers/plans/2026-08-03-execution-workspace-store-layout-authority.md`,
landed via PR #269) records the gap with file-and-line witnesses: §1 row 7
("GLG spec leaves storage unspecified (CX-17 verbatim)... A data-zone home
would put an immutable authority-adjacent record in a disposable per-checkout
zone, and the bare name `receipts` collides with CI's canonical context
receipts — constraints P-1's decision must honor"); §2 ("Placed by
prerequisite, not by this plan: GLG event receipts (row 7). Immutable,
authority-adjacent records fit neither a disposable data zone nor an
unratified committed home"); §5 ("any path P-1 places outside `data/` roots
gc is ratified for"); §7.2 ("P-1 — GLG journey-event-receipt storage (GLG
unit): zone, path, retention, honoring row 7's constraints... The GLG lane's
own authority PR records the decision's `SI-<n>` ledger entry (citing handle
L-2)"); §10 L-2. The plan assigns the decision to this feature's own lane as
a hard PR-A prerequisite.

## Resolution

Feature supersession (`spec/verdi-evidence-model` §The amendment ladder rung
4): `spec/guided-lifecycle-governance-v3` supersedes
`spec/guided-lifecycle-governance-v2` in one PR carrying the storage
decision the plan's P-1 prerequisite requires, adding dc-27 (data-zone root
`data/journey/`, path grammar `data/journey/event-receipts/<ref-slug>.jsonl`,
retention, immutability, writer/reader authority, and the receipt/telemetry
and receipt-naming separations) while carrying every other v2 object
byte-identical. Cascade fold: zero affected stories — no in-flight or closed
story declares edges into `spec/guided-lifecycle-governance-v2` — so the
single-owner acceptance price applies.
