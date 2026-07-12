import { test, expect, type Locator, type Page } from "@playwright/test";
import { DESIGN_SPEC, READONLY_SPEC, AC_IDS, DECISION_PLAIN, boardPath } from "./fixtures";
import { expectAutosaved } from "./helpers";

// Drag robustness under REAL input conditions — the regression suite for
// the owner-reported "none of the elements are draggable" defect. The
// original gesture code listened to mouse events only and never captured
// the pointer, which the suite's synthetic mouse stream could not
// distinguish from real input; these tests drive the input classes that
// exposed it: (a) touch drags (dead under mouse-only listeners), (b) a
// release the page never sees (a stuck gesture then chased a button-up
// cursor and committed a phantom position), (c) a drag attempt on a
// read-only board (dead-silent immobility reading as breakage), and
// (d) collision-free drops (a drop may not bury one card under another,
// and resolving it may move ONLY the dragged card).

const position = (el: Locator) =>
  el.evaluate((node) => ({
    left: (node as HTMLElement).style.left,
    top: (node as HTMLElement).style.top,
  }));

const cardRects = (page: Page) =>
  page.locator('[data-testid^="card-"]').evaluateAll((els) =>
    els.map((el) => ({
      id: el.getAttribute("data-testid"),
      x: (el as HTMLElement).offsetLeft,
      y: (el as HTMLElement).offsetTop,
      w: (el as HTMLElement).offsetWidth,
      h: (el as HTMLElement).offsetHeight,
    })),
  );

test.describe("board drag robustness (real-input regression)", () => {
  test.use({ hasTouch: true });

  test("a touch drag moves a card and persists (pointer events, not mouse-only)", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const card = page.getByTestId(`card-${AC_IDS[1]}`);
    const before = await position(card);
    const box = await card.boundingBox();
    expect(box).not.toBeNull();
    const cx = box!.x + box!.width / 2;
    const cy = box!.y + box!.height / 2;

    // A real finger: trusted touch events through the browser's input
    // pipeline (not synthesized DOM events).
    const cdp = await page.context().newCDPSession(page);
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchStart",
      touchPoints: [{ x: cx, y: cy, id: 1 }],
    });
    for (let i = 1; i <= 12; i++) {
      await cdp.send("Input.dispatchTouchEvent", {
        type: "touchMove",
        touchPoints: [{ x: cx + i * 15, y: cy + i * 25, id: 1 }],
      });
    }
    await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await expectAutosaved(page);

    const after = await position(page.getByTestId(`card-${AC_IDS[1]}`));
    expect(after).not.toEqual(before);

    await page.reload();
    expect(await position(page.getByTestId(`card-${AC_IDS[1]}`))).toEqual(after);
  });

  test("a release the page never sees lands the drop instead of stranding the drag", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const card = page.getByTestId(`card-${DECISION_PLAIN}`);
    const box = await card.boundingBox();
    expect(box).not.toBeNull();
    const sx = box!.x + box!.width / 2;
    const sy = box!.y + box!.height / 2;

    // Drag a little...
    await page.mouse.move(sx, sy);
    await page.mouse.down();
    await page.mouse.move(sx + 40, sy + 40, { steps: 4 });
    const mid = await position(card);

    // ...then the release happens where the page cannot see it (outside
    // the window, over a native scrollbar): the page's next real input is
    // a button-up pointer stream. Trusted CDP events, buttons: 0.
    const cdp = await page.context().newCDPSession(page);
    for (const [x, y] of [
      [sx + 400, sy + 300],
      [sx + 500, sy + 350],
      [sx + 600, sy + 400],
    ]) {
      await cdp.send("Input.dispatchMouseEvent", {
        type: "mouseMoved",
        x,
        y,
        button: "none",
        buttons: 0,
      });
    }

    // The card must NOT chase the button-up cursor; the drop lands where
    // the drag actually was, and autosaves.
    await expectAutosaved(page);
    const after = await position(page.getByTestId(`card-${DECISION_PLAIN}`));
    expect(after).toEqual(mid);

    // No half-open gesture remains: further button-up movement is inert.
    await page.mouse.move(sx + 200, sy + 200);
    expect(await position(page.getByTestId(`card-${DECISION_PLAIN}`))).toEqual(after);
  });

  test("a read-only board refuses a drag visibly, never silently", async ({
    page,
  }) => {
    await page.goto(boardPath(READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );

    const card = page.locator('[data-testid^="card-"]').first();
    const before = await position(card);
    const box = await card.boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
    await page.mouse.down();
    await page.mouse.move(box!.x + 220, box!.y + 180, { steps: 6 });
    await page.mouse.up();

    // The refusal is visible, names why, and speaks the notice channel.
    const refusal = page.getByTestId("drag-refusal");
    await expect(refusal).toBeVisible();
    await expect(refusal).toHaveText(/frozen with the accepted spec/);
    await expect(refusal).toHaveText(/supersession/);

    // And nothing moved.
    expect(await position(card)).toEqual(before);
  });

  test("a drop onto an occupied spot resolves collision-free and moves only the dragged card", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const dragged = `card-${AC_IDS[1]}`;
    const others = new Map(
      (await cardRects(page))
        .filter((r) => r.id !== dragged)
        .map((r) => [r.id, r]),
    );

    // Drop AC-2 squarely onto AC-1's footprint.
    const target = await page.getByTestId(`card-${AC_IDS[0]}`).boundingBox();
    const source = await page.getByTestId(dragged).boundingBox();
    expect(target).not.toBeNull();
    expect(source).not.toBeNull();
    await page.mouse.move(source!.x + source!.width / 2, source!.y + source!.height / 2);
    await page.mouse.down();
    await page.mouse.move(target!.x + target!.width / 2, target!.y + target!.height / 2, {
      steps: 10,
    });
    await page.mouse.up();
    await expectAutosaved(page);

    const after = await cardRects(page);
    const draggedRect = after.find((r) => r.id === dragged)!;
    // Collision-free: the dragged card overlaps no other card.
    for (const r of after) {
      if (r.id === dragged) continue;
      const overlaps =
        draggedRect.x < r.x + r.w &&
        r.x < draggedRect.x + draggedRect.w &&
        draggedRect.y < r.y + r.h &&
        r.y < draggedRect.y + draggedRect.h;
      expect(overlaps, `resolved drop still overlaps ${r.id}`).toBe(false);
    }
    // And ONLY the dragged card moved: every other card sits exactly where
    // it was (the ratified add/drag-never-reflows property, at the drop).
    for (const r of after) {
      if (r.id === dragged) continue;
      expect(others.get(r.id), `card ${r.id} moved during drop resolution`).toEqual(r);
    }

    // The resolution is durable: a fresh projection reproduces it.
    await page.reload();
    const reloaded = await cardRects(page);
    expect(reloaded.find((r) => r.id === dragged)).toEqual(draggedRect);
  });
});
