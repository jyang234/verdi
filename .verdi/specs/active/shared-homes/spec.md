---
id: spec/shared-homes
kind: spec
title: "Shared Homes"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-QH-3
problem: { text: "shared behaviors live as hand copies against the never-copy-paste rule, and one pair has already silently diverged. The atomic-write idiom exists four times — boardio/boardstate.go and boardio/graduate.go re-inline it in the SAME package as writeFileAtomic (whose own comment names graduate.go as a user), boardlayout/file.go is the fourth — with divergent behavior (only boardstate does MkdirAll) and a uniform missing fsync, so none is crash-durable. The canonjson→sha256→\"sha256:\"+hex digest tail is hand-copied ten times across seven packages, every copy cross-referencing another: a format-string drift surface. The YAML double-quote helper exists three times (align, decisionsweep, workbench), each comment pointing at the others instead of sharing. The twin classifyArtifactPath tables (lint/walk.go, index/walk.go) HAVE diverged: index omits reaffirmation in both classify and decodeEntry while lint's comment still claims a mirror — the exact bug class lint's knownTopLevelEntries comment memorializes. commitdesign's titleCase re-implements designscaffold.HumanizeName with byte-indexing that is not rune-safe. And five smaller pairs repeat verbatim: evidence's candidate-filter/dangling-check, align's self-declared byte-identical Preserve pair, provider's criteria-changed pair (fake vs jira), mcpserve's four lone-ref decode prologues, cmd/verdi's five fold-load prologues.", anchor: "#problem" }
outcome: { text: "one home per shared behavior, proven equivalent. internal/atomicfile owns the atomic write (MkdirAll + temp + write + close + fsync + rename), consumed by every former copy — the durability gap closed once. canonjson owns Digest, and the ten digest copies collapse with digest strings proven byte-identical over fixtures. internal/artifact owns the YAML double-quote helper (the emission side of the seam, code-health dc-4) and the path-classification table — both walks consume the table, and index gains the reaffirmation case its copy silently lost, healing the divergence with a witness test. titleCase is deleted for HumanizeName. The five small pairs each collapse to one helper — evidence, align (one generic), provider (one generic), mcpserve, cmd/verdi — with every existing test still green and scaffold outputs proven byte-identical.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "internal/atomicfile (new leaf package) owns Write: MkdirAll + CreateTemp + write + close + fsync + rename, with happy and negative (unwritable destination) table tests; the four hand copies (boardio boardstate/graduate + writeFileAtomic itself, boardlayout/file.go) are replaced by calls to it, all existing boardio/boardlayout tests green", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "canonjson.Digest(v) (\"sha256:\"+hex over canonjson.Marshal) replaces all ten hand-rolled digest tails (bundle.recordDigest, runtime probe's recordDigest, decisionsweep.exemptionDigest, align's ComputeDigest/ComputeDecisionDigest/adrCorpusDigest tails, artifact.ObjectContentHash, commitdesign.freezeBoard, cmd/verdi's rollupDigest and selfHostedDigest); a byte-equivalence test pins the exact digest string for a committed fixture value against the pre-change golden, and every caller's existing tests stay green", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "internal/artifact owns YAML emission quoting (one exported double-quote helper carrying code-health dc-4's seam rationale) — the three yamlDQ/yamlDoubleQuote copies in align/render.go, decisionsweep/render.go, workbench/obligationauthor.go are deleted for it, with a byte-equivalence test over representative strings (plain, quotes, newlines, unicode) proving identical output to the old copies", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "internal/artifact owns the path-classification table (one exported classify function); lint's and index's walks both consume it (the walks stay separate — their failure handling legitimately differs); index thereby gains the reaffirmation case its copy silently lost, in BOTH classification and its decodeEntry, with a witness test proving a reaffirmation file is now classified and indexed where before it was silently skipped; lint's tolerant-walk behavior is unchanged (its tests are the proof) and the stale mirrors-index comment is corrected", evidence: [static, behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "the small pairs collapse, equivalence proven by existing tests plus one scaffold byte-pin: commitdesign's titleCase deleted for designscaffold.HumanizeName (commitdesign scaffold output byte-identical for a slug fixture); evidence's candidate-filter + dangling-evidence_for block extracted once and called by Fold and FoldFeature; align's PreserveDispositions pair becomes one generic with two thin wrappers; provider's criteriaStatusesChanged/criteriaChanged becomes one generic helper in internal/provider called by fake and jira; mcpserve's four lone-ref decode prologues become one helper; cmd/verdi's five fold-load prologues become one foldStoryEvidence helper threading preview as the one real parameter", evidence: [static, behavioral], anchor: "#ac-5" }
links:
  - { type: implements, ref: "spec/code-health#ac-3" }
decisions:
  - { id: dc-1, text: "atomicfile gains fsync (file, before rename) as part of the extraction even though no copy has it today — the audit flagged the uniform gap as the reason a shared helper pays: one durability fix covering all sites. This is the story's one behavior addition beyond pure extraction, disclosed here (code-health dc-2's witness is the audit's crash-durability finding); parent-directory fsync is NOT added (macOS/CI filesystems differ on dir-fsync semantics and no witness demands it — the smallest reversible step)", anchor: "#dc-1" }
  - { id: dc-2, text: "Digest lives in canonjson, not artifact: the digest IS a property of the canonical encoding (marshal then hash), canonjson is already the leaf everyone reaches through, and artifact.ObjectContentHash remains as a one-line wrapper (its exported API and doc survive; its body collapses). Field-projecting callers (probe, selfevidence, decisionsweep) keep their projection structs and call Digest on them — the helper owns the tail, not the shape", anchor: "#dc-2" }
  - { id: dc-3, text: "the classification table's home is internal/artifact (both consumers already import it; artifact imports neither) as an exported func ClassifyPath-style API returning (kind, ok) — fail closed on unknown. index's decodeEntry gains the reaffirmation arm decoding through the same strict seam as its other kinds; indexing reaffirmations is a witnessed behavior change (the audit's divergence finding), not scope creep", anchor: "#dc-3" }
  - { id: dc-4, text: "align's report/decision-report twin ORCHESTRATION is left alone (code-health dc-3: the middles diverge load-bearingly; only the byte-identical Preserve pair collapses here). Anything this story extracts must leave call-site behavior bit-for-bit identical except the two disclosed additions (fsync, reaffirmation indexing)", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "no network in any test; byte-equivalence proofs are pure unit tests over committed fixtures (digest golden string, YAML quote table, commitdesign scaffold bytes); the reaffirmation witness uses a fixture store under testdata", anchor: "#co-1" }
  - { id: co-2, text: "make verify green at every commit; one shared-home per commit (atomicfile, digest, yaml-quote, classify-table, small-pairs) so any equivalence regression bisects to one move", anchor: "#co-2" }
  - { id: co-3, text: "scope excludes the siblings: no transport work (forge-transport owns jira.go's doJSON even though this story touches jira.go's criteriaChanged — coordinate by editing only the criteria region), no file moves or renames (file-topics owns sync.go/accept.go splits; the foldStoryEvidence helper lands in a NEW file, leaving existing files in place)", anchor: "#co-3" }
---
# Shared Homes

## Problem

Shared behaviors live as hand copies against the never-copy-paste rule, and
one pair has already silently diverged.

The atomic-write idiom exists four times — boardio/boardstate.go and
boardio/graduate.go re-inline it in the SAME package as writeFileAtomic
(whose own comment names graduate.go as a user), boardlayout/file.go is the
fourth — with divergent behavior (only boardstate does MkdirAll) and a
uniform missing fsync, so none is crash-durable. The
canonjson→sha256→"sha256:"+hex digest tail is hand-copied ten times across
seven packages, every copy cross-referencing another: a format-string drift
surface. The YAML double-quote helper exists three times (align,
decisionsweep, workbench), each comment pointing at the others instead of
sharing. The twin classifyArtifactPath tables (lint/walk.go, index/walk.go)
HAVE diverged: index omits reaffirmation in both classify and decodeEntry
while lint's comment still claims a mirror — the exact bug class lint's
knownTopLevelEntries comment memorializes from the last one-walk-patched
incident. commitdesign's titleCase re-implements
designscaffold.HumanizeName with byte-indexing that is not rune-safe. And
five smaller pairs repeat verbatim: evidence's
candidate-filter/dangling-check, align's self-declared byte-identical
Preserve pair, provider's criteria-changed pair (fake vs jira), mcpserve's
four lone-ref decode prologues, cmd/verdi's five fold-load prologues.

## Outcome

One home per shared behavior, proven equivalent. internal/atomicfile owns
the atomic write, consumed by every former copy — the durability gap closed
once. canonjson owns Digest, and the ten digest copies collapse with digest
strings proven byte-identical over fixtures. internal/artifact owns the YAML
double-quote helper (the emission side of the seam, code-health dc-4) and
the path-classification table — both walks consume the table, and index
gains the reaffirmation case its copy silently lost, healing the divergence
with a witness test. titleCase is deleted for HumanizeName. The five small
pairs each collapse to one helper with every existing test still green and
scaffold outputs proven byte-identical.

## AC-1

internal/atomicfile (new leaf package) owns Write: MkdirAll + CreateTemp +
write + close + fsync + rename (dc-1), with happy and negative (unwritable
destination) table tests. The four hand copies — boardio's boardstate.go and
graduate.go inlines, boardio's writeFileAtomic itself, boardlayout/file.go —
are replaced by calls to it, all existing boardio/boardlayout tests green.
Evidence: static + behavioral.

## AC-2

canonjson.Digest(v) — "sha256:"+hex over canonjson.Marshal — replaces all
ten hand-rolled digest tails: bundle's recordDigest, the runtime probe's
recordDigest, decisionsweep's exemptionDigest, align's
ComputeDigest/ComputeDecisionDigest/adrCorpusDigest tails,
artifact.ObjectContentHash, commitdesign's freezeBoard, cmd/verdi's
rollupDigest and selfHostedDigest. A byte-equivalence test pins the exact
digest string for a committed fixture value against the pre-change golden,
and every caller's existing tests stay green. Evidence: static + behavioral.

## AC-3

internal/artifact owns YAML emission quoting: one exported double-quote
helper carrying code-health dc-4's seam rationale. The three
yamlDQ/yamlDoubleQuote copies — align/render.go, decisionsweep/render.go,
workbench/obligationauthor.go — are deleted for it, with a byte-equivalence
test over representative strings (plain, quotes, newlines, unicode) proving
identical output to the old copies. Evidence: static + behavioral.

## AC-4

internal/artifact owns the path-classification table as one exported
classify function; lint's and index's walks both consume it — the walks stay
separate, their failure handling legitimately differs. index thereby gains
the reaffirmation case its copy silently lost, in BOTH classification and
decodeEntry, with a witness test proving a reaffirmation file is now
classified and indexed where before it was silently skipped. lint's
tolerant-walk behavior is unchanged (its tests are the proof) and the stale
mirrors-index comment is corrected. Evidence: static + behavioral.

## AC-5

The small pairs collapse, equivalence proven by existing tests plus one
scaffold byte-pin. commitdesign's titleCase is deleted for
designscaffold.HumanizeName, with commitdesign's scaffold output
byte-identical for a slug fixture. evidence's candidate-filter +
dangling-evidence_for block extracts once, called by Fold and FoldFeature.
align's PreserveDispositions pair becomes one generic with two thin
wrappers. provider's criteriaStatusesChanged/criteriaChanged becomes one
generic helper in internal/provider called by fake and jira. mcpserve's four
lone-ref decode prologues become one helper. cmd/verdi's five fold-load
prologues become one foldStoryEvidence helper threading preview as the one
real parameter. Evidence: static + behavioral.

## DC-1

atomicfile gains fsync (file, before rename) as part of the extraction even
though no copy has it today — the audit flagged the uniform gap as exactly
why a shared helper pays: one durability fix covering all sites. This is the
story's one behavior addition beyond pure extraction, disclosed here
(code-health dc-2's witness is the audit's crash-durability finding).
Parent-directory fsync is NOT added — macOS/CI filesystems differ on
dir-fsync semantics and no witness demands it; the smallest reversible step.

## DC-2

Digest lives in canonjson, not artifact: the digest IS a property of the
canonical encoding (marshal then hash), canonjson is already the leaf
everyone reaches through, and artifact.ObjectContentHash remains as a
one-line wrapper — its exported API and doc survive; its body collapses.
Field-projecting callers (probe, selfevidence, decisionsweep) keep their
projection structs and call Digest on them: the helper owns the tail, not
the shape.

## DC-3

The classification table's home is internal/artifact — both consumers
already import it; artifact imports neither — as an exported
ClassifyPath-style func returning (kind, ok), fail closed on unknown.
index's decodeEntry gains the reaffirmation arm decoding through the same
strict seam as its other kinds. Indexing reaffirmations is a witnessed
behavior change (the audit's divergence finding), not scope creep.

## DC-4

align's report/decision-report twin ORCHESTRATION is left alone
(code-health dc-3: the middles diverge load-bearingly; only the
byte-identical Preserve pair collapses here). Anything this story extracts
must leave call-site behavior bit-for-bit identical except the two
disclosed additions: fsync (dc-1) and reaffirmation indexing (dc-3).

## CO-1

No network in any test. Byte-equivalence proofs are pure unit tests over
committed fixtures — the digest golden string, the YAML quote table, the
commitdesign scaffold bytes. The reaffirmation witness uses a fixture store
under testdata.

## CO-2

make verify green at every commit; one shared-home per commit (atomicfile,
digest, yaml-quote, classify-table, small-pairs) so any equivalence
regression bisects to one move.

## CO-3

Scope excludes the siblings. No transport work — forge-transport owns
jira.go's doJSON even though this story touches jira.go's criteriaChanged;
coordinate by editing only the criteria region. No file moves or renames —
file-topics owns the sync.go/accept.go splits; the foldStoryEvidence helper
lands in a NEW file, leaving existing files in place.
