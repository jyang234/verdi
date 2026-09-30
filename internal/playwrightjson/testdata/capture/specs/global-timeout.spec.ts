// Capture fixture (not a suite test): a run stopped by its global timeout
// (global-timeout.config.ts) while a test is still running, with one test
// before it and one it never reaches. The run reports the timeout as an
// error of its own.
import { test, expect } from "@playwright/test";

test.describe("global timeout", () => {
  test("passes before the stop", () => {
    expect(1).toBe(1);
  });

  test("is running when the run stops", async () => {
    test.setTimeout(60_000);
    await new Promise((resolve) => setTimeout(resolve, 30_000));
  });

  test("never starts", () => {});
});
