# Local-Operator Principal Resolution and Human-Fallback Dispositions for the Lifecycle Gate

**Status:** ratified by the owner on 2026-09-05 (all decisions as drafted). Amends the policy-conflict gate authority design
(2026-08-12) §12 by ratification. Authored by the FABLE controller from the owner-approved spike
(branch `agent/lifecycle-gate-spike`, commit 026050b0, report `spike/report.md`) while Codex (owner gate) is
unavailable; the one independent cross-model review owed to Claude-authored authority is disclosed as not yet
performed and is owed to Codex on return.

## 1. Problem

Through the built CLI, a story-class specification's build-phase accepted context can never reach the `pass`
verdict the lifecycle gate (`build start --context-request`) and the sealed execution request both require. A
semantic conflict row is proven only by a legal disposition concluding no-conflict (`deriveSemanticProof`); a
disposition is legal only when its approvals resolve through principal resolutions; and the CLI supplies no
principal resolver ("nil in v1 production"). Judges never prove a row on their own. Consequently every VATC
flight on a real store is refused at runway open (ATC IL-126), and the F12 genuine canary cannot run.

The spike proved, on the exact pinned kernel 5d49ba94 with no verdict override, that two additions suffice: an
opt-in local principal resolver wired at the single provider factory, and a hand-authored `policy-disposition`
artifact whose witness is copied from the kernel's own semantic row. It also corrected a recorded premise: the
disposition match operand is the raw content digest of the target specification, not the manifest digest, and
`policy-disposition` artifacts already live outside the specification (`.verdi/policy/dispositions/`), so
recording one does not move the digest its witness must match. ATC ledger rows IL-108 and IL-126 are corrected
accordingly (§7).

## 2. Decisions

### 2.1 A new trust-source kind, honest about its strength

`identity_trust_sources` gains the closed kind `local-operator`. Its resolver reads the store's Git identity
(`user.email`, falling back to `user.name`, read through one `gitx` primitive that distinguishes absent from
broken configuration) and resolves a claim `{trust_source: <id>, subject: <identity>}` through
`governanceprincipal.Resolver` only. It mints `authenticated` only when the subject equals a subject the
profile's `role_mappings` bind to that source; any other subject resolves `violated-with-witness`; an absent
identity resolves `unproven`. The resolution witness code is the new `local-operator-asserted`, never
`trust-subject-verified`: the evidence is a self-assertion, and the witness says so.

A profile may declare a `local-operator` source only if its class is `solo`; the profile validator refuses every
other class by name. This is the ratified reading of the solo class ("one authenticated principal fills every
role, with the collapsed separation of duties disclosed by the kernel").

### 2.2 The resolver is wired once, opt-in, and disclosed

The resolver is bound at the one production factory that builds the policy-conflict service
(`cmd/verdi/context_conflict.go`), which the lifecycle provider (`build start`, `gate`, `close`) and
`context conflict` share. It resolves only when the resolved profile declares a `local-operator` source;
every other profile receives nil actors and byte-identical output to today. Every report produced with a
local-operator resolution carries the new disclosure code `local-operator-asserted`, so no consumer can read
such a pass as an authenticated-identity pass. The sealed execution path needs no change: it verifies a report
byte-for-byte and never re-evaluates (spike §6).

### 2.3 Disposition authoring from the kernel's own row

A new verb `verdi disposition record` writes one `policy-disposition` artifact from a
`verdi.policy-conflict-report/v1` document: it selects the semantic row by `input_id`, copies `input_id`,
`target_digest`, the claim identities and exemptions verbatim, resolves the scaffold template identity and
digest through `humanartifact.ResolveScaffold`, and fills `conclusion`, `origin` (`human-fallback` when no
judgment is present, `judge-result` otherwise), at least one compensating control, the expiry, and the
approvals (`role` + the principal id the resolver minted) from explicit operands. It refuses a row it cannot
find, a conclusion outside the closed set, a missing compensating control, and any operand that would make
the witness differ from the report. `RenderDisposition` gains multi-claim and `human-fallback` support; no
schema changes. Nothing in this verb evaluates anything: it records a human's ruling over a kernel-printed
witness.

### 2.4 Instruction projection regeneration verb

`verdi context project` wraps `instructionprojection.Generate(root)` and prints the manifests and managed
files it wrote with their digests. Library-only generation forced a throwaway program in every fixture
regeneration to date; the verb removes that.

### 2.5 Threat-model amendment (authority design §12)

§12's sentence "does not … authenticate a local username" becomes: "does not authenticate a local username;
a `solo` profile may declare a `local-operator` source whose resolutions are self-asserted, carry the
`local-operator-asserted` witness, and are disclosed in every report that relies on them. That limitation is
recorded, never silently upgraded." Team, high-assurance, and experimental profiles are unchanged.

### 2.6 Exclusions

No judge configuration; no change to `deriveSemanticProof`, `Authorize`, or the disposition match rule; no
new resolution state; no forge or signed-commit change; no ATC change beyond the ledger corrections and the
separately ratified IL-125.

## 3. Verification

- `internal/gitx`: table test for the config primitive (present, absent, broken, malformed).
- `internal/governanceprincipal`: `local-operator` kind validation (solo only), resolver arms (bound subject
  ⇒ authenticated with the asserted witness; other subject ⇒ violated; absent ⇒ unproven), witness/disclosure
  codes in the closed vocabularies.
- `cmd/verdi`: byte-identity test for a profile without the source; `context conflict` reaching `pass` with
  the `local-operator-asserted` disclosure; `build start --context-request` exit 0 cutting the branch;
  subject mismatch ⇒ `violated-with-witness`, never a favourable default; `disposition record` positive and
  every refusal; `context project` output and digests.
- Fixture: a hermetic store (feature + story + obligation + constitution/profile declarations + recorded
  disposition) reaching `pass`, committed under `testdata/`.
- Full `make verify` on the candidate head; pin rebuilt; the ATC real-pin flight arm turns from skip into a
  pass that reaches adapter verification; then the F12 genuine canary.

## 4. Coverage and losslessness witness

| Source | Destination | Coverage |
|---|---|---|
| Spike step 2 (resolver at the shared factory, opt-in) | §2.1, §2.2 | Same wiring; kind and witness renamed to state the weakness. |
| Spike step 3 (folding premise refuted; dispositions already external) | §1, §7 | Ledger corrections; no new artifact kind. |
| Spike step 4 (pass with genuine resolution) | §3 | Reproduced as a committed fixture and CLI tests. |
| Spike step 5 (tooling gaps: multi-claim/human-fallback scaffold; projection verb) | §2.3, §2.4 | Both verbs specified. |
| Spike owner decisions (1)(2) | §2.1, §2.5 | Ratified as a solo-only, disclosed, weaker kind. |
| Spike step 6 (sealed path does not re-evaluate) | §2.2 | No ATC change. |

## 5. Owner decisions embodied here

1. Self-asserted local identity is admitted for solo profiles only, under its own kind, witness, and disclosure.
2. No new resolution state; `Authorize` is unchanged.
3. Two verbs are added rather than leaving fixture authoring to hand-edited YAML and throwaway programs.

## 6. Routing

Verdi build rules: Sonnet implements, Opus reviews, FABLE orchestrates and commits locally while Codex is
unavailable; the owner ratifies this document and each task's acceptance.

## 7. Ledger corrections (ATC PLAN.md)

IL-108: the `pass` vacuity finding stands, but its stated mechanism ("the accepted-context target digest is the
manifest digest, which folds the disposition") is withdrawn; the disposition match operand is the target
specification's raw content digest. IL-126: the block stands at the pinned line and is lifted by this design;
the "disposition-required" reason arises because the CLI supplies no principal resolver, not because a
witness cannot match.
