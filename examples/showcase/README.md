# examples/showcase — the LoanServ store

This is a real, lint-clean Verdi store — not a screenshot, not a mockup.
Every file under `.verdi/` here is what a Verdi store actually looks like
after a mid-size team has used it for about a year. It is also, by design,
the one corpus Verdi's own end-to-end suite drives: the happy-path fixture
for every capability *is* this store, so a new capability that ships
without touching this tree fails `make verify` (`make showcase-coverage`)
rather than drifting silently out of the public example.

The narrative: **LoanServ**, a fictional loan-servicing team, adopted
Verdi about a year before this snapshot. Their system is seven services —
`loansvc` at the center, publishing outbox events to `notification-svc`,
`payments-gw`, `escrow-svc`, `rate-engine`, `doc-vault`, and
`borrower-portal` — and their store tracks that system's design history
the same way their git history tracks its code: specs for what each
feature and story is supposed to do, ADRs for the architectural calls
that bind across features, and evidence (attestations, obligations,
waivers) for what has actually been proven. Four features sit at four
different points in the lifecycle at once — one closed and archived, one
built with evidence flowing, one accepted and waiting on stories, one
still in design — because that is what a store looks like mid-flight, not
just at a demo checkpoint.

Read `spec/stale-decline` first if you want the deep tour: it is the
richest feature here — four acceptance criteria declaring static,
static+behavioral, behavioral, and runtime evidence between them, plus
the feature's own outcome attestation rounding out all four evidence
kinds this corpus recognizes, three implementing stories (including a
spike), a mid-build deviation with a real disposition, and a design that
traces straight back to `adr/0002-outbox-events` and the incident that
produced it. Everything else in this store either supports that narrative directly
or demonstrates a capability `stale-decline` alone doesn't reach — a
supersession chain, an audited exemption, a live design draft.

## Two zones

**The committed tree** — everything under this directory — is the public,
permanent half of the store: specs, ADRs, decisions, evidence, and the two
diagrams, all lint-clean and individually vetted (`docs/showcase-vetting.md`
records the three-column bar — lint-clean, editorially exemplary,
narrative-coherent — for every file here). This is what you clone and
browse; it never changes shape at runtime.

**Design branches** are the other half, and they exist because VL-004
forbids committing a draft: a spec mid-authorship, an open board with
stickies still being triaged, a diagram proposal still being verified
against the real topology, all belong to the *design* stage, which by
construction has nothing committable yet. Verdi's e2e harness
(`cmd/e2eharness`) provisions these on deterministic, named branches on
top of this same committed tree — same LoanServ services, same ADRs, same
canon — so the workbench's live-draft surfaces (the board, the diagram
proposal editor, the directory's draft-boards view) have something real
to show without ever putting an uncommittable artifact in the tree you're
reading. A branch like this is documented and vetted exactly like a
committed file; it just lives one `git checkout` away instead of on disk
here. If you're driving the workbench locally, `verdi serve` on this store
and follow the directory page's draft-boards link to find one.

## How `layers.txt` builds this history

Every commit SHA that appears in this store — every `frozen.commit`, every
pinned `ref@sha`, every `derived/<slug>/<sha>/` directory name — has to be
a real commit in a real git history, or the artifacts that cite it
wouldn't decode as the honest point-in-time records they claim to be
(02 §Generated artifacts and digests). But the tree you're looking at
right now isn't a git repository by itself — `examples/showcase/` is a
working directory, checked into *this* repository's own history.

`layers.txt` is the bridge: a manifest, `<layer> <path>` per line, that
groups this store's files into the deterministic git history a real
Verdi store would have. `internal/fixturegit` builds that history once,
layer by layer — layer 1 is content that pins nothing (nothing exists
yet to pin), layer 2's files pin layer 1's resulting commit, layer 3
pins layer 2's, and so on — and the resulting SHAs are golden: baked
into `internal/corpus/corpus_test.go` once and checked on every test run,
so they never drift silently. Five layers exist here today: an
unfrozen foundation (the component specs, both diagrams, the one
still-`proposed` ADR, the store manifest); the frozen `stale-decline`
feature and its first ADR pair; an archived closed quartet; a
supersession-chain and round-four-archive layer folded in from the
store's dex-only fixtures; and the newest ADR pair. Any tooling that
needs this store as a *real* git repository — the unit fixture gates, the
dex and workbench test harnesses, the lint self-check — builds it fresh
from `layers.txt` every run; nothing about the deterministic history is
itself committed here.

## Linting this store

`make verify`'s `lint-store` step self-lints *this repository's own*
`.verdi/`, not this fixture — proving `examples/showcase` itself lint-clean
needs a real, git-real checkout, for the same reason the previous section
just gave: frozen stamps and pinned refs only resolve against actual git
history. This store's committed content is split, by design, across two
independent `internal/fixturegit` histories — the `layers.txt` history
above, and a second, dedicated, unchained history for the
`loan-workflow`/`loan-workflow-v2`/`rate-lock`/`rate-lock-v2` pairs and the
`escrow-autopay` cluster (`internal/artifact/v2fixture_test.go`'s own
golden SHAs) — because merging both into one chained repo would force
re-deriving every SHA in whichever one is chained second, contradicting
the "build once, bake in, test forever" pins both already carry. So the
gate rebuilds each history separately and lints each independently. For the
`layers.txt` family, `internal/corpus/corpus_test.go`'s
`TestFixtureRepo_MatchesGoldenSHAs` proves the fixturegit rebuild is
golden-SHA-stable (it asserts SHAs, the pin-resolution precondition — not
lint) and `internal/lint`'s `TestClean_CorpusLintsGreen` is the lint-clean
proof over that rebuilt tree; for the second,
`internal/lint/v2clean_test.go`'s `TestV2FixtureCorpus_LintsClean`. Both
exit 0 with the mutable zone present, and with it removed (simulating a bare
clone) report only `SeverityDisclosure` (VL-017) notices on this store's
new-class specs, never a verdict failure. A single-commit checkout (the
shape `cmd/e2eharness` provisions for the Playwright/e2e suite, since those
tests need real git history for other reasons — `gitx`'s commit/diff
primitives — not a `verdi lint` proof) collapses both histories' pins into
one new commit and cannot resolve them; the resulting VL-009/VL-003/VL-015
noise on that construction is a structural property of collapsing history,
not a defect in any file it reports on (`docs/showcase-vetting.md`'s Task
1.8 section has the full accounting).

## The link-type map

02 §Link taxonomy's closed vocabulary carries **eleven** typed edges (not
nine — an earlier draft of this file undercounted; `internal/artifact`'s
own `LinkType` enum and its "one of the eleven known link types" doc
comment are the count that actually ships). Every one of the eleven has a
natural exemplar already living in this tree — the audit this task ran
found no gap needing a bolt-on instance:

| Type | Exemplar |
|---|---|
| `implements` | `spec/borrower-update-api` → `spec/stale-decline#ac-2` |
| `resolves` | `spec/borrower-update-mobile-spike` → `spec/escrow-autopay#oq-1` |
| `supersedes` | `adr/0002-outbox-events` → `adr/0001-outbox-events` |
| `exempts` | `spec/escrow-autopay` (`dc-1`) → `adr/0001-outbox-events` |
| `verifies` | `attestation/jira-loan-1482--ac-2` → `spec/stale-decline` |
| `derived-from` | `diagram/loansvc-topology` → `spec/store-layout-notes` |
| `annotates` | `conflict/stale-decline-incident` → `adr/0003-retry-policy` |
| `depends-on` | `diagram/loansvc-topology` → `adr/0005-event-schema-registry` |
| `story` | `spec/stale-decline` → `jira:LOAN-1482` |
| `impacts` | `spec/stale-decline` → `svc/loansvc/boundary-contract` |
| `challenges` | `conflict/pii-outbox-leak` → `adr/0002-outbox-events` |

(`evidence-for` is not a twelfth link type: it's a `verdi.bindings.yaml`
field name for a producer→AC join, not an entry in the `links:` edge
vocabulary — a distinction worth naming since it's an easy one to
misremember, as this file itself once did.)

## Diagrams: two tiers you can see today

`diagrams/loansvc-topology.mermaid` is the **full** topology — all seven
LoanServ services, every edge agreeing with the `declares.boundaries`
blocks in `spec/stale-decline` and `spec/escrow-autopay` and with
`adr/0005-event-schema-registry`'s own account of the graph, each edge
labeled with the outbox event class and schema version it depends on.
`diagrams/borrower-journey.mermaid` is **illustrative** — a sequence
diagram of a borrower's actual round trip through escrow autopay
enrollment and a scheduled charge that fails and retries, the kind of
picture no topology extractor could ever verify (a sequence diagram has
no truth generator, spec/illustrative-class dc-2), and dex badges it
exactly that way: "illustrative · not deterministically verifiable". The third tier —
a `class: proposal` diagram, machine-verified against regenerated
truth — lives on a design branch, not the committed tree, for the same
VL-004 reason every other in-progress artifact does (see "Two zones"
above).

## Trace these threads

- **An AC, all the way to proof.** `spec/stale-decline#ac-2` ("loansvc
  retries the charge API through the outbox... exactly once per decline")
  is implemented by `spec/borrower-update-api` (its own `ac-1` carries the `implements` edge), which carries two
  obligations — `obligations/borrower-update-api/ac-1--static.md` and
  `ac-1--behavioral.md` — each a concrete, checkable claim rather than a
  restated AC. The feature-level outcome floor for the same AC is
  `attestations/jira-loan-1482/ac-2.md`, the QA lead's direct sign-off.
  Follow the `verified-by`/`implemented-by` backlinks from
  `spec/stale-decline`'s own dex page to walk the whole chain in the
  other direction.
- **A decision, superseded with a named reason.**
  `adr/0001-outbox-events` (accepted 2025-08-20, synchronous dual-write)
  is superseded by `adr/0002-outbox-events` (accepted 2025-11-05), and
  the reason is on the record, not implied: a mid-request failover on
  notification-svc in October 2025 left a batch of stale-decline notices
  ambiguously delivered, and the retry that followed re-sent every one of
  them. `adr/0002`'s own Context section narrates the incident;
  `spec/stale-decline`'s Design notes independently narrate the same
  causal chain, cross-checked rather than restated.
- **A deliberate scar.** `spec/borrower-update-mobile` carries the
  `spec-stale` badge on purpose: its mid-build deviation
  (`deviation-report.md`, an accepted client-retry-loop divergence dated
  2026-07-12) is exactly the kind of drift the badge exists to surface,
  not hide. `verdi audit` against this store exits non-zero because of
  it — a witnessed, disclosed "violated-with-witness" outcome, not a bug
  in the fixture. A second scar sits beside it:
  `conflicts/pii-outbox-leak.md` was filed against `adr/0002-outbox-events`
  over unredacted PII in the outbox log, and its `status: superseded` the
  same day `adr/0004-pii-redaction-at-ingest` was accepted is this
  store's one demonstrated "conflict settled by a later decision," not a
  second unresolved dispute.

## Vetting

Every artifact in this tree earned its place against a three-column bar —
lint-clean, editorially exemplary, narrative-coherent — recorded per file
in `docs/showcase-vetting.md`. Nothing here is a stub kept around
because deleting it was more work than writing real content.
