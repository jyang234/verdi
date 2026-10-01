// Capture config for outcomes.json: outcomes.spec.ts and other.spec.ts in one
// run, with the producer's run shape (one worker, no retries, recording off).
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]outcomes\.spec\.ts$/, /[\\/]other\.spec\.ts$/],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
