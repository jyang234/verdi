import { test, expect, type Page } from "@playwright/test";
import { FEATURE_SPEC, boardPath } from "./fixtures";

// Owner directive (R4-I-35): cards must never render stacked, in any
// mode. The regression fixture is testdata/corpus's accepted-pending-build
// layout.json — ac-1 stored at (40,20) and ac-2 at (220,20), a 20px
// footprint overlap under the uniform CardWidth=200 (positions saved
// before the footprint enlargement). The spec is accepted and on main, so
// its board is READ-ONLY: drags are refused and the collision could never
// be repaired by hand — display-time resolution is the only fix. The
// projection nudges the canonical-order later claimant to the nearest
// free spot and NEVER writes layout.json (deliberately kept colliding as
// the permanent fixture — do not "clean it up").

const allCardRects = (page: Page) =>
  page.locator(".objcard, .refcard").evaluateAll((els) =>
    els.map((el) => ({
      id:
        el.getAttribute("data-id") ||
        el.getAttribute("data-ref") ||
        el.getAttribute("data-testid"),
      x: (el as HTMLElement).offsetLeft,
      y: (el as HTMLElement).offsetTop,
      w: (el as HTMLElement).offsetWidth,
      h: (el as HTMLElement).offsetHeight,
    })),
  );

test.describe("board: stored collisions render resolved (never stacked)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
  });

  test("the colliding-fixture board renders zero overlapping cards, deterministically", async ({
    page,
  }) => {
    const rects = await allCardRects(page);
    expect(rects.length).toBeGreaterThan(2);

    // The first claimant (canonical order) keeps its stored position
    // verbatim — the store itself was never rewritten.
    const ac1 = rects.find((r) => r.id === "ac-1");
    expect(ac1).toBeDefined();
    expect({ x: ac1!.x, y: ac1!.y }).toEqual({ x: 40, y: 20 });

    // The collider was nudged off its stored, overlapping spot.
    const ac2 = rects.find((r) => r.id === "ac-2");
    expect(ac2).toBeDefined();
    expect({ x: ac2!.x, y: ac2!.y }).not.toEqual({ x: 220, y: 20 });

    // No two cards' rendered rects intersect — object cards and
    // reference cards alike.
    for (let i = 0; i < rects.length; i++) {
      for (let j = i + 1; j < rects.length; j++) {
        const a = rects[i];
        const b = rects[j];
        const overlaps =
          a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;
        expect(
          overlaps,
          `cards ${a.id} and ${b.id} render stacked: ${JSON.stringify(a)} vs ${JSON.stringify(b)}`,
        ).toBe(false);
      }
    }

    // Pure function of the same inputs: a reload reproduces the identical
    // resolved board.
    await page.reload();
    const reloaded = await allCardRects(page);
    expect(reloaded).toEqual(rects);
  });
});
