# Spec Documents — Wave 3 Implementation Plan (ac-7 harness render/check, ac-8 skills, ac-9 import MCP tools)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give coding agents verdi-native entry points: four generated, stamped, drift-checked skills (specify, clarify, plan, tasks) for Claude Code and Codex, rendered by `verdi harness render` and verified by `verdi harness check`; two MCP tools (`import_preview`, `import_apply`) that wrap the frozen import contract with the delegated-agent actor; and a hermetic transcript proof for every skill's tool sequence.

**Architecture:** A new package `internal/skillpack` embeds the four skill templates, renders them per host with a generated-file marker and three stamps (engine digest, template digest, render commit), and checks on-disk copies for drift with the render-commit line masked. A new CLI verb `harness` wraps it and joins `make verify` through `lint-store`. `internal/mcpserve` gains `import_preview`/`import_apply` over `specimport.Service.Preview/Apply` with `draftmutation.NewDelegatedAgent(harness, session)` exactly as `mutate_draft` mints its actor; the import record already names harness and session. `get_document` gains the serve-time readiness snapshot so the `clarify` skill can list unproven concerns. Each template carries a machine-readable `verdi-sequence` block; a transcript test parses it from the embedded template and replays it against an in-process MCP server, so the proof is bound to the shipped skill text.

**Tech Stack:** Go 1.25 (`github.com/jyang234/verdi`), `embed`, `internal/canonjson`, `internal/atomicfile`, `internal/gitx`, `internal/specdoc` + `internal/specdocload` (Waves 1–2), `internal/specimport` (import contract), `internal/draftmutation`, `internal/mcpserve` (`strictUnmarshal`, `toolJSON`/`toolError`, NDJSON `ServeConn`), `internal/fixturegit`, `internal/readinesspilot`.

**Spec:** `.verdi/specs/active/spec-documents/spec.md` on main d15d6986 (accepted in PR #327). This plan implements ac-7, ac-8, ac-9 under co-1..co-6, dc-4, dc-5, dc-8, and resolves oq-1 through the `codex-prompt-conventions` spike (recorded below and in the invention ledger as SI-201). Waves 1 and 2 (`docs/superpowers/reports/2026-09-18-spec-documents-wave-{1,2}.md`) are the base.

## Global Constraints

- No network in any test (co-1). CLI paths through the built binary over `internal/fixturegit` repos with `CI_DEFAULT_BRANCH=main`; MCP paths through the in-process `Backend` over `mcpserve.ServeConn`'s real NDJSON framing; git through fixtures. No browser-facing work in this wave (co-4 is not engaged).
- The document is never authority (co-2): skills read documents through `get_document`; nothing reads a document or a skill file back into store objects.
- The write surface stays closed (co-3): the only MCP write tools after this wave are `mutate_draft`, `add_annotation`, `import_apply`. `import_preview`, `get_document`, and every other tool are read-only. Skills write only through those tools or the `verdi design import` verb.
- Skills are projections (dc-4): generated from templates embedded in the binary, stamped, drift-checked, re-rendered by verdi, never hand-edited. Every byte written is canonical: no wall clock, username, absolute path, or random identifier enters a rendered skill except the declared render-commit stamp.
- Brief-driven drafting reuses the frozen import contract (dc-5, `docs/superpowers/specs/2026-09-14-spec-import-contract.md`): `import_preview`/`import_apply` add only the MCP wrapping; request/preview/result/record schemas, digests, coverage, byte offsets, origins, and the exact write set are unchanged. Client-supplied candidate/provenance/actor fields do not exist.
- The CLI-verb and MCP-tool inventories are serialized registries: Task 2 alone touches the verb registry; Task 3 alone touches the MCP inventory and its four pins plus the showcase-coverage rows. No other task edits `dispatch.go`, `tooldefs.go`, `server.go`'s switch, `server_test.go`'s count, `serve_integration_test.go`'s count, `specalign/mcptools_test.go`, `specalign/verbs_test.go`, or `showcasealign/coverage_test.go`.
- co-5: every new production literal containing a class word (`feature`, `story`, `component`, `spike`) or a lifecycle state word (`draft`, `proposed`, `accepted-pending-build`, `accepted`, `superseded`, `closed`) routes through `*model.Model` (`DisplayClass`, `DisplayState`) or carries `// vocab:identity — <why>` on its line or the line above. Embedded templates are data files, not Go literals, and are outside the witness; the skill text still uses the reader's own words where a vocabulary is configurable ("the spike word your store uses"). `go test -count=1 ./internal/specalign/ -run TestVocabProseWitness` must pass after every task.
- co-6: an unavailable fact is stated, never omitted: a skill that finds readiness "not supplied for this render" says so and continues with what the document does state.
- Instruction conformance (spec/instruction-conformance AC-2/AC-3): rendered skills in this repository are enumerated instruction files. Every `verdi <verb>` reference inside backticks must be a dispatch-recognized verb; the retired `verdi board commit` ritual is never described as a current step. `go test -count=1 ./internal/specalign/ -run 'Instruction'` must pass after Task 2 and Task 5.
- Exit codes: every verb exits 0 (clean) / 1 (verdict: drift, missing) / 2 (operational: flag shape, unusable root, I/O).
- Risk tiers (dc-8): Task 3 (ac-9) is Tier 3 — Sonnet implementer, independent Opus review, a fresh Opus fixer and a fresh Opus re-reviewer for any Critical or Important finding. Tasks 1, 2, 4, 5 are Tier 2.
- gofmt-clean, `golangci-lint` clean (`.golangci.yml`), `go vet` clean, `go test -race` clean. Commit subjects imperative. Implementer commits end with `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`. Never write a `// path/to/file.go` code-block marker into a file.
- Work happens in `/Users/johnyang/code/verdi-system/verdi-wt/spec-documents-w3` on branch `agent/spec-documents-wave-3` (base main d15d6986). Never use bare `git stash`. Read `/Users/johnyang/code/verdi-system/CLAUDE.md` and the repo's `CLAUDE.md` first. Port base for anything that binds: `VERDI_E2E_PORT_BASE=4390` for the wave gate; lanes use 4490.

---

## Rulings recorded in this plan

- **R-W3-1 (oq-1 resolved — Codex output location and marker).** The spike `codex-prompt-conventions` answered oq-1 on 2026-09-18 against Codex CLI rust-v0.155.0 (2026-09-17) and its hosted docs: Codex reads repository-local reusable instructions as Agent Skills at `.agents/skills/<name>/SKILL.md` (frontmatter `name` + `description`; scanned from the working directory up to the repository root; `<repo>/.codex/skills` is also scanned through the project config layer but is doc-implicit); custom prompts (`~/.codex/prompts`) are deprecated, home-only, and have no repository-local form; no host convention marks a generated file. Therefore ac-7's "Codex prompt equivalents" are rendered as skills at `.agents/skills/verdi-{specify,clarify,plan,tasks}/SKILL.md`, byte-identical to the Claude Code skills except the `host=` value in the generated-skill marker, and the generated marker is verdi's own HTML-comment block placed immediately after the frontmatter (the same posture `internal/instructionprojection` uses for AGENTS.md/CLAUDE.md). Recorded as SI-201 in `docs/superpowers/invention-ledger.md` before Task 1 dispatches. Cost if wrong: one directory constant and one ledger amendment.
- **R-W3-2 (drift ignores the render commit).** `verdi harness check` recomputes each skill from the running binary and compares bytes with the `<!-- verdi:render-commit … -->` line removed from both sides: the render commit is provenance, and comparing it would report drift after every commit of the consuming repository. Engine digest and template digest are compared (they are content). Cost if wrong: one normalization function.
- **R-W3-3 (readiness reaches MCP).** `clarify` needs unproven readiness concerns; the only readiness source is the snapshot `verdi serve` builds at startup. `Backend` gains `Readiness *readinesspilot.Snapshot`, set by `runServe` on the socket server it starts, and `get_document` passes it to the loader, which already gates on `TargetRef`. Wave 2's parity boundary is extended: the parity test gains an arm proving the board and MCP render identical bytes under the same snapshot. A standalone `verdi mcp` with no snapshot renders "Readiness was not supplied for this render." and `clarify` proceeds on the document's other sections. Cost if wrong: one field and one parity arm.
- **R-W3-4 (harness and session are tool arguments).** The MCP server holds no per-connection identity (`initialize` ignores its params; SI-163: the MCP actor stays delegated-agent). `import_apply` therefore takes `harness` (required) and `session` (optional) exactly like `mutate_draft`; `import_preview` takes neither (preview is read-only and writes no record). The record's `actor.harness`/`actor.session` fields already exist and are populated by `specimport.Service.Apply`. Cost if wrong: two argument fields.
- **R-W3-5 (a completed preview with findings is a result, not an error).** `import_preview` returns the `PreviewResult` JSON with `ready:false` and `isError:false` when the preview completes with blocking findings (the contract's HTTP mapping: 200 with `ready:false`); operational and validation failures return `isError:true` with the CLI's closed code vocabulary as the message prefix (`invalid-request: …`, `stale-preview: …`, …). `import_apply` refusals are `isError:true` with the same prefixes. Cost if wrong: one boolean.
- **R-W3-6 (the sequence block binds the proof to the skill).** Each template carries one fenced block with info string `verdi-sequence`. The transcript test parses that block from the embedded template bytes and replays it; if the prose and the block disagree, the block is wrong and the test is the witness that finds it in review. Cost if wrong: one parser.
- **R-W3-7 (render root).** `harness render|check` default to `store.FindRoot(".")`; `-o <repo root>` names an existing directory with no ancestor search and no store requirement (rendering reads nothing from a store). The render commit is `gitx.RevParse(ctx, root, "HEAD")` or the literal `none` when the root is not inside a git repository. Cost if wrong: one flag semantics note.
- **R-W3-9 (get_document renders drafts on request).** ac-8's clarify, plan, and tasks skills read a draft on its design branch, but ac-5's `get_document(ref, kind, commit?)` renders only the accepted bytes or a pinned commit. The tool gains an optional boolean `proposed` (the CLI's `--proposed` flag by name): true selects the loader's working-tree mode over the serving checkout, with Proposed derived exactly as the CLI derives it (R-W2-4); `proposed` and `commit` together are refused by name. Recorded as SI-202 (a public MCP interface extension). Cost if wrong: one optional argument.
- **R-W3-8 (this repository carries its own rendered skills).** `make verify` runs `verdi harness check` from `lint-store`, so `.claude/skills/verdi-*/SKILL.md` and `.agents/skills/verdi-*/SKILL.md` are committed in this repository and re-rendered whenever a template changes; the check is the drift gate. They are subject to spec/instruction-conformance's enumeration. Cost if wrong: eight files to delete.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/skillpack/doc.go` | Package doc: projections, stamps, drift, host parity. |
| `internal/skillpack/host.go` | `Host`, `Hosts()`, `ParseHosts`, `Path(host, skill)`. |
| `internal/skillpack/templates/{specify,clarify,plan,tasks}.md` | The four embedded skill templates (frontmatter + body + `verdi-sequence` block). |
| `internal/skillpack/engine.go` | `EngineDigest()`, `TemplateDigest(skill)`, `Skills()`. |
| `internal/skillpack/render.go` | `Render(host, skill, renderCommit) (Rendered, error)`, the marker block, `RenderCommit(ctx, root)`. |
| `internal/skillpack/write.go` | `Write(ctx, root, hosts) (Result, error)` — atomic writes, sorted result. |
| `internal/skillpack/check.go` | `Check(root, hosts) (Report, error)` — `missing`/`drift` findings with the render-commit line masked. |
| `internal/skillpack/sequence.go` | `ParseSequence(template []byte) (Sequence, error)` — the `verdi-sequence` grammar (used by Task 5's proof and by a static template test). |
| `internal/skillpack/*_test.go`, `testdata/golden/<host>/verdi-<skill>.md` | Unit tests and goldens. |
| `cmd/verdi/harness.go` | `verdi harness render|check [--host claude|codex|all] [-o <repo root>]`. |
| `cmd/verdi/dispatch.go`, `help.go`, `internal/specalign/verbs_test.go`, `internal/showcasealign/coverage_test.go`, `CLAUDE.md`, `Makefile` | Verb registry, usage, inventory pins, `cli:harness` coverage row, gate wiring. |
| `.claude/skills/verdi-*/SKILL.md`, `.agents/skills/verdi-*/SKILL.md` | This repository's own rendered skills (R-W3-8). |
| `internal/mcpserve/tool_import.go` | `ImportPreview`, `ImportApply`, sentinel→code mapping. |
| `internal/mcpserve/tooldefs.go`, `server.go`, `backend.go`, `tool_get_document.go` | Two tool definitions and dispatch arms; `Backend.Readiness`; readiness through `get_document`. |
| `internal/mcpserve/tool_import_test.go`, `tool_get_document_test.go` | Tool tests. |
| `internal/mcpserve/server_test.go`, `cmd/verdi/serve_integration_test.go`, `internal/specalign/mcptools_test.go`, `internal/showcasealign/coverage_test.go`, `internal/showcasealign/mcp_showcase_test.go` | The four inventory pins 19→21 and the two showcase rows with proofs. |
| `cmd/verdi/serve.go` | Passes the readiness snapshot to the socket MCP server's `Backend`. |
| `cmd/verdi/document_parity_e2e_test.go` | New arm: board and MCP under the same readiness snapshot. |
| `internal/skillpack/transcript_test.go`, `testdata/store/…` | The four hermetic transcript proofs (ac-8). |
| `docs/superpowers/invention-ledger.md` | SI-201 (controller-authored, Task 0). |
| `docs/superpowers/reports/2026-09-18-spec-documents-wave-3.md` | Wave report (Task 6). |

---

## Task 0 (controller): record SI-201

Controller-authored before any dispatch (spec-only authority work). Append to the successor section of `docs/superpowers/invention-ledger.md`, after SI-200, one row:

| ID | Bounded decision and authority | Destination | Status |
|---|---|---|---|
| SI-201 | spec/spec-documents oq-1 is answered by the `codex-prompt-conventions` spike (2026-09-18, Codex CLI rust-v0.155.0 and its hosted docs): Codex reads repository-local reusable instructions as Agent Skills at `.agents/skills/<name>/SKILL.md` (`name` + `description` frontmatter); custom prompts are deprecated and home-only; no host marks generated files. ac-7's Codex output is therefore rendered as skills at `.agents/skills/verdi-{specify,clarify,plan,tasks}/SKILL.md`, byte-identical to the Claude Code skills at `.claude/skills/…` except the `host=` marker value, with verdi's own HTML-comment generated-skill marker after the frontmatter. Smallest reversible option: one directory constant. | `internal/skillpack/host.go`; plan `docs/superpowers/plans/2026-09-18-spec-documents-wave-3.md` R-W3-1 | recorded 2026-09-18 |

Commit: `Record SI-201: Codex skill location and generated marker for spec-documents oq-1`.

---

### Task 1: `internal/skillpack` — templates, stamps, render, write, check, sequence

**Files:**
- Create: `internal/skillpack/doc.go`, `host.go`, `engine.go`, `render.go`, `write.go`, `check.go`, `sequence.go`
- Create: `internal/skillpack/templates/specify.md`, `clarify.md`, `plan.md`, `tasks.md`
- Create: `internal/skillpack/host_test.go`, `engine_test.go`, `render_test.go`, `write_test.go`, `check_test.go`, `sequence_test.go`, `testdata/golden/claude/verdi-specify.md` (and the other seven goldens)

**Interfaces:**
- Consumes: `canonjson.Digest`, `atomicfile.Write(path, data, perm)`, `gitx.RevParse(ctx, dir, "HEAD")`, `specdoc.EngineDigest()`.
- Produces (Task 2 and Task 5 rely on these exact names):
  ```go
  type Host string
  const ( HostClaude Host = "claude"; HostCodex Host = "codex" )
  func Hosts() []Host
  func ParseHosts(s string) ([]Host, error)          // "", "all" → both; "claude"; "codex"; else error
  func Skills() []string                              // {"specify","clarify","plan","tasks"}
  func Path(h Host, skill string) string              // ".claude/skills/verdi-specify/SKILL.md" / ".agents/skills/verdi-specify/SKILL.md"
  func EngineDigest() string
  func TemplateDigest(skill string) (string, error)
  func Template(skill string) ([]byte, error)
  type Rendered struct { Host Host; Skill, Path string; Content []byte; Digest string }
  func Render(h Host, skill, renderCommit string) (Rendered, error)
  func RenderCommit(ctx context.Context, root string) string   // 40-hex or "none"
  type FileDigest struct { Path, Digest string }
  type Result struct { RenderCommit string; Files []FileDigest }   // sorted by Path
  func Write(ctx context.Context, root string, hosts []Host) (Result, error)
  type Finding struct { Code, Path, Expected, Actual string }      // Code ∈ {"missing","drift"}
  type Report struct { Findings []Finding; Checked int }
  func (r Report) Clean() bool
  func Check(root string, hosts []Host) (Report, error)
  type Step struct { Kind string; Tool string; Args map[string]string }  // Kind ∈ {"call","show","confirm","loop","end"}
  type Sequence []Step
  func ParseSequence(template []byte) (Sequence, error)
  ```

- [ ] **Step 1: Write the failing host and engine tests**

```go
package skillpack

import (
	"strings"
	"testing"
)

func TestParseHosts(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Host
		err  bool
	}{
		{"", []Host{HostClaude, HostCodex}, false},
		{"all", []Host{HostClaude, HostCodex}, false},
		{"claude", []Host{HostClaude}, false},
		{"codex", []Host{HostCodex}, false},
		{"cursor", nil, true},
		{"Claude", nil, true},
	} {
		got, err := ParseHosts(tc.in)
		if (err != nil) != tc.err {
			t.Fatalf("ParseHosts(%q) err = %v, want err %v", tc.in, err, tc.err)
		}
		if !tc.err && strings.Join(hostStrings(got), ",") != strings.Join(hostStrings(tc.want), ",") {
			t.Fatalf("ParseHosts(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func hostStrings(hs []Host) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, string(h))
	}
	return out
}

func TestPath(t *testing.T) {
	for _, tc := range []struct{ h Host; skill, want string }{
		{HostClaude, "specify", ".claude/skills/verdi-specify/SKILL.md"},
		{HostCodex, "tasks", ".agents/skills/verdi-tasks/SKILL.md"},
	} {
		if got := Path(tc.h, tc.skill); got != tc.want {
			t.Fatalf("Path(%s,%s) = %q, want %q", tc.h, tc.skill, got, tc.want)
		}
	}
}

func TestEngineDigestIsStableAndBindsDocumentEngine(t *testing.T) {
	a, b := EngineDigest(), EngineDigest()
	if a != b || !strings.HasPrefix(a, "sha256:") {
		t.Fatalf("EngineDigest unstable or unprefixed: %q %q", a, b)
	}
	if got := engineDescriptorDigest(engineDescriptor{ID: engineID, Version: engineVersion, Skills: Skills(), Hosts: hostStrings(Hosts()), Document: "sha256:other"}); got == a {
		t.Fatal("EngineDigest must change when the document engine digest changes")
	}
}

func TestTemplateDigestPerSkill(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Skills() {
		d, err := TemplateDigest(s)
		if err != nil {
			t.Fatalf("TemplateDigest(%s): %v", s, err)
		}
		if !strings.HasPrefix(d, "sha256:") || seen[d] {
			t.Fatalf("TemplateDigest(%s) = %q: unprefixed or duplicate", s, d)
		}
		seen[d] = true
	}
	if _, err := TemplateDigest("deploy"); err == nil {
		t.Fatal("unknown skill must refuse")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/skillpack/ -run 'TestParseHosts|TestPath|TestEngineDigest|TestTemplateDigest' -count=1`
Expected: build failure (package does not exist).

- [ ] **Step 3: Write `doc.go`, `host.go`, `engine.go` and the four templates**

`internal/skillpack/doc.go`:

```go
// Package skillpack renders and verifies the four verdi skills (specify,
// clarify, plan, tasks) that coding-agent harnesses read from a consuming
// repository (spec/spec-documents ac-7, ac-8; dc-4: "Skills are
// projections, not authority"). Templates are embedded in the binary so a
// skill can never drift from the engine that serves it; every render is
// stamped with the skillpack engine digest, the template digest, and the
// render commit, and marked as generated; Check recomputes each skill from
// the running binary and reports drift or absence (R-W3-2: the render
// commit is provenance and is masked from the comparison).
//
// Both hosts receive the same Agent Skills format (frontmatter `name` +
// `description`, Markdown body): Claude Code at .claude/skills/<name>/
// SKILL.md and Codex at .agents/skills/<name>/SKILL.md (SI-201, the
// codex-prompt-conventions spike). The bytes differ only in the marker's
// host= value.
//
// Every byte written is canonical (no wall clock, username, absolute
// path, or random identifier) except the declared render-commit stamp.
package skillpack
```

`internal/skillpack/host.go`:

```go
package skillpack

import (
	"fmt"
	"path"
)

// Host is a coding-agent harness that reads skills from a repository.
type Host string

const (
	HostClaude Host = "claude"
	HostCodex  Host = "codex"
)

// Hosts lists every supported host in render order.
func Hosts() []Host { return []Host{HostClaude, HostCodex} }

// ParseHosts maps the --host flag value to hosts: "" and "all" select
// every host; a single host name selects that host; anything else is a
// usage error.
func ParseHosts(s string) ([]Host, error) {
	switch s {
	case "", "all":
		return Hosts(), nil
	case string(HostClaude):
		return []Host{HostClaude}, nil
	case string(HostCodex):
		return []Host{HostCodex}, nil
	}
	return nil, fmt.Errorf("skillpack: unknown host %q (want claude, codex, or all)", s)
}

// dir is the repository-relative skills directory each host scans
// (Claude Code: .claude/skills; Codex: .agents/skills — SI-201).
func (h Host) dir() string {
	switch h {
	case HostClaude:
		return ".claude/skills"
	case HostCodex:
		return ".agents/skills"
	}
	return ""
}

// Path is the slash-separated repository-relative path of one rendered
// skill: <host dir>/verdi-<skill>/SKILL.md.
func Path(h Host, skill string) string {
	return path.Join(h.dir(), "verdi-"+skill, "SKILL.md")
}
```

`internal/skillpack/engine.go`:

```go
package skillpack

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/specdoc"
)

//go:embed templates/*.md
var templates embed.FS

var skillNames = []string{"specify", "clarify", "plan", "tasks"}

// Skills lists the four skill names in render order.
func Skills() []string { return append([]string(nil), skillNames...) }

const (
	engineID      = "verdi.skillpack"
	engineVersion = 1
)

// engineDescriptor is what EngineDigest hashes: the renderer's identity,
// the skill and host inventories, and the document engine the skills read
// through get_document, so a document-shape change re-stamps every skill.
type engineDescriptor struct {
	ID       string   `json:"id"`
	Version  int      `json:"version"`
	Skills   []string `json:"skills"`
	Hosts    []string `json:"hosts"`
	Document string   `json:"document_engine"`
}

func engineDescriptorDigest(d engineDescriptor) string {
	digest, err := canonjson.Digest(d)
	if err != nil {
		panic("skillpack: engine descriptor is not canonical-JSON encodable: " + err.Error())
	}
	return digest
}

// EngineDigest is the canonical digest of the skillpack renderer's identity.
func EngineDigest() string {
	hosts := make([]string, 0, len(Hosts()))
	for _, h := range Hosts() {
		hosts = append(hosts, string(h))
	}
	return engineDescriptorDigest(engineDescriptor{
		ID: engineID, Version: engineVersion, Skills: Skills(), Hosts: hosts, Document: specdoc.EngineDigest(),
	})
}

// Template returns the embedded template bytes for skill.
func Template(skill string) ([]byte, error) {
	for _, s := range skillNames {
		if s == skill {
			return templates.ReadFile("templates/" + skill + ".md")
		}
	}
	return nil, fmt.Errorf("skillpack: unknown skill %q", skill)
}

// TemplateDigest is "sha256:"+hex over the exact embedded template bytes.
func TemplateDigest(skill string) (string, error) {
	b, err := Template(skill)
	if err != nil {
		return "", err
	}
	return contentDigest(b), nil
}

func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
```

The four templates. Each begins with frontmatter, then the body, then exactly one `verdi-sequence` block. Write them verbatim.

`internal/skillpack/templates/specify.md`:

````markdown
---
name: verdi-specify
description: Turn a brief, a Markdown draft, or an existing spec file into a proposed verdi spec on its own design branch through the import contract — preview first, apply only after the human confirms the preview digest.
---

# verdi-specify

Use this skill when the user has a brief or a document and wants a verdi spec drafted from it. It writes only through the import contract (`import_preview` then `import_apply` over MCP, or `verdi design import preview` then `verdi design import apply` on the CLI). It never edits `.verdi/` directly.

## What the import contract guarantees

- Preview is read-only and deterministic: the same request over the same commit yields the same preview digest.
- Apply recomputes the preview and refuses when the digest changed (`stale-preview`), when a blocking finding remains (`unresolved`), or when the target already exists (`target-exists`).
- Every applied import records byte-offset provenance for each mapped span, coverage accounting for every source byte, the origin of each field (`copied-source`, `user-edited-source`, `user-added`, `native`), and the harness and session that proposed the mapping.

## Steps

1. Collect the source. For a file in the repository, run `verdi design import source --root <dir> --file <relative path>` (add `--start-line`/`--end-line` for a range) and keep the printed Source JSON. For text the user pasted, build a Source with `id` (lowercase slug), `label`, and base64 `data` yourself.
2. Compose the request: `schema: verdi.spec-import-request/v1`, `target` (`slug`, `class` feature or story, `title`, optional `story` parent), `format` (`markdown-v1` for ordinary headed Markdown, `native` for an existing verdi spec file, `manual-v1` when you supply every mapping), `primary` (the source id), `sources`, optional `mappings`, `retain_unmapped` (true only when the user disposes of the unmapped remainder as retained-only), `defer_statements` (true only when the user explicitly defers problem and outcome).
3. Preview. Call `import_preview` with the request (or `verdi design import preview --request -` with the JSON on stdin).
4. Show the human the preview: the `digest`, each field with its origin and evidence, every finding with its code and whether it blocks, and the coverage totals per source. If `ready` is false, fix the request (a mapping, a transform, a retained-only disposition) and preview again; never apply a not-ready preview.
5. Ask the human to confirm that exact digest. Do not apply on any other signal.
6. Apply. Call `import_apply` with the same request, `preview_digest` set to the confirmed digest, `harness` set to your harness id (`claude-code` or `codex`), and `session` set to your session id when you have one (or `verdi design import apply --request - --preview <digest> --harness <id>`).
7. Report the result: `status` (`created` or `already-created`), `branch`, `commit`, `spec_ref`, `board_path`, and any disclosure. Point the human at the board path and at `verdi design import record --branch <branch> --spec <slug>` for the provenance record.

## Refusals you must relay verbatim

`invalid-request`, `invalid-source`, `unsupported-format`, `unresolved`, `dirty-context`, `stale-preview`, `target-exists`, `policy-forbidden`, `actor-forbidden`, `provenance-mismatch`. Each names its cause; do not retry blindly. A dirty checkout must be committed or cleaned by the human first.

```verdi-sequence
call import_preview
show
confirm
call import_apply
```
````

`internal/skillpack/templates/clarify.md`:

````markdown
---
name: verdi-clarify
description: Surface what a draft spec still leaves unproven — readiness concerns and unclaimed open questions — and propose one decision or one research stub at a time through mutate_draft, showing each proposal before it is written.
---

# verdi-clarify

Use this skill on a draft spec on its design branch when the user asks what is still unclear, unresolved, or blocking. It reads the rendered spec document and writes only through `mutate_draft`, one operation per call, and only after the human has seen the proposal.

## Steps

1. Call `get_design_context` with the draft's ref (`spec/<slug>`). Keep `identity` (`checkout`, `branch`, `head`): every `mutate_draft` call carries exactly those three as `expected`. Then read the draft's bytes from `<checkout>/.verdi/specs/active/<slug>/spec.md`: `base_spec_b64` is their standard base64, and `base_digest` is `sha256:` followed by the lowercase hex SHA-256 of those exact bytes. Re-read them before every call; a stale base is refused.
2. Call `get_document` with `ref` `spec/<slug>`, `kind` `spec`, and `proposed` true (the working-tree draft on this branch, never the accepted bytes). Read three sections:
   - **Readiness.** Each concern is listed with its state, timing, blocking flag, summary, and witnesses. Collect every concern whose state is not proven. If the section says "Readiness was not supplied for this render.", say so to the human and continue with the next two sections.
   - **Open questions.** A question is unclaimed when its line ends with "unclaimed; blocks acceptance until a … claims it or a decision answers it." Collect them.
   - **Decisions.** Read them so a proposal never duplicates a ratified decision.
3. For each unclaimed question, in the document's order, prepare exactly one proposal:
   - a decision, when the human can answer now: `{"op":"add-decision","id":"<next dc-id>","text":"<the answer, with its rationale>","anchor":"<id>"}`; or
   - a research stub that claims the question, when an investigation is needed first: `{"op":"add-stub","slug":"<kebab-slug>","spike":true,"resolves":["<oq-id>"]}` (the store may display this class under its own word; use the word the document uses).
4. Show the human the proposal exactly as it will be sent: the question it answers or claims and the operation JSON. Ask for confirmation.
5. On confirmation, call `mutate_draft` with `harness` (your harness id), optional `session`, `schema` `verdi.draftmutation/v1`, `spec`, `base_digest`, `base_spec_b64`, `expected`, and `operations` holding that one operation. On a stale-base refusal, repeat from step 1; never resend against a guessed base.
6. Repeat steps 4–5 for the next question. Then, for each unproven readiness concern that a decision could settle, offer one `add-decision` the same way; a concern whose witness is missing evidence is reported, not decided.
7. Finish by listing what was written (operation, resulting id or slug) and what remains open.

## Never

Never batch several operations into one `mutate_draft` call in this skill. Never edit `.verdi/specs/…` directly. Never remove a question a decision has not answered.

```verdi-sequence
call get_design_context
call get_document kind=spec proposed=true
loop
show
confirm
call mutate_draft operations=1
end
```
````

`internal/skillpack/templates/plan.md`:

````markdown
---
name: verdi-plan
description: Read a spec's plan document and propose, one at a time through mutate_draft, a stub for each acceptance criterion nothing covers yet.
---

# verdi-plan

Use this skill when a draft spec has acceptance criteria that no stub covers and the user wants the plan filled in. The plan document is a projection of the spec's own `stubs` block; coverage is computed from it, never declared.

## Steps

1. Call `get_design_context` with `spec/<slug>`. Keep `identity` (`checkout`, `branch`, `head`): every `mutate_draft` call carries exactly those three as `expected`. Then read the draft's bytes from `<checkout>/.verdi/specs/active/<slug>/spec.md`: `base_spec_b64` is their standard base64, and `base_digest` is `sha256:` followed by the lowercase hex SHA-256 of those exact bytes. Re-read them before every call; a stale base is refused.
2. Call `get_document` with `ref` `spec/<slug>`, `kind` `plan`, and `proposed` true, and show the human the plan as it stands. The plan document lists only what is planned, so call `get_document` again with `kind` `spec` and `proposed` true: in its criteria section, an uncovered criterion's coverage line reads "not yet planned."; a covered one reads "covered by …"; "not computed for this render." means the facts were unavailable, in which case say so and stop. Collect the uncovered criterion ids in document order.
3. For each uncovered criterion, prepare exactly one stub: `{"op":"add-stub","slug":"<kebab-slug describing the deliverable>","acceptance_criteria":["<ac-id>"]}`. A stub may cover several criteria when they are one deliverable; say why.
4. Show the human the criterion text and the operation JSON. Ask for confirmation.
5. On confirmation, call `mutate_draft` with `harness` (your harness id), optional `session`, `schema` `verdi.draftmutation/v1`, `spec`, `base_digest`, `base_spec_b64`, `expected`, and `operations` holding that one operation. On a stale-base refusal, repeat from step 1; never resend against a guessed base.
6. When every criterion is covered, call `get_document` with `kind` `plan` and `proposed` true once more and show the human the plan section as it now reads.

```verdi-sequence
call get_design_context
call get_document kind=plan proposed=true
call get_document kind=spec proposed=true
loop
show
confirm
call mutate_draft operations=1
end
call get_document kind=plan proposed=true
```
````

`internal/skillpack/templates/tasks.md`:

````markdown
---
name: verdi-tasks
description: Read a spec's tasks document — plan, evidence, and readiness — and turn it into a work list for the human without writing anything to the store.
---

# verdi-tasks

Use this skill when the user asks what to work on next for a spec. It reads and never writes: no `mutate_draft`, no `add_annotation`, no import.

## Steps

1. Call `get_document` with `ref` `spec/<slug>` and `kind` `tasks`, adding `proposed` true when the spec is a draft on its design branch (omit it for an accepted spec).
2. From the plan section, list each stub with the criteria it covers. From the evidence section, note each criterion's evidence state and what is still unproven; if it says "Evidence was not supplied for this render.", say so and skip the evidence-based selections below. From the readiness section, list the concerns that need attention, blocking ones first; if readiness was not supplied for this render, say so.
3. Present a work list in this order: blocking readiness concerns; criteria whose evidence table row has "—" in its Detail column (nothing implements or evidences them yet); criteria whose Detail column names an unsatisfied evidence kind ("<kind> unsatisfied"); then everything else, keeping each criterion's State column word as the document prints it. Quote ids so the human can find each item on the board.
4. If the human wants any of it changed, hand off to verdi-clarify or verdi-plan; this skill writes nothing.

```verdi-sequence
call get_document kind=tasks proposed=true
show
```
````

- [ ] **Step 4: Run the host/engine tests**

Run: `go test ./internal/skillpack/ -run 'TestParseHosts|TestPath|TestEngineDigest|TestTemplateDigest' -count=1`
Expected: PASS.

- [ ] **Step 5: Write the failing render, write, check, and sequence tests**

```go
package skillpack

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderStampsAndGolden(t *testing.T) {
	update := os.Getenv("SKILLPACK_UPDATE_GOLDEN") == "1"
	for _, h := range Hosts() {
		for _, s := range Skills() {
			r, err := Render(h, s, "0123456789abcdef0123456789abcdef01234567")
			if err != nil {
				t.Fatalf("Render(%s,%s): %v", h, s, err)
			}
			if r.Path != Path(h, s) || r.Digest != contentDigest(r.Content) {
				t.Fatalf("Render(%s,%s) path/digest mismatch", h, s)
			}
			body := string(r.Content)
			for _, want := range []string{
				"---\nname: verdi-" + s + "\n",
				"<!-- verdi:generated-skill host=" + string(h) + " skill=" + s + " -->\n",
				"<!-- verdi:engine-digest " + EngineDigest() + " -->\n",
				"<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->\n",
				"<!-- verdi: this is a generated skill;",
				"```verdi-sequence\n",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("Render(%s,%s) lacks %q:\n%s", h, s, want, body)
				}
			}
			td, _ := TemplateDigest(s)
			if !strings.Contains(body, "<!-- verdi:template-digest "+td+" -->\n") {
				t.Fatalf("Render(%s,%s) lacks its template digest", h, s)
			}
			golden := filepath.Join("testdata", "golden", string(h), "verdi-"+s+".md")
			if update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, r.Content, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("golden %s: %v (run with SKILLPACK_UPDATE_GOLDEN=1 to create)", golden, err)
			}
			if !bytes.Equal(want, r.Content) {
				t.Fatalf("Render(%s,%s) differs from golden %s", h, s, golden)
			}
		}
	}
}

func TestRenderHostsDifferOnlyInMarker(t *testing.T) {
	for _, s := range Skills() {
		a, _ := Render(HostClaude, s, "none")
		b, _ := Render(HostCodex, s, "none")
		na := strings.Replace(string(a.Content), "host=claude", "host=X", 1)
		nb := strings.Replace(string(b.Content), "host=codex", "host=X", 1)
		if na != nb {
			t.Fatalf("skill %s: hosts differ beyond the marker", s)
		}
	}
}

func TestRenderRefusesUnknown(t *testing.T) {
	if _, err := Render(HostClaude, "deploy", "none"); err == nil {
		t.Fatal("unknown skill must refuse")
	}
	if _, err := Render(Host("cursor"), "specify", "none"); err == nil {
		t.Fatal("unknown host must refuse")
	}
	if _, err := Render(HostClaude, "specify", "HEAD"); err == nil {
		t.Fatal("render commit must be 40 hex or none")
	}
}

func TestRenderCommit(t *testing.T) {
	plain := t.TempDir()
	if got := RenderCommit(context.Background(), plain); got != "none" {
		t.Fatalf("RenderCommit outside git = %q, want none", got)
	}
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got := RenderCommit(context.Background(), repo); len(got) != 40 {
		t.Fatalf("RenderCommit inside git = %q, want 40 hex", got)
	}
}

func TestWriteThenCheckClean(t *testing.T) {
	root := t.TempDir()
	res, err := Write(context.Background(), root, Hosts())
	if err != nil {
		t.Fatal(err)
	}
	if res.RenderCommit != "none" || len(res.Files) != 8 {
		t.Fatalf("Write result = %+v", res)
	}
	for i := 1; i < len(res.Files); i++ {
		if res.Files[i-1].Path >= res.Files[i].Path {
			t.Fatalf("Write result not sorted by path: %v", res.Files)
		}
	}
	rep, err := Check(root, Hosts())
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Clean() || rep.Checked != 8 {
		t.Fatalf("Check after Write = %+v", rep)
	}
	// A second Write is byte-idempotent.
	res2, _ := Write(context.Background(), root, Hosts())
	for i := range res.Files {
		if res.Files[i] != res2.Files[i] {
			t.Fatalf("Write not idempotent at %v vs %v", res.Files[i], res2.Files[i])
		}
	}
}

func TestCheckFindings(t *testing.T) {
	root := t.TempDir()
	if _, err := Write(context.Background(), root, Hosts()); err != nil {
		t.Fatal(err)
	}
	// drift: an edit anywhere in the body
	p := filepath.Join(root, filepath.FromSlash(Path(HostClaude, "plan")))
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(b, []byte("\nhand edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	// missing: a deleted codex skill
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(Path(HostCodex, "tasks")))); err != nil {
		t.Fatal(err)
	}
	// a changed render-commit line alone is NOT drift (R-W3-2)
	q := filepath.Join(root, filepath.FromSlash(Path(HostCodex, "specify")))
	c, _ := os.ReadFile(q)
	c = bytes.Replace(c, []byte("<!-- verdi:render-commit none -->"), []byte("<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->"), 1)
	if err := os.WriteFile(q, c, 0o644); err != nil {
		t.Fatal(err)
	}
	// truncated file counts as drift
	r := filepath.Join(root, filepath.FromSlash(Path(HostClaude, "tasks")))
	d, _ := os.ReadFile(r)
	if err := os.WriteFile(r, d[:len(d)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Check(root, Hosts())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rep.Findings {
		got[f.Path] = f.Code
	}
	want := map[string]string{
		Path(HostClaude, "plan"):  "drift",
		Path(HostCodex, "tasks"):  "missing",
		Path(HostClaude, "tasks"): "drift",
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for p, code := range want {
		if got[p] != code {
			t.Fatalf("finding for %s = %q, want %q (all: %v)", p, got[p], code, got)
		}
	}
	for _, f := range rep.Findings {
		if f.Code == "drift" && (f.Expected == "" || f.Actual == "" || f.Expected == f.Actual) {
			t.Fatalf("drift finding must carry distinct expected/actual digests: %+v", f)
		}
	}
	// Findings are sorted by path.
	for i := 1; i < len(rep.Findings); i++ {
		if rep.Findings[i-1].Path >= rep.Findings[i].Path {
			t.Fatalf("findings not sorted: %v", rep.Findings)
		}
	}
	// Scoped to one host: the codex findings alone.
	rep2, _ := Check(root, []Host{HostCodex})
	if len(rep2.Findings) != 1 || rep2.Findings[0].Code != "missing" || rep2.Checked != 4 {
		t.Fatalf("Check(codex) = %+v", rep2)
	}
}

func TestParseSequence(t *testing.T) {
	for _, s := range Skills() {
		b, _ := Template(s)
		seq, err := ParseSequence(b)
		if err != nil {
			t.Fatalf("ParseSequence(%s): %v", s, err)
		}
		if len(seq) == 0 || seq[0].Kind != "call" {
			t.Fatalf("sequence for %s must open with a call: %+v", s, seq)
		}
	}
	got, err := ParseSequence([]byte("---\nname: x\n---\n\n```verdi-sequence\ncall get_document kind=plan\nloop\nshow\nconfirm\ncall mutate_draft operations=1\nend\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Sequence{
		{Kind: "call", Tool: "get_document", Args: map[string]string{"kind": "plan"}},
		{Kind: "loop"}, {Kind: "show"}, {Kind: "confirm"},
		{Kind: "call", Tool: "mutate_draft", Args: map[string]string{"operations": "1"}},
		{Kind: "end"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].Kind != want[i].Kind || got[i].Tool != want[i].Tool || len(got[i].Args) != len(want[i].Args) {
			t.Fatalf("step %d = %+v, want %+v", i, got[i], want[i])
		}
		for k, v := range want[i].Args {
			if got[i].Args[k] != v {
				t.Fatalf("step %d arg %s = %q, want %q", i, k, got[i].Args[k], v)
			}
		}
	}
	for _, bad := range []string{
		"no block at all",
		"```verdi-sequence\n```\n",                        // empty
		"```verdi-sequence\nfrobnicate x\n```\n",           // unknown kind
		"```verdi-sequence\ncall\n```\n",                   // call without tool
		"```verdi-sequence\nloop\nshow\n```\n",             // unclosed loop
		"```verdi-sequence\ncall a\n```\n```verdi-sequence\ncall b\n```\n", // two blocks
	} {
		if _, err := ParseSequence([]byte(bad)); err == nil {
			t.Fatalf("ParseSequence must refuse %q", bad)
		}
	}
}

func TestEveryWriteIsShownAndConfirmedFirst(t *testing.T) {
	// Static witness for ac-8's "shows the human every proposal before it
	// is written": in every template, each call to a write tool is
	// preceded, within the same loop body or at top level, by show then
	// confirm.
	writeTools := map[string]bool{"mutate_draft": true, "import_apply": true, "add_annotation": true}
	for _, s := range Skills() {
		b, _ := Template(s)
		seq, _ := ParseSequence(b)
		shown, confirmed := false, false
		for _, st := range seq {
			switch st.Kind {
			case "loop", "end":
				shown, confirmed = false, false
			case "show":
				shown = true
			case "confirm":
				confirmed = shown
			case "call":
				if writeTools[st.Tool] && !confirmed {
					t.Fatalf("skill %s calls %s without show+confirm before it", s, st.Tool)
				}
				if writeTools[st.Tool] && s == "tasks" {
					t.Fatalf("verdi-tasks must not call a write tool")
				}
			}
		}
	}
}
```

- [ ] **Step 6: Run to verify they fail**

Run: `go test ./internal/skillpack/ -count=1`
Expected: build failure (`Render`, `Write`, `Check`, `ParseSequence` undefined).

- [ ] **Step 7: Write `render.go`, `write.go`, `check.go`, `sequence.go`**

`internal/skillpack/render.go`:

```go
package skillpack

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/gitx"
)

// Rendered is one skill rendered for one host.
type Rendered struct {
	Host    Host
	Skill   string
	Path    string
	Content []byte
	Digest  string
}

var renderCommitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// noRenderCommit is the render-commit stamp value when the root is not
// inside a git repository. vocab:identity — stamp grammar (identity)
const noRenderCommit = "none"

// RenderCommit is the 40-hex HEAD of the repository containing root, or
// "none" when root is not inside a git repository (R-W3-7).
func RenderCommit(ctx context.Context, root string) string {
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil || !renderCommitRe.MatchString(head) {
		return noRenderCommit
	}
	return head
}

// Render renders skill for host: the embedded template with the
// generated-skill marker and the three stamps inserted immediately after
// the frontmatter's closing delimiter. renderCommit is 40 hex or "none".
func Render(h Host, skill, renderCommit string) (Rendered, error) {
	if h.dir() == "" {
		return Rendered{}, fmt.Errorf("skillpack: unknown host %q", h)
	}
	if renderCommit != noRenderCommit && !renderCommitRe.MatchString(renderCommit) {
		return Rendered{}, fmt.Errorf("skillpack: render commit %q must be 40 lowercase hex or %q", renderCommit, noRenderCommit)
	}
	tmpl, err := Template(skill)
	if err != nil {
		return Rendered{}, err
	}
	head, body, err := splitFrontmatter(tmpl)
	if err != nil {
		return Rendered{}, fmt.Errorf("skillpack: template %s: %w", skill, err)
	}
	var b bytes.Buffer
	b.Write(head)
	fmt.Fprintf(&b, "<!-- verdi:generated-skill host=%s skill=%s -->\n", h, skill)
	fmt.Fprintf(&b, "<!-- verdi:engine-digest %s -->\n", EngineDigest())
	fmt.Fprintf(&b, "<!-- verdi:template-digest %s -->\n", contentDigest(tmpl))
	fmt.Fprintf(&b, "<!-- verdi:render-commit %s -->\n", renderCommit)
	b.WriteString("<!-- verdi: this is a generated skill; edits here never change verdi, and any difference is reported as drift by `verdi harness check` until this file is regenerated with `verdi harness render`. -->\n")
	b.Write(body)
	content := b.Bytes()
	return Rendered{Host: h, Skill: skill, Path: Path(h, skill), Content: content, Digest: contentDigest(content)}, nil
}

var frontmatterClose = []byte("\n---\n")

// splitFrontmatter returns the frontmatter including its closing "---\n"
// line, and the body that follows. A template must open with "---\n" and
// carry a closing delimiter line.
func splitFrontmatter(tmpl []byte) (head, body []byte, err error) {
	if !bytes.HasPrefix(tmpl, []byte("---\n")) {
		return nil, nil, fmt.Errorf("template must open with a frontmatter delimiter")
	}
	i := bytes.Index(tmpl[3:], frontmatterClose)
	if i < 0 {
		return nil, nil, fmt.Errorf("template frontmatter has no closing delimiter")
	}
	cut := 3 + i + len(frontmatterClose)
	return tmpl[:cut], tmpl[cut:], nil
}
```

`internal/skillpack/write.go`:

```go
package skillpack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/atomicfile"
)

// FileDigest is one written file's repository-relative slash path and
// content digest.
type FileDigest struct {
	Path   string
	Digest string
}

// Result is what Write wrote, sorted by Path.
type Result struct {
	RenderCommit string
	Files        []FileDigest
}

// Write renders every skill for every host in hosts under root, writing
// each file atomically, and returns the sorted digests. It reads nothing
// from a store; root need only be an existing directory.
func Write(ctx context.Context, root string, hosts []Host) (Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return Result{}, fmt.Errorf("skillpack: root: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("skillpack: root %q is not a directory", root)
	}
	res := Result{RenderCommit: RenderCommit(ctx, root)}
	for _, h := range hosts {
		for _, s := range Skills() {
			r, err := Render(h, s, res.RenderCommit)
			if err != nil {
				return Result{}, err
			}
			full := filepath.Join(root, filepath.FromSlash(r.Path))
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return Result{}, fmt.Errorf("skillpack: %s: %w", r.Path, err)
			}
			if err := atomicfile.Write(full, r.Content, 0o644); err != nil {
				return Result{}, fmt.Errorf("skillpack: %s: %w", r.Path, err)
			}
			res.Files = append(res.Files, FileDigest{Path: r.Path, Digest: r.Digest})
		}
	}
	sort.Slice(res.Files, func(i, j int) bool { return res.Files[i].Path < res.Files[j].Path })
	return res, nil
}
```

`internal/skillpack/check.go`:

```go
package skillpack

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Finding is one skill that is absent or differs from what the running
// binary renders. Code is "missing" or "drift". Expected and Actual are
// content digests of the render-commit-masked bytes (empty for missing).
type Finding struct {
	Code     string
	Path     string
	Expected string
	Actual   string
}

// Report is Check's fail-closed result: Checked counts every (host,
// skill) pair examined; Findings is sorted by Path.
type Report struct {
	Findings []Finding
	Checked  int
}

// Clean reports zero findings.
func (r Report) Clean() bool { return len(r.Findings) == 0 }

var renderCommitLine = regexp.MustCompile(`(?m)^<!-- verdi:render-commit [^>]*-->\n`)

// maskRenderCommit removes the render-commit stamp line (R-W3-2).
func maskRenderCommit(b []byte) []byte { return renderCommitLine.ReplaceAll(b, nil) }

// Check recomputes every skill for hosts from the running binary and
// compares the on-disk bytes under root with the render-commit line
// masked on both sides.
func Check(root string, hosts []Host) (Report, error) {
	var rep Report
	for _, h := range hosts {
		for _, s := range Skills() {
			rep.Checked++
			want, err := Render(h, s, noRenderCommit)
			if err != nil {
				return Report{}, err
			}
			wantMasked := maskRenderCommit(want.Content)
			full := filepath.Join(root, filepath.FromSlash(want.Path))
			got, err := os.ReadFile(full)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				rep.Findings = append(rep.Findings, Finding{Code: "missing", Path: want.Path})
				continue
			case err != nil:
				return Report{}, fmt.Errorf("skillpack: %s: %w", want.Path, err)
			}
			gotMasked := maskRenderCommit(got)
			if !bytes.Equal(wantMasked, gotMasked) {
				rep.Findings = append(rep.Findings, Finding{Code: "drift", Path: want.Path, Expected: contentDigest(wantMasked), Actual: contentDigest(gotMasked)})
			}
		}
	}
	sort.Slice(rep.Findings, func(i, j int) bool { return rep.Findings[i].Path < rep.Findings[j].Path })
	return rep, nil
}
```

`internal/skillpack/sequence.go`:

```go
package skillpack

import (
	"bytes"
	"fmt"
	"strings"
)

// Step is one line of a template's verdi-sequence block.
//
// Grammar (one step per line, blank lines ignored):
//
//	call <tool> [key=value ...]   a tool call the skill makes
//	show                          the skill shows the human a proposal or result
//	confirm                       the skill waits for the human's confirmation
//	loop                          opens a body repeated once per item
//	end                           closes the body
type Step struct {
	Kind string
	Tool string
	Args map[string]string
}

// Sequence is a template's declared tool sequence (R-W3-6).
type Sequence []Step

var (
	fenceOpen  = []byte("```verdi-sequence\n")
	fenceClose = []byte("```\n")
)

// ParseSequence extracts and parses the single verdi-sequence block of a
// template. Zero or two blocks, an empty block, an unknown step, a call
// without a tool, or an unbalanced loop is an error.
func ParseSequence(template []byte) (Sequence, error) {
	start := bytes.Index(template, fenceOpen)
	if start < 0 {
		return nil, fmt.Errorf("skillpack: template has no verdi-sequence block")
	}
	rest := template[start+len(fenceOpen):]
	stop := bytes.Index(rest, fenceClose)
	if stop < 0 {
		return nil, fmt.Errorf("skillpack: verdi-sequence block is not closed")
	}
	if bytes.Contains(rest[stop+len(fenceClose):], fenceOpen) {
		return nil, fmt.Errorf("skillpack: template has more than one verdi-sequence block")
	}
	var seq Sequence
	depth := 0
	for _, line := range strings.Split(string(rest[:stop]), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "call":
			if len(fields) < 2 {
				return nil, fmt.Errorf("skillpack: verdi-sequence: call without a tool name")
			}
			st := Step{Kind: "call", Tool: fields[1], Args: map[string]string{}}
			for _, kv := range fields[2:] {
				k, v, ok := strings.Cut(kv, "=")
				if !ok || k == "" || v == "" {
					return nil, fmt.Errorf("skillpack: verdi-sequence: bad argument %q", kv)
				}
				st.Args[k] = v
			}
			seq = append(seq, st)
		case "show", "confirm":
			if len(fields) != 1 {
				return nil, fmt.Errorf("skillpack: verdi-sequence: %s takes no arguments", fields[0])
			}
			seq = append(seq, Step{Kind: fields[0]})
		case "loop":
			depth++
			seq = append(seq, Step{Kind: "loop"})
		case "end":
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("skillpack: verdi-sequence: end without loop")
			}
			seq = append(seq, Step{Kind: "end"})
		default:
			return nil, fmt.Errorf("skillpack: verdi-sequence: unknown step %q", fields[0])
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("skillpack: verdi-sequence: loop without end")
	}
	if len(seq) == 0 {
		return nil, fmt.Errorf("skillpack: verdi-sequence block is empty")
	}
	return seq, nil
}
```

- [ ] **Step 8: Create the goldens, run the package tests, race, vet, vocab**

Run: `SKILLPACK_UPDATE_GOLDEN=1 go test ./internal/skillpack/ -run TestRenderStampsAndGolden -count=1 && go test -race -count=1 ./internal/skillpack/ && go vet ./internal/skillpack/ && gofmt -l internal/skillpack && go test -count=1 ./internal/specalign/ -run TestVocabProseWitness`
Expected: all PASS; `gofmt -l` prints nothing; eight goldens under `internal/skillpack/testdata/golden/{claude,codex}/`.

- [ ] **Step 9: Commit**

```bash
git add internal/skillpack
git commit -m "Add skillpack: embedded verdi skills with stamps, drift check, and sequence grammar"
```

---

### Task 2: `verdi harness render|check`, registry, this repository's skills, and the gate

**Files:**
- Create: `cmd/verdi/harness.go`, `cmd/verdi/harness_test.go`
- Modify: `cmd/verdi/dispatch.go` (`verbPhase` + `usage` banner + one dispatch arm), `cmd/verdi/help.go` (`topLevelUsage` row, `verbUsage` entry), `internal/specalign/verbs_test.go` (`inV0` + package-doc paragraph), `internal/showcasealign/coverage_test.go` (`"cli:harness"` row), `CLAUDE.md` (verbs line), `Makefile` (`lint-store`)
- Create: `.claude/skills/verdi-{specify,clarify,plan,tasks}/SKILL.md`, `.agents/skills/verdi-{specify,clarify,plan,tasks}/SKILL.md` (rendered by the built binary, committed)

**Interfaces:**
- Consumes: Task 1's `skillpack.ParseHosts`, `Write`, `Check`, `store.FindRoot`, `store.RootAt`-style explicit root (use `os.Stat` directly; no store requirement per R-W3-7).
- Produces: the `harness` verb; `harnessUsage` constant reused by `help.go`.

- [ ] **Step 1: Write the failing built-binary tests**

```go
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestHarnessRenderAndCheck(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := t.TempDir()

	// Usage errors exit 2 before any root is resolved.
	for _, args := range [][]string{{"harness"}, {"harness", "frobnicate"}, {"harness", "render", "--host", "cursor", "-o", root}, {"harness", "render", "-o"}, {"harness", "check", "extra", "-o", root}} {
		code, _, stderr := runVerdi(t, bin, root, args...)
		if code != 2 || !strings.Contains(stderr, "usage: verdi harness") {
			t.Fatalf("%v: code %d stderr %q", args, code, stderr)
		}
	}

	// -o must exist.
	if code, _, stderr := runVerdi(t, bin, root, "harness", "render", "-o", filepath.Join(root, "nope")); code != 2 || !strings.Contains(stderr, "harness render:") {
		t.Fatalf("missing -o: code %d stderr %q", code, stderr)
	}

	// check before render: every skill missing, exit 1, one line per finding on stdout.
	code, stdout, _ := runVerdi(t, bin, root, "harness", "check", "-o", root)
	if code != 1 || strings.Count(stdout, "missing  ") != 8 {
		t.Fatalf("check before render: code %d stdout %q", code, stdout)
	}

	// render all: 8 sorted "<digest>  <path>" lines on stdout, exit 0.
	code, stdout, stderr := runVerdi(t, bin, root, "harness", "render", "-o", root)
	if code != 0 {
		t.Fatalf("render: code %d stderr %q", code, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 8 {
		t.Fatalf("render printed %d lines: %q", len(lines), stdout)
	}
	paths := make([]string, 0, len(lines))
	for i, l := range lines {
		digest, path, ok := strings.Cut(l, "  ")
		if !ok || !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 || path == "" {
			t.Fatalf("line %d %q is not '<digest>  <path>'", i, l)
		}
		paths = append(paths, path)
	}
	if !sort.StringsAreSorted(paths) {
		t.Fatalf("render output not sorted by path: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "verdi-clarify", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	// check clean: exit 0, empty stdout.
	if code, stdout, _ := runVerdi(t, bin, root, "harness", "check", "-o", root); code != 0 || stdout != "" {
		t.Fatalf("check clean: code %d stdout %q", code, stdout)
	}

	// --host codex only renders four; a later claude check still passes (untouched).
	code, stdout, _ = runVerdi(t, bin, root, "harness", "render", "--host", "codex", "-o", root)
	if code != 0 || strings.Count(stdout, "\n") != 4 {
		t.Fatalf("render codex: code %d stdout %q", code, stdout)
	}

	// drift: edit one file → exit 1, "drift  <path>".
	p := filepath.Join(root, ".claude", "skills", "verdi-plan", "SKILL.md")
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(b, []byte("edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = runVerdi(t, bin, root, "harness", "check", "-o", root)
	if code != 1 || strings.TrimSpace(stdout) != "drift  .claude/skills/verdi-plan/SKILL.md" {
		t.Fatalf("check drift: code %d stdout %q", code, stdout)
	}
	// scoped to codex the drift is invisible.
	if code, stdout, _ := runVerdi(t, bin, root, "harness", "check", "--host", "codex", "-o", root); code != 0 || stdout != "" {
		t.Fatalf("check codex after claude drift: code %d stdout %q", code, stdout)
	}
	// re-render repairs it.
	runVerdi(t, bin, root, "harness", "render", "-o", root)
	if code, _, _ := runVerdi(t, bin, root, "harness", "check", "-o", root); code != 0 {
		t.Fatalf("check after re-render: code %d", code)
	}
}

func TestHarnessDefaultsToStoreRoot(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)
	sub := filepath.Join(root, "cmd")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	// From a subdirectory, no -o: the store root is found by ancestor search.
	if code, _, stderr := runVerdi(t, bin, sub, "harness", "render"); code != 0 {
		t.Fatalf("render from subdir: code %d stderr %q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "verdi-tasks", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	// Inside a git repository the render commit is HEAD, not none.
	b, _ := os.ReadFile(filepath.Join(root, ".claude", "skills", "verdi-tasks", "SKILL.md"))
	if strings.Contains(string(b), "verdi:render-commit none") {
		t.Fatal("render inside a git repo must stamp HEAD")
	}
	if code, _, _ := runVerdi(t, bin, sub, "harness", "check"); code != 0 {
		t.Fatalf("check from subdir: code %d", code)
	}
	// Outside any store and without -o: operational error, exit 2.
	if code, _, stderr := runVerdi(t, bin, t.TempDir(), "harness", "check"); code != 2 || !strings.Contains(stderr, "harness check:") {
		t.Fatalf("no store: code %d stderr %q", code, stderr)
	}
}
```

`runVerdi` is `cmd/verdi/vocabulary_cli_test.go:81` `func runVerdi(t *testing.T, bin, dir string, args ...string) (int, string, string)`; `buildVerdiBinary` is `serve_integration_test.go:284`; `newIntegrationStoreRoot` is `serve_integration_test.go:313`.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./cmd/verdi/ -run TestHarness -count=1`
Expected: FAIL (`harness` is an unknown verb; usage banner, exit 2, for every invocation).

- [ ] **Step 3: Write `cmd/verdi/harness.go`**

```go
// verdi harness render|check [--host claude|codex|all] [-o <repo root>]
// (spec/spec-documents ac-7): renders the four verdi skills for one or
// both hosts from the templates embedded in this binary, or checks the
// rendered copies for drift. Both read nothing from a store: -o names an
// existing directory (no ancestor search); without it the store root is
// found from the working directory (R-W3-7). Kept in its own file per the
// context_project.go convention so dispatch.go's diff is one arm.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/skillpack"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage/flag grammar (identity)
const harnessUsage = "usage: verdi harness render|check [--host claude|codex|all] [-o <repo root>]"

func cmdHarness(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, harnessUsage)
		return 2
	}
	sub := args[0]
	if sub != "render" && sub != "check" {
		fmt.Fprintf(stderr, "harness: unknown subcommand %q\n%s\n", sub, harnessUsage)
		return 2
	}
	hostFlag, out, err := parseHarnessFlags(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "harness %s: %v\n%s\n", sub, err, harnessUsage)
		return 2
	}
	hosts, err := skillpack.ParseHosts(hostFlag)
	if err != nil {
		fmt.Fprintf(stderr, "harness %s: %v\n%s\n", sub, err, harnessUsage)
		return 2
	}
	root, err := harnessRoot(out)
	if err != nil {
		fmt.Fprintf(stderr, "harness %s: %v\n", sub, err)
		return 2
	}
	switch sub {
	case "render":
		res, err := skillpack.Write(context.Background(), root, hosts)
		if err != nil {
			fmt.Fprintf(stderr, "harness render: %v\n", err)
			return 2
		}
		var b strings.Builder
		for _, f := range res.Files {
			fmt.Fprintf(&b, "%s  %s\n", f.Digest, f.Path)
		}
		if _, err := io.WriteString(stdout, b.String()); err != nil {
			fmt.Fprintf(stderr, "harness render: writing output: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "harness render: wrote %d skill files under %s (render commit %s)\n", len(res.Files), root, res.RenderCommit)
		return 0
	default:
		rep, err := skillpack.Check(root, hosts)
		if err != nil {
			fmt.Fprintf(stderr, "harness check: %v\n", err)
			return 2
		}
		var b strings.Builder
		for _, f := range rep.Findings {
			fmt.Fprintf(&b, "%s  %s\n", f.Code, f.Path)
		}
		if _, err := io.WriteString(stdout, b.String()); err != nil {
			fmt.Fprintf(stderr, "harness check: writing output: %v\n", err)
			return 2
		}
		if !rep.Clean() {
			fmt.Fprintf(stderr, "harness check: %d of %d skill files drifted or are missing under %s; run `verdi harness render` to regenerate\n", len(rep.Findings), rep.Checked, root)
			return 1
		}
		fmt.Fprintf(stderr, "harness check: %d skill files match this binary under %s\n", rep.Checked, root)
		return 0
	}
}

// parseHarnessFlags accepts --host <v> | --host=<v> and -o <dir> | -o=<dir>,
// each at most once, no positionals.
func parseHarnessFlags(args []string) (host, out string, err error) {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, inline := a, "", false
		if k, v, ok := strings.Cut(a, "="); ok && (k == "--host" || k == "-o") {
			name, value, inline = k, v, true
		}
		switch name {
		case "--host", "-o":
			if seen[name] {
				return "", "", fmt.Errorf("%s given twice", name)
			}
			seen[name] = true
			if !inline {
				if i+1 >= len(args) {
					return "", "", fmt.Errorf("%s requires a value", name)
				}
				i++
				value = args[i]
			}
			if strings.TrimSpace(value) == "" {
				return "", "", fmt.Errorf("%s requires a value", name)
			}
			if name == "--host" {
				host = value
			} else {
				out = value
			}
		default:
			return "", "", fmt.Errorf("unexpected argument %q", a)
		}
	}
	return host, out, nil
}

// harnessRoot resolves -o (an existing directory, no store required) or,
// absent, the store root found from the working directory.
func harnessRoot(out string) (string, error) {
	if out != "" {
		info, err := os.Stat(out)
		if err != nil {
			return "", fmt.Errorf("-o %q: %w", out, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("-o %q is not a directory", out)
		}
		return out, nil
	}
	return store.FindRoot(".")
}
```

- [ ] **Step 4: Wire the registry**

In `cmd/verdi/dispatch.go`: add `"harness": 25, // spec/spec-documents ac-7 — generated, stamped, drift-checked skills for Claude Code and Codex` to `verbPhase`; append `harness` to the `usage` banner's verb list (after `experiment`); add the arm `if verb == "harness" { return cmdHarness(args[1:], os.Stdout, stderr) }` next to the `experiment` arm.

In `cmd/verdi/help.go`: add the `topLevelUsage` row `  harness      render or drift-check the verdi skills for Claude Code and Codex`; add `"harness": harnessUsage,` to `verbUsage` (reuse the constant; never duplicate the literal).

In `internal/specalign/verbs_test.go`: add `"harness"` to `inV0` and a package-doc paragraph in the file's changelog style: "`harness` (spec/spec-documents ac-7, wave 3, phase 25) renders and drift-checks the four verdi skills; a bare `verdi harness` fails on usage parsing alone (exit 2) BEFORE resolving a store root, so the inventory proof stays hermetic."

In `internal/showcasealign/coverage_test.go`: add the row `"cli:harness": {goE2E("internal/showcasealign/cli_showcase_test.go")},` in the `cli:` group, sorted with its neighbours, and add a harness case to `internal/showcasealign/cli_showcase_test.go` in the shape of the `cli:experiment`/`cli:context` cases (against the provisioned showcase store root: `harness check -o <root>` → exit 1 with eight `missing  ` lines; `harness render -o <root>` → exit 0, eight lines; `harness check -o <root>` → exit 0, empty stdout). A test that only names `examples/showcase` in a disclosure comment is the gap pattern `coverage_test.go:48-53` records as closed; the row must map to a test that drives the real binary against the real showcase store. Read `coverage_test.go:744-760` (`realCapabilities`) to confirm the key form.

In `CLAUDE.md` (repo), extend the "CLI verbs:" sentence: "…`journey` (GLG AC-1), and `harness` (spec/spec-documents ac-7 — rendered, drift-checked agent skills) are real too; …".

- [ ] **Step 5: Run the verb tests and the inventories**

Run: `go test ./cmd/verdi/ -run 'TestHarness|TestVerbUsageRegistry' -count=1 && go test -count=1 ./internal/specalign/ -run 'TestV0CLIVerbInventory|TestV1CLIVerbForms|TestVocabProseWitness' && go test -count=1 ./internal/showcasealign/ -run 'TestShowcaseCoverage'`
Expected: PASS.

- [ ] **Step 6: Render this repository's own skills and wire the gate**

Run from the worktree root: `go build -o .build/verdi ./cmd/verdi && .build/verdi harness render` — eight files appear under `.claude/skills/` and `.agents/skills/`. Then in `Makefile`, `lint-store` gains a fourth line and its comment a sentence:

```make
# `verdi harness check` (spec/spec-documents ac-7, wave 3) verifies that
# this repository's own rendered skills (.claude/skills/verdi-*/SKILL.md,
# .agents/skills/verdi-*/SKILL.md) match the templates embedded in the
# binary just built — the drift gate; regenerate with `verdi harness render`.
lint-store:
	go build -o $(LINT_STORE_BIN) ./cmd/verdi
	$(LINT_STORE_BIN) lint
	$(LINT_STORE_BIN) model check
	$(LINT_STORE_BIN) harness check
```

Run: `make lint-store` → prints `harness check: 8 skill files match this binary under …`, exit 0. Then the instruction-conformance gate over the real tree: `go test -count=1 ./internal/specalign/ -run 'Instruction'` → PASS (every `verdi <verb>` in the skills is recognized; no retired-ritual phrase). If a verb reference fails, fix the TEMPLATE in Task 1's package (never the rendered file), re-run `.build/verdi harness render`, and re-run both.

- [ ] **Step 7: Commit (two commits)**

```bash
git add cmd/verdi/harness.go cmd/verdi/harness_test.go cmd/verdi/dispatch.go cmd/verdi/help.go internal/specalign/verbs_test.go internal/showcasealign/coverage_test.go CLAUDE.md
git commit -m "Add verdi harness render and check over skillpack"
git add .claude/skills .agents/skills Makefile
git commit -m "Render this repository's verdi skills and gate them in lint-store"
```

---

### Task 3: MCP `import_preview` and `import_apply` (Tier 3, serialized registry)

**Files:**
- Create: `internal/mcpserve/tool_import.go`, `internal/mcpserve/tool_import_test.go`
- Modify: `internal/mcpserve/tooldefs.go` (two entries), `internal/mcpserve/server.go` (two arms), `internal/mcpserve/server_test.go` (19→21 + names), `cmd/verdi/serve_integration_test.go` (19→21), `internal/specalign/mcptools_test.go` (two names), `internal/showcasealign/coverage_test.go` (two `mcp:` rows), `internal/showcasealign/mcp_showcase_test.go` (two subtests), `internal/mcpserve/doc.go` (tool count prose), `README.md` and `docs/architecture-and-journeys.md` (tool lists: add the two tools; correct the count)

**Interfaces:**
- Consumes: `specimport.NewService().Preview(ctx, root, req)`, `.Apply(ctx, root, req, digest, actor)`, `specimport.DecodeRequest([]byte)`, `draftmutation.NewDelegatedAgent(harness, session)`, the sentinel table at `cmd/verdi/designimport.go:414-430` (copy its code vocabulary; do not import package main).
- Produces: tools `import_preview` `{request: object}` and `import_apply` `{harness, session?, preview_digest, request}`; the transcript proof (Task 5) calls them by these names and argument keys.

- [ ] **Step 1: Write the failing tests**

```go
package mcpserve

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specimport"
)

// importReadyRequest mirrors cmd/verdi/designimport_test.go's ready request
// over the same sample markdown (copy cmd/verdi/testdata/specimport/sample.md
// to internal/mcpserve/testdata/specimport/sample.md; fixtures live under
// testdata/ only).
func importReadyRequest(t *testing.T, slug string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "specimport", "sample.md"))
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"schema":  specimport.RequestSchema,
		"target":  map[string]any{"slug": slug, "class": "feature", "title": "Imported sample"},
		"format":  "markdown-v1",
		"primary": "brief",
		"sources": []map[string]any{{"id": "brief", "label": "brief.md", "data": base64.StdEncoding.EncodeToString(data)}},
		"retain_unmapped": true,
	}
}

func decodeText(t *testing.T, res map[string]any) (text string, isError bool) {
	t.Helper()
	content := res["content"].([]map[string]any)
	isErr, _ := res["isError"].(bool)
	return content[0]["text"].(string), isErr
}

func TestImportPreviewThenApply_RecordNamesHarnessAndSession(t *testing.T) {
	root := importFixtureStore(t) // a clean committed fixturegit store on main with an adopted draft-write policy: reuse internal/designapp/conformance_test.go's conformanceStore recipe WITHOUT the design/sample checkout (apply creates the branch itself)
	b := &Backend{Root: root}
	req := importReadyRequest(t, "imported-sample")

	raw, _ := json.Marshal(map[string]any{"request": req})
	text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw))
	if isErr {
		t.Fatalf("preview errored: %s", text)
	}
	var preview specimport.PreviewResult
	if err := json.Unmarshal([]byte(text), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.Ready || preview.Schema != specimport.PreviewResultSchema || len(preview.Digest) != 64 {
		t.Fatalf("preview = %+v", preview)
	}
	// Preview is read-only: no design branch yet.
	if _, err := os.Stat(filepath.Join(root, ".git", "refs", "heads", "design", "imported-sample")); err == nil {
		t.Fatal("preview must not create the design branch")
	}

	raw, _ = json.Marshal(map[string]any{"harness": "claude-code", "session": "s-42", "preview_digest": preview.Digest, "request": req})
	text, isErr = decodeText(t, b.ImportApply(context.Background(), raw))
	if isErr {
		t.Fatalf("apply errored: %s", text)
	}
	var result specimport.Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != specimport.StatusCreated || result.Branch != "design/imported-sample" || result.PreviewDigest != preview.Digest {
		t.Fatalf("apply = %+v", result)
	}
	view, err := specimport.ReadRecord(context.Background(), root, result.Branch, "imported-sample")
	if err != nil {
		t.Fatal(err)
	}
	if view.Record.Actor.Harness != "claude-code" || view.Record.Actor.Session != "s-42" {
		t.Fatalf("record actor = %+v, want harness claude-code session s-42", view.Record.Actor)
	}
	if !view.CurrentSpecMatches {
		t.Fatalf("record view = %+v", view)
	}
	// Retry with the same digest is already-created, never a second branch.
	text, isErr = decodeText(t, b.ImportApply(context.Background(), raw))
	if isErr || !strings.Contains(text, `"status":"already-created"`) {
		t.Fatalf("retry: isErr %v text %s", isErr, text)
	}
}

func TestImportApply_Refusals(t *testing.T) {
	root := importFixtureStore(t)
	b := &Backend{Root: root}
	req := importReadyRequest(t, "refused")
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing harness", map[string]any{"preview_digest": strings.Repeat("a", 64), "request": req}, "import_apply: harness is required"},
		{"bad digest shape", map[string]any{"harness": "codex", "preview_digest": "HEAD", "request": req}, "import_apply: preview_digest must be 64 lowercase hex characters"},
		{"stale digest", map[string]any{"harness": "codex", "preview_digest": strings.Repeat("a", 64), "request": req}, "import_apply: stale-preview:"},
		{"actor field refused", map[string]any{"harness": "codex", "preview_digest": strings.Repeat("a", 64), "request": req, "actor": "human"}, "import_apply: malformed arguments"},
		{"unknown request field", map[string]any{"harness": "codex", "preview_digest": strings.Repeat("a", 64), "request": func() map[string]any { r := importReadyRequest(t, "x"); r["candidate"] = "zz"; return r }()}, "import_apply: invalid-request:"},
	} {
		raw, _ := json.Marshal(tc.args)
		text, isErr := decodeText(t, b.ImportApply(context.Background(), raw))
		if !isErr || !strings.HasPrefix(text, tc.want) {
			t.Fatalf("%s: isErr %v text %q, want prefix %q", tc.name, isErr, text, tc.want)
		}
	}
}

func TestImportPreview_NotReadyIsAResultAndInvalidIsAnError(t *testing.T) {
	root := importFixtureStore(t)
	b := &Backend{Root: root}
	// retain_unmapped=false with unmapped prose → unresolved-coverage finding, ready:false, NOT isError (R-W3-5).
	req := importReadyRequest(t, "notready")
	req["retain_unmapped"] = false
	raw, _ := json.Marshal(map[string]any{"request": req})
	text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw))
	if isErr || !strings.Contains(text, `"ready":false`) || !strings.Contains(text, `"unresolved-coverage"`) {
		t.Fatalf("not-ready preview: isErr %v text %s", isErr, text)
	}
	// Invalid request → isError with the CLI's code.
	bad := importReadyRequest(t, "bad")
	bad["format"] = "docx"
	raw, _ = json.Marshal(map[string]any{"request": bad})
	text, isErr = decodeText(t, b.ImportPreview(context.Background(), raw))
	if !isErr || !strings.HasPrefix(text, "import_preview: unsupported-format:") && !strings.HasPrefix(text, "import_preview: invalid-request:") {
		t.Fatalf("invalid preview: isErr %v text %q", isErr, text)
	}
	// Oversize envelope → isError before any decode.
	huge := json.RawMessage(`{"request":{"pad":"` + strings.Repeat("x", specimport.MaxEnvelopeBytes+2) + `"}}`)
	if text, isErr := decodeText(t, b.ImportPreview(context.Background(), huge)); !isErr || !strings.Contains(text, "exceed") {
		t.Fatalf("oversize: isErr %v text %.80q", isErr, text)
	}
	// Dirty checkout → dirty-context.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(map[string]any{"request": importReadyRequest(t, "dirty")})
	if text, isErr := decodeText(t, b.ImportPreview(context.Background(), raw)); !isErr || !strings.HasPrefix(text, "import_preview: dirty-context:") {
		t.Fatalf("dirty: isErr %v text %q", isErr, text)
	}
}
```

`importFixtureStore` lives in `tool_import_test.go`: build the store with `fixturegit.Build` over the files `conformanceStore` uses (`internal/designapp/conformance_test.go:110-137`: `.verdi/verdi.yaml`, `.verdi/.gitignore`, the `internal/policyauthority/testdata/store` tree with `mode: draft-write`), plus a committed `README.md`; stay on `main`; return the symlink-resolved root. Copy `cmd/verdi/testdata/specimport/sample.md` to `internal/mcpserve/testdata/specimport/sample.md`.

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/mcpserve/ -run TestImport -count=1`
Expected: build failure (`ImportPreview`/`ImportApply` undefined).

- [ ] **Step 3: Write `tool_import.go`**

```go
package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/specimport"
)

// import_preview / import_apply (spec/spec-documents ac-9) wrap the frozen
// import contract (docs/superpowers/specs/2026-09-14-spec-import-contract.md)
// exactly as the CLI does: preview is read-only and takes no actor;
// apply mints a delegated-agent actor from harness/session (R-W3-4,
// SI-163: never a caller-supplied actor field) and recomputes the preview
// under the digest handshake. The record's actor.harness/actor.session
// are populated by specimport itself. Nothing here touches the request
// or result schemas.

type importPreviewArgs struct {
	Request json.RawMessage `json:"request"`
}

type importApplyArgs struct {
	Harness       string          `json:"harness"`
	Session       string          `json:"session,omitempty"`
	PreviewDigest string          `json:"preview_digest"`
	Request       json.RawMessage `json:"request"`
}

var importPreviewDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// importSentinel maps a specimport sentinel to the CLI's closed code
// vocabulary (cmd/verdi/designimport.go designImportSentinels); the code
// prefixes every isError text so a harness can match refusals by name.
type importSentinel struct {
	err  error
	code string
}

var importSentinels = []importSentinel{
	{specimport.ErrInvalidRequest, "invalid-request"},
	{specimport.ErrInvalidSource, "invalid-source"},
	{specimport.ErrUnsupportedFormat, "unsupported-format"},
	{specimport.ErrInvalidModel, "invalid-model"},
	{specimport.ErrIdentityUnavailable, "identity-unavailable"},
	{specimport.ErrAuthorityInvalid, "authority-invalid"},
	{specimport.ErrIOFailure, "io-failure"},
	{specimport.ErrUnresolved, "unresolved"},
	{specimport.ErrDirtyContext, "dirty-context"},
	{specimport.ErrStalePreview, "stale-preview"},
	{specimport.ErrTargetExists, "target-exists"},
	{specimport.ErrPolicyForbidden, "policy-forbidden"},
	{specimport.ErrActorForbidden, "actor-forbidden"},
	{specimport.ErrImportRecordMissing, "provenance-mismatch"},
	{specimport.ErrProvenanceMismatch, "provenance-mismatch"},
}

func importToolError(tool string, err error) map[string]any {
	for _, s := range importSentinels {
		if errors.Is(err, s.err) {
			return toolError(fmt.Sprintf("%s: %s: %s", tool, s.code, err.Error()))
		}
	}
	return toolError(fmt.Sprintf("%s: io-failure: %s", tool, err.Error()))
}

// decodeImportRequest re-canonicalizes the inner request object and hands
// it to specimport's own strict decoder (the contract's sole decoder).
func decodeImportRequest(tool string, raw json.RawMessage) (specimport.Request, map[string]any) {
	if len(raw) == 0 {
		return specimport.Request{}, toolError(tool + ": request is required")
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return specimport.Request{}, toolError(tool + ": malformed request: " + err.Error())
	}
	canon, err := canonjson.Marshal(generic)
	if err != nil {
		return specimport.Request{}, toolError(tool + ": malformed request: " + err.Error())
	}
	req, err := specimport.DecodeRequest(canon)
	if err != nil {
		return specimport.Request{}, importToolError(tool, err)
	}
	return req, nil
}

// ImportPreview implements import_preview: a read-only preview of an
// import request. A completed preview with blocking findings is a result
// (ready:false), not an error (R-W3-5).
func (b *Backend) ImportPreview(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) > specimport.MaxEnvelopeBytes {
		return toolError("import_preview: arguments exceed the 12 MiB import envelope")
	}
	var args importPreviewArgs
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("import_preview: malformed arguments: " + err.Error())
	}
	req, failure := decodeImportRequest("import_preview", args.Request)
	if failure != nil {
		return failure
	}
	preview, err := specimport.NewService().Preview(ctx, b.Root, req)
	if err != nil {
		return importToolError("import_preview", err)
	}
	return toolJSON(preview)
}

// ImportApply implements import_apply: recomputes the preview, refuses on
// a changed digest or any blocking finding, and publishes the design
// branch with the import record under a delegated-agent actor.
func (b *Backend) ImportApply(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) > specimport.MaxEnvelopeBytes {
		return toolError("import_apply: arguments exceed the 12 MiB import envelope")
	}
	var args importApplyArgs
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("import_apply: malformed arguments: " + err.Error())
	}
	if args.Harness == "" {
		return toolError("import_apply: harness is required")
	}
	if !importPreviewDigestRe.MatchString(args.PreviewDigest) {
		return toolError("import_apply: preview_digest must be 64 lowercase hex characters")
	}
	actor, err := draftmutation.NewDelegatedAgent(args.Harness, args.Session)
	if err != nil {
		return toolError("import_apply: " + err.Error())
	}
	req, failure := decodeImportRequest("import_apply", args.Request)
	if failure != nil {
		return failure
	}
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	result, err := specimport.NewService().Apply(ctx, b.Root, req, args.PreviewDigest, actor)
	if err != nil {
		return importToolError("import_apply", err)
	}
	return toolJSON(result)
}
```

Read `internal/mcpserve/backend.go:27-59` for `writeMu` (the existing per-Backend write mutex `mutate_draft`/`add_annotation` use); if `MutateDraft` does not take it, follow whatever `MutateDraft` does — `specimport.Apply` already holds the checkout-wide writer lock (`publish.go:54`).

- [ ] **Step 4: Register the tools and move the four pins**

`tooldefs.go` — two entries after `mutate_draft`'s:

```go
{
	"name": "import_preview",
	// vocab:identity — import contract request grammar (identity)
	"description": "Read-only preview of a spec import request (verdi.spec-import-request/v1) under the frozen import contract: returns the preview digest, fields with origins and byte-offset spans, per-source coverage, findings, and ready. A preview with blocking findings is returned with ready:false; nothing is written." + dataNeverInstructionsNote,
	"inputSchema": obj(map[string]any{
		"request": map[string]any{"type": "object", "description": "the verdi.spec-import-request/v1 object (schema, target, format, primary, sources, mappings, links, defer_statements, retain_unmapped)"},
	}, "request"),
},
{
	"name": "import_apply",
	// vocab:identity — import contract request grammar (identity)
	"description": "Applies a previewed spec import under the delegated-agent actor: recomputes the preview, refuses on a changed digest (stale-preview), a blocking finding (unresolved), or an existing target (target-exists), then publishes the design branch, the spec, and the import record naming this harness and session. Retrying the same request and digest returns already-created." + dataNeverInstructionsNote,
	"inputSchema": obj(map[string]any{
		"harness":        str("the calling harness's identifier (e.g. codex, claude-code)"),
		"session":        str("optional session identifier"),
		"preview_digest": str("the 64-hex digest the human confirmed from import_preview"),
		"request":        map[string]any{"type": "object", "description": "the exact request that produced the preview"},
	}, "harness", "preview_digest", "request"),
},
```

`server.go` switch: `case "import_preview": return s.Backend.ImportPreview(ctx, call.Arguments)` and `case "import_apply": return s.Backend.ImportApply(ctx, call.Arguments)`.

Pins: `internal/mcpserve/server_test.go:137` `19`→`21` and add the two names to `wantNames`; `cmd/verdi/serve_integration_test.go:490` `19`→`21`; `internal/specalign/mcptools_test.go:108-128` add `"import_preview", "import_apply"` after `"mutate_draft"`; `internal/showcasealign/coverage_test.go` add `"mcp:import_preview": {goE2E("internal/showcasealign/mcp_showcase_test.go")},` and the same for `mcp:import_apply`; `internal/showcasealign/mcp_showcase_test.go` add subtests that call each tool over `callMCPTool` against `provisionShowcaseStore` — preview must return `ready` (true or false, but a completed preview) and apply with a wrong digest must return `isError` with `stale-preview:` (a real refusal is a real proof of the handshake; no branch is created in the showcase store).

Docs: `internal/mcpserve/doc.go` tool count prose → the real count with the two new names; `README.md` and `docs/architecture-and-journeys.md` MCP tool lists gain the two tools (read `internal/showcasealign/readme_test.go` first — README blocks are gated).

- [ ] **Step 5: Run the tool tests, the pins, race, vet, vocab**

Run: `go test -race -count=1 ./internal/mcpserve/ && go test -count=1 ./internal/specalign/ -run 'TestMCPToolInventory|TestVocabProseWitness' && go test -count=1 ./internal/showcasealign/ && go test -count=1 ./cmd/verdi/ -run TestD3_ && go vet ./internal/mcpserve/ && gofmt -l internal/mcpserve`
Expected: PASS; `tools/list` returns 21.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserve internal/specalign/mcptools_test.go internal/showcasealign cmd/verdi/serve_integration_test.go README.md docs/architecture-and-journeys.md
git commit -m "Add import_preview and import_apply MCP tools over the import contract"
```

---

### Task 4: Readiness and the `proposed` argument reach `get_document` over MCP (R-W3-3, R-W3-9)

**Files:**
- Modify: `internal/mcpserve/backend.go` (`Readiness *readinesspilot.Snapshot`), `internal/mcpserve/tool_get_document.go` (pass `Readiness: b.Readiness`; the `proposed` argument), `internal/mcpserve/tooldefs.go` (the `get_document` entry's `proposed` property — an argument on an existing tool, not an inventory change), `cmd/verdi/serve.go:296` (`srv.Backend.Readiness = readiness`), `internal/mcpserve/tool_get_document_test.go`, `cmd/verdi/document_parity_e2e_test.go` (new arm)

**Interfaces:**
- Consumes: `specdocload.Request.Readiness`, `specdoc.WithReadiness` gating (Wave 2), `readinesspilot.Snapshot`, the parity test's board leg with `workbench.Deps{Readiness: &snap}` (`document_parity_e2e_test.go:147-245`).
- Produces: `Backend.Readiness`; `get_document` argument `proposed` (boolean, optional; the transcript replays of Task 5 pass `"proposed": true`).

- [ ] **Step 1: Write the failing tests**

In `internal/mcpserve/tool_get_document_test.go`, add:

```go
func TestGetDocument_ReadinessWhenSnapshotTargetsSpec(t *testing.T) {
	root := getDocumentFixtureStore(t) // the file's existing fixture helper
	snap := readinesspilot.Snapshot{ /* copy the minimal valid literal from cmd/verdi/document_parity_e2e_test.go's readiness arm, with TargetRef "spec/<the fixture spec>" */ }
	if err := snap.Validate(); err != nil {
		t.Fatal(err)
	}
	b := &Backend{Root: root, Readiness: &snap}
	raw, _ := json.Marshal(map[string]any{"ref": snap.TargetRef, "kind": "spec"})
	text, isErr := decodeText(t, b.GetDocument(context.Background(), raw))
	if isErr {
		t.Fatal(text)
	}
	if !strings.Contains(text, "## Readiness") || strings.Contains(text, "Readiness was not supplied for this render.") {
		t.Fatalf("readiness section not rendered from the snapshot:\n%s", text)
	}
	// A snapshot targeting another spec leaves the document untouched.
	other := snap
	other.TargetRef = "spec/other"
	b2 := &Backend{Root: root, Readiness: &other}
	text2, _ := decodeText(t, b2.GetDocument(context.Background(), raw))
	if !strings.Contains(text2, "Readiness was not supplied for this render.") {
		t.Fatalf("foreign snapshot must not leak:\n%s", text2)
	}
}

func TestGetDocument_ProposedRendersTheWorkingTreeDraft(t *testing.T) {
	root := getDocumentDraftStore(t) // internal/designapp/conformance_test.go:110-160's conformanceStore recipe: committed store on main, checked out on design/sample, an UNCOMMITTED draft at .verdi/specs/active/sample/spec.md that main does not carry
	b := &Backend{Root: root}
	// Accepted mode cannot see a draft main does not carry.
	raw, _ := json.Marshal(map[string]any{"ref": "spec/sample", "kind": "spec"})
	if text, isErr := decodeText(t, b.GetDocument(context.Background(), raw)); !isErr {
		t.Fatalf("accepted mode must refuse a draft absent from the default branch: %s", text)
	}
	// proposed:true renders the working tree, marked proposed.
	raw, _ = json.Marshal(map[string]any{"ref": "spec/sample", "kind": "spec", "proposed": true})
	text, isErr := decodeText(t, b.GetDocument(context.Background(), raw))
	if isErr {
		t.Fatal(text)
	}
	var res getDocumentResult
	if err := json.Unmarshal([]byte(text), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Proposed || !strings.Contains(res.Markdown, "Proposed, not accepted") {
		t.Fatalf("proposed render = %+v", res)
	}
	// proposed and commit together are refused by name.
	raw, _ = json.Marshal(map[string]any{"ref": "spec/sample", "kind": "spec", "proposed": true, "commit": strings.Repeat("a", 40)})
	if text, isErr := decodeText(t, b.GetDocument(context.Background(), raw)); !isErr || !strings.Contains(text, "proposed and commit") {
		t.Fatalf("proposed+commit: isErr %v text %q", isErr, text)
	}
	// An accepted spec with proposed:true renders unmarked (Proposed derived, R-W2-4).
	root2 := getDocumentFixtureStore(t)
	raw, _ = json.Marshal(map[string]any{"ref": acceptedFixtureRef(t, root2), "kind": "spec", "proposed": true})
	text, _ = decodeText(t, (&Backend{Root: root2}).GetDocument(context.Background(), raw))
	if strings.Contains(text, `"proposed":true`) {
		t.Fatalf("exact accepted bytes must not be marked proposed:\n%s", text)
	}
}
```

`acceptedFixtureRef` returns the ref the file's existing happy-path test renders; read `tool_get_document_test.go` and reuse its literal.

In `cmd/verdi/document_parity_e2e_test.go`, add a fourth test `TestDocumentParity_BoardAndMCPShareReadiness`: build the same `snap` targeting the rendered spec, the board leg via `workbench.Deps{Readiness: &snap}` and the MCP leg via `Backend{Root: root, Readiness: &snap}`; assert the two Markdown bodies are byte-identical AND contain a populated Readiness section; assert the CLI leg (no readiness) differs from them ONLY by that section (strip from `## Readiness` to the next `## ` heading or EOF on the board bytes and compare with the CLI bytes after the same strip — pin that the divergence is exactly the section).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/mcpserve/ -run 'TestGetDocument_Readiness|TestGetDocument_Proposed' -count=1 && go test ./cmd/verdi/ -run TestDocumentParity_BoardAndMCP -count=1`
Expected: FAIL (`Backend` has no `Readiness` field; `proposed` is an unknown argument).

- [ ] **Step 3: Implement**

`tool_get_document.go`: `getDocumentArgs` gains `Proposed bool `json:"proposed"``; after the commit guard, `if args.Proposed && commit != "" { return toolError("get_document: proposed and commit are mutually exclusive") }`; `mode = specdocload.ModeWorkingTree` when `args.Proposed`. `tooldefs.go`'s `get_document` entry gains `"proposed": boolean("render the serving checkout's working-tree bytes (a draft on its design branch) instead of the accepted bytes; the result's proposed flag is derived from the store, never from this argument; incompatible with commit")` — read `tooldefs.go:26` for `boolean`'s exact signature.

`backend.go`: add `Readiness *readinesspilot.Snapshot` with the doc comment "the startup readiness snapshot verdi serve built (nil when serving without --context-request or under standalone verdi mcp); get_document passes it to the loader, which uses it only when its TargetRef is the rendered spec (R-W3-3)". `tool_get_document.go:85`: `Readiness: b.Readiness` in the `specdocload.Request`. `cmd/verdi/serve.go:296`: after `srv := mcpserve.NewServer(root)`, `srv.Backend.Readiness = readiness` (the `runServe` parameter).

- [ ] **Step 4: Run, race, vet**

Run: `go test -race -count=1 ./internal/mcpserve/ -run TestGetDocument && VERDI_E2E_PORT_BASE=4490 go test -count=1 ./cmd/verdi/ -run 'TestDocumentParity' && go vet ./internal/mcpserve/ ./cmd/verdi/`
Expected: PASS (all parity arms).

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserve/backend.go internal/mcpserve/tool_get_document.go internal/mcpserve/tool_get_document_test.go cmd/verdi/serve.go cmd/verdi/document_parity_e2e_test.go
git commit -m "Pass the serve-time readiness snapshot and a proposed argument through get_document"
```

---

### Task 5: Hermetic transcript proofs for the four skills (ac-8)

**Files:**
- Create: `internal/skillpack/transcript_test.go`, `internal/skillpack/testdata/store/spec.md` (a draft spec with one unclaimed open question, one claimed question, one uncovered criterion, one covered criterion), `internal/skillpack/testdata/specimport/sample.md` (copy of `cmd/verdi/testdata/specimport/sample.md`)
- Modify (only if a template's prose or sequence proves wrong under replay): `internal/skillpack/templates/*.md` + goldens + this repository's rendered skills (`.build/verdi harness render` after `go build`)

**Interfaces:**
- Consumes: Task 1's `ParseSequence`, `Template`; `mcpserve.NewServer`, `mcpserve.ServeConn` (NDJSON), tool names `get_design_context`, `get_document`, `mutate_draft`, `import_preview`, `import_apply`; `draftmutation.ResolveCanonicalIdentity`, `draftmutation.DigestBytes` (as `internal/mcpserve/tool_mutate_draft_test.go:13-28` uses them); the fixture recipe of `internal/designapp/conformance_test.go:110-160`.
- Produces: nothing new in production; four passing transcript tests and, per skill, a `transcript-<skill>.ndjson` written under `t.TempDir()` and asserted (not committed).

- [ ] **Step 1: Write the transcript harness and the four tests**

```go
package skillpack_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/skillpack"
	"github.com/jyang234/verdi/internal/store"
)

// transcript is an ordered record of every tools/call a replay made,
// driven over mcpserve.ServeConn's real NDJSON framing (never a Backend
// method call), so the proof is a wire transcript.
type transcript struct {
	t     *testing.T
	srv   *mcpserve.Server
	calls []transcriptCall
}

type transcriptCall struct {
	Tool    string          `json:"tool"`
	Args    map[string]any  `json:"args"`
	IsError bool            `json:"is_error"`
	Text    string          `json:"text"`
}

func (tr *transcript) call(tool string, args map[string]any) (text string, isError bool) {
	tr.t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": len(tr.calls) + 1, "method": "tools/call", "params": map[string]any{"name": tool, "arguments": args}})
	var out bytes.Buffer
	if err := mcpserve.ServeConn(context.Background(), bytes.NewReader(append(req, '\n')), &out, tr.srv); err != nil {
		tr.t.Fatalf("ServeConn: %v", err)
	}
	var resp struct {
		Result struct {
			Content []struct{ Text string `json:"text"` } `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		tr.t.Fatalf("decoding response: %v\n%s", err, out.String())
	}
	text, isError = resp.Result.Content[0].Text, resp.Result.IsError
	tr.calls = append(tr.calls, transcriptCall{Tool: tool, Args: args, IsError: isError, Text: text})
	return text, isError
}

// assertFollows proves the recorded calls are exactly the template's
// declared call steps, in order, with loop bodies repeated n times.
func (tr *transcript) assertFollows(seq skillpack.Sequence, loopCount int) {
	tr.t.Helper()
	var want []skillpack.Step
	for i := 0; i < len(seq); i++ {
		st := seq[i]
		if st.Kind == "loop" {
			j := i + 1
			for ; seq[j].Kind != "end"; j++ {
			}
			body := seq[i+1 : j]
			for n := 0; n < loopCount; n++ {
				for _, b := range body {
					if b.Kind == "call" {
						want = append(want, b)
					}
				}
			}
			i = j
			continue
		}
		if st.Kind == "call" {
			want = append(want, st)
		}
	}
	if len(want) != len(tr.calls) {
		tr.t.Fatalf("transcript has %d calls, the template declares %d: %+v", len(tr.calls), len(want), tr.calls)
	}
	for i := range want {
		if tr.calls[i].Tool != want[i].Tool {
			tr.t.Fatalf("call %d is %s, template declares %s", i, tr.calls[i].Tool, want[i].Tool)
		}
		if k := want[i].Args["kind"]; k != "" && tr.calls[i].Args["kind"] != k {
			tr.t.Fatalf("call %d kind %v, template declares %s", i, tr.calls[i].Args["kind"], k)
		}
		if p := want[i].Args["proposed"]; p != "" && fmt.Sprint(tr.calls[i].Args["proposed"]) != p {
			tr.t.Fatalf("call %d proposed %v, template declares %s", i, tr.calls[i].Args["proposed"], p)
		}
		if n := want[i].Args["operations"]; n != "" {
			ops, _ := tr.calls[i].Args["operations"].([]map[string]any)
			if fmt.Sprint(len(ops)) != n {
				tr.t.Fatalf("call %d carries %d operations, template declares %s", i, len(ops), n)
			}
		}
	}
}

func (tr *transcript) write(dir, skill string) {
	tr.t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, c := range tr.calls {
		if err := enc.Encode(c); err != nil {
			tr.t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "transcript-"+skill+".ndjson"), b.Bytes(), 0o644); err != nil {
		tr.t.Fatal(err)
	}
}

const draftSpecName = "sample"

// draftStore is internal/designapp/conformance_test.go's conformanceStore
// recipe: a committed store with the draft-write policy adopted, checked
// out on design/sample, with testdata/store/spec.md as the working-tree
// draft. It also commits README.md so the import arm's preview sees a clean
// tracked checkout on main BEFORE the design branch checkout (the specify
// replay runs against a second store built by importStore).
func draftStore(t *testing.T) string { /* copy conformanceStore verbatim, replacing conformanceSpec with the bytes of testdata/store/spec.md */ }

func importStore(t *testing.T) string { /* the same files, no branch checkout, no draft: apply creates design/<slug> itself */ }

func sequenceFor(t *testing.T, skill string) skillpack.Sequence {
	t.Helper()
	tmpl, err := skillpack.Template(skill)
	if err != nil {
		t.Fatal(err)
	}
	seq, err := skillpack.ParseSequence(tmpl)
	if err != nil {
		t.Fatal(err)
	}
	return seq
}

func mutateArgs(t *testing.T, root string, op map[string]any) map[string]any {
	t.Helper()
	base, err := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := draftmutation.ResolveCanonicalIdentity(context.Background(), root, "spec/"+draftSpecName, draftmutation.GitIdentityReader{})
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{
		"harness": "claude-code", "session": "transcript",
		"schema": draftmutation.RequestSchema, "spec": "spec/" + draftSpecName,
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   map[string]any{"checkout": identity.Checkout, "branch": identity.Branch, "head": identity.Head},
		"operations": []map[string]any{op},
	}
}

func TestTranscript_Specify(t *testing.T) {
	root := importStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "specify")
	data, _ := os.ReadFile(filepath.Join("testdata", "specimport", "sample.md"))
	req := map[string]any{
		"schema": "verdi.spec-import-request/v1", "format": "markdown-v1", "primary": "brief",
		"target":  map[string]any{"slug": "imported", "class": "feature", "title": "Imported"},
		"sources": []map[string]any{{"id": "brief", "label": "brief.md", "data": base64.StdEncoding.EncodeToString(data)}},
		"retain_unmapped": true,
	}
	text, isErr := tr.call("import_preview", map[string]any{"request": req})
	if isErr {
		t.Fatal(text)
	}
	var preview struct {
		Digest string `json:"digest"`
		Ready  bool   `json:"ready"`
	}
	json.Unmarshal([]byte(text), &preview)
	if !preview.Ready {
		t.Fatalf("preview not ready: %s", text)
	}
	// The human sees the digest (show, confirm) — then apply with THAT digest.
	text, isErr = tr.call("import_apply", map[string]any{"harness": "claude-code", "session": "transcript", "preview_digest": preview.Digest, "request": req})
	if isErr || !strings.Contains(text, `"status":"created"`) {
		t.Fatalf("apply: %v %s", isErr, text)
	}
	tr.assertFollows(seq, 0)
	tr.write(t.TempDir(), "specify")
	// Handshake witness: a different digest is refused and writes nothing new.
	tr2 := &transcript{t: t, srv: mcpserve.NewServer(importStore(t))}
	if text, isErr := tr2.call("import_apply", map[string]any{"harness": "claude-code", "preview_digest": strings.Repeat("0", 64), "request": req}); !isErr || !strings.Contains(text, "stale-preview") {
		t.Fatalf("stale digest must refuse: %v %s", isErr, text)
	}
}

func TestTranscript_Clarify(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "clarify")
	if text, isErr := tr.call("get_design_context", map[string]any{"ref": "spec/" + draftSpecName}); isErr {
		t.Fatal(text)
	}
	text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if isErr {
		t.Fatal(text)
	}
	if !strings.Contains(text, "Readiness was not supplied for this render.") {
		t.Fatalf("standalone server must disclose absent readiness:\n%s", text)
	}
	// Exactly one unclaimed question in the fixture: oq-2 (oq-1 is claimed by the fixture's spike stub).
	if strings.Count(text, "unclaimed; blocks acceptance") != 1 || !strings.Contains(text, "oq-2") {
		t.Fatalf("fixture must render one unclaimed question:\n%s", text)
	}
	// One proposal, shown and confirmed, then written: a research stub claiming oq-2.
	spike := true
	if text, isErr := tr.call("mutate_draft", mutateArgs(t, root, map[string]any{"op": "add-stub", "slug": "answer-oq-2", "spike": spike, "resolves": []string{"oq-2"}})); isErr {
		t.Fatal(text)
	}
	tr.assertFollows(seq, 1)
	tr.write(t.TempDir(), "clarify")
	// The document now shows the question claimed.
	text, _ = tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if strings.Contains(text, "unclaimed; blocks acceptance") {
		t.Fatalf("oq-2 still unclaimed after the stub:\n%s", text)
	}
}

func TestTranscript_Plan(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "plan")
	tr.call("get_design_context", map[string]any{"ref": "spec/" + draftSpecName})
	if text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "plan", "proposed": true}); isErr || !strings.Contains(text, "## Plan") {
		t.Fatalf("plan document: %v\n%s", isErr, text)
	}
	text, _ := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "spec", "proposed": true})
	if strings.Count(text, "not yet planned.") != 1 || !strings.Contains(text, "ac-2") {
		t.Fatalf("fixture must render exactly one uncovered criterion (ac-2):\n%s", text)
	}
	if text, isErr := tr.call("mutate_draft", mutateArgs(t, root, map[string]any{"op": "add-stub", "slug": "cover-ac-2", "acceptance_criteria": []string{"ac-2"}})); isErr {
		t.Fatal(text)
	}
	text, _ = tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "plan", "proposed": true})
	if !strings.Contains(text, "cover-ac-2") {
		t.Fatalf("plan document does not list the new stub:\n%s", text)
	}
	tr.assertFollows(seq, 1)
	tr.write(t.TempDir(), "plan")
}

func TestTranscript_Tasks(t *testing.T) {
	root := draftStore(t)
	tr := &transcript{t: t, srv: mcpserve.NewServer(root)}
	seq := sequenceFor(t, "tasks")
	before, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	text, isErr := tr.call("get_document", map[string]any{"ref": "spec/" + draftSpecName, "kind": "tasks", "proposed": true})
	if isErr || !strings.Contains(text, "## Plan") || !strings.Contains(text, "## Readiness") {
		t.Fatalf("tasks document: %v\n%s", isErr, text)
	}
	tr.assertFollows(seq, 0)
	for _, c := range tr.calls {
		if c.Tool == "mutate_draft" || c.Tool == "add_annotation" || c.Tool == "import_apply" {
			t.Fatalf("verdi-tasks called a write tool: %s", c.Tool)
		}
	}
	after, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, draftSpecName))
	if !bytes.Equal(before, after) {
		t.Fatal("verdi-tasks changed the draft")
	}
	out, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	if strings.TrimSpace(string(out)) != "?? "+filepath.ToSlash(strings.TrimPrefix(store.SpecPath(root, store.ZoneActive, draftSpecName), root+"/")) && strings.TrimSpace(string(out)) != "" {
		// the draft itself is untracked in this fixture; nothing else may appear
		t.Fatalf("verdi-tasks left the checkout changed:\n%s", out)
	}
	tr.write(t.TempDir(), "tasks")
}
```

`testdata/store/spec.md` — a valid draft (run `verdi lint` shape rules from `internal/artifact` by reading `internal/designapp/conformance_test.go`'s `conformanceSpec` literal and adapting it): two criteria `ac-1`, `ac-2`; one stub `{ slug: do-ac-1, acceptance_criteria: [ac-1] }`; two open questions `oq-1`, `oq-2`; one spike stub `{ slug: probe-oq-1, spike: true, resolves: [oq-1] }`; a `## Problem`, `## Outcome`, and body sections for every id. `ac-2` uncovered and `oq-2` unclaimed are the two facts the replays find.

- [ ] **Step 2: Run to verify they fail, then make them pass**

Run: `go test ./internal/skillpack/ -run TestTranscript -count=1 -v`
Expected first: FAIL on fixture assembly or wording; adjust the FIXTURE (never the assertions' meaning) until each replay follows its template. If a template's sequence is wrong under replay (e.g. `plan` needs a second `get_document`), fix the template, regenerate goldens (`SKILLPACK_UPDATE_GOLDEN=1 …`), rebuild and re-render this repository's skills (`go build -o .build/verdi ./cmd/verdi && .build/verdi harness render`), and re-run `make lint-store`.

- [ ] **Step 3: Full package checks**

Run: `go test -race -count=1 ./internal/skillpack/ && go vet ./internal/skillpack/ && gofmt -l internal/skillpack && go test -count=1 ./internal/specalign/ -run 'TestVocabProseWitness|Instruction|TestGateCacheHonesty'`
Expected: PASS. (`internal/skillpack` does not exec the binary, so `CROSS_BINARY_PKGS` is untouched.)

- [ ] **Step 4: Commit**

```bash
git add internal/skillpack .claude/skills .agents/skills
git commit -m "Prove each verdi skill's tool sequence with a hermetic MCP transcript"
```

---

### Task 6: Wave gate

- [ ] **Step 1: Static gates**

Run: `go build ./... && gofmt -l . && go vet ./... && golangci-lint run ./...`
Expected: clean.

- [ ] **Step 2: Full gate**

Run from the worktree root: `VERDI_E2E_PORT_BASE=4390 make verify`
Expected: `verify OK`, exit 0; `git status --porcelain` empty; `find e2e -name '*.png' -o -name '*.webm' -o -name 'trace.zip' | grep -v node_modules` prints nothing.

- [ ] **Step 3: Wave report**

Write `docs/superpowers/reports/2026-09-18-spec-documents-wave-3.md` in the evidence format (Status; Risk tier 3 for ac-9, 2 for the rest; Base..Head; Commits; Files changed; Contract implemented per ac-7, ac-8, ac-9 and the oq-1 resolution; Explicit exclusions; RED/GREEN; Reviewer verdict; Residual risks; Integration prerequisites), then:

```bash
git add docs/superpowers/reports/2026-09-18-spec-documents-wave-3.md
git commit -m "Report spec-documents wave 3: harness skills, import MCP tools, transcripts"
```

---

## Self-review

**Spec coverage.** ac-7: Task 1 (templates in the binary, three stamps, generated marker, both hosts), Task 2 (`harness render [--host] [-o]`, `harness check` exit 1 on drift, `make verify` via `lint-store`). ac-8: Task 1's templates carry each skill's behaviour and sequence; Task 5 proves each sequence by a hermetic MCP transcript (specify: preview → show → confirm → apply with the confirmed digest and a stale-digest refusal; clarify: readiness disclosure, unclaimed questions, one operation per `mutate_draft`; plan: uncovered criteria, one stub per call; tasks: reads only, store bytes unchanged); the static test `TestEveryWriteIsShownAndConfirmedFirst` pins "shown before written" in every template. ac-9: Task 3 (two tools over `specimport.Preview/Apply`, delegated-agent actor from `harness`/`session`, digest handshake, record naming harness and session, schemas untouched), Task 4 gives `clarify` its readiness source. oq-1: Task 0 records SI-201; R-W3-1 pins the Codex location and marker. Drafts over MCP: R-W3-9/SI-202 (`get_document proposed`), found by the preflight scan. co-3: the only new write tool is `import_apply`. dc-5: no import schema changes.

**Placeholders.** Three helper bodies in Task 3 and Task 5 (`importFixtureStore`, `draftStore`, `importStore`) are specified by pointing at the exact existing recipe to copy (`internal/designapp/conformance_test.go:110-160`) with the one substitution named; the readiness snapshot literal in Task 4 points at the Wave 2 parity test's existing literal. Everything else carries its code.

**Type consistency.** `skillpack.ParseHosts/Write/Check/Render/RenderCommit/Template/ParseSequence/Sequence/Step` (Task 1) are what Task 2 and Task 5 call. `Backend.ImportPreview/ImportApply` (Task 3) are what `server.go`'s arms and Task 5's replays call by tool name `import_preview`/`import_apply` with keys `request`, `harness`, `session`, `preview_digest`. `Backend.Readiness` (Task 4) is what `serve.go` sets and `tool_get_document.go` reads; `get_document`'s `proposed` argument (Task 4) is what the clarify/plan/tasks sequences declare (`proposed=true`) and Task 5's replays pass. The `verdi-sequence` blocks in the four templates are the exact sequences `assertFollows` compares.

**Known limits carried.** The transcript proves the tool sequence, not the agent's prose behaviour; the static show/confirm test pins the declared ordering. `clarify` under standalone `verdi mcp` sees no readiness (disclosed, R-W3-3). Codex's `.codex/skills` root is source-proven but doc-implicit; the plan renders only `.agents/skills` (SI-201). The instruction-conformance gate scans this repository's rendered skills; a consuming repository's own gates are its own.
