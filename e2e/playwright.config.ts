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
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
