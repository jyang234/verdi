// Capture fixture (not a suite test): a second file in the same run whose
// test shares a title path with one in outcomes.spec.ts. A title path is
// scoped to its own file, so this is not a duplicate.
import { test, expect } from "@playwright/test";

test.describe("outcomes", () => {
  test("passes", () => {
    expect("other").toBe("other");
  });
});
