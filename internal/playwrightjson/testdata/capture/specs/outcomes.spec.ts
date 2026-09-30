// Capture fixture (not a suite test): every per-test outcome the producer
// maps (SI-294), in one file run once with --workers=1 --retries=0, as the
// producer runs its named files. capture.sh runs it; it never joins the e2e
// shards, which collect only e2e/tests/*.spec.ts.
import { test, expect } from "@playwright/test";

test("passes at the top level", () => {
  expect(1 + 1).toBe(2);
});

test.describe("outcomes", () => {
  test("passes", () => {
    expect(true).toBe(true);
  });

  test("fails", () => {
    expect(1).toBe(2);
  });

  test("times out", async () => {
    test.setTimeout(500);
    await new Promise((resolve) => setTimeout(resolve, 5_000));
  });

  test.skip("is skipped declaratively", () => {});

  test("skips itself at run time", () => {
    test.skip(true, "skipped at run time");
  });

  test.describe("nested describe", () => {
    test("passes in a nested describe", () => {});
  });

  test.describe(() => {
    test("passes inside an anonymous describe", () => {});
  });
});

test.describe("titles: a colon in the describe", () => {
  test("case: a colon in the title", () => {});
});

test.describe("serial group", () => {
  test.describe.configure({ mode: "serial" });

  test("first fails", () => {
    expect(1).toBe(2);
  });

  test("second is skipped after the failure", () => {});
});

// A suite-level retry override survives the producer's --retries=0, so a
// named test can still be retried: the flaky case is reachable in its run.
test.describe("retried once", () => {
  test.describe.configure({ retries: 1 });

  test("fails first then passes", () => {
    expect(test.info().retry).toBe(1);
  });

  test("fails on every attempt", () => {
    expect(1).toBe(2);
  });
});

test.describe("expected failures", () => {
  test("fails as test.fail() expects", () => {
    test.fail();
    expect(1).toBe(2);
  });

  test("passes despite test.fail()", () => {
    test.fail();
    expect(1).toBe(1);
  });
});
