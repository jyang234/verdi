import { test, expect, type Page, type Locator } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { boardPath, coverageChipTestId, refCardTestId, slotChipTestId, stubCardTestId } from "./fixtures";
import { transformRotates } from "./helpers";

// spec/wall-canvas-v2 ac-1 and ac-2 (lane F2a): every card renders at the
// design's footprint, unrotated, keeping its receipts' exact texts; the
// stub's slug is its first line, inside the card; yarn draws in two layers
// — every thread under the cards, the selection's threads above them; and
// clicking a card selects it, emphasizes its threads, recedes the rest, and
// names it in the status pill.
//
// The tests are the producers their obligations name
// (.verdi/obligations/wall-canvas-v2/ac-1--behavioral.md,
// ac-2--behavioral.md), titled exactly as the claims spell them under the
// obligations' `wall-canvas ›` prefix (SI-350 (12)). ac-1's producer is
// HELD (test.fixme, SI-355): its claim includes the readiness mark, which
// waits on the owner's decision (SI-352); the same assertion routine runs
// unheld under a second title no obligation names, so the cards, receipts
// and yarn layers are exercised in every run. The lane that lands the mark
// adds its assertion to the held test, removes the hold, and deletes the
// second test, in one change (SI-355 (3)).
//
// The wall is this file's own fixture (cmd/e2eharness/provision_board.go,
// canvasWallSpecName), served from the serving branch in authoring mode
// under its domain refusal: it carries every card kind and every receipt,
// and offers no pins and no typed writes. Its constants live here, not in
// fixtures.ts (SI-350 (11)); a change there changes them together. The file
// passes when run alone (BL-98) and assumes nothing another file wrote.
// Every assertion reads the DOM and computed styles — never a screenshot
// (recording stays off).
//
// After the lane's review (SI-358): the emphasis holds under hover and a
// receded card keeps its text legible and its focus ring visible; the
// status pill is drawn in the wall frame's reserved row below the canvas,
// in view whenever the frame fills the viewport, and covers nothing at
// 1440 px, 320 px and 200 % zoom — beside a tall sticky and under a paper
// mid-drag; the pill's words reach a live region outside the swapped
// region, never re-announced across a slow double click; and the wall
// passes the accessibility scan with a selection active, in light and
// dark. After the closure check: the canvas is bounded by the viewport
// alone (never by where the frame sits on the page), a region swap keeps
// its scroll on both axes, and the pill's summary stays readable at 320 px
// in every mode.

// The selection asset's bound (wallselect.js measure), mirrored here so the
// tests state the contract in numbers: the canvas is the least of its
// content height and the viewport's height less the status row and this
// margin, never under one card (boardlayout CardHeight, 140 px) plus a
// 10 px margin above and below it.
const VIEW_MARGIN = 16;
const CANVAS_FLOOR = 160;

const WALL = {
  SPEC: "decline-canvas-wall",
  STUB_SLUG: "notice-retraction",
  ADR_REF: "adr/0001-outbox-events",
  STICKY_ID: "a-01J8Z0K3CANVASSTCKY0000001",
  STICKY_BODY: "who signs off the retraction copy?",
  OBLIGATION_TITLE: "a Playwright test retracts a stale notice on every channel",
};

const CARD_IDS = ["ac-1", "ac-2", "co-1", "dc-1", "oq-1"];

// offsetRect reads an element's canvas-coordinate box, the frame the yarn
// paths are drawn in (offset* against the positioned canvas).
async function offsetRect(el: Locator): Promise<{ x: number; y: number; w: number; h: number }> {
  return el.evaluate((node) => {
    const h = node as HTMLElement;
    return { x: h.offsetLeft, y: h.offsetTop, w: h.offsetWidth, h: h.offsetHeight };
  });
}

// quad parses one yarn path's "M ax ay Q cx cy bx by".
function quad(d: string): { ax: number; ay: number; bx: number; by: number } {
  const m = d.match(/^M (-?[\d.]+) (-?[\d.]+) Q (-?[\d.]+) (-?[\d.]+) (-?[\d.]+) (-?[\d.]+)$/);
  expect(m, `thread path is not a single quadratic curve: ${d}`).not.toBeNull();
  const [, ax, ay, , , bx, by] = m!;
  return { ax: Number(ax), ay: Number(ay), bx: Number(bx), by: Number(by) };
}

async function expectFootprint(el: Locator, what: string, w: number, h: number | null): Promise<void> {
  const box = await el.boundingBox();
  expect(box, `${what} has no layout box`).not.toBeNull();
  expect(Math.abs(box!.width - w), `${what} width ${box!.width}, want ${w}`).toBeLessThanOrEqual(0.5);
  if (h === null) {
    expect(box!.height, `${what} height`).toBeGreaterThanOrEqual(112);
  } else {
    expect(Math.abs(box!.height - h), `${what} height ${box!.height}, want ${h}`).toBeLessThanOrEqual(0.5);
  }
  const transform = await el.evaluate((node) => getComputedStyle(node).transform);
  expect(transformRotates(transform), `${what} is rotated: ${transform}`).toBe(false);
}

const canvas = (page: Page) => page.getByTestId("board");
// The visual pill, in the frame's row while something is selected, and the
// live region outside the swapped region that speaks its words.
const pill = (page: Page) => page.getByTestId("wall-status");
const live = (page: Page) => page.getByTestId("wall-status-live");
const overlayThreads = (page: Page) => page.locator("#board-canvas svg.yarn-overlay path.yarn-thread");
const baseThreads = (page: Page) => page.locator("#board-canvas svg.yarn-svg path.yarn-thread");

// frameIntoView scrolls the page so the wall frame's top meets the
// viewport's: the frame is bounded to the viewport, so the frame then fills
// it and its row is in view, with the canvas scrolling inside itself.
const frameIntoView = (page: Page) =>
  page.getByTestId("wall-frame").evaluate((el) => el.scrollIntoView({ block: "start" }));

// scrollOf reads the canvas's scroll offsets, rounded.
const scrollOf = (page: Page) => canvas(page).evaluate((el) => [Math.round(el.scrollLeft), Math.round(el.scrollTop)]);

// parkSticky moves this wall's one sticky through the scratch write the
// wall accepts and waits for the swap that carries it: the poll's region
// swap is the one under test wherever a test forces a swap.
async function parkSticky(page: Page, x: number, y: number): Promise<void> {
  const moved = await page.request.post(boardPath(WALL.SPEC) + "/api/sticky-position", {
    data: { id: WALL.STICKY_ID, x, y },
  });
  expect(moved.status(), await moved.text()).toBe(200);
  await expect(page.getByTestId(`sticky-${WALL.STICKY_ID}`)).toHaveCSS("top", `${y}px`, { timeout: 8_000 });
}

// Computed-style readers: the ring is the first shadow in the list, a
// spread with no blur ("0px 0px 0px 3px"); the recede is a grayscale
// filter with the shadow gone.
const shadowOf = (el: Locator) => el.evaluate((node) => getComputedStyle(node).boxShadow);
const filterOf = (el: Locator) => el.evaluate((node) => getComputedStyle(node).filter);
const opacityOf = (el: Locator) => el.evaluate((node) => Number(getComputedStyle(node).opacity));
const RING_3 = /(^|, )rgba?\([^)]*\) 0px 0px 0px 3px/;
const RING_2 = /(^|, )rgba?\([^)]*\) 0px 0px 0px 2px/;

async function expectReceded(el: Locator, what: string): Promise<void> {
  await expect.poll(() => filterOf(el), `${what} recedes`).toBe("grayscale(1)");
  await expect.poll(() => shadowOf(el), `${what} drops its shadow`).toBe("none");
  await expect.poll(() => opacityOf(el), `${what} keeps its ink`).toBe(1);
}

async function expectEmphasized(el: Locator, what: string, ring: RegExp): Promise<void> {
  await expect.poll(() => filterOf(el), `${what} is not receded`).toBe("none");
  await expect.poll(() => shadowOf(el), `${what} wears its ring`).toMatch(ring);
}

// contrastIn computes the WCAG contrast ratio of two computed colours.
function contrastIn(fg: string, bg: string): number {
  const parse = (c: string) => {
    const m = c.match(/rgba?\(([^)]+)\)/);
    expect(m, `not an rgb colour: ${c}`).not.toBeNull();
    return m![1].split(",").slice(0, 3).map((v) => Number(v.trim()) / 255);
  };
  const lum = (rgb: number[]) => {
    const [r, g, b] = rgb.map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4));
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const a = lum(parse(fg));
  const b = lum(parse(bg));
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

// overlapsOf lists the headings, texts and controls whose VISIBLE boxes
// intersect the pill's (SI-358 (4)): the pill must cover none of them. An
// element's visible box is its own, clipped by every ancestor that clips
// its overflow (the canvas scrolls: a paper past its foot is not on
// screen), so what is counted is what a reader could see.
async function overlapsOf(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const pillEl = document.querySelector('[data-testid="wall-status"]');
    if (!pillEl) return ["no pill"];
    const r = pillEl.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return ["pill has no box"];
    const visibleBox = (el: Element) => {
      let b = { left: 0, top: 0, right: 0, bottom: 0 };
      const own = el.getBoundingClientRect();
      b = { left: own.left, top: own.top, right: own.right, bottom: own.bottom };
      for (let p = el.parentElement; p; p = p.parentElement) {
        const o = getComputedStyle(p);
        if (o.overflow === "visible" && o.overflowX === "visible" && o.overflowY === "visible") continue;
        const c = p.getBoundingClientRect();
        b = {
          left: Math.max(b.left, c.left),
          top: Math.max(b.top, c.top),
          right: Math.min(b.right, c.right),
          bottom: Math.min(b.bottom, c.bottom),
        };
      }
      return b;
    };
    const sel =
      "h1, h2, h3, h4, p, span, a, button, input, textarea, select, summary, li, td, th, label, " +
      ".objcard, .stubcard, .refcard, .sticky, .yarn-chip, .zone-label, .board-notice, .placard";
    const hits: string[] = [];
    for (const el of Array.from(document.querySelectorAll(sel))) {
      if (pillEl.contains(el) || el.contains(pillEl)) continue;
      const cs = getComputedStyle(el);
      if (cs.display === "none" || cs.visibility === "hidden") continue;
      // An element drawn at opacity 0 by itself or an ancestor (the idle
      // trash target) is not on screen either.
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
        hits.push(el.tagName.toLowerCase() + (el.className && typeof el.className === "string" ? "." + el.className.split(" ").join(".") : ""));
      }
    }
    return hits;
  });
}

async function openWall(page: Page): Promise<void> {
  await page.goto(boardPath(WALL.SPEC));
  await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
  // The serving branch is not the design branch: typed writes are refused,
  // so the wall offers no pins (SI-350 (14) as amended).
  await expect(page.getByTestId("asd-domain-refusal")).toBeVisible();
  await expect(page.locator("#board-canvas .yarn-handle")).toHaveCount(0);
  await expect(baseThreads(page)).toHaveCount(2);
}

// assertCardsReceiptsAndLayers is ac-1's one shared assertion routine
// (SI-355 (1)): footprints and rotation, the receipts' exact texts, the
// stub's slug as its first line, and the two yarn layers.
async function assertCardsReceiptsAndLayers(page: Page): Promise<void> {
  await openWall(page);

  // Every card kind at the handoff's footprint, unrotated: object cards
  // 200×140, the stub 200×140 (SI-350 (3)), the reference 200×72, the
  // sticky 200 wide and at least 112 tall.
  for (const id of CARD_IDS) {
    await expectFootprint(page.getByTestId(`card-${id}`), `card ${id}`, 200, 140);
  }
  const stub = page.getByTestId(stubCardTestId(WALL.STUB_SLUG));
  await expectFootprint(stub, "the stub card", 200, 140);
  const ref = page.getByTestId(refCardTestId(WALL.ADR_REF));
  await expectFootprint(ref, "the reference card", 200, 72);
  const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
  await expectFootprint(sticky, "the sticky", 200, null);
  await expect(sticky.locator(".sticky-body")).toHaveText(WALL.STICKY_BODY);
  await expect(sticky.locator(".sticky-foot")).toHaveText("scratch · delete");
  // No `?` watermark on the open question's card.
  const watermark = await page
    .getByTestId("card-oq-1")
    .evaluate((node) => getComputedStyle(node, "::after").content);
  expect(watermark, "the open question's ? watermark").toMatch(/^(none|normal)$/);

  // Every receipt, with its existing text exactly (dc-1): the coverage
  // chips, the obligation rows, the evidence slots, the attestation chips.
  await expect(page.getByTestId(coverageChipTestId("ac-1"))).toHaveText("covered by 1 stub");
  await expect(page.getByTestId(coverageChipTestId("ac-2"))).toHaveText("no stub");
  const rows = page.getByTestId("obligations-ac-1").locator(".obligation");
  await expect(rows).toHaveCount(3);
  await expect(rows.locator(".obligation-kind")).toHaveText(["behavioral", "static", "attestation"]);
  await expect(rows.nth(0).locator(".obligation-title")).toHaveText(WALL.OBLIGATION_TITLE);
  await expect(page.getByTestId("obligation-none-ac-1-static")).toHaveText("no obligation");
  await expect(page.getByTestId("obligation-none-ac-1-attestation")).toHaveText("no obligation");
  await expect(page.getByTestId("obligation-none-ac-2-attestation")).toHaveText("no obligation");
  await expect(page.getByTestId(slotChipTestId("ac-1", "behavioral"))).toHaveText("no record");
  await expect(page.getByTestId(slotChipTestId("ac-1", "static"))).toHaveText("1 record");
  await expect(page.getByTestId(slotChipTestId("ac-1", "attestation"))).toHaveText("attested");
  await expect(page.getByTestId(slotChipTestId("ac-2", "attestation"))).toHaveText("no attestation");

  // The stub's slug is its first line, inside the card: the first child,
  // the one hook the scoping specs share, laid out within the card's box
  // and above the kind line; the pin after it anchors the coverage yarn
  // and is no handle (dc-3).
  const tab = stub.locator(".stub-tab");
  await expect(tab).toHaveText(WALL.STUB_SLUG);
  expect(await stub.evaluate((node) => node.firstElementChild?.className)).toBe("stub-tab");
  const stubBox = (await stub.boundingBox())!;
  const tabBox = (await tab.boundingBox())!;
  const kindBox = (await stub.locator(".card-kind").boundingBox())!;
  expect(tabBox.y, "the slug starts inside the card").toBeGreaterThanOrEqual(stubBox.y);
  expect(tabBox.y + tabBox.height, "the slug is the first line").toBeLessThanOrEqual(kindBox.y + 0.5);
  await expect(stub.locator(".stub-meta")).toHaveText("claims ac-1");
  await expect(stub.locator(".stub-pushpin")).toHaveCount(1);
  await expect(stub.locator("button, .yarn-handle")).toHaveCount(0);
  // The pin's centre, in the card's frame (its layout box against the
  // card's), then in the canvas's.
  const pinBox = (await stub.locator(".stub-pushpin").boundingBox())!;
  const stubRect = await offsetRect(stub);
  const pinCentre = {
    x: stubRect.x + pinBox.x + pinBox.width / 2 - stubBox.x,
    y: stubRect.y + pinBox.y + pinBox.height / 2 - stubBox.y,
  };
  // The coverage thread ties to the pin's centre, on the card's top edge.
  const covers = page.locator("#board-canvas svg.yarn-svg path.yarn-thread--type-covers");
  await expect(covers).toHaveCount(1);
  const coversD = quad((await covers.getAttribute("d"))!);
  expect(coversD.ax).toBeCloseTo(stubRect.x + stubRect.w / 2, 0);
  expect(coversD.ay).toBeCloseTo(stubRect.y, 0);
  expect(pinCentre.x).toBeCloseTo(coversD.ax, 0);
  expect(pinCentre.y).toBeCloseTo(coversD.ay, 0);
  // Every other thread joins the facing sides' midpoints (SI-350 (8)):
  // dc-1 → the ADR, left to right.
  const exempts = page.locator("#board-canvas svg.yarn-svg path.yarn-thread--type-exempts");
  const exemptsD = quad((await exempts.getAttribute("d"))!);
  const dcRect = await offsetRect(page.getByTestId("card-dc-1"));
  const refRect = await offsetRect(ref);
  expect(exemptsD.ax).toBeCloseTo(dcRect.x + dcRect.w, 0);
  expect(exemptsD.ay).toBeCloseTo(dcRect.y + dcRect.h / 2, 0);
  expect(exemptsD.bx).toBeCloseTo(refRect.x, 0);
  expect(exemptsD.by).toBeCloseTo(refRect.y + refRect.h / 2, 0);

  // Two layers: the base layer under the cards carries every thread; the
  // overlay above the cards carries only the selection's, traced over the
  // same curve.
  const base = page.locator("#board-canvas svg.yarn-svg");
  await expect(base).toHaveCount(1);
  const zIndexOf = (el: Locator) => el.evaluate((node) => Number(getComputedStyle(node).zIndex));
  const cardZ = await zIndexOf(page.getByTestId("card-co-1"));
  expect(await zIndexOf(base)).toBeLessThan(cardZ);
  // With nothing selected there is no overlay at all: the region's markup
  // stays the server's (spec 38 reads it back byte for byte around a
  // drawer's open and close).
  await expect(page.locator("#board-canvas svg.yarn-overlay")).toHaveCount(0);
  await expect(overlayThreads(page)).toHaveCount(0);
  await page.getByTestId("card-ac-1").click();
  await expect(page.getByTestId("card-ac-1")).toHaveAttribute("data-selected", "true");
  await expect(baseThreads(page)).toHaveCount(2);
  await expect(overlayThreads(page)).toHaveCount(1);
  const overlay = page.locator("#board-canvas svg.yarn-overlay");
  expect(await zIndexOf(overlay)).toBeGreaterThan(cardZ);
  await expect(overlayThreads(page)).toHaveClass(/yarn-thread--type-covers/);
  expect(await overlayThreads(page).getAttribute("d")).toBe(await covers.getAttribute("d"));
  await canvas(page).click({ position: { x: 300, y: 520 } });
  await expect(page.locator("#board-canvas [data-selected]")).toHaveCount(0);
  await expect(page.locator("#board-canvas svg.yarn-overlay")).toHaveCount(0);
}

test.describe("wall-canvas", () => {
  // Held (SI-355): the claim includes the readiness mark, which waits on
  // the owner's decision (SI-352). Skipped counts as falsified, so ac-1
  // stays unproven, never a pass; the reason rides the run's report as an
  // annotation (SI-358 (7)).
  test.fixme(
    "Cards at the new footprint with their receipts, and yarn in two layers",
    {
      annotation: {
        type: "fixme",
        description:
          "held: the claim's readiness mark waits on the owner's decision (SI-352); the same routine runs unheld under the SI-355 title below",
      },
    },
    async ({ page }) => {
      await assertCardsReceiptsAndLayers(page);
    },
  );

  test("cards, receipts, and yarn layers before the readiness mark (SI-355)", async ({ page }) => {
    await assertCardsReceiptsAndLayers(page);
  });

  test("Selecting a card emphasizes its threads and names them", async ({ page }) => {
    await openWall(page);
    const dc1 = page.getByTestId("card-dc-1");
    const co1 = page.getByTestId("card-co-1");
    const ref = page.getByTestId(refCardTestId(WALL.ADR_REF));
    const stub = page.getByTestId(stubCardTestId(WALL.STUB_SLUG));
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
    const exemptsChip = page.locator('.yarn-chip[data-edge-type="exempts"]');
    const coversChip = page.locator('.yarn-chip[data-edge-type="covers"]');
    const base = page.locator("#board-canvas svg.yarn-svg");

    // Clicking a card selects it: the card and every card threaded to it
    // are emphasized — the selected card's 3 px ink ring, the linked
    // card's 2 px ring in the thread's colour — and everything else
    // recedes: every other kind of paper, the other chips, and the base
    // yarn layer.
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await expect(ref).toHaveAttribute("data-linked", "exempts");
    await expect(canvas(page)).toHaveAttribute("data-selection", "card");
    await expect(exemptsChip).toHaveAttribute("data-hot", "true");
    await expect(coversChip).not.toHaveAttribute("data-hot", /./);
    await expectEmphasized(dc1, "the selected card", RING_3);
    await expectEmphasized(ref, "the linked reference card", RING_2);
    await expectEmphasized(exemptsChip, "the selection's chip", /./);
    for (const [el, what] of [
      [co1, "co-1"],
      [page.getByTestId("card-ac-1"), "ac-1"],
      [page.getByTestId("card-ac-2"), "ac-2"],
      [page.getByTestId("card-oq-1"), "oq-1"],
      [stub, "the stub card"],
      [sticky, "the sticky"],
      [coversChip, "the other thread's chip"],
    ] as [Locator, string][]) {
      await expectReceded(el, what);
    }
    await expect.poll(() => opacityOf(base), "the base yarn layer recedes").toBeCloseTo(0.25, 2);
    // The emphasis holds while the pointer is on a card (SI-358 (5)): the
    // rings are drawn under the hover lift, not replaced by it.
    await dc1.hover();
    await expect.poll(() => shadowOf(dc1), "the selected ring under the pointer").toMatch(RING_3);
    await ref.hover();
    await expect.poll(() => shadowOf(ref), "the linked ring under the pointer").toMatch(RING_2);
    await page.mouse.move(300, 520);
    await expect.poll(() => shadowOf(dc1), "the selected ring with the pointer off").toMatch(RING_3);
    await expect.poll(() => shadowOf(ref), "the linked ring with the pointer off").toMatch(RING_2);
    // The status pill names the card and its threads; its words reach the
    // live region, a polite status region outside the swapped region.
    await expect(pill(page).locator(".wall-status-id")).toHaveText("dc-1");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText(`exempts → ${WALL.ADR_REF}`);
    await expect(live(page)).toHaveAttribute("role", "status");
    await expect(live(page)).toHaveText(`dc-1 exempts → ${WALL.ADR_REF}`);

    // A card with no threads says so — and this wall offers no pin to
    // drag, so the pill does not mention one (SI-350 (14) as amended).
    await co1.click();
    await expect(co1).toHaveAttribute("data-selected", "true");
    await expect(dc1).not.toHaveAttribute("data-selected", /./);
    await expect(page.locator("#board-canvas [data-linked]")).toHaveCount(0);
    await expect(pill(page).locator(".wall-status-id")).toHaveText("co-1");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText("no threads yet");
    await expect(live(page)).toHaveText("co-1 no threads yet");

    // Clicking the wall clears the selection: the pill leaves the canvas,
    // the live region empties, and nothing recedes.
    await canvas(page).click({ position: { x: 300, y: 520 } });
    await expect(page.locator("#board-canvas [data-selected]")).toHaveCount(0);
    await expect(canvas(page)).not.toHaveAttribute("data-selection", /./);
    await expect(pill(page)).toHaveCount(0);
    await expect(live(page)).toBeEmpty();
    await expect.poll(() => filterOf(co1)).toBe("none");
    await expect.poll(() => shadowOf(co1)).not.toBe("none");

    // Clicking the same card clears it too (a beat apart: two quick clicks
    // are a double click, which edits and keeps the card selected).
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await page.waitForTimeout(600);
    await dc1.click();
    await expect(dc1).not.toHaveAttribute("data-selected", /./);
    await expect(pill(page)).toHaveCount(0);

    // Clicking a thread's chip selects the thread: both ends light up, the
    // pill names the type and the pair, the overlay carries that one thread.
    await exemptsChip.click();
    await expect(exemptsChip).toHaveAttribute("data-selected", "true");
    await expect(canvas(page)).toHaveAttribute("data-selection", "thread");
    await expect(dc1).toHaveAttribute("data-linked", "exempts");
    await expect(ref).toHaveAttribute("data-linked", "exempts");
    await expect(page.locator("#board-canvas .objcard[data-selected], #board-canvas .refcard[data-selected]")).toHaveCount(0);
    await expect(pill(page).locator(".wall-status-id")).toHaveText("exempts");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText(`dc-1 → ${WALL.ADR_REF}`);
    await expect(overlayThreads(page)).toHaveCount(1);
    await expect(overlayThreads(page)).toHaveClass(/yarn-thread--type-exempts/);
    // Selection writes nothing.
    await expect(page.getByTestId("autosave-status")).toHaveText("");
  });

  test("arrival on #obj-<id> selects the card, and the selection survives a region swap", async ({ page }) => {
    // SI-340 (11), SI-350's carry-in: the Document page's chips link here.
    await page.goto(boardPath(WALL.SPEC) + "#obj-oq-1");
    await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
    const oq1 = page.getByTestId("card-oq-1");
    await expect(oq1).toHaveAttribute("data-selected", "true");
    await expect(pill(page).locator(".wall-status-id")).toHaveText("oq-1");
    // hashchange re-targets the selection.
    await page.evaluate(() => {
      window.location.hash = "#obj-dc-1";
    });
    const dc1 = page.getByTestId("card-dc-1");
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await expect(oq1).not.toHaveAttribute("data-selected", /./);
    await expect(overlayThreads(page)).toHaveCount(1);

    // co-2: a region swap replaces the canvas; the selection, its marks
    // and the overlay are re-applied, and the pill's text is unchanged. The
    // swap is forced by a scratch write this wall accepts (a sticky move).
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
    const before = await sticky.evaluate((el) => (el as HTMLElement).style.top);
    const moved = await page.request.post(boardPath(WALL.SPEC) + "/api/sticky-position", {
      data: { id: WALL.STICKY_ID, x: 1408, y: 300 },
    });
    expect(moved.status(), await moved.text()).toBe(200);
    await expect(sticky).not.toHaveCSS("top", before, { timeout: 8_000 });
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await expect(page.getByTestId(refCardTestId(WALL.ADR_REF))).toHaveAttribute("data-linked", "exempts");
    await expect(overlayThreads(page)).toHaveCount(1);
    await expect(pill(page).locator(".wall-status-id")).toHaveText("dc-1");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText(`exempts → ${WALL.ADR_REF}`);
    await expect(live(page)).toHaveText(`dc-1 exempts → ${WALL.ADR_REF}`);
  });

  test("a region swap keeps the canvas's scroll on both axes (Wave 6 §5.1; co-2)", async ({ page }) => {
    // The canvas is bounded to the viewport, so on a short window it
    // scrolls vertically as well as horizontally. The sticky is parked far
    // down and right first, so both axes have room whatever the file's
    // earlier tests left.
    await page.setViewportSize({ width: 1440, height: 600 });
    await openWall(page);
    await parkSticky(page, 1408, 900);
    const room = await canvas(page).evaluate((el) => ({ x: el.scrollWidth - el.clientWidth, y: el.scrollHeight - el.clientHeight }));
    expect(room.x, "the canvas scrolls horizontally").toBeGreaterThanOrEqual(240);
    expect(room.y, "the canvas scrolls vertically").toBeGreaterThanOrEqual(160);
    await canvas(page).evaluate((el) => {
      el.scrollLeft = 200;
      el.scrollTop = 120;
    });
    await expect.poll(() => scrollOf(page)).toEqual([200, 120]);
    // A forced swap replaces the canvas. The swap restores the offsets on
    // the new canvas before the asset measures it, so the new canvas must
    // be bounded the moment it is inserted — a content-tall canvas would
    // clamp the vertical offset to 0 (the closure check's witness: (200,
    // 120) became (200, 0)).
    await parkSticky(page, 1408, 920);
    await expect.poll(() => scrollOf(page)).toEqual([200, 120]);
    // And the swap left the bound in place: the canvas still scrolls.
    expect(await canvas(page).evaluate((el) => el.scrollHeight - el.clientHeight)).toBeGreaterThanOrEqual(160);
  });

  test("the canvas is bounded by the viewport alone at 1440 px, 320 px and 200 % zoom (Wave 6 §5.2)", async ({ page }) => {
    for (const shape of [
      { width: 1440, height: 900, zoom: "" },
      { width: 320, height: 640, zoom: "" },
      { width: 720, height: 450, zoom: "200%" },
    ]) {
      const label = `${shape.width}×${shape.height}${shape.zoom ? " at " + shape.zoom : ""}`;
      await page.setViewportSize({ width: shape.width, height: shape.height });
      await page.goto(boardPath(WALL.SPEC));
      await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
      if (shape.zoom) {
        await page.evaluate((z) => {
          (document.body.style as unknown as { zoom: string }).zoom = z;
        }, shape.zoom);
        await page.evaluate(() => window.dispatchEvent(new Event("resize")));
      }
      await expect(page.locator("#boardv2-region")).toHaveAttribute("data-wall-measured", "true");
      // Lengths in the frame's own scale (a zoomed body scales its boxes).
      const m = await page.evaluate(() => {
        const c = document.getElementById("board-canvas") as HTMLElement;
        const f = c.closest(".wall-frame") as HTMLElement;
        const row = f.querySelector(".wall-status-row") as HTMLElement;
        const scale = f.getBoundingClientRect().width / f.offsetWidth;
        return {
          scale,
          frameTop: f.getBoundingClientRect().top + window.scrollY,
          height: c.getBoundingClientRect().height / scale,
          content: parseFloat(c.style.minHeight),
          view: document.documentElement.clientHeight / scale,
          row: row.getBoundingClientRect().height / scale,
          viewport: document.documentElement.clientHeight,
        };
      });
      // The bound reads the viewport alone: the frame's place on the page
      // (thousands of pixels down at 320 px, where the layout stacks) does
      // not enter it. At one card plus margin, at least; at most the
      // viewport.
      const want = Math.max(CANVAS_FLOOR, Math.min(m.content, m.view - m.row - VIEW_MARGIN));
      expect(Math.abs(m.height - want), `${label}: canvas ${m.height}, want ${want} (frame top ${m.frameTop}, view ${m.view}, row ${m.row}, content ${m.content})`).toBeLessThanOrEqual(1);
      expect(m.height, `${label}: at least one card plus margin`).toBeGreaterThanOrEqual(CANVAS_FLOOR);
      expect(m.height * m.scale, `${label}: at most the viewport`).toBeLessThanOrEqual(m.viewport);
      // A card can be brought fully into view by scrolling: the page to
      // the frame, the canvas to the card. ac-2 is the lowest object card,
      // the reference card the rightmost.
      for (const [el, what] of [
        [page.getByTestId("card-ac-2"), "ac-2"],
        [page.getByTestId(refCardTestId(WALL.ADR_REF)), "the reference card"],
      ] as [Locator, string][]) {
        await el.scrollIntoViewIfNeeded();
        const box = (await el.boundingBox())!;
        const clip = (await canvas(page).boundingBox())!;
        const vp = await page.evaluate(() => ({ w: document.documentElement.clientWidth, h: document.documentElement.clientHeight }));
        expect(box.y, `${label}: ${what} top in the viewport`).toBeGreaterThanOrEqual(-0.5);
        expect(box.y + box.height, `${label}: ${what} bottom in the viewport`).toBeLessThanOrEqual(vp.h + 0.5);
        expect(box.x, `${label}: ${what} left in the viewport`).toBeGreaterThanOrEqual(-0.5);
        expect(box.x + box.width, `${label}: ${what} right in the viewport`).toBeLessThanOrEqual(vp.w + 0.5);
        expect(box.y, `${label}: ${what} top inside the canvas`).toBeGreaterThanOrEqual(clip.y - 0.5);
        expect(box.y + box.height, `${label}: ${what} bottom inside the canvas`).toBeLessThanOrEqual(clip.y + clip.height + 0.5);
        expect(box.x, `${label}: ${what} left inside the canvas`).toBeGreaterThanOrEqual(clip.x - 0.5);
        expect(box.x + box.width, `${label}: ${what} right inside the canvas`).toBeLessThanOrEqual(clip.x + clip.width + 0.5);
      }
    }
  });

  test("the status pill covers no content or control at 1440 px, 320 px and 200 % zoom (SI-358 (4))", async ({ page }) => {
    // A tall comment sticky at the canvas's bottom-left: taller than the
    // server's working estimate, so the canvas's content outgrows its
    // min-height — the persistent case the review found under the old,
    // in-canvas pill.
    await page.goto(boardPath(WALL.SPEC));
    await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
    const tallText = Array.from({ length: 26 }, (_, i) => `line ${i + 1} of a tall note that keeps growing`).join(" — ");
    const made = await page.request.post(boardPath(WALL.SPEC) + "/api/sticky", { data: { text: tallText, type: "comment" } });
    expect(made.status(), await made.text()).toBe(200);
    // The write is the server's; a fresh page renders it without waiting
    // on the poll.
    await page.reload();
    const tall = page.locator('[data-testid^="sticky-"]').filter({ hasText: "line 26 of a tall note" });
    await expect(tall).toHaveCount(1);
    const tallID = (await tall.getAttribute("data-id"))!;
    const parked = await page.request.post(boardPath(WALL.SPEC) + "/api/sticky-position", { data: { id: tallID, x: 40, y: 392 } });
    expect(parked.status(), await parked.text()).toBe(200);

    const select = (key: string) =>
      page.evaluate((k) => {
        (window as unknown as { __WALLSELECT__: { select: (s: unknown) => void } }).__WALLSELECT__.select({ kind: "card", key: k });
      }, key);
    const inViewport = async (label: string) => {
      const vp = await page.evaluate(() => ({ w: document.documentElement.clientWidth, h: document.documentElement.clientHeight }));
      const box = (await pill(page).boundingBox())!;
      expect(box, `${label}: the pill has a box`).not.toBeNull();
      expect(box.width, `${label}: the pill is drawn`).toBeGreaterThan(40);
      expect(box.y, `${label}: the pill's top is in the viewport`).toBeGreaterThanOrEqual(0);
      expect(box.y + box.height, `${label}: the pill's bottom is in the viewport (${box.y + box.height} of ${vp.h})`).toBeLessThanOrEqual(vp.h + 0.5);
    };

    for (const shape of [
      { width: 1440, height: 900, zoom: "" },
      { width: 320, height: 800, zoom: "" },
      { width: 720, height: 450, zoom: "200%" },
    ]) {
      const label = `${shape.width}×${shape.height}${shape.zoom ? " at " + shape.zoom : ""}`;
      await page.setViewportSize({ width: shape.width, height: shape.height });
      await page.goto(boardPath(WALL.SPEC));
      await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
      if (shape.zoom) {
        await page.evaluate((z) => {
          (document.body.style as unknown as { zoom: string }).zoom = z;
        }, shape.zoom);
        await page.evaluate(() => window.dispatchEvent(new Event("resize")));
      }
      const tallSticky = page.getByTestId(`sticky-${tallID}`);
      await expect(tallSticky).toHaveCSS("top", "392px");
      // The pill lives in the frame's reserved row, outside the canvas's
      // scroll area, so the tall sticky cannot meet it.
      await select("dc-1");
      await expect(page.getByTestId("card-dc-1")).toHaveAttribute("data-selected", "true");
      await expect(pill(page)).toBeAttached();
      expect(await pill(page).evaluate((el) => el.parentElement?.getAttribute("data-testid"))).toBe("wall-status-row");
      expect(await overlapsOf(page), `${label}: the pill covers nothing beside a tall sticky`).toEqual([]);
      // With a card near the top selected and the frame scrolled into
      // view, the pill is in the viewport: the canvas is bounded to the
      // room the viewport has, so the frame fills it and its row is in
      // view — or, where the viewport is too short for even the floor
      // canvas and the row, the row leads the canvas and shows with its
      // top.
      const ac1 = page.getByTestId("card-ac-1");
      await ac1.scrollIntoViewIfNeeded();
      await select("ac-1");
      await expect(ac1).toHaveAttribute("data-selected", "true");
      await frameIntoView(page);
      await inViewport(`${label}, ac-1 selected`);
      expect(await overlapsOf(page), `${label}: the pill covers nothing with ac-1 selected`).toEqual([]);
      // A paper mid-drag over the canvas's foot, where the pill used to
      // sit, cannot meet it either: the pill is not in the scroll area.
      const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
      await sticky.scrollIntoViewIfNeeded();
      await frameIntoView(page);
      const grip = (await sticky.locator(".sticky-body").boundingBox())!;
      const foot = (await canvas(page).boundingBox())!;
      const viewH = await page.evaluate(() => document.documentElement.clientHeight);
      await page.mouse.move(grip.x + grip.width / 2, grip.y + 10);
      await page.mouse.down();
      // The canvas's visible foot: on the short zoomed viewport the canvas
      // itself runs past the viewport's edge.
      await page.mouse.move(foot.x + 80, Math.min(foot.y + foot.height - 20, viewH - 10), { steps: 8 });
      await expect(sticky).toHaveClass(/dragging/);
      expect(await overlapsOf(page), `${label}: the pill covers nothing under a paper mid-drag`).toEqual([]);
      await inViewport(`${label}, mid-drag`);
      await page.mouse.up();
      await expect(page.getByTestId("autosave-status")).toHaveText("saved", { timeout: 8_000 });
    }
  });

  test("the pill's summary is readable at 320 px in authoring (ac-2; SI-358 (4))", async ({ page }) => {
    // At 320 px the row is 256 px wide; beside the band the authoring
    // wall's fixed pin-toolbox tab holds, the pill had 64 px and its
    // summary 1 px. The pill takes a line of its own above the band
    // instead, and the summary wraps rather than clips.
    await page.setViewportSize({ width: 320, height: 640 });
    await openWall(page);
    const dc1 = page.getByTestId("card-dc-1");
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    const summary = pill(page).locator(".wall-status-summary");
    await expect(summary).toHaveText(`exempts → ${WALL.ADR_REF}`);
    // Fully visible: the summary's rendered width is not less than its
    // content's, and the pill clips none of it.
    const fit = await pill(page).evaluate((el) => {
      const s = el.querySelector(".wall-status-summary") as HTMLElement;
      return { sw: s.scrollWidth, cw: s.clientWidth, pw: el.scrollWidth, pcw: el.clientWidth, ph: el.scrollHeight, pch: el.clientHeight };
    });
    expect(fit.cw, `the summary's rendered width (${fit.cw}) holds its content (${fit.sw})`).toBeGreaterThanOrEqual(fit.sw);
    expect(fit.pcw, "the pill clips nothing across").toBeGreaterThanOrEqual(fit.pw);
    expect(fit.pch, "the pill clips nothing down").toBeGreaterThanOrEqual(fit.ph);
    expect(fit.cw, "the summary has room to read").toBeGreaterThanOrEqual(120);
    // And the pill still covers nothing — the toolbox tab included — with
    // the frame filling the viewport, where the tab's band meets the row.
    await frameIntoView(page);
    const tabEl = page.locator("#pin-toolbox-tab");
    await expect(tabEl).toBeVisible();
    expect(await overlapsOf(page), "the pill covers nothing at 320 px").toEqual([]);
    const tab = (await tabEl.boundingBox())!;
    const box = (await pill(page).boundingBox())!;
    expect(box.y + box.height, `the pill (bottom ${box.y + box.height}) sits above the toolbox tab (top ${tab.y})`).toBeLessThanOrEqual(tab.y);
  });

  test("a slow double click never re-announces the same selection (SI-358 (5); Wave 6 §5.2)", async ({ page }) => {
    await openWall(page);
    const dc1 = page.getByTestId("card-dc-1");
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    const words = `dc-1 exempts → ${WALL.ADR_REF}`;
    await expect(live(page)).toHaveText(words);
    // Record every rewrite of the live region from here on.
    await page.evaluate(() => {
      const el = document.getElementById("wall-status-live")!;
      const w = window as unknown as { __liveLog: string[] };
      w.__liveLog = [];
      new MutationObserver(() => w.__liveLog.push(el.textContent || "")).observe(el, { childList: true, characterData: true, subtree: true });
    });
    await page.waitForTimeout(700);
    // Two clicks on the selected card, further apart than the repeated-
    // click window but inside the OS double-click interval: the first
    // clears (after its window), the second re-selects. The pill may blink;
    // the live region keeps its words, unspoken a second time.
    await dc1.click();
    await page.waitForTimeout(400);
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await page.waitForTimeout(900);
    await expect(live(page)).toHaveText(words);
    expect(await page.evaluate(() => (window as unknown as { __liveLog: string[] }).__liveLog)).toEqual([]);
    // A real clear, left alone, is spoken once the hold passes.
    await canvas(page).click({ position: { x: 300, y: 520 } });
    await expect(live(page)).toBeEmpty();
  });

  test("a selection keeps every text legible and every focus ring visible, in light and dark (SI-358 (2), (3))", async ({ page }) => {
    for (const scheme of ["light", "dark"] as const) {
      await page.emulateMedia({ colorScheme: scheme });
      await openWall(page);
      const dc1 = page.getByTestId("card-dc-1");
      const oq1 = page.getByTestId("card-oq-1");
      await dc1.click();
      await expect(dc1).toHaveAttribute("data-selected", "true");
      await expectReceded(oq1, `${scheme}: oq-1`);

      // The automated scan covers text contrast on the receded papers and
      // chips: a recede that faded them would fail it.
      const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
      const violations = results.violations.map((v) => ({
        id: v.id,
        impact: v.impact,
        nodes: v.nodes.length,
        targets: v.nodes.slice(0, 3).map((n) => n.target.join(" ")),
      }));
      expect(violations, `${scheme}: ${JSON.stringify(violations, null, 2)}`).toEqual([]);

      // Tab from the selected card lands on the next card, receded: focus
      // lifts the recede and the ring reads at 3:1 or better against the
      // wall (WCAG 1.4.11; Wave 6 §5.2 "visible focus").
      await page.keyboard.press("Tab");
      await expect(oq1).toBeFocused();
      await expect.poll(() => filterOf(oq1), `${scheme}: a focused card is not receded`).toBe("none");
      const ring = await oq1.evaluate((node) => {
        const c = getComputedStyle(node);
        return { style: c.outlineStyle, width: parseFloat(c.outlineWidth), color: c.outlineColor };
      });
      expect(ring.style, `${scheme}: focus ring style`).not.toBe("none");
      expect(ring.width, `${scheme}: focus ring width`).toBeGreaterThanOrEqual(2);
      const wall = await canvas(page).evaluate((node) => getComputedStyle(node).backgroundColor);
      expect(contrastIn(ring.color, wall), `${scheme}: focus ring ${ring.color} on the wall ${wall}`).toBeGreaterThanOrEqual(3);
      // The selection itself is untouched by moving focus.
      await expect(dc1).toHaveAttribute("data-selected", "true");
    }
  });
});
