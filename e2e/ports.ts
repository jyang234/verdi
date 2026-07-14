// The e2e suite's port derivation (D6-28): before VERDI_E2E_PORT_BASE
// existed, playwright.config.ts and e2e/tests/fixtures.ts always pointed
// at 127.0.0.1:4173 (workbench), :4174 (dex static site), and :4177
// (control server) — the same hard-coded trio cmd/e2eharness/ports.go
// binds. Fine for one run, but two concurrent `make verify` invocations in
// sibling git worktrees collide on those ports and one loses with
// "address already in use", paying a retry tax during parallel
// implementation waves.
//
// resolvePorts is the single knob that fixes this, mirrored exactly on
// both sides: set VERDI_E2E_PORT_BASE and every port derives from it as
// base, base+1, base+2, base+3 — cmd/e2eharness/ports.go's resolvePorts
// derives the SAME quartet from the SAME variable, so the harness's
// listeners and the test runner's URLs always agree. `go run
// ./cmd/e2eharness` (this config's webServer.command) inherits the parent
// Node process's environment unchanged, so the variable needs no explicit
// plumbing to reach the Go side. Unset (or unparsable): the historical
// hard-coded ports below, byte-for-byte — zero behavior change.
//
// The inspection server (spec/draft-boards; cmd/e2eharness/inspect.go)
// joined this derivation after it was first proven out for the original
// workbench/dex/control trio — its historical hard-coded port (4178) was
// not itself base+3 of the historical workbench default, so the default
// case below preserves 4178 verbatim while the override case folds it
// into the same base+N ladder as the other three.

const DEFAULT_WORKBENCH_PORT = 4173;
const DEFAULT_DEX_PORT = 4174;
const DEFAULT_CONTROL_PORT = 4177;
const DEFAULT_INSPECT_PORT = 4178;

export const PORT_BASE_ENV_VAR = "VERDI_E2E_PORT_BASE";

const MIN_PORT_BASE = 1;
const MAX_PORT_BASE = 65533;

export interface ResolvedPorts {
  workbench: number;
  dex: number;
  control: number;
  inspect: number;
}

// resolvePorts reads PORT_BASE_ENV_VAR out of env (process.env by default;
// overridable for tests) and derives the trio. Any missing, non-numeric,
// or out-of-range value fails CLOSED to the historical defaults —
// printing a notice via console.error — rather than silently deriving a
// half-valid port set.
export function resolvePorts(env: NodeJS.ProcessEnv = process.env): ResolvedPorts {
  const defaults: ResolvedPorts = {
    workbench: DEFAULT_WORKBENCH_PORT,
    dex: DEFAULT_DEX_PORT,
    control: DEFAULT_CONTROL_PORT,
    inspect: DEFAULT_INSPECT_PORT,
  };

  const raw = env[PORT_BASE_ENV_VAR];
  if (!raw) {
    return defaults;
  }

  const base = Number(raw);
  if (!Number.isInteger(base) || base < MIN_PORT_BASE || base > MAX_PORT_BASE) {
    console.error(
      `e2e: ${PORT_BASE_ENV_VAR}=${JSON.stringify(raw)} is not a usable port base ` +
        `(want an integer in ${MIN_PORT_BASE}..${MAX_PORT_BASE}) — falling back to ` +
        `default ports ${defaults.workbench}/${defaults.dex}/${defaults.control}/${defaults.inspect}`,
    );
    return defaults;
  }

  return { workbench: base, dex: base + 1, control: base + 2, inspect: base + 3 };
}
