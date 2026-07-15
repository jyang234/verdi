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

```console
$ go install github.com/jyang234/verdi/cmd/verdi@latest
```

Requires Go 1.25 or newer. Verdi is a single static binary with no cgo — the
install puts `verdi` on your `GOBIN` path. (Until the repository is public,
clone it and `go install ./cmd/verdi` from the checkout instead.)

## See it in two minutes

Clone the repo — the canonical example store ships inside it — build, and
open the workbench:

```console
$ git clone https://github.com/jyang234/verdi
$ cd verdi
$ go install ./cmd/verdi          # puts `verdi` on your PATH (GOBIN)
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

There is no `verdi init`: a valid store is just `.verdi/verdi.yaml` plus git.
From the root of any git repository —

```console
$ mkdir -p .verdi
$ cat > .verdi/verdi.yaml <<'YAML'
schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
YAML
$ git add .verdi/verdi.yaml && git commit -m "adopt verdi"
```

Set `forge` and the `providers` block to your own forge and tracker. Then cut
your first feature and open its board:

```console
$ verdi design start --kind feature --name my-first-feature
design start: no toolchain configured (verdi.yaml toolchain: block, I-4); skipping baseline regeneration
design start: created branch design/my-first-feature
design start: scaffolded spec/my-first-feature (kind: feature, status: draft)
design start: board: http://127.0.0.1:4173/board/spec/my-first-feature (run `verdi serve` from this checkout)
$ verdi serve      # edit the draft spec and its board at http://127.0.0.1:4173
```

`design start` scaffolds a draft spec under `.verdi/specs/active/` on a new
`design/…` branch. Author the spec, graduate stickies on the board, then
`verdi accept spec/my-first-feature` freezes it (merging the spec's MR is
acceptance). A story is the same flow with `--kind story` and a required
tracker ref (`verdi design start jira:LOAN-42 --kind story --name …`).

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
| `verdi design start <ref> --kind feature\|story --name <n>` | Cut a design branch; scaffold a draft spec and open its board |
| `verdi accept <spec>` | Flip `draft → accepted-pending-build` and freeze; merging the spec MR is acceptance |
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
