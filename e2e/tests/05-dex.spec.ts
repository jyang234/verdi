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

test("highlighted code is legible in dark mode (light-on-dark, not github light ink)", async ({ page }) => {
  // The svcfix boundary-contract permalink pretty-prints its JSON through
  // chroma. In a dark-mode browser the page background goes dark; the
  // defect was that chroma's pinned github (light) palette was baked inline
  // into the HTML, leaving near-black code on the dark page. With
  // class-based output plus the composed github-dark palette, the same
  // markup must restyle to light-on-dark.
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto(`${dexBase}/a/svc/svcfix/boundary-contract/`);

  const code = page.locator("pre.chroma-chroma").first();
  await expect(code).toBeVisible();

  const { color, background } = await code.evaluate((el) => {
    const cs = getComputedStyle(el as HTMLElement);
    return { color: cs.color, background: cs.backgroundColor };
  });

  // github-dark: foreground #e6edf3, background #0d1117 — i.e. light text on
  // a dark block, the opposite of the github light ink that would have
  // shown through before the fix.
  expect(background).toBe("rgb(13, 17, 23)");
  expect(color).toBe("rgb(230, 237, 243)");
});

test("dex search returns a known hit", async ({ page }) => {
  await page.goto(`${dexBase}/search/`);
  await page.fill("#search-box", "outbox");

  const results = page.locator("#search-results li a");
  await expect(results.first()).toBeVisible({ timeout: 5_000 });
  const texts = await results.allTextContents();
  expect(texts.some((t) => t.toLowerCase().includes("outbox"))).toBeTruthy();
});
