import { test, expect } from "@playwright/test";

// A dex-built page (served statically by cmd/e2eharness on :4174, over
// the same scratch store `verdi dex build` produced from): the temporal
// banner renders honestly per class, and search over the build-emitted
// inverted index returns a known hit (PLAN.md Phase 10 exit criteria: "a
// dex-built page renders its temporal banner + search returns a known
// hit").
const dexBase = "http://127.0.0.1:4174";

test("a frozen dex page renders its point-in-time temporal banner", async ({ page }) => {
  await page.goto(`${dexBase}/a/adr/0001-outbox-events/`);
  await expect(page.locator(".temporal-banner")).toContainText("point-in-time record");
  await expect(page.locator(".temporal-banner")).toContainText("frozen");
});

test("an authored-living dex page renders its last-modified banner", async ({ page }) => {
  await page.goto(`${dexBase}/a/spec/store-layout-notes/`);
  const banner = page.locator(".temporal-banner");
  await expect(banner).toBeVisible();
  await expect(banner).not.toContainText("point-in-time record");
});

test("dex search returns a known hit", async ({ page }) => {
  await page.goto(`${dexBase}/search/`);
  await page.fill("#search-box", "outbox");

  const results = page.locator("#search-results li a");
  await expect(results.first()).toBeVisible({ timeout: 5_000 });
  const texts = await results.allTextContents();
  expect(texts.some((t) => t.toLowerCase().includes("outbox"))).toBeTruthy();
});
