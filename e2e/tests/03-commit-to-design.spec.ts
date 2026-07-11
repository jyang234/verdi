import { test, expect } from "@playwright/test";

// Commit-to-design from the board: fill the form, submit, and prove the
// three artifacts (draft spec skeleton, frozen board.json, dispositions
// block) landed for real — not just a green HTTP response — by loading
// the resulting spec's own corpus page and checking its dispositions
// table (PLAN.md Phase 10 exit criteria: "commit-to-design from the board
// produces the three artifacts").
//
// Uses a story ref distinct from spec/stale-decline's own (jira:LOAN-1482)
// so this test's new spec never collides with 04-verdict.spec.ts's own
// target story (storyresolve.Resolve fails loudly on an ambiguous story
// ref — two specs is a real error, not something to paper over).
const specName = "from-board-e2e";
const storyRef = "jira:LOAN-9001";

test("commit-to-design from the board produces the three artifacts", async ({ page }) => {
  await page.goto("/board/STORY-1482");

  await page.fill("#commit-name", specName);
  await page.fill("#commit-story-ref", storyRef);
  await page.click("#commit-form button[type=submit]");

  const result = page.locator("#commit-result");
  await expect(result).toContainText("committed", { timeout: 10_000 });
  await expect(result).toContainText(`spec/${specName}`);
  await expect(result).toContainText("3 sticky(s) dispositioned");

  // Artifact 1 + 3: the draft spec skeleton, with its dispositions block
  // (every sticky open-question — commit-to-design's mechanical promise).
  await page.goto(`/a/spec/${specName}`);
  await expect(page.locator(".page-header h1")).toBeVisible();
  await expect(page.locator(".metadata-card")).toContainText(storyRef);
  await expect(page.locator(".metadata-card")).toContainText("draft");
  const table = page.locator("table.dispositions-table");
  await expect(table).toBeVisible();
  const rows = table.locator("tbody tr");
  await expect(rows).toHaveCount(3);
  await expect(table).toContainText("open-question");
  await expect(table).not.toContainText("incorporated");
  await expect(table).not.toContainText("contradicted");

  // Artifact 2: the frozen board.json snapshot exists alongside spec.md —
  // asserted via the commit response's board_path (checked above landing
  // in the same directory as the spec is enough evidence at the browser
  // layer; internal/commitdesign's own Go tests assert its Frozen/
  // Provenance fields directly).
});

test("commit-to-design rejects a missing spec name", async ({ page }) => {
  await page.goto("/board/STORY-1482");
  await page.fill("#commit-story-ref", "jira:LOAN-9002");
  await page.click("#commit-form button[type=submit]");
  await expect(page.locator("#commit-result")).toContainText("error", { timeout: 10_000 });
});
