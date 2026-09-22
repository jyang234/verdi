# Closing machinery — plan and rulings

**Goal.** Make `verdi close` able to close a story, then a feature, in this
repository, honestly. No spec has been archived since 2026-07-30; every close
is blocked today regardless of how the work was built.

**Base.** origin/main `d4f6de02`. Worktree `verdi-wt/closing-machinery`, branch
`agent/closing-machinery`. Ledger next free before this plan: SI-227.

**Owner decisions (2026-09-22).**

- **D1 — declared solo mode.** The owner approves their own closes; every
  closed record permanently discloses that no independent approver existed.
- **D2 — per-test records win.** CI records each named test's own result, and
  an obligation matches its exact test. This resolves the conflict between
  03's "coarse" rule and GLG v3 AC-2 / SI-71's exact producer match.

**Out of scope.** The out-of-process features and any exception for them; the
282 legacy obligations without a quality block; the public-execution-contract
release evidence; every feature close. The pilot is one story.

## Verified gaps (d4f6de02)

1. **Countersign.** `verdi.yaml` has no `countersign:` block
   (`internal/store/manifest.go:116`); no governance profile is adopted; the
   resolver hard-wires `SeparationDifferentFromAuthor`
   (`internal/lifecyclecountersign/resolve.go`) instead of asking the kernel's
   authorization interpreter, which GLG v3 DC-23/DC-24 require; the solo
   starter profile (`internal/designscaffold/templates/governance-profile-solo.md`)
   trusts only a self-asserted `local-operator` identity, does not list the
   close transition, and maps no `story-review`/`feature-uat` role.
2. **Evidence.** 41 elaborated obligations name `go-test:<pkg>:<Test>`
   producers; the only CI producer (`cmd/verdi/selfevidence.go`) emits coarse
   `verdi-verify-static`/`-behavioral` records, so every such obligation reads
   `producer-missing`. No grammar for `go-test:` exists anywhere.
3. **Job field.** SI-71 matches `authoritative_source.ref` ("verify") against
   `provenance.job`; the GitHub adapter stamps `GITHUB_RUN_ATTEMPT` there
   (`internal/forge/github/github.go:738`) to serve I-25's retry ordering. Even
   a correct record reads `source-ref-mismatch`.
4. **No CI close.** No workflow runs `verdi close`; close refuses outside CI
   without `--force-local`, which is non-authoritative.
5. **Freshness.** SI-71 rules code freshness as exact commit equality;
   `verify.yml` path filters skip spec-only commits, so a close branch head
   never carries fresh records.

## Rulings

**R-CM-1 (SI-227) — close countersign separation comes from the profile.**
The lifecycle countersign resolver asks the kernel authorization interpreter
for the separation rule (GLG v3 AC-3, DC-23), never a hard-wired constant.
Under a `team` or `high-assurance` profile nothing changes: the approver must
differ from the author, and vatc-forge-countersign ac-2's self-approval
refusal stands. Under a `solo` profile whose role mappings give the close
roles to the author's principal, the author fills the approver role: on a
forge that forbids self-approval, the approver act is the authenticated
author's own open change targeting the default branch at the candidate head.
The countersign record carries the kernel's solo role-collapse disclosure, and
the archived closure record keeps it. A `local-operator` identity alone never
proves a close countersign: the solo profile used for closes must declare a
forge-identity trust source and map `story-review` and `feature-uat` to the
owner's forge subject. Authority: GLG v3 AC-3 ("one authenticated principal
may fill author and approver roles where configured, with the collapsed
separation visibly disclosed"), DC-23; vatc-forge-countersign ac-2.

**R-CM-2 (SI-228, ratifies into 03) — named-test producers.** A producer ref
of the form `go-test:<package path relative to the module root>:<TestName>`
names exactly one top-level Go test. In the authoritative CI job, verdi runs
`go test -json -count=1` once per named package, restricted to the named
tests, and emits one record per obligation: `pass` only on that test's own
terminal pass event, `fail` on its fail event, `abstain` when it was skipped.
A named test absent from the run emits no record and prints a disclosure, so
the obligation reads `producer-missing` for that story only and CI stays
green for everyone else. Coarse suite records remain for bindings. Per-test
mapping is opt-in per elaborated obligation, and a renamed test surfaces as a
blocker, which answers 03's rot concern. 03's two "coarse" sentences are
narrowed accordingly (text below).

**R-CM-3 (SI-229, ratifies into 03) — the job's name is its own field.**
`provenance` gains an optional `job_name`: the CI job's declared name (GitHub
`GITHUB_JOB`, GitLab `CI_JOB_NAME`). `provenance.job` keeps its I-25 meaning,
the retry-ordering id. SI-71's authoritative-source match compares
`authoritative_source.ref` with `job_name`; a record without `job_name` reads
`source-ref-missing`. Chosen over redefining `job`, which would break I-25's
retry ordering on GitHub, where every attempt shares one run id.

**R-CM-4 (no ledger entry) — CI close workflow.** Authorized by 03 §Closure
ritual step 1 ("a manually triggered CI job"). A dispatch-only workflow takes
a ref, produces the evidence bundle for the exact commit it evaluates, then
runs `verdi close` in the same run. SI-71's commit-equality freshness is
unchanged: the close job makes its own evidence fresh.

**R-CM-5 (no ledger entry) — store adoption is an owner ritual.** Adopt the
solo starter through `verdi policy adopt --starter --profile solo`, extend
the profile per R-CM-1 through the policy store's own governed change path,
and add the `countersign:` block. If the starter or the change path cannot
express the extension, stop and record a ruling before any workaround.

## 03 ratification text (applied to origin and mirror together, after review)

§Declarations and binding, replace "Unit tests deliberately stay coarse —
suite pass/fail and the flowmap coverage delta — because per-test AC mapping
would rot and poison the matrix's credibility." with:

> Unit tests stay coarse by default — suite pass/fail and the flowmap
> coverage delta — because unmanaged per-test AC mapping would rot and poison
> the matrix's credibility. The one sanctioned exception is an elaborated
> evidence obligation that names a single test as its producer
> (`go-test:<package>:<TestName>`): that obligation is matched per test, and
> a renamed or removed test surfaces as a missing producer, a closure
> blocker, never as a silent pass.

§Evidence records, JSON example: `provenance` gains `"job_name": "verify"`.
After the `provenance.job` sentence, add:

> `provenance.job_name` is optional: the CI job's declared name. Authoritative
> -source matching compares an obligation's CI-job reference with
> `job_name`; `job` stays the ordering id.

§Evidence records, Bundle assembly: after "a `go test -json` suite run
produces coarse behavioral records (suite pass/fail, no per-test AC mapping —
see §Declarations)", add:

> ; for each elaborated obligation naming a test producer, the same job also
> emits one record per named test — `pass` on that test's terminal pass,
> `fail` on its failure, `abstain` when skipped, and no record when the test
> did not run.

08 entry: "Closing machinery — named-test producers and job name
(2026-09-22)", citing D2, SI-228, SI-229, and the mirror sync.

## Lanes

| Lane | Scope | Worker | Review |
|---|---|---|---|
| L1 | R-CM-2 emitter in `verdi sync --produce`; R-CM-3 field in `internal/artifact`, GitHub/GitLab adapters, obligation matching | Sonnet | Opus |
| L2 | R-CM-1 kernel-driven separation and solo collapse in `internal/lifecyclecountersign` and the countersign record | Sonnet | Opus, Tier 3 |
| L3 | R-CM-4 dispatch-only close workflow | Sonnet | Opus |
| L4 | R-CM-5 adoption and `countersign:` block | controller + owner merge | — |

Order: Codex reviews SI-227..229 and the 03 text; the owner ratifies; L1–L3
run in parallel worktrees; L4 lands; the full gate runs serially; then the
pilot.

**Pilot.** Close `spec/vatc-machine-projections` through the CI close job. Its
three criteria declare only static and behavioral evidence (four elaborated
obligations), so it exercises the machinery and needs no attestation. Success
is an archived spec whose closure record shows four per-test records, fresh at
the closed commit, and a countersign carrying the solo disclosure.
