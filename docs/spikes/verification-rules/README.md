# Verification-rules spike — answer

Resolves `spec/verification-rules#oq-1..oq-5` (via
`spec/verification-rules-spike`): the clause-enumeration measure, the
citation-seam trial, the design-spec-02 frontmatter delta, the
cross-specification comparison ruling, and the required-field/decoder
count. Base: `verdi` @ `5f60c76c` (main). All evidence lives beside this
file under `docs/spikes/verification-rules/`; nothing under
`docs/design/specs/` or `.verdi/specs/` was written.

## oq-1 — clause-enumeration measure and threshold

**Status: ANSWERED-WITH-CAVEAT.**

**Recommendation: threshold the under-enumeration lint at claims > 16**
(the calibrated mechanical claim count, defined below), giving a **backlog
of 12** criteria/constraints out of **277** scored across **39** active
spec directories (not 28 — see Deviations). The naive "more than three
claims" cut the story also asks about is far too broad to use directly:
it flags **231/277 (83%)** of the corpus, because ordinary AC/constraint
prose already carries 2-3 commas as a matter of style.

**Method** (`_scratch/measure/main.go`, `go run
./docs/spikes/verification-rules/_scratch/measure .` — exit 0, full
output `measure.out`, 114 lines, unreduced): decode every
`.verdi/specs/active/*/spec.md` through the real `artifact.SplitFrontmatter`
+ `artifact.DecodeSpec` (never a bespoke parse), score every
`acceptance_criteria[].text`/`constraints[].text` by three mechanical
proxies — `1 + count(',')+count(';')` (list items), whole-word
`never`/`no` occurrences floored at 1 (prohibition count), `1 +
count("and")` (and-joined phrase count) — and take **claims =
max(proxy1, proxy2, proxy3)** per object (methodology choice, not a spec
ambiguity: recorded here, not in an invention ledger, since it governs
only this throwaway measurement, not product behavior).

Histogram (full table in `measure.out`): claims ranges from 2 to 24; no
object scores below 2 or above 24; the corpus has real mass all the way
up (e.g. 22 objects score exactly 9, 23 score exactly 8).

**Calibration** (`labels.md`): hand-labeled the top 20 by claims (spanning
14–24) TRUE (genuine independent multi-claim, deserving `clauses:`) or
FALSE (mechanical over-count — a closed-taxonomy completeness claim, or a
one-consequence disjunction, read as N claims by the proxies) using an
explicit written rule (top-level semicolon/`and` segments only; a
segment whose payload is one enumerated set is one claim, not N).
Result: **13 TRUE / 7 FALSE (35% unconditioned false-positive rate)**.
False-positive rate by candidate threshold, from those 20 labels:

| T (flag claims>T) | labeled sample | false | FP rate | corpus backlog at T |
|---|---|---|---|---|
| 13 | 20 | 7 | 35.0% | 23 |
| 14 | 17 | 7 | 41.2% | 17 |
| 15 | 13 | 3 | 23.1% | 13 |
| **16** | **12** | **2** | **16.7%** | **12** |
| 17 | 7 | 2 | 28.6% | 7 |
| 18 | 4 | 0 | 0.0% | 4 |

T=16 is the lowest (most inclusive) threshold at which the sample's
false-positive rate first drops under one in five, matching the story's
instruction ("choose the threshold where hand-labelled false positives
fall under one in five... report the count above threshold as the
backlog"). **Caveat, disclosed rather than smoothed over:** the curve is
not monotonic (T=17 regresses to 28.6%, a small-sample artifact of a
5-item all-TRUE band at claims=17 dropping out while a 3-item
2-FALSE-of-3 band at claims=18 is still counted at T=16 and T=17 both);
this calibration rests on 20 single-rater hand labels with no independent
cross-check, and a differently-labeled top 20 could move T by one or two.
`spec/verification-rules` ac-2 should treat T=16/backlog=12 as a starting
point the feature's own larger-sample lint work re-validates, not a
final number.

## oq-2 — citation seam: bindings fragments vs. test-side markers

**Status: ANSWERED.**

**Recommendation: Seam B (test-side `// verdi:clause` markers + a
collecting gate test).** Seam A requires two structural, wide-blast-radius
changes to already-shared code before it can express even one clause
citation; Seam B works today, fully additively, with a total blast radius
of one new test file and one-line comments.

**Trial setup:** readiness-recovery co-2's text ("no readiness file,
cache, status field, transition, receipt, event log, or artifact kind is
added; no recovery lifecycle state is invented; a projection is never
authority", `.verdi/specs/active/readiness-recovery/spec.md`) enumerates
**exactly nine** prohibitions matching the story's count: (c1) no
readiness file, (c2) no cache, (c3) no status field, (c4) no transition,
(c5) no receipt, (c6) no event log, (c7) no artifact kind, (c8) no
invented recovery lifecycle state, (c9) a projection is never authority.

**Witnessed today: 0 of 9.** The readiness-recovery feature has no
implementation anywhere in this checkout — `grep -rli "readiness.recovery\|recover
--apply\|readinessrecovery" --include="*.go" .` = 0 hits, `internal/
readinessload` (the package the brief names) does not exist (see
Deviations); `internal/readinesspilot` is a different, pre-existing
"Wave 3.5" projection package unrelated to readiness-recovery. There is
nothing to cite yet, by either seam — the trial below is necessarily a
mechanism proof, not a real-witness inventory.

### Seam A: bindings fragments

Real code, real calls (`_scratch/seamA/main.go`, `go run
./docs/spikes/verification-rules/_scratch/seamA .` — exit 0, full output
`seam-a.out`, 66 lines):

1. **Two-level fragment `spec/readiness-recovery#co-2/c1` — REJECT, not
   misparse.** `artifact.ParseRef` fails with `invalid fragment object id
   "co-2/c1"` (the shared `objectIDRe` in `internal/artifact/ref.go`
   forbids `/`); `artifact.ResolveBindingAC` and a full
   `artifact.DecodeBindings` of a scratch `verdi.bindings.yaml` carrying
   all nine `co-2/c1..c9` entries both fail the same way, at decode time,
   with a clear, actionable, fail-closed error — this is also exactly
   what `internal/lint` VL-003 would report (`internal/lint/snapshot.go:135`
   assigns `artifact.DecodeBindings`'s own error verbatim to
   `Snapshot.RootBindingsErr`, which `vl003.go`'s `checkBindings` surfaces
   as `"verdi.bindings.yaml (root): does not decode: ..."` — traced by
   source, not separately re-run, since the decode call is identical).
2. **One-level `spec/readiness-recovery#co-2` (bare constraint, no clause
   suffix) parses and resolves shape-wise, but is structurally blocked
   one layer later, with a MISLEADING message.** `ParseRef`/
   `ResolveBindingAC` both accept it. But VL-003's `targetACSet`
   (`internal/lint/vl003.go:317-327`) builds its declared-id set from
   `Spec.AcceptanceCriteria` **only** — never `Constraints` — so even a
   plain, undecomposed constraint citation is rejected today, with a
   message that reads as if `co-2` were undeclared
   (`"... names ac \"co-2\", which \"spec/readiness-recovery\" does not
   declare"`) when it is declared, just not as an AC. Confirmed against
   the real committed `readiness-recovery/spec.md`: its declared AC set
   is `{ac-1..ac-10}`; `co-2` sits only in `artifact.DeclaredObjectIDs`
   (which does include constraints) — a set VL-003's bindings path never
   consults.
3. **A hyphen-joined flat id (`co-2-c1`, no `/`) parses today with zero
   grammar changes** (`objectIDRe` allows repeated `-[a-z0-9]+` groups) —
   but is not a `DeclaredObjectIDs` member either, since clauses are not
   top-level objects in the current 02 schema. Renaming the fragment
   syntax alone would not be enough; the resolution-target set still has
   to grow.

**Seam A's blast radius, concretely:** to make ONE clause citation work,
Seam A needs (a) `internal/artifact/ref.go`'s `objectIDRe` — the ONE
regex every fragment ref in the whole system validates against (spec
`links[].ref`, board pins, layout `positions` keys, the `[vd:<object-id>]`
forge comment-token grammar) — loosened to accept a clause suffix, a
system-wide grammar change for a bindings-only need; and (b)
`internal/lint/vl003.go`'s `targetACSet` (or an equivalent) extended from
AC-only to the general `DeclaredObjectIDs`, plus a genuinely new
resolution layer for clause ids nested one level inside their parent
object (nothing today resolves ids inside `clauses:` at all). Two
structural changes to shared code, neither scoped to bindings alone.

### Seam B: test-side markers + gate test

Real trial in the worktree (constraint 2: reverted before commit, kept as
`seam-b.patch`, 69 lines, plus `seam-b-baseline.out`/
`seam-b-with-marker.out`, 17 lines each). Added a 34-line gate test,
`internal/readinesspilot/clausewitness_test.go` (`TestClauseWitnessCoverage`,
comfortably near the story's "~30 lines"), that walks `internal/**/*_test.go`
for `// verdi:clause <ref>` comments and reports each of co-2's nine
refs found or missing:

- **Baseline (no markers anywhere), `go test ./internal/readinesspilot/...
  -run TestClauseWitnessCoverage -v`: exit 1.** Output: nine
  `MISSING WITNESS: spec/readiness-recovery#co-2/cN` lines, one per
  clause, test fails closed — this is "how a missing witness reads in
  each seam's output" for Seam B: a named, itemized `go test` failure,
  not a silent gap.
- Added ONE synthetic marker (`// verdi:clause
  spec/readiness-recovery#co-2/c9`, explicitly commented as a
  mechanism-proof placement, not a real witness) above
  `TestDeriveHappyPath` in `internal/readinesspilot/derive_test.go`.
  Rerun: exit 1 still (8 remain missing, correctly), but c9 now reports
  `WITNESSED`. Mechanism proven both directions.
- **Blast radius:** one new test file (~34 lines) + one seven-line
  comment block above an existing test signature (zero behavior change —
  comments only). No product non-test file touched; no shared package's
  parsing grammar touched.

### Ruling

Seam B wins on blast radius (additive-only vs. two shared-grammar
changes) and works unconditionally today; its cost is a second citation
idiom alongside `verdi.bindings.yaml` (a marker convention with no
`verdi lint` integration yet — the story's own plan says the real gate
test is "built under test in the feature's plan", so this spike's
34-line version is a prototype, not the delivered rule). Seam A remains
available later as a deliberate, ratified grammar change if clause-level
bindings become common enough to justify touching the shared ref grammar
— not rejected outright, just not the smallest reversible option today.

## oq-3 — design-spec-02 frontmatter delta and consumer census

**Status: ANSWERED.**

**The delta** (`_fence-copies/02-artifact-contract.{original,draft}.md`,
diff kept at `02-clauses.diff`, 46 diff lines: **26 added, 3 changed/
removed**, 2 file-header lines, 2 hunk headers, 13 unchanged context
lines shown for anchoring): adds an optional `clauses: [{id, text}, ...]`
list to `acceptance_criteria`/`constraints` entries only (§Object model's
existing YAML example block), plus one new paragraph stating: clause ids
are short tokens (`c1`, `c2`, ...) scoped to their parent, not globally
unique; a clause carries no `anchor`/`evidence`/`links` of its own (it
decomposes its parent's `text`, it is not a new object kind); `clauses:`
is `omitempty` and never required for a ≤3-claim object (A8: existing
statusless objects keep today's meaning); and — flagged explicitly in the
draft prose as a **prerequisite**, not a free addition — citing one
clause needs a fragment ref one level deeper than §Identity and
references' current grammar, which oq-2 found `objectIDRe` rejects today.

### Per-consumer blast radius

| consumer | must-change | may-change | untouched | deciding file |
|---|---|---|---|---|
| `internal/artifact` | **yes** — `Constraint`/`AcceptanceCriterion` structs need a `Clauses []Clause` field to decode the new key at all; a `Clause` type + `Validate()` (id shape, text non-empty) | `ref.go`'s `objectIDRe`/`ResolveBindingAC`, only if Seam A is later chosen (oq-2) | — | `internal/artifact/object.go` (`Constraint`), `internal/artifact/spec.go` (`AcceptanceCriterion`) |
| `internal/lint` | **yes**, for a new under-enumeration rule (oq-1's calibrated threshold — the spec seed says oq-1 "fixes ac-2's threshold as a decision", implying a real VL-0xx) | VL-003's bindings resolution, only if Seam A is chosen | Seam B needs no `internal/lint` change at all | `internal/lint/engine.go`'s `allRules` (new rule), `internal/lint/vl003.go` (conditional) |
| `internal/journey` | no | per-clause obligation-quality assessment (GLG ac-2's "obligation... claim... unelaborated" is thematically adjacent) is a plausible future extension | **yes, today** — both AC-consuming sites (`port.go:265`, `facts.go:393`) read only `.ID`/`.Evidence`, never `.Text`, so a new sibling field changes nothing they do | `internal/journey/port.go:265`, `internal/journey/facts.go:393` |
| `internal/readinesspilot` | no | no | **yes** — zero references to `artifact.AcceptanceCriterion`/`Constraint` anywhere in the package (`grep` confirmed); it consumes a pre-derived `Input.Shape`, not `SpecFrontmatter`, directly | n/a |
| `internal/specdoc*` | **yes** — this is spec-documents' "one canonical document model... from a spec's decoded objects" (spec-documents ac-1); `Item`/`Criterion` (`model.go:22-38`) carry only `{ID, Text, Detail}`/`{...Evidence,Coverage}`, `build.go:72-73` copies only `ID`/`Text`/`Detail` from the decoded spec, and `markdown.go:85-86` renders one bullet per object with no sub-list — all three need a `Clauses` field/copy/render path or the shared document core silently flattens every clause back into its parent's bundled text | `internal/specdocload` — untouched (no AC/constraint field references at all) | `facts.go`/`kind.go` — untouched (coverage-by-id and section-enum only, never touch `.Text`) | `internal/specdoc/model.go`, `build.go`, `markdown.go` |
| `internal/dex` | no | rendering `ac.Clauses` in the live-mapping table (`featurelens.go:68-69`) would be a readability improvement | **yes, today** — reads only `ac.ID`/`ac.Text`; dex renders no constraints at all anywhere in the package (`grep` for `.Constraints`/`range.*Constraint` = 0 hits), so there is no constraint-rendering surface to even consider | `internal/dex/featurelens.go:68-69` |
| `internal/mcpserve` | no | clause-level review-comment targeting (`[vd:co-2/c1]`) if the story wants it — `review.go:84` calls the shared, exported `artifact.DeclaredObjectIDs`, which does not walk into `clauses:` | **yes, today** | `internal/mcpserve/review.go:84`, deciding function `internal/artifact/spec.go`'s `DeclaredObjectIDs` |

## oq-4 — cross-specification comparison: mechanical or judge task?

**Status: ANSWERED-WITH-CAVEAT.**

**Ruling: alignment and identity-verification are mechanical and already
built (VL-015); classifying a genuine text change as `amended` (real
claim change) vs. `amended_advisory` (non-substantive) is not mechanically
checked at all today and remains a judge task — but the corpus has no
real example of the hard case to test that judgment against.**

**Are the older versions "actually marked superseded"? No — by design.**
None of the six version files (GLG v1/v2/v3, comparative-spike-experiments
v1/v2/v3) carries a self-referential `status`/`state` field; supersession
is recorded exactly once, forward, on the successor's own
`links: {type: supersedes}`. `internal/lint/vl010.go`'s doc comment
(lines 26-33) names why: a round-5 mechanism used to flip a predecessor's
own `status:` byte to `superseded` in place, and
`docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md`
("Task 7") deleted that writer entirely — supersession is now derived
purely from Git reachability (`internal/specstate`), never written back
to a predecessor's frozen bytes. `disclosure-seam` (`status: superseded`
in its own frontmatter) predates that change and is grandfathered (A8:
frozen artifacts are never rewritten) — its byte is not evidence the
mechanism still runs. Checking a predecessor's own frontmatter for
`superseded` is the wrong test today and will read "not marked" for
every future supersession too. Full evidence:
`versioned-specs-alignment.md` §1.

**Mechanical alignment result** (`_scratch/versioncompare/main.go`,
exit 0, `versioncompare.out`): all **58** aligned acceptance-criterion/
constraint pairs across the four available transitions (GLG v1→v2,
v2→v3; comparative-spike-experiments v1→v2, v2→v3) are **byte-identical
text, zero additions, zero removals** — matching each successor's own
`supersession:` manifest (`carried:` names every existing ac-/co- id,
`amended: []`, `amended_advisory: []`, every time). The only structural
change any transition makes is pure **addition** of new decisions
(GLG v2 adds dc-16..26; v3 adds dc-27; CSE v2 adds dc-20..28 and removes
oq-1; CSE v3 adds nothing) — never a rewording or removal of an existing
criterion or constraint.

**Already mechanical, already built:** `internal/lint/vl003.go`'s sibling
`vl015.go` (02 §Lint rules table, VL-015 row) already enforces exactly
the "same-anchor-different-text" check the investigation plan asks
whether to build: every predecessor object must be classified exactly
once across `carried`/`amended`/`amended_advisory`/`removed`
(`vl015.go:118-145`), and every object declared `carried` must be
byte-identical to its predecessor or the rule fails closed
(`vl015.go:148-160`, "content drifted from its predecessor — byte-identical
required"). The alignment key is the frontmatter id namespace itself,
not fuzzy text matching — no new lint is needed for that half.

**Not mechanically checked, genuinely a judge task:** VL-015 never
inspects an `Amended`/`AmendedAdvisory` entry's actual text at all beyond
counting it toward bucket-partition completeness — nothing verifies that
a change belongs in the substantive bucket rather than the advisory one,
or vice versa. The only two real `amended` examples in the whole corpus
(`versioned-specs-alignment.md` §3) are decision-level, not ac/co-level
(`clauses:` is out of scope for decisions): `dc-10`
(comparative-spike-experiments→v2, adds three new invariant categories to
a protected-scope list) and `dc-21` (v2→v3, inserts a whole new rule
mid-sentence about fixed-vs-diagnostic process measurements). **Both are
unambiguous different-claim scope expansions**, hand-classified with no
real difficulty; `amended_advisory` is declared `[]` in every transition
in the corpus — it has never actually been used.

**Caveat:** this spike cannot show whether the *hard* case — a change
that reads like a rewording but is argued to be substantive, or vice
versa — is easy or hard to classify by machine or by hand, because no
such example exists anywhere in the four available version-pairs. The
judge-task ruling above is about what's mechanically checked TODAY, not
an empirical difficulty measurement; `spec/verification-rules` should
treat PA-021 (whatever concrete case motivated it) as the first real test
of that judgment, not assume this spike settled it.

## oq-5 — contract-required fields vs. decoder enforcement

**Status: ANSWERED.**

Full table, citations, and the per-row reasoning: `required-fields.md`
(command outputs: `required-fields-evidence.out`, 45 lines). Summary: of
**8** fields 02's Common frontmatter table marks `# required`
(unconditional or conditional) — `id`, `title`, `status`, `owners`,
`problem`, `outcome`, `frozen`, `provenance` — the decoder:

- **rejects absence unconditionally for 3** (`id`, `title`, `owners` —
  `internal/artifact/common.go`'s `validateBase`);
- **enforces the conditional correctly and bidirectionally for 1**
  (`frozen` — `requireFrozen`, used by every kind, rejects both wrongful
  absence and wrongful presence);
- **never enforces presence at all for 1**, despite the table's
  unconditional-sounding "required iff generated" — `provenance`'s
  top-level presence (`common.go:342-350` validates it only if non-nil;
  no `requireProvenance`-shaped helper exists anywhere: `grep -rn
  requireProvenance internal/artifact/*.go` = 0 hits; the one
  Frozen/Provenance pairing check that does exist, `board.go:80`, is a
  board-record-specific XOR, not a generated-artifact rule);
- **splits by kind/class for 3** (`status`, `problem`, `outcome`):
  every non-spec kind and spec/component still reject absence exactly as
  02 says; spec/story rejects absence (`spec.go:419-434`, explicit
  unconditional checks, "no grandfathering tension"); **spec/feature —
  the corpus's most common class — accepts absence for all three**
  (`spec.go:327-408`, `validateObjectBlocks` tolerates nil, no
  feature-specific override anywhere).

**UAT-008 is the `status` row**, confirmed and narrowed by this spike:
its witness (`docs/design/uat/uat-findings.md:133-138`, open/low/docs)
already names `internal/artifact/spec.go`'s feature/story comments; this
spike's own read shows the acceptance is scoped to exactly spec/feature
and spec/story, not a general decoder laxity — every other kind and
spec/component still enforces the table as written. The `problem`/
`outcome` split is the same shape, on spec/feature specifically, and is
not yet in `uat-findings.md` — named here for the ratification request to
fold in or file separately; opening a new UAT-0xx entry is outside this
spike's write set.

## Deviations from the investigation plan (constraint 10)

1. **"28 active specs" vs. 39 directories found.** `ls .verdi/specs/active
   | wc -l` = 39 (`active-specs-count.out`). Reconciled exactly: 39 total
   − 5 additional version-revision directories (`comparative-spike-
   experiments-v2/-v3`, `context-integrity-v2`, `guided-lifecycle-
   governance-v2/-v3` — frozen predecessors are never removed, R4-I-4, so
   a 3-version family occupies 3 directories) − 6 additional spike-story
   sibling directories (`disclosure-enumeration-spike`,
   `mutation-ratchet-spike`, `ritual-write-scope-spike`,
   `self-governance-spike`, `strict-lint-target-spike`,
   `verification-rules-spike` — a spike is its own story-class spec
   object, always a separate directory from its parent feature) = **28**.
   The story's "28" reads as a count of distinct spec *families*, not raw
   directories; oq-1's measurement scores all 39 directories (every real
   decodable object in the corpus), which is the more complete reading of
   "across the active specs" and is the superset of any 28-family count.
2. **`internal/readinessload` does not exist** (oq-2's lane specifics
   name it as the package holding co-2's witnessing tests). The nearest
   readiness-named package, `internal/readinesspilot`, is a different,
   pre-existing "Wave 3.5" projection feature, confirmed unrelated by its
   own package doc comment. The readiness-recovery feature named in
   `spec/readiness-recovery` (whose spec.md itself DOES exist and DID
   supply co-2's real text) has no implementation anywhere in this
   checkout — `MEMORY.md`-adjacent context places its build on a separate,
   unmerged branch. Substitution made: Seam B's trial used
   `internal/readinesspilot` as the nearest real Go package to prove the
   marker-and-gate-test mechanism, with every synthetic placement labeled
   as such in both the code comment and this README; see oq-2.
3. **Seam A's trial target widened from "VL-003" to "VL-003 as invoked
   through the real decode-then-lint pipeline."** Rather than standing up
   a full scratch store directory and running the built binary's `lint`
   verb (a materially larger and riskier setup for the same answer), the
   trial calls `artifact.DecodeBindings`/`ResolveBindingAC` directly and
   traces VL-003's exact call path by source citation — `Snapshot.
   RootBindingsErr` is assigned the identical `DecodeBindings` error
   VL-003 surfaces verbatim, confirmed by reading `internal/lint/
   snapshot.go:135` — rather than re-deriving the same answer through a
   slower, harder-to-audit end-to-end run. Recorded because it is a
   scope choice a reader might expect done differently, not because it
   weakens the answer.

## Evidence index

| file | what it is |
|---|---|
| `_scratch/measure/main.go`, `measure.out` | oq-1 mechanical measurement + histogram + threshold sweep |
| `labels.md` | oq-1 top-20 hand labels + FP-rate-vs-threshold table |
| `_scratch/seamA/main.go`, `seam-a.out` | oq-2 Seam A trial (real `artifact` calls) |
| `seam-b.patch`, `seam-b-baseline.out`, `seam-b-with-marker.out` | oq-2 Seam B trial (reverted before commit) |
| `_fence-copies/02-artifact-contract.{original,draft}.md`, `02-clauses.diff` | oq-3 frontmatter delta |
| `_scratch/versioncompare/main.go`, `versioncompare.out`, `versioned-specs-alignment.md` | oq-4 alignment + classification |
| `required-fields.md`, `required-fields-evidence.out` | oq-5 required-field table |
| `active-specs-count.out` | 28-vs-39 reconciliation |

## Spec seed follow-through

- oq-1 → `spec/verification-rules` ac-2's threshold: recommend **16**
  (backlog 12), with the calibration caveat above carried into that
  decision, not hidden.
- oq-2 → ac-1's citation seam: recommend **Seam B**.
- oq-3 → the co-1 ratification request: the 46-line `02-clauses.diff` is
  the draft amendment; the consumer table above sizes the accompanying
  code plan (`internal/artifact`, `internal/lint`, `internal/specdoc*`
  must-change; `internal/journey`, `internal/dex`, `internal/mcpserve`
  optional follow-on; `internal/readinesspilot`, `internal/specdocload`
  untouched).
- oq-4 → PA-021: closed as a **judge task for the substantive-vs-advisory
  distinction specifically**, not for alignment (which is mechanical and
  already shipped as VL-015); the caveat above should travel with that
  closure.
- oq-5 → ac-4's enumerating test: the eight-row table above is the
  starting fixture; UAT-008 is row `status`, and the `problem`/`outcome`
  spec/feature gap is a new row for the same ratification request to
  adjudicate alongside it.
