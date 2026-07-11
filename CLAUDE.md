# verdi

Ground rules: `../CLAUDE.md`. Build contract, phases, exit criteria,
invention ledger: `../PLAN.md`. Read both before writing code here.

## Make targets

- `make verify` — the full gate: build, fmt-check, vet, lint, test, fixture.
- `make build` / `make test` / `make vet` / `make fmt-check` / `make fmt`
- `make lint` — golangci-lint if installed, else a non-failing warning.
- `make fixture` — the fixturegit determinism test package.
- `make tidy` — `go mod tidy`.
