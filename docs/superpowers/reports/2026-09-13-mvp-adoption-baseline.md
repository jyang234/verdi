# MVP adoption baseline — 2026-09-13

**Result:** blocked before acceptance and implementation. This is an agent-driven
disposable local probe, not a completed user journey, human usability evaluation,
real acceptance, hosted CI result, or release approval.

## Identity and method

- Installed Verdi source: `4fb98233386ffa986d54f231a0f9954f83874fa5`.
- Equivalent merged tree: `b281e7676945018e7594b8aea67879b02321751c`.
- Binary SHA-256: `6d3a4b5173db25d5c561c1d288045a0e960294f35a124daceaeb2c550a67024e`.
- Binary: `/Users/johnyang/code/verdi-system/.local/verdi-system/releases/public-v2-4fb98233-72383a06/bin/verdi`.
- Scratch root: `/private/tmp/verdi-mvp-baseline-crot6nh3`; three new Git repos
  with explicit synthetic local identities and initial `main` commits, no remote.
- Machine transcript: `/Users/johnyang/code/verdi-system/.local/verdi-system/development/mvp-readiness-20260913/baseline.json`.
- Transcript SHA-256: `48f7c6924718dcbe133a4175487f760f07638a9168f0bb92c5708a1a4413b5a7`.

The transcript contains every probe command, cwd, exit code, stdout/stderr, and
the browser observation. Git timestamps/SHAs are observations, not deterministic
fixture ratchets. Browser evidence was the Codex in-app accessibility tree and
screenshot of the installed binary's real board. No browser mutation or human
claim was submitted. The script did not set CI authority flags or use the test
TTY override. No tracker, provider, forge, or hosted service was invoked.

## Results

| ID | Observation | Classification / consequence |
|---|---|---|
| B-01 | Bare `verdi init` exited 0 and created the manifest. `verdi lint` then exited 1 with VL-012 for missing `.gitattributes` entries. The README's manual manifest path produced the same finding. | Proven onboarding gap. Preserve init's manifest-only contract and document required repository plumbing; this does not justify changing init semantics. |
| B-02 | The README's flagless `design start --kind feature --name my-first-feature` exited 2 in a real non-TTY subprocess and instructed the caller to supply statements. | Expected refusal under CLI Creation AC-2; document interactive behavior and a complete noninteractive example. It is not proof that the TTY interview fails. |
| B-03 | That refused command left `design/my-first-feature` checked out with no spec. Retrying the same name with both advised statement flags exited 2 because the branch already existed. | Violated-with-witness: the suggested correction cannot complete without another intervention. R1 owns preparation before the branch effect. |
| B-04 | In a third fresh repo, supplying both statement flags on the first invocation exited 0, created the branch/scaffold, and disclosed absent toolchain baseline regeneration. | Proven scaffold creation only. Its template still contains placeholder AC/stub/body content; this is not a completed meaningful feature. |
| B-05 | `spec state` and `journey --json` returned exit 0 with explicit unproven state because no default branch resolved; matrix showed no signal and unreconciled stubs. | Honest projection, not a successful acceptance/gate. The quickstart needs supported default-branch/tracker/toolchain setup. No default-branch or CI proof was fabricated to advance the probe. |
| B-06 | The board disclosed the unresolved default branch and `displayed bytes: proposed (unproven)`, while displaying `READ-ONLY · SEALED RECORD` and “This spec is accepted; the wall is its photograph. Change means supersession (the amendment ladder).” | Violated-with-witness: unknown lifecycle truth is presented as acceptance. R2 owns the FABLE presentation repair; read-only refusal itself stays intact. |
| B-07 | `accept spec/my-first-feature` exited 0 with the retirement notice; surrounding Git status remained clean in the two refused-start repos. | Proven compatibility notice, not acceptance. README's freeze instruction is false for specs. The command intentionally does not require the referenced spec to exist. |

Source confirmation for B-03: `cmd/verdi/design.go` calls
`gitx.CheckoutNewBranch` before template loading and the statement/interview
switch. Existing `designstatements_test.go` negative cases assert exit/message,
or absent spec directory, without asserting unchanged branch refs and a usable
same-name retry. B-06's renderer maps `modeReadOnly` to “sealed record” and emits
the accepted-spec paragraph in `internal/workbench/boardspecrender.go`, although
the projection also uses read-only mode when acceptance is unproven.

## Reproduction and limits

After a new `git init -b main`, an initial commit, `verdi init`, and committing
the manifest, run without an attached terminal:

```sh
verdi design start --kind feature --name my-first-feature
git status --short --branch
verdi design start --kind feature --name my-first-feature \
  --problem 'Readers cannot distinguish current evidence from missing evidence.' \
  --outcome 'Readers can inspect evidence and identify the next action for one small change.'
```

Observed: first exit 2, current branch `design/my-first-feature`, then exit 2
with `fatal: A branch named 'design/my-first-feature' already exists.`
The third repo used the final command first and succeeded. Its board was opened
at `/board/spec/my-first-feature` with `verdi serve --http 127.0.0.1:42873`.
The sandbox initially refused the Unix socket bind; the authorized local run
started both listeners and shut down on SIGINT with exit 0. This environmental
restriction is not a product defect.

The baseline did not complete a supported edit, feature/story authoring,
review/acceptance, implementation, alignment, authoritative evidence, recovery,
or a second independent run. It ran no full build/race/Playwright gate suite.
Those remain unproven, rather than inherited from older reports. Placeholder
readiness labels and other visual impressions require separate contract checks
before becoming additional defects.

Next execution follows the proposed
[MVP release amendment](../plans/2026-08-29-wave-6-workbench-presentation.md#mvp-release-amendment--2026-09-13),
after its review/adoption boundary. No source/runtime, installed pair, historical
flight, frozen artifact, or canonical specification was changed by this probe.
