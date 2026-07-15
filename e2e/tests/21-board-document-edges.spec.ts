import { test, expect, type Locator, type Page } from "@playwright/test";
import { SHOWCASE, boardPath, refCardTestId } from "./fixtures";

// Document-level edges (projection.go: frontmatter `links:` declared on
// the spec document itself, emitted with From:"spec") have exactly ONE
// on-board endpoint — the document is not a card; it lives ABOVE the
// canvas as the placards header. The designed treatment: the edge's
// thread enters the board past the canvas's TOP edge (pointing off-board,
// toward the document) and ties to the on-board endpoint's top edge, the
// type chip riding the thread — never the old degeneration (a bare chip
// parked at the fixed 16,16 corner stack with no thread, reading as a
// broken element). layoutYarn() is mode-independent, so the read-only
// fixture proves the treatment's geometry for all three board modes.
//
// Canvas-relative geometry throughout (offsetLeft/offsetTop — the canvas
// is every element's offsetParent), matching what layoutYarn computes.

const offsetRect = (el: Locator) =>
  el.evaluate((node) => ({
    x: (node as HTMLElement).offsetLeft,
    y: (node as HTMLElement).offsetTop,
    w: (node as HTMLElement).offsetWidth,
    h: (node as HTMLElement).offsetHeight,
  }));

const docChip = (page: Page) =>
  page.locator(`.yarn-chip[data-from="spec"][data-edge-type="${SHOWCASE.DOC_EDGE_TYPE}"]`);

test.describe("board: document-level edges hang from the top of the board", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
  });

  test("the document edge's chip rides a thread tied to its on-board endpoint, not the canvas corner", async ({
    page,
  }) => {
    const chip = docChip(page);
    await expect(chip).toHaveCount(1);
    await expect(chip).toHaveAttribute("data-to", SHOWCASE.DOC_EDGE_TARGET);
    await expect(chip).toHaveAttribute("data-layer", "spec");

    const ref = page.getByTestId(refCardTestId(SHOWCASE.DOC_EDGE_TARGET));
    await expect(ref).toBeVisible();

    const chipRect = await offsetRect(chip);
    const refRect = await offsetRect(ref);

    // The old corner-stack degeneration is gone: the chip is positioned
    // relative to its resolvable endpoint, not parked at the fixed
    // (16,16) orphan slot.
    expect(
      chipRect.x === 16 && chipRect.y === 16,
      "chip still parked at the corner-stack slot",
    ).toBe(false);

    // Designed anchor: the chip rides the thread hanging above its one
    // on-board endpoint — horizontally within the endpoint's span (± the
    // thread's gentle bow) and wholly above the endpoint's top edge.
    const chipCenterX = chipRect.x + chipRect.w / 2;
    expect(chipCenterX).toBeGreaterThan(refRect.x - 40);
    expect(chipCenterX).toBeLessThan(refRect.x + refRect.w + 40);
    expect(chipRect.y + chipRect.h).toBeLessThanOrEqual(refRect.y + 2);

    // Deterministic: a fresh projection reproduces the same treatment.
    await page.reload();
    expect(await offsetRect(docChip(page))).toEqual(chipRect);
  });

  test("the document edge draws an off-board thread stub tied to the endpoint's top edge", async ({
    page,
  }) => {
    const thread = page.locator(
      "#board-canvas svg.yarn-svg path.yarn-thread--offboard",
    );
    await expect(thread).toHaveCount(1);
    // Spec-layer yarn: the layer class rides along with the modifier.
    await expect(thread).toHaveClass(/yarn-thread--spec/);

    // "M ax ay Q cx cy bx by" — the thread STARTS above the canvas (the
    // SVG clips at the top edge, so the yarn reads as continuing off the
    // board toward the document) and TIES to the reference card's top edge.
    const d = (await thread.getAttribute("d"))!;
    const m = d.match(
      /^M (-?[\d.]+) (-?[\d.]+) Q (-?[\d.]+) (-?[\d.]+) (-?[\d.]+) (-?[\d.]+)$/,
    );
    expect(m, `thread path is not a single quadratic curve: ${d}`).not.toBeNull();
    const [, , ayS, , , bxS, byS] = m!;
    const ay = Number(ayS);
    const bx = Number(bxS);
    const by = Number(byS);

    const refRect = await offsetRect(
      page.getByTestId(refCardTestId(SHOWCASE.DOC_EDGE_TARGET)),
    );
    expect(ay, "thread must start above the canvas (off-board)").toBeLessThan(0);
    expect(by).toBeCloseTo(refRect.y, 1);
    expect(bx).toBeGreaterThanOrEqual(refRect.x);
    expect(bx).toBeLessThanOrEqual(refRect.x + refRect.w);
  });
});
