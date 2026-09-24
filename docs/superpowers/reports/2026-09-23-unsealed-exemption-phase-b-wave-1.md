# Unsealed-provenance exemption, phase B wave 1 report: forge merge records, the exemption witness and payload, signed-commit approvals, and the shared CI ref

Status: whole-wave review APPROVE; pushed for the owner's risk gate and merge (owner authorization 2026-09-23: push and open
the pull request once approved). After this wave the exemption program is parked by owner direction (2026-09-23): the
workbench redesign comes next, then the real-use checkpoint.
Risk tier: Tier 3 for every lane except E4d (Tier 2, comment-only).
Base..Head: origin/main `3bc854b6` (PR #350) → eight `--no-ff` lane merges, controller ledger commits (SI-254..SI-258 and
in-place amendments) and the backlog commit (BL-53..BL-62) → `844b1697` → this report.
Plan: `docs/superpowers/plans/2026-09-23-unsealed-exemption-phase-b.md` (R-PB-1..R-PB-7). Design:
`docs/superpowers/specs/2026-09-23-unsealed-provenance-exemption-design.md`.
Ledger added in this wave: SI-254 (what "merged into the default branch" means in forge merge records), SI-255 (the wire forms of
the required-input witness, the escalation record, and the `unsealed-provenance` payload, and the immutable cutoff), SI-256 (how
an approval row is bound to the signed commit that introduced it), SI-257 (the CI ref beside the repository facts, "the branch
being closed", and its sealing for reverification), SI-258 (the committed close context-request template and its
instantiation). Next free: SI-259.

## Lanes and accepted ranges

Every implementer, reviewer, fixer, and re-reviewer ran on `claude-opus-5-5`, confirmed from each transcript's recorded model
field (the lane ledger records the counts).

| Lane | Range (base..head) | Merge | Chain | Result |
|---|---|---|---|---|
| E1a the exemption's required-input witness family | fd69b6a1..b69f7264 | 92c22873 | implementer → independent review | APPROVE |
| E1p the `unsealed-provenance` payload and its immutable cutoff | fd69b6a1..2802f1cd | 66058516 | implementer → independent review | APPROVE |
| EF forge merge records (GitHub, GitLab, fake) | 3bc854b6..fa598e9f | e547d18e | implementer → review (F1 Critical authority gap → owner; F2 Important base repository unchecked) → fresh fixer → fresh re-review | APPROVE |
| E4a the shared CI-ref fact and "the branch being closed" | fd69b6a1..89ac8187 | 95c3bd1f | implementer → independent review | APPROVE |
| E4b the committed close context-request template, close.yml instantiation | 5dcafdac..94c8c929 | 920ea2cf | implementer (one STOP, adjudicated R-PBW1-7) → review (header overclaimed) → fresh fixer → fresh re-review | APPROVE |
| E2 signed-commit approval authentication | fd69b6a1..8df20c15 | 705e078c | implementer → review (Critical line-break desynchronization; Important withdrawn approval restored by merge) → fresh fixer → fresh re-review | APPROVE |
| E4c the sealed CI ref for the conflict evaluator's reverification | ed452115..496a3fba | 6977b571 | implementer → independent review (one Minor, closed by a test-only commit, controller-verified) | APPROVE |
| E4d close.yml's header made true after E4c (comment-only) | 471b39e7..f281d97f | 844b1697 | implementer → independent review | APPROVE |

For every merge, the integrated diff equals the reviewed range's diff byte for byte (`cmp` of the two `git diff` outputs).

## Gates

- Gate 1 at `471b39e7` (`VERDI_E2E_PORT_BASE=4400 make verify`, serial, idle machine): red at the test step only — `cmd/verdi`
  `panic: test timed out after 10m0s` (600.95 s) under `go test -race ./...`.
- Timing witness: `cmd/verdi` alone under `-race` takes 565 s on main `3bc854b6` and 546 s on the wave head (1,012 top-level
  tests each), so the wave did not slow it; main's own `make test` fails the same way locally (`FAIL cmd/verdi 601.691s`).
- Gate 2 at `844b1697` (`GOFLAGS=-timeout=25m VERDI_E2E_PORT_BASE=4400 make verify`, serial, idle): **verify OK**, 2,157 s — build
  1 s, fmt-check 0, vet 2, lint 1, test 1,015, fixture 0, lint-store 17, spec-align 246, lint-showcase 6, showcase-coverage 6,
  e2e 863 (330 passed). Ruling R-PBW1-12 discloses the accommodation: the package timeout is a hang guard, not an assertion; no
  test or step changed; CI's `make verify` stays the definitive gate on push. The local runtime problem is backlog BL-63.
- Spec fidelity at `844b1697`, with the transient `verdi-wt/docs` link: `TestSelfHostedSpecFidelity` PASS.

## What shipped

- **Forge merge records (EF).** `forge.Forge.MergeRecords` reads the change requests the forge associates with a commit, with
  the forge's own default branch; GitHub and GitLab adapters decode published examples as tolerant subsets (R-W1-8) and refuse a
  change whose base repository or target project is not the queried one. `ProveMergedIntoDefault` proves only that the forge
  reports the commit as the merge commit of a change it reports merged into its default branch — not that the forge created the
  commit (see the open owner item below).
- **The exemption witness (E1a).** `policyartifact.Exemption.RequiredInput`: phase `review`, exactly the three sealed-provenance
  inputs, one story, the accepted-spec digest, the implementation commit and tree, the inventory entry, the landed snapshot,
  and an optional escalation record; never mixed with claim witnesses; expiry mandatory. Existing exemptions' digests are
  unchanged.
- **The payload (E1p).** `internal/unsealedprovenance`: a singleton typed constitution payload with `permitted`, `cap`, an
  inventory, and an optional cutoff; constitution submit-preparation blocks any proposal that removes or moves an accepted
  cutoff.
- **Signed-commit approvals (E2).** `internal/signedapproval` authenticates an approval row only when every line of the row
  blames to one non-boundary commit whose own copy carries that exact row, the row survives every later commit that touches the
  file, the content outside the approval rows is byte-identical at the closed head, and the forge verifies the commit for the
  row's principal; kernel inputs are built per artifact. The forge read for commit verification is lane EF2 (not yet built).
- **The shared CI ref (E4a, E4c).** `repositoryfacts.Snapshot.CIRef` beside the unchanged `Facts`, validated against an exact
  `show-ref` read of `refs/remotes/origin/<name>`; "the branch being closed" feeds close's conflict gate, the compiler's stage 3,
  the countersign, and (sealed with the operands) the conflict evaluator's reverification. Six pinned policyconflict and
  contextcompile files and `compiler.go` are bound as reviewed successors; `checks.json` is untouched.
- **The close request (E4b, E4d).** `.github/verdi/close-context-request.json` (codex `1`, phase `review`, universal scope,
  empty grants); close.yml fills only `spec` with `jq`, accepts only `spec/<name>`, and passes the request; its header now says
  exactly what the code does.

Nothing here makes a close possible: a review-phase close is still blocked-unproven by the three sealed-provenance inputs until
lanes E1b and E1c, which are parked.

## Controller rulings (the wave ledger is gitignored; this is their repository-visible home)

- R-PBW1-1: every lane Tier 3.
- R-PBW1-2: the forge commit-verification read moved from E2 to a new lane EF2, so wave-1 write sets stayed disjoint.
- R-PBW1-3: lane commits carry the implementing model's own trailer.
- R-PBW1-4: GitHub's commit response for R-PB-1 is approval decoding under R-W1-8 (for EF2).
- R-PBW1-5: E1a split into E1a and E1p; E4 split into E4a and E4b.
- R-PBW1-6: SI-254..SI-258 recorded before the lanes that needed them.
- R-PBW1-7: the template's codex adapter version is `1`; the filled request lives in the git-ignored `.build/`.
- R-PBW1-8: the reverification consumer of the branch being closed (missed in SI-257's first text) became lane E4c.
- R-PBW1-9: SI-256 tightened after E2's review (LF/CRLF-only UTF-8 frontmatter, the whole-path withdrawal rule, one
  signed-commit source); later I1/I2 and the row-identity reading.
- R-PBW1-10, R-PBW1-11: close.yml's header is made true inside this wave (lane E4d) rather than deferred.
- R-PBW1-12: the definitive gate ran with `GOFLAGS=-timeout=25m`, disclosed above.

## Whole-wave review

Independent Opus 5.5 review of `844b1697`: **APPROVE**, no Critical or Important findings, nine Minor. Its probes showed the
required-input exemption inert and never favorable through the real review-phase evaluation, a binary without
`unsealedprovenance` failing closed at load, exactly the seven pinned files changed and each bound as a successor, all eight
integrations patch-identical, and the close path with the production provider exiting 1 `blocked-unproven` on a detached
checkout with a validated CI ref. The Minor findings are recorded here: BL-63 (the local timeout), BL-64 (F1), BL-65 (the
escalation threshold and claim exemptions), SI-254(a), SI-255, and SI-256 amended in place, BL-53 widened, and the next-lane
obligations below.

## Open owner items

1. **F1, the forge-created merge proof (blocks E1c; backlog BL-64).** Forge "merged" state does not prove the forge created the merge commit:
   GitLab marks a merge request merged when its commits are pushed directly, and GitHub documents indirect merges. The
   controller's recommendation (option A): on GitHub, the landing and issuance merge commits must also carry GitHub's own verified
   signature (`web-flow`) and the landing commit's second parent must equal the change's forge-reported head; GitLab is
   unsupported for this proof. PRs #348–#350 already meet A.
2. **X-1 (BL-53), the strict-YAML seam.** `artifact.DecodeStrict` drops null list items, truncates floats into integers, and
   accepts `yes`/`no` booleans; the governance profile's `at_least` and `minimum` are exposed. Scheduled as lane X1 by the
   process-hardening plan (PR #351).
3. **The escalation threshold and claim-family exemptions (BL-65; decide before E1b and L4).**

## Residual risks and obligations carried forward

- Unproven until a live run: `jq` on the GitHub runner image, the `fetch-depth: 0` checkout creating
  `refs/remotes/origin/close/<name>`, the runner's locale, GitHub's `merge_commit_sha` for indirect merges.
- Accepted fail-closed consequences: on a store that is not adopted every close.yml run exits 2 at the close step (SI-258); a
  detached pull-request pipeline reads the countersign's source branch as unproven (SI-257).
- For E1b: deep-copy `RequiredInput` in the pinned `cloneExemption`; carry `required_input` through `EffectiveExemption` if the
  effective view needs it; build per-artifact kernel inputs from `signedapproval` (authorize one artifact per request); decide
  whether required-input resolution uses the exemption's scope; replace the witness-less inertness in `resolveExemption`.
- For E1c: never read `ProveMergedIntoDefault`'s "proven" as forge-created (F1); map an error wrapping `forge.ErrUnavailable`
  from `MergeRecords` to unproven Scope; never widen the governance allowlist to all of `.verdi/policy/` (SI-256); derive
  disclosure codes from `Rows[].Reason`, never by parsing strings; define the functions behind `accepted_spec_digest` and
  `use_history_digest` (only their format is fixed); refuse a leading `:` in paths passed to the approval binding (BL-61);
  re-check close.yml's header when the exemption is wired.
- For EF2: the verifier contract E2 enforces (unavailable ⇒ reason only; signer a canonical positive decimal); record which
  account counts as the signer (R-PB-1 says the committer, but a GitHub web-interface commit's committer is `web-flow`; a
  verified commit with a null committer must be representable or refused); EF's carried minors (two doc phrases, a test row
  pinning the compared repository id).
- For E6 and L4: token scopes for the new reads (Metadata read, Pull requests read); the adopted profile needs exactly one
  `signed-commit` trust source mapping the approval roles and `unsealed-exemption-escalation`, and `policy-exemption-approval`
  in the catalog transitions and the profile's `applicable_transitions`; link `internal/unsealedprovenance` into
  `cmd/e2eharness`; the codex `1` registration; optionally surface the CI ref's reason code when close refuses an empty
  expected branch (`resolveBranchBeingClosed` drops it).
- Backlog rows BL-53..BL-62 carry the rest (lax GitLab pagination drain, duplicated status handling, the token on foreign
  `next` links, forge contract-suite coverage, close.yml pin limits, replace refs, the pull-request source-branch fact,
  `PathExistsAt` pathspec magic, single-line checks).

## Next authorized action

The owner's risk gate and merge of this wave's pull request. The exemption program stays parked after this wave; the
workbench redesign plan is next.
