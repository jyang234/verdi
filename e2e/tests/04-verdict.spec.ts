import { test, expect } from "@playwright/test";
import { SHOWCASE } from "./fixtures";

// The verdict viewer: cross-commit per-AC diff of examples/showcase's two
// derived verdicts.json snapshots for spec/stale-decline — the
// SHOWCASE.READONLY_SPEC feature, whose story is jira:LOAN-1482 (PLAN.md
// Phase 10 exit criteria: "verdict viewer diffs the fixture's two canned
// snapshots").
const snapshotA = "f6dd4c4df724c0b16cae435e96f7e34ac94026c9";
const snapshotB = "16219044c9d6d41de9a0de9464ed24d49283b40c";

test("verdict viewer shows the cross-commit diff", async ({ page }) => {
  await page.goto(`/verdict/jira:LOAN-1482?a=${snapshotA}&b=${snapshotB}`);

  const table = page.locator("table.verdict-diff");
  await expect(table).toBeVisible();

  const rows = table.locator("tbody tr");
  await expect(rows).toHaveCount(4); // ac-1..ac-4

  // ac-1: evidence only in snapshot A.
  const ac1 = rows.filter({ hasText: "ac-1" });
  await expect(ac1).toContainText("removed in B");
  await expect(ac1).toContainText("retryWorker");

  // ac-3: evidence only in snapshot B.
  const ac3 = rows.filter({ hasText: "ac-3" });
  await expect(ac3).toContainText("added in B");
  await expect(ac3).toContainText("abstain");
});

test("verdict viewer without a/b shows a snapshot picker", async ({ page }) => {
  await page.goto(`/verdict/spec/${SHOWCASE.READONLY_SPEC}`);
  await expect(page.locator("body")).toContainText(snapshotA);
  await expect(page.locator("body")).toContainText(snapshotB);
});
