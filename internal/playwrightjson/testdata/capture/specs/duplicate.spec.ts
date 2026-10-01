// Capture fixture (not a suite test): two tests in one file that share a
// title path, once as two describes with one title and once through a
// describe title that itself contains " › ". Playwright refuses the file at
// load time, so the run reports an error of its own.
import { test } from "@playwright/test";

test.describe("dup", () => {
  test("same title", () => {});
});

test.describe("dup", () => {
  test("same title", () => {});
});

test.describe("a › b", () => {
  test("c", () => {});
});

test.describe("a", () => {
  test.describe("b", () => {
    test("c", () => {});
  });
});
