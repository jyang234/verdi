// Capture fixture (not a suite test): a run interrupted by SIGINT, as a
// cancelled job's runner is, while its second test is running. capture.sh
// sends the signal once that test has written CAPTURE_MARKER.
import * as fs from "node:fs";
import { test, expect } from "@playwright/test";

test.describe("interrupted run", () => {
  test("passes before the interrupt", () => {
    expect(1).toBe(1);
  });

  test("is running when the run is interrupted", async () => {
    test.setTimeout(60_000);
    fs.writeFileSync(process.env.CAPTURE_MARKER as string, "running\n");
    await new Promise((resolve) => setTimeout(resolve, 30_000));
  });

  test("never starts", () => {});
});
