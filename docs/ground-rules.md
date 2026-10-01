# Verdi ground rules

The Go-style, testing, and commit rules for building Verdi. This file is their one statement: the workspace `CLAUDE.md` and
`AGENTS.md` point here instead of carrying their own copies, and it lives in the repository so that CI can check the rules
against what enforces them (process-hardening plan D-PH-3). Every rule binds every change to this repository.

## Go style

- Effective Go, `gofmt`-clean, `golangci-lint`-clean. No cgo.
- Single responsibility: one package = one concern, one file ≈ one topic.
  No god packages — split before a package accumulates a second concern.
- Anything used by two or more packages lives in a shared `internal/` package
  (schemas/decoding in `internal/artifact`); never copy-paste across packages.
- Accept interfaces, return structs; define interfaces at the consumer
  (the 04 §port pattern). `-er` names for single-method interfaces.
- `context.Context` is the first parameter of anything that blocks or does
  I/O; never stored in a struct.
- Errors: wrap with `%w` and context; distinguish verdict failures from
  operational errors — every verb exits 0 (clean) / 1 (verdict) / 2 (operational).
- Strict decode everywhere: YAML via the single `internal/artifact` seam
  (`KnownFields(true)` + dialect rejection of anchors/aliases/tags); JSON via
  `DisallowUnknownFields` + trailing-data rejection; unknown enum values fail
  closed.
- Deterministic outputs: canonical JSON (sorted keys, no HTML escaping,
  trailing newline). No wall-clock or randomness in generated artifacts
  except declared stamps.
- Design zero values to be useful; otherwise provide a constructor.
- No package-level mutable state: a package-level variable is only a
  sentinel error, a compiled regular expression, an embedded file, or a
  blank (`_`) compile-time check. A fixed table is a function or a constant,
  and state lives in values passed explicitly.

## Testing rules (each one is a merge blocker)

- Every function: happy-path **and** negative-path unit tests, table-driven.
- Every integration (git, upstream CLIs, forge API, Jira, MCP socket, file
  locking): an integration test against hermetic fakes — `httptest`,
  `fixturegit`, canned JSON. **No network in any test.**
- Every browser-facing behavioral path (workbench pages, board, dex output):
  a Playwright e2e test under `e2e/`. CLI behavioral paths: end-to-end
  Go tests driving the built binary.
- Fixtures are committed and deterministic (fixturegit stable SHAs, digest
  ratchets). `testdata/` is the only home for fixtures.

## Commits

- Every commit builds: each commit passes `make build` on its own, so any
  commit in a series can be checked out, bisected, or reverted alone.
- Never a bare `git stash`: it hides work in an unnamed entry that another
  lane or a later session cannot attribute. Set work aside on a named branch
  or in a worktree.
