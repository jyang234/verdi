// Capture config for duplicate.json: duplicate.spec.ts alone.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]duplicate\.spec\.ts$/],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
