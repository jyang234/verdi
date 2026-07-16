import { test, expect } from "@playwright/test";
import { SHOWCASE } from "./fixtures";

// Corpus artifact page: title, frontmatter card, and the I-5 dispositions
// table (05 §Workbench; PLAN.md Phase 10 exit criteria: "corpus page
// renders title/frontmatter/dispositions table"). The page under test is
// SHOWCASE.READONLY_SPEC (stale-decline, the corpus's richest committed
// feature) — every assertion below is that spec's real rendered content.
test("corpus page renders title, frontmatter, and the dispositions table", async ({ page }) => {
  // Reach the spec page the way a user does: from the home index, not a
  // hand-typed URL (DEFECT A made home a real, clickable index).
  await page.goto("/");
  await page.locator(`a[href="/a/spec/${SHOWCASE.READONLY_SPEC}"]`).click();

  await expect(page).toHaveTitle(/Stale decline handling \(fixture\)/);
  await expect(page.locator(".page-header h1")).toHaveText("Stale decline handling (fixture)");

  // Frontmatter card.
  const meta = page.locator(".metadata-card");
  await expect(meta).toContainText("platform-team");
  await expect(meta).toContainText("jira:LOAN-1482");
  await expect(meta).toContainText("feature");

  // Rendered markdown body (a phrase from the renovated Design notes).
  await expect(page.locator(".content")).toContainText("routed through the outbox pattern");

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
