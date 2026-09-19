# Check policy setup during adoption

Use this check when a draft board reports `policy-forbidden: project has not
adopted policy authority`. In authoring mode, ordinary human editing remains
available; review and read-only modes retain their editing restrictions. A
semantic review packet needs project policy. The notice's “Go to it” link opens
inline, read-only setup guidance on the board. The commands below inspect that
prerequisite; they do not adopt a policy or approve a specification.

The board's refusal concerns the checkout it is serving. It does not establish
whether the default branch already contains accepted policy. Inspect both
snapshots and their reasons first. If policy is accepted but missing here,
investigate the difference through the project's existing process; an older
branch is one possible cause. An unresolved default branch is missing proof,
not confirmation that initial setup is needed.

Run them from the project root with the selected release's `verdi` on `PATH`:

```sh
verdi context constitution inspect --request - <<'JSON'
{"schema":"verdi.constitution-inspect-request/v1"}
JSON

verdi context constitution validate --request - <<'JSON'
{"schema":"verdi.constitution-validate-request/v1"}
JSON

verdi context constitution impact-review --request - <<'JSON'
{"schema":"verdi.constitution-impact-review-request/v1","targets":[]}
JSON

verdi context constitution submit-preparation --request - <<'JSON'
{"schema":"verdi.constitution-submit-preparation-request/v1","targets":[]}
JSON
```

Read the returned fields, not just the exit code:

| Result | What to inspect |
|---|---|
| Inspect | `accepted.adopted` and `proposed.adopted`, their reasons and exact Git identities |
| Validate | `snapshot.adopted` and its reason; exit 0 can still mean no policy is adopted |
| Impact review | `coverage.state` and `coverage.reasons`; absent consumer inventories leave coverage unproven |
| Submission preparation | `ready_for_submission` and `blocking_reasons`; false means preparation is incomplete |

An empty `targets` list asks for no supplemental previews. It does not waive
registered-consumer coverage or make an empty/missing inventory sufficient.
Neither `adopted: true` on the proposed snapshot nor `ready_for_submission: true`
constitutes review approval or acceptance on the default branch.

## What initial setup requires

`verdi policy adopt --starter [--profile solo|team]` writes the initial
constitution, one governance profile, one starter policy, and the consumers
inventory in a single commit on a fresh `policy/adopt` branch (spec/spec-
documents ac-10, dc-6). Its `context constitution propose` operation creates
or amends one policy, overlay or exemption once that initial store exists; it
does not create the initial constitution or governance profile itself. The
starter writes:

- `.verdi/policy/constitution.md` selects the governance profile and declares
  the project's role, transition, evidence, subject and adapter catalogs.
- `.verdi/policy/profiles/<profile-id>.md` declares the supported identity trust
  sources, role mappings and applicable approval requirements. Configuring a
  mapping is not proof that an identity was authenticated or approved a change.
- `.verdi/policy/policies/<name>.md` carries project requirements. Additional
  overlays, exemptions and dispositions are included only when actually needed.
- `.verdi/constitution/consumers.json` declares the real registered consumers
  needed for impact coverage. The starter writes this file EMPTY and discloses
  that it did so; register real consumers here before impact review.

Keep the initial files on a proposal branch, retain their source authority,
and use the commands above to inspect and validate them. A missing or incomplete
policy directory is a setup state, not acceptance. A directory created merely
to silence the first diagnostic is not a complete policy. Authenticated
judgment/disposition requirements, when reached, remain in force.

Do not copy a test fixture's identities, approvals or trust facts into a real
project. Verdi's supported `local-operator` trust source is limited to solo
profiles and explicitly represents a local Git identity self-assertion. Using
it is a project governance decision; it does not prove forge review, CI,
independent runner identity or sealed execution.

`verdi context project` writes managed instruction projections. Before using
it, review the declared destination paths and preserve the existing authority
in files such as `AGENTS.md` and `CLAUDE.md`. It is not a read-only setup check.

Follow the project's existing review and acceptance process before treating
the proposal as accepted. If the actual required external proof is deferred,
its transition stays incomplete. Then return to the draft board and inspect
the next blocker.

## Record the adoption result

Record the release and binary identity, policy revision, command outputs,
missing proof and any source-code consultation or developer help. A user who
must discover these schemas in implementation code has not completed the
no-coaching milestone. The current manual setup path and its discoverability
remain usability work to validate; this diagnostic guide alone does not prove
that initial policy adoption is complete.
