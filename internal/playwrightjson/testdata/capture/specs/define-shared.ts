// Capture fixture (not a suite test, not a spec file): a helper module that
// declares tests when a spec file calls it. Playwright takes a test's location
// from its direct caller, so the report names this module as each such spec's
// file while listing the test under the calling spec file's own file suite.
import { test, expect } from "@playwright/test";

export function defineSharedChecks(page: string): void {
  test.describe(`shared checks for ${page}`, () => {
    test("renders", () => {
      expect(page).toBe("home");
    });
  });
}
