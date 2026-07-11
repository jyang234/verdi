import { test, expect, type Locator, type Page } from "@playwright/test";
import { DESIGN_SPEC, AC_IDS, boardPath } from "./fixtures";
import { addSticky, expectAutosaved } from "./helpers";

// Position persistence for the v1 board's drag surfaces — the two
// stores 02 §Record schemas splits them into: an object card's drag
// lands in the spec-sidecar layout.json ("positions only, never
// content; autosaved ... never committed per-drag"), while a scratch
// sticky's drag rewrites its own annotation record's board {x, y}.
// Written by V1-P6 alongside the pre-authored acceptance set (CLAUDE.md:
// every browser-facing behavioral path gets a Playwright test; the
// acceptance contract did not cover dragging).

async function dragBy(page: Page, el: Locator, dx: number, dy: number) {
  const box = await el.boundingBox();
  expect(box).not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.down();
  await page.mouse.move(box!.x + box!.width / 2 + dx, box!.y + box!.height / 2 + dy, {
    steps: 10,
  });
  await page.mouse.up();
  await expectAutosaved(page);
}

const position = (el: Locator) =>
  el.evaluate((node) => ({
    left: (node as HTMLElement).style.left,
    top: (node as HTMLElement).style.top,
  }));

test.describe("V1-P6: dragged positions persist", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("a dragged card's position lands in layout.json and survives reload", async ({
    page,
  }) => {
    const card = page.getByTestId(`card-${AC_IDS[2]}`);
    const before = await position(card);

    await dragBy(page, card, 70, 320);

    const after = await position(card);
    expect(after).not.toEqual(before);

    // A fresh projection reads the stored coordinate back verbatim.
    await page.reload();
    expect(await position(page.getByTestId(`card-${AC_IDS[2]}`))).toEqual(after);
  });

  test("a dragged sticky's position lands in its annotation record and survives reload", async ({
    page,
  }) => {
    const sticky = await addSticky(page, "drag me: sticky position check");
    const before = await position(sticky);

    await dragBy(page, sticky.first(), 180, 120);

    const moved = page
      .locator('[data-testid^="sticky-"]')
      .filter({ hasText: "drag me: sticky position check" });
    const after = await position(moved);
    expect(after).not.toEqual(before);

    await page.reload();
    expect(
      await position(
        page
          .locator('[data-testid^="sticky-"]')
          .filter({ hasText: "drag me: sticky position check" }),
      ),
    ).toEqual(after);
  });
});
