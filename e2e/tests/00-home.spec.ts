import { test, expect } from "@playwright/test";

// The workbench home page (DEFECT A): a real, server-rendered index of the
// store, not a dead-end health skeleton. A user landing on `/` sees the
// active specs, the store's boards, and the discovered services — every one
// a working link they can click through, so the home page is worth landing
// on (05 §Workbench).
test("home lists specs, boards, and services, and links each", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Workbench/);

  // A feature spec, linked to its corpus page, with its verdict/matrix
  // links — scoped to the exhaustive Directory section: spec/home-
  // status-glance's leading glance section links this SAME entry to the
  // SAME hrefs by the frozen contract's own design (dc-3: "mirrors
  // directory.go's existing per-source rules exactly"), so an unscoped
  // page-wide lookup now resolves twice. The glance's own dedicated
  // coverage lives in 43-home-status-glance.spec.ts.
  const directory = page.locator(".home-directory");
  await expect(directory.locator('a[href="/a/spec/stale-decline"]')).toBeVisible();
  await expect(directory.locator('a[href="/matrix/jira:LOAN-1482"]')).toBeVisible();
  await expect(directory.locator('a[href="/verdict/jira:LOAN-1482"]')).toBeVisible();

  // The store's board, linked to its board page (the v0 board key route,
  // which the glance section never emits — no collision here).
  await expect(page.locator('a[href="/board/STORY-1482"]')).toBeVisible();

  // The discovered service (the e2e harness folds in testdata/svcfix).
  await expect(page.locator(".home-services")).toContainText("svcfix");
});

test("home clicks through to a spec page", async ({ page }) => {
  await page.goto("/");
  await page.locator(".home-directory").locator('a[href="/a/spec/stale-decline"]').click();
  await expect(page).toHaveTitle(/Stale decline handling \(fixture\)/);
  await expect(page.locator(".page-header h1")).toHaveText("Stale decline handling (fixture)");
});

test("home clicks through to the board", async ({ page }) => {
  await page.goto("/");
  await page.locator('a[href="/board/STORY-1482"]').click();
  await expect(page.locator(".page-header h1")).toHaveText("Board: STORY-1482");
  await expect(page.locator(".card")).toContainText("spec/stale-decline");
});
