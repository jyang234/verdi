# verdi

Ground rules: `../CLAUDE.md`. Build contract, phases, exit criteria,
invention ledger: `../PLAN.md`. Read both before writing code here.

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

CLI verbs: 05 §CLI's table plus invented `gate` (I-7) and `board` (I-20) are real v0 verbs; `close`/`gc`/`waivers`/`verify-artifact` are recognized but out of v0 scope.
