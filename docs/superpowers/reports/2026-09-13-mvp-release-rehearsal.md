# MVP R0–R3 local execution — 2026-09-13

The owner adopted the independently reviewed amendment at
`4acd6cccc468b39ca61e6d6a9a482b5265377b2d` and authorized R0–R3 while keeping
actual forge/CI integration testing deferred. The owner subsequently adopted
SI-199: Local MVP acceptance and Forge/CI integration validation are separate,
R0–R2 stay unchanged, and authoritative closure is not a blanket requirement for
defining and inspecting a meaningful change. This report records local evidence, not release
acceptance, canonical feature acceptance, or Wave 6 completion.

## Installation and onboarding (R0)

[README](../../../README.md) and [local rehearsal guide](../../../docs/local-adoption.md)
now document one source-build installation, manifest-only `init`, terminal versus
explicit-statement creation, generated-file attributes, ignored local data,
remote default-branch setup, tracker/toolchain prerequisites, and merge-signaled
spec acceptance. `accept <spec>` is documented as a non-mutating compatibility
notice. Later rehearsal observations added the Semantic review policy prerequisite
and the limited meaning of structural readiness labels.

The installation block executed from clean source commit
`0cabf9ddb6989041f0571e47c3de0036cf742a46`, producing
`.build/bin/verdi`, SHA-256
`31bfd10628de594749d6c0d0c88d88ff6d863b86630e1576d861497db6a0e9a0`.
Go build metadata and the source revision were captured. The sandbox initially
prevented access to the ordinary Go cache; the identical local build succeeded
with approved cache access. This was an execution-environment intervention,
not an extra product setup step. The installed Verdi/ATC pair was not replaced.

The marked README setup/feature blocks and the guide's local Git setup block
were extracted and executed verbatim in one shell. They passed against installed
Verdi source `4fb98233386ffa986d54f231a0f9954f83874fa5`, binary SHA-256
`6d3a4b5173db25d5c561c1d288045a0e960294f35a124daceaeb2c550a67024e`.
`init`, `model check`, initial `lint`, feature creation, state, journey, and matrix
all exited 0. The state was `proposed` with relation `new`; the journey and matrix
still disclosed missing proof. No terminal-test overrides or CI identity variables
were introduced. Existing README/showcase regression:

```text
go test -count=1 ./internal/showcasealign/...
ok  github.com/jyang234/verdi/internal/showcasealign  61.735s
```

## Retry repair (R1)

Implemented at `6f65c4eea164f099cd7066a4fa30d082a3afd46e`. Template loading
and statement preparation now finish before branch creation or provider lookup.
Preparation refusals preserve HEAD, branch, refs, index, tracked and untracked
files, and ignored store data. This promise does not extend to arbitrary later
I/O failures. Existing `--from-stub` behavior is unchanged.

The actual runtime producer was `claude-sonnet-5`; test corrections used
`claude-opus-5`. The controller inspected the consolidated diff and ran these
checks with external access disabled:

```text
New refusal and same-name retry cases against unchanged runtime:
FAIL github.com/jyang234/verdi/cmd/verdi  3.703s
Same cases after the preparation reorder:
ok   github.com/jyang234/verdi/cmd/verdi  3.702s
go test -count=1 ./cmd/verdi -run 'Test(RunDesignStart|Run_DesignStart|CmdDesignStart)'
ok   github.com/jyang234/verdi/cmd/verdi  8.258s
go test -race -count=1 ./cmd/verdi -run 'Test(RunDesignStart|Run_DesignStart|CmdDesignStart)'
ok   github.com/jyang234/verdi/cmd/verdi  9.466s
```

The behavioral RED exposed branch mutation in all eight preparation refusals
and the built-binary retry case. It followed a sequencing correction: the
producer initially reordered runtime before adding tests, then restored the
baseline before obtaining RED. An earlier helper-name compilation failure was
repaired and is not counted as behavioral RED. Optional real-Jira fixtures were
removed or replaced before execution. This was not an uninterrupted TDD sequence.

The required final independent Opus review remains pending. Automatic approval
review rejected its read-only invocation before the model ran because it would
send the private R1 patch, source, and test evidence to Anthropic without explicit
permission for that payload and destination. Earlier producer/fixer permissions
do not substitute for that permission. No review approval is claimed.

The documented installation was repeated from clean source `6f65c4ee`, producing
candidate SHA-256
`704d69db669a5046e84aabf14bff089b0b1ed77125ccb0d515b2dd171a7d1d0a`.
The extracted onboarding commands passed again in a new synthetic project.
A separate candidate smoke check confirmed that a non-terminal start without
statements exits 2 with repository/store snapshots unchanged; the corrected
same-name invocation exits 0 and reports a new proposed spec. These checks do not
constitute either required real-project adoption journey.

## Board labels (R2)

No frontend runtime or test changes have been made. Automatic approval review
rejected the prescribed FABLE invocation before the model ran. The stated reason
was that the Claude CLI could read and transmit relevant private Verdi source and
tests to Anthropic without specific authorization for that payload and destination,
despite the R2 task and FABLE routing authorization. No workaround or substitute
frontend producer was used. The bounded task packet is ready; this lane needs
that specific permission before it can execute. The baseline false sealed/accepted
labels remain a known release blocker.

## Continued adoption rehearsal (R3)

All project, tracker, and landing data below is synthetic. The project lives in
`/private/var/folders/67/pw7jvbv12d76jpz89mltjtyw0000gn/T/tmp.MbOYtdHQsq/project`;
its `origin` is the sibling local bare `origin.git`. No hosted repository was
pushed, reviewed, or merged. No CI, human approval, principal, attestation, or
successful closure was manufactured.

| Stage | Observation and verdict |
|---|---|
| Default branch | Local bare-repository seed, clone, initial push, and `git remote set-head origin -a` establish genuine local `origin/HEAD` and `origin/main`. `spec state` correctly changes from the earlier baseline's unknown state to a new proposal. This is synthetic Git plumbing, not hosted proof. |
| Feature | `design start --kind feature --name my-first-feature` with paired flags creates the proposal. The feature was authored as request receipts: unique stable IDs, exact lookup, and explicit not-found during one store instance's lifetime. One meaningful AC and a `request-receipt` story stub replaced TODO placeholders. |
| Supported browser edit | At `/board/spec/my-first-feature`, Set outcome → enter meaningful lookup/not-found text → Apply returned “Saved. Changed: outcome replaced.” The spec diff showed exactly the outcome replacement, and a design-provenance record disclosed unauthenticated attribution and not-applicable policy. Reload retained the saved text. This agent-operated synthetic edit is not a human authorship or approval claim. |
| External draft edit | The board displayed the authored AC and corrected stub after an external edit. The live heading initially retained the earlier title; a full page reload is the documented route after external edits. No content loss was observed. |
| Readiness limitation | A scaffold with a TODO AC was structurally “Ready” under Define success because an AC existed. This does not prove semantic completeness. The README now explains that limitation; no new readiness algorithm is inferred or implemented. |
| Review preparation | Opening Semantic review returned `verdi.design-failure/v1`, classification `verdict`, code `policy-forbidden`, detail `project has not adopted policy authority`. This prerequisite is now disclosed in onboarding. No policy check was waived to obtain a packet. |
| Synthetic feature landing | A local merge/push exercised only the Git projection: feature state became `accepted-pending-build`, relation `exact`, baseline commit `d23ca52844b1df7d53448edaba63465eb1ae1e6a`, blob `211484931e4c7c730cdc52c92d40c996bc74f609`. This was not reviewed real-project acceptance. |
| Story | With explicitly configured `providers.jira.mode: fake`, `design start jira:LOCAL-1 --kind story --name request-receipt` and paired flags succeeded. Missing fake issue title was disclosed as raw-ref fallback. The story was authored with a real `implements` edge, behavior, and static/behavioral evidence requirements. |
| Obligations | `verdi obligation scaffold spec/request-receipt` created both obligation files. They retain their unauthored markers and unresolved design debt. Generated ownership is not proof of actual human authorship. No first-person human statement was fabricated. |
| Evidence inspection | Story and feature matrices exited 0 while reporting no signal/pending evidence. `journey --json` disclosed missing forge facts, author-vouch proof, incomplete contributor coverage, and unavailable policy/profile authority. `lint` exited 0 with explicit VL-017 mutable-zone absence disclosures; this is not proof of review readiness. |
| Alignment | On the design branch, `verdi align` produced a decision-conflict report with computed `proven` and judged `disclosed-unproven-complete`, containing `judged-decision-coverage-absent`. No judge or upstream network process was invoked. |
| Blocked-path correction | After committing the authored story, `gate` exited 1 for a stale report and instructed `verdi align` again. Following that instruction refreshed the covered HEAD. A second `gate` still exited 1, now accurately identifying the unresolved missing-judge finding. The corrective action fixed freshness without falsely passing judgment. |
| Build boundary | `verdi build start spec/request-receipt` exited 1 because the story proposal has not landed on the default branch. The story was not merged around the unresolved review/obligation requirements. Implementation, build-branch alignment, real CI evidence, and closure remain unproven. |

The meaningful story is retained on `design/request-receipt`, commit
`bdcb410c403d9dc06214b8045d964f8c7c80bdfb`, with a refreshed local alignment report
in the working tree. This is the exact stopping point. Continue by establishing
project policy/review prerequisites and authentic human obligation/review input,
then follow the project's existing rules for the specific acceptance/build
steps if the chosen journey requires them. The narrower local milestone need not
force authoritative closure; this synthetic project nevertheless cannot count as
either required real-project adoption run. The temporary browser tab was closed and its own
localhost server was shut down cleanly.

## Required local verification

`make verify` exited 0 against runtime/source commit
`6f65c4eea164f099cd7066a4fa30d082a3afd46e`. Only this evidence report changed
after that commit during the gate. The unchanged target covered build, formatting,
vet, lint, the full `go test -race ./...` suite, fresh cross-binary integration
tests, fixtures, store lint/model check, specification alignment, README/showcase
checks, and Playwright:

```text
ok  github.com/jyang234/verdi/cmd/verdi  465.060s
ok  github.com/jyang234/verdi/internal/workbench  109.522s
276 passed (11.4m)
verify OK
```

The full transcript is `make-verify.log`. Specification alignment disclosed no
skipped checks. Store lint explicitly disclosed VL-017 checks as unproven because
the uncommitted mutable data zone is absent; successful lint is not a pass for
those missing witnesses. The command used cached Go/npm/browser dependencies,
disabled module/npm network retrieval, and directed HTTP proxies to a closed
loopback port with a localhost exception for the test servers. The local browser
harness shut down normally. These are local test results, not authoritative CI
or configured-service integration evidence. Passing the existing gate does not
resolve R2's known presentation defect or replace R1's pending independent review.

## Evidence locations and remaining work

Workspace-local evidence is under
`.local/verdi-system/development/mvp-readiness-20260913/execution-r0-r3/`:

- `r0-install.sh` / `r0-install.log`: literal documented build and binary identity.
- `r0-documented-commands.sh` / `.log`: extracted local Git/store/feature commands.
- `r1-sonnet-brief.txt` / `r1-sonnet-result.json`: producer task and model provenance.
- `r1-sonnet-runtime-result.json` and `r1-opus-*-result.json`: runtime producer and test-fixer model evidence.
- `r1-red-behavior.log`, `r1-green-behavior.log`, `r1-focused.log`, `r1-focused-race.log`: refusal regression and focused verification.
- `r1-consolidated-review.patch`, `r1-final-opus-review-brief.txt`, `r1-final-opus-review-blocked.json`: prepared review and approval rejection.
- `r0-r1-candidate-install.sh` / `.log`, `candidate-documented-commands.sh` / `.log`, `candidate-retry.json`: clean candidate build, repeated onboarding, and unchanged-state retry witness.
- `r2/task.txt` / `r2/status.md`: bounded frontend packet and approval-review rejection.
- `r3-story-inspection.json`: exact argv, cwd, exit codes, stdout/stderr, and binary hash.
- `r3-correction.json`: stale gate → align refresh → truthful remaining refusal.
- `make-verify.log`: complete required local gate, including full race and 276 passing browser tests.

R1 is implemented with focused local evidence; its required independent review
is pending. R2 remains blocked. R3 reached and preserved the local pre-review
boundary. No R4 release acceptance is claimed: the two real-project journeys on
one release, including an independent second journey, remain outstanding.
Actual configured forge approval/merge enforcement and CI production/retrieval
remain deferred under the owner's instruction. Local execution does not
automatically supply authoritative CI evidence; any chosen step requiring
external proof remains incomplete until that specific proof exists.
