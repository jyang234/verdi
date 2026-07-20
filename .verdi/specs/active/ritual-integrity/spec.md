---
id: spec/ritual-integrity
kind: spec
title: "Ritual Integrity"
owners: [platform-team]
class: feature
status: accepted-pending-build
problem: { text: "Phase 1's dogfood verified five ritual defects that tax the process the rest of extensibility Phase 2 depends on repeating three more times. X-8 (systemic, 5 occurrences): judge-backed verbs run a real judge exchange that takes 6-7 minutes, past an agent's foreground patience and past what a background monitor can survive (a monitor dies with the turn; nothing can resume a stopped agent), so agents and humans alike park on open-ended gates. X-14/X-18: report regeneration discards every prior disposition wholesale (identity.go's fail-closed Kind+ID+Text hash never matches a judge's re-worded prose), and because a discarded disposition is simply re-recorded and re-accepted on the next pass, the same standing adjudication re-counts against the spec-stale deviations budget every regeneration — a feature can be blocked by its own history repeating. X-15 (+X-11b): the closure ancestry check hard-fails operationally on a record referencing a commit unreachable from HEAD, which happens routinely once a branch that produced CI evidence is deleted post-merge — an unrelated story's branch deletion bricked model-schema's closure twice in one round, and VL-009's is-a-real-commit check has the same hole from the opposite direction (satisfiable by a locally-dangling, unreachable object). The small traps: X-1's anchor case-asymmetry (ResolveAnchor slugifies the heading side but not the frontmatter anchor: value), align's doubled judged-judged- id construction on repeated runs, and VL-003's blind spot on fragment-qualified verdi.bindings.yaml entries (a typo'd AC id currently passes lint silently). And no mechanical check ties the owner-accepted Integration & Startup Guide's capability claims to anything real, so a claim can go stale — a witness renamed or a capability regressed — with nothing in make verify to catch it.", anchor: problem }
outcome: { text: "Judge-backed verbs print their report path as the first line of output and support a bounded --wait, so a caller never parks on an open-ended gate again — expiry is a named, resumable exit condition, never a hang. Judged-finding dispositions are reaffirmed, not discarded, across regeneration: a regenerated finding whose slug matches a prior disposition is pre-filled with a candidate showing the old ruling beside the new text, and nothing is dispositioned until a human confirms it as a working-tree edit at the covering head — the slug is an untrusted hint, never a trusted identity key, so the frozen identity rule is unchanged and the freeze-in-place forcing function survives; previously-dispositioned findings absent from a fresh run stay counted by the spec-stale budget until a human marks them fixed, and the feature-close budget unions its implementing stories' archived reports with its own, closing the counterweight's laundering drain at the mechanism that actually causes it. Evidence referencing a commit unreachable from HEAD degrades honestly — quarantined at sync, read as a per-record disclosed-unproven at closure — so a branch deletion can never again brick an unrelated story's closure. The three small traps close with negative pins so they cannot silently regress. And every guide capability claim lives in a strict-decoded, atomic-row manifest whose EXISTS/PARTIAL rows carry anchored, pass-coupled witnesses, checked in make verify and disclosed as inventory-scoped until the guide itself is in-repo and a completeness check becomes possible.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "judge-backed verbs (align, and close's internal freeze-align) print their report path as the first line of stdout before the judge runs, write the report atomically at completion (the existing atomicfile seam — no partial content is ever observable at the path), and support a bounded --wait[=seconds] that polls internally and exits 2 with the report path on expiry rather than leaving a caller parked on an open-ended gate (X-8, 5 occurrences; L-N1); close's freeze-align inherits the same contract from the shared align engine rather than a per-verb reimplementation", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "a regenerated judged finding whose rule/boundary-derived slug matches a prior dispositioned finding is pre-filled with a candidate — the old ruling and old text shown beside the new text — never silently carried: a candidate is not a disposition, and AllDispositioned stays false until a human confirms each one as a working-tree edit at the covering head, so the slug is an untrusted hint and the frozen identity rule (Kind+ID+Text) is unchanged byte-for-byte, with the existing holds-to-violated negative test retained and an escalation under a stable slug presenting both texts rather than inheriting the old ruling; confirmed reaffirmations carry carried-from: <covers-sha>, excluded from the report digest and omitempty so every existing frozen archive still verifies; findings dispositioned before but absent from a fresh run land in a not-resurfaced: section, persisted across regenerations until a human marks them fixed, and consumed by exactly the disposition pre-fill and the deviations counterweight — the spec-stale budget counts unique accepted-deviation identities across findings: union not-resurfaced: (closing the X-18 judge-re-roll laundering drain: non-reproduction never uncounts), and the feature-close budget unions the implementing stories' archived reports with the feature's own report (the cross-report fix X-18 actually needed; the within-report unique-identity framing was proven a no-op by the design wave); two findings sharing a slug within one report is a disclosed judge-contract-violation finding, never a silent dedupe", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "sync quarantines a record referencing a commit unreachable from HEAD, recording the reason on the record rather than dropping it silently; the closure ancestry check reads a quarantined record as a per-record disclosed-unproven for its acceptance criterion rather than hard-failing the gate operationally, so deleting a branch can never again brick an unrelated story's closure (X-15, twice in one round); VL-009's commit check tightens from is-a-real-commit, satisfiable by a locally-dangling unreachable object (X-11b's exact false-green), to reachability-from-HEAD", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "ResolveAnchor slugifies both the heading side and the frontmatter anchor: value, so anchor: AC-1 resolves against ## AC-1 regardless of case (X-1's exact witness); a freshly minted judged finding id carries exactly one judged- prefix, while an archived report fixture carrying the old doubled judged-judged- form still decodes and round-trips untouched — the fix is prospective-only, since dispositions reference the ids as originally minted; VL-003 cross-checks a fragment-qualified verdi.bindings.yaml entry (spec/<name>#<ac-id>) against the named spec's own declared acceptance criteria, so a typo'd AC id reds lint instead of passing silently", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "every Integration & Startup Guide capability claim lives in a strict-decoded, atomic-capability-row manifest (verdi/docs/guide-claims.yaml) — one capability, one status, one witness set per row, with a bundled multi-capability row shape rejected at decode; every EXISTS or PARTIAL row's witness is bound three ways — its name exists in the corpus, it carries a // guide-claim: <row-id> anchor at its declaration, and it is PASS-coupled in make verify (a skipped or never-run witness cannot satisfy the gate); every non-EXISTS row and every status downgrade carries a cite: naming a chronicle/ledger entry, its presence gated in CI and its resolution checked workspace-side with a loud skip; the gate is wired into make verify and discloses its own scope honestly as inventory-only — row-to-witness, not yet guide-to-row completeness, which needs the guide in-repo", evidence: [static, attestation], anchor: ac-5 }
stubs:
  - { slug: judge-ergonomics, acceptance_criteria: [ac-1] }
  - { slug: finding-identity, acceptance_criteria: [ac-2] }
  - { slug: evidence-resilience, acceptance_criteria: [ac-3] }
  - { slug: ritual-traps, acceptance_criteria: [ac-4] }
  - { slug: guide-claims-gate, acceptance_criteria: [ac-5] }
frozen: { at: 2026-07-20, commit: 59cade5f8f69f7c52543ee277a2205e778caa6bb }
---
# Ritual Integrity

## Problem

Phase 1's dogfood — building `spec/operating-model` and its four stories
through Verdi's own ritual — verified five ritual defects, each witnessed
with a citation in the extensibility chronicle, that tax the very process
the rest of Phase 2 depends on repeating three more times
(`creation-surfaces`, `guide-publication`). Each fix in this feature must
land before the heavier features run the ritual in anger.

**X-8, judge gates park their caller (systemic, 5 occurrences).** The
round's gates run 6-7 minutes (`align`'s real LLM judge exchange,
`verify`'s e2e suite), naturally exceeding an agent's foreground patience.
A background monitor cannot rescue this: it dies with the turn, and
nothing can resume a stopped agent from a completion event alone. Every
occurrence — a builder idling silently on a port-poll, two builders
self-catching hung tail-f monitors, a Fable builder and two fix agents all
parking on align/verify awaiting an event that never resumes them — cost
real time and required a human or controller nudge to recover. The round's
workaround (chained bounded foreground waits) is a process discipline
applied by every dispatch; it is not a property of the tool.

**X-14/X-18, report regeneration discards dispositions and the discard
double-counts.** `internal/align/identity.go`'s `Identity` folds
Kind+ID+Text into the disposition-carry hash, deliberately, so that a
verdict or witness change voids a stale disposition — a load-bearing,
fail-closed design. But judge prose varies run to run, so a JUDGED finding
never content-hashes the same across two runs, and the same fail-closed
design that protects against stale dispositions discards *every*
disposition wholesale on regeneration, not just the ones that actually
changed. X-18 showed the second-order cost directly: a discarded
disposition is simply re-recorded (and re-accepted) on the very next pass,
so the identical standing adjudication consumes budget against the
spec-stale deviations threshold every time the report regenerates — a
feature can be blocked from closing by its own settled history repeating
itself.

**X-15 (+X-11b), evidence/branch-lifecycle coupling bricks closure.** The
closure gate's ancestry check hard-fails with an *operational* exit (exit
2, "Not a valid commit name") when a synced CI evidence bundle carries a
record referencing a commit that no longer exists anywhere — which happens
routinely once the branch that produced the evidence is deleted after its
PR merges. This bit model-schema's closure twice in the same round, from
an *unrelated* story's branch deletion, and clearing/re-syncing did not
help because the same poisoned bundle was still the latest successful CI
run. VL-009 has the identical hole from the opposite direction: its
is-a-real-commit check is satisfiable by a locally-dangling object that no
branch or ref reaches — a false green (X-11b) that would let a
closure-time check believe evidence is sound when its source has already
been deleted upstream.

**The small traps.** X-1: `ResolveAnchor` lowercases a heading's text via
`SlugifyHeading` but does not transform the frontmatter `anchor:` value
before comparing, so `anchor: AC-1` silently fails to resolve against
`## AC-1` unless the author already knows, by unwritten convention, to
write anchors in lowercase. Align's finding-id construction doubles the
`judged-` prefix (`judged-judged-…`) on certain regeneration paths — a
tool defect discovered mid-round, whose fix must not disturb archived
reports whose dispositions already reference the doubled ids. VL-003
validates a bindings entry's *bare* AC id against its owning spec's
declared criteria but never resolves a *fragment-qualified*
`spec/<name>#<ac-id>` entry against the *named* spec's own ACs at all — a
typo'd AC id in such an entry passes lint with no finding.

**No mechanical guide-truth check.** The owner-accepted Integration &
Startup Guide makes capability claims (Appendix B) that nothing in
`make verify` ties to the corpus. A witness can be renamed, gutted, or
simply never run again, and the guide's claim stays exactly as confident
as the day it was written — the gap Phase 2's own guide-publication
feature cannot honestly close without this gate existing first.

## Outcome

Judge-backed verbs — `align`, and `close`'s internal freeze-align — print
their report path as the first line of stdout, before the judge runs, and
write the report atomically at completion so no partial content is ever
observable at that path. A new `--wait[=seconds]` flag does bounded
internal polling and exits 2 *with the report path* on expiry, so a caller
— human or agent — is never left parked on an open-ended gate: the wait is
bounded, and expiry is a named, resumable condition rather than a hang.
The contract lives once in the shared align engine, so every current and
future judge caller inherits it rather than reimplementing it per verb.

Judged-finding dispositions are **reaffirmed**, never silently carried and
never silently discarded, across report regeneration. Because the judge
has no verdict axis — it emits only `{id, text, confidence}` — a slug
derived from the rule or boundary a judged finding attacks is treated as
an **untrusted hint**, never a trusted identity key: when a regenerated
finding's slug matches a prior dispositioned finding, `align` presents a
**candidate** — the old ruling and old text shown beside the new text —
and nothing is dispositioned until a human confirms it as a working-tree
edit at the covering head. `AllDispositioned` stays false until every
candidate is confirmed. This preserves the frozen identity rule
(Kind+ID+Text, `identity.go`) completely unchanged — the ratification
touch is schema-additive only — and preserves the freeze-in-place forcing
function (X-16): a disposition still requires a human touch at the
freezing head. An escalation under a stable slug — a low-confidence
cosmetic ruling followed by a high-confidence real regression at the same
slug — presents both texts side by side, so nothing is silently
inherited. A confirmed reaffirmation carries `carried-from: <covers-sha>`
provenance, excluded from the report digest (so `VerifyDigest` is
unaffected on every existing frozen archive) and `omitempty` (old fixtures
keep decoding unchanged). A finding dispositioned before but absent from a
fresh run lands in a `not-resurfaced:` section, persisted across
regenerations until a human marks it fixed, with exactly two consumers:
the disposition pre-fill UI and the deviations counterweight. The
spec-stale budget counts unique accepted-deviation identities across
`findings:` union `not-resurfaced:`, so a non-reproducible judge simply
not re-emitting a finding never uncounts it — closing the X-18
judge-re-roll laundering drain outright. The feature-close budget is a
union over the implementing stories' archived reports plus the feature's
own report — the actual cross-report mechanism X-18 needed; the
design wave proved the within-report "unique identities" framing was a
no-op. Two findings sharing a slug within one report is disclosed as a
judge-contract violation, never silently deduplicated.

Evidence referencing a commit unreachable from HEAD degrades honestly
instead of bricking closure. `sync` quarantines such a record at ingest,
recording the reason; the closure ancestry check reads a quarantined
record as a **per-record disclosed-unproven** for the acceptance criterion
it would have evidenced, never as an operational failure — a branch
deletion, however unrelated, can never again brick a story's closure.
VL-009 tightens from "is a real commit" (satisfiable by a locally-dangling
object nothing reaches) to "reachable from HEAD," closing the false green
from the other direction.

The three small traps close with negative pins proving they stay closed:
anchor resolution is symmetric on both sides; a fresh judged finding
carries exactly one `judged-` prefix while an archived report's doubled
ids still round-trip untouched; and VL-003 reds a fragment-qualified
binding naming an AC the target spec does not declare.

Every guide capability claim lives in a strict-decoded, atomic-row
manifest (`verdi/docs/guide-claims.yaml`) — one capability, one status,
one witness set per row. Every EXISTS or PARTIAL row's witness is bound
three ways: the name exists in the corpus, it carries a
`// guide-claim: <row-id>` anchor at its declaration (so a rename or a
gutted test becomes a visible lie, never a silent gap), and it is
PASS-coupled in `make verify` (a skipped or never-run witness cannot
satisfy the gate). Every non-EXISTS row and every status downgrade
carries a `cite:` naming a chronicle or ledger entry, its presence gated
in CI and its resolution checked workspace-side with a loud skip. The
gate is wired into `make verify`, and discloses its own scope honestly:
inventory-only (row-to-witness) until the guide itself is in-repo, when a
guide-to-row completeness check becomes possible — a later-phase
requirement, not claimed here.

## Ac 1

Every judge-backed verb prints its report path as the first line of
stdout, before the judge subprocess ever runs, so a caller — human or
agent — always has a filesystem location to watch without parsing
anything else the verb prints. The report is written through the existing
atomicfile seam at completion, so no partial or truncated content is ever
observable at that path mid-run — a waiting reader either sees nothing
yet or sees the finished report, never a half-written one. A new
`--wait[=seconds]` flag turns that watchability into a bounded verb
behavior: the verb polls internally, on the caller's behalf, up to the
given bound (or a sane default), and on expiry exits 2 *with the report
path already printed* — never a silent hang, and never a caller left
guessing where to look. The contract is implemented once, in the shared
align engine, so `align` and `close`'s internal freeze-align both inherit
it rather than carrying two divergent implementations; any future
judge-backed verb gets the contract by construction. This is the direct,
chronicle-verbatim fix for X-8: five documented occurrences of an agent or
a human parked on an open-ended judge gate with no way to resume cheaply,
because a background monitor dies with the turn and nothing can resume a
stopped agent from a completion event alone.

## Ac 2

A judged finding's slug — derived from the rule or boundary the finding
attacks, not from the judge's prose — is an untrusted *hint* for
regeneration, never a trusted identity key. When `align` regenerates a
report and a fresh finding's slug matches a finding the prior report
already dispositioned, the fresh finding is pre-filled as a **candidate**:
the old ruling and old text rendered beside the new text, so a human sees
exactly what changed before deciding anything. A candidate is explicitly
*not* a disposition — `AllDispositioned` stays false until a human
confirms each candidate individually, as a working-tree edit at the
report's covering head, exactly the same working-tree-edit discipline
X-16 already established for fresh findings. This is why the frozen
identity rule in `internal/align/identity.go` (content hash over
Kind+ID+Text, deliberately fail-closed so a verdict or witness change
voids a stale disposition) is **unchanged, byte-for-byte** — the existing
holds-to-violated negative test keeps passing unmodified — and why an
escalation under a stable slug (a low-confidence cosmetic ruling followed
by a high-confidence real regression at the identical slug) cannot
silently inherit the old, wrong ruling: the human sees both texts and
must choose. A confirmed reaffirmation gains `carried-from: <covers-sha>`
provenance on the disposition, a schema-additive field on
`internal/artifact/deviation.go` that is excluded from the report digest
(so `VerifyDigest` on every existing frozen archive is unaffected) and
`omitempty` (so every old fixture keeps decoding as-is). A finding that
was dispositioned in a prior report but that a fresh judge run simply does
not re-emit lands in a `not-resurfaced:` section — renamed deliberately
from a "resolved:" framing, since a non-reproducible judge failing to
re-emit a finding proves nothing about whether the underlying issue is
fixed. That section persists across further regenerations until a human
explicitly marks the finding fixed, and it has exactly two consumers: the
disposition pre-fill UI (so a `not-resurfaced` finding that resurfaces
later still pre-fills correctly) and the deviations counterweight. The
spec-stale budget counts unique accepted-deviation identities across
`findings:` union `not-resurfaced:` — the mechanism that finally closes
X-18's judge-re-roll laundering drain, since a judge simply failing to
reproduce a finding on a later run can no longer make a standing,
accepted deviation quietly stop counting. The feature-close budget is a
union over every implementing story's *archived* report plus the
feature's own report — the actual cross-report fix X-18's own postmortem
named as needed; the Task 0 design wave proved that counting "unique
identities" only within a single report, without the cross-report union,
was a no-op against the exact laundering sequence X-18 witnessed. Two
findings that land on the same slug within one report is disclosed as a
judge-contract violation in its own right — never silently deduplicated
into one.

## Ac 3

`sync` quarantines, rather than silently drops or hard-fails on, any
evidence record whose referenced commit is not reachable from HEAD at
sync time — the exact shape a deleted branch produces once its PR has
merged and CI evidence for it has already been captured. The record is
kept, annotated with the quarantine reason, never discarded outright. The
closure gate's ancestry check — the one that today hard-fails with an
operational exit 2 ("Not a valid commit name") on exactly this shape,
which bricked model-schema's closure twice in one round from an
*unrelated* story's branch deletion (X-15) — instead reads a quarantined
record as a **per-record disclosed-unproven** against the acceptance
criterion it would otherwise have evidenced. The gate's own verdict stays
honest (an AC that only that record would have evidenced is not silently
marked proven), and the exit code discipline is preserved: a quarantined
record is never, by itself, an operational failure. `VL-009`'s own commit
check is tightened the same direction from the opposite side: today it
proves only "is a real commit," which a locally-dangling object no branch
or ref reaches already satisfies (X-11b's exact false green — a
`frozen.commit` that looks pinned but that no history actually retains
reachable); the check becomes "is reachable from HEAD," closing that hole
without changing behavior for any commit that legitimately is reachable.

## Ac 4

Three independently-witnessed traps close with pins proving they stay
closed. `ResolveAnchor` (`internal/artifact/object.go`) is extended to
slugify the frontmatter `anchor:` value through the same
`SlugifyHeading` transform already applied to heading text, so
`anchor: AC-1` resolves against a `## AC-1` heading regardless of case —
closing X-1's case-asymmetry, which today silently fails to resolve
unless an author already knows, from unwritten convention, to write every
anchor in lowercase. Align's finding-id construction, which on certain
regeneration paths doubles the `judged-` prefix into `judged-judged-…`,
is fixed prospectively only: a freshly minted judged finding id carries
exactly one `judged-` prefix, while a fixture standing in for an already-
archived report that carries the old doubled form must still decode and
round-trip completely untouched, since real archived dispositions
reference those ids exactly as originally minted — silently renumbering
them would break every existing disposition's own identity. `VL-003`
gains a check it does not have today: cross-checking a fragment-qualified
`verdi.bindings.yaml` entry (`spec/<name>#<ac-id>`) against the *named*
spec's own declared acceptance criteria, not only (as today) a bare
`ac-<slug>` entry against the bindings file's own primary spec — so a
typo'd AC id inside a fragment-qualified entry reds lint by name instead
of passing silently, the exact blind spot this feature's own bindings
entries must otherwise be authored around by hand.

## Ac 5

Every capability claim the owner-accepted Integration & Startup Guide
makes is transcribed into `verdi/docs/guide-claims.yaml`, a strict-decoded
manifest of atomic capability rows: one row names exactly one capability,
carries exactly one status (EXISTS/PARTIAL/INVENTED), and binds exactly
one witness set — a bundled, multi-capability row shape is rejected at
decode, so a row can never quietly grow to cover more ground than its own
status honestly describes. Every `EXISTS` or `PARTIAL` row's witness is
bound three separate ways, closing the ADJ-50 lying-gate class a
name-existence-only check would otherwise permit: the witness's name must
exist in the corpus; the witness must carry a `// guide-claim: <row-id>`
anchor at its own declaration, the same discipline `vocab:identity`
markers already use, so a rename or a gutted test becomes a visible lie
rather than a silent gap; and the witness must be PASS-coupled inside
`make verify`, the `require-pass.sh` precedent, so a witness that is
merely present but skipped, or never actually invoked, cannot satisfy the
row. Every row that is *not* `EXISTS`, and every row whose status is
*downgraded* from a prior value, carries a `cite:` field naming a
chronicle or ledger entry — its presence is gated in CI, and its
resolution (does the cited entry actually exist) is checked
workspace-side only, with a loud skip when the chronicle is unavailable,
since the chronicle itself lives outside this repository. The whole check
is wired into `make verify`, which grows to include it, and its own
documentation discloses its scope honestly: this feature proves
row-to-witness binding only (an inventory of what is claimed is backed by
something real) — it does not yet prove guide-to-row completeness (that
every claim the guide's prose makes has a corresponding row at all), which
requires the guide to be in-repo first and is a later-phase requirement
this feature does not claim to satisfy.
