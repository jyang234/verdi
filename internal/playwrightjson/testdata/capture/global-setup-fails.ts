// Capture fixture: a global setup that fails, the run-level error a broken
// harness produces before any test runs (setup-fails.config.ts).
export default async function globalSetup(): Promise<void> {
  throw new Error("capture fixture: global setup failed");
}
