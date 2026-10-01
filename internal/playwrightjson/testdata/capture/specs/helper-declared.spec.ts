// Capture fixture (not a suite test): a spec file that declares one test of
// its own and imports a helper module (define-shared.ts) that declares
// another. Both tests belong to this file's suite, the unit Playwright's own
// title path and duplicate-title check work in.
import { test, expect } from "@playwright/test";
import { defineSharedChecks } from "./define-shared";

test.describe("local", () => {
  test("passes", () => {
    expect(1).toBe(1);
  });
});

defineSharedChecks("home");
