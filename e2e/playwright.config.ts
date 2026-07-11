import { defineConfig, devices } from "@playwright/test";

// Chromium only (task instruction: "package.json + playwright.config
// (chromium only)"). The suite runs fully serially (workers: 1): every
// test shares ONE scratch store (cmd/e2eharness provisions it once for
// the whole run), and several tests write real state to it (autosave,
// commit-to-design) — parallel workers would race on the same board file
// and git repository.

// The v1-acceptance project holds EXECUTABLE ACCEPTANCE CRITERIA for UI
// that does not exist yet (V1-P6 board v2, V1-P8 dex v2 — PLAN-V1.md §5;
// contract: e2e/tests-v1/README.md). Those specs fail by design until
// their phase lands, so the project materializes ONLY when explicitly
// requested — `--project v1-acceptance` (or V1_ACCEPTANCE=1). A bare
// `npx playwright test` / `make e2e` / CI run never defines it, so the
// default suite's test count and green-ness are untouched. Flip-in
// protocol (a spec moves tests-v1/ → tests/ in the same commit that makes
// it pass): tests-v1/README.md.
const v1AcceptanceRequested =
  process.env.V1_ACCEPTANCE === "1" ||
  process.argv.some(
    (arg, i, argv) =>
      arg === "--project=v1-acceptance" ||
      (arg === "--project" && argv[i + 1] === "v1-acceptance"),
  );
// Worker processes re-evaluate this config without the runner's argv; pin
// the detection into the env (inherited by workers) so the project exists
// there too — otherwise every run dies with "Project not found in the
// worker process".
if (v1AcceptanceRequested) {
  process.env.V1_ACCEPTANCE = "1";
}

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  use: {
    baseURL: "http://127.0.0.1:4173",
    trace: "retain-on-failure",
  },
  // Builds the binary, provisions the scratch store, starts `verdi serve`
  // (workbench, :4173) and a static file server over the built dex site
  // (:4174) — see cmd/e2eharness/main.go's own doc comment. cwd: ".."
  // points the harness's `go build`/`go run` at the verdi module root
  // (this config file lives in verdi/e2e/).
  webServer: {
    command: "go run ./cmd/e2eharness",
    cwd: "..",
    url: "http://127.0.0.1:4173/healthz",
    // reuseExistingServer MUST stay false. Playwright's readiness probe hits
    // only ONE url (:4173/healthz), but a single harness process owns TWO
    // ports: :4173 (workbench) and :4174 (dex static site). If reuse were
    // enabled, any stale process answering :4173 — a leftover `verdi serve`
    // from an interrupted run, or an unrelated demo server on that port —
    // would satisfy the probe, so Playwright would adopt it and NEVER start
    // the harness. Then :4174 stays down (every dex/presentation test fails
    // with net::ERR_CONNECTION_REFUSED) and :4173 serves a foreign/empty
    // store (workbench tests fail on missing fixtures) — the exact
    // nondeterministic 12-15/23 flake this defect produced. false makes the
    // harness authoritative every run and turns port contention into a loud,
    // deterministic failure instead of silent corruption. This also matches
    // CI, where !process.env.CI was already false (CLAUDE.md: trust CI parity).
    reuseExistingServer: false,
    // Send SIGTERM to the harness's process group on teardown (instead of the
    // default hard SIGKILL) so main.go's signal handler runs its graceful
    // subprocess stop, reaping `verdi serve` cleanly. Guarantees no orphaned
    // listener is left on :4173 to break the next run.
    gracefulShutdown: { signal: "SIGTERM", timeout: 10_000 },
    timeout: 60_000,
  },
  projects: [
    {
      name: "chromium",
      testDir: "./tests",
      use: { ...devices["Desktop Chrome"] },
    },
    // Opt-in only (see v1AcceptanceRequested above). Shares the webServer
    // harness so the acceptance specs can run against it on demand; they
    // are EXPECTED to fail until V1-P6/V1-P8 land and flip them into
    // ./tests. Serial like the default project: they mutate one store.
    ...(v1AcceptanceRequested
      ? [
          {
            name: "v1-acceptance",
            testDir: "./tests-v1",
            use: { ...devices["Desktop Chrome"] },
          },
        ]
      : []),
  ],
});
