import { test, expect } from "@playwright/test";

// Corpus artifact page: title, frontmatter card, and the I-5 dispositions
// table (05 §Workbench; PLAN.md Phase 10 exit criteria: "corpus page
// renders title/frontmatter/dispositions table").
test("corpus page renders title, frontmatter, and the dispositions table", async ({ page }) => {
  await page.goto("/a/spec/stale-decline");

  await expect(page).toHaveTitle(/Stale decline handling \(fixture\)/);
  await expect(page.locator(".page-header h1")).toHaveText("Stale decline handling (fixture)");

  // Frontmatter card.
  const meta = page.locator(".metadata-card");
  await expect(meta).toContainText("platform-team");
  await expect(meta).toContainText("jira:LOAN-1482");
  await expect(meta).toContainText("feature");

  // Rendered markdown body.
  await expect(page.locator(".content")).toContainText("Charge API calls are retried");

  // I-5 dispositions table: every disposition value present.
  const table = page.locator("table.dispositions-table");
  await expect(table).toBeVisible();
  await expect(table).toContainText("incorporated");
  await expect(table).toContainText("contradicted");
  await expect(table).toContainText("open-question");

  // Links/backlinks panel.
  await expect(page.locator(".connections")).toContainText("implements");
});

test("corpus page 404s on an unknown artifact", async ({ page }) => {
  const resp = await page.goto("/a/spec/does-not-exist-at-all");
  expect(resp?.status()).toBe(404);
});
