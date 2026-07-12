import { test, expect } from "@playwright/test";
import { DESIGN_SPEC, boardPath } from "./fixtures";

// Presentation-meaningful assertions for the visual system ("the registry"):
// the board is a real spatial canvas (absolute positioning, SVG yarn that
// follows a drag), the dex's temporal banner carries its class-specific
// styling hook with per-class color, the copy-reference button truncates
// only its visible label (the clipboard/title form stays full), and the
// dark theme actually renders dark pages with light ink.
//
// Runs after 02/03 in the serial suite: earlier tests may have moved the
// first sticky and committed the board, so nothing here depends on the
// fixture's original coordinates — only on structural presentation truths.

const dexBase = "http://127.0.0.1:4174";

test("board stickies are absolutely positioned at their coordinates", async ({ page }) => {
  await page.goto("/board/STORY-1482");

  const sticky = page.locator(".sticky").first();
  await expect(sticky).toBeVisible();

  const { position, left, top } = await sticky.evaluate((el) => {
    const cs = getComputedStyle(el as HTMLElement);
    return { position: cs.position, left: (el as HTMLElement).style.left, top: (el as HTMLElement).style.top };
  });
  expect(position).toBe("absolute");
  // The inline style IS the stored coordinate (board.go renders left/top
  // from the autosaved x/y).
  expect(left).toMatch(/^\d+(\.\d+)?px$/);
  expect(top).toMatch(/^\d+(\.\d+)?px$/);

  // Per-type treatment: each sticky carries its annotation type as both a
  // data attribute and a type-specific class (paper color).
  await expect(page.locator('.sticky[data-type="question"].sticky--question')).toHaveCount(1);
  await expect(page.locator('.sticky[data-type="agent-task"].sticky--agent-task')).toHaveCount(1);
});

test("yarn renders as an SVG thread and follows a dragged sticky", async ({ page }) => {
  await page.goto("/board/STORY-1482");

  // The yarn overlay exists and draws the fixture's one pin->sticky thread.
  const thread = page.locator("#board-canvas svg.yarn-svg path.yarn-thread");
  await expect(thread).toHaveCount(1);
  const dBefore = await thread.getAttribute("d");
  expect(dBefore).toBeTruthy();

  // Drag the thread's sticky endpoint; the path must be redrawn.
  const sticky = page.locator('.sticky[data-key="a-01J8Z0K3AAAAAAAAAAAAAAAAAA"]');
  const box = await sticky.boundingBox();
  expect(box).not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.down();
  await page.mouse.move(box!.x + 320, box!.y + 240, { steps: 10 });
  await page.mouse.up();

  const dAfter = await thread.getAttribute("d");
  expect(dAfter).toBeTruthy();
  expect(dAfter).not.toBe(dBefore);

  await expect(page.locator("#autosave-status")).toHaveText("saved", { timeout: 5_000 });
});

test("dex temporal banners carry their class-specific styling hook and color", async ({ page }) => {
  // Frozen page: the temporal--frozen hook, styled distinctly.
  await page.goto(`${dexBase}/a/adr/0001-outbox-events/`);
  const frozenBanner = page.locator(".temporal-banner.temporal--frozen");
  await expect(frozenBanner).toBeVisible();
  const frozenColor = await frozenBanner.evaluate((el) => getComputedStyle(el).borderLeftColor);

  // Authored-living page: its own hook, and a DIFFERENT class color — the
  // temporal classes are a visual language, not one shared accent.
  await page.goto(`${dexBase}/a/spec/store-layout-notes/`);
  const authoredBanner = page.locator(".temporal-banner.temporal--authored-living");
  await expect(authoredBanner).toBeVisible();
  const authoredColor = await authoredBanner.evaluate((el) => getComputedStyle(el).borderLeftColor);

  expect(authoredColor).not.toBe(frozenColor);

  // Listing pages are living-gated.
  await page.goto(`${dexBase}/by-kind/`);
  await expect(page.locator(".temporal-banner.temporal--living-gated")).toBeVisible();
});

test("copy-reference truncates the visible sha but copies/announces the full form", async ({ page }) => {
  await page.goto(`${dexBase}/a/spec/stale-decline/`);
  const btn = page.locator("button.copy-ref");
  await expect(btn).toBeVisible();

  const full = await btn.getAttribute("data-copy-ref");
  expect(full).toMatch(/^spec\/stale-decline@[0-9a-f]{40}$/);
  // title carries the full pinned form for hover/AT.
  expect(await btn.getAttribute("title")).toBe(full);
  // The visible label shows the short sha, not all 40 hex chars.
  const label = await btn.textContent();
  expect(label).toContain("spec/stale-decline@" + full!.slice(full!.indexOf("@") + 1, full!.indexOf("@") + 8));
  expect(label).not.toContain(full!.slice(full!.indexOf("@") + 1));
});

test("dispositions-table sticky ids keep their full form in the DOM under a truncated display", async ({ page }) => {
  await page.goto(`${dexBase}/a/spec/stale-decline/`);
  const cell = page.locator("table.dispositions-table code.ulid").first();
  await expect(cell).toBeVisible();
  // Full ULID intact as text (selection copies the full form) and in title...
  await expect(cell).toHaveText("a-01J8Z0K3AAAAAAAAAAAAAAAAAA");
  expect(await cell.getAttribute("title")).toBe("a-01J8Z0K3AAAAAAAAAAAAAAAAAA");
  // ...while CSS clips the rendered box narrower than the full id would run.
  const { clientWidth, scrollWidth } = await cell.evaluate((el) => ({
    clientWidth: (el as HTMLElement).clientWidth,
    scrollWidth: (el as HTMLElement).scrollWidth,
  }));
  expect(clientWidth).toBeLessThan(scrollWidth);
});

test("dark theme renders dark pages with light ink on workbench and dex", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });

  const luminance = (rgb: string) => {
    const m = rgb.match(/\d+/g)!.map(Number);
    return (m[0] + m[1] + m[2]) / 3;
  };

  // Home, a dex artifact page, and the v2 board page — the board's whole
  // visual system (canvas, cards, yarn, chips) must go dark with the
  // theme, not just the site chrome.
  for (const url of ["/", `${dexBase}/a/spec/stale-decline/`, boardPath(DESIGN_SPEC)]) {
    await page.goto(url);
    const { bg, fg } = await page.evaluate(() => {
      const cs = getComputedStyle(document.body);
      return { bg: cs.backgroundColor, fg: cs.color };
    });
    expect(luminance(bg), `${url} background should be dark`).toBeLessThan(80);
    expect(luminance(fg), `${url} ink should be light`).toBeGreaterThan(160);
  }
});
