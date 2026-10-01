// Capture config for sigint.json: sigint.spec.ts, interrupted by capture.sh.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./specs",
  testMatch: [/[\\/]sigint\.spec\.ts$/],
  fullyParallel: false,
  workers: 1,
  retries: 0,
  use: { trace: "off", screenshot: "off", video: "off" },
});
