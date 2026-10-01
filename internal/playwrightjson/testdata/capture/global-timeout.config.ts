// Capture config for global-timeout.json: global-timeout.spec.ts under a
// global timeout that stops the run while its second test is still running.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]global-timeout\.spec\.ts$/],
  globalTimeout: 3_000,
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
