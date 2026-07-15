import { test, expect } from "@playwright/test";

// The board: loads stickies (including board-only ones, I-34), a pinned
// card, and yarn; dragging a sticky updates its position and autosaves;
// reloading proves the position actually persisted server-side (PLAN.md
// Phase 10 exit criteria: "board autosave round-trips").
test("board loads stickies, drag updates position, and autosave persists across reload", async ({ page }) => {
  await page.goto("/board/STORY-1482");

  // The pinned card and all three stickies (one targeted, two board-only
  // per I-34) render with their resolved annotation content.
  await expect(page.locator(".card")).toContainText("spec/stale-decline");
  const stickies = page.locator(".sticky");
  await expect(stickies).toHaveCount(3);
  await expect(page.locator("#board-canvas")).toContainText("write up a retry note for the charge API path");
  await expect(page.locator("#board-canvas")).toContainText("what about partial refunds?");
  await expect(page.locator("#board-canvas")).toContainText("should partial refunds share the stale-decline retry budget?");

  // Yarn.
  await expect(page.locator(".yarn")).toContainText("relates");

  // Drag the first sticky to a new position. Grab it near its top-left
  // corner, not its center: real paper-sized stickies at the fixture's
  // coordinates overlap, so a center-point grab lands on whichever LATER
  // sticky is stacked on top — the same element a human clicking that
  // pixel would drag.
  const sticky = stickies.first();
  const before = await sticky.boundingBox();
  expect(before).not.toBeNull();

  const targetX = (before!.x) + 260;
  const targetY = (before!.y) + 180;

  await page.mouse.move(before!.x + 8, before!.y + 8);
  await page.mouse.down();
  await page.mouse.move(targetX + 10, targetY + 10, { steps: 12 });
  await page.mouse.up();

  await expect(page.locator("#autosave-status")).toHaveText("saved", { timeout: 5_000 });

  const afterLeft = await sticky.evaluate((el) => (el as HTMLElement).style.left);
  const afterTop = await sticky.evaluate((el) => (el as HTMLElement).style.top);

  // Reload: a fresh page load must reflect the autosaved position, not the
  // original fixture position — proving the write landed on disk
  // (internal/boardio's atomic save), not just in this page's in-memory
  // state.
  await page.reload();
  const reloadedSticky = page.locator(".sticky").first();
  await expect(reloadedSticky).toBeVisible();
  const reloadedLeft = await reloadedSticky.evaluate((el) => (el as HTMLElement).style.left);
  const reloadedTop = await reloadedSticky.evaluate((el) => (el as HTMLElement).style.top);

  expect(reloadedLeft).toBe(afterLeft);
  expect(reloadedTop).toBe(afterTop);
  // And it must differ from the fixture's original position (200, 60).
  expect(reloadedLeft).not.toBe("200px");
});

test("an unused board loads an empty page, not an error", async ({ page }) => {
  const resp = await page.goto("/board/NEVER-USED-BOARD-1");
  expect(resp?.status()).toBe(200);
  await expect(page.locator(".sticky")).toHaveCount(0);
});
