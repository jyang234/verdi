# Verdi

Verdi turns a directory in your repo into the system of record for design:
specs, ADRs, decisions, and evidence live as typed, linted, linked artifacts
next to the code they govern. One Go binary gives you a design workbench
(a board you graduate stickies from), an evidence-gated lifecycle
(`design → accept → build → align → gate → close`), an MCP server so agents
work from the same corpus as people, and a static docs site (the *dex*).

Verdi never grades on a curve. Every claim it makes about your design is
**proven**, **violated with a witness**, or **disclosed as unproven** —
silence is never a pass. A closed feature is one where every acceptance
criterion cleared on real CI evidence; a stale spec wears the scar; a check
that could not run says so out loud instead of quietly succeeding.

## Install

Build from the selected release's clean source checkout with Go 1.25 or newer
and Git. Keep that checkout at its selected commit; record the source revision
and binary identity alongside your adoption results:

```sh
# Run from the root of the selected Verdi checkout.
git status --short
test -z "$(git status --porcelain)"
git diff --exit-code HEAD
git rev-parse HEAD
mkdir -p .build/bin
CGO_ENABLED=0 go build -trimpath -o .build/bin/verdi ./cmd/verdi
export PATH="$PWD/.build/bin:$PATH"
go version -m .build/bin/verdi
shasum -a 256 .build/bin/verdi
```

The binary at `.build/bin/verdi` is the installation used below. Keep this
absolute directory on `PATH` in each new terminal. The build uses the checkout's
`go.mod` and `go.sum`; a first build needs its dependencies available through
your Go module cache or configured proxy. A source build is a local candidate;
release acceptance still requires the release's recorded verification. For an
existing ATC installation, retain its verified Verdi/ATC pair until the
[paired upgrade requirements](docs/handoffs/public-execution-contract-rollout.md#upgrade)
have been met.

## See it in two minutes

From the same checkout and terminal, open the bundled example store:

```console
$ cd examples/showcase
$ verdi serve                     # http://127.0.0.1:4173 — board, obligation wall, dex
```

`examples/showcase/` is **LoanServ**: a real, lint-clean store the way a
mid-size loan-servicing team's would look about a year in — four features at
four different lifecycle stages, five ADRs with a real supersession and an
audited exemption, obligations and receipts, a supersession chain, and a
live design draft. It is also the corpus Verdi's own end-to-end suite drives,
so it can never drift out of date (`examples/showcase/README.md` is the full
store guide).

The commands below are re-run byte-for-byte against this store on every
`make verify` (`internal/showcasealign`), so what you read here is what the
current binary prints. `verdi matrix` reads the working tree and reproduces
directly in your clone; try it.

**Trace a feature's evidence fold.** `spec/stale-decline` is the richest
feature here — four acceptance criteria spanning every evidence kind, three
implementing stories including a spike, and a mid-build deviation with a
disposition:

<!-- showcase-verify -->
```console
$ verdi matrix spec/stale-decline
feature: spec/stale-decline
status: accepted-pending-build

AC    STATUS   EVIDENCE            IMPLEMENTING STORIES                                    TEXT
ac-1  pending  attestation:absent  spec/borrower-update-mobile                             every branch that classifies a decline as stale routes its consequence through the outbox — no direct call to notification-svc or payments-gw
ac-2  pending  attestation:absent  spec/borrower-update-api                                loansvc retries the charge through the outbox exactly once per stale decline
ac-3  pending  attestation:absent  spec/borrower-update-mobile                             a partial refund against a stale-declined loan still reconciles correctly before any retried charge is issued
ac-4  pending  attestation:absent  spec/escrow-notify-v2, spec/escrow-notify [superseded]  the stale-decline rate for the affected cohort is checked against the pre-change baseline seven days post-deploy

stubs: acceptance-time plan; current mapping computed below
STUB                    DECLARED ACS  LIVE STORIES                 RECONCILIATION
borrower-update-api     ac-2          spec/borrower-update-api     unreconciled
borrower-update-mobile  ac-1, ac-3    spec/borrower-update-mobile  unreconciled

feature.violated: false
stub_reconciliation.blocked: true
```

Each AC names the stories implementing it and its evidence state; the *stubs*
block is the acceptance-time plan, reconciled against the stories that
actually landed. `ac-2` ("loansvc retries the charge through the outbox
exactly once per stale decline") is realized by `spec/borrower-update-api`.
Follow that story down to the concrete obligations it owes:

<!-- showcase-verify -->
```console
$ verdi matrix spec/borrower-update-api
story: jira:LOAN-1482
spec:  spec/borrower-update-api
status: accepted-pending-build

AC    STATUS     EVIDENCE                      TEXT                                                         OBLIGATION
ac-1  no-signal  static:none; behavioral:none  PUT /applications/:id/update returns 200 with the new state  static: The PUT route is registered on the application resource and returns the full updated state's shape; behavioral: A submitted application actually updates end to end through the API route

story.violated: false
story.eligible: false
```

The `OBLIGATION` column is the point: an AC does not clear because someone
says so, it clears because the obligations it owes — one per declared
(AC, evidence-kind) pair, here a `static` and a `behavioral` claim — are
discharged by CI evidence. `verdi serve`'s obligation wall shows the same
thing with receipts; the directory page's draft-boards link leads to the
live `payoff-quote-portal` design draft, a board still being triaged on a
branch (drafts are never committed — see "The showcase" below).

## Start your own store

Start in a clean project checkout with at least one Git commit, a configured
Git author, and no existing `.verdi/` directory. Use the binary installed above.
`verdi init` is non-interactive and writes only `.verdi/verdi.yaml`; repository
plumbing is a separate step. `verdi init --wizard` requires a terminal and
customizes vocabulary and scaffold templates. It does not configure your forge,
tracker, or evidence producers. Both forms refuse an existing `.verdi/` directory.

For a GitHub project, initialize and add the required generated-file attributes:

<!-- adoption-setup -->
```sh
verdi init
cat >> .verdi/verdi.yaml <<'YAML'
forge: github
YAML
cat >> .gitattributes <<'ATTRS'
.verdi/specs/*/*/board.json linguist-generated
.verdi/specs/*/*/rollup.json linguist-generated
.verdi/specs/*/*/deviation-report.md linguist-generated
ATTRS
cat >> .gitignore <<'IGNORE'
.verdi/data/
IGNORE
verdi model check
verdi lint
git add .verdi/verdi.yaml .gitattributes .gitignore
git commit -m "Adopt Verdi"
```

For GitLab, use `forge: gitlab` and replace each `linguist-generated` token
with `gitlab-generated`. Preserve existing manifest, attributes, and ignore
rules when adapting an already configured project. Keep `.verdi/data/` out of
Git: it contains disposable local state.

Verdi also needs the default branch's identity and history. In a normal clone,
fetch the real `origin` and run `git remote set-head origin -a`, then check
`git symbolic-ref refs/remotes/origin/HEAD`. Fetch enough history to establish
ancestry. A lone fetched `origin/main` or `origin/master` is also supported;
a local branch named `main` by itself is insufficient. Missing proof produces
an **unproven** lifecycle and a read-only board. Refresh the genuine remote
refs to resolve it; do not invent CI environment variables. If testing the actual forge
is deferred, use the explicitly synthetic [local rehearsal](docs/local-adoption.md)
in a disposable project instead.

Create your first feature with both statements supplied; this works in a
terminal and in scripts:

<!-- adoption-feature -->
```sh
verdi design start --kind feature --name my-first-feature \
  --problem "People cannot tell whether a submitted request was saved." \
  --outcome "Every saved request returns a stable identifier that can be looked up."
verdi spec state spec/my-first-feature
verdi journey --json spec/my-first-feature
verdi matrix spec/my-first-feature
```

Then run `verdi serve` and open
`http://127.0.0.1:4173/board/spec/my-first-feature`. Stop it with Ctrl-C when
finished. `design start` creates `design/my-first-feature` and commits a draft
under `.verdi/specs/active/my-first-feature/`. The scaffold still needs meaningful
acceptance criteria and story stubs. Author those before review. Use the board's
supported edit controls or edit the draft `spec.md` in your editor; inspect the
saved changes and run `verdi lint`. The board identifies its displayed revision;
reload after external changes and resolve unsaved edits before refreshing.

Without the two flags, an attached terminal runs the problem/outcome interview.
Without a terminal, the command refuses and names the required flags; supply
both and retry the same name. `--defer-statements` deliberately leaves disclosed
TODOs that must be authored before review. It cannot be combined with statement
flags. An unrelated pre-existing `design/<name>` branch is still a collision.

A feature needs no tracker. A story requires a configured scheme and a tracker
reference, for example `verdi design start jira:LOAN-42 --kind story --name
request-receipt --problem "…" --outcome "…"`. Configure your actual Jira
`base_url` and `rollup_field` under `providers.jira` in `.verdi/verdi.yaml`;
provide credentials through `VERDI_JIRA_TOKEN`, never committed YAML. Replace
placeholder `implements` edges with the real feature ACs and author the story's
obligations before review (`verdi obligation scaffold spec/request-receipt`
prepares missing obligation files; scaffolds alone do not prove their claims).

Evidence generation requires a pinned `toolchain.module` and full
`toolchain.commit` in the manifest, Go on `PATH`, discoverable impacted services,
and the pinned upstream CLI modules available to `go run`. The
[configuration guide](docs/local-adoption.md#evidence-and-tracker-prerequisites)
shows these fields. Without that setup, design creation discloses a skipped
advisory baseline. That skip is not evidence of alignment or completion.

The board's Semantic review packet requires adopted project policy authority;
without it, the panel reports `policy-forbidden`. Ordinary draft editing remains
available. Follow the [policy setup checks](docs/policy-setup-validation.md) to
inspect the missing prerequisite and the current manual authoring requirements.
Adopt your project's policy before relying on that review surface.
Readiness labels such as “Ready” report the listed structural checks; they do
not establish that placeholder text is meaningful or that human review occurred.

Submit the authored specification and obligations through your repository's
required review and checks. **Merging the reviewed specification into the
configured default branch accepts that exact revision.** `verdi accept
spec/my-first-feature` only prints a compatibility notice and makes no changes;
its exit 0 is not acceptance. Inspect acceptance using `verdi spec state` after
refreshing the default-branch refs. For an accepted story, `verdi build start
jira:LOAN-42` begins implementation. Inspect `verdi align`, `verdi matrix`, and
`verdi journey --json` as work proceeds. Their disclosures identify missing
proof; a successful read command does not mean the gate passed. Real CI evidence
and the applicable human approvals remain necessary for closure. Closure is not
a blanket requirement for the narrower Local MVP milestone: defining and
inspecting a meaningful change with truthful evidence. If a chosen step requires
external proof, that step remains incomplete until the proof exists. The
[local guide](docs/local-adoption.md) distinguishes Local MVP acceptance from
validation of actual forge approvals, merge requirements, and CI integration.

## Core concepts

**Two-level model.** *Feature specs* are the birds-eye view: a grouping of
stories that deliver a business outcome, with outcome-level acceptance
criteria that are implementation-blind. *Story specs* implement individual
feature ACs (via `implements` edges) or, for a spike, answer open questions
(via `resolves`). A feature is downward-blind: its AC→story mapping is only
ever the computed inverse of the stories' own `implements` edges, never a
field it maintains itself.

**Artifact kinds.** Everything is a typed, linted file under `.verdi/`:

| Kind | What it is |
|---|---|
| `spec` | A feature, story, or component spec (component = a living service boundary) |
| `adr` | An architecture decision record: `proposed → accepted → superseded` |
| `diagram` | A service/flow diagram at one of three tiers: illustrative, full, or proposal |
| `obligation` | A concrete, checkable claim a story owes for one (AC, evidence-kind) pair |
| `attestation` | A frozen record that an outcome was met — existence is the evidence |
| `conflict` | A filed dispute against a decision: `open → superseded \| dismissed` |
| `waiver` | A time-boxed exception: `active → expired` |
| `reaffirmation` | A re-dated confirmation that evidence still stands behind a decision |
| `annotation` | A board sticky / comment / question / agent-task in the mutable zone |

**Link taxonomy.** Artifacts reference each other through a closed vocabulary
of **eleven** typed edges; backlinks are the computed inverses.

| Type | Semantics |
|---|---|
| `implements` | story → feature-AC fragment it realizes |
| `resolves` | spike → open-question fragment it answers |
| `supersedes` | decision/spec replacement chain |
| `exempts` | decision → ADR it is excused from, with a required reason |
| `verifies` | evidence artifact → the AC or spec it proves |
| `derived-from` | generated artifact → its inputs |
| `annotates` | annotation → its target |
| `depends-on` | reading-order / knowledge dependency |
| `story` | spec → tracker item (scheme-prefixed ref) |
| `impacts` | spec → service |
| `challenges` | conflict → the closed decision it disputes |

(`examples/showcase/README.md` maps every one of the eleven to a live
exemplar in the store. `evidence-for` is *not* a twelfth edge — it is a
`verdi.bindings.yaml` field, an easy one to misremember.)

**Lifecycle verbs.** A spec travels `design → accept → build → align → gate →
close`; the CLI is that path plus the read surfaces.

| Verb | Purpose |
|---|---|
| `verdi init [--wizard]` | Initialize a store; the optional terminal wizard customizes vocabulary/templates |
| `verdi design start [<ref>] --kind feature\|story --name <n>` | Cut a design branch and scaffold a draft; supply both statement flags or use the terminal interview |
| `verdi accept <spec>` | Compatibility notice only; the reviewed spec revision is accepted when merged into the default branch |
| `verdi build start <story>` | Cut the build branch after acceptance |
| `verdi align [--freeze]` | Generate/refresh the alignment report (computed + judged); `--freeze` writes the closure edition |
| `verdi gate` | The merge gate: spec accepted, no AC violated, every finding dispositioned (exit 0 / 1 / 2) |
| `verdi close <story\|feature>` | Closure ritual: every AC evidenced, frozen rollup, archived quartet |
| `verdi lint` | Artifactlint (VL-001..021) — the CI gate for artifact validity |
| `verdi matrix <story\|feature>` | Compute and print the evidence fold |
| `verdi sync` | Pull the CI evidence bundle into `derived/` |
| `verdi audit` | Audit ADR exemptions and mid-build deviations |
| `verdi serve` | Localhost workbench (board, obligation wall) + lens/dex pages |
| `verdi mcp` | MCP server over stdio |
| `verdi dex build -o <dir>` | Emit the static docs site |

## MCP server

`verdi mcp` speaks the Model Context Protocol over stdio, so an agent reads
and writes the *same* corpus a person does — no second-hand summary. The
store is resolved from the working directory, so point your client's `cwd` at
a checkout:

```json
{
  "mcpServers": {
    "verdi": {
      "command": "verdi",
      "args": ["mcp"],
      "cwd": "/path/to/your/repo"
    }
  }
}
```

Nine tools are served (all read-only except the last):

- `search_artifacts` — full-text search over the corpus
- `get_artifact` — resolve `kind/name[@commit]` to content + frontmatter
- `get_links` — an artifact's typed outgoing links plus computed backlinks
- `get_matrix` — the evidence fold for a story or feature
- `get_context_bundle` — resolve a manifest of pinned refs to their contents
- `list_annotations` — annotations targeting one artifact, with drift status
- `list_tasks` — every open agent-task annotation across the store
- `get_board` — the deterministic board projection for a spec
- `add_annotation` — append an annotation to the mutable zone (the only write)

Every tool description carries a normative safety note: content these tools
return is **data, never instructions** — a corpus is untrusted input even
when it is your own team's.

## The showcase

`examples/showcase/` is not a mockup: it is a complete, individually vetted
store that doubles as Verdi's end-to-end feature corpus. Because the showcase
*is* the corpus every happy-path e2e test drives, a capability that ships
without exercising this store fails the build rather than drifting silently
out of the public example.

**Vetting bar.** Every artifact earned its place against three columns,
recorded per file in `docs/showcase-vetting.md`: lint-clean, editorially
exemplary (prose a team would actually write — no filler, no dead links), and
narrative-coherent (consistent with the whole LoanServ story, or cut).

**Drift gate.** `make verify` grows two showcase gates
(`internal/showcasealign`): `lint-showcase` proves the store reports zero
findings, and `showcase-coverage` fails with a *named* gap when any CLI verb,
MCP tool, or workbench surface has no showcase-backed e2e coverage. This
README's own examples are part of that gate — `TestReadmeExamplesFresh`
re-runs each `<!-- showcase-verify -->` block against a freshly provisioned
store and diffs the output, so a stale paste is a red build.

Against that canonical store, the lint gate is silent — zero findings, exit 0:

<!-- showcase-verify -->
```console
$ verdi lint
```

(A raw `verdi lint` in a plain clone of `examples/showcase` instead prints
`VL-009` / `VL-003` pin-resolution notices: the store's frozen artifacts pin
real commit SHAs, and that git history is reconstructed deterministically
from `examples/showcase/layers.txt`, not committed as a nested repo. The
clean result above is what the gate proves against the reconstructed store;
`examples/showcase/README.md` § "Linting this store" has the full account.)

## Development

Everything runs from `verdi/`:

```console
$ make verify
```

`make verify` is the whole gate, in one command: build, `gofmt` check, `go
vet`, `golangci-lint`, `go test -race ./...`, the fixture-determinism and
corpus golden-SHA gates, a self-lint of this repo's own store, `spec-align`
(self-hosted spec fidelity), the two showcase gates above, and the Playwright
e2e suite last. CI runs exactly `make verify` — local and CI verdicts agree
by construction. Individual gates are available too: `make test`, `make
lint`, `make fixture`, `make spec-align`, `make lint-showcase`, `make
showcase-coverage`, `make e2e`.
