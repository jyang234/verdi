# Unsealed-provenance exemption — phase B plan (implementation)

**Goal.** Build what the ratified exemption and the CF-1 decisions require, so that
`spec/vatc-machine-projections` can close in CI under an unsealed-provenance
exemption, honestly and only as the exception. Nothing here changes the ratified
rules; where implementation needs a choice the ratified text does not make, this
plan records it as a ruling and a ledger entry before any code.

**Base.** origin/main `67bcd9c9` (PR #349 merged: `spec/context-integrity-v3`
DC-25/CO-7, `docs/superpowers/specs/2026-09-23-unsealed-provenance-exemption-design.md`,
SI-239 to SI-249). Worktree `verdi-wt/phase-b-plan`, branch
`design/unsealed-exemption-phase-b`. Ledger next free: SI-250.

**Binding authority.** The exemption design (§1–§13) is the contract; this plan
implements it. Plan `docs/superpowers/plans/2026-09-23-unsealed-provenance-exemption.md`
(walls W1–W12, decisions 2–8). Closing-machinery wave 1 (PR #348) is the base the
close path already stands on.

**Owner decisions carried in.**

- CF-1 decisions 2–8 as confirmed on PR #349.
- **Approval signing (2026-09-23): option 1.** The owner approves by making a
  commit signed with an SSH signing key registered on their GitHub account (push
  stays HTTPS). Verdi trusts GitHub's own verification result for that commit.

## Rulings

**R-PB-1 (SI-250) — an approval is a forge-verified signed commit.** An approval
row `(role, principal)` in an exemption, escalation, or disposition artifact is
authenticated only when all hold:

- the commit that introduced that exact row is signed, and the forge reports the
  signature verified and attributed to the principal's forge account (GitHub:
  `GET /repos/{owner}/{repo}/commits/{sha}`, `commit.verification.verified` with
  reason `valid`, and the verified committer's account), read through the forge
  port;
- the artifact's content other than its approval rows is byte-identical between
  that commit and the closed head H, so an approval binds exactly the artifact it
  approved; any later content change requires fresh approvals;
- each approval row was introduced by its own signed commit, so two approvals are
  two acts. The escalation approval (design §9) is a distinct row with role
  `unsealed-exemption-escalation`, introduced by a distinct signed commit, and binds
  the exemption id and the digest of the use history it reviewed.

The fact reaches the kernel through its existing `TrustFactReader` port as a
`signed-commit` trust source, the source kind the kernel already defines; subjects
are the verified forge account. Unsigned, unverified, attributed to another
account, forge unreachable, or adapter unsupported: the fact is unavailable and the
approval is unproven. A commit made in GitHub's web interface is signed by GitHub
and verified for the logged-in account, so it also satisfies the rule; the owner's
chosen practice is an SSH signing key. Rejected: Verdi verifying signatures itself
against signer keys kept in the constitution (the `internal/experimenthuman`
Ed25519 precedent), because the owner chose forge verification and it reuses the
forge trust the countersign already relies on. No change to
`internal/governanceprincipal` (pinned).

**R-PB-2 (no new ledger entry; implements SI-249) — forge merge records.** The forge
port gains one read: the change requests associated with a commit, each with its
merged state, target branch, and merge commit (GitHub:
`GET /repos/{owner}/{repo}/commits/{sha}/pulls`; GitLab:
`GET /projects/:id/repository/commits/:sha/merge_requests`). A commit is a
forge-created landing or issuance merge only when some associated change is merged,
targets the default branch, reports that exact commit as its merge commit, and the
commit has two parents. Decoding follows R-W1-8 (known fields strictly typed,
unknown fields ignored, trailing data rejected, unknown enum values and missing ids
fail closed); fixtures are the providers' published examples verbatim, with any
departure disclosed. Unsupported adapters return no facts, which the evaluator
reads as unproven.

**R-PB-3 (SI-251) — the CI ref is a shared repository fact (decision 6).**
`internal/repositoryfacts` gains a CI-ref fact beside the unchanged current-branch
fact: the provider's ref name (`GITHUB_REF_NAME`, GitLab `CI_COMMIT_REF_NAME`),
accepted only when it names a branch and the remote-tracking head of that branch
equals HEAD. The physical detached-HEAD state stays recorded. Every consumer that
needs "the branch being closed" — the conflict gate's expected branch, the context
request, and the countersign — reads this one fact; the countersign's private
`resolveCIRefName` fallback is replaced by it. A missing, mismatched, or
non-branch ref is unknown and blocks where a branch is required.

**R-PB-4 (SI-252) — the committed context-request template (decision 3).** The CI
close request is instantiated from a committed, reviewed template at
`.github/verdi/close-context-request.json`, holding the adapter, phase `review`,
grants, and scope. The workflow fills only `spec` from the dispatch input, which
`verdi close` validates against the story it closes; `expected` is computed by the
lifecycle adapter from repository facts, as today. Dispatch inputs cannot supply
adapter, grants, or scope. The template is pinned by a workflow test. No new CLI
verb is introduced; the CLI registry is unchanged.

**R-PB-5 (SI-253) — `--prepare` order (decision 8).** `verdi close --prepare` runs
the constitutional conflict gate first and still stops on a blocking verdict or an
ineffective exemption before it writes anything; then it writes or refreshes the
alignment report; then it reports the countersign and every other remaining
blocker, and prints READY only when none remains. Only the countersign refusal
moves after the report write. `verdi close` itself is unchanged: it re-proves every
requirement before branch creation, freezing, archiving, or publication.

**R-PB-6 (no ledger entry) — the judge in CI (decision 4).** `close.yml` installs
the judge command named by `align.judge_cmd` (`claude`) at a pinned version and
exposes its credential only from a secret of the protected `close` environment.
The judge runs through the existing judge port; a missing credential, failed
install, or failed run is judge-unavailable and blocks, with no canned result. The
close job's permissions do not change.

**R-PB-7 (no ledger entry) — the registered adapter (decision 2).** The adopted
constitution registers adapter `codex` at the version the compiler supports,
matched by the template (R-PB-4). Registration is configuration, not proof of
isolation; the exemption never claims sealed provenance. It lands with L4.

## Lanes

All lanes are Tier 3 except E3f. Tier 3 implementers are Opus 5.5; reviews and
fixes are Opus 5.5; frontend work goes to a FABLE lane. No lane touches a file pinned
by `internal/sealedexec/testdata/public-consolidation/checks.json` without binding a
successor; `internal/governanceprincipal` is not edited at all.

| Lane | Scope | Tier | Depends on |
|---|---|---|---|
| EF | R-PB-2 forge merge-record read: port method, GitHub and GitLab adapters, fake, normalizer, published fixtures | 3 | — |
| E2 | R-PB-1 signed-commit trust facts: forge commit-verification read, the `signed-commit` `TrustFactReader`, approval-row binding rules | 3 | — |
| E1a | Exemption schema (design §4): the required-input witness family, `landed_snapshot`, strict decode, mandatory expiry; the typed `unsealed-provenance` payload (§8); the append-only cutoff record (§7) | 3 | — |
| E1b | Evaluator (design §5, §10): the five-state required-input resolution, `RequiredInputEvaluation` rows, reason `required-input-exemption-effective`, the three codes' blocking rule | 3 | E1a |
| E1c | Close adapter (design §3, §5, §9): H_e-to-H content identity over the allowlist, landing and issuance facts from EF, approvals from E2, W6 compensating controls, the cap at the issuance merge's first parent, the escalation metric from the archive's use history | 3 | E1b, EF, E2 |
| E3 | Closure record and `verdi.rollup/v1` `provenance_exemption` block; the label in close output, archive, and `verdi audit` | 3 | E1c |
| E3f | Board and journey presentation of the label | 2 (FABLE) | E3 |
| E4 | R-PB-3 CI-ref fact and its consumers; R-PB-4 template and workflow instantiation | 3 | — |
| E5 | R-PB-5 `--prepare` order | 3 | E1c |
| E6 | R-PB-6 judge installation and secret wiring in `close.yml`; R-PB-7 adapter-version validation | 3 | E4 |
| E7 | The composed acceptance witness (below) | 3 | all |

**Waves.** Wave 1: EF, E2, E1a, E4 in parallel (disjoint write sets; the interfaces
E1c consumes — the merge-record facts, the trust fact, and the schema types — are
frozen by each lane's brief before dispatch). Wave 2: E1b, then E1c. Wave 3: E3,
E5, E6, then E3f. Wave 4: E7, the full serial gate (`make verify` including e2e),
the whole-wave review, and the owner risk gate. Each lane runs the repository's
chain: controller pre-review with the full GREEN list, independent Opus review,
fresh Opus fixer and re-reviewer for Critical or Important findings.

**Acceptance witness (E7).** Design §13 in full, through the built binary and the
real conflict evaluator, with hermetic fakes for the forge (merge records, commit
verification, environment review) and a hermetic judge fake behind the existing
judge port. The positive run: adopted fixture (payload permitting one exemption,
the pilot inventoried) → implementation landed by a forge-created merge at D →
H_e chosen → the exemption committed with a forge-verified signed approval and
merged by a second forge-created merge M_e → close branch cut at M_e with the
dispositions → report-only R → CI records for R → artifact fetch → detached close
with the CI-ref fact and the first-attempt environment approval → archive and
rollup carrying `provenance_exemption` with the three inputs `unproven` →
publication. Every negative in design §13 refuses with nothing archived or
published, plus: an unsigned or unverified approval commit; an approval commit
attributed to another account; an approval whose artifact content changed after
it; an escalation filled by the exemption's own approval commit; a CI ref that does
not match HEAD; a request template missing or altered by dispatch input; a missing
judge credential.

## Owner actions (not lanes)

1. Before the pilot: create an SSH signing key, add it on GitHub as a **Signing
   Key**, and configure `git config gpg.format ssh` and `user.signingkey`.
2. With E6: add the judge credential as a secret of the `close` environment.
3. L4, after wave 4 passes: merge the adoption change the controller prepares —
   the solo profile with forge-sourced author and approver mappings and the close
   transition (BL-7), the escalation threshold (design §9), the constitution with
   adapter `codex` and the `unsealed-provenance` payload (`permitted: true`,
   `cap: 1`, inventory: the pilot), and the `verdi.yaml` `countersign:` block.
4. The pilot: choose H_e, sign and merge the exemption, prepare R, dispatch
   `close.yml`, and approve the `close` environment review.

## Review route

This plan records rulings (SI-250 to SI-253), so it gets one independent
cross-model review of its exact head, at most one correction pass, and one
closure check before any lane is dispatched. The ledger entries are recorded on
this branch; the owner's merge ratifies them.

## Out of scope

Every story other than the pilot; feature closes; the sealed review path (the
review-capsule widening and sealed-review receipts); the 282 legacy obligations
without a quality block; GitLab's environment-review adapter.

## Risks

- **Forge coupling.** Landing, issuance, approvals, and the countersign all rest on
  GitHub's authenticated records. That is the chosen trust boundary (SI-249,
  R-PB-1); an outage blocks closes rather than weakening them.
- **Judge credential in CI.** The close job holds a model credential. It is scoped
  to the protected `close` environment and never to push-triggered workflows.
- **Size.** Eleven lanes over four waves. The plan keeps each lane to one authority
  boundary; lanes that grow past review budget split before dispatch.
- **Pilot freshness.** The window from H_e to H must hold only allowlisted records;
  close the pilot promptly after the exemption merges.
