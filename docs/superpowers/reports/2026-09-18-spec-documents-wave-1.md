# spec/spec-documents — Wave 1 report (ac-1 document core, ac-2 `verdi spec doc`)

Status: COMPLETE. Risk tier 3 (ac-1: the document is a projection that must never read as authority). Branch agent/spec-documents-wave-1, base af4a07f3 (main after PR #326), code head 5aca4bd2. Spec: spec/spec-documents (design/spec-documents @ bfb7b3a7, PR #327). Plan: docs/superpowers/plans/2026-09-17-spec-documents-wave-1.md (amended in review; see the ledger below).

Contract implemented: `internal/specdoc` (kinds and engine digest; facts from stubs and from the matrix projection; fence-aware body sections; the Document model and Build; the Markdown renderer whose golden output is the cross-consumer byte contract; the HTML renderer over the shared goldmark engine) and `verdi spec doc <spec-ref> [--kind spec|plan|tasks] [--format md|html] [--at <commit>] [--proposed] [-o <path>]` (default-branch bytes, pinned commit, or working tree; status through specstate; evidence through the matrix projection; exit 0/2, never 1; `-o` refuses store paths).

Explicit exclusions: readiness facts (ac-1's "when present" clause) are disclosed-as-unproven in this wave and carried to Wave 2, which owns the snapshot and adds the seam with its consumer; no board, docs-site, or MCP consumer (Wave 2); no parity test (ac-6, Wave 2).

Process: subagent-driven — one Sonnet implementer per task (Tasks 1–3 and 5–6 batched), an independent Opus task review after each, fix rounds resumed the original implementer, scoped Opus re-reviews closed each round, a final whole-branch Opus review returned REQUEST CHANGES with three Important findings, one fresh-implementer fix wave and one scoped re-review closed them. Every Important finding was a test-strength or layout defect; no production defect survived into the gate.

GREEN (controller runs): `make verify` on 5aca4bd2 → `verify OK`, exit 0 (build, fmt-check, vet, lint, race tests 109 packages, fixture, lint-store, spec-align, lint-showcase, showcase-coverage, e2e 314 passed 11.6m); recording-artifact scan 0 files; tree clean. Earlier informational runs green on 0d8e54df and f739ddd6.

Residual risks: see the ledger's "minor (deferred)" lines below; none blocks merge. Integration prerequisites: none; Wave 2 consumes `specdoc.Build`, `RenderMarkdown`, `RenderHTML`, `FactsFromSpec`, `WithMatrix(f, rec, headCommit)`, and the golden Markdown as its parity target.

## Ledger (verbatim from the plan workspace)

# SDD ledger — plan: docs/superpowers/plans/2026-09-17-spec-documents-wave-1.md
Spec: .verdi/specs/active/spec-documents/spec.md on design/spec-documents (PR #327). Branch agent/spec-documents-wave-1, base af4a07f3, plan commit 92013189. Controller: Fable 5.1. Implementers: impl-sonnet-max (repo rule). Reviewers: review-opus-max (repo rule: Opus for every task-scoped review).

## Preflight scan (2026-09-17)
| Pair / task | Produces vs consumes | Finding |
|---|---|---|
| T1 → T4 | Kind/Sections/Stamp/EngineDigest vs Build reads Kind.Sections(), fills Stamp.Engine | consistent |
| T2 → T4 | Facts{Coverage,Claims,Evidence,EvidenceSource}, KindEvidence vs Build copies them; nil-map = unknown | consistent |
| T3 → T4 | bodySections(body) map[id]text vs Build reads sections[obj.ID] | consistent; H1 excluded, "problem"/"outcome" keys unused by Build (Build takes Problem/Outcome text from frontmatter) — intentional |
| T4 → T5/T6 | Document fields vs renderer reads Words, Identity, *Known flags, Evidence rows | consistent |
| T5 → T6 | RenderMarkdown string vs RenderHTML wraps it | consistent |
| T5 → T7 | footer/header wording vs CLI tests assert "not authority", "Proposed, not accepted", "commit `<sha>`" | consistent (footer uses "commit `%s`") |
| T7 self | readSpecBytesEitherZone(root,name)(relPath,content,err) and runGitCmd/gitOutput helpers exist in cmd/verdi | verified against specstate.go:109, gc_test.go:53, close_test.go:2054 |
| T7 self | fixture uses supersedeManifestYAML (designsupersede_test.go:17) | verified |
| T5 self | test expects "Status | not resolved for this render" while identity table renders "| Status | not resolved for this render |" | substring present; consistent |
| T5 self | vocab witness: literals "Proposed, not accepted" and the footer carry `// vocab:identity` on the line above | plan-mandated; reviewer may still flag — adjudicate then |
| T2 self | TestFactsFromSpec expects `"ac-3": {}` (non-nil empty) and impl seeds `[]string{}` | consistent under reflect.DeepEqual |
| T4 self | Build sets cr.Coverage = append([]string{}, ...) → non-nil even when empty | consistent with test `len==0 && CoverageKnown` |
| Global | co-5 vocabulary: renderer routes class words via doc.Words; state word "accepted" appears only inside vocab:identity-marked literals | consistent |
Scan: no contradictions found. Rulings:
- Ruling: Tasks 1–3 batched into one implementer dispatch (same package, plan carries complete code, no interface between them beyond T1's types) — saves two review seats; cost if wrong: one larger review diff (~400 lines).
- Ruling: Tasks 5+6 batched (renderers over one model) — same reasoning.
- Ruling: implementers on impl-sonnet-max, reviewers on review-opus-max per CLAUDE.md (overrides the skill's cheapest-tier suggestion); cost if wrong: token spend only.

## Progress
Tasks 1-3: dispatched (batched) to impl-sonnet-max at BASE 92013189
Tasks 1-3: implementer DONE at e5c32160 (75ecf564, 537bc20d, e5c32160; 11/11 race tests, static clean); task review dispatched to review-opus-max over 92013189..e5c32160
Tasks 1-3: review — spec ✅, quality Needs work: F1 Important (path-marker lines transcribed as package doc comments), F2 Important (facts fixture kills neither the Spike-branch nor the declared-guard mutant). Fix round 1/5 dispatched (resume implementer).
Tasks 1-3: minor (deferred): F3 nil/empty asymmetry of ACEvidence.Kinds/Stories is the only feature-vs-story discriminator and is unpinned — Task 4 consumes Kinds/Stories only for rendering, so no discriminator is needed; pin with a fixture if Wave 2 needs one.
Tasks 1-3: minor (deferred): F4 Kind.Sections() default branch renders spec for an unknown Kind — unreachable via Build (ParseKind runs first); consider `default: return nil` in the final review.
Tasks 1-3: minor (deferred): F5 bodySections splits on `## ` inside fenced code blocks — no spec body does today; note in doc comment or track fences later.
Ruling: co-5 vocabulary discipline binds string literals (the witness scans literals), not identifiers or comments; `Stamp.Proposed` and comment prose are not gaps — cost if wrong: a witness change later would flag them, cheaply.
Ruling: for every remaining dispatch, the `// path/to/file.go` marker at the top of a plan code block names the file and is never written into it — cost if wrong: none.
Tasks 1-3: fix round 1 landed (e1a89b3b F1, c8897b6d F2; mutation proof recorded); scoped re-review dispatched over e5c32160..c8897b6d
Tasks 1-3: fix round 1/5 (2 addressed, 0 open; commits e1a89b3b..c8897b6d; re-reviewer re-ran all three mutants, all killed)
Tasks 1-3: minor (deferred): deleting only the claims-side `declared` guard (facts.go:65) leaves tests green; one more stub `{Slug: "strayspike", Spike: true, Resolves: []string{"oq-9"}}` kills it — folded into Task 4's dispatch as a one-line extra.
Tasks 1-3: complete (commits 92013189..c8897b6d, review clean)
Task 4: dispatched to impl-sonnet-max at BASE c8897b6d
Task 4: implementer DONE_WITH_CONCERNS at 1e6b00b7 (2d79a6c6 model+Build, 1e6b00b7 strayspike; 15/15 race; concerns = e2e deferred to Task 8, Sonnet trailer as instructed); review dispatched over c8897b6d..1e6b00b7
Task 4: review — spec ✅, quality Needs work: F1 Important (fragment supersedes exclusion unpinned), F2 Important (Words values unasserted), F3 Important (identity table rows unasserted; DisplayState/Revision row unproven); F4 Minor (EvidenceRow aliases caller slices), F5 Minor (Sections length-only compare). Fix round 1/5 dispatched (resume implementer) with F1–F5.
Ruling: F6 (commit trailer says Sonnet) is correct — the implementer seat is Sonnet; the Fable trailer belongs to controller commits only — cost if wrong: none.
Task 4: fix round 1 landed (4d859566, F1–F5); scoped re-review dispatched over 1e6b00b7..4d859566
Task 4: fix round 1/5 (5 addressed, 0 open; commit 4d859566; re-reviewer re-ran all three Important mutants, all killed)
Task 4: minor (deferred): EvidenceRow defensive copies unasserted by any test (correct by reading).
Task 4: complete (commits c8897b6d..4d859566, review clean)
Tasks 5-6: dispatched (batched: Markdown + HTML renderers) to impl-sonnet-max at BASE 4d859566
Tasks 5-6: implementer DONE at ecb9c23e (76f874c2 markdown+goldens, ecb9c23e html; 19/19 race; goldens reviewed by eye, no renderer edits); review dispatched over 4d859566..ecb9c23e
Tasks 5-6: review — spec ❌ (F2 silent empty criteria section violates ac-1/co-6), quality Needs work. Critical: F1 pluralWord("story")="storys"; F2 no empty-state line for criteria. Important: F3 list indent breaks at item 10; F4 Question.Detail never rendered; F5 constraint detail is a lazy continuation (HTML merges it) + whitespace-only lines; F6 hard-coded "Planned"/"Research" adjectives double under the plain preset; F7 determinism test drops Build errors; A1 Evidence/Coverage continuation lines fold into one HTML paragraph; A3 plan/tasks subsets never state criterion text; A4 Revision counts read as criteria counts; A5 decision headings are bare ids; A6 anchors only on criteria. Minor: A2, A7, A9, A10, F8, F9, F10.
Ruling (plan conflicts, spec as authority — dc-3 "ids as unobtrusive anchors", ac-1 "never omits a section", ac-6 parity, ac-11 plain preset): the plan's renderer layout is amended as follows. Words gains StoryPlural/SpikePlural from DisplayClassPlural (F1). Criteria section gets "No acceptance criteria are declared." (F2). Continuation indent computed from the marker width (F3). Evidence/Coverage become nested list items under the criterion (A1). Question detail rendered under its bullet; constraint detail as a separate indented paragraph, indenting non-empty lines only (F4, F5). Adjectives dropped: the class word alone, capitalised at sentence start ("Story `slug` covers …", "Spike `slug` answers …", "until a spike claims it") (F6). Decision headings become the decision text with `<a id="dc-N"></a>`; constraints and questions get the same anchor (A5, A6). Plan lines carry each covered criterion's text in parentheses so plan/tasks read standalone; Evidence table's Criterion column is "id — text" (A3). Revision row reads "vs <predecessor>: N objects carried, N amended, N amended (advisory), N removed, N added" (A4, in build.go). Proposed header uses the reviewer's wording (A7). claimsLine returns the predicate only, label in the renderer (A9). `|` escaped in table cells (F8). Determinism test checks errors and all kinds (F7); a test pins RenderHTML == render.RenderMarkdown(RenderMarkdown(doc)) and the trailing newline (F10). A second fixture (story class, renamed vocabulary via a real *model.Model, ≥10 criteria with prose under ac-10, a constraint with prose, a question with prose, no stubs) with its own goldens closes the blind spots. Cost if wrong: Wave 2 consumers re-pin against the amended bytes; nothing is merged yet.
Ruling: A2 (slug in Plan vs spec ref in Evidence) is intended — a stub is not yet a story; the Plan section gains one lead sentence stating that a planned story becomes spec/<slug> when instantiated — cost if wrong: one sentence.
Tasks 5-6: minor (deferred): A8 raw `<a id>` visible to CLI readers (accepted cost of one Markdown source); A10 fragment supersedes links absent from Identity (object-level overrides; Wave 2 may add an Overrides line); F9 `-update` runs are vacuous by design.
Tasks 5-6: fix round 1/5 dispatched (resume implementer) with F1–F8, F10, A1, A3–A7, A9 and the second fixture.
Tasks 5-6: fix round 1 landed (b9f1e785 plurals + revision format; 3fe1bb7c layout rewrite; 69fce453 renamed fixture + goldens; 26/26 race; RED for F1/F2/F3 captured against pre-fix code).
Ruling: second fixture is class feature, not story — the story class requires problem/outcome by contract, so "no problem statement" can only be exercised on a feature; the fixture's other purposes are class-independent — cost if wrong: one Identity row not exercising a rename.
Ruling: the implementer's `withPlannedAdjective` spelling guard is rejected — a renderer never inspects a display word; the adjective "planned" is dropped from the plan lead sentence, the empty-plan line, and the coverage line ("covered by …") — cost if wrong: three golden lines.
Tasks 5-6: addendum landed (689baeee); scoped re-review dispatched over ecb9c23e..689baeee (4 commits)
Tasks 5-6: fix round 1/5 (19 addressed, 0 open; commits b9f1e785..689baeee; re-reviewer killed the indent mutant; reading check: amended layout reads correctly for PM and agent). New from the fix diff: F1' Important (pluralIf plural branch untested — always-singular mutant survives); minors N1 lead sentence above an empty plan, N2 "1 objects", N3 double period, A3' dangling dash for an id absent from Criteria, A5' goldmark slugs the anchor markup into the heading id (accepted, cosmetic).
Tasks 5-6: fix round 2/5 dispatched (resume implementer) with F1', N1, N2, N3, A3'.
Tasks 5-6: minor (deferred): A5' heading slug includes the anchor markup — cosmetic; `#dc-1` resolves via the explicit anchor.
Tasks 5-6: fix round 2 landed (121f441b; pluralIf mutant killed); scoped re-review dispatched over 689baeee..121f441b
Tasks 5-6: fix round 2/5 (5 addressed, 0 open; commit 121f441b; pluralIf mutant killed on 6 lines)
Tasks 5-6: minor (deferred): cmd/verdi/designsupersede.go:291 still prints "%d objects carried" unpluralised (different surface, pinned by its tests) — align in a later round.
Tasks 5-6: complete (commits 4d859566..121f441b, review clean)
Task 7: dispatched to impl-sonnet-max at BASE 121f441b
Task 7: implementer DONE at 0d8e54df (verb + tests; full cmd/verdi race 470s ok; full specalign ok; one vocab:identity marker for 'draft' homograph); review dispatched over 121f441b..0d8e54df; Task 8 make verify started on 0d8e54df at port base 4390 (log scratchpad/w1-verify-1.log)
Task 7: review — spec ❌: F1 Critical (flags after the ref silently dropped; brief defect), F2 Important (--proposed test commits the edit first, cannot distinguish HEAD from worktree), F3 Important (help text edit untested; topLevelUsage row stale), F4 Minor (extra positionals ignored), F5 Minor (doubled "usage:" prefix; report's byte-identical claim false). Clean: --at never falls back; -o never writes on failure; archive zone works. Fix round 1/5 dispatched (resume implementer) with F1–F5; the concurrent make verify on 0d8e54df becomes informational; the gate re-runs on the fixed head.
Ruling: the brief's flag-parse block is amended — loop re-parse so flags may appear in any position; any second positional refuses — cost if wrong: one parse helper.
Task 7: fix round 1 landed (7a391dc7 parse loop + usage form; f739ddd6 tests); scoped re-review dispatched over 0d8e54df..f739ddd6
Task 7: fix round 1/5 (5 addressed, 0 open; commits 7a391dc7..f739ddd6; re-reviewer drove the binary: flags in any position, `--` separator, equals spellings, extra positional refused)
Task 7: minor (deferred): `matrixprojection.Project` resolves evidence against the live tree, not the `--at` commit — the Evidence section's source line says "matrix at <commit>" using the render commit; a pinned render's evidence is therefore "as of now", which the section wording should say (Wave 2 candidate).
Task 7: complete (commits 121f441b..f739ddd6, review clean)
Task 8: informational make verify on 0d8e54df still running; definitive make verify on f739ddd6 queued behind it; final whole-branch review dispatched over af4a07f3..f739ddd6 (deferred minors: see the "minor (deferred)" lines above; rulings: see "Ruling:" lines).
Task 8: informational make verify on 0d8e54df = verify OK (314 e2e, 109 pkgs; tree moved to f739ddd6 mid-run). Definitive make verify started on f739ddd6 (log scratchpad/w1-verify-2.log).
Final review (whole branch af4a07f3..f739ddd6): REQUEST CHANGES. Important: F1 Evidence "Source: matrix at <render commit>" is a false provenance claim (the matrix reads the live tree); F2 evidenceDetail's Kinds branch (obligation state) has no test or golden; F3 `-o` can overwrite a store artifact under .verdi/. Minor: F4 empty-plan wording collides with the plain preset; F5 escapeCell ignores newlines; F6 proposed header wrong on the default branch; F7 Words.Feature unused. Contract freeze: EvidenceSource must be core-owned; decision headings hide the id. ac-1 clause "readiness snapshot when present" UNPROVEN in Wave 1 (no seam, no consumer).
Ruling: ac-1's readiness clause is disclosed-as-unproven for Wave 1 and carried to Wave 2, whose board tab owns the snapshot and adds the Facts seam with its consumer; doc.go's comment is amended to say so — cost if wrong: Wave 2 re-pins the Evidence/readiness wording.
Ruling: EvidenceSource wording is owned by the core: WithMatrix(f, rec, headCommit) sets "matrix over the working tree at <12 hex>" and the CLI passes the HEAD it resolved, never the render commit — cost if wrong: one golden line per document.
Ruling (contested l.49 accepted): decision headings render `### dc-N — <text> <a id="dc-N"></a>` so the id is visible in HTML like every other object — cost if wrong: four golden lines.
Ruling: F7 Words.Feature stays (Wave 2's Identity row and docs-site shell will read it) — cost if wrong: one dead field.
Ruling: the fence-aware bodySections fix (deferred l.31) is pulled into this wave: the docs site will render every corpus spec in Wave 2 — cost if wrong: six lines.
Final fix wave dispatched (one fresh impl-sonnet-max) with F1–F6, the two freeze items, the readiness disclosure comment, and the fence fix; the definitive make verify on f739ddd6 becomes informational; the gate re-runs on the fixed head.
Task 8: make verify on f739ddd6 = VERIFY_EXIT=0 (314 passed (12.0m)); informational since the final fix wave follows.
Final fix wave landed: f739ddd6..5aca4bd2 (8 commits; items 1-9). Ruling: the store-containment check resolves symlinks on the longest existing ancestor before Abs+Rel (macOS /var→/private/var made the literal check pass a store path as outside; witnessed by the item's own refusal test) — cost if wrong: one helper. Ruling: the proposed header names the commit without backticks (matches the EvidenceSource prose) — cost if wrong: one golden line. Scoped re-review dispatched over f739ddd6..5aca4bd2; definitive make verify started on 5aca4bd2 (log scratchpad/w1-verify-3.log).
Final fix wave re-review: all findings ADDRESSED (F1–F6, freeze(b), items 8, 9); containment check reviewed on five edge shapes, no blocking finding; byte contract judged freezable for Wave 2.
Final: minor (deferred): case-insensitive filesystems let `-o <root>/.VERDI/x` past the store guard (accident guardrail, not an adversary boundary); the Evidence Source line's terminal period abuts the hex prefix; under --at the Identity commit and the Source commit are two facts with no explaining line; the empty-plan sentence is active voice against passive siblings; goldmark's heading slug for decisions includes the anchor markup (HTML ids churn on text edits); a bare `\r` is not collapsed by escapeCell; the fence tracker deviates from CommonMark on info-string closers and >3-space indents (unreachable in the corpus).
Ruling: the plan file keeps its original code blocks (it is the argument, not the implementation); a closing "Amendments during execution" note pointing to this ledger is appended so a reader is not misled — cost if wrong: none.
Task 8: complete — make verify on 5aca4bd2: verify OK, VERIFY_EXIT=0 (314 e2e passed 11.6m, 109 race packages, recording-artifact scan 0, tree clean). Wave 1 complete (commits af4a07f3..5aca4bd2 + this report commit).
