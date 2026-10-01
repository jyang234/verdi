// Capture config for helper-declared.json: helper-declared.spec.ts alone,
// whose shared tests a helper module it imports (specs/define-shared.ts)
// declares. The helper matches no testMatch here, so it is never a test file
// of its own.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]helper-declared\.spec\.ts$/],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
