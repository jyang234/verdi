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
schema change, cheap to draft, but its blast radius — measured at ~480
failing test cases across roughly 20 packages, not five — is real and
should be budgeted, not discovered mid-build). The golangci-lint
exemption's review window is read by `verdi lint`/`verdi journey`
nowhere, but IS read by the policy-conflict path (`context conflict`,
`gate`, `close`, `build start`, the readiness snapshot) and does exactly
what dc-8 promises there, proven both ways (oq-4, revised in fix round
1: this is a wiring gap between two existing surfaces, not a dead
mechanism). Record the disclosure index as **a new record file in the
journey `Blocker` shape**, not as obligations and not left in the ledger
(oq-5: obligations are the wrong artifact shape and require a story spec
that does not exist; the ledger is free prose with no
machine-detectable item boundary; a dedicated record file is
structurally ready the moment a small journey reader is added, and is
what dc-3 already names).

**Fix round 1 (independent review, `lane-self-governance-review.md`):**
seven findings, all accepted by the controller and addressed below in
place, each marked where it lands — F1 (blast-radius package count was
double-counted), F2 (oq-4's "no" was over-generalized past the two
surfaces measured; the policy-conflict path reads the window and this
round proves it with new transcripts), F3 (an oq-3 "no MCP tool"
sentence was false — three `constitution_*` tools exist), F4 (every
number now carries its command), F5 (28 active specs → 39, measured),
F6 (a source path in the oq-5 exemplar record pointed nowhere real),
F7 (the blast-radius failure shapes are broken down, not asserted
uniform).

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
this repository — `ls -d .verdi/specs/active/*/ | wc -l` = **39** active
spec directories today, not the story's own "28" (fix round 1, F5: the
story's figure is unverified in its own text and this spike never
checked it before now; the two numbers are not reconciled further here —
see "Deviations" — but inertness does not depend on which is right,
since every measurement below ran against the real, current store) — is
inert to lint, model check, and spec
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
running the real suite in a fresh clone (`evidence/
oq3-rung-field-blast-radius.txt`, fix round 1 rewrite — review finding
F1 caught the original number's methodology bug, F4 asks for the command
beside every figure, F7 asks for the shapes named, all three folded into
that one file and summarized here):

  git clone -q --no-hardlinks <lane worktree> /tmp/sg-fix-rung
  cd /tmp/sg-fix-rung && git apply oq3-rung-field.patch && go test ./... > testall.log 2>&1
  grep -c '^--- FAIL' testall.log            # => 480  (top-level cases; anchored excludes indented nested subtests)
  grep -cE '^FAIL[[:space:]]+github' testall.log   # => 20  (distinct packages; anchored excludes one nested false-positive, below)

`go test ./...` goes from green to **~480 failing top-level test cases
across 20 packages** (fix round 1 correction: the original evidence said
"41 packages" from `grep '^FAIL' | wc -l`, which double-counts — every
failing package prints both a bare `FAIL` trailer line and a
`FAIL<tab>pkg<tab>time` line, so an unanchored count runs to roughly
double the real figure). This spike's own two independent fresh-clone
re-derivations of this fix round agree with each other (480/20 both
times) but land one case and one package above the independent review's
own single re-derivation (479/19); the residual is disclosed, not
resolved, in the evidence file — most likely run-to-run variance in
`internal/sealedexec/claude`, the one failing package whose test spawns
a real `go test ./cmd/verdi` subprocess rather than running in-process.
Either figure is "the high hundreds across roughly a fifth of the
module's packages," which is what drives the recommendation; the exact
last digit does not.

Not one failure shape but (at least) three, all rooted in the same
change (counted over all 976 `--- FAIL` lines including nested subtests,
`grep -c -- '--- FAIL' testall.log`; review finding F7):

- **494 lines** — the field is absent from an encoded claim: `policyartifact: policy.claims[0].rung is missing` (`grep -cE 'policy\.claims\[[0-9]+\]\.rung is missing' testall.log`). Committed YAML fixtures and JSON round-trips.
- **310 lines** — the field is present but zero-valued: `policyartifact: unknown enforcement rung ""` (`grep -cE 'unknown enforcement rung \\?"\\?"' testall.log`). Hand-built `Claim{}` Go literals across `internal/policyartifact` (`claim_test.go`), `internal/policyauthority`, `internal/policyconflict`, and `internal/contextcompile` — every package that constructs a real `policyartifact.Claim` value in its own tests, never just the one package the story anticipated. **34 of these 310** are a distinct migration cost worth naming on their own, not just more of the same: `policyconflict: apply exemptions: row "row-1" claim [0] sha256:...: ...unknown enforcement rung ""` (`grep -cE 'apply exemptions:.*claim \[[0-9]+\]' testall.log`) — an EXISTING committed exemption fixture witnessing an EXISTING committed claim also breaks, so a real migration has to update every committed exemption fixture too, a cost `testdata/golden-digests.json`'s digest ratchet alone does not disclose.
- **54 lines** — a design-failure classification surfaces the same root cause through a different rendering: `authority-invalid: resolving effective policy` (CLI/log text) or the equivalent embedded `{"code":"authority-invalid","detail":"resolving effective policy",...}` (MCP tool-result JSON) (`grep -cE 'authority-invalid: resolving effective policy|"code":"authority-invalid"' testall.log`).
- **118 lines residual**, not itemized further (proportionate to a spike's timebox, not a migration audit): a sample is `recordvalidate_test.go`'s "no published record exercised evidence kind/origin/disposition/transform ..." coverage-completeness assertions, which fail as a side effect of upstream fixtures no longer decoding, not because they test `rung` directly.

Committed-fixture consequence, named once rather than per-shape:
`internal/policyartifact/testdata/store/policies/go-toolchain.md` and its
siblings (`TestFixtureDigests_Ratchet`, which also ratchets `testdata/
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

**Consumer that counts rules per rung:** none exists today — this
narrower claim survives fix round 1 (review finding F3); a broader one
this README made alongside it does not and is retracted here. `cmd/verdi/
policy.go` implements only `adopt` (`cmdPolicy` refuses any other
subcommand); no lint rule touches policy content (grep-verified: no
VL-0xx rule references `Policy`/`Claim`). But "no MCP tool references
policy content at all" was false, not merely imprecise: `internal/
mcpserve/tooldefs.go:300,307,314` register three real tools —
`constitution_inspect` (the accepted/proposed constitution's "complete
effective rule ledger, unflattened"), `constitution_validate` (strict
cross-validation of the proposed store), and `constitution_impact_review`
(diffs constitution layers through the policy-conflict path) — and
`internal/mcpserve/tool_experiment.go` imports both `policyartifact` and
`policyauthority` directly, calling `policyauthority.Load`/`Resolve` to
select applicable payloads for an experiment-policy decision. None of the
four counts rules **by rung** — `constitution_inspect` returns the whole
unflattened ledger, `constitution_impact_review` diffs the whole layer
set, and `tool_experiment.go`'s resolution is about a different,
already-registered payload kind (`experimentpolicy`, not ground rules)
— so the oq-3 answer itself (no per-rung counter exists) is unchanged;
only the "no MCP tool touches policy at all" evidence sentence was wrong,
and ac-3's "a new MCP policy tool" is new work landing NEXT TO three
existing constitution-facing tools, not onto a greenfield MCP surface.

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
trivially.

Fix round 1 (controller instruction): there is a third candidate referent
for "artifact contract," and it is checked, not just named. This store
self-hosts a component spec at the same name, `spec/verdi-artifact-
contract` (`schema: verdi.artifact/v1`, "Identity and references,"
"lint" — the same subject matter as the workspace-root 02-artifact-
contract.md, self-hosted rather than imported). Its full 713 lines carry
**zero** occurrences of "policy claim," "Claim," "policyartifact," or
"enforcement rung" (`grep -n 'policy claim\|Claim\b\|policyartifact\|
enforcement rung' .verdi/specs/active/verdi-artifact-contract/spec.md` —
one incidental hit, "constitution 2," a citation of the workspace
constitution's clause 2, unrelated to policy-authority constitution
artifacts). So this third referent does not cover policy claims either,
and the lane's context-integrity-v2 reading stands against it too — but
its existence means co-1's ambiguity is now three-way (02-artifact-
contract.md, spec/verdi-artifact-contract, spec/context-integrity-v2),
not two-way, and none of the first two is where a rung field would
actually land. This spike cannot resolve which co-1 meant from the text
alone; spec/self-governance's author should confirm the referent before
treating the amendment order as settled. Either way, co-1's outer rule
("through the ratification flow") is unaffected: whichever spec owns the
field, it is ratified before the build, not coded first and ratified
after.

## oq-4 — exemption review window, end to end

**Status: ANSWERED** (yes on the policy-conflict path; no on lint/journey
— revised in fix round 1, review finding F2, from an over-generalized
"no" to a scoped answer naming both).

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

### `verdi lint` / `verdi journey` — no, confirmed unchanged

Two full transcripts (`evidence/oq4-exemption/{lapsed,future}-
{lint,journey}.std{out,err}`):

| | `verdi lint` exit | `verdi journey --json` exit | either mentions the exemption? |
|---|:---:|:---:|---|
| lapsed window (2026-01-01) | 0 | 0 | no — `grep exempt` on both outputs: no match |
| future window (2027-06-30) | 0 | 0 | no — byte-identical to the lapsed run |

`diff` between the lapsed and future runs is empty for both commands —
the review window's direction has **zero** observable effect on either
surface. `diff` between lint with and without the exemption file present
at all is also empty (`no-exemption-lint.stdout`). `verdi journey`'s
blocker list is the unchanged 3-blocker baseline from oq-1 in both cases.
`internal/journey`'s closed `ReasonCode` vocabulary (`reason.go`, 9
codes, all `obligation-*`/`principal-*`/`forge-facts`/`lifecycle-state`/
`default-branch`) has no code for an exemption at all; `internal/journey`
imports `policyauthority` only for *profile* loading (`port.go`,
`project.go`), never `policyartifact` or `policyconflict`.

### `verdi context conflict` — yes: the window is read, and it changes the outcome (fix round 1)

The independent review (finding F2) traced a real, live consumer this
spike had not exercised: `internal/policyconflict/service.go:120-131`
calls `ResolveExemptionAuthority` (→ `authority.go:285`
`resolveBound(e.Expiry, e.ReviewCondition, in.EvaluatedOn)`) then
`ApplyEffectiveExemptions`; `cmd/verdi/context_conflict.go`'s own doc
comment says `context conflict`, `build start`, `gate`, and `close` all
share that service; `cmd/verdi/readiness_snapshot.go:218` feeds the same
`Report` into `readinesspilot`, which `serve`, workbench, `specdoc`, and
MCP all consume downstream. This fix round re-ran oq-4 through that exact
path, end to end, against a real store, and kept both transcripts under
the fence (`evidence/oq4-exemption/conflict-{future,lapsed}-window-
report.json`, `conflict-path-SUMMARY.txt`).

Building a real request required three more real gaps this exercise
surfaced along the way (all fixed in the same throwaway clone, never in
this worktree): the starter constitution registers no harness adapter
(`context conflict` refuses "adapter mismatch: requested codex/1;
registered []" until one is added and `verdi context project` generates
its managed instruction projection); the untouched starter-solo profile
maps no role to the `policy-exemption-approval` transition at all (every
exemption approval resolves "violated-with-witness" regardless of the
window until the profile and the constitution's transition catalog both
name it); and the repository's own committed `align.judge_cmd` is
`[claude, -p, --output-format, json]` — the REAL Claude Code CLI — which
this exercise's first attempt actually invoked (it failed on an unrelated
real-CLI response-schema mismatch, `unknown field "duration_api_ms"`,
before this was caught) in violation of this spike's own "no network"
constraint; every run after that one points `judge_cmd` at a hermetic
local `/bin/sh` script returning a canned no-conflict result instead
(`conflict-path-SUMMARY.txt` discloses the one real-CLI call rather than
hiding it).

With those gaps closed, two identical requests (`_scratch/oq4context`
built each through the real `contextcompile`/`policyconflict` encode
seams, never hand-typed JSON), differing in nothing but the exemption's
own `expiry`:

| | `resolution.bound` | `resolution.{match,freshness,scope,authorization}` | `removed_claims` | mechanical domain |
|---|---|---|---|---|
| future window (2027-06-30) | **proven** | all proven | `[golangci-lint-standard-set]` — excused | `open_domain: true`, no active values |
| lapsed window (2026-01-01) | **violated-with-witness** | all proven, unchanged | `[]` — not excused | `open_domain: false`, `["standard"]` still active |

Every other field in both full reports is either byte-identical
(`verdict` — both "blocked-unproven," `semantic`, `disclosures`) or a
content-address digest that trivially differs because it hashes the
changed `expiry` byte (the report's own digest, `effective_policy_digest`,
the named commit, `manifest_digest`, the judge's `input_digest`). The
window is the one substantive variable, and it alone flips whether the
claim is excused — exactly matching `authority_test.go:922`'s own unit
proof, now reproduced end to end through the real CLI over a real store.
(Both runs' overall verdict stays "blocked-unproven" for an unrelated
reason — `spec/self-governance-spike`'s own prose registers as semantic
candidates needing a disposition, which a "no-conflict" judge finding
alone does not supply; a target spec with no semantic candidates would
very likely reach "pass" for the future window, not attempted here.)

**Answer:** no on `verdi lint` and `verdi journey` (measured, unchanged
from before this fix round); **yes** on the policy-conflict path
(`context conflict`/`gate`/`close`/`build start`/the readiness snapshot),
now measured rather than assumed. This makes oq-4 a **wiring gap between
two existing surfaces** — the mechanism spec/self-governance ac-4 asks
for already exists and already works, `verdi lint`/`verdi journey` are
simply not two of the places it is wired to — rather than either a
confirmation of ac-4 as originally read or a defect against
spec/readiness-recovery ac-1 as this README first proposed. Whether
lint/journey *should* consume the policy-conflict path (making the gap
ac-4's to close) or whether ac-4 is satisfied by the path that already
exists (making this a documentation-and-cross-reference fix, not a
defect) is a decision for spec/self-governance's own author, not this
spike.

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
- (Fix round 1, F5) The parent spec's own ac-1 text asserts "the 28
  active specs"; `ls -d .verdi/specs/active/*/ | wc -l` measures **39**
  today. This spike does not reconcile the two further (a first check —
  comparing each spec directory's first-commit timestamp against
  spec/self-governance's own — did not cleanly separate "older than
  self-governance" from "newer," so a precise account of which 11 specs
  the story's count predates was not reached inside this round's timebox)
  — the discrepancy is recorded, per Global Constraint 10, rather than
  either silently keeping "28" or silently asserting a reconciliation
  this spike did not actually verify.
- (Fix round 1) An independent Opus review of the base..head this spike
  had already committed returned REVISE with seven findings (F1-F7,
  `lane-self-governance-review.md`); the controller accepted all seven
  and this round addresses each in place, with a fix-round-1 marker at
  its exact location rather than a silent rewrite — see the
  "Fix round 1" section of the lane report for the finding-to-commit map.

## Evidence index

- `evidence/oq1-adoption/` — adoption commit stats, full battery
  transcripts, blocker table, timing/load disclosure, re-runnable script
  reference.
- `evidence/oq2-decode-output.txt` — all five claim decodes plus the
  payload-kind probe, verbatim.
- `oq3-rung-field.patch` (top of this directory) — the draft field,
  captured and reverted; `evidence/oq3-rung-field-blast-radius.txt` — the
  blast radius, corrected in fix round 1 (F1/F4/F7: commands beside every
  number, the failure-shape breakdown); `evidence/oq3-testall-summary.txt`
  — the fix round's own fresh-clone `go test ./...` per-package roll-up
  (101 lines) every headline number is reproducible from.
- `evidence/oq4-exemption/` — the claim, the registered subject, both
  exemption variants, the lint/journey transcripts, the subject-catalog
  refusal, and (fix round 1, F2) the policy-conflict path's own request
  (`conflict-request.json`), both full reports (`conflict-{future,
  lapsed}-window-report.json`), the corrected profile
  (`profile-with-exemption-approval.md`), and `conflict-path-SUMMARY.txt`.
- `evidence/oq5-candidate-{a,b}/`, `evidence/oq5-candidate-c-ledger-
  test.txt`, `disclosures.md` — all three candidates tried, the item
  enumeration, and the recommendation's supporting transcripts.
- `_scratch/` — `oq2decode/`, `oq4digest/`, `oq4context/` (fix round 1:
  builds the real `context conflict --request` document through the
  exported `contextcompile`/`policyconflict` encode seams),
  `oq5ledger/` (Go, gofmt-clean, excluded from `./...` by its underscore
  prefix), `sg-battery.sh` (the oq-1 battery, re-runnable).
