// Capture config for no-tests.json: no-tests.spec.ts alone, a spec file that
// declares no tests. Run with --pass-with-no-tests, as the producer runs
// (SI-308), the run is no error: the report only lists no test for the file.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]no-tests\.spec\.ts$/],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
