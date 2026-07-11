import { defineConfig, devices } from "@playwright/test";

// Chromium only (task instruction: "package.json + playwright.config
// (chromium only)"). The suite runs fully serially (workers: 1): every
// test shares ONE scratch store (cmd/e2eharness provisions it once for
// the whole run), and several tests write real state to it (autosave,
// commit-to-design) — parallel workers would race on the same board file
// and git repository.
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
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
