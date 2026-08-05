# verdi

Ground rules: `../CLAUDE.md`. Build contract, phases, exit criteria:
`../PLAN.md`; its invention-ledger recording role now has the
repository-visible successor named below. Read both before writing code
here.

## Make targets

- `make verify` — full gate: build, fmt-check, vet, lint, test, fixture, lint-store, spec-align, e2e (last, slowest). Missing node/npm HARD-FAILS verify's e2e step — no silent skip.
- `make build` / `make test` / `make vet` / `make fmt-check` / `make fmt`
- `make lint` — golangci-lint if installed, else a non-failing warning.
- `make fixture` — fixturegit + corpus + svcfixcanned determinism tests.
- `make fixture-regen` — re-captures testdata/svcfix-canned/ from the real toolchain; opt-in, non-hermetic, never part of verify.
- `make lint-store` — builds the binary, self-lints this repo's own `.verdi/specs/active/` store.
- `make spec-align` — internal/specalign: self-hosted spec fidelity, v0 checklist audit, MCP tool + CLI verb inventories.
- `make e2e` — the Playwright suite (e2e/) alone.
- `make tidy` — `go mod tidy`.

## Change-lane routing

- Apply Superpowers workflows proportionally. Before invoking one, determine
  whether its extra stages will materially improve correctness, expose a
  consequential unknown, or reduce implementation risk. Use the smallest
  useful workflow. Do not manufacture a design document, implementation plan,
  review chain, or subagent handoff for an already-decided mechanical edit.
  Each workflow artifact must carry useful decisions or evidence; otherwise it
  is ceremony and must be omitted. Direct user and system requirements still
  apply.
- **Spec-only authority work is controller-authored.** If a change is limited
  to specifications, plans, authority records, and the invention ledger, the
  main controller writes and repairs it directly. Do not launch a Claude
  producer or fixer to copy, synthesize, or reword Markdown. Mechanical
  specification or documentation edits may remain single-agent.
- Use Claude implementers for implementation-heavy changes where a separate
  Codex review is useful: runtime code, schemas, tooling, tests, workflows, or
  UI. Split mixed authority/runtime changes unless they are genuinely
  inseparable. Claude's permitted read-only review of Codex-authored
  substantial specifications is a narrow exception to this implementation
  assignment; it does not authorize Claude specification production or repair.
- Substantial spec-only authoring receives exactly one independent cross-model
  review of the consolidated exact head: Claude reviews Codex-authored work,
  and Codex reviews Claude-authored work. The reviewer is read-only and
  supplies the fresh challenge. The authoring controller retains authority
  interpretation, adjudicates every finding, authors every accepted
  correction, runs verification, and produces the final handoff.
- After at most one author correction pass, the same independent reviewer
  performs one closure check. Do not start an automatic third review round or
  a reviewer/fixer chain. A blocking finding must cite binding authority, show
  a reachable conforming state and its concrete incorrect result, and remain
  inside the declared threat model. Wording preferences, out-of-model
  interference, and optional hardening are non-blocking; any later concern
  returns to the controller for explicit adjudication.
- Every canonical promotion includes a source-coverage/losslessness witness
  that maps all source authority to its destination, names each transformation
  or intentional omission, and reports the coverage total.

CLI verbs: 05 §CLI's table plus invented `gate` (I-7), `board` (I-20), and `audit` (R4-I-10) are real verbs; `close` (round 6, spec/close-verb) and `gc` (round 6, spec/worktree-manager — managed-worktree reclamation) are real too; only `waivers`/`verify-artifact` remain recognized but out of scope.

## Successor authorities

- Orchestration for the four-feature program is
  `docs/superpowers/plans/2026-08-01-four-feature-orchestration.md`; scoped
  to that program only, it does not override the binding specs in
  `docs/design/specs/00..05-*.md` or the v0 instructions above.
- `docs/superpowers/invention-ledger.md` succeeds `../PLAN.md` §7's
  recording role only, not its build-contract role — `../PLAN.md` remains
  the historical v0 build contract at the workspace root. The ledger
  carries a byte-exact, never-edited historical import of the PLAN/PLAN-V1
  §7 invention history; new entries go in its successor section under the
  `SI-<n>` namespace.
- Recording rule: a spec ambiguity changing proof meaning, authority,
  lifecycle state, or a public interface gets a ledger entry and blocks
  implementation until recorded — never resolved silently and never from
  what similar tools do.
- Three-valued honesty: every completion claim is proven,
  violated-with-witness, or disclosed-as-unproven; silence is never a pass.
- Every verb exits 0 (clean) / 1 (verdict) / 2 (operational).
- The CLI-verb and MCP-tool inventories are serialized shared registries:
  any change to either is isolated and serialized, never made concurrently
  across feature lanes.
