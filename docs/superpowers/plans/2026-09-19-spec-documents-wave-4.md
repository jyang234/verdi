# Spec Documents — Wave 4 Implementation Plan (ac-10 policy adopt --starter, ac-11 plain vocabulary, ac-12 guidance-first cards)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the last three criteria of spec/spec-documents: a project adopts a starter constitution with one verb (`verdi policy adopt --starter [--profile solo|team]`, closing tracker UAT-018), new stores speak plain words by default (`verdi init` writes the `plain` vocabulary preset; lint lines lead with the sentence), and readiness cards lead with what to do next (guidance first on the wall shell; formal identifiers in the technical disclosure).

**Architecture:** ac-10 lands under spec/context-integrity dc-4's configurable-scaffold seam: four new embedded templates in `internal/designscaffold/templates/` (constitution, solo profile, team profile, starter policy, consumers inventory) resolve through the existing `humanartifact.ResolveScaffold` (store override wins) and render through `designscaffold.RenderValue`; four new `humanartifact.Render*` functions strict-decode and kernel-round-trip each result; a new package `internal/policyadopt` composes the four renders, proves the composed tree through `policyauthority.LoadFromSource` + `Resolve` + the design_assistance grant over an in-memory tree BEFORE anything is written, then writes the four files; `cmd/verdi/policy.go` cuts `policy/adopt` from the resolved default branch (design start's own dc-7 chain, generalized), writes, stages exactly the four paths, and commits. The profile and the consumers inventory gain the `template` provenance seam they lack today (`policyartifact.TemplateRecord` moves down to `governanceprincipal` and is aliased back; `constitutionimpact.Inventory` gains an optional `template` field). ac-11 is a preset in `internal/initwizard` (`PlainPreset`), a `--vocabulary plain|canonical` flag on `verdi init` (default plain; the wizard's defaults are seeded from the preset), and a one-line reshape of `lint.Finding.String()`'s violation branch. ac-12 is a Fable lane over `boardshellrender.go` (guidance primary, fact secondary, timing into the disclosure), plus the guide's no-wizard disclaimer and the context/policy concern's guidance naming the verb.

**Tech Stack:** Go 1.25 (`github.com/jyang234/verdi`), `embed`, `text/template` via `internal/designscaffold`, `internal/humanartifact`, `internal/policyartifact`, `internal/policyauthority`, `internal/governanceprincipal`, `internal/constitutionimpact`, `internal/draftmutation` (`ResolvePolicyGrant`), `internal/gitx`, `internal/atomicfile`, `internal/fixturegit`, `internal/initwizard`, `internal/model`, `internal/lint`, `internal/workbench` string-builder renderers, Playwright (`e2e/tests`).

**Spec:** `.verdi/specs/active/spec-documents/spec.md` on main 96c126a5 (accepted in PR #327). This plan implements ac-10, ac-11, ac-12 under co-1..co-6, dc-6, dc-7, dc-8; binding presentation authority for ac-12 is `docs/superpowers/specs/2026-08-29-wave-6-workbench-presentation-design.md` §3.1/§8.2 (dc-2). Waves 1–3 (`docs/superpowers/reports/2026-09-18-spec-documents-wave-{1,2,3}.md`) are the base. Ledger rows SI-204 (ac-10 starter shape), SI-205 (plain preset mapping), SI-206 (ac-12 primary-line rule) are recorded in Task 0 before any dispatch.

## Global Constraints

- No network in any test (co-1). CLI paths through the built binary (`buildVerdiBinary` + `runVerdi`, `cmd/verdi/vocabulary_cli_test.go:81`) over `internal/fixturegit` repos; browser paths through Playwright over `cmd/e2eharness` (`VERDI_E2E_PORT_BASE`); git through fixtures.
- The document and the skills are never authority (co-2); this wave adds no document or skill surface and touches no `internal/specdoc` or `internal/skillpack` file.
- The write surface stays closed (co-3): no MCP tool is added or changed; `verdi policy adopt` is a CLI verb that writes exactly four store paths and one commit on a new branch, never to the default branch.
- co-4: all workbench markup, CSS, JS, and Playwright paths (Task 4) are Fable work; screenshot, trace, video, and screen recording stay disabled; run the recording-artifact scan after every Playwright run.
- co-5: every new production literal containing a class word (`feature`, `story`, `component`, `spike`) or a lifecycle state word (`draft`, `proposed`, `accepted-pending-build`, `accepted`, `superseded`, `closed`) routes through `*model.Model` (`DisplayClass`, `DisplayState`) or carries `// vocab:identity — <why>` on its line or the line above. Embedded templates are data files, outside the witness. `go test -count=1 ./internal/specalign/ -run TestVocabProseWitness` must pass after every task. New CLI usage literals carry `// vocab:identity — CLI usage/flag grammar (identity)` (precedent `cmd/verdi/harness.go:22`).
- co-6: three-valued honesty in every card and every verb line: the starter's empty consumers inventory and the team profile's empty role mappings are DISCLOSED on stdout and in each artifact's rationale, never presented as complete.
- dc-6: the starter lands under context-integrity dc-4's scaffold seam using `policyartifact.TemplateRecord`, before that spec's acceptance, with ledger row SI-204 (the repository-visible successor of PLAN.md §7 is `docs/superpowers/invention-ledger.md`).
- dc-7: the plain vocabulary is configuration in `.verdi/model.yaml`'s Vocabulary block written at init; no existing store is ever rewritten (`verdi init` is create-only and refuses on any existing `.verdi/`, `cmd/verdi/init.go:109`).
- ac-11 (verbatim): "the renameable set and the vocabulary witness are unchanged" — `internal/model/validate.go:171 validateVocabulary`, `internal/initwizard/vocabulary.go:60 RenameableIDs`, and `internal/specalign/vocabprose_test.go` are NOT edited.
- ac-12 (verbatim): "the four area labels and the proven, needs-attention, not-enough-evidence triad are unchanged; no derivation changes" — `internal/readinesspilot` is NOT edited; `boardspecasd.go`'s derivation of states, areas, and attention order is NOT edited (only the context/policy guidance STRING at `boardspecasd.go:353` changes, Task 4).
- Serialized registries: Task 2 alone edits `cmd/verdi/dispatch.go`, `cmd/verdi/help.go`'s `topLevelUsage`/`verbUsage` (Task 3 edits ONLY the `"init"` usage value in `verbUsage` and the init line in `topLevelUsage` — sequential tasks, disjoint hunks), `internal/specalign/verbs_test.go`, `internal/showcasealign/coverage_test.go`.
- Exit codes: 0 clean / 1 verdict / 2 operational. For `policy adopt`: 2 for flag shape, unusable root, unreadable git identity, an existing `policy/adopt` branch, I/O; 1 when the checkout already carries `.verdi/policy` or `.verdi/constitution/consumers.json` (adoption is a verdict about the store, not an operational fault); 0 on success.
- Risk tiers (dc-8): Tasks 1 and 2 (ac-10) are Tier 3 — Sonnet implementer, independent Opus review, a fresh Opus fixer and a fresh Opus re-reviewer for any Critical or Important finding. Task 3 (ac-11) is Tier 2. Task 4 (ac-12 + the ac-10 guide text) is Tier 2, Fable implementer.
- gofmt-clean, `golangci-lint` clean (`.golangci.yml`), `go vet` clean, `go test -race` clean. Commit subjects imperative. Implementer commits end with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` (Fable lane: `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`). Never write a `// path/to/file.go` code-block marker into a file.
- Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/spec-documents-w4` on branch `agent/spec-documents-wave-4` (base main 96c126a5). Never use bare `git stash`. Read `/Users/johnyang/code/verdi-system/CLAUDE.md` and the repo's `CLAUDE.md` first. Port base: `VERDI_E2E_PORT_BASE=4390` for the wave gate; lanes use 4490.

---

## Rulings recorded in this plan

- **R-W4-1 (starter shape — SI-204).** ac-10 names four artifacts and the TemplateRecord seam, but today only `policy.md` has a template, `RenderPolicy` enforces empty claims/instructions/payloads (a placeholder skeleton), the governance profile's frontmatter is a closed 12-field strict decode with no `template` key, and the consumers inventory is canonical JSON with no template field. Smallest reversible shape: (a) four new embedded templates `policy-constitution.md`, `governance-profile-solo.md`, `governance-profile-team.md`, `policy-starter.md`, `constitution-consumers.json` (the embed pattern widens from `templates/*.md` to `templates/*`); (b) four new renderers in `internal/humanartifact` sharing `verifyKernelRoundTrip`; (c) `policyartifact.TemplateRecord` moves to `governanceprincipal.TemplateRecord` with `policyartifact.TemplateRecord` a type alias (no caller changes), `Profile` gains `Template *TemplateRecord` (optional, digest-bound like every other field), and `constitutionimpact.Inventory` gains `Template *policyartifact.TemplateRecord` (optional, canonical-encoded as `template`); (d) the one real rule is the typed `design_assistance` payload — mode `draft-write` for the solo profile (one principal fills every role; ac-8's skills write drafts through `mutate_draft`, which needs draft-write) and `proposal-only` for the team profile (agents propose, humans write, until the team's own review grants more); claims, instructions, adapters, and subject catalogs are explicitly empty, never placeholder rules; (e) the solo profile binds every role to the checkout's own local git identity through a `local-operator` trust source (the ratified 2026-09-05 local-operator design, SI-183) — a real, self-asserted identity, never a fabricated one; the team profile declares a `forge` trust source, requires one reviewer distinct from the author at `accept` and one policy-owner at `policy-disposition-approval`, and maps NO subjects (the team's own data, disclosed); (f) `--owner <kebab-handle>` is accepted: default `local-operator` for solo, required for team (an owner handle must be kebab-case, `policyartifact/kernel.go:147`, and a team has no honest default); (g) the consumers inventory is written empty and disclosed as registering no consumers yet (impact coverage over it resolves `consumer-universe-empty`, not proven); (h) profile ids `starter-solo`/`starter-team`, policy id `policy/starter`, constitution id `policy-constitution/constitution` (fixed by the decoder). Cost if wrong: template bytes and one constant per profile.
- **R-W4-2 (prove before write).** `policyadopt.Plan` renders all four artifacts into memory, loads them with `policyauthority.LoadFromSource(fstest.MapFS)`, resolves the effective policy, and proves exactly one valid `design_assistance` payload with the expected mode, BEFORE any file is written and BEFORE the branch is cut. A refusal leaves the checkout byte-untouched on its current branch. Cost if wrong: none (strictness only).
- **R-W4-3 (branch base).** `policy/adopt` is cut from the resolved default branch exactly as `design start` cuts `design/<name>` (spec/uat-round-1 dc-7 chain, I-130): `resolveDesignStartBase` and `checkoutNewDesignBranch` are generalized behind a verb-prefix parameter (`resolveBranchBase(ctx, root, verb, …)`, `checkoutNewBranchDisclosed(ctx, root, verb, …)`) with the two design-start callers delegating byte-identically. The plan is re-proven after the checkout (the base tree may differ from the checkout the verb started on). Cost if wrong: one parameter.
- **R-W4-4 (plain preset mapping — SI-205).** ac-11 lists four phrases (planned story, research task, revision, research spike) but the v1 renameable set is exactly classes `feature`/`story`/`spike`, states `draft`/`accepted-pending-build`/`closed`/`superseded`, verbs `merge`/`close`, and the spec pairs no phrase with an id. Two phrases are the pair Wave 1 already pinned as the renamed-vocabulary fixture (`internal/specdoc/renamedstory_test.go:24`: story → "planned story", spike → "research spike"); "research task" and "revision" name no renameable id. The preset is therefore `classes: {story: "planned story", spike: "research spike"}` and nothing else; the two unplaced phrases are recorded in SI-205 for the owner. Extending the preset is one map literal in `internal/initwizard/preset.go`. Cost if wrong: two map entries.
- **R-W4-5 (lint line grammar).** A violation line becomes `<message> (<path>) [<rule>]` — the sentence leads, the rule code trails in brackets, the path rides between in parentheses. Disclosure lines (`disclosed-unproven [lint:VL-017] <path>: <text>`) are the shared disclosure grammar (`internal/disclosure.Render`, recognized by `IsRendered` across gate/closure/dex) and are NOT reshaped: ac-11 speaks of lint's own output, and a second disclosure grammar would split the seam. Cost if wrong: one format string.
- **R-W4-6 (the preset seeds the wizard).** `initwizard.RunInterview` gains a `seed model.Vocabulary` parameter: each rename prompt's Enter-default is the seed's value for that id when present, else the id; the result starts as a copy of the seed. `--wizard` with the default preset therefore lands the preset unless the operator overrides an entry; `--vocabulary canonical --wizard` is today's interview byte-for-byte. Cost if wrong: one parameter.
- **R-W4-7 (ac-12 primary-line rule — SI-206).** The readiness page's `readinesspilot.Concern` carries one prose field (`Summary`, the journey's clearing condition or presence statement) and no separate guidance sentence; the wall's `asdConcern` carries both `Summary` (the fact) and `Guidance` (what to do) but renders the fact first and the guidance after the chip. Rule for both surfaces: the primary line is the guidance sentence when the row carries one, otherwise the summary; when both exist the fact stays visible as a secondary line (co-6: the state of affairs is never hidden behind the instruction); concern id, timing, and blocking flag live in the technical disclosure. On `/readiness` this is already the rendered shape (`readinessrender.go:280-353`: Summary primary; Concern/Blocking/Timing rows in the disclosure) and is pinned by `TestReadinessRender_SummariesArePrimaryCopy` and `TestReadinessRender_TechnicalDetailsComplete` — no change there. On the wall the inline `.asd-timing` span leaves the stage line and becomes a `Timing` row in the disclosure (`now` / `later — waits on <label>`), plus `data-timing` on the article. No derivation changes. Cost if wrong: markup only.
- **R-W4-8 (the guide points at the verb).** The not-adopted policy guide's `ritual-note` "the workbench has no setup wizard and no adoption control" becomes a pointer at `verdi policy adopt --starter [--profile solo|team]`; the guide stays read-only markup (no control), keeps its four read-only checks and the four file rows, and the context/policy concern's guidance names the verb AFTER the inspect-first step (`TestPolicyGuide_PolicyLessCheckoutIsInspectFirst` orders "Inspect" before "adopt"). Cost if wrong: prose.
- **R-W4-9 (`--starter` is the only form).** `verdi policy` and `verdi policy adopt` without `--starter` exit 2 with the usage line; `--profile` values other than `solo`/`team` exit 2 by name. Bare `verdi policy` fails on argument shape before any root resolution (specalign's inventory runs it against the live checkout). Cost if wrong: none.

---

## File structure

| File | Responsibility |
|---|---|
| `docs/superpowers/invention-ledger.md` | SI-204, SI-205, SI-206 (controller, Task 0). |
| `internal/governanceprincipal/template.go` | `TemplateRecord` (moved from policyartifact) + `Validate`. |
| `internal/governanceprincipal/profile.go`, `decode.go` | `Profile.Template`; `profileDoc.Template`; decode/validate pass-through. |
| `internal/policyartifact/kernel.go` | `type TemplateRecord = governanceprincipal.TemplateRecord`. |
| `internal/constitutionimpact/schema.go`, `inventory.go` | `Inventory.Template`; `inventoryDoc.Template`; encode/decode. |
| `internal/designscaffold/render.go` | embed pattern `templates/*`. |
| `internal/designscaffold/templates/policy-constitution.md`, `governance-profile-solo.md`, `governance-profile-team.md`, `policy-starter.md`, `constitution-consumers.json` | The five starter templates. |
| `internal/humanartifact/kernel.go` | kernel rows for `policy-constitution` and `governance-profile`. |
| `internal/humanartifact/starter.go`, `starter_test.go` | `RenderConstitution`, `RenderProfile`, `RenderStarterPolicy`, `RenderConsumersInventory` + data types. |
| `internal/policyadopt/doc.go`, `plan.go`, `write.go`, `plan_test.go` | Starter composition, in-memory proof, file writes. |
| `cmd/verdi/policy.go`, `policy_test.go` | `verdi policy adopt --starter [--profile solo\|team] [--owner <handle>]`. |
| `cmd/verdi/design.go` | `resolveBranchBase` / `checkoutNewBranchDisclosed` generalization (R-W4-3). |
| `cmd/verdi/dispatch.go`, `help.go`, `internal/specalign/verbs_test.go`, `internal/showcasealign/coverage_test.go`, `cli_showcase_test.go`, `README.md`, `docs/policy-setup-validation.md` | Registry, usage, inventory pin, `cli:policy` row + showcase proof, prose. |
| `internal/initwizard/preset.go`, `preset_test.go`, `interview.go` | `PlainPreset`, `ParseVocabularyPreset`, seeded interview. |
| `cmd/verdi/init.go`, `init_test.go` | `--vocabulary plain\|canonical`; default plain. |
| `internal/lint/finding.go`, `finding_test.go`, `cmd/verdi/designsupersede_test.go`, `cmd/verdi/lint.go` | Violation line grammar (R-W4-5). |
| `internal/workbench/boardshellrender.go`, `boardspecasd.go:353`, `boardshellrender_test.go` (new), `internal/dex/assets/style.css` | Fable: guidance-first cards, timing row, guide pointer. |
| `e2e/tests/50-design-workbench.spec.ts`, `e2e/tests/81-guidance-first-cards.spec.ts` | Fable: updated and new Playwright paths. |
| `docs/superpowers/reports/2026-09-19-spec-documents-wave-4.md` | Wave report (Task 6). |

---

## Task 0 (controller): record SI-204, SI-205, SI-206

Controller-authored before any dispatch (spec-only authority work). Append three rows after SI-203 in `docs/superpowers/invention-ledger.md`'s final 4-column table (text in the ledger; summaries: SI-204 = R-W4-1 (a)–(h) under dc-6; SI-205 = R-W4-4; SI-206 = R-W4-7). Commit: `Record SI-204..206 for spec-documents wave 4`.

---

### Task 1: Template seams and the four starter renderers (Tier 3)

**Files:**
- Create: `internal/governanceprincipal/template.go`, `internal/designscaffold/templates/policy-constitution.md`, `internal/designscaffold/templates/governance-profile-solo.md`, `internal/designscaffold/templates/governance-profile-team.md`, `internal/designscaffold/templates/policy-starter.md`, `internal/designscaffold/templates/constitution-consumers.json`, `internal/humanartifact/starter.go`, `internal/humanartifact/starter_test.go`
- Modify: `internal/policyartifact/kernel.go:32-50` (alias), `internal/governanceprincipal/profile.go:222-235` (`Template` field), `internal/governanceprincipal/decode.go:16-28,86-165` (`profileDoc.Template`, pass-through after `Validate`), `internal/constitutionimpact/schema.go:75-78`, `internal/constitutionimpact/inventory.go:16-19,34-60,65-90`, `internal/designscaffold/render.go:19-21` (embed pattern), `internal/humanartifact/kernel.go:73-84` (two rows), tests beside each.
- Test: as above plus `internal/governanceprincipal/decode_test.go` (template round trip + bad digest), `internal/policyartifact/profile_test.go` (stored profile with template line), `internal/constitutionimpact/inventory_test.go` (template field canonical round trip; unknown sibling key still refused).

**Interfaces:**
- Consumes: `humanartifact.Scaffold`, `ResolveScaffold`, `verifyKernelRoundTrip` (`policy.go:436`), `designscaffold.RenderValue`, `policyartifact.DecodeConstitution/DecodePolicy/DecodeStoredProfile`, `constitutionimpact.DecodeInventory/EncodeInventory`, `governanceprincipal.DecodeProfile`.
- Produces (Task 2 consumes exactly these):

```go
package humanartifact

// Template filenames (bare, under .verdi/templates/ for overrides).
const (
	ConstitutionTemplate  = "policy-constitution.md"
	ProfileSoloTemplate   = "governance-profile-solo.md"
	ProfileTeamTemplate   = "governance-profile-team.md"
	StarterPolicyTemplate = "policy-starter.md"
	InventoryTemplate     = "constitution-consumers.json"
)

type ConstitutionScaffoldData struct {
	Title            string
	Owners           []string
	ProfileID        string
	TemplateIdentity string
	TemplateDigest   string
}
type ProfileScaffoldData struct {
	ProfileID        string
	Class            governanceprincipal.Class // ClassSolo or ClassTeam
	Subject          string                    // solo: the local git identity; team: ""
	TemplateIdentity string
	TemplateDigest   string
}
type StarterPolicyScaffoldData struct {
	Name                 string // "starter"
	Title                string
	Owners               []string
	DesignAssistanceMode string // "draft-write" | "proposal-only"
	TemplateIdentity     string
	TemplateDigest       string
}
type InventoryScaffoldData struct {
	TemplateIdentity string
	TemplateDigest   string
}

func RenderConstitution(scaffold Scaffold, data ConstitutionScaffoldData) (string, error)
func RenderProfile(scaffold Scaffold, data ProfileScaffoldData, catalog governanceprincipal.Catalog) (string, error)
func RenderStarterPolicy(scaffold Scaffold, data StarterPolicyScaffoldData) (string, error)
func RenderConsumersInventory(scaffold Scaffold, data InventoryScaffoldData) ([]byte, error)
```

- [ ] **Step 1: Move `TemplateRecord` down and add the two seams (failing tests first)**

`internal/governanceprincipal/decode_test.go` — add:

```go
func TestDecodeProfile_TemplateRecordRoundTrips(t *testing.T) {
	catalog := Catalog{Roles: []string{"author"}, Transitions: []string{"accept"}}
	raw := []byte(`schema: verdi.governance-profile/v1
id: p
class: solo
applicable_transitions: [accept]
identity_trust_sources: [{id: local, kind: local-operator}]
role_mappings: [{role: author, trust_source: local, subjects: [a@x]}]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
template: {identity: "embedded:governance-profile-solo.md", digest: "sha256:` + strings.Repeat("a", 64) + `"}
`)
	p, err := DecodeProfile(raw, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if p.Template == nil || p.Template.Identity != "embedded:governance-profile-solo.md" {
		t.Fatalf("template = %+v", p.Template)
	}
	// The record is content: a profile with and without it digests differently.
	without, err := DecodeProfile(bytes.Replace(raw, []byte("template:"), []byte("#template:"), 1), catalog)
	if err != nil {
		t.Fatal(err)
	}
	d1, _ := p.Digest()
	d2, _ := without.Digest()
	if d1 == d2 {
		t.Fatal("template record must be digest-bound")
	}
	// A malformed digest fails closed, naming the field.
	bad := bytes.Replace(raw, []byte("sha256:"), []byte("md5:"), 1)
	if _, err := DecodeProfile(bad, catalog); err == nil || !strings.Contains(err.Error(), "template.digest") {
		t.Fatalf("bad digest: %v", err)
	}
}
```

`internal/constitutionimpact/inventory_test.go` — add:

```go
func TestInventory_TemplateFieldRoundTrips(t *testing.T) {
	rec := &policyartifact.TemplateRecord{Identity: "embedded:constitution-consumers.json", Digest: "sha256:" + strings.Repeat("b", 64)}
	enc, err := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{}, Template: rec})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1","template":{"digest":"sha256:` + strings.Repeat("b", 64) + `","identity":"embedded:constitution-consumers.json"}}` + "\n"
	if string(enc) != want {
		t.Fatalf("encoded = %s", enc)
	}
	dec, err := DecodeInventory(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Template == nil || *dec.Template != *rec {
		t.Fatalf("decoded template = %+v", dec.Template)
	}
	// Absent template still encodes exactly as before (no "template" key).
	enc0, _ := EncodeInventory(Inventory{Schema: InventorySchema, Consumers: []Consumer{}})
	if string(enc0) != `{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1"}`+"\n" {
		t.Fatalf("no-template encoding changed: %s", enc0)
	}
	// A bad record fails closed before the canonical-bytes check.
	if _, err := DecodeInventory([]byte(`{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1","template":{"digest":"nope","identity":"x"}}` + "\n")); err == nil {
		t.Fatal("bad template digest accepted")
	}
}
```

Run: `go test ./internal/governanceprincipal/ -run TestDecodeProfile_TemplateRecord -count=1; go test ./internal/constitutionimpact/ -run TestInventory_TemplateField -count=1` — Expected: FAIL (unknown field `template`; no `Template` field).

Implement:
- `internal/governanceprincipal/template.go`: move the `TemplateRecord` type, its doc comment, `sha256Re`, and `Validate` verbatim from `policyartifact/kernel.go:28-50` (keep the AC-1 wording; add one sentence: "Declared here, below policyartifact, so the governance profile can carry the same record; policyartifact aliases it."). If `policyartifact` still uses `sha256Re` elsewhere, keep a local copy there (grep first).
- `policyartifact/kernel.go`: `type TemplateRecord = governanceprincipal.TemplateRecord` with the original doc comment above it. Everything else compiles unchanged (verify: `go build ./...`).
- `governanceprincipal/profile.go`: `Template *TemplateRecord `json:"template,omitempty"`` after `EscalationThresholds`, doc comment "the resolved scaffold record a starter-written profile carries (spec/spec-documents ac-10, SI-204); optional — a hand-authored profile carries none; digest-bound like every other exported field".
- `governanceprincipal/decode.go`: `profileDoc.Template *TemplateRecord `yaml:"template"``; in `docToProfile` after the mandatory-field switch: `if doc.Template != nil { if err := doc.Template.Validate(); err != nil { return Profile{}, fmt.Errorf("governanceprincipal: profile %w", err) } }` and set `Template: doc.Template` in the literal. (`Validate` already says `template.digest …`; the prefix makes the message name the profile.)
- `constitutionimpact/schema.go`: `Inventory.Template *policyartifact.TemplateRecord` (doc: "the resolved scaffold record a starter-written inventory carries (ac-10, SI-204); optional; encoded as the canonical `template` key when present"). `inventory.go`: `inventoryDoc.Template *policyartifact.TemplateRecord `json:"template,omitempty"``; `DecodeInventory`: after the schema check, `if doc.Template != nil { if err := doc.Template.Validate(); err != nil { return Inventory{}, fmt.Errorf("constitutionimpact: decoding inventory: %w", err) } }`, carry it into `inventory`, and `EncodeInventory` sets `Template: inventory.Template` on the doc. Check the import graph first (`go list -deps ./internal/policyartifact | grep constitutionimpact` must be empty).

Run: the two tests → PASS; `go build ./... && go test -count=1 ./internal/policyartifact/ ./internal/governanceprincipal/ ./internal/constitutionimpact/ ./internal/policyauthority/ ./internal/humanartifact/` → PASS (existing golden digests are unchanged because no fixture carries a profile template).

Commit: `Move TemplateRecord below policyartifact and add the profile and inventory template seams`

- [ ] **Step 2: The five templates and the embed pattern**

`internal/designscaffold/render.go:20`: `//go:embed templates/*` (keep the comment; add "widened from *.md for the consumers-inventory JSON template (ac-10)"). Add `TestCanonical_EveryTemplateFileIsEmbedded` in `render_test.go` listing the seven existing plus five new names and asserting `Canonical(name)` succeeds for each.

Write the templates (byte-exact; every value position that renders caller data uses `safe` or `printf "%q"` exactly as `policy.md` does):

`templates/policy-constitution.md`:
```
---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
selected_profile: {{safe .ProfileID}}
environments: [local]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, policy-disposition-approval]
  evidence_sources: []
  escalation_metrics: []
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters: []
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter constitution written by `verdi policy adopt --starter`. It selects
the {{safe .ProfileID}} governance profile and registers the minimal
catalog that profile needs: three roles, the accept and
policy-disposition-approval transitions, one local environment, no
constraint subjects, and no harness adapters. Add subjects, environments,
and adapters through the project's own review process; acceptance of this
file is the owner's merge to the default branch.
```

`templates/governance-profile-solo.md`:
```
---
schema: verdi.governance-profile/v1
id: {{safe .ProfileID}}
class: solo
applicable_transitions: [accept, policy-disposition-approval]
identity_trust_sources:
  - {id: local, kind: local-operator}
role_mappings:
  - {role: author, trust_source: local, subjects: [{{printf "%q" .Subject}}]}
  - {role: reviewer, trust_source: local, subjects: [{{printf "%q" .Subject}}]}
  - {role: policy-owner, trust_source: local, subjects: [{{printf "%q" .Subject}}]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter solo profile: one authenticated principal fills every role, with
the collapsed separation of duties disclosed by the kernel. The bound
subject is this checkout's own configured Git identity — a bare
self-assertion the resolver reports as local-operator-asserted, never
independently verified. A policy disposition needs the policy owner's
approval; acceptance itself is the owner's merge to the default branch.
```

`templates/governance-profile-team.md`:
```
---
schema: verdi.governance-profile/v1
id: {{safe .ProfileID}}
class: team
applicable_transitions: [accept, policy-disposition-approval]
identity_trust_sources:
  - {id: forge, kind: forge}
role_mappings: []
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [accept], roles: [reviewer], minimum: 1}
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules:
  - {transitions: [accept], left_role: author, right_role: reviewer, relation: different-principal}
  - {transitions: [policy-disposition-approval], left_role: author, right_role: policy-owner, relation: different-principal}
evidence_source_restrictions: []
escalation_thresholds: []
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter team profile: acceptance needs one reviewer who is not the author,
and a policy disposition needs a policy owner who is not its author, all
authenticated through the forge. It maps no subjects yet — this file names no one — so every
role resolves unproven until the team adds its own role_mappings through
the project's review process. Nothing here is a fabricated identity.
```

`templates/policy-starter.md`:
```
---
schema: verdi.policy/v1
id: policy/{{.Name}}
kind: policy
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads:
  design_assistance: {mode: {{safe .DesignAssistanceMode}}, layout: false}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter policy: one real rule and nothing else. The design_assistance
payload sets mode {{safe .DesignAssistanceMode}} — what delegated agents
may do on a design branch. No constraint claims, no instruction lines, and
no other payloads are declared; add them through the project's own review
process. Acceptance of this policy is the owner's merge to the default
branch.
```

`templates/constitution-consumers.json` (one line, newline-terminated — canonical JSON, keys sorted):
```
{"consumers":[],"schema":"verdi.constitution-consumer-inventory/v1","template":{"digest":{{printf "%q" .TemplateDigest}},"identity":{{printf "%q" .TemplateIdentity}}}}
```

Commit: `Embed the five starter scaffold templates`

- [ ] **Step 3: Failing renderer tests**

`internal/humanartifact/starter_test.go` (package `humanartifact`):

```go
func starterScaffold(t *testing.T, name string) Scaffold {
	t.Helper()
	s, err := ResolveScaffold(t.TempDir(), name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func starterCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{Roles: []string{"author", "reviewer", "policy-owner"}, Transitions: []string{"accept", "policy-disposition-approval"}}
}

func TestRenderConstitution_RoundTrip(t *testing.T) {
	s := starterScaffold(t, ConstitutionTemplate)
	out, err := RenderConstitution(s, ConstitutionScaffoldData{Title: "Starter constitution", Owners: []string{"local-operator"}, ProfileID: "starter-solo", TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
	if err != nil {
		t.Fatal(err)
	}
	c, err := policyartifact.DecodeConstitution([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if c.SelectedProfile != "starter-solo" || c.Template == nil || c.Template.Digest != s.Digest || len(c.Adapters) != 0 || len(c.Environments) != 1 {
		t.Fatalf("decoded = %+v", c)
	}
	// Anti-synthesis: a template that changes the selected profile fails by name.
	bad := s
	bad.Template = bytes.Replace(s.Template, []byte("selected_profile: {{safe .ProfileID}}"), []byte("selected_profile: other"), 1)
	if _, err := RenderConstitution(bad, ConstitutionScaffoldData{Title: "t", Owners: []string{"o"}, ProfileID: "starter-solo", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil || !strings.Contains(err.Error(), "selected_profile") {
		t.Fatalf("synthesized profile accepted: %v", err)
	}
	// Owners must be kebab handles (kernel rule), reported by the decoder.
	if _, err := RenderConstitution(s, ConstitutionScaffoldData{Title: "t", Owners: []string{"Not Kebab"}, ProfileID: "starter-solo", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil {
		t.Fatal("ungrammatical owner accepted")
	}
}

func TestRenderProfile_SoloBindsTheSubjectAndTeamMapsNoOne(t *testing.T) {
	solo := starterScaffold(t, ProfileSoloTemplate)
	out, err := RenderProfile(solo, ProfileScaffoldData{ProfileID: "starter-solo", Class: governanceprincipal.ClassSolo, Subject: "dev@example.invalid", TemplateIdentity: solo.Identity, TemplateDigest: solo.Digest}, starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := policyartifact.DecodeStoredProfile([]byte(out), starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if sp.ID != "starter-solo" || sp.Profile.Class != governanceprincipal.ClassSolo || len(sp.Profile.RoleMappings) != 3 || sp.Profile.Template == nil || sp.Profile.Template.Identity != solo.Identity {
		t.Fatalf("solo = %+v", sp.Profile)
	}
	for _, m := range sp.Profile.RoleMappings {
		if len(m.Subjects) != 1 || m.Subjects[0] != "dev@example.invalid" {
			t.Fatalf("mapping %+v binds a subject the caller did not supply", m)
		}
	}
	// A subject containing a quote or newline is rendered safely (printf %q), never a second key.
	if _, err := RenderProfile(solo, ProfileScaffoldData{ProfileID: "starter-solo", Class: governanceprincipal.ClassSolo, Subject: "a\"b\nrole_mappings: []", TemplateIdentity: solo.Identity, TemplateDigest: solo.Digest}, starterCatalog()); err != nil {
		t.Fatalf("quoted subject: %v", err)
	}
	// Anti-synthesis: a template that adds a subject the caller did not supply fails by name.
	bad := solo
	bad.Template = bytes.Replace(solo.Template, []byte(`subjects: [{{printf "%q" .Subject}}]}`), []byte(`subjects: [{{printf "%q" .Subject}}, alice]}`), 1)
	if _, err := RenderProfile(bad, ProfileScaffoldData{ProfileID: "starter-solo", Class: governanceprincipal.ClassSolo, Subject: "dev@example.invalid", TemplateIdentity: solo.Identity, TemplateDigest: solo.Digest}, starterCatalog()); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("synthesized subject accepted: %v", err)
	}

	team := starterScaffold(t, ProfileTeamTemplate)
	out, err = RenderProfile(team, ProfileScaffoldData{ProfileID: "starter-team", Class: governanceprincipal.ClassTeam, TemplateIdentity: team.Identity, TemplateDigest: team.Digest}, starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	tp, err := policyartifact.DecodeStoredProfile([]byte(out), starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if tp.Profile.Class != governanceprincipal.ClassTeam || len(tp.Profile.RoleMappings) != 0 || len(tp.Profile.DistinctnessRules) != 2 || len(tp.Profile.RequiredApprovers) != 2 {
		t.Fatalf("team = %+v", tp.Profile)
	}
	// Class mismatch between data and template fails by name.
	if _, err := RenderProfile(team, ProfileScaffoldData{ProfileID: "starter-team", Class: governanceprincipal.ClassSolo, TemplateIdentity: team.Identity, TemplateDigest: team.Digest}, starterCatalog()); err == nil || !strings.Contains(err.Error(), "class") {
		t.Fatalf("class mismatch accepted: %v", err)
	}
}

func TestRenderStarterPolicy_ExactlyOneRealRule(t *testing.T) {
	s := starterScaffold(t, StarterPolicyTemplate)
	for _, mode := range []string{"draft-write", "proposal-only"} {
		out, err := RenderStarterPolicy(s, StarterPolicyScaffoldData{Name: "starter", Title: "Starter policy", Owners: []string{"local-operator"}, DesignAssistanceMode: mode, TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
		if err != nil {
			t.Fatal(err)
		}
		p, err := policyartifact.DecodePolicy([]byte(out))
		if err != nil {
			t.Fatal(err)
		}
		da, ok := p.Payloads[policyartifact.DesignAssistancePayloadKind].(*policyartifact.DesignAssistancePayload)
		if !ok || da.Mode != mode || da.Layout || len(p.Claims) != 0 || len(p.Instructions) != 0 || len(p.Payloads) != 1 {
			t.Fatalf("mode %s: decoded = %+v", mode, p)
		}
	}
	// The existing placeholder scaffold is refused here: it renders no rule.
	placeholder := starterScaffold(t, "policy.md")
	if _, err := RenderStarterPolicy(placeholder, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "draft-write", TemplateIdentity: placeholder.Identity, TemplateDigest: placeholder.Digest}); err == nil || !strings.Contains(err.Error(), "design_assistance") {
		t.Fatalf("placeholder accepted: %v", err)
	}
	// An unknown mode is refused by the payload's own validator, not silently written.
	if _, err := RenderStarterPolicy(s, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "yes", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil {
		t.Fatal("bad mode accepted")
	}
	// Anti-synthesis: a template that adds a claim fails by name.
	bad := s
	bad.Template = bytes.Replace(s.Template, []byte("claims: []"), []byte("claims:\n  - {id: c, family: action, operator: required-values, subject: x, values: [y], scope: {phases: [], environments: [], paths: [], refs: []}, overridable: false}"), 1)
	if _, err := RenderStarterPolicy(bad, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "draft-write", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil || !strings.Contains(err.Error(), "claims") {
		t.Fatalf("synthesized claim accepted: %v", err)
	}
}

func TestRenderConsumersInventory_EmptyCanonicalWithRecord(t *testing.T) {
	s := starterScaffold(t, InventoryTemplate)
	out, err := RenderConsumersInventory(s, InventoryScaffoldData{TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := constitutionimpact.DecodeInventory(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Consumers) != 0 || inv.Template == nil || inv.Template.Digest != s.Digest {
		t.Fatalf("inventory = %+v", inv)
	}
	// A non-canonical override (pretty-printed) fails closed through DecodeInventory's canonical-bytes check.
	bad := s
	bad.Template = []byte("{\n  \"consumers\": [],\n  \"schema\": \"verdi.constitution-consumer-inventory/v1\"\n}\n")
	if _, err := RenderConsumersInventory(bad, InventoryScaffoldData{TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil {
		t.Fatal("non-canonical override accepted")
	}
}

func TestStarterTemplates_StoreOverrideChangesIdentity(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	canon, _ := designscaffold.Canonical(StarterPolicyTemplate)
	if err := os.WriteFile(filepath.Join(dir, StarterPolicyTemplate), bytes.Replace(canon, []byte("Starter policy: one real rule"), []byte("Our policy: one real rule"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ResolveScaffold(root, StarterPolicyTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if s.Identity != "store:.verdi/templates/"+StarterPolicyTemplate {
		t.Fatalf("identity = %s", s.Identity)
	}
	out, err := RenderStarterPolicy(s, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "draft-write", TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
	if err != nil || !strings.Contains(out, "Our policy") || !strings.Contains(out, s.Identity) {
		t.Fatalf("override render: %v\n%s", err, out)
	}
}
```

Run: `go test ./internal/humanartifact/ -run 'TestRender(Constitution|Profile|StarterPolicy|ConsumersInventory)|TestStarterTemplates' -count=1` — Expected: FAIL (undefined symbols).

- [ ] **Step 4: Implement the renderers and kernel rows**

`internal/humanartifact/kernel.go` `kernelFieldTable`: add
```go
	policyartifact.KindConstitution:   {"schema", "id", "kind", "title", "owners", "template", "selected_profile", "environments", "catalog", "subjects", "adapters"},
	policyartifact.KindProfileStorage: {"schema", "id", "class", "applicable_transitions", "identity_trust_sources", "role_mappings", "ownership_sources", "signature_requirements", "required_approvers", "distinctness_rules", "evidence_source_restrictions", "escalation_thresholds", "template"},
```
(extend the table's doc comment: "constitution and governance-profile rows added by spec/spec-documents ac-10 (SI-204): the starter scaffold creates both kinds, so both resolve through this one renderer"). Update `kernel_test.go`'s expectations if it enumerates kinds.

`internal/humanartifact/starter.go`: the constants and types above, plus:

```go
// RenderConstitution renders scaffold against data through designscaffold.RenderValue,
// strict-decodes the result (policyartifact.DecodeConstitution), and verifies the kernel
// round trip: id (fixed by the decoder), title, owners, template record, and — this
// scaffold's own kernel field — selected_profile must equal what data supplied.
func RenderConstitution(scaffold Scaffold, data ConstitutionScaffoldData) (string, error) {
	content, err := designscaffold.RenderValue(scaffold.Template, data)
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendering constitution scaffold: %w", err)
	}
	c, err := policyartifact.DecodeConstitution([]byte(content))
	if err != nil {
		return "", fmt.Errorf("humanartifact: rendered constitution scaffold failed strict decode: %w", err)
	}
	wantID := policyartifact.KindConstitution + "/" + policyartifact.ConstitutionName
	if err := verifyKernelRoundTrip(policyartifact.KindConstitution, wantID, data.Title, data.Owners, c.ID, c.Title, c.Owners, c.Template, scaffold); err != nil {
		return "", err
	}
	if c.SelectedProfile != data.ProfileID {
		return "", fmt.Errorf("humanartifact: rendered constitution kernel mismatch: selected_profile = %q, want %q", c.SelectedProfile, data.ProfileID)
	}
	return content, nil
}
```

`RenderProfile`: render → `policyartifact.DecodeStoredProfile([]byte(content), catalog)` → checks, each naming its field: `sp.ID == data.ProfileID`; `sp.Profile.Class == data.Class` ("class = %q, want %q"); `sp.Profile.Template` non-nil with identity/digest equal to scaffold's ("template.identity"/"template.digest"); every `RoleMapping.Subjects` equals exactly `[data.Subject]` when `data.Subject != ""` and `len(RoleMappings) == 0` when it is empty ("role_mappings[%d].subjects = %v, want exactly the caller's subject %q — a template must not synthesize identities" / "role_mappings = %d entries, want none: no subject was supplied"). Reuse `verifyKernelRoundTrip`'s template-record comparison by calling it with the profile's id as both wantID and gotID, `""` titles, and nil owner sets is NOT acceptable (it would pass vacuous fields) — write the three explicit comparisons instead.

`RenderStarterPolicy`: render → `DecodePolicy` → `verifyKernelRoundTrip(KindPolicy, "policy/"+data.Name, …)` → `scopesEqual(p.Scope, universalScope)`; `len(p.Claims) == 0` ("claims = %v, want none: the starter renders one real rule, never claims"); `len(p.Instructions) == 0`; `len(p.Payloads) == 1` and the one entry is `DesignAssistancePayloadKind` typed as `*policyartifact.DesignAssistancePayload` with `Mode == data.DesignAssistanceMode` and `!Layout` ("payloads = %v, want exactly one design_assistance payload with mode %q"). `DecodePolicy` already runs the payload's `Validate`, so an unknown mode fails there.

`RenderConsumersInventory`: render → `constitutionimpact.DecodeInventory([]byte(content))` (canonical-bytes check included) → `len(inv.Consumers) == 0` ("consumers = %d, want none: the starter registers no consumers") and `inv.Template` equal to scaffold identity/digest → return `[]byte(content)`.

Run: Step 3 tests → PASS. `go test -race -count=1 ./internal/humanartifact/ ./internal/designscaffold/ ./internal/policyartifact/ ./internal/governanceprincipal/ ./internal/constitutionimpact/ && go vet ./internal/... && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness`.

Commit: `Render the starter constitution, profiles, policy, and inventory through the shared scaffold seam`

---

### Task 2: `internal/policyadopt` and `verdi policy adopt --starter` (Tier 3, serialized registry)

**Files:**
- Create: `internal/policyadopt/doc.go`, `plan.go`, `write.go`, `plan_test.go`, `cmd/verdi/policy.go`, `cmd/verdi/policy_test.go`
- Modify: `cmd/verdi/design.go:693-770` (R-W4-3 generalization; `designsupersede.go` caller unchanged in behavior), `cmd/verdi/dispatch.go` (`"policy": 26`, usage const, one arm after the phase lookup), `cmd/verdi/help.go` (`topLevelUsage` row `policy           adopt a starter constitution on a policy/adopt branch`; `verbUsage["policy"] = policyUsage`), `internal/specalign/verbs_test.go:136-141` (+ header comment block), `internal/showcasealign/coverage_test.go` (`"cli:policy"` row + comment), `internal/showcasealign/cli_showcase_test.go` (`TestCLIShowcasePolicyAdopt`), `README.md:293` table (+ one row), `docs/policy-setup-validation.md:53-69` (the "What initial setup requires" section names the verb; the four bullets stay as what the starter writes; the "do not fabricate an empty inventory" sentence becomes "the starter writes an EMPTY inventory and says so; register real consumers before impact review").

**Interfaces:**
- Consumes: Task 1's five `humanartifact` renderers and constants; `policyauthority.LoadFromSource(fs.FS)`, `policyauthority.Resolve`; `policyartifact.Constitution.Catalog` → `governanceprincipal.Catalog`; `draftmutation.ResolvePolicyGrant` is NOT called (it needs identity/checkout plumbing) — the grant proof re-implements the same three-line payload lookup over `EffectivePolicy.Policies` (`draftmutation/policy.go:289-310`) and is pinned equal by a test that runs the real `ResolvePolicyGrant` after the write (cmd/verdi test); `atomicfile.Write`; `gitx.AddPaths`, `CreateCommit`, `CheckoutNewBranchFrom`, `CurrentBranch`, `RevParse`; `readLocalGitIdentity` (`cmd/verdi/actorlocal.go:126`).
- Produces:

```go
package policyadopt

type Input struct {
	Profile governanceprincipal.Class // ClassSolo | ClassTeam
	Owner   string                    // kebab-case owner handle
	Subject string                    // solo: the checkout's local git identity; team: ""
}

type File struct {
	RelPath  string // store-relative, forward slashes
	Content  []byte
	Template policyartifact.TemplateRecord
}

type Plan struct {
	Files                []File // constitution, profile, policy, inventory — in that order
	ProfileID            string
	DesignAssistanceMode string
	EffectiveDigest      string // policyauthority.EffectivePolicy.Digest() over the composed tree
}

var ErrAlreadyAdopted = errors.New("policyadopt: this checkout already carries .verdi/policy or .verdi/constitution/consumers.json")

func Compose(root string, in Input) (*Plan, error)     // resolve four scaffolds under root, render, prove in memory (R-W4-2); ErrAlreadyAdopted if either path exists
func Write(root string, p *Plan) ([]string, error)     // atomicfile.Write each file (mkdir -p its dir); returns the written store-relative paths
```

- [ ] **Step 1: Failing package tests**

`internal/policyadopt/plan_test.go`:

```go
func TestCompose_SoloProvesTheTreeBeforeAnyWrite(t *testing.T) {
	root := t.TempDir() // no store at all: Compose reads only .verdi/templates overrides
	p, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "dev@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".verdi/policy/constitution.md", ".verdi/policy/profiles/starter-solo.md", ".verdi/policy/policies/starter.md", ".verdi/constitution/consumers.json"}
	for i, f := range p.Files {
		if f.RelPath != want[i] || !strings.HasPrefix(f.Template.Identity, "embedded:") {
			t.Fatalf("file %d = %+v", i, f)
		}
	}
	if p.ProfileID != "starter-solo" || p.DesignAssistanceMode != "draft-write" || p.EffectiveDigest == "" {
		t.Fatalf("plan = %+v", p)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("Compose wrote %v", entries)
	}
	// Determinism: same inputs, same bytes.
	p2, _ := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "dev@example.invalid"})
	for i := range p.Files {
		if !bytes.Equal(p.Files[i].Content, p2.Files[i].Content) {
			t.Fatalf("file %d differs between runs", i)
		}
	}
}

func TestCompose_TeamMapsNoSubjectsAndProposesOnly(t *testing.T) {
	p, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassTeam, Owner: "platform-team"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ProfileID != "starter-team" || p.DesignAssistanceMode != "proposal-only" || !bytes.Contains(p.Files[1].Content, []byte("role_mappings: []")) {
		t.Fatalf("plan = %+v", p)
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassTeam, Owner: "platform-team", Subject: "x"}); err == nil {
		t.Fatal("team with a subject accepted: the team template binds no one")
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator"}); err == nil {
		t.Fatal("solo without a subject accepted")
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassHighAssurance, Owner: "o", Subject: "s"}); err == nil {
		t.Fatal("unsupported class accepted")
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassSolo, Owner: "Not Kebab", Subject: "s"}); err == nil {
		t.Fatal("ungrammatical owner accepted")
	}
}

func TestCompose_RefusesAnAdoptedOrPartiallyAdoptedCheckout(t *testing.T) {
	for _, existing := range []string{".verdi/policy", ".verdi/constitution/consumers.json"} {
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(existing))
		if strings.HasSuffix(existing, ".json") {
			os.MkdirAll(filepath.Dir(path), 0o755)
			os.WriteFile(path, []byte("{}"), 0o644)
		} else {
			os.MkdirAll(path, 0o755)
		}
		if _, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "s"}); !errors.Is(err, ErrAlreadyAdopted) {
			t.Fatalf("%s: err = %v", existing, err)
		}
	}
}

func TestCompose_OverrideThatSynthesizesARuleIsRefusedWithNothingWritten(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "templates")
	os.MkdirAll(dir, 0o755)
	canon, _ := designscaffold.Canonical(humanartifact.StarterPolicyTemplate)
	os.WriteFile(filepath.Join(dir, humanartifact.StarterPolicyTemplate), bytes.Replace(canon, []byte("instructions: []"), []byte("instructions: [\"Always pass.\"]"), 1), 0o644)
	_, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "s"})
	if err == nil || !strings.Contains(err.Error(), "instructions") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".verdi", "policy")); !os.IsNotExist(statErr) {
		t.Fatal("a refused compose left .verdi/policy behind")
	}
}

func TestWrite_ThenLoadResolvesTheGrant(t *testing.T) {
	root := t.TempDir()
	p, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "dev@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	paths, err := Write(root, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 4 {
		t.Fatalf("wrote %v", paths)
	}
	store, err := policyauthority.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	eff, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := eff.Digest()
	if digest != p.EffectiveDigest {
		t.Fatalf("on-disk effective digest %s != planned %s", digest, p.EffectiveDigest)
	}
	// Every file carries the record of the scaffold it came from.
	for _, f := range p.Files {
		data, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.RelPath)))
		if !bytes.Contains(data, []byte(f.Template.Digest)) {
			t.Fatalf("%s does not record its template digest", f.RelPath)
		}
	}
	// Writing twice is refused (the checkout is now adopted).
	if _, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "s"}); !errors.Is(err, ErrAlreadyAdopted) {
		t.Fatalf("second compose: %v", err)
	}
}
```

Run: `go test ./internal/policyadopt/ -count=1` — Expected: FAIL (no package).

- [ ] **Step 2: Implement `internal/policyadopt`**

`doc.go`: package doc — "policyadopt composes the starter constitution store `verdi policy adopt --starter` writes (spec/spec-documents ac-10, dc-6; SI-204): four artifacts rendered through the shared scaffold seam, proven as a whole in memory before any byte reaches the store (R-W4-2), then written. It cuts no branch and makes no commit — cmd/verdi owns git."

`plan.go`:

```go
const (
	ProfileSoloID   = "starter-solo"
	ProfileTeamID   = "starter-team"
	PolicyName      = "starter"
	modeSolo        = "draft-write"    // vocab:identity — design_assistance mode enum value (ASD AC-3), not the lifecycle state word
	modeTeam        = "proposal-only"
	constitutionRel = ".verdi/policy/constitution.md"
	policyRel       = ".verdi/policy/policies/" + PolicyName + ".md"
	inventoryRel    = constitutionimpact.InventoryPath
)

func profileRel(id string) string { return ".verdi/policy/profiles/" + id + ".md" }

func Compose(root string, in Input) (*Plan, error) {
	if err := refuseAdopted(root); err != nil {
		return nil, err
	}
	var profileID, profileTemplate, mode string
	switch in.Profile {
	case governanceprincipal.ClassSolo:
		if in.Subject == "" {
			return nil, fmt.Errorf("policyadopt: the solo profile binds the checkout's own git identity; none was supplied")
		}
		profileID, profileTemplate, mode = ProfileSoloID, humanartifact.ProfileSoloTemplate, modeSolo
	case governanceprincipal.ClassTeam:
		if in.Subject != "" {
			return nil, fmt.Errorf("policyadopt: the team profile maps no subjects; refusing the supplied subject %q", in.Subject)
		}
		profileID, profileTemplate, mode = ProfileTeamID, humanartifact.ProfileTeamTemplate, modeTeam
	default:
		return nil, fmt.Errorf("policyadopt: profile class %q is not a starter profile (solo or team)", in.Profile)
	}
	// Resolve all four scaffolds first so a missing override is reported before any render.
	cs, err := humanartifact.ResolveScaffold(root, humanartifact.ConstitutionTemplate)
	// … ps (profileTemplate), pol (StarterPolicyTemplate), inv (InventoryTemplate), each `if err != nil { return nil, err }`
	constitution, err := humanartifact.RenderConstitution(cs, humanartifact.ConstitutionScaffoldData{Title: "Starter constitution", Owners: []string{in.Owner}, ProfileID: profileID, TemplateIdentity: cs.Identity, TemplateDigest: cs.Digest})
	if err != nil { return nil, err }
	decodedC, err := policyartifact.DecodeConstitution([]byte(constitution)) // for the catalog RenderProfile validates against
	if err != nil { return nil, err }
	catalog := governanceprincipal.Catalog{Roles: decodedC.Catalog.Roles, Transitions: decodedC.Catalog.Transitions, EvidenceSources: decodedC.Catalog.EvidenceSources, EscalationMetrics: decodedC.Catalog.EscalationMetrics}
	profile, err := humanartifact.RenderProfile(ps, humanartifact.ProfileScaffoldData{ProfileID: profileID, Class: in.Profile, Subject: in.Subject, TemplateIdentity: ps.Identity, TemplateDigest: ps.Digest}, catalog)
	policy, err := humanartifact.RenderStarterPolicy(pol, humanartifact.StarterPolicyScaffoldData{Name: PolicyName, Title: "Starter policy", Owners: []string{in.Owner}, DesignAssistanceMode: mode, TemplateIdentity: pol.Identity, TemplateDigest: pol.Digest})
	inventory, err := humanartifact.RenderConsumersInventory(inv, humanartifact.InventoryScaffoldData{TemplateIdentity: inv.Identity, TemplateDigest: inv.Digest})
	files := []File{
		{RelPath: constitutionRel, Content: []byte(constitution), Template: policyartifact.TemplateRecord{Identity: cs.Identity, Digest: cs.Digest}},
		{RelPath: profileRel(profileID), Content: []byte(profile), Template: policyartifact.TemplateRecord{Identity: ps.Identity, Digest: ps.Digest}},
		{RelPath: policyRel, Content: []byte(policy), Template: policyartifact.TemplateRecord{Identity: pol.Identity, Digest: pol.Digest}},
		{RelPath: inventoryRel, Content: inventory, Template: policyartifact.TemplateRecord{Identity: inv.Identity, Digest: inv.Digest}},
	}
	digest, err := prove(files, mode)
	if err != nil { return nil, err }
	return &Plan{Files: files, ProfileID: profileID, DesignAssistanceMode: mode, EffectiveDigest: digest}, nil
}

// prove loads the composed tree exactly as the store would (policyauthority.LoadFromSource over
// an in-memory FS), resolves it, and requires exactly one valid design_assistance payload with
// the planned mode — the same lookup draftmutation.ResolvePolicyGrant performs at write time.
func prove(files []File, mode string) (string, error) {
	tree := fstest.MapFS{}
	for _, f := range files {
		tree[f.RelPath] = &fstest.MapFile{Data: f.Content, Mode: 0o644}
	}
	store, err := policyauthority.LoadFromSource(tree)
	if err != nil { return "", fmt.Errorf("policyadopt: the composed starter tree does not load: %w", err) }
	eff, err := policyauthority.Resolve(store)
	if err != nil { return "", fmt.Errorf("policyadopt: the composed starter tree does not resolve: %w", err) }
	var found *policyartifact.DesignAssistancePayload
	for _, entry := range eff.Policies {
		raw, ok := entry.Payloads[policyartifact.DesignAssistancePayloadKind]
		if !ok { continue }
		if found != nil { return "", fmt.Errorf("policyadopt: the composed tree carries two design_assistance payloads") }
		typed, ok := raw.(*policyartifact.DesignAssistancePayload)
		if !ok { return "", fmt.Errorf("policyadopt: design_assistance payload is not the registered typed payload") }
		found = typed
	}
	if found == nil || found.Mode != mode {
		return "", fmt.Errorf("policyadopt: the composed tree grants design_assistance mode %v, want %q", found, mode)
	}
	if _, err := constitutionimpact.DecodeInventory(tree[inventoryRel].Data); err != nil { return "", err }
	return eff.Digest()
}
```

Read `policyauthority.EffectivePolicy` (`resolve.go`) for the exact `Policies` entry type and `Payloads` field name before writing `prove`; the shape above mirrors `draftmutation/policy.go:289-303`. `refuseAdopted`: `os.Stat` on `.verdi/policy` and on `inventoryRel`; exists → `ErrAlreadyAdopted` (wrapped with the path); any error other than not-exist → operational.

`write.go`: `Write` — for each file `os.MkdirAll(filepath.Dir(abs), 0o755)` then `atomicfile.Write(abs, f.Content, 0o644)`; on any failure return the error naming the path (partial writes are on a fresh branch the caller reports; do not attempt rollback — disclose).

Run: Step 1 tests → PASS; `go test -race -count=1 ./internal/policyadopt/`.

Commit: `Compose and prove the starter constitution store before writing it`

- [ ] **Step 3: Failing verb tests (built binary)**

`cmd/verdi/policy_test.go` (helpers: `buildVerdiBinary`, `runVerdi(t, bin, dir, args...) (int, string, string)`, `fixturegit.Build`; set `CI_DEFAULT_BRANCH=main` in the env as the other verb tests do — read `harness_test.go`/`designsupersede_test.go` for `runVerdiBinary`'s env form):

```go
func adoptFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"}, Message: "init store"}})
}

func TestPolicyAdopt_UsageAndFlagShape(t *testing.T) {
	bin := buildVerdiBinary(t)
	dir := t.TempDir() // no store, no git: every case below must fail before touching either
	for _, args := range [][]string{{"policy"}, {"policy", "adopt"}, {"policy", "frobnicate"}, {"policy", "adopt", "--starter", "--profile", "solo", "--profile", "team"}, {"policy", "adopt", "--starter", "--profile", "high-assurance"}, {"policy", "adopt", "--starter", "--owner"}, {"policy", "adopt", "--starter", "--profile", "team"}} {
		code, _, stderr := runVerdi(t, bin, dir, args...)
		if code != 2 || !strings.Contains(stderr, "usage: verdi policy adopt --starter") {
			t.Fatalf("%v: code %d stderr %q", args, code, stderr)
		}
	}
}

func TestPolicyAdopt_SoloWritesFourPathsOnPolicyAdopt(t *testing.T) {
	bin := buildVerdiBinary(t)
	repo := adoptFixture(t)
	code, stdout, stderr := runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 0 {
		t.Fatalf("code %d\n%s\n%s", code, stdout, stderr)
	}
	for _, want := range []string{"policy adopt: base ", "policy adopt: switched checkout from main to policy/adopt", "policy adopt: wrote .verdi/policy/constitution.md (embedded:policy-constitution.md sha256:", "policy adopt: wrote .verdi/policy/profiles/starter-solo.md", "policy adopt: wrote .verdi/policy/policies/starter.md", "policy adopt: wrote .verdi/constitution/consumers.json", "design_assistance mode draft-write", "registers no consumers", "policy adopt: committed ", "acceptance is the owner's merge"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
	// Exactly the four paths, on policy/adopt, and main untouched.
	branch := gitOutput(t, repo.Dir, "rev-parse", "--abbrev-ref", "HEAD")
	if branch != "policy/adopt" {
		t.Fatalf("branch = %s", branch)
	}
	names := gitOutput(t, repo.Dir, "show", "--name-only", "--format=", "HEAD")
	if names != ".verdi/constitution/consumers.json\n.verdi/policy/constitution.md\n.verdi/policy/policies/starter.md\n.verdi/policy/profiles/starter-solo.md" {
		t.Fatalf("committed paths:\n%s", names)
	}
	if gitOutput(t, repo.Dir, "rev-parse", "main") != repo.Head {
		t.Fatal("main moved")
	}
	if gitOutput(t, repo.Dir, "status", "--porcelain") != "" {
		t.Fatal("tree dirty after adopt")
	}
	// The written store is the one the real grant resolver accepts, with the fixture's git identity bound.
	store, err := policyauthority.Load(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sp := store.Profiles["starter-solo"]
	if sp == nil || sp.Profile.RoleMappings[0].Subjects[0] != "fixture@verdi.invalid" {
		t.Fatalf("profile = %+v", sp)
	}
	// model check runs its policy-scaffold round trips now that .verdi/policy exists.
	if code, _, stderr := runVerdi(t, bin, repo.Dir, "model", "check"); code != 0 {
		t.Fatalf("model check after adopt: %d %s", code, stderr)
	}
	// inspect: proposed adopted, accepted not.
	code, stdout, _ = runVerdiStdin(t, bin, repo.Dir, `{"schema":"verdi.constitution-inspect-request/v1"}`, "context", "constitution", "inspect", "--request", "-")
	if code != 0 || !strings.Contains(stdout, `"proposed":{"adopted":true`) && !strings.Contains(stdout, `"adopted":true`) {
		t.Fatalf("inspect: %d %s", code, stdout)
	}
	// A second adopt is a verdict, not an operational fault.
	code, _, stderr = runVerdi(t, bin, repo.Dir, "policy", "adopt", "--starter")
	if code != 1 || !strings.Contains(stderr, "already carries") {
		t.Fatalf("second adopt: %d %s", code, stderr)
	}
}
```

(`gitOutput` and `runVerdiStdin`: write them in this file if the package has no equivalent — `exec.Command("git", "-C", dir, args...)` trimmed; `runVerdiStdin` sets `cmd.Stdin`. Read `context_constitution_test.go` for the inspect result's exact JSON before pinning the substring.)

Add:
- `TestPolicyAdopt_TeamRequiresOwnerAndProposesOnly`: `--profile team --owner platform-team` → exit 0; profile has no role mappings; policy mode `proposal-only`; stdout says "maps no subjects yet".
- `TestPolicyAdopt_OverrideRecordedAndSynthesisRefusedBeforeBranching`: commit `.verdi/templates/policy-starter.md` (canonical bytes with the rationale's first words changed) on main → adopt → the policy's `template.identity` is `store:.verdi/templates/policy-starter.md`. Then in a fresh fixture commit an override that adds an instruction → adopt exits 2, stderr names `instructions`, `git branch --list policy/adopt` is empty, `git status --porcelain` empty, still on main.
- `TestPolicyAdopt_NoLocalIdentityRefusesSolo`: fixture repo with `git config --local --unset user.email` and `user.name` → exit 2, stderr "configure user.email"; nothing written; team still works there.
- `TestPolicyAdopt_ExistingBranchRefused`: `git branch policy/adopt` first → exit 2 naming the branch; tree untouched.

Run: `go test ./cmd/verdi/ -run TestPolicyAdopt -count=1` — Expected: FAIL (unknown verb).

- [ ] **Step 4: Implement the verb and the registry**

`cmd/verdi/design.go`: rename `resolveDesignStartBase` → `resolveBranchBase(ctx, root, verb string, stdout, stderr)` with every `"design start:"`/`"design start: "` literal replaced by `verb+":"`/`verb+": "`, and keep `func resolveDesignStartBase(ctx, root, stdout, stderr) (string, bool) { return resolveBranchBase(ctx, root, "design start", stdout, stderr) }` (both existing callers untouched). Same for `checkoutNewDesignBranch` → `checkoutNewBranchDisclosed(ctx, root, verb, branch, baseRef, stdout, stderr)` with the thin wrapper. Run `go test ./cmd/verdi/ -run 'TestDesignStart|TestDesignSupersede' -count=1` to prove the disclosure lines are byte-identical.

`cmd/verdi/policy.go`:

```go
// verdi policy adopt --starter [--profile solo|team] [--owner <handle>] (spec/spec-documents
// ac-10, dc-6; SI-204): renders a starter constitution, one profile, one policy, and the
// consumers inventory through internal/policyadopt, cuts policy/adopt from the resolved
// default branch (design start's own dc-7 chain, R-W4-3), writes exactly those four paths,
// stages exactly them, and commits. Kept in its own file per the harness.go convention.
package main

// vocab:identity — CLI usage/flag grammar (identity)
const policyUsage = "usage: verdi policy adopt --starter [--profile solo|team] [--owner <kebab-handle>]"

func cmdPolicy(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "adopt" {
		if len(args) > 0 { fmt.Fprintf(stderr, "policy: unknown subcommand %q\n", args[0]) }
		fmt.Fprintln(stderr, policyUsage)
		return 2
	}
	opts, err := parsePolicyAdoptFlags(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "policy adopt: %v\n%s\n", err, policyUsage)
		return 2
	}
	root, err := store.FindRoot(".")
	if err != nil { fmt.Fprintln(stderr, "policy adopt:", err); return 2 }
	return runPolicyAdopt(context.Background(), root, opts, stdout, stderr)
}
```

`parsePolicyAdoptFlags`: hand-rolled loop (precedent `disposition_record.go:171`): `--starter` (required, at most once), `--profile <v>` / `--profile=<v>` (at most once; `solo` default; anything else → error naming the legal values), `--owner <v>` (at most once; kebab check by the same `kebabRe` grammar — copy the regexp literal from `policyartifact` or export `policyartifact.IsOwnerHandle`; default `local-operator` for solo; for team missing → error "the team profile needs --owner <kebab-handle>: a team has no honest default owner"). No positionals.

`runPolicyAdopt`:
1. solo: `identity, available, err := readLocalGitIdentity(ctx, root)`; error → 2; `!available` → stderr "policy adopt: the solo profile binds this checkout's git identity, but neither user.email nor user.name is configured in this repository (git config --local user.email …)" → 2. (Requires `verifyGitTopLevel(ctx, root)` first, as `resolveLocalActors` does.)
2. `plan, err := policyadopt.Compose(root, in)`; `errors.Is(err, policyadopt.ErrAlreadyAdopted)` → stderr `policy adopt: <err>` → 1; other → 2. (Nothing written, no branch yet.)
3. `baseRef, ok := resolveBranchBase(ctx, root, "policy adopt", stdout, stderr)`; `!ok` → 2. `checkoutNewBranchDisclosed(ctx, root, "policy adopt", "policy/adopt", baseRef, stdout, stderr)`; false → 2.
4. Re-compose on the new checkout (R-W4-3): `plan, err = policyadopt.Compose(root, in)`; `ErrAlreadyAdopted` → stderr "policy adopt: the default branch already carries policy; left the checkout on policy/adopt with nothing written" → 1; other → 2.
5. `paths, err := policyadopt.Write(root, plan)`; error → stderr names the path and that the checkout is on policy/adopt with partial files → 2. For each file print `policy adopt: wrote <rel> (<identity> <digest>)`.
6. `gitx.AddPaths(ctx, root, paths...)`; `sha, err := gitx.CreateCommit(ctx, root, "policy adopt: starter constitution ("+profile+" profile)")`; → print `policy adopt: committed <short sha> on policy/adopt`.
7. Disclosures (stdout, always): `policy adopt: design_assistance mode <mode> for the <profile> profile — <one clause: solo "your delegated agents may write drafts on design branches you alone can merge" / team "delegated agents may propose; humans write, until the team's own review grants more">`; team: `policy adopt: the team profile maps no subjects yet — add role_mappings to .verdi/policy/profiles/starter-team.md before any approval can be proven`; always: `policy adopt: the consumers inventory registers no consumers yet — register real consumers before impact review`; always: `policy adopt: nothing here is accepted: acceptance is the owner's merge of policy/adopt to the default branch`.
Return 0.

Registry: `dispatch.go` `"policy": 26, // spec/spec-documents ac-10 (dc-6, SI-204) — verdi policy adopt --starter writes the starter constitution store on a policy/adopt branch; closes tracker UAT-018`; the `usage` const gains `policy`; one arm `if verb == "policy" { return cmdPolicy(args[1:], os.Stdout, stderr) }` after `harness`'s. `help.go`: the `topLevelUsage` row and `"policy": policyUsage` in `verbUsage`. `internal/specalign/verbs_test.go`: `"policy"` appended to `inV0` (bare `verdi policy` prints usage and exits 2 before any root — the default arm is safe) plus the customary header-comment paragraph. `internal/showcasealign/coverage_test.go`: `"cli:policy": {goE2E("internal/showcasealign/cli_showcase_test.go")}` with a comment in the file's style. `cli_showcase_test.go`: `TestCLIShowcasePolicyAdopt` — `provisionShowcaseStore` (a real fixturegit clone of examples/showcase, which carries no `.verdi/policy`) → `verdi policy adopt --starter` exit 0 → `git show --name-only` lists exactly the four paths → `verdi lint` exits 0 on the adopted store → `verdi context constitution inspect` reports the proposed snapshot adopted → a second adopt exits 1. Include `SHOWCASE.`-marker reference exactly as the neighbouring tests do (read `TestCLIShowcaseHarness` for the marker convention). `README.md`: table row `| \`verdi policy adopt --starter [--profile solo|team]\` | Adopt a starter constitution (constitution, profile, policy, consumers inventory) on a policy/adopt branch |` and one sentence in the constitution section if one exists (grep "context constitution" in README). `docs/policy-setup-validation.md` as described in Files.

Run: Step 3 tests → PASS; `go test -race -count=1 ./cmd/verdi/ -run 'TestPolicyAdopt|TestDesign|TestHelp|TestVerbUsage' && go test -count=1 ./internal/specalign/ ./internal/showcasealign/ -run 'TestV0CLIVerbInventory|TestShowcaseCoverage|TestCLIShowcasePolicyAdopt|TestVocabProseWitness|Instruction' && go vet ./cmd/... ./internal/...`.

Commit series: `Generalize design start's base resolution and branch disclosure behind a verb prefix`; `Add verdi policy adopt --starter`; `Register the policy verb in dispatch, help, the inventory pin, and the showcase coverage`; `Point the policy-setup doc and README at verdi policy adopt`.

---

### Task 3: The plain vocabulary preset and the sentence-first lint line (Tier 2)

**Files:**
- Create: `internal/initwizard/preset.go`, `preset_test.go`
- Modify: `internal/initwizard/interview.go:60-130` (seed parameter), `interview_test.go`, `cmd/verdi/init.go:57-120,265-300` (`--vocabulary`), `cmd/verdi/init_test.go` (expectations), `cmd/verdi/help.go` (ONLY `"init": "usage: verdi init [--wizard] [--vocabulary plain|canonical]"` and the init line of `topLevelUsage`: `init             scaffold a new verdi store (plain vocabulary by default)`), `internal/showcasealign/cli_showcase_test.go:818` (`TestCLIShowcaseInit`: the fresh-dir case now also asserts `.verdi/model.yaml` decodes to the preset; add a `--vocabulary canonical` case asserting no model.yaml; keep the refusal cases and add `{"init", "--vocabulary", "plain"}` to them), `README.md:124-126,293`, `internal/lint/finding.go:116-121`, `internal/lint/finding_test.go:9-15`, `cmd/verdi/designsupersede_test.go:547-559`, `cmd/verdi/lint.go:19` (doc comment: "every VL rule").

**Interfaces:**
- Consumes: `model.Vocabulary`, `initwizard.RenderModelYAML`, `WriteModelYAML`, `VocabularyEmpty`, `CandidateModel`, `RenameableIDs`, `runModelCheck`.
- Produces:

```go
package initwizard

// PlainPreset is the `plain` vocabulary preset verdi init writes by default (spec/spec-documents
// ac-11, dc-7; SI-205): the two class renames the spec pairs with a renameable id.
func PlainPreset() model.Vocabulary // Classes: {"story": "planned story", "spike": "research spike"} — fresh maps each call

// ParseVocabularyPreset maps a --vocabulary value to its preset: "plain" → PlainPreset(),
// "canonical" → the empty vocabulary (no model.yaml); anything else is an error naming both.
func ParseVocabularyPreset(name string) (model.Vocabulary, error)

func RunInterview(in io.Reader, out io.Writer, seed model.Vocabulary) (InterviewResult, error) // R-W4-6
```

- [ ] **Step 1: Failing tests**

`internal/initwizard/preset_test.go`:

```go
func TestPlainPreset_ValidatesAgainstTheCanonicalModelAndIsFresh(t *testing.T) {
	p := PlainPreset()
	if p.Classes["story"] != "planned story" || p.Classes["spike"] != "research spike" || len(p.Classes) != 2 || len(p.States) != 0 || len(p.Verbs) != 0 {
		t.Fatalf("preset = %+v", p)
	}
	if _, err := model.DecodeModel(RenderModelYAML(p)); err != nil {
		t.Fatalf("preset model.yaml does not decode: %v", err)
	}
	p.Classes["story"] = "mutated"
	if PlainPreset().Classes["story"] != "planned story" {
		t.Fatal("PlainPreset shares state between calls")
	}
	// Display chain: the renamed words flow through the model.
	m := CandidateModel(PlainPreset())
	if m.DisplayClass("story") != "planned story" || m.DisplayClassPlural("spike") != "research spikes" || m.DisplayClass("feature") != "feature" {
		t.Fatalf("display: %s %s %s", m.DisplayClass("story"), m.DisplayClassPlural("spike"), m.DisplayClass("feature"))
	}
}

func TestParseVocabularyPreset(t *testing.T) {
	for _, tc := range []struct{ in string; wantEmpty, wantErr bool }{{"plain", false, false}, {"canonical", true, false}, {"", false, true}, {"Plain", false, true}, {"fancy", false, true}} {
		v, err := ParseVocabularyPreset(tc.in)
		if (err != nil) != tc.wantErr || (err == nil && VocabularyEmpty(v) != tc.wantEmpty) {
			t.Fatalf("%q: v=%+v err=%v", tc.in, v, err)
		}
		if err != nil && !strings.Contains(err.Error(), "plain") {
			t.Fatalf("%q: error must name the legal values: %v", tc.in, err)
		}
	}
}
```

`internal/initwizard/interview_test.go` — add `TestRunInterview_SeedIsTheDefault`: seed = `PlainPreset()`, script of 9 blank lines + `n\nn\ny\n` → result.Vocabulary deep-equals the seed; the transcript shows `[Enter to keep "planned story"]` for the story prompt and `[Enter to keep "feature"]` for feature; a second run with the story answer `"Task"` → Classes{story: "Task", spike: "research spike"}; an EMPTY seed reproduces today's transcript byte-for-byte against the file's existing golden/expectation (read the existing interview tests for how the transcript is asserted and reuse that).

`internal/lint/finding_test.go`: `want := "something broke (.verdi/adr/foo.md) [VL-001]"`.

`cmd/verdi/init_test.go`: `TestInit_Bare_EmptyDir_CreatesMinimalSkeleton` now expects exactly `[".verdi", ".verdi/model.yaml", ".verdi/verdi.yaml"]`, model.yaml bytes `== initwizard.RenderModelYAML(initwizard.PlainPreset())`, and `verdi model check` exit 0; add `TestInit_Bare_VocabularyCanonical_WritesNoModelYAML` with the OLD expectations (`[".verdi", ".verdi/verdi.yaml"]`); `TestInit_Wizard_AllDefaults_MatchesBarePath` expects the preset tree; `TestInit_Wizard_RealRenames_AndTemplateCopy` wantVocab gains `"spike": "research spike"` (the seed survives an untouched prompt; story was renamed to "Task"); add a `--vocabulary bogus` case to `TestInit_UnknownArgument_UsageError`'s table and a `--vocabulary` without a value case; add `TestInit_ExistingStore_VocabularyFlagStillRefuses` (existing `.verdi/verdi.yaml` + `--vocabulary plain` → exit 2, bytes untouched, no model.yaml appears — dc-7).

Run: `go test ./internal/initwizard/ ./internal/lint/ -count=1; go test ./cmd/verdi/ -run 'TestInit' -count=1` — Expected: FAIL.

- [ ] **Step 2: Implement**

`preset.go` as specified (doc comments carry SI-205's disclosure: "research task" and "revision" name no renameable id and are not written). `interview.go`: add the `seed` parameter; `var vocab model.Vocabulary` becomes a deep copy of `seed` (fresh maps); `runRenamePrompt` shows `[Enter to keep %q]` with the seed's value for that id when present (read the function; the current default value is the id itself — keep that when the seed has no entry); an Enter with a seeded value leaves the seeded entry in place. `cmd/verdi/init.go`: the flag loop gains `--vocabulary <v>` / `--vocabulary=<v>` (at most once; missing value → usage exit 2); `cmdInit` passes the parsed preset to `runInit(cwd, wizard, preset, …)` → `stageCandidateStore(tempRoot, wizard, preset, …)`: bare path writes `WriteModelYAML(tempRoot, preset)` when `!VocabularyEmpty(preset)` (same crash-simulation hook `model.yaml`), returning `CandidateModel(preset)` so the existing promotion decode-compare gate covers it; wizard path calls `RunInterview(stdin, stdout, preset)`. Update the `"init: unknown argument"` message's usage text. `lint/finding.go`: `return fmt.Sprintf("%s (%s) [%s]", f.Message, f.Path, f.Rule)` with the doc comment stating R-W4-5 (disclosure branch unchanged). `designsupersede_test.go:557`: `if strings.HasSuffix(line, "]") && strings.Contains(line, "[VL-") && strings.Contains(line, successorRelPath)` and update its comment to the new grammar. `help.go`, `README.md`, `lint.go` doc as listed.

Run: Step 1 tests → PASS; `go test -race -count=1 ./internal/initwizard/ ./internal/lint/ ./internal/model/ && go test -count=1 ./cmd/verdi/ -run 'TestInit|TestRunLintVerb|TestDesignSupersede|TestHelp|TestVerbUsage|TestVocabularyCLI' && go test -count=1 ./internal/showcasealign/ -run 'TestCLIShowcaseInit|TestShowcaseLintClean' && go test -count=1 ./internal/specalign/ -run 'TestVocabProseWitness|Instruction' && go vet ./cmd/... ./internal/...`. Also `grep -rn '"VL-' e2e/tests cmd/e2eharness` to confirm no browser path parses the CLI line.

Commit series: `Add the plain vocabulary preset and seed the init wizard from it`; `Write the plain preset by default from verdi init with --vocabulary canonical opting out`; `Lead lint violation lines with the sentence and trail the rule code`.

---

### Task 4: Guidance-first concern cards, the timing row, and the guide's verb pointer (Tier 2, Fable lane)

**Files:**
- Modify: `internal/workbench/boardshellrender.go:281-380` (`writePolicySetupGuide` ritual-note + details summary + closing note; `writeASDConcern`), `internal/workbench/boardspecasd.go:353` (guidance string only), `internal/dex/assets/style.css:3775-3803` (`.asd-timing*` removed, `.asd-guidance` removed, `.asd-fact` added; tokens only), `internal/workbench/policyguide_test.go` (pins for the verb pointer; existing pins must keep passing), `e2e/tests/50-design-workbench.spec.ts:140-154,173,529-533` (selectors), `internal/workbench/boardspecasd_test.go` (unchanged unless a guidance string pin needs the new sentence).
- Create: `internal/workbench/boardshellrender_test.go` (wall card markup pins), `e2e/tests/81-guidance-first-cards.spec.ts`.
- NOT modified: `internal/workbench/readinessrender.go`, `internal/readinesspilot/*`, the derivation in `boardspecasd.go` (only line 353's string).

**Interfaces:**
- Consumes: `asdConcern{ID, Area, State, Blocking, Summary, Guidance, Witnesses, Dest, HumanReview}`, `asdView.Shell.CurrentFocus`, `asdAreaAfter`, `asdAreaLabels`, `writeReadinessFact`, `writeASDState`, `policySetupGuideID`; Task 2's usage line `verdi policy adopt --starter [--profile solo|team]`.
- Produces: the wall card markup below; `data-timing="now|later"` on the article when timing is determinable; testids `asd-guidance-<id>` (primary line when the row carries guidance), `asd-summary-<id>` (primary line otherwise), `asd-fact-<id>` (secondary fact line when guidance is present).

Target card markup (`writeASDConcern`):

```
<article class="readiness-card|readiness-row readiness-concern--<state>" data-concern-id="<id>" data-area-id="<area>"[ data-timing="now|later"]>
  [<span class="readiness-rank">N</span>]
  <div class="readiness-copy">
    <p class="readiness-stage"><AREA LABEL></p>
    [<p class="asd-human-review" data-testid="asd-human-review">Human review</p>]
    <p class="readiness-summary" data-testid="asd-guidance-<id>"><GUIDANCE></p>        ← when Guidance != ""
    <p class="readiness-summary" data-testid="asd-summary-<id>"><SUMMARY></p>          ← otherwise
    <span class="readiness-state readiness-state--<state>"><PLAIN WORD></span>
    [<p class="asd-fact" data-testid="asd-fact-<id>"><SUMMARY></p>]                    ← when Guidance != ""
    <details class="readiness-tech"><summary>Technical details</summary><dl class="readiness-tech-facts">
      State / Concern / Area / Blocking rows as today
      [<dt>Timing</dt><dd><code>now</code></dd> | <dd><code>later — waits on <LABEL></code></dd>]   ← same condition the inline span used
      Witnesses as today
    </dl></details>
    [<p class="readiness-dest">…</p>]
  </div>
</article>
```

Guide text (`writePolicySetupGuide`, not-adopted arm): the `ritual-note` paragraph becomes: `Initial setup is one verb: run <code>verdi policy adopt --starter [--profile solo|team]</code> from the project root. It writes a starter constitution, one profile, one policy, and the consumers inventory, and commits exactly those files on a <code>policy/adopt</code> branch. This guide adopts nothing and the workbench has no adoption control; a policy directory that is proposed or validated is not accepted: acceptance is the owner&#39;s merge to the default branch through the project&#39;s own review process.` The details summary becomes `Files the starter writes (or author by hand, only when no policy is accepted)`; the closing note: `Run the verb only after the Inspect check confirms no accepted policy. It leaves you on the policy/adopt branch; open the project&#39;s own review from there. Do not copy a fixture&#39;s identities, approvals or trust facts into a real project. <code>verdi context constitution propose</code> amends one policy, overlay or exemption; <code>verdi policy adopt --starter</code> creates the initial constitution and profile.` `boardspecasd.go:353` guidance: `Inspect the accepted and proposed policy snapshots first (policy setup guide below): if policy is already accepted, inspect why this checkout lacks it; an older branch may need updating through the project's own process. Only when no policy is accepted, run verdi policy adopt --starter from the project root; human editing does not require one.` (keeps "Inspect" before "adopt" and "inspect why this checkout lacks").

- [ ] **Step 1: Failing Go tests**

`internal/workbench/boardshellrender_test.go`:

```go
func TestWriteASDConcern_GuidanceIsPrimaryFactIsSecondary(t *testing.T) {
	asd := &asdView{Shell: asdShell{CurrentFocus: asdAreaSuccess}}
	var b strings.Builder
	writeASDConcern(&b, asdConcern{ID: "success/ac", Area: asdAreaSuccess, State: asdStateViolated, Blocking: true, Summary: "No acceptance criteria are declared.", Guidance: "Declare what must be true when this lands.", Witnesses: []string{"w"}}, asd, 1)
	html := b.String()
	primary := `<p class="readiness-summary" data-testid="asd-guidance-success/ac">Declare what must be true when this lands.</p>`
	fact := `<p class="asd-fact" data-testid="asd-fact-success/ac">No acceptance criteria are declared.</p>`
	if !strings.Contains(html, primary) || !strings.Contains(html, fact) || strings.Index(html, primary) > strings.Index(html, fact) {
		t.Fatalf("guidance must lead and the fact follow:\n%s", html)
	}
	if strings.Contains(html, "asd-timing") || !strings.Contains(html, `data-timing="now"`) || !strings.Contains(html, `<dt>Timing</dt><dd><code>now</code></dd>`) {
		t.Fatalf("timing must live in the disclosure and the data attribute:\n%s", html)
	}
	for _, want := range []string{`<dt>Concern</dt><dd><code>success/ac</code></dd>`, `<dt>Blocking</dt><dd><code>true</code></dd>`, `<span class="readiness-state readiness-state--violated-with-witness">Needs attention</span>`, `<p class="readiness-stage">Define success</p>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q:\n%s", want, html)
		}
	}
}

func TestWriteASDConcern_NoGuidanceShowsSummaryAsPrimaryAndLaterTiming(t *testing.T) {
	asd := &asdView{Shell: asdShell{CurrentFocus: asdAreaShape}}
	var b strings.Builder
	writeASDConcern(&b, asdConcern{ID: "review/x", Area: asdAreaReview, State: asdStateUnproven, Summary: "Human review has not accepted this proposal yet."}, asd, 2)
	html := b.String()
	if !strings.Contains(html, `<p class="readiness-summary" data-testid="asd-summary-review/x">Human review has not accepted this proposal yet.</p>`) || strings.Contains(html, "asd-fact") {
		t.Fatalf("summary must be the primary line when there is no guidance:\n%s", html)
	}
	if !strings.Contains(html, `data-timing="later"`) || !strings.Contains(html, `<dt>Timing</dt><dd><code>later — waits on Define the work</code></dd>`) {
		t.Fatalf("later timing:\n%s", html)
	}
	// A proven row carries no timing at all.
	b.Reset()
	writeASDConcern(&b, asdConcern{ID: "shape/problem", Area: asdAreaShape, State: asdStateProven, Summary: "The problem statement is present."}, asd, 0)
	if strings.Contains(b.String(), "Timing") || strings.Contains(b.String(), "data-timing") {
		t.Fatalf("proven rows carry no timing:\n%s", b.String())
	}
}

func TestPolicyGuide_NamesTheAdoptVerb(t *testing.T) {
	html := renderBoardRegion(badgeRenderProjection(modeAuthoring), &boardGitState{Branch: "design/x", DefaultBranch: "main"}, policyForbiddenView())
	guide := policyGuideSection(t, html)
	for _, want := range []string{"verdi policy adopt --starter [--profile solo|team]", "policy/adopt", "no adoption control", "not accepted"} {
		if !strings.Contains(guide, want) {
			t.Fatalf("guide missing %q", want)
		}
	}
	if strings.Contains(guide, "no setup wizard") {
		t.Fatal("the no-wizard disclaimer survived")
	}
	if n := strings.Count(guide, `<pre class="asd-policy-guide-cmd">`); n != 4 {
		t.Fatalf("read-only check blocks = %d, want 4 (adopt is a pointer, not a here-doc)", n)
	}
}
```

Run: `go test ./internal/workbench/ -run 'TestWriteASDConcern|TestPolicyGuide' -count=1` — Expected: FAIL.

- [ ] **Step 2: Implement markup, CSS, and the guidance string**, then run `go test -race -count=1 ./internal/workbench/ && go test -count=1 ./internal/specalign/ -run 'TestVocabProseWitness|Instruction' && go vet ./internal/workbench/`. Every new literal with a class/state word carries a `// vocab:identity` marker with its reason (the guide's `verdi policy adopt` text has none).

- [ ] **Step 3: Playwright**

Update `50-design-workbench.spec.ts`: line 143's `getByTestId("asd-guidance-shape/question/oq-2")` still resolves (primary line); line 529's `policyRow.locator("p.readiness-summary")` expectations move to `policyRow.getByTestId("asd-fact-context/policy")`; line 533 unchanged. New `e2e/tests/81-guidance-first-cards.spec.ts` (imports `SHOWCASE` from `./fixtures` as 50 does; reuse its `DESIGN()`/`DRAFT_B()` helpers by copying their two-line definitions):
1. on the design wall, for every `[data-concern-id]` row: the first `p.readiness-summary` inside `.readiness-copy` precedes `.readiness-state`; rows with `[data-testid^="asd-guidance-"]` also carry `[data-testid^="asd-fact-"]` after the chip; no `.asd-timing` exists; every non-proven row inside the focus queue has `data-timing` and, after opening its `.readiness-tech summary`, a `dt:text-is("Timing") + dd` whose text is `now` or starts with `later — waits on`; every row's disclosure has `Concern` and `Blocking` rows;
2. the four rail labels and the three chip words are unchanged (`RAIL`/`PLAIN_LABELS` copied from `49-readiness-pilot.spec.ts:63-102`);
3. on `/readiness`: the first child of each `.readiness-copy` after `.readiness-stage` is `p.readiness-summary` and the disclosure carries `Concern`, `Timing`, `Blocking` (the ac-12 readiness-page pin, R-W4-7);
4. on `DRAFT_B()` the policy guide text names `verdi policy adopt --starter` and contains no `setup wizard`, and still has exactly four `pre.asd-policy-guide-cmd` blocks;
5. dark mode: `.readiness-summary` and `.asd-fact` colours differ between `colorScheme: "light"` and `"dark"` emulations (the 49 spec's pattern at lines 664-699).

Run: `cd e2e && VERDI_E2E_PORT_BASE=4490 npx playwright test tests/50-design-workbench.spec.ts tests/49-readiness-pilot.spec.ts tests/75-policy-guide-code.spec.ts tests/81-guidance-first-cards.spec.ts` → all pass; then `find e2e -name '*.png' -o -name '*.webm' -o -name 'trace.zip' | grep -v node_modules` prints nothing.

Commit series: `Lead wall concern cards with their guidance and move timing into the technical disclosure`; `Point the policy setup guide and the context/policy concern at verdi policy adopt --starter`; `Add the guidance-first cards Playwright path`.

---

### Task 5: Wave gate

- [ ] **Step 1:** `go build ./... && gofmt -l . && go vet ./... && golangci-lint run ./...` — clean.
- [ ] **Step 2:** `VERDI_E2E_PORT_BASE=4390 make verify` from the worktree root — `verify OK`, exit 0; `git status --porcelain` empty; recording-artifact scan prints nothing.

### Task 6: Wave report

Write `docs/superpowers/reports/2026-09-19-spec-documents-wave-4.md` in the evidence format (Status; Risk tier 3 for ac-10, 2 for ac-11/ac-12; Base..Head; Commits; Files changed; Contract implemented per ac-10/11/12; Explicit exclusions; RED/GREEN; Reviewer verdicts; Residual risks including SI-205's two unplaced phrases and the team profile's empty mappings; Integration prerequisites), then commit `Report spec-documents wave 4: policy adopt starter, plain vocabulary, guidance-first cards`. Update the UAT tracker entry (controller, outside git): UAT-018 → fixed pending merge, naming the closing commit.

---

## Self-review

**Spec coverage.** ac-10: Task 1 (templates through the existing scaffold renderer with override; `TemplateRecord` recorded in all four artifacts — the profile and inventory seams added), Task 2 (the verb; `policy/adopt` branch; exactly four paths staged and committed; no artifact kind added — constitution, profile, policy, inventory all pre-exist; every rendered rule real: one typed design_assistance payload, no placeholder claims), Task 4 (the guide's no-wizard disclaimer replaced; the context/policy concern names the verb). ac-11: Task 3 (preset written by `verdi init` by default; `--vocabulary canonical` opts out; existing stores untouched by construction and pinned; lint line sentence-first with the trailing bracketed code; renameable set and witness untouched). ac-12: Task 4 (guidance primary on the wall; id/timing/blocking in the disclosure; `/readiness` already conforming and pinned; labels and triad unchanged; no derivation change). dc-6: SI-204 in Task 0. dc-7: preset is configuration; init is create-only. co-4: Task 4 is the Fable lane.

**Placeholders.** `prove`'s loop over `EffectivePolicy.Policies` points at the exact draftmutation lines it mirrors and tells the implementer to read `resolve.go` for the field names; `gitOutput`/`runVocabStdin`-style helpers are named with their one-line bodies; the interview transcript assertion reuses the file's existing pattern by instruction. Everything else carries its code or exact text.

**Type consistency.** `humanartifact.{ConstitutionTemplate, ProfileSoloTemplate, ProfileTeamTemplate, StarterPolicyTemplate, InventoryTemplate}` and the four `Render*` signatures (Task 1) are what `policyadopt.Compose` calls (Task 2); `policyadopt.{Input, Plan, File, Compose, Write, ErrAlreadyAdopted}` are what `cmd/verdi/policy.go` calls; `resolveBranchBase`/`checkoutNewBranchDisclosed` (Task 2) keep the design-start wrappers; `initwizard.{PlainPreset, ParseVocabularyPreset}` and the three-argument `RunInterview` (Task 3) are what `init.go` calls; Task 4's testids are what `81-*.spec.ts` and the updated `50-*.spec.ts` select; the usage line Task 4 quotes is Task 2's `policyUsage` minus `[--owner …]`.

**Known limits carried.** The team starter maps no subjects (disclosed). The consumers inventory is empty (disclosed; impact coverage resolves `consumer-universe-empty`). Two of ac-11's four phrases are unplaced (SI-205). Disclosure lines keep the shared grammar (R-W4-5). `docs/guide-claims.yaml`'s `4-init-wizard` row is not touched.

## Amendments during execution

Recorded here by the controller as they happen; where a code block here disagrees with HEAD, HEAD is right.

- Task 1 (R-W4-10): `governanceprincipal.validateClassCoverage` requires, for a team profile, an approval rule AND a different-principal rule covering EVERY applicable transition; the team template as first written covered only `accept`. The template gains `{transitions: [policy-disposition-approval], left_role: author, right_role: policy-owner, relation: different-principal}` — a disposition's author and its approving policy owner must be different people, the same separation the accept rule already states. Task 2's `prove` and the wave report inherit the two-rule shape.
