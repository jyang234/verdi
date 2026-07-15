---
id: spec/public-readme
kind: spec
title: "Public Readme"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-24
problem: { text: "verdi has no public README: nothing explains what verdi is or lets a newcomer see it work in minutes, and public-showcase#ac-3 requires the quick start's commands to reproduce verbatim against examples/showcase — but even once written, nothing would keep those commands honest as the binary's behavior changes over time, the same silent-drift risk showcase-drift-gate closed for capability coverage, left open here for the README's own prose", anchor: "#problem" }
outcome: { text: "a top-level README.md quick-starts a reader from examples/showcase through core concepts, MCP, and the showcase's own drift gate, documents starting a fresh store with no verdi init verb (ledger L-A), and every console block claiming verbatim reproduction is tagged <!-- showcase-verify --> and re-run by a new TestReadmeExamplesFresh (ledger L-D) wired into the existing showcase-coverage make target, so a drifted example is a named make verify failure rather than a silently stale doc", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every README console block tagged <!-- showcase-verify --> reproduces verbatim against a provisioned showcase store", evidence: [behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/public-showcase#ac-3" }
decisions:
  - { id: dc-1, text: "README examples are fenced console blocks immediately preceded by an HTML comment tag <!-- showcase-verify --> (or <!-- showcase-verify exit=1 --> for a deliberately-nonzero example); a new TestReadmeExamplesFresh in internal/showcasealign parses each tagged block's first line ($ verdi ...) into argv and its remaining lines into the expected stdout, then re-runs it against a provisioned showcase store and requires byte-identical output — ledger L-D (PLAN-V1.md R4-I-51), the smallest reversible mechanism, and consistent with 00 §Testing rules: CLI-behavioral paths are proven by Go tests driving the built binary, never by hand-verification or a Playwright spec", anchor: "#dc-1" }
  - { id: dc-2, text: "the README's own-store quick start documents a manual scaffold instead of a new verdi init verb: mkdir -p .verdi, a five-line verdi.yaml verbatim from examples/showcase/.verdi/verdi.yaml's shape, git add, then verdi design start --kind feature --name <name> — under ten lines total, per ledger L-A (PLAN-V1.md R4-I-48): a valid store is only .verdi/verdi.yaml plus git, documentable without a dedicated verb, and reversible — a future init verb could wrap the same lines without changing what makes a store valid", anchor: "#dc-2" }
  - { id: dc-3, text: "TestReadmeExamplesFresh lands in internal/showcasealign/readme_test.go, the exact package showcase-drift-gate's showcase-coverage make target already names in its -run pattern in anticipation of this story; this story completes the two-step wire-up that target's own Makefile comment demands — (a) the test existing in that package, already selected the moment it exists, and (b) appending TestReadmeExamplesFresh to the target's required-PASS guard list so a deleted, renamed, or skipped freshness test is caught as a named guard failure rather than go test -run's vacuous zero-match pass — not a new make target, not a silent auto-detect", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network, ever: the README's paste-verbatim output is captured by hand once against a real scratch store during authoring (no hand-typed output), and thereafter verified continuously by TestReadmeExamplesFresh, which provisions its own showcase store exactly as Task 3.1's provisionShowcaseStore helper does (fixturegit stable SHAs) and execs the already-built verdi binary exactly as every other Go e2e test in this repo does — never a live service", anchor: "#co-1" }
  - { id: co-2, text: "silence is never a pass: a README with zero showcase-verify blocks, or a malformed tag, is a hard test failure rather than a vacuous green; a genuinely drifted example names the exact command line and a want/got diff rather than a bare pass/fail, mirroring showcase-drift-gate's own naming discipline; and the showcase-coverage Makefile guard's two-step wire-up (dc-3) means the freshness test's own deletion or renaming is caught as a named guard failure, not a silent pass", anchor: "#co-2" }
---
# Public Readme

## Problem

verdi has been dogfooding its own design process for six rounds, but none of
that depth has a public face. There is no README a newcomer can read to
learn what verdi is, and no path to seeing it run in minutes.
`spec/public-showcase#ac-3` requires the quick start section of that README
to be a sequence of commands a reader can run verbatim against
`examples/showcase` and get the output shown — no hand-substitution, no
"your results may vary." But writing such a README once is not enough:
`showcase-drift-gate` (this feature's sibling story) already closed the risk
that a shipped capability silently stops appearing anywhere in the showcase
or its e2e coverage — it did not, and by its own stated boundary could not,
close the parallel risk that a README's own printed example output silently
stops matching what the binary actually prints once that capability's
behavior changes. Nothing today authors the README, and nothing would keep
it honest afterward.

## Outcome

A top-level `README.md` quick-starts a reader: an opening value-proposition
paragraph (what verdi is, and its three-valued honesty rule — proven,
violated-with-a-witness, or disclosed-as-unproven); a "See it in two
minutes" tour that runs directly against `examples/showcase`
(`showcase-corpus-renovation`'s vetted corpus) — cloning, provisioning a
store, and walking a handful of "trace this" paths: an AC through its
obligation, receipt, and archived closure; the ADR-0001→0002 supersession;
the live `payoff-quote-portal` draft board; a "Start your own store"
section documenting the no-`verdi init` manual scaffold (ledger L-A: a bare
`.verdi/verdi.yaml` plus git, under ten lines); "Core concepts" (the
two-level model, artifact kinds, the link taxonomy, the verb table
condensed from spec 05); "MCP" (a stdio config snippet and the nine tool
names, correcting `internal/mcpserve/doc.go`'s stale eight-tool count along
the way); a short paragraph naming the showcase store and its drift gate
(what `showcase-corpus-renovation` and `showcase-drift-gate` built); and a
"Development" section (`make verify`, CI parity). Every console block that
claims verbatim reproduction is tagged `<!-- showcase-verify -->` and
re-run by a new `TestReadmeExamplesFresh` (`internal/showcasealign`, ledger
L-D), which `showcase-drift-gate`'s `showcase-coverage` make target already
names in its `-run` pattern in anticipation of this story and which this
story wires all the way in — so a README example that drifts from real
binary output turns `make verify` red and names exactly which command
drifted, rather than going stale silently.

## AC-1

`TestReadmeExamplesFresh` (`internal/showcasealign/readme_test.go`) reads
the repository's own `README.md`, parses every fenced ` ```console ` block
immediately preceded by a `<!-- showcase-verify -->` tag (a tag reading
`<!-- showcase-verify exit=1 -->` declares a deliberately-nonzero example):
the fence's first line (`$ verdi ...`) splits into argv, and its remaining
lines are the expected stdout, verbatim. The test provisions a showcase
store exactly as Task 3.1's `provisionShowcaseStore` helper does and
re-runs each parsed command against it through the same `runBinary` harness
`TestShowcaseCoverage` already uses, requiring the actual exit code to
agree with the tag's declared expectation and the actual stdout — after
trailing-whitespace normalization — to be byte-identical to the README's
own text. A README with zero tagged blocks is a hard failure
(`t.Fatal`), never a vacuous pass, because `public-showcase#ac-3` requires
the quick start to be checked, not merely written; a malformed tag (no
` ```console ` fence following, or a first line that isn't `$ verdi ...`)
is equally a hard failure, not a skip. The test is wired into the existing
`showcase-coverage` make target rather than a new one: `showcase-drift-gate`
already authored that target's `-run` pattern to select
`TestReadmeExamplesFresh` by name in anticipation of this story (its own
DC-3), and this story completes the wire-up's second half by appending
`TestReadmeExamplesFresh` to the target's required-PASS guard list (this
story's own DC-3), so a deleted, renamed, or skipped freshness test is
caught as a named guard failure rather than `go test -run`'s vacuous
zero-match pass.

Evidence: **behavioral** (`TestReadmeExamplesFresh` run against the real
`README.md` and a freshly provisioned showcase store, in both directions —
a genuinely drifted or missing example fails naming the exact command and
a want/got diff, and a truthful, fully-tagged README passes clean — plus
`make showcase-coverage` end to end showing the test's own `--- PASS:`
line satisfying the Makefile guard).

## DC-1

Tagged blocks, re-run and diffed. README examples are fenced ` ```console `
blocks immediately preceded by `<!-- showcase-verify -->` (or
`<!-- showcase-verify exit=1 -->` for a deliberately-nonzero example).
`TestReadmeExamplesFresh` parses each tagged block's first line into argv
and its remaining lines into the expected stdout, then re-runs it against a
provisioned showcase store and requires byte-identical output. Ledger L-D
(PLAN-V1.md R4-I-51): the smallest reversible mechanism — swappable for
doc-generation later — and consistent with 00 §Testing rules, which holds
CLI-behavioral paths to Go tests driving the built binary, never
hand-verification or a Playwright spec.

## DC-2

No `verdi init`, a documented manual scaffold. The README's "start your own
store" section documents `mkdir -p .verdi`, a five-line `verdi.yaml`
verbatim from `examples/showcase/.verdi/verdi.yaml`'s shape, `git add`, then
`verdi design start --kind feature --name <name>` — under ten lines total.
Ledger L-A (PLAN-V1.md R4-I-48): a valid store is only `.verdi/verdi.yaml`
plus git, documentable without a dedicated verb, and reversible — a future
`init` verb could wrap the same lines without changing what makes a store
valid.

## DC-3

Complete the existing wire-up; do not invent a new gate.
`TestReadmeExamplesFresh` lands in `internal/showcasealign/readme_test.go`
— the exact package `showcase-drift-gate`'s `showcase-coverage` make target
already names in its `-run` pattern (`'TestShowcaseCoverage|
TestReadmeExamplesFresh'`), placed there in anticipation of this sibling
story. That pattern match alone already enforces the test's verdict once
the test exists. This story completes the two-step wire-up the target's
own Makefile comment demands in full: (a) the test existing in that
package (done by placing it there), and (b) appending
`TestReadmeExamplesFresh` to the target's required-PASS guard list, so a
deleted, renamed, or skipped freshness test is caught as a named guard
failure rather than `go test -run`'s vacuous zero-match pass — the same
drift-by-deletion class `showcase-drift-gate`'s own CO-2/DC-2 guarded
against for `TestShowcaseCoverage`. No new make target, no silent
auto-detect.

## CO-1

No network, ever. The README's paste-verbatim output is captured by hand
once against a real scratch store during authoring — no hand-typed output
— and thereafter verified continuously by `TestReadmeExamplesFresh`, which
provisions its own showcase store exactly as Task 3.1's
`provisionShowcaseStore` helper does (fixturegit stable SHAs, no live
service) and execs the already-built `verdi` binary exactly as every other
Go e2e test in this repository does. Nothing in this gate ever reaches, or
needs, a live service — matching 00's no-network rule and the testing
rule that CLI-behavioral paths are proven by Go tests driving the binary,
not Playwright.

## CO-2

Silence is never a pass. A README with zero `<!-- showcase-verify -->`
blocks, or one carrying a malformed tag, is a hard test failure, not a
vacuous green — the quick start must be checked, not merely written, per
`public-showcase#ac-3`. A genuinely drifted example names the exact command
line and shows a want/got diff rather than reporting a bare pass/fail,
mirroring `showcase-drift-gate`'s own naming discipline (00 §Provenance
discipline: silence is never a pass). And the `showcase-coverage` Makefile
guard's two-step wire-up (DC-3) means `TestReadmeExamplesFresh` itself being
deleted, renamed, or skipped is caught as a named guard failure, not a
silent pass of `go test -run` matching nothing.
