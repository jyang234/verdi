# Self-governance spike — answer

Resolves `spec/self-governance-spike` (`spec/self-governance#oq-1..oq-5`):
what `verdi policy adopt --starter` does to a copy of this store; whether
five CLAUDE.md ground rules fit the policy-claim grammar; whether the
enforcement rung is a schema change; whether an exemption's review window
does anything end to end today; and where the process-disclosure index
should live.

All committing rituals below ran in throwaway `/tmp` clones of the
`spike/self-governance` worktree, never in the worktree itself (lane
brief constraint 4). The main checkout's store was never touched. Every
number below carries the command that produced it and the evidence file
holding the (reduced) raw output, under `docs/spikes/self-governance/`.

## Recommendation, up front

Adopt the starter store on its own branch as spec/self-governance's plan
already intends (oq-1: safe, inert, zero new blockers either profile).
Build the ground rules as **policy claims with a new `rung` field on
`Claim`** exactly as ac-2 already commits to (oq-2: all five rules decode
as claims today, but only as inert labels — none is genuinely
checkable by any existing operator without that field; oq-3: it is a
schema change, cheap to draft, but its blast radius on committed
fixtures across five packages is real and should be budgeted, not
discovered mid-build). Do **not** expect the golangci-lint exemption to
do anything observable today (oq-4: proven no, both a lapsed and a
future window, byte-identical before/after). Record the disclosure index
as **a new record file in the journey `Blocker` shape**, not as
obligations and not left in the ledger (oq-5: obligations are the wrong
artifact shape and require a story spec that does not exist; the ledger
is free prose with no machine-detectable item boundary; a dedicated
record file is structurally ready the moment a small journey reader is
added, and is what dc-3 already names).

## oq-1 — adoption dry run

**Status: ANSWERED.**

Commands (full battery, both profiles, before and after adoption, exact
transcripts under `evidence/oq1-adoption/`, script at
`_scratch/sg-battery.sh`):

```
verdi lint
verdi model check
verdi journey --json spec/readiness-recovery
verdi journey --json spec/spec-documents
verdi journey --json spec/verdi-store-layout   (component-class probe)
verdi spec doc spec/readiness-recovery
verdi spec doc spec/spec-documents
verdi spec doc spec/verdi-store-layout
```

Adoption commits (`git show --stat`, `evidence/oq1-adoption/
adoption-commit-{solo,team}.show-stat.txt`): both profiles write and
commit exactly 4 files, 76 insertions, 0 deletions — `.verdi/policy/
constitution.md`, `.verdi/policy/policies/starter.md`, `.verdi/policy/
profiles/starter-{solo,team}.md`, `.verdi/constitution/consumers.json`.
The starter policy's `claims:` list is **empty** in both profiles (one
real rule — the `design_assistance` payload — and nothing else, by the
adopt verb's own design); the starter constitution's `subjects:` catalog
is **empty in all six families**. Nothing exists yet for any claim to
constrain.

Blocker-by-reason-code, `spec/readiness-recovery` and `spec/spec-documents`
(identical shape for both; full numbers and both profiles in
`evidence/oq1-adoption/SUMMARY.txt`):

| reason code | before (either profile) | after solo | after team |
|---|:---:|:---:|:---:|
| `obligation-countersign-unproven` | 1 | 1 | 1 |
| `obligation-fold-green-unproven` | 1 | 1 | 1 |
| `principal-resolution-unproven` | 1 | 1 | 1 |
| **total** | **3** | **3** | **3** |

**New blockers introduced: 0. Blockers cleared: 0.** The one observable
change is text-only: `principal-resolution-unproven`'s
`clearing_condition` narrows from "a governance profile is adopted and
the required principals resolve as authenticated" to "the required
principals resolve as authenticated" (the first half is now true —
`principals.profile_adopted` flips `false`→`true` and
`selected_profile_id`/`digest` populate — the blocker itself does not
clear because principal *authentication* is still unproven).
`spec/verdi-store-layout` (component-class) refuses identically before
and after: exit 2, `"journey: spec/verdi-store-layout is a component spec
… journey requires class: feature or class: story"`.

Lint and model-check failures verbatim: **none, either profile, before or
after.** `verdi lint` prints exactly one disclosure both before and after
— `disclosed-unproven [lint:VL-017]` (the mutable-zone notice, unrelated
to policy content, exit 0, 107 affected specs) — byte-identical in all
four runs (`evidence/oq1-adoption/lint-representative.stdout`). `verdi
model check` prints `model: OK — verdi.model/v1, 2 classes, 4
transitions, digest sha256:c08c9b15…` — the identical digest in all four
runs (`model-check-representative.stdout`): the model/lifecycle schema
does not read policy content. `verdi spec doc` output is byte-identical
before/after except the trailing `"Source: matrix over the working tree
at <sha>."` line, which simply names the new adoption commit.

**Answer:** adopting the starter store, either profile, on a copy of
this 28-active-spec repository is inert to lint, model check, and spec
doc, and downgrades exactly one blocker's remaining-condition text
without changing any reason code, any blocker count, or introducing any
new blocker. The two profiles are behaviorally identical on every probed
surface except the printed `design_assistance` mode line and the
populated profile id/digest — nothing readiness derives from that
difference today.

## oq-2 — expressibility of five ground rules

**Status: ANSWERED.**

Kernel files named by the story, verified present (`ls
internal/policyartifact/`): `payload.go`, `claim.go`, `grammar.go` all
exist as named — no discrepancy. (`grammar.go` turned out to define
`.verdi/policy/`'s *directory* grammar, not the claim payload grammar;
`claim.go` is where `Family`, `Operator`, and `Claim` actually live.)

All five rules were written as a real `verdi.policy/v1` artifact (one
claim each) and decoded through the actual exported seam,
`policyartifact.DecodePolicy` — never a hand-built `Claim{}` — from
`_scratch/oq2decode/main.go` (`go run ./docs/spikes/self-governance/
_scratch/oq2decode/`, full transcript `evidence/oq2-decode-output.txt`).

| # | CLAUDE.md rule | claim attempt (family / operator / subject / values) | decoder result | verdict |
|---|---|---|---|---|
| 1 | context is the first parameter of anything doing I/O | action / required-values / `io-function-signature` / `[context-first-parameter]` | **DECODED OK** | needs a new payload kind |
| 2 | errors wrap with `%w` | action / equals / `error-wrap-format-verb` / `[percent-w]` | **DECODED OK** | needs a new payload kind |
| 3 | no god packages ("two sentences to describe is two packages") | configuration / equals / `package-scope-discipline` / `[single-concern]` | **DECODED OK** | cannot be meaningfully expressed today (needs oq-3's rung+sentence, not a payload) |
| 4 | every commit builds | action / required-values / `commit-build-status` / `[clean-exit]` | **DECODED OK** | fits as-is |
| 5 | never bare git stash | action / forbidden-values / `git-stash-invocation` / `[bare]` | **DECODED OK** | fits as-is |

All five decode and validate through the real grammar as it stands — the
grammar is purely structural (kebab-case shapes, operator/operand arity,
closed enums) and has no concept of "does this claim's content actually
mean what its author intends," so decode success alone is a low bar
every rule clears. The verdicts above turn on a question the decoder
cannot answer: does any of the eleven closed operators' *declared*
comparison semantics correspond to what the rule actually says, well
enough that a future Wave-3 evaluator could check it against a real
repository fact using only today's vocabulary?

- **Rules 4 and 5 are the genuine fits.** "Every commit builds" is a
  gate's own pass/fail verdict (PA-016's pre-commit hook already exists)
  — `required-values`/`clean-exit` names a real, eventually-resolvable
  fact, modeled directly on the committed fixture's own
  `verify-required` claim (`internal/policyartifact/testdata/store/
  policies/go-toolchain.md`). "Never bare git stash" is a literal
  prohibition — `forbidden-values`' declared meaning ("must not be any of
  these") matches "never do X" without a stretch.
- **Rules 1 and 2 have no operator affinity at all**, today or ever,
  under this vocabulary: no operator inspects a Go function's parameter
  list or an `Errorf` call's format string, and DC-5 forbids a project
  from ever registering a new operator (only Verdi can). Their only
  honest mechanical enforcement is external (a golangci-lint analyzer —
  `contextcheck`, `errorlint` or similar) — a `ground_rule`-shaped
  **new payload kind** naming that analyzer would be a true statement;
  cramming them into `family`/`operator`/`values` produces only a
  syntactically-legal label. Proven, not asserted: the same harness
  probed whether such a kind is registered today —
  `payloads: {ground_rule: {...}}` decodes to `policyartifact: unknown
  payload kind "ground_rule" (typed feature payloads must be registered;
  there is no untyped fallback)` (`evidence/oq2-decode-output.txt`,
  bottom). Building that home is real, non-trivial work (a new Go type
  registered from `init()`), not a free alternative to oq-3's field.
- **Rule 3 is a human-judgment heuristic** ("a package you would need two
  sentences to describe is two packages") — dc-2 already names exactly
  this shape as the prose rung's job: a claim that exists to be
  *declared and countable*, with a sentence a human can hunt with, never
  mechanically checked. It needs oq-3's `rung`/`violation_looks_like`
  fields on `Claim` itself, not a payload — a payload would not make it
  any more checkable.

One general caveat applies to all five and is recorded once here rather
than five times: `policyartifact.DecodePolicy` has no subject registry,
but the real loader (`policyauthority.Load`, what `verdi lint` and `verdi
journey` actually call) does — every one of these five invented subjects
would additionally need registering in the adopted constitution's
`subjects:` catalog before a real CLI run would accept it (proven in
oq-4's evidence, `subject-not-registered-refusal.txt`). Decoding through
`policyartifact` alone is necessary but not sufficient for a claim to be
usable.

**Note for the owner, not a recommendation:** ac-2 already commits ground
rules to being policy *claims*; this spike does not question that. But
rules 1–3's total lack of operator affinity is worth naming plainly: an
alternative this research surfaced — a single self-governance-owned
`ground_rules` **payload** carrying `{id, rung, violation_looks_like,
enforcing_mechanism}` per rule, registered exactly like ASD's
`design_assistance` payload — would need zero changes to
`internal/policyartifact` or any ratification of context-integrity-v2 or
02-artifact-contract at all (payloads are the feature-extension escape
hatch that bypasses kernel ratification by design, per `payload.go`'s own
doc comment). It is a smaller-blast-radius shape than oq-3's field for
the two rules (1, 2) that no operator will ever check anyway. Recorded
for completeness because it is a real, cheaper alternative for at least
a subset of rules — not something this spike is authorized to decide in
place of ac-2.

## oq-3 — is the rung a schema change?

**Status: ANSWERED-WITH-CAVEAT** (caveat: the amendment-order half turns
on which spec co-1's "artifact contract" names; both readings are given).

**Attribute or schema:** schema. Every existing `Claim` attribute already
carries fixed, unrelated meaning: `Family` and `Operator` are closed
Verdi-owned vocabularies (DC-5: a project can never register either);
`Subject`/`Values` are the comparison operands; `Bound` is numeric and
tied specifically to `minimum`/`maximum`; `Scope` is applicability, not
strength; `Overridable` is DC-3's distinct overlay-refinement bit. None
is a free closed-enum slot. Proven, not just read: a draft `Rung` field
(`unrepresentable`/`analyzer`/`gate-assertion`/`review`/`prose`, plus an
optional `violation_looks_like` string required exactly on the
`review`/`prose` rungs per dc-2) was added to `Claim`/`claimDoc` in
`internal/policyartifact/claim.go`, `go build ./...` still exits 0 (every
existing call site uses keyed struct literals, so a new field compiles
everywhere unchanged), and the diff was captured, then reverted before
any commit — `oq3-rung-field.patch` (154 lines) at the top of this
directory; `git status --porcelain` was clean again immediately after
(verified).

**Touched-sites list**, measured by making the field *mandatory* and
running the real suite (`evidence/oq3-rung-field-blast-radius.txt`):
`go test ./...` goes from green to **480 failing test cases across 41
packages** (up from the single `internal/policyartifact` package this
story anticipated). Every failure is the same shape —
`policyartifact: policy.claims[0].rung is missing` — from two sources:

- **Hand-built `Claim{}` Go literals** that never set the new field
  (zero-valued `Rung("")` fails the closed-enum check): dozens of cases
  each in `internal/policyartifact` (`claim_test.go`), `internal/
  policyauthority`, `internal/policyconflict`, and `internal/
  contextcompile` — every package that constructs or decodes a real
  `policyartifact.Claim` value in its own tests.
- **Committed YAML fixtures** missing a `rung:` line: `internal/
  policyartifact/testdata/store/policies/go-toolchain.md` and its sibling
  fixtures (`TestFixtureDigests_Ratchet`, which also ratchets `testdata/
  golden-digests.json` — every existing fixture's digest changes the
  moment a mandatory field is added, so that golden file needs
  regenerating too), plus `TestDecodePolicy_Happy` and eight
  `TestDecodePolicy_Negative` subtests whose fixture YAML predates the
  field.

Not broken by the same change, and worth naming precisely: `go build
./...` (every consumer uses keyed literals) and `internal/humanartifact`'s
own test suite (its scaffold round-trip works at a level the new field
did not reach in this draft — a real implementation should still add a
row for it, but nothing failed empirically). `internal/lint` has zero
existing touch points on policy content at all (grep-verified: no VL-0xx
rule references `Policy`/`Claim`), so ac-2's "a missing sentence on those
rungs is a lint finding" is wholly new code, not a modification.

**Consumer that counts rules per rung:** none exists today. `cmd/verdi/
policy.go` implements only `adopt` (`cmdPolicy` refuses any other
subcommand); no MCP tool references policy content at all
(grep-verified across `internal/mcpserve`); no lint rule touches policy
content. The nearest planned home is exactly what spec/self-governance
ac-3 already names: a new `verdi policy` subcommand plus an MCP policy
tool — genuinely new code either way, not a repurposed existing surface.

**Amendment order — the caveat.** co-1 says: "if the enforcement rung
needs a schema field, the artifact contract amendment lands first."
`Claim` does **not** embed `kernelDoc` (unlike `Policy`, `Exemption`,
`Disposition`) — it carries no `schema`/`id`/`kind`/`title`/`owners`/
`template` envelope at all; its own field set (`family`, `operator`,
`subject`, `values`, `bound`, `scope`, `overridable`) is owned, by the
package's own doc comment, by **spec/context-integrity-v2** ("AC-1:
'typed claims'"), not by the universal per-artifact kernel envelope
`docs/design/specs/02-artifact-contract.md` defines and every artifact
kind shares. Structurally, adding `rung` to `Claim` is a
context-integrity-v2 amendment, and nothing here requires touching
02-artifact-contract first — unless co-1's "artifact contract" is using
that phrase loosely to mean "whichever spec owns this schema" rather than
naming the numbered document, in which case co-1 and this reading agree
trivially. This spike cannot resolve which co-1 meant from the text
alone; spec/self-governance's author should confirm the referent before
treating the amendment order as settled. Either way, co-1's outer rule
("through the ratification flow") is unaffected: whichever spec owns the
field, it is ratified before the build, not coded first and ratified
after.

## oq-4 — exemption review window, end to end

**Status: ANSWERED** (no, on a real build rule, proven with both a
lapsed and a future window).

Setup, in a throwaway clone (`/tmp/spike-sg-exempt`, adopted solo
profile): the starter policy ships `claims: []` and the starter
constitution ships zero registered subjects in every family (oq-1), so
this experiment first hand-added one real claim — `golangci-lint-standard-
set` (family `configuration`, operator `equals`, subject
`golangci-lint-linter-set`, value `standard`) — modeling process-audit
PA-004/PA-016's actual golangci-lint parity exception. The first attempt
to load it refused: `policyauthority: policy policy/starter claim
golangci-lint-standard-set: subject "golangci-lint-linter-set" is not
registered for family "configuration" in the constitution subject
catalog` (exit 2) — a second, independent reason the starter store is
inert beyond having zero claims (`evidence/oq4-exemption/
subject-not-registered-refusal.txt`). Registering the subject in
`constitution.md`'s `subjects.configuration` cleared it.

An exemption was then authored witnessing that claim's real
`ClaimDigest` (computed through the same real decoder,
`_scratch/oq4digest/`, never a synthetic value — modeled on the
committed `legacy-service-go.md` fixture's own exact-witness discipline),
first with `expiry: "2026-01-01"` (lapsed as of 2026-09-20) and then, in
the same clone, with `expiry: "2027-06-30"` (future).

Two full transcripts (`evidence/oq4-exemption/{lapsed,future}-
{lint,journey}.std{out,err}`):

| | `verdi lint` exit | `verdi journey --json` exit | either mentions the exemption? |
|---|:---:|:---:|---|
| lapsed window (2026-01-01) | 0 | 0 | no — `grep exempt` on both outputs: no match |
| future window (2027-06-30) | 0 | 0 | no — byte-identical to the lapsed run |

`diff` between the lapsed and future runs is empty for both commands —
the review window's direction has **zero** observable effect on either
surface today. `diff` between lint with and without the exemption file
present at all is also empty (`no-exemption-lint.stdout`). `verdi
journey`'s blocker list is the unchanged 3-blocker baseline from oq-1 in
both cases.

This is not a shape gap in the exemption artifact — `Expiry`/
`ReviewCondition` decode and validate exactly as documented (DC-8: at
least one required). It is a wiring gap: `internal/journey`'s closed
`ReasonCode` vocabulary (`reason.go`, 9 codes, all
`obligation-*`/`principal-*`/`forge-facts`/`lifecycle-state`/
`default-branch`) has **no code for an exemption at all**, lapsed or not
— confirmed by reading the closed map (exhaustive, no catch-all) and then
by running both windows and finding no trace. `internal/journey` imports
`policyauthority` only for *profile* loading (`port.go`, `project.go`);
it never imports `policyartifact` or `policyconflict`, so nothing in the
readiness derivation path is positioned to notice an exemption exists,
let alone whether its window has lapsed. The actual bound-checking logic
(`policyconflict/authority.go`'s `resolveBound`: proven / violated /
unproven by expiry vs. review condition) exists and is exercised by that
package's own tests — it is simply never called from `verdi lint` or
`verdi journey` today.

**Answer: no**, an exemption's review window does not work end to end
today for a build rule — not because the exemption artifact is broken,
but because neither surface the story names reads exemptions at all yet.
This turns oq-4 into the defect against spec/readiness-recovery ac-1 the
story's own "Spec seed" anticipates, not a confirmation of ac-4.

## oq-5 — home of the process-disclosure index

**Status: ANSWERED** (recommend candidate (b): a new record file in the
journey `Blocker` shape).

**Source extraction.** `grep -ni 'deferred\|residual\|parked'` against
the wave-1 SDD ledger (read-only, never modified:
`verdi-wt/readiness-recovery-w1/.superpowers/sdd/
2026-09-19-readiness-recovery-wave-1/progress.md`) returns exactly **9
lines** — matching spec/self-governance's own count. Those 9 lines carry
**more than nine items**: hand-extracting every clause actually marked
deferred/residual/parked (as opposed to a resolved clause riding in the
same line, e.g. line 29's "M7 exported constructors ride") gives 21 raw
clauses, one of which (the M5 double-index-walk note) is a verbatim
restatement across two lines — **20 distinct items**. Full enumeration
with line citations: `disclosures.md`, this directory. This is the
discrepancy recorded under "Deviations" below.

**Tried against all three candidates**, judged by the story's own three
criteria:

| candidate | appears in `verdi journey --json`? | has owner + clearing condition? | survives the ledger folder's deletion? |
|---|---|---|---|
| (a) obligation under a self-hosted spec | no (proven: not even reachable — see below) | no (wrong fields entirely) | yes (independent location) |
| (b) new record file, blocker shape | no *today* (proven, unwired) — structurally ready | **yes, natively** | yes (independent location) |
| (c) parsed from the ledger by a scratch gate test | no (nothing in `.verdi` changed) | no (free prose, no field boundary) | **no** (the ledger folder *is* the only home) |

- **(a) obligations — tried, refused.** `verdi obligation author
  spec/readiness-recovery ac-1 static` refuses: `spec/readiness-recovery
  resolves to a feature-class spec, not a story` (exit 1,
  `evidence/oq5-candidate-a/feature-class-refusal.txt`) — the verb only
  scaffolds obligations against a *story* spec's declared-evidence
  acceptance criteria, and wave-1 has no story spec of its own (it ran
  from a plan document against the feature spec directly). A real
  successful scaffold was also run for shape comparison
  (`verdi obligation author spec/disclosure-seam ac-1 behavioral`,
  `successful-scaffold-shape-demo.md`): the artifact's real fields are
  `title`/`owners`/`for_kind`/`quality.state`/`links`/`frozen`, and its
  body's fixed job is "state what this evidence must specifically show"
  — there is no `reason` or `clearing_condition` field anywhere in the
  shape. Recording these 20 items here would require inventing a story
  spec purely as an attachment point, then writing 20 test-evidence
  scaffolds for content that is not test evidence.
- **(b) a new record file, blocker shape — tried, structurally the best
  fit.** All 20 items were recorded as real `journey.Blocker`-shaped JSON
  (`id`, `reason`, `class`, `witnesses`, `owner`, `clearing_condition`,
  `transition` — every field dc-3 asks for) at
  `evidence/oq5-candidate-b/wave-1-readiness-recovery.json`, placed under
  a plausible new path (`.verdi/process-disclosures/`). `verdi journey
  --json spec/readiness-recovery` was then run with that file present:
  the blocker list is the unchanged 3-item baseline, and the string
  `"process-disclosure"` does not appear anywhere in the output
  (`journey-with-record-present.stdout`, exit 0) — confirmed rather than
  assumed. This is expected: `internal/store/paths.go` names no such
  directory, so nothing reads it yet. The reason code used
  (`process-disclosure-open`) is a placeholder — it is not in
  `journey.ReasonCode`'s closed 9-value set and would need one real
  ledger entry there (a small, single-file change, unlike oq-3's field).
  What this candidate proves is that the *shape* is ready the moment a
  reader exists — nothing about the record itself needs to change later.
- **(c) parsed from the ledger by a scratch gate test — tried,
  fragile.** `_scratch/oq5ledger/ledger_test.go` parses a committed COPY
  of the ledger (`testdata/progress.md`, taken verbatim at the moment
  this spike read the live file) and proves the line count (9, robust)
  and then *demonstrates* — rather than hides — the candidate's central
  weakness: a naive mechanical split of the 9 matched lines produces 32
  clauses, not the 20 a human curated, because ledger prose has no
  machine-detectable boundary between a deferred clause and an unrelated
  sentence sharing its line (`evidence/oq5-candidate-c-ledger-test.txt`,
  all 3 subtests pass — the third proves no line contains a structured
  `reason:`/`owner:`/`clearing_condition:` field at all). `verdi journey
  --json` is unaffected by construction: this candidate touches nothing
  under `.verdi/`. And by definition it does not survive the ledger
  folder's deletion — leaving the items in the ledger, as the candidate's
  own name says, means the ledger *is* the only copy.

**Recommendation:** (b). It is the only candidate with the right fields
natively, the only one whose failure to surface today is "not wired yet"
rather than "wrong shape," and the only one independent of the SDD
ledger's own lifecycle. Wiring `verdi journey` to read it, and adding one
real `ReasonCode`, is real but small follow-on work — smaller than either
inventing a story spec for (a) or building robust prose-parsing for (c).

## Deviations from the investigation plan

- The story's "nine deferred, residual and parked items" names the wave-1
  ledger's matching-*line* count correctly (verified: exactly 9 lines).
  The *item* count within those lines is 20 distinct entries (21 raw
  clauses, one exact duplicate) — this README and `disclosures.md` record
  and reconcile both counts rather than silently picking one.
- `internal/policyartifact/grammar.go` exists as named but defines
  `.verdi/policy/`'s directory-entry classification, not the claim
  payload grammar the investigation plan's step 2 implies; the claim
  grammar itself lives in `claim.go` (confirmed present, as named) and
  is what this spike actually read for oq-2/oq-3.
- oq-4's "review window" is `Exemption.Expiry` (the only actual
  calendar-bound field); `ReviewCondition` is free text with no date
  semantics of its own. Both fields exist and either can independently
  bound a departure (DC-8); this spike exercised `Expiry` because only a
  date can be "lapsed" or "future" in the sense oq-4 asks for.
- oq-1's investigation plan names "one component spec" without choosing
  one; this spike used `spec/verdi-store-layout` (`class: component`,
  confirmed via frontmatter) — any of the ~15 other component-class specs
  in the store would very likely behave identically, since the
  component-class refusal in `verdi journey` and the source-line-only
  diff in `verdi spec doc` are both class-driven, not content-driven.
- The clones' `git clone` from the worktree set `origin/HEAD` to
  `origin/spike/self-governance` (refs are shared across every worktree
  of this repository, including branch tips), not `origin/main` — `git
  remote set-head origin main` (the lane brief's own documented remedy)
  was applied in every clone before running `policy adopt`, and both
  branches' tips were confirmed identical (`5f60c76c`) beforehand, so
  this had no effect on content, only on which ref's name `policy adopt`
  printed as its resolved base.
- An unrelated Claude API billing error interrupted this session mid-way
  through the team-profile adoption run (after the solo battery had
  fully completed). No spike state was lost; the interrupted command was
  simply re-issued once billing recovered, and both clones' git state was
  re-verified against the pre-interruption record before continuing.

## Evidence index

- `evidence/oq1-adoption/` — adoption commit stats, full battery
  transcripts, blocker table, timing/load disclosure, re-runnable script
  reference.
- `evidence/oq2-decode-output.txt` — all five claim decodes plus the
  payload-kind probe, verbatim.
- `oq3-rung-field.patch` (top of this directory) — the draft field,
  captured and reverted; `evidence/oq3-rung-field-blast-radius.txt` — the
  480-test/41-package blast radius, measured.
- `evidence/oq4-exemption/` — the claim, the registered subject, both
  exemption variants, all four transcripts, and the subject-catalog
  refusal.
- `evidence/oq5-candidate-{a,b}/`, `evidence/oq5-candidate-c-ledger-
  test.txt`, `disclosures.md` — all three candidates tried, the item
  enumeration, and the recommendation's supporting transcripts.
- `_scratch/` — `oq2decode/`, `oq4digest/`, `oq5ledger/` (Go, gofmt-clean,
  excluded from `./...` by its underscore prefix), `sg-battery.sh` (the
  oq-1 battery, re-runnable).
