import { test, expect, type Page, type Locator } from "@playwright/test";
import { refCardTestId, stubCardTestId } from "./fixtures";
import { dragToTrash, expectAutosaved, wallToolbar } from "./helpers";

// spec/wall-canvas-v2 ac-6 (lane F2c): the keyboard reaches every card and
// action — the arrows move the selection within and across columns and
// reveal the card, Enter edits, Escape closes the innermost open thing one
// layer per press, Delete removes the selected card or thread through the
// existing confirmation, and the trash and Delete both refuse a declared
// stub in plain language — and a minimap shows every card and the viewport
// and moves the viewport when dragged.
//
// The last test is the producer its obligation names
// (.verdi/obligations/wall-canvas-v2/ac-6--behavioral.md), titled exactly
// as the claim spells it under the obligations' `wall-canvas ›` prefix
// (SI-350 (12)). The wall is this file's own fixture
// (cmd/e2eharness/provision_board.go, canvasKeysSpecName), opened on its
// writable path, where the domain is live, so Enter edits and Delete
// writes; its constants live here, not in fixtures.ts (SI-350 (11)). The
// file passes alone (BL-98) and assumes nothing another file wrote; its
// own writes sit in the last test, after every test that reads the whole
// wall. Every control is reached from the keyboard — Tab, the arrows,
// Enter, Escape, Delete — never by .focus() (BL-173 (3)); the minimap, a
// pointer aid, is dragged with the mouse, and the trash's refusal is
// provoked with the mouse, since the trash is a drop target. Every
// assertion reads the DOM and computed geometry, never a screenshot
// (recording stays off).

const WALL = {
  SPEC: "decline-canvas-keys",
  STUB_SLUG: "notice-retraction",
  ADR_REF: "adr/0001-outbox-events",
  STICKY_ID: "a-01J8Z0K3CANVASSTCKY0000002",
  // The writable address: the wall under its own design branch (SI-350 (11)).
  WRITABLE_PATH: "/b/design%2Fdecline-canvas-keys/board/spec/decline-canvas-keys",
};

// The minimap's inner padding (wallminimap.js PAD), mirrored so the test
// states the frame's geometry in numbers.
const MINIMAP_PAD = 3;
// A viewport the wall outgrows on both axes, so every reveal is a real
// move of the canvas's scroll; and the desktop viewport the raw-mouse
// trash drag needs (helpers.ts dragToTrash).
const SMALL = { width: 1000, height: 640 };
const DESKTOP = { width: 1880, height: 1000 };
const CARD_SELECTOR = "#board-canvas .objcard, #board-canvas .stubcard, #board-canvas .refcard, #board-canvas .sticky";

const canvas = (page: Page) => page.getByTestId("board");
const minimap = (page: Page) => page.getByTestId("wall-minimap");
const confirm = (page: Page) => page.locator("#edge-confirm");
const selected = (page: Page) => page.locator("#board-canvas [data-selected]");
const editor = (page: Page) => page.getByRole("textbox", { name: "Card text" });

async function openWritableWall(page: Page): Promise<void> {
  await page.goto(WALL.WRITABLE_PATH);
  await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
  await expect(page.getByTestId("yarn-handle-ac-1")).toBeVisible();
}

// activeKey names the focused element by its test id, else its id.
const activeKey = (page: Page) =>
  page.evaluate(() => {
    const el = document.activeElement;
    return el && el !== document.body ? el.getAttribute("data-testid") || el.id || el.tagName.toLowerCase() : null;
  });

// activeAction names the toolbar action that has the focus, or null when
// the focus is elsewhere.
const activeAction = (page: Page) =>
  page.evaluate(() => {
    const el = document.activeElement;
    return el && el.closest('[data-testid="wall-toolbar"]') ? el.getAttribute("data-wall-action") : null;
  });

// selectedKey reads the selection seam (wallselect.js).
const selectedKey = (page: Page) =>
  page.evaluate(() => {
    const w = window as unknown as { __WALLSELECT__: { selection: () => { kind: string; key: string } | null } };
    const s = w.__WALLSELECT__.selection();
    return s ? `${s.kind}:${s.key}` : null;
  });

// tabUntil presses Tab, or Shift+Tab, until the focus satisfies `done`,
// within a bound: the keyboard's own way to a control, never .focus().
async function tabUntil(page: Page, what: string, done: () => Promise<boolean>, back = false, max = 120): Promise<void> {
  for (let i = 0; i < max; i++) {
    if (await done()) return;
    await page.keyboard.press(back ? "Shift+Tab" : "Tab");
  }
  expect(await done(), `${what} within ${max} presses of ${back ? "Shift+Tab" : "Tab"}`).toBe(true);
}
const focusIs = (page: Page, key: string) => async () => (await activeKey(page)) === key;
const focusOnAction = (page: Page, action: string) => async () => (await activeAction(page)) === action;

// columnsOf groups the wall's cards into columns by their left edge, left
// to right, each column top to bottom, by test id.
const columnsOf = (page: Page) =>
  page.evaluate((sel) => {
    const byLeft = new Map<number, HTMLElement[]>();
    for (const c of Array.from(document.querySelectorAll<HTMLElement>(sel))) {
      if (c.classList.contains("sticky-draft")) continue;
      const list = byLeft.get(c.offsetLeft) ?? [];
      list.push(c);
      byLeft.set(c.offsetLeft, list);
    }
    return Array.from(byLeft.entries())
      .sort((a, b) => a[0] - b[0])
      .map(([, els]) => els.sort((a, b) => a.offsetTop - b.offsetTop).map((el) => el.getAttribute("data-testid")!));
  }, CARD_SELECTOR);

// expectRevealed: the card lies whole inside the canvas's visible box (its
// scrollport, not its content) and inside the viewport.
async function expectRevealed(page: Page, card: Locator, what: string): Promise<void> {
  const box = (await card.boundingBox())!;
  expect(box, `${what} has a box`).not.toBeNull();
  const port = await canvas(page).evaluate((c) => {
    const r = c.getBoundingClientRect();
    return { x: r.left + c.clientLeft, y: r.top + c.clientTop, w: c.clientWidth, h: c.clientHeight };
  });
  expect(box.x, `${what}: inside the canvas on the left`).toBeGreaterThanOrEqual(port.x - 0.5);
  expect(box.x + box.width, `${what}: inside the canvas on the right`).toBeLessThanOrEqual(port.x + port.w + 0.5);
  expect(box.y, `${what}: inside the canvas at the top`).toBeGreaterThanOrEqual(port.y - 0.5);
  expect(box.y + box.height, `${what}: inside the canvas at the foot`).toBeLessThanOrEqual(port.y + port.h + 0.5);
  const vp = page.viewportSize()!;
  expect(box.x, `${what}: inside the viewport on the left`).toBeGreaterThanOrEqual(-0.5);
  expect(box.x + box.width, `${what}: inside the viewport on the right`).toBeLessThanOrEqual(vp.width + 0.5);
  expect(box.y, `${what}: inside the viewport at the top`).toBeGreaterThanOrEqual(-0.5);
  expect(box.y + box.height, `${what}: inside the viewport at the foot`).toBeLessThanOrEqual(vp.height + 0.5);
}

// minimapGeometry reads the minimap against the canvas: the scale the asset
// derives (the frame's inner box over the canvas's scroll size), the
// viewport frame's box inside the minimap, the canvas's scroll state, and
// the blocks drawn.
const minimapGeometry = (page: Page) =>
  page.evaluate((pad) => {
    const h = document.querySelector<HTMLElement>('[data-testid="wall-minimap"]')!;
    const c = document.getElementById("board-canvas")!;
    const view = h.querySelector<HTMLElement>(".wall-minimap-view");
    const hr = h.getBoundingClientRect();
    const vr = view ? view.getBoundingClientRect() : null;
    return {
      scale: Math.min((h.clientWidth - 2 * pad) / c.scrollWidth, (h.clientHeight - 2 * pad) / c.scrollHeight),
      view: vr ? { left: vr.left - hr.left - h.clientLeft, top: vr.top - hr.top - h.clientTop, width: vr.width, height: vr.height } : null,
      scroll: {
        left: c.scrollLeft,
        top: c.scrollTop,
        width: c.clientWidth,
        height: c.clientHeight,
        maxLeft: c.scrollWidth - c.clientWidth,
        maxTop: c.scrollHeight - c.clientHeight,
      },
      blocks: Array.from(h.querySelectorAll<HTMLElement>(".wall-minimap-card")).map((b) => b.getAttribute("data-kind")),
    };
  }, MINIMAP_PAD);

const near = (a: number, b: number, what: string, tol = 1.5) =>
  expect(Math.abs(a - b), `${what}: ${a} vs ${b}`).toBeLessThanOrEqual(tol);

// overlapsOf lists the headings, texts and controls whose VISIBLE boxes
// intersect the minimap's (SI-358 (4): nothing in the row covers content
// or a control). An element's visible box is its own, clipped by every
// ancestor that clips its overflow (the canvas scrolls: a paper past its
// foot is not on screen), so what is counted is what a reader could see.
async function overlapsOf(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const mm = document.querySelector('[data-testid="wall-minimap"]');
    if (!mm) return ["no minimap"];
    const r = mm.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return ["minimap has no box"];
    const visibleBox = (el: Element) => {
      const own = el.getBoundingClientRect();
      let b = { left: own.left, top: own.top, right: own.right, bottom: own.bottom };
      for (let p = el.parentElement; p; p = p.parentElement) {
        const o = getComputedStyle(p);
        if (o.overflow === "visible" && o.overflowX === "visible" && o.overflowY === "visible") continue;
        const c = p.getBoundingClientRect();
        b = { left: Math.max(b.left, c.left), top: Math.max(b.top, c.top), right: Math.min(b.right, c.right), bottom: Math.min(b.bottom, c.bottom) };
      }
      return b;
    };
    const sel =
      "h1, h2, h3, h4, p, span, a, button, input, textarea, select, summary, li, td, th, label, " +
      ".objcard, .stubcard, .refcard, .sticky, .yarn-chip, .zone-label, .board-notice, .placard, .wall-status, .wall-slot";
    const hits: string[] = [];
    for (const el of Array.from(document.querySelectorAll(sel))) {
      if (mm.contains(el) || el.contains(mm)) continue;
      const cs = getComputedStyle(el);
      if (cs.display === "none" || cs.visibility === "hidden") continue;
      let transparent = false;
      for (let p: Element | null = el; p && !transparent; p = p.parentElement) {
        if (Number(getComputedStyle(p).opacity) === 0) transparent = true;
      }
      if (transparent) continue;
      const b = visibleBox(el);
      if (b.right - b.left <= 0 || b.bottom - b.top <= 0) continue;
      const ix = Math.min(r.right, b.right) - Math.max(r.left, b.left);
      const iy = Math.min(r.bottom, b.bottom) - Math.max(r.top, b.top);
      if (ix > 0 && iy > 0) {
        hits.push(el.tagName.toLowerCase() + (typeof el.className === "string" && el.className ? "." + el.className.split(" ").join(".") : ""));
      }
    }
    return hits;
  });
}

test.describe("wall-canvas", () => {
  test("Tab reaches the toolbar and Enter activates its action (BL-173 (3))", async ({ page }) => {
    await openWritableWall(page);
    // Idle: the first action is the sticky; Enter opens the inline draft
    // the rail's button did (SI-350 (16)), and its Escape discards it.
    await tabUntil(page, "Tab reaches the toolbar", async () => (await activeAction(page)) !== null);
    expect(await activeAction(page)).toBe("sticky");
    await page.keyboard.press("Enter");
    const draft = page.locator(".sticky-draft");
    await expect(draft).toBeVisible();
    await expect(draft.getByRole("textbox", { name: "Sticky text" })).toBeFocused();
    await page.keyboard.press("Escape");
    await expect(draft).toHaveCount(0);

    // With a card selected from the keyboard, the toolbar's first action
    // is Edit; Enter opens the card editor, and Escape leaves the card
    // selected and focused.
    await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    const ac1 = page.getByTestId("card-ac-1");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await tabUntil(page, "Tab reaches the toolbar", async () => (await activeAction(page)) !== null);
    expect(await activeAction(page)).toBe("edit");
    await page.keyboard.press("Enter");
    await expect(editor(page)).toBeVisible();
    await expect(editor(page)).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(editor(page)).toHaveCount(0);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();
  });

  test("the keys rest where they do not belong: Delete inside an editor, the arrows under a confirmation or in a dialog", async ({ page }) => {
    await openWritableWall(page);
    await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    const ac1 = page.getByTestId("card-ac-1");
    await expect(ac1).toHaveAttribute("data-selected", "true");

    // Delete and Backspace inside the editor edit text; neither reaches
    // the card.
    await page.keyboard.press("Enter");
    await expect(editor(page)).toBeFocused();
    const original = await editor(page).inputValue();
    await page.keyboard.press("End");
    await page.keyboard.type(" xy");
    await page.keyboard.press("Backspace");
    await page.keyboard.press("ArrowLeft");
    await page.keyboard.press("Delete");
    await expect(editor(page)).toHaveValue(original + " ");
    await expect(confirm(page)).toBeHidden();
    await expect(editor(page)).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(editor(page)).toHaveCount(0);
    await expect(ac1.locator(".card-text")).toHaveText(original);

    // Under a confirmation (a modal), the arrows and Delete rest: the
    // selection stays and no second confirmation opens.
    await page.keyboard.press("Delete");
    await expect(confirm(page)).toBeVisible();
    const consequence = await confirm(page).locator("#edge-confirm-consequence").textContent();
    await page.keyboard.press("ArrowRight");
    await page.keyboard.press("ArrowDown");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Delete");
    await expect(confirm(page)).toBeVisible();
    expect(await confirm(page).locator("#edge-confirm-consequence").textContent()).toBe(consequence);
    await page.keyboard.press("Escape");
    await expect(confirm(page)).toBeHidden();
    await expect(ac1).toHaveAttribute("data-selected", "true");

    // In a dialog's own field (the pin tray's search), the arrows and
    // Delete are the field's; Escape closes the tray and nothing else.
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
    await tabUntil(page, "Tab reaches the pin action", focusOnAction(page, "pin"));
    await page.keyboard.press("Enter");
    const tray = page.getByRole("dialog", { name: "Pin an artifact" });
    await expect(tray).toBeVisible();
    await expect(page.locator("#pin-search")).toBeFocused();
    await page.keyboard.type("ab");
    await page.keyboard.press("ArrowLeft");
    await page.keyboard.press("Delete");
    await expect(page.locator("#pin-search")).toHaveValue("a");
    expect(await selectedKey(page)).toBeNull();
    await expect(confirm(page)).toBeHidden();
    await page.keyboard.press("Escape");
    await expect(tray).toBeHidden();
  });

  test("the minimap covers no content or control at 1440 px, 320 px and 200 % zoom (SI-358 (4); Wave 6 §5.2)", async ({ page }) => {
    for (const shape of [
      { width: 1440, height: 900, zoom: "" },
      { width: 320, height: 800, zoom: "" },
      { width: 720, height: 450, zoom: "200%" },
    ]) {
      const label = `${shape.width}×${shape.height}${shape.zoom ? " at " + shape.zoom : ""}`;
      await page.setViewportSize({ width: shape.width, height: shape.height });
      await openWritableWall(page);
      if (shape.zoom) {
        await page.evaluate((z) => {
          (document.body.style as unknown as { zoom: string }).zoom = z;
        }, shape.zoom);
        await page.evaluate(() => window.dispatchEvent(new Event("resize")));
      }
      const mm = minimap(page);
      await expect(mm).toBeVisible();
      await expect(mm.locator(".wall-minimap-view")).toHaveCount(1);
      // The minimap lives in the frame's row, outside the canvas's scroll
      // area, so no paper, label or control can meet it; the page never
      // scrolls sideways for it.
      expect(await mm.evaluate((el) => el.parentElement?.getAttribute("data-testid"))).toBe("wall-status-row");
      expect(await overlapsOf(page), `${label}: the minimap covers nothing with nothing selected`).toEqual([]);
      await page.getByTestId("card-dc-1").scrollIntoViewIfNeeded();
      await page.getByTestId("card-dc-1").click();
      await expect(page.getByTestId("card-dc-1")).toHaveAttribute("data-selected", "true");
      await expect(page.getByTestId("wall-status")).toBeAttached();
      expect(await overlapsOf(page), `${label}: the minimap covers nothing beside the pill and the card toolbar`).toEqual([]);
      const overflow = await page.evaluate(() => {
        const el = document.scrollingElement!;
        return el.scrollWidth - el.clientWidth;
      });
      expect(overflow, `${label}: no horizontal page scroll`).toBeLessThanOrEqual(1);
      const frame = (await page.getByTestId("wall-frame").boundingBox())!;
      const box = (await mm.boundingBox())!;
      expect(box.x, `${label}: the minimap sits inside the frame`).toBeGreaterThanOrEqual(frame.x - 0.5);
      expect(box.x + box.width, `${label}: the minimap sits inside the frame`).toBeLessThanOrEqual(frame.x + frame.width + 0.5);
    }
  });

  test("The keyboard and the minimap reach every card", async ({ page }) => {
    await page.setViewportSize(SMALL);
    await openWritableWall(page);
    // Every typed write goes over one route; what the keys post is read
    // here, so an edit is proven one operation and a cancel none.
    const posted: Array<{ request: { operations: Array<Record<string, unknown>> } }> = [];
    await page.route("**/api/mutate_draft", async (route) => {
      posted.push(route.request().postDataJSON());
      await route.continue();
    });
    const scrolls = await canvas(page).evaluate((c) => c.scrollWidth > c.clientWidth);
    expect(scrolls, "the wall is wider than the canvas's viewport, so a reveal is a real move").toBe(true);
    const ac1 = page.getByTestId("card-ac-1");
    const ac2 = page.getByTestId("card-ac-2");

    // -- The arrows (ac-6). Tab reaches the first card; the arrows select
    // from the focused card, and from then on from the selection.
    await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
    expect(await selectedKey(page)).toBeNull();
    const columns = await columnsOf(page);
    expect(columns.length, "a column per zone: ac, co, dc, oq, stub, reference, scratch").toBe(7);
    expect(columns[0]).toEqual(["card-ac-1", "card-ac-2"]);
    // Down and up stay in the column.
    await page.keyboard.press("ArrowDown");
    await expect(ac2).toHaveAttribute("data-selected", "true");
    await expect(ac2).toBeFocused();
    await expectRevealed(page, ac2, "ac-2 after ArrowDown");
    await page.keyboard.press("ArrowUp");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();
    await page.keyboard.press("ArrowUp");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    // Right crosses to every column in turn — the nearest card by
    // position — each revealed as the canvas scrolls under it; the far
    // columns start beyond the canvas's right edge.
    const far = (await page.getByTestId(columns[columns.length - 1][0]).boundingBox())!;
    const port = (await canvas(page).boundingBox())!;
    expect(far.x, "the scratch column starts beyond the canvas's right edge").toBeGreaterThan(port.x + port.width);
    const visited = ["card-ac-1"];
    for (let i = 1; i < columns.length; i++) {
      await page.keyboard.press("ArrowRight");
      const key = (await selected(page).getAttribute("data-testid"))!;
      expect(columns[i], `ArrowRight ${i} lands in column ${i}`).toContain(key);
      await expect(selected(page)).toBeFocused();
      await expectRevealed(page, selected(page), `${key} after ArrowRight`);
      visited.push(key);
    }
    expect(visited).toEqual([
      "card-ac-1",
      "card-co-1",
      "card-dc-1",
      "card-oq-1",
      stubCardTestId(WALL.STUB_SLUG),
      refCardTestId(WALL.ADR_REF),
      `sticky-${WALL.STICKY_ID}`,
    ]);
    // Left returns the same way, and the first column's nearest card is ac-1.
    await page.keyboard.press("ArrowLeft");
    expect(columns[columns.length - 2]).toContain((await selected(page).getAttribute("data-testid"))!);
    await expectRevealed(page, selected(page), "after ArrowLeft");
    for (let i = 0; i < columns.length; i++) await page.keyboard.press("ArrowLeft");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();
    await expectRevealed(page, ac1, "ac-1 after returning");

    // -- The minimap (ac-6): one block per card, the viewport as a frame
    // at the canvas's scale, and a drag of the frame that moves the
    // viewport. A pointer aid, hidden from assistive technology: the
    // arrows above are its keyboard path.
    const mm = minimap(page);
    await expect(mm).toBeVisible();
    // The arrows' reveal brought the whole frame into view, its row with
    // the canvas (the frame fits the viewport), so the minimap is on
    // screen without a scroll of the page.
    const mmBox = (await mm.boundingBox())!;
    expect(mmBox.y, "the minimap's top is in the viewport").toBeGreaterThanOrEqual(0);
    expect(mmBox.y + mmBox.height, "the minimap's bottom is in the viewport").toBeLessThanOrEqual(SMALL.height + 0.5);
    expect(await mm.getAttribute("aria-hidden")).toBe("true");
    expect(await mm.evaluate((el) => el.hasAttribute("tabindex"))).toBe(false);
    const cardsOnWall = await page.locator(CARD_SELECTOR).count();
    expect(cardsOnWall).toBe(8);
    let g = await minimapGeometry(page);
    expect(g.blocks.length, "one block per card").toBe(cardsOnWall);
    expect([...g.blocks].sort()).toEqual(["acceptance-criterion", "acceptance-criterion", "constraint", "decision", "open-question", "reference", "sticky", "stub"]);
    expect(g.view, "the viewport frame is drawn").not.toBeNull();
    near(g.view!.left, MINIMAP_PAD + g.scroll.left * g.scale, "frame left");
    near(g.view!.top, MINIMAP_PAD + g.scroll.top * g.scale, "frame top");
    near(g.view!.width, g.scroll.width * g.scale, "frame width");
    near(g.view!.height, g.scroll.height * g.scale, "frame height");
    const before = g.scroll;
    const scale = g.scale;
    const fb = (await mm.locator(".wall-minimap-view").boundingBox())!;
    const dx = 40;
    const dy = 12;
    await page.mouse.move(fb.x + fb.width / 2, fb.y + fb.height / 2);
    await page.mouse.down();
    await page.mouse.move(fb.x + fb.width / 2 + dx, fb.y + fb.height / 2 + dy, { steps: 4 });
    await page.mouse.up();
    g = await minimapGeometry(page);
    expect(g.scroll.left, "the viewport moved right").toBeGreaterThan(before.left);
    near(g.scroll.left, Math.max(0, Math.min(before.maxLeft, before.left + dx / scale)), "scrollLeft after the drag", 1 / scale + 1);
    near(g.scroll.top, Math.max(0, Math.min(before.maxTop, before.top + dy / scale)), "scrollTop after the drag", 1 / scale + 1);
    near(g.view!.left, MINIMAP_PAD + g.scroll.left * g.scale, "the frame follows the scroll");
    near(g.view!.top, MINIMAP_PAD + g.scroll.top * g.scale, "the frame follows the scroll");
    // The drag touched neither the selection nor the focus.
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();

    // -- Enter edits the selected card through the card editor; Enter
    // applies (ac-5, ac-6).
    await page.keyboard.press("Enter");
    await expect(editor(page)).toBeVisible();
    await expect(editor(page)).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    const original = await editor(page).inputValue();
    const edited = original + " — edited from the keyboard";
    await editor(page).fill(edited);
    await page.keyboard.press("Enter");
    await expectAutosaved(page);
    await expect(editor(page)).toHaveCount(0);
    expect(posted.length).toBe(1);
    expect(posted[0].request.operations).toEqual([expect.objectContaining({ op: "edit-ac", id: "ac-1", text: edited })]);
    await expect(ac1.locator(".card-text")).toHaveText(edited);
    await expect(ac1).toHaveAttribute("data-selected", "true");

    // -- Escape closes the innermost open thing, one layer per press: an
    // editor, then a confirmation, then a dialog, then the selection.
    // The editor alone: Escape cancels it, writes nothing and leaves the
    // selection and the focus.
    await page.keyboard.press("Enter");
    await expect(editor(page)).toBeVisible();
    await editor(page).fill("discarded by Escape");
    await page.keyboard.press("Escape");
    await expect(editor(page)).toHaveCount(0);
    await expect(ac1.locator(".card-text")).toHaveText(edited);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();
    // The selection alone: Escape clears it; the focus stays.
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
    await expect(ac1).toBeFocused();
    // Three layers at once: a dialog (the pin tray, opened from the idle
    // toolbar), the selection made under it, then an editor and then a
    // confirmation over both. Each Escape closes the innermost and
    // nothing else.
    await tabUntil(page, "Tab reaches the pin action", focusOnAction(page, "pin"));
    await page.keyboard.press("Enter");
    const tray = page.getByRole("dialog", { name: "Pin an artifact" });
    await expect(tray).toBeVisible();
    await expect(page.locator("#pin-search")).toBeFocused();
    await tabUntil(page, "Shift+Tab returns to ac-1", focusIs(page, "card-ac-1"), true);
    await expect(tray).toBeVisible();
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(tray).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(editor(page)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(editor(page)).toHaveCount(0);
    await expect(tray).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();
    await page.keyboard.press("Delete");
    await expect(confirm(page)).toBeVisible();
    await expect(confirm(page)).toHaveAttribute("aria-label", "Remove ac-1 from the spec");
    await page.keyboard.press("Escape");
    await expect(confirm(page)).toBeHidden();
    await expect(tray).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(tray).toBeHidden();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
    await expect(ac1).toBeFocused();
    expect(posted.length, "no Escape wrote anything").toBe(1);

    // -- Delete removes a card and a thread through the existing
    // confirmation (ac-6). The card: co-1, one column to the right.
    const co1 = page.getByTestId("card-co-1");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("ArrowRight");
    await expect(co1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Delete");
    await expect(confirm(page)).toBeVisible();
    await expect(confirm(page)).toHaveAttribute("aria-label", "Remove co-1 from the spec");
    await expect(confirm(page)).toContainText("Its body prose stays in the document");
    await tabUntil(page, "Tab reaches Confirm", focusIs(page, "edge-confirm-ok"));
    await page.keyboard.press("Enter");
    await expectAutosaved(page);
    await expect(co1).toHaveCount(0);
    expect(await selectedKey(page), "a selection whose card is gone clears itself").toBeNull();
    // The thread: the exempts edge from dc-1 to the ADR, selected from
    // its chip with Enter; the reference card hangs on that edge alone.
    const exempts = page.locator('.yarn-chip[data-edge-type="exempts"]');
    const ref = page.getByTestId(refCardTestId(WALL.ADR_REF));
    await expect(exempts).toHaveCount(1);
    await tabUntil(page, "Tab reaches the exempts chip", async () =>
      page.evaluate(() => document.activeElement?.getAttribute("data-edge-type") === "exempts"),
    );
    await page.keyboard.press("Enter");
    await expect(exempts).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Delete");
    await expect(confirm(page)).toBeVisible();
    await expect(confirm(page)).toHaveAttribute("aria-label", "Remove exempts");
    await tabUntil(page, "Tab reaches Confirm", focusIs(page, "edge-confirm-ok"));
    await page.keyboard.press("Enter");
    await expectAutosaved(page);
    await expect(exempts).toHaveCount(0);
    await expect(ref).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("card-co-1")).toHaveCount(0);
    await expect(page.locator('.yarn-chip[data-edge-type="exempts"]')).toHaveCount(0);
    await expect(page.getByTestId("card-ac-1").locator(".card-text")).toHaveText(edited);

    // -- The trash and Delete both refuse a declared stub in plain
    // language, in the same words from one source (dc-3; parent dc-13).
    const stub = page.getByTestId(stubCardTestId(WALL.STUB_SLUG));
    await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    for (let i = 0; i < 6 && (await stub.getAttribute("data-selected")) !== "true"; i++) {
      await page.keyboard.press("ArrowRight");
    }
    await expect(stub).toHaveAttribute("data-selected", "true");
    await expect(stub).toBeFocused();
    await expectRevealed(page, stub, "the stub, selected from the keyboard");
    await expect(wallToolbar(page).getByRole("button", { name: /delete/i })).toHaveCount(0);
    await page.keyboard.press("Delete");
    const refusal = page.getByRole("alertdialog", { name: /this stub stays/i });
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText("declared stub");
    await expect(refusal).toContainText("stubs block");
    await expect(refusal).toContainText(/not built yet/);
    await expect(refusal).toContainText(/edit the spec document/);
    await expect(page.locator("#edge-confirm-ok")).toBeHidden();
    const byKey = await page.locator("#edge-confirm-consequence").textContent();
    await page.keyboard.press("Escape");
    await expect(refusal).toBeHidden();
    await expect(stub).toHaveAttribute("data-selected", "true");
    // The trash, with the mouse, on the desktop viewport the raw drag needs.
    await page.setViewportSize(DESKTOP);
    await dragToTrash(page, stub);
    await expect(refusal).toBeVisible();
    expect(await page.locator("#edge-confirm-consequence").textContent(), "the trash's words are Delete's").toBe(byKey);
    await expect(page.locator("#edge-confirm-ok")).toBeHidden();
    await refusal.getByRole("button", { name: "Cancel" }).click();
    await expect(refusal).toBeHidden();
    await page.reload();
    await expect(page.getByTestId(stubCardTestId(WALL.STUB_SLUG))).toHaveCount(1);
    await expect(page.locator('.yarn-chip[data-edge-type="covers"]')).toHaveCount(1);
  });
});
