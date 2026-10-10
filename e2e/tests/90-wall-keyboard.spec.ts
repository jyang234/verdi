import { test, expect, type Page, type Locator } from "@playwright/test";
import { refCardTestId, stubCardTestId } from "./fixtures";
import { dragToTrash, expectAutosaved, grabPoint, wallToolbar } from "./helpers";

// spec/wall-canvas-v2 ac-6 (lane F2c): the keyboard reaches every card and
// action — the arrows move the selection on the handoff's grid, within a
// column and to the adjacent column at the same row (SI-363 (1)), and
// reveal the card; Enter edits; Escape closes the innermost open thing one
// layer per press (SI-363 (2)); Delete, and Backspace as the handoff's ⌫,
// removes the selected card or thread through the existing confirmation;
// and the trash and Delete both refuse a declared stub in plain language —
// and a minimap shows every card and the viewport and moves the viewport
// when dragged.
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

// interactionLive reads boardspec.js's hold contract (co-2).
const interactionLive = (page: Page) =>
  page.evaluate(() => (window as unknown as { __BOARDV2API__: { interactionLive: () => boolean } }).__BOARDV2API__.interactionLive());

// stickyPosition reads a sticky's stored position from its inline style,
// the server's own px.
const stickyPosition = (sticky: Locator) =>
  sticky.evaluate((el) => ({ x: parseFloat((el as HTMLElement).style.left), y: parseFloat((el as HTMLElement).style.top) }));

// sendStickyHome writes the sticky back to its home after a failed step,
// once the page's own write of the drag has settled, so that write cannot
// land after this one. Best effort: the failure that brought the test here
// is the one it reports.
async function sendStickyHome(page: Page, home: { x: number; y: number }): Promise<void> {
  try {
    await expect(page.getByTestId("autosave-status")).not.toHaveText("saving…", { timeout: 10_000 });
  } catch {
    // Still saving: the write home goes anyway.
  }
  try {
    await page.request.post(WALL.WRITABLE_PATH + "/api/sticky-position", { data: { id: WALL.STICKY_ID, x: home.x, y: home.y } });
  } catch {
    // The page is gone; nothing more can be sent from here.
  }
}

// forceSwap asks the transport for a fresh projection past its conditional
// token and waits for the region swap it applies (boardspecasd.js
// refresh(force); the `wall-region-swapped` event), writing nothing.
const forceSwap = (page: Page) =>
  page.evaluate(
    () =>
      new Promise<string>((resolve) => {
        const t = setTimeout(() => resolve("no swap"), 8000);
        document.addEventListener(
          "wall-region-swapped",
          () => {
            clearTimeout(t);
            setTimeout(() => resolve("swapped"), 50);
          },
          { once: true },
        );
        (window as unknown as { __verdiASD: { refresh: (force: boolean) => Promise<void> } }).__verdiASD.refresh(true);
      }),
  );

// gridOf reads the zone grid the keyboard follows: the server's zone
// labels, left to right, each a column's left edge and width.
const gridOf = (page: Page) =>
  page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLElement>("#board-canvas .zone-label"))
      .map((l) => ({ kind: (l.getAttribute("data-testid") || "").replace("zone-label-", ""), x: parseFloat(l.style.left), w: parseFloat(l.style.width) }))
      .sort((a, b) => a.x - b.x),
  );

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

// minimapGeometry reads the minimap against the canvas, in one step so a
// rebuild cannot slip between two reads: the scale the asset derives (the
// frame's inner box over the canvas's scroll size), the viewport frame's
// box inside the minimap and its centre in the viewport, the canvas's
// scroll state, and the blocks drawn.
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
      centre: vr ? { x: vr.left + vr.width / 2, y: vr.top + vr.height / 2 } : null,
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

  test("every card is reachable by the arrows, on the grid and off it (SI-363 (1))", async ({ page }) => {
    // The handoff's model: the zone columns left to right, each card in
    // the column nearest its centre, rows by y; Up and Down within the
    // column, Left and Right to the adjacent column at the same row,
    // clamped. A breadth-first search over real arrow presses from the
    // first card must reach every card: the fixture's eight, the sticky
    // dragged off the grid so its box straddles the stub and reference
    // bands, and a twin paper with the very centre of that sticky — a
    // DOM-only clone, since the server's display-time collision
    // resolution (boardlayout/display.go) nudges two stored overlapping
    // papers apart, so no stored wall can hold two equal centres.
    await page.setViewportSize(DESKTOP);
    await openWritableWall(page);
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
    const home = await stickyPosition(sticky);
    const grid = await gridOf(page);
    const col = (kind: string) => grid.find((g) => g.kind === kind)!;
    expect(grid.map((g) => g.kind)).toEqual(["acceptance-criterion", "constraint", "decision", "open-question", "stub", "reference", "scratch"]);
    const stubBand = col("stub");
    const refBand = col("reference");
    // The drag writes the sticky's new place to the store every later test
    // of this file reads, and the test's last step writes it home; a step
    // failing between them — an autosave past its bound under load
    // (BL-154) — would leave the sticky off its lane, and the last test
    // would read it as a column of its own (90:880). So the way home is
    // also taken on the way out.
    let restored = false;
    try {
      // A raw drag never scrolls: the sticky comes into view first, then
      // the pointer carries it by the distance to its new place.
      const target = { x: stubBand.x + stubBand.w - 97, y: home.y + 160 };
      const grip = await grabPoint(page, sticky);
      await page.mouse.move(grip.x, grip.y);
      await page.mouse.down();
      await page.mouse.move(grip.x + (target.x - home.x), grip.y + (target.y - home.y), { steps: 8 });
      await page.mouse.up();
      await expectAutosaved(page);
      const moved = await stickyPosition(sticky);
      const width = await sticky.evaluate((el) => (el as HTMLElement).offsetWidth);
      expect(moved.x, "the sticky left the scratch lane").not.toBe(home.x);
      expect(moved.x, "the sticky's box reaches into the stub band").toBeLessThan(stubBand.x + stubBand.w);
      expect(moved.x + width, "the sticky's box reaches into the reference band").toBeGreaterThan(refBand.x);
      const stickyCentre = moved.x + width / 2;
      expect(Math.abs(stickyCentre - (stubBand.x + stubBand.w / 2)), "nearer the stub column than the reference column by its centre").toBeLessThan(
        Math.abs(stickyCentre - (refBand.x + refBand.w / 2)),
      );
      // The twin: the same box, so the same centre, in the document after it.
      const twinKey = "a-twin";
      await page.evaluate(
        ([id, key]) => {
          const el = document.querySelector<HTMLElement>(`[data-testid="sticky-${id}"]`)!;
          const twin = el.cloneNode(true) as HTMLElement;
          twin.removeAttribute("data-selected");
          twin.removeAttribute("tabindex");
          twin.setAttribute("data-id", key);
          twin.setAttribute("data-testid", `sticky-${key}`);
          el.parentElement!.appendChild(twin);
        },
        [WALL.STICKY_ID, twinKey],
      );
      const keys: string[] = await page.evaluate((sel) => {
        const seam = (window as unknown as { __WALLSELECT__: { keyOf: (el: Element) => { key: string } } }).__WALLSELECT__;
        return Array.from(document.querySelectorAll(sel))
          .filter((el) => !el.classList.contains("sticky-draft"))
          .map((el) => seam.keyOf(el).key);
      }, CARD_SELECTOR);
      expect(keys.length).toBe(9);
      expect(keys[0]).toBe("ac-1");
      // The search. Each press is a real keystroke from the card it starts
      // on, which the seam selects and focuses first — the state a Tab stop
      // and an arrow leave — so every edge is the keyboard's own.
      const place = (key: string) =>
        page.evaluate((k) => {
          const w = window as unknown as {
            __WALLSELECT__: { select: (s: { kind: string; key: string }) => void; elementOf: (s: { kind: string; key: string }) => HTMLElement | null };
          };
          w.__WALLSELECT__.select({ kind: "card", key: k });
          const el = w.__WALLSELECT__.elementOf({ kind: "card", key: k })!;
          if (!el.hasAttribute("tabindex")) el.setAttribute("tabindex", "-1");
          el.focus({ preventScroll: true });
        }, key);
      const current = () =>
        page.evaluate(() => {
          const s = (window as unknown as { __WALLSELECT__: { selection: () => { key: string } | null } }).__WALLSELECT__.selection();
          return s ? s.key : null;
        });
      const reached = new Set<string>([keys[0]]);
      const queue = [keys[0]];
      const edges = new Map<string, string | null>();
      while (queue.length) {
        const k = queue.shift()!;
        for (const arrow of ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"]) {
          await place(k);
          await page.keyboard.press(arrow);
          const n = await current();
          edges.set(`${k} ${arrow}`, n);
          if (n && !reached.has(n)) {
            reached.add(n);
            queue.push(n);
          }
        }
      }
      expect(keys.filter((k) => !reached.has(k)), "cards the arrows never reach").toEqual([]);
      // The model's edges: the straddling sticky joined the stub column
      // below the stub; the twin, with the same centre, is the next row by
      // document order; Left and Right keep the row, clamped to the
      // neighbouring column's length.
      const stubKey = `stub:${WALL.STUB_SLUG}`;
      expect(edges.get(`${stubKey} ArrowDown`)).toBe(WALL.STICKY_ID);
      expect(edges.get(`${WALL.STICKY_ID} ArrowDown`)).toBe(twinKey);
      expect(edges.get(`${twinKey} ArrowUp`)).toBe(WALL.STICKY_ID);
      expect(edges.get(`${WALL.STICKY_ID} ArrowUp`)).toBe(stubKey);
      expect(edges.get(`${twinKey} ArrowRight`)).toBe(WALL.ADR_REF);
      expect(edges.get(`${twinKey} ArrowLeft`)).toBe("oq-1");
      expect(edges.get(`${WALL.ADR_REF} ArrowLeft`)).toBe(stubKey);
      expect(edges.get(`oq-1 ArrowRight`)).toBe(stubKey);
      expect(edges.get(`ac-2 ArrowRight`), "co-1 is the constraint column's only row").toBe("co-1");
      expect(edges.get(`ac-1 ArrowLeft`), "the wall's edge").toBe("ac-1");
      // The wall as it was: the twin leaves the document and the sticky
      // returns to the scratch lane through the write the wall accepts.
      await page.evaluate((key) => document.querySelector(`[data-testid="sticky-${key}"]`)?.remove(), twinKey);
      const back = await page.request.post(WALL.WRITABLE_PATH + "/api/sticky-position", { data: { id: WALL.STICKY_ID, x: home.x, y: home.y } });
      expect(back.status(), await back.text()).toBe(200);
      restored = true;
      await page.reload();
      expect(await stickyPosition(page.getByTestId(`sticky-${WALL.STICKY_ID}`))).toEqual(home);
      await expect(page.locator(CARD_SELECTOR)).toHaveCount(8);
    } finally {
      if (!restored) await sendStickyHome(page, home);
    }
  });

  test("Escape closes the branch menu, leaves the posture popover its own key, and cancels an unfocused slot or draft before the selection (SI-363 (2))", async ({ page }) => {
    await openWritableWall(page);
    const ac1 = page.getByTestId("card-ac-1");
    const select = async () => {
      await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
      await page.keyboard.press("ArrowDown");
      await page.keyboard.press("ArrowUp");
      await expect(ac1).toHaveAttribute("data-selected", "true");
    };

    // The branch menu has no backdrop: Escape closes it, and only then
    // the selection.
    await select();
    await tabUntil(page, "Tab reaches the branch switcher", focusIs(page, "branch-switcher"));
    await page.keyboard.press("Enter");
    const menu = page.locator("#branch-menu");
    await expect(menu).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(menu).toBeHidden();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // The top bar's posture popover: Enter on its focused summary toggles
    // it and opens no editor (F2CR-4); its Escape is topbar.js's, which
    // closes it and returns the focus to the summary, not also a clear.
    await select();
    await tabUntil(page, "Shift+Tab reaches the posture summary", focusIs(page, "topbar-posture"), true);
    const popover = page.getByTestId("asd-posture-tech");
    await page.keyboard.press("Enter");
    await expect(popover).toHaveJSProperty("open", true);
    await expect(editor(page)).toHaveCount(0);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(popover).toHaveJSProperty("open", false);
    await expect(page.getByTestId("topbar-posture")).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // An open add slot whose focus has left (its text keeps it open and
    // holds the projection): Escape cancels it first, leaving the focus
    // where it was; the next Escape clears the selection.
    await select();
    await tabUntil(page, "Tab reaches the constraint slot", focusIs(page, "slot-open-co"));
    await page.keyboard.press("Enter");
    const coSlot = page.getByTestId("slot-co");
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await expect(page.getByTestId("slot-text-co")).toBeFocused();
    await page.keyboard.type("kept until Escape");
    await tabUntil(page, "Shift+Tab returns to ac-1", focusIs(page, "card-ac-1"), true);
    await expect(coSlot).toHaveAttribute("data-open", "true");
    expect(await interactionLive(page)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(coSlot).not.toHaveAttribute("data-open", /./);
    expect(await interactionLive(page)).toBe(false);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(ac1).toBeFocused();
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // A sticky draft whose focus has left (text without a type keeps it,
    // with its hint): Escape discards it, and nothing is written.
    const stickies = await page.locator('[data-testid^="sticky-"]').count();
    await tabUntil(page, "Tab reaches the sticky action", focusOnAction(page, "sticky"));
    await page.keyboard.press("Enter");
    const draft = page.locator(".sticky-draft");
    await expect(draft.getByRole("textbox", { name: "Sticky text" })).toBeFocused();
    await page.keyboard.type("unsent");
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => !!document.activeElement?.closest(".sticky-draft"))).toBe(false);
    await expect(draft).toBeVisible();
    await expect(draft.getByTestId("sticky-type-hint")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(draft).toHaveCount(0);
    expect(await interactionLive(page)).toBe(false);
    await page.reload();
    await expect(page.locator('[data-testid^="sticky-"]')).toHaveCount(stickies);

    // -- Combined layers, one press closing exactly one, in SI-363 (2)'s
    // amended order (the closure check's R-1 and R-2).
    // The branch menu with the focus inside it, on a menuitem.
    await select();
    await tabUntil(page, "Tab reaches the branch switcher", focusIs(page, "branch-switcher"));
    await page.keyboard.press("Enter");
    await expect(menu).toBeVisible();
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement?.closest("#branch-menu") !== null && document.activeElement?.getAttribute("role") === "menuitem")).toBe(true);
    await page.keyboard.press("Escape");
    await expect(menu).toBeHidden();
    // The focus the hidden item would have kept returns to the switcher.
    await expect(page.getByTestId("branch-switcher")).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // openSlotWithText opens the constraint slot from the keyboard and
    // types into it; the text keeps it open once the focus leaves.
    const openSlotWithText = async (text: string) => {
      await tabUntil(page, "Tab reaches the constraint slot", focusIs(page, "slot-open-co"));
      await page.keyboard.press("Enter");
      await expect(page.getByTestId("slot-text-co")).toBeFocused();
      await page.keyboard.type(text);
    };

    // A slot with text under the posture popover: the first press closes
    // only the popover, through topbar.js, and the text survives; the
    // second cancels the slot; the third clears the selection.
    await select();
    await openSlotWithText("survives the popover");
    await tabUntil(page, "Shift+Tab reaches the posture summary", focusIs(page, "topbar-posture"), true);
    await page.keyboard.press("Enter");
    await expect(popover).toHaveJSProperty("open", true);
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await page.keyboard.press("Escape");
    await expect(popover).toHaveJSProperty("open", false);
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await expect(page.getByTestId("slot-text-co")).toHaveValue("survives the popover");
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(coSlot).not.toHaveAttribute("data-open", /./);
    expect(await interactionLive(page)).toBe(false);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // The same under a reference card's peek, which a click opens (and
    // which selects the card): the peek closes first, through
    // boardspec.js, the text survives; then the slot; then the selection.
    await select();
    await openSlotWithText("survives the peek");
    const ref = page.getByTestId(refCardTestId(WALL.ADR_REF));
    await ref.scrollIntoViewIfNeeded();
    await ref.click();
    await expect(ref).toHaveAttribute("data-selected", "true");
    const peek = page.locator("#ref-peek");
    await expect(peek).toBeVisible();
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await page.keyboard.press("Escape");
    await expect(peek).toHaveCount(0);
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await expect(page.getByTestId("slot-text-co")).toHaveValue("survives the peek");
    await expect(ref).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(coSlot).not.toHaveAttribute("data-open", /./);
    expect(await interactionLive(page)).toBe(false);
    await expect(ref).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
  });

  test("Escape closes the picker, a menu, the drawer, then the strip editor or a popover, before the selection — one layer a press (SI-368 (17))", async ({ page }) => {
    test.setTimeout(150_000);
    await openWritableWall(page);
    const ac1 = page.getByTestId("card-ac-1");
    const select = async () => {
      await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
      await page.keyboard.press("ArrowDown");
      await page.keyboard.press("ArrowUp");
      await expect(ac1).toHaveAttribute("data-selected", "true");
    };
    const drawer = page.getByTestId("record-drawer");
    const more = page.getByTestId("wall-more-menu");
    const writes: string[] = [];
    // Every write through the board's api; the drawer's and the menu's
    // on-demand reads are not writes.
    page.on("request", (r) => {
      if (r.method() === "POST" && /\/api\//.test(r.url()) && !/\/api\/(get_design_|prepare_design_review|get_board)/.test(r.url())) writes.push(r.url());
    });

    // Commit and push's popover over the selection: the first press closes
    // only the popover — it used to clear the selection with it (F3AR-6) —
    // and the second clears the selection.
    const commit = page.getByTestId("wall-commit-popover");
    await select();
    await tabUntil(page, "Shift+Tab reaches the Commit count", focusIs(page, "wall-commit-count"), true);
    await expect(commit).toHaveJSProperty("open", true);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(commit).toHaveJSProperty("open", false);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // The ⋯ menu over the drawer over a strip editor whose focus has left,
    // over the selection: each press closes one, in that order, the drawer
    // wherever the focus is, and the editor's typed text is never written.
    const onProblemHeadline = async () =>
      page.evaluate(() => !!document.activeElement?.matches('[data-testid="placard-problem"] > .placard-text'));
    const stripEditor = page.getByTestId("case-strip-editor-problem");
    await select();
    await tabUntil(page, "Shift+Tab reaches the problem's headline", onProblemHeadline, true);
    await page.keyboard.press("Enter");
    await expect(page.getByTestId("case-strip-text-problem")).toBeFocused();
    await page.keyboard.press("End");
    await page.keyboard.type(" [90-escape]");
    await tabUntil(page, "Shift+Tab reaches the readiness pill", focusIs(page, "readiness-pill"), true);
    await expect(stripEditor).toBeVisible();
    await page.keyboard.press("Enter");
    await expect(drawer).toBeVisible();
    await tabUntil(page, "Shift+Tab reaches ⋯", focusIs(page, "wall-more"), true, 300);
    await page.keyboard.press("Enter");
    await expect(more).toBeVisible();
    await expect(drawer).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(more).toBeHidden();
    await expect(page.getByTestId("wall-more")).toBeFocused();
    await expect(drawer).toBeVisible();
    await expect(stripEditor).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(stripEditor).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(stripEditor).toHaveCount(0);
    expect(await interactionLive(page)).toBe(false);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
    await expect(page.getByTestId("placard-problem")).not.toContainText("[90-escape]");

    // The type picker over the drawer over a selected thread: the picker
    // closes first, then the drawer, then the selection.
    const chip = page.getByTestId(`yarn-chip-spec-exempts-dc-1-${WALL.ADR_REF}`);
    await tabUntil(page, "Tab reaches the exempts thread", focusIs(page, `yarn-chip-spec-exempts-dc-1-${WALL.ADR_REF}`));
    await page.keyboard.press("Enter");
    await expect(chip).toHaveAttribute("data-selected", "true");
    await tabUntil(page, "Shift+Tab reaches the readiness pill", focusIs(page, "readiness-pill"), true);
    await page.keyboard.press("Enter");
    await expect(drawer).toBeVisible();
    await tabUntil(page, "Shift+Tab reaches Retype", focusOnAction(page, "retype"), true, 300);
    await page.keyboard.press("Enter");
    const picker = page.locator("#edge-picker");
    await expect(picker).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(picker).toBeHidden();
    await expect(drawer).toBeVisible();
    await expect(chip).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(chip).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // A press that closes a layer returns the focus to that layer's opener
    // or leaves it where the user put it, and never leaves an open layer
    // without the focus (SI-368 (30), F3CR-5). The posture popover over the
    // drawer over the selection: the first press shuts the drawer and the
    // focus stays on the popover's summary, the popover still open; the
    // second shuts the popover, its summary keeping the focus; the third
    // clears the selection.
    const posture = page.getByTestId("asd-posture-tech");
    const postureSummary = page.getByTestId("topbar-posture");
    await select();
    await tabUntil(page, "Shift+Tab reaches the readiness pill", focusIs(page, "readiness-pill"), true);
    await page.keyboard.press("Enter");
    await expect(drawer).toBeVisible();
    await tabUntil(page, "Shift+Tab reaches the posture's summary", focusIs(page, "topbar-posture"), true, 300);
    await page.keyboard.press("Enter");
    await expect(posture).toHaveJSProperty("open", true);
    await expect(drawer).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(posture).toHaveJSProperty("open", true);
    await expect(postureSummary).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(posture).toHaveJSProperty("open", false);
    await expect(postureSummary).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();

    // The drawer opened from ⋯, the focus then moved onto a card: the press
    // shuts the drawer and the focus stays on the card, never pulled back
    // to ⋯; the next clears the selection, the card keeping the focus.
    await select();
    await tabUntil(page, "Shift+Tab reaches ⋯", focusIs(page, "wall-more"), true, 300);
    await page.keyboard.press("Enter");
    await expect(more).toBeVisible();
    for (let i = 0; i < 8 && !(await focusIs(page, "wall-more-provenance")()); i++) await page.keyboard.press("ArrowDown");
    await expect(page.getByTestId("wall-more-provenance")).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(drawer).toBeVisible();
    await expect(page.getByTestId("record-tab-provenance")).toHaveAttribute("aria-selected", "true");
    await tabUntil(page, "Tab reaches ac-1 from the drawer", focusIs(page, "card-ac-1"), false, 300);
    await expect(drawer).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(ac1).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
    await expect(ac1).toBeFocused();

    // The add-object dialog, from the toolbar's Card: one press closes it,
    // backdrop and dialog together, and ends the interaction that held the
    // projection (BL-175 (1)).
    await tabUntil(page, "Tab reaches the Card action", focusOnAction(page, "card"));
    await page.keyboard.press("Enter");
    const dialog = page.locator("#asd-op-dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId("asd-op-text")).toBeFocused();
    await page.keyboard.type("never declared [90-escape]");
    expect(await interactionLive(page)).toBe(true);
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await expect(page.locator("#modal-backdrop")).toBeHidden();
    expect(await interactionLive(page)).toBe(false);

    expect(writes, "no layer an Escape closed wrote anything").toEqual([]);
  });

  test("a Tab past the Commit count shuts its popover with the press, so the next Escape clears the selection (SI-368 (17))", async ({ page }) => {
    // The popover opens on the count's focus and shuts when the focus
    // leaves it (wallstrip.js). Shut on a timer instead, it stood open
    // after the focus had moved on: the browser ran the timer behind the
    // keys that followed (0 to 22 presses later, measured over this
    // suite's back-to-back keys), and an Escape among them closed that
    // popover, a layer the user had left, instead of clearing the
    // selection (the next test's intermittent "card:stub:notice-
    // retraction", 3 runs of its flow in 100). Each pass reads the
    // popover once, right after the Tab, never polled.
    await openWritableWall(page);
    const ac1 = page.getByTestId("card-ac-1");
    const commit = page.getByTestId("wall-commit-popover");
    for (let pass = 0; pass < 5; pass++) {
      await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
      await page.keyboard.press("ArrowDown");
      await page.keyboard.press("ArrowUp");
      await expect(ac1).toHaveAttribute("data-selected", "true");
      await tabUntil(page, "Shift+Tab reaches the Commit count", focusIs(page, "wall-commit-count"), true);
      await expect(commit).toHaveJSProperty("open", true);
      await page.keyboard.press("Tab");
      const after = await commit.evaluate((d) => {
        const a = document.activeElement;
        return { open: (d as HTMLDetailsElement).open, left: !!a && a !== document.body && !d.contains(a), active: a ? a.getAttribute("data-testid") || a.tagName : null };
      });
      expect(after.left, `pass ${pass}: the Tab moved the focus out of the popover (to ${after.active})`).toBe(true);
      expect(after.open, `pass ${pass}: the popover shut with the press that moved the focus to ${after.active}`).toBe(false);
      await page.keyboard.press("Escape");
      expect(await selectedKey(page), `pass ${pass}: the next Escape clears the selection`).toBeNull();
    }
  });

  test("a Shift+Tab off the branch switcher shuts its menu with the press, so the next Escape clears the selection (SI-368 (17), (33))", async ({ page }) => {
    // The menu opens on Enter at the switcher and shuts when the focus
    // leaves both (wallstrip.js). Shut on a timer instead, it stood open
    // after the focus had moved on: the browser ran the timer behind the
    // next key, so an Escape pressed right after the Shift+Tab closed the
    // menu, a layer the user had left, and the selection survived it
    // (F3G3R-1, 10 runs in 10). Each pass presses Shift+Tab and Escape
    // back to back and reads the outcome once, never polled.
    await openWritableWall(page);
    const ac1 = page.getByTestId("card-ac-1");
    const menu = page.locator("#branch-menu");
    for (let pass = 0; pass < 5; pass++) {
      await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
      await page.keyboard.press("ArrowDown");
      await page.keyboard.press("ArrowUp");
      await expect(ac1).toHaveAttribute("data-selected", "true");
      await tabUntil(page, "Shift+Tab reaches the branch switcher", focusIs(page, "branch-switcher"), true);
      await page.keyboard.press("Enter");
      await expect(menu).toBeVisible();
      await page.keyboard.press("Shift+Tab");
      await page.keyboard.press("Escape");
      const after = await menu.evaluate((m) => {
        const a = document.activeElement;
        const left = !!a && a !== document.body && !m.contains(a) && !a.closest('[data-testid="branch-switcher"]');
        return { hidden: (m as HTMLElement).hidden, left, active: a ? a.getAttribute("data-testid") || a.tagName : null };
      });
      expect(after.left, `pass ${pass}: the Shift+Tab moved the focus off the switcher and out of the menu (to ${after.active})`).toBe(true);
      expect(await selectedKey(page), `pass ${pass}: the Escape after the Shift+Tab clears the selection`).toBeNull();
      expect(after.hidden, `pass ${pass}: the menu is shut`).toBe(true);
    }
  });

  test("after Escape clears the selection, the focused card survives a region swap (Wave 6 §5.1)", async ({ page }) => {
    await openWritableWall(page);
    const stub = page.getByTestId(stubCardTestId(WALL.STUB_SLUG));
    await tabUntil(page, "Tab reaches ac-1", focusIs(page, "card-ac-1"));
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    for (let i = 0; i < 6 && (await stub.getAttribute("data-selected")) !== "true"; i++) {
      await page.keyboard.press("ArrowRight");
    }
    await expect(stub).toBeFocused();
    await page.keyboard.press("Escape");
    expect(await selectedKey(page)).toBeNull();
    await expect(stub).toBeFocused();
    expect(await forceSwap(page)).toBe("swapped");
    await expect(page.getByTestId(stubCardTestId(WALL.STUB_SLUG))).toBeFocused();
    await page.keyboard.press("ArrowRight");
    await expect(page.getByTestId(refCardTestId(WALL.ADR_REF))).toHaveAttribute("data-selected", "true");
    await expect(page.getByTestId(refCardTestId(WALL.ADR_REF))).toBeFocused();
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
    // Right crosses to the adjacent column in turn, at the same row
    // (SI-363 (1)), each card revealed as the canvas scrolls under it;
    // the far columns start beyond the canvas's right edge.
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
    const from = g.centre!;
    const dx = 40;
    const dy = 12;
    await page.mouse.move(from.x, from.y);
    await page.mouse.down();
    await page.mouse.move(from.x + dx, from.y + dy, { steps: 4 });
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
    // Backspace deletes as Delete does (the handoff's ⌫/Del; SI-363 (3)).
    await page.keyboard.press("Backspace");
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
