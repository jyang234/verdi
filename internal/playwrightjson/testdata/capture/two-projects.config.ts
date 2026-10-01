// Capture config for two-projects.json: other.spec.ts under two projects, so
// the one test appears once per project in the report.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]other\.spec\.ts$/],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
  projects: [{ name: "alpha" }, { name: "beta" }],
});
