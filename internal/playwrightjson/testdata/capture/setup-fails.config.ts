// Capture config for setup-fails.json: a failing global setup in front of
// other.spec.ts, which never runs.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]other\.spec\.ts$/],
  globalSetup: "./global-setup-fails.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
