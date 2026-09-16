# Local adoption rehearsal

Use this guide with the binary built by the [README installation](../README.md#install).
The owner-adopted milestones are distinct: **Local MVP accepted** requires two
complete real-project adoption journeys on the same release (second independent),
passing required local gates, working supported edits, existing acceptance rules,
and truthful evidence. **Forge/CI integration validated** requires actual
configured approval/merge enforcement and CI evidence production/retrieval.
A required external proof still blocks its specific step; authoritative closure
is not a blanket requirement for defining and inspecting a meaningful change.

This is a disposable synthetic Git project. It can exercise authoring, saved edits,
Git-derived lifecycle, and honest missing-evidence disclosures. It does not supply
real forge review, human approval, CI, or either final real-project adoption run.

## Prepare a local default branch

The following commands create their own local bare remote. Their clone, push,
and remote-head lookup touch only that directory; they contact no hosted forge.
Run them in the terminal where the selected Verdi binary is on `PATH`:

<!-- adoption-local-git -->
```sh
VERDI_REHEARSAL_DIR="$(mktemp -d)"
git init --bare --initial-branch=main "$VERDI_REHEARSAL_DIR/origin.git"
git clone "$VERDI_REHEARSAL_DIR/origin.git" "$VERDI_REHEARSAL_DIR/project"
cd "$VERDI_REHEARSAL_DIR/project"
git config user.name "Local rehearsal"
git config user.email "rehearsal@example.invalid"
printf '# Local adoption rehearsal\n\nSynthetic project; no human approval or CI is asserted.\n' > README.md
git add README.md
git commit -m "Seed local rehearsal"
git push origin main
git remote set-head origin -a
git symbolic-ref refs/remotes/origin/HEAD
```

Now execute the README's [store setup and feature commands](../README.md#start-your-own-store)
in this project. Keep `forge: github` for that example's attribute convention;
a filesystem remote supplies no GitHub review feed. The workbench must disclose
that absence. Preserve `VERDI_REHEARSAL_DIR` until your results are recorded.

## Author and inspect a meaningful change

For an existing specification, use [Import existing spec](import-existing-spec.md)
from the workbench home page. Review the copied fields, correct an input error,
select expected evidence, acknowledge retained source content and preview before
creating the proposal. A feature can be imported without an issue tracker; a
story still needs its configured scheme, tracker reference and required links.
Follow the returned board link, save a supported edit and reload. Inspect the
source record separately: it can verify the original import while reporting that
the current specification has changed. Record retained bytes and missing proof
truthfully; neither import nor a ready preview establishes acceptance.

Use a small feature you can describe precisely. Replace the scaffold's placeholder
ACs and story stubs with the outcome and plan you intend to implement. A request
receipt is one example: saving a request returns a stable identifier; looking up
that identifier returns the saved request; unknown identifiers produce a clear
not-found result. Inspect the board, save a supported draft edit, reload, and
check that the intended content persisted. Record every intervention.

Opening **Semantic review** without an adopted project policy returns
`policy-forbidden: project has not adopted policy authority`. Record that
prerequisite; do not create an approval or relax a policy check to get a packet.
The initial authoring path does not itself adopt project policy.

Use the [policy setup checks](policy-setup-validation.md) to inspect the actual
accepted/proposed snapshots and submission blockers. The current binary has no
first-time Constitution setup page; record any manual policy authoring and
developer explanation as interventions in the acceptance run.

Run `verdi lint`, `verdi spec state spec/my-first-feature`,
`verdi journey --json spec/my-first-feature`, and
`verdi matrix spec/my-first-feature`. An exit-0 projection may still contain
blockers or unproven facts. Inspect the content of the result.

The documented non-terminal refusal can also be exercised in a separate clean
project: run `verdi design start --kind feature --name my-first-feature` with
stdin redirected from `/dev/null`; expect exit 2 and no new branch. Retry the
same name with both statement flags. It should create the proposal normally.
Do not delete or reuse an unrelated pre-existing branch to force success.

## Evidence and tracker prerequisites

For a real Jira-backed story, the manifest uses these fields, with your actual
service and field identifiers:

```yaml
providers:
  jira:
    base_url: https://your-team.atlassian.net
    rollup_field: customfield_12345
```

Set `VERDI_JIRA_TOKEN` through your environment or secret manager. A purely local
rehearsal may add `mode: fake` under `providers.jira`; this selects the built-in
fake tracker. Name that use in your report. Its title fallback and any tracker
publish/read-back observations are synthetic, not proof from a real issue tracker.

Graph/contract generation needs an explicit upstream pin:

```yaml
toolchain:
  module: github.com/jyang234/golang-code-graph
  commit: cd38b1a56bb782177a207d741a39807821cf2c1c
services:
  discovery: flowmap
```

This pin is the one in this source checkout's own manifest. Use the pin validated
for your selected release. Verdi executes the upstream CLIs through
`go run <module>/cmd/<tool>@<commit>` and strict-decodes their results. Service
roots need `.flowmap.yaml` and the spec's `impacts` must name discovered services.
A missing toolchain or missing impacted service leaves the advisory baseline
unproven. The real runner can require the Go proxy even with a warm cache; defer
that network-dependent validation while actual upstream integration testing is deferred. Hermetic
regression tests use canned upstream outputs and disclose that substitution.

Alignment's optional `align.judge_cmd` is an argv array, for example
`["your-approved-judge", "--json"]`, not a shell command string. Configure a real
approved judge for judged alignment. When it is absent, inspect the disclosed
missing-judge finding rather than treating the computed section as a complete
alignment proof. Local evidence remains advisory; it cannot discharge a CI gate.

## Acceptance and stopping point

Review the authored feature/story and its obligations before testing Git landing.
For a story, `verdi obligation scaffold spec/<name>` runs before acceptance;
complete the obligation content and any required human authorship. Never invent
a principal, attestation, review approval, or CI record to advance a rehearsal.

A local merge and push to the disposable bare remote can exercise the
Git-derived acceptance projection. Record it as **synthetic landing**, including
the exact default-branch commit and spec bytes. It is not the repository's real
reviewed acceptance. Keep real forge enforcement, required checks, countersigning,
and accountable-human requirements explicitly unproven.

Continue locally through the available story build, implementation, alignment,
and matrix inspection. If a supported operation requires real approval or CI,
record its diagnostic and stop that transition. Do not force closure or fabricate
CI variables. A useful report identifies the binary SHA-256, source revision,
commands and browser actions, saved edits, local test results, synthetic inputs,
blocking facts, and the exact next action requiring a real environment.
