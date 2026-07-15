import { test, expect } from "@playwright/test";

// The verdict viewer: cross-commit per-AC diff of examples/showcase's two
// derived verdicts.json snapshots for spec/stale-decline (PLAN.md Phase 10
// exit criteria: "verdict viewer diffs the fixture's two canned
// snapshots").
const snapshotA = "2350631724b1e69ccdd84da40686a8f079955dc4";
const snapshotB = "74c957aed504671bd4fc4ceb30907d2f4813e9b7";

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
  await page.goto("/verdict/spec/stale-decline");
  await expect(page.locator("body")).toContainText(snapshotA);
  await expect(page.locator("body")).toContainText(snapshotB);
});
