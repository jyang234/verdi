import { test, expect, type Page, type Locator } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { EDGE, SHOWCASE, boardPath, coverageChipTestId, refCardTestId, slotChipTestId, stubCardTestId } from "./fixtures";
import { expectAutosaved, toolbarAction, transformRotates, wallToolbar } from "./helpers";

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
// obligations' `wall-canvas ›` prefix (SI-350 (12)). ac-1's producer was
// held (test.fixme, SI-355) while the readiness mark waited on the owner's
// decision (SI-352); lane M-ui drew the mark from the composed refresh's
// facts (SI-360, SI-362) and lifted the hold in one change (SI-355 (3)):
// the test asserts the mark, and the unheld copy that ran meanwhile is
// gone. The marks are read on the serving path, whose readiness shares
// the wall's branch and head; the writable path shows the one
// unavailable notice instead (SI-350 (2); SI-360 (3)).
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
// Lane F2b (ac-3, ac-4, ac-5): the contextual toolbar, drag-to-thread and
// the add-in-place slots write, so their tests run on the wall's writable
// path (WALL.WRITABLE_PATH, its own design branch, where the domain is
// live and every pin is drawn; cmd/e2eharness/provision_board.go
// canvasWallWritablePath). Review and read-only modes are read on the
// harness's badge review rig and sealed wall (EDGE.BADGE_REVIEW_SPEC,
// SHOWCASE.READONLY_SPEC), both of which carry yarn, so each renders the
// yarn key the modes' toolbar must offer. Writes stay inside this file, ordered so a
// later test here still finds what it needs; every run provisions a fresh
// store.
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
  // The writable address: the wall under its own design branch (SI-350 (11)).
  WRITABLE_PATH: "/b/design%2Fdecline-canvas-wall/board/spec/decline-canvas-wall",
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

// receiptOverlaps lists the pairs among a paper's prose, obligation rows,
// coverage or claims chip, badge row, mark and (on a stub) meta line whose
// boxes intersect by more than half a pixel on both axes (SI-365 (1), (5):
// the register never overlaps anything). Empty is the only passing value.
async function receiptOverlaps(card: Locator): Promise<string[]> {
  return card.evaluate((node) => {
    const parts: [string, Element][] = [];
    const add = (name: string, el: Element | null) => { if (el) parts.push([name, el]); };
    add("prose", node.querySelector(".card-text, .stub-title"));
    add("meta", node.querySelector(".stub-meta"));
    node.querySelectorAll(".card-obligations .obligation").forEach((o, i) => add(`obligation ${i + 1}`, o));
    add("chip", node.querySelector(".coverage-chip, .oq-claims"));
    add("badges", node.querySelector(".card-badges"));
    add("mark", node.querySelector(".readiness-mark"));
    const out: string[] = [];
    for (let i = 0; i < parts.length; i++) {
      for (let j = i + 1; j < parts.length; j++) {
        const a = parts[i][1].getBoundingClientRect();
        const b = parts[j][1].getBoundingClientRect();
        const w = Math.min(a.right, b.right) - Math.max(a.left, b.left);
        const h = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
        if (w > 0.5 && h > 0.5) out.push(`${parts[i][0]} × ${parts[j][0]} (${Math.round(w * h)} px²)`);
      }
    }
    return out;
  });
}

// expectMark asserts one paper's readiness mark (dc-1; SI-350 (1); the
// handoff's "readiness marks"): the dot — 10 px in its 2 px ring, round,
// straddling the card's right edge in its top band, hidden from the
// accessibility tree — and the one chip whose word is the meaning, its
// title naming the Focus next concerns that name the card (each given
// concern among them), drawn inside the card on its head line (SI-365
// (5)(c)): above the prose, over no receipt, and topmost at its centre so
// its title is a hover away. owner is the paper's testid stem (an object
// id, or stub-<slug>).
async function expectMark(page: Page, card: Locator, owner: string, word: string, concerns: string[]): Promise<void> {
  const dot = page.getByTestId(`readiness-dot-${owner}`);
  const mark = page.getByTestId(`readiness-mark-${owner}`);
  const chips = mark.locator(".readiness-chip");
  await expect(dot).toHaveAttribute("aria-hidden", "true");
  await expect(chips).toHaveText([word]);
  await expect(chips).toHaveAttribute("data-mark", word);
  await expect(chips).toHaveAttribute("title", /^Focus next: /);
  const title = (await chips.getAttribute("title"))!;
  for (const concern of concerns) {
    expect(title.slice("Focus next: ".length).split(", "), `${owner}: the chip's title names ${concern}`).toContain(concern);
  }
  await expect(card.locator(`[data-testid="readiness-mark-${owner}"]`), `${owner}: the mark is the card's own`).toHaveCount(1);
  await expect(card.locator(`[data-testid="readiness-dot-${owner}"]`), `${owner}: the dot is the card's own`).toHaveCount(1);
  await card.scrollIntoViewIfNeeded();
  const cardBox = (await card.boundingBox())!;
  const dotBox = (await dot.boundingBox())!;
  const markBox = (await mark.boundingBox())!;
  expect(Math.abs(dotBox.width - 14), `${owner}: dot width ${dotBox.width}`).toBeLessThanOrEqual(0.5);
  expect(Math.abs(dotBox.height - 14), `${owner}: dot height ${dotBox.height}`).toBeLessThanOrEqual(0.5);
  expect(await dot.evaluate((el) => getComputedStyle(el).borderRadius), `${owner}: the dot is round`).toBe("50%");
  expect(Math.abs(dotBox.x + dotBox.width / 2 - (cardBox.x + cardBox.width)), `${owner}: the dot straddles the card's right edge`).toBeLessThanOrEqual(4);
  expect(dotBox.y, `${owner}: the dot sits in the card's top band`).toBeGreaterThanOrEqual(cardBox.y);
  expect(dotBox.y + dotBox.height, `${owner}: the dot sits in the card's top band`).toBeLessThanOrEqual(cardBox.y + 30);
  expect(markBox.x, `${owner}: the chip is inside the card`).toBeGreaterThanOrEqual(cardBox.x);
  expect(markBox.x + markBox.width, `${owner}: the chip is inside the card`).toBeLessThanOrEqual(cardBox.x + cardBox.width + 0.5);
  expect(markBox.y + markBox.height, `${owner}: the chip is inside the card`).toBeLessThanOrEqual(cardBox.y + cardBox.height + 0.5);
  expect(markBox.y, `${owner}: the chip is inside the card`).toBeGreaterThanOrEqual(cardBox.y - 0.5);
  // The head line (SI-365 (5)(c), replacing the foot-row truth): the chip
  // ends above the prose — the card text, or a stub's title — and no
  // receipt or prose box meets it.
  const proseBox = (await card.locator(".card-text, .stub-title").first().boundingBox())!;
  expect(markBox.y + markBox.height, `${owner}: the chip sits on the head line, above the prose`).toBeLessThanOrEqual(proseBox.y + 0.5);
  expect(await receiptOverlaps(card), `${owner}: nothing overlaps`).toEqual([]);
  // Topmost at its centre: the chip itself answers elementFromPoint, so
  // nothing paints over its word and its title tooltip is reachable.
  const chipBox = (await chips.boundingBox())!;
  const topmost = await page.evaluate(
    ([x, y]) => { const el = document.elementFromPoint(x, y); return el ? `${el.tagName.toLowerCase()}.${el.className}` : null; },
    [chipBox.x + chipBox.width / 2, chipBox.y + chipBox.height / 2],
  );
  expect(topmost, `${owner}: the chip is topmost at its centre`).toMatch(/^span\.readiness-chip$/);
}

// markChip is one paper's mark chip row.
const markChip = (page: Page, owner: string) => page.getByTestId(`readiness-mark-${owner}`).locator(".readiness-chip");

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

// openWritableWall opens the wall on its own design branch: authoring with
// the domain live, so every object card carries its pin.
async function openWritableWall(page: Page): Promise<void> {
  await page.goto(WALL.WRITABLE_PATH);
  await expect(canvas(page)).toHaveAttribute("data-board-mode", "authoring");
  await expect(page.getByTestId("yarn-handle-ac-1")).toBeVisible();
}

// actionsOf lists the toolbar's actions in order, by the action each
// control carries (walltoolbar.js data-wall-action): the exact set ac-3
// speaks of.
const actionsOf = (page: Page) =>
  wallToolbar(page)
    .locator("[data-wall-action]")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-wall-action")));

// interactionLive reads boardspec.js's hold contract (co-2).
const interactionLive = (page: Page) =>
  page.evaluate(() => (window as unknown as { __BOARDV2API__: { interactionLive: () => boolean } }).__BOARDV2API__.interactionLive());

// dragPin drags a pin to the target's centre and releases there, in the
// bounded canvas: each endpoint is scrolled into view before the pointer
// reaches it (the pointer is captured, so the gesture survives the
// scroll), and the target lights while the pointer is over it (ac-4).
// Returns the drop point in viewport coordinates.
async function dragPin(page: Page, fromId: string, target: Locator): Promise<{ x: number; y: number }> {
  const handle = page.getByTestId(`yarn-handle-${fromId}`);
  await handle.scrollIntoViewIfNeeded();
  const hb = (await handle.boundingBox())!;
  await page.mouse.move(hb.x + hb.width / 2, hb.y + hb.height / 2);
  await page.mouse.down();
  await target.scrollIntoViewIfNeeded();
  const tb = (await target.boundingBox())!;
  const drop = { x: tb.x + tb.width / 2, y: tb.y + tb.height / 2 };
  await page.mouse.move(drop.x, drop.y, { steps: 4 });
  await expect(target).toHaveAttribute("data-drop-target", "true");
  await page.mouse.up();
  await expect(target).not.toHaveAttribute("data-drop-target", /./);
  return drop;
}

// stickyPosition reads a sticky's stored position from its inline style,
// the server's own px.
const stickyPosition = (sticky: Locator) =>
  sticky.evaluate((el) => ({ x: parseFloat((el as HTMLElement).style.left), y: parseFloat((el as HTMLElement).style.top) }));

// backgroundMove writes a sticky's position outside the page, as another
// author would, and waits for the poll to carry the change: the next
// snapshot that is not a 304. Whether the wall then shows it is the
// caller's to assert — a held projection does not (co-2).
async function backgroundMove(page: Page, id: string, x: number, y: number): Promise<void> {
  const carried = page.waitForResponse((r) => r.url().endsWith("/snapshot") && r.status() === 200);
  const moved = await page.request.post(WALL.WRITABLE_PATH + "/api/sticky-position", { data: { id, x, y } });
  expect(moved.status(), await moved.text()).toBe(200);
  await carried;
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

  // The readiness mark (dc-1; SI-350 (1)): the cards Focus next names —
  // ac-2 by success/coverage/ac-2, oq-1 by shape/question/oq-1, the stub
  // by review/blocker/stub-unreconciled/<slug> — wear the dot and the
  // chip, whose word is the meaning (Wave 6 §5.2); nothing else wears
  // one, and no notice is drawn: the serving path's readiness shares the
  // wall's branch and head.
  await expectMark(page, page.getByTestId("card-ac-2"), "ac-2", "no stub", ["success/coverage/ac-2"]);
  await expectMark(page, page.getByTestId("card-oq-1"), "oq-1", "unresolved", ["shape/question/oq-1"]);
  await expectMark(page, stub, `stub-${WALL.STUB_SLUG}`, "unresolved", [`review/blocker/stub-unreconciled/${WALL.STUB_SLUG}`]);
  await expect(page.locator("#board-canvas .readiness-mark")).toHaveCount(3);
  await expect(page.locator("#board-canvas .readiness-dot")).toHaveCount(3);
  await expect(page.getByTestId("readiness-mark-ac-1")).toHaveCount(0);
  await expect(page.getByTestId("wall-marks-unavailable")).toHaveCount(0);
  // The register (SI-365 (1), (5)): the coverage chips keep their full
  // text (dc-1; never ellipsized), and on the AC cards nothing meets
  // anything — prose, obligation rows, chip, badges, mark. This replaces
  // the foot-row truth ("the mark shares the coverage chip's row"), which
  // SI-365 (5)(c) retired for the head line.
  for (const id of ["ac-1", "ac-2"]) {
    const chip = page.getByTestId(coverageChipTestId(id));
    await expect(chip).toHaveText(id === "ac-1" ? "covered by 1 stub" : "no stub");
    expect(await chip.evaluate((el) => el.scrollWidth <= el.clientWidth), `${id}: the coverage chip shows its full text`).toBe(true);
    expect(await receiptOverlaps(page.getByTestId(`card-${id}`)), `${id}: nothing overlaps`).toEqual([]);
  }

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
  // The marks survive the selection: the server's markup, which the
  // selection's attributes leave alone.
  await expect(markChip(page, "ac-2")).toHaveText(["no stub"]);
  await expect(markChip(page, `stub-${WALL.STUB_SLUG}`)).toHaveText(["unresolved"]);
  await expect(page.locator("#board-canvas .readiness-dot")).toHaveCount(3);
  const overlay = page.locator("#board-canvas svg.yarn-overlay");
  expect(await zIndexOf(overlay)).toBeGreaterThan(cardZ);
  await expect(overlayThreads(page)).toHaveClass(/yarn-thread--type-covers/);
  expect(await overlayThreads(page).getAttribute("d")).toBe(await covers.getAttribute("d"));
  await canvas(page).click({ position: { x: 300, y: 520 } });
  await expect(page.locator("#board-canvas [data-selected]")).toHaveCount(0);
  await expect(page.locator("#board-canvas svg.yarn-overlay")).toHaveCount(0);
}

test.describe("wall-canvas", () => {
  test("Cards at the new footprint with their receipts, and yarn in two layers", async ({ page }) => {
    await assertCardsReceiptsAndLayers(page);
  });

  test("the readiness marks are unavailable on a wall whose branch is not the serving root's (SI-350 (2); SI-360 (3))", async ({ page }) => {
    // The writable path serves the wall from its own design branch, and
    // readiness derives only for the serving checkout's branch, so the
    // two cannot describe one commit: one notice in the board's
    // disclosure vocabulary says so, and no card wears a dot or a chip —
    // the coverage chips keep their own texts (dc-1).
    await openWritableWall(page);
    const notice = page.getByTestId("wall-marks-unavailable");
    await expect(notice).toHaveCount(1);
    await expect(notice).toHaveAttribute("role", "status");
    await expect(notice).toHaveClass(/(^| )board-notice( |$)/);
    await expect(notice).toHaveText(
      /^The readiness marks are unavailable: this wall serves branch design\/decline-canvas-wall from its own working tree, and readiness derives only for the serving checkout's branch, so the two cannot describe one commit\.$/,
    );
    await expect(page.locator("#board-canvas .readiness-mark, #board-canvas .readiness-dot, #board-canvas [data-mark]")).toHaveCount(0);
    // The notice sits in the board's disclosure channel, outside the
    // canvas, so it covers no card (SI-364 (2); SI-358 (4)): never a
    // child of #board-canvas, always of .board-notices.
    await expect(page.locator("#board-canvas").getByTestId("wall-marks-unavailable")).toHaveCount(0);
    await expect(page.locator(".board-notices").getByTestId("wall-marks-unavailable")).toHaveCount(1);
    await expect(page.getByTestId(coverageChipTestId("ac-2"))).toHaveText("no stub");
    await expect(page.getByTestId(coverageChipTestId("ac-1"))).toHaveText("covered by 1 stub");
  });

  test("the marks return with the composed poll after the page's own save (SI-362 (2), (7)(a))", async ({ page }) => {
    // The drag's write is sticky-position, a legacy action: its response
    // is the {dirty} receipt and carries no projection, so nothing is
    // applied from it. The refresh that follows (boardspec.js mutate →
    // refreshFragment → __verdiASD.refresh) is the composed snapshot,
    // which the moved position makes a 200 that re-applies the region
    // with the marks. A plain mutation response (SI-362 (2)) is the typed
    // mutate_draft path, which this test does not exercise. The sticky is
    // dragged by the pointer, the one write this wall accepts from the
    // page, and moved back after.
    await openWall(page);
    await expect(markChip(page, "ac-2")).toHaveText(["no stub"]);
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
    const at = await stickyPosition(sticky);
    await sticky.scrollIntoViewIfNeeded();
    const grip = (await sticky.locator(".sticky-body").boundingBox())!;
    await page.mouse.move(grip.x + grip.width / 2, grip.y + 10);
    await page.mouse.down();
    await page.mouse.move(grip.x + grip.width / 2 + 24, grip.y + 26, { steps: 6 });
    await page.mouse.up();
    await expectAutosaved(page);
    await expect(markChip(page, "ac-2")).toHaveText(["no stub"], { timeout: 8_000 });
    await expect(markChip(page, "oq-1")).toHaveText(["unresolved"]);
    await expect(markChip(page, `stub-${WALL.STUB_SLUG}`)).toHaveText(["unresolved"]);
    await expect(page.locator("#board-canvas .readiness-mark")).toHaveCount(3);
    await expect(page.getByTestId("wall-marks-unavailable")).toHaveCount(0);
    const moved = await page.request.post(boardPath(WALL.SPEC) + "/api/sticky-position", { data: { id: WALL.STICKY_ID, x: at.x, y: at.y } });
    expect(moved.status(), await moved.text()).toBe(200);
    await expect(sticky).toHaveCSS("top", `${at.y}px`, { timeout: 8_000 });
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
    // The marks are re-drawn with the composed region the poll swapped in
    // (SI-360 (2)): the server's markup, swap after swap.
    await expect(markChip(page, "ac-2")).toHaveText(["no stub"]);
    await expect(markChip(page, "oq-1")).toHaveText(["unresolved"]);
    await expect(page.getByTestId(`readiness-dot-stub-${WALL.STUB_SLUG}`)).toHaveCount(1);
    await expect(page.locator("#board-canvas .readiness-mark")).toHaveCount(3);
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
    // At 320 px the row is 256 px wide; beside the toolbar (lane F2b put
    // it in the row, where the pin toolbox's fixed tab used to hold a
    // band) the pill has too little room to read, so it takes a line of
    // its own, and the summary wraps rather than clips.
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
    // And the pill still covers nothing — the toolbar beside it in the row
    // included — with the frame filling the viewport.
    await frameIntoView(page);
    await expect(wallToolbar(page)).toBeVisible();
    expect(await overlapsOf(page), "the pill covers nothing at 320 px").toEqual([]);
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

      // The same scan with a MARKED card selected (SI-365 (5)): ac-2 wears
      // the "no stub" chip on its head line, at full strength while the
      // other papers recede.
      const ac2 = page.getByTestId("card-ac-2");
      await ac2.click();
      await expect(ac2).toHaveAttribute("data-selected", "true");
      await expect(markChip(page, "ac-2")).toHaveText(["no stub"]);
      const marked = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"]).analyze();
      expect(
        marked.violations.map((v) => ({ id: v.id, impact: v.impact, targets: v.nodes.slice(0, 3).map((n) => n.target.join(" ")) })),
        `${scheme}, ac-2 selected`,
      ).toEqual([]);
      await dc1.click();
      await expect(dc1).toHaveAttribute("data-selected", "true");

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

  // -- lane F2b: ac-3, ac-4, ac-5 ------------------------------------------------

  test("The toolbar offers exactly the legal actions for the selection and mode", async ({ page }) => {
    await openWritableWall(page);
    const toolbar = wallToolbar(page);
    const ac1 = page.getByTestId("card-ac-1");
    const stub = page.getByTestId(stubCardTestId(WALL.STUB_SLUG));
    const ref = page.getByTestId(refCardTestId(WALL.ADR_REF));
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);

    // Nothing selected: the sticky, card and pin-artifact actions through
    // their existing dialogs, and the yarn key — nothing else.
    await expect.poll(() => actionsOf(page)).toEqual(["sticky", "card", "pin", "yarn-key"]);
    for (const name of ["Sticky", "Card", "Pin an artifact", "Yarn key"]) {
      await expect(toolbar.getByRole("button", { name, exact: true })).toBeVisible();
    }
    // The sticky action is the existing inline draft (SI-350 (16)); Escape discards it.
    await toolbar.getByRole("button", { name: "Sticky", exact: true }).click();
    await expect(page.locator(".sticky-draft")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator(".sticky-draft")).toHaveCount(0);
    // The card action is the existing add-object dialog.
    await toolbar.getByRole("button", { name: "Card", exact: true }).click();
    await expect(page.locator("#asd-op-dialog")).toBeVisible();
    await page.locator("#asd-op-cancel").click();
    await expect(page.locator("#asd-op-dialog")).toBeHidden();
    // The pin-artifact action is the existing tray; the button controls it.
    const pin = toolbar.getByRole("button", { name: "Pin an artifact", exact: true });
    await expect(pin).toHaveAttribute("aria-controls", "pin-tray");
    await expect(pin).toHaveAttribute("aria-expanded", "false");
    await pin.click();
    await expect(page.getByRole("dialog", { name: "Pin an artifact" })).toBeVisible();
    await expect(pin).toHaveAttribute("aria-expanded", "true");
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog", { name: "Pin an artifact" })).toBeHidden();
    await expect(pin).toHaveAttribute("aria-expanded", "false");
    // The yarn key action opens the existing yarn key (SI-350 (9)): the
    // rail's section, brought into view and focused.
    await toolbar.getByRole("button", { name: "Yarn key", exact: true }).click();
    await expect(page.getByTestId("yarn-key")).toBeFocused();
    await expect(page.getByTestId("yarn-key")).toBeInViewport();

    // An object card: edit, the thread hint, read in document, the thread
    // count, and delete — and nothing else.
    await ac1.scrollIntoViewIfNeeded();
    await ac1.click();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["edit", "read", "delete", "yarn-key"]);
    await expect(toolbar.locator(".wall-toolbar-kind")).toHaveText("acceptance criterion");
    await expect(toolbar.locator(".wall-toolbar-id")).toHaveText("ac-1");
    await expect(toolbar.locator(".wall-toolbar-hint")).toHaveText("Thread — drag the pin");
    await expect(toolbar.locator(".wall-toolbar-count")).toHaveText("1 thread");
    await expect(toolbar.getByRole("link", { name: "Read in document ↗" })).toHaveAttribute("href", `${WALL.WRITABLE_PATH}/document#ac-1`);
    await expect(toolbar.getByRole("button", { name: "Edit", exact: true })).toBeVisible();
    await expect(toolbar.getByRole("button", { name: "Delete", exact: true })).toBeVisible();

    // A stub: edit (the existing Correct stub dialog), read in document (the
    // Plan section, where stubs are listed, SI-350 (7)) and the count — no
    // graduate, delete or retype.
    await stub.scrollIntoViewIfNeeded();
    await stub.click();
    await expect(stub).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["edit", "read", "yarn-key"]);
    await expect(toolbar.locator(".wall-toolbar-kind")).toHaveText("story stub");
    await expect(toolbar.locator(".wall-toolbar-id")).toHaveText(WALL.STUB_SLUG);
    await expect(toolbar.locator(".wall-toolbar-count")).toHaveText("1 thread");
    await expect(toolbar.getByRole("link", { name: "Read in document ↗" })).toHaveAttribute("href", `${WALL.WRITABLE_PATH}/document#plan`);
    await expect(toolbar.getByRole("button", { name: /Graduate|Delete|Retype|Remove/ })).toHaveCount(0);
    await toolbar.getByRole("button", { name: "Edit", exact: true }).click();
    await expect(page.getByRole("dialog", { name: "Correct stub" })).toBeVisible();
    await page.locator("#asd-stub-cancel").click();
    await expect(page.getByRole("dialog", { name: "Correct stub" })).toBeHidden();

    // A reference card, held by dc-1's exempts edge: delete (through the
    // existing confirmation) and the count; nothing to edit or read.
    await ref.scrollIntoViewIfNeeded();
    await ref.click();
    await expect(ref).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["delete", "yarn-key"]);
    await expect(toolbar.locator(".wall-toolbar-kind")).toHaveText("reference");
    await expect(toolbar.locator(".wall-toolbar-count")).toHaveText("1 thread");
    await toolbar.getByRole("button", { name: "Delete", exact: true }).click();
    const takeOff = page.getByRole("alertdialog", { name: `Take ${WALL.ADR_REF} off the wall` });
    await expect(takeOff).toBeVisible();
    await takeOff.getByRole("button", { name: "Cancel" }).click();
    await expect(takeOff).toBeHidden();
    await expect(ref).toBeVisible();
    await page.keyboard.press("Escape"); // the peek the click opened

    // A sticky: graduate (the existing menu) and delete; no edit, since no
    // endpoint updates a sticky's text (SI-352 (2)).
    await sticky.scrollIntoViewIfNeeded();
    await sticky.click();
    await expect(sticky).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["graduate", "delete", "yarn-key"]);
    await expect(toolbar.locator(".wall-toolbar-kind")).toHaveText("comment sticky");
    await expect(toolbar.locator(".wall-toolbar-count")).toHaveText("no threads yet");
    await toolbar.getByRole("button", { name: "Graduate", exact: true }).click();
    await expect(page.locator("#graduate-menu")).toBeVisible();
    await page.locator("#graduate-menu-cancel").click();
    await expect(page.locator("#graduate-menu")).toBeHidden();

    // A spec-layer edge: retype (the existing picker) and remove; no graduate.
    const exemptsChip = page.locator('.yarn-chip[data-edge-type="exempts"]');
    await exemptsChip.scrollIntoViewIfNeeded();
    await exemptsChip.click();
    await expect(exemptsChip).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["retype", "delete", "yarn-key"]);
    await expect(toolbar.locator(".wall-toolbar-kind")).toHaveText("exempts");
    await expect(toolbar.locator(".wall-toolbar-pair")).toHaveText(`dc-1 → ${WALL.ADR_REF}`);
    await expect(toolbar.getByRole("button", { name: "Remove exempts edge", exact: true })).toBeVisible();
    await toolbar.getByRole("button", { name: "Retype", exact: true }).click();
    const picker = page.getByRole("dialog", { name: "Edge type" });
    await expect(picker).toBeVisible();
    await expect(picker.getByRole("menuitem")).toHaveText(["supersedes"]);
    await page.keyboard.press("Escape");
    await expect(picker).toBeHidden();

    // A scoping thread (the stub's coverage yarn): nothing but the key.
    const coversChip = page.locator('.yarn-chip[data-edge-type="covers"]');
    await coversChip.scrollIntoViewIfNeeded();
    await coversChip.click();
    await expect(coversChip).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["yarn-key"]);

    // A relates thread between two cards: graduate to a typed edge and
    // delete. Written through the existing scratch path, deleted through
    // the toolbar's own path, so the wall is as it was.
    const related = await page.request.post(WALL.WRITABLE_PATH + "/api/relates", { data: { from: "co-1", to: "oq-1" } });
    expect(related.status(), await related.text()).toBe(200);
    await page.reload();
    const relatesChip = page.locator('.yarn-chip[data-edge-type="relates"][data-from="co-1"][data-to="oq-1"]');
    await expect(relatesChip).toHaveCount(1);
    await relatesChip.scrollIntoViewIfNeeded();
    await relatesChip.click();
    await expect(relatesChip).toHaveAttribute("data-selected", "true");
    await expect.poll(() => actionsOf(page)).toEqual(["graduate", "delete", "yarn-key"]);
    await expect(toolbar.getByRole("button", { name: "Graduate to a typed edge", exact: true })).toBeVisible();
    await toolbar.getByRole("button", { name: "Delete thread", exact: true }).click();
    await expectAutosaved(page);
    await expect(relatesChip).toHaveCount(0);

    // Review and read-only modes: only the yarn key and the read actions,
    // with nothing, a card, and a thread selected. The badge review rig
    // and the sealed wall both carry yarn, so each renders the yarn key
    // (writeYarnKey) and the toolbar offers exactly it with nothing
    // selected, the read actions beside it with a card selected, and it
    // alone with a thread selected.
    for (const [spec, mode] of [
      [EDGE.BADGE_REVIEW_SPEC, "review"],
      [SHOWCASE.READONLY_SPEC, "readonly"],
    ] as const) {
      await page.goto(boardPath(spec));
      await expect(canvas(page)).toHaveAttribute("data-board-mode", mode);
      const chips = page.locator("#board-canvas .yarn-chip");
      await expect(chips.first()).toBeVisible();
      await expect(page.getByTestId("yarn-key")).toHaveCount(1);
      await expect.poll(() => actionsOf(page), `${mode}: nothing selected`).toEqual(["yarn-key"]);
      await expect(toolbar.locator(".wall-toolbar-hint")).toHaveCount(0);
      const card = page.locator("#board-canvas .objcard").first();
      await card.scrollIntoViewIfNeeded();
      await card.click();
      await expect(card).toHaveAttribute("data-selected", "true");
      await expect.poll(() => actionsOf(page), `${mode}: a card selected`).toEqual(["read", "yarn-key"]);
      await expect(toolbar.locator(".wall-toolbar-count")).toBeVisible();
      await chips.first().scrollIntoViewIfNeeded();
      await chips.first().click();
      await expect(chips.first()).toHaveAttribute("data-selected", "true");
      await expect.poll(() => actionsOf(page), `${mode}: a thread selected`).toEqual(["yarn-key"]);
      await expect(toolbar.getByRole("button", { name: /Sticky|Card|Pin an artifact|Edit|Graduate|Delete|Remove|Retype/ })).toHaveCount(0);
    }
  });

  test("Drag-to-thread offers only the legal edge types", async ({ page }) => {
    await openWritableWall(page);
    const picker = page.getByRole("dialog", { name: "Edge type" });
    // The server's own tables, embedded for the picker (boardspecrender.go
    // legalPairTable; edgetypes.go), pinned at both ends: a decision
    // reaches an ADR by supersedes or exempts; no typed edge joins two
    // acceptance criteria.
    const table = await page.evaluate(() => {
      const s = (window as unknown as { __BOARDV2__: { legal: Record<string, string[]>; consequences: Record<string, string>; gate: string[] } }).__BOARDV2__;
      return { legal: s.legal, consequences: s.consequences, gate: s.gate };
    });
    expect(table.legal["decision|adr"]).toEqual(["supersedes", "exempts"]);
    expect(table.legal["acceptance-criterion|acceptance-criterion"]).toBeUndefined();
    expect(table.gate).toContain("supersedes");

    // Every source and target kind pair the fixture offers (the four
    // object kinds, and the ADR as a target): the picker lists exactly the
    // legal types with their consequence labels, plus the scratch thread,
    // at the drop point; open, it holds the projection (co-2, SI-350 (13));
    // Escape closes it with nothing written.
    const sources: Array<[string, string]> = [
      ["ac-1", "acceptance-criterion"],
      ["co-1", "constraint"],
      ["dc-1", "decision"],
      ["oq-1", "open-question"],
    ];
    const targets: Array<[string, string, Locator]> = [
      ["ac-2", "acceptance-criterion", page.getByTestId("card-ac-2")],
      ["co-1", "constraint", page.getByTestId("card-co-1")],
      ["dc-1", "decision", page.getByTestId("card-dc-1")],
      ["oq-1", "open-question", page.getByTestId("card-oq-1")],
      [WALL.ADR_REF, "adr", page.getByTestId(refCardTestId(WALL.ADR_REF))],
    ];
    const vp = page.viewportSize()!;
    let pairs = 0;
    for (const [from, fromKind] of sources) {
      for (const [to, toKind, target] of targets) {
        if (to === from) continue;
        pairs++;
        const drop = await dragPin(page, from, target);
        await expect(picker, `${from} → ${to}`).toBeVisible();
        await expect(picker.locator("#edge-picker-pair")).toHaveText(`${from} → ${to}`);
        const legal = table.legal[`${fromKind}|${toKind}`] ?? [];
        await expect(picker.getByRole("menuitem"), `${from} → ${to}`).toHaveText([...legal, "relates (scratch)"]);
        for (const t of legal) {
          expect(table.consequences[t], `consequence of ${t}`).not.toBe("");
          await expect(picker.getByTestId(`consequence-${t}`)).toHaveText(table.consequences[t]);
        }
        if (legal.length === 0) await expect(picker.getByTestId("picker-no-typed-edge")).toBeVisible();
        const box = (await picker.boundingBox())!;
        expect(Math.abs(box.x - Math.max(8, Math.min(drop.x, vp.width - box.width - 8))), `${from} → ${to}: picker x`).toBeLessThanOrEqual(1);
        expect(Math.abs(box.y - Math.max(8, Math.min(drop.y, vp.height - box.height - 8))), `${from} → ${to}: picker y`).toBeLessThanOrEqual(1);
        expect(await interactionLive(page), `${from} → ${to}: the open picker holds the projection`).toBe(true);
        await page.keyboard.press("Escape");
        await expect(picker).toBeHidden();
        expect(await interactionLive(page), `${from} → ${to}: the closed picker releases it`).toBe(false);
      }
    }
    expect(pairs).toBe(17);
    await expect(page.getByTestId("autosave-status")).toHaveText("");
    await expect(baseThreads(page)).toHaveCount(2);

    // A gate-bearing type asks for confirmation; choosing writes the edge
    // through the existing typed-edge path and selects the new thread.
    await dragPin(page, "dc-1", page.getByTestId(refCardTestId(WALL.ADR_REF)));
    await picker.getByRole("menuitem", { name: /^supersedes/ }).click();
    const confirm = page.getByRole("alertdialog", { name: /confirm supersedes/i });
    await expect(confirm).toBeVisible();
    await expect(confirm.locator("#edge-confirm-consequence")).toHaveText(table.consequences.supersedes);
    // The consequence is read for longer than the toolbar gives a chosen
    // thread to appear (15 s): the page's clock jumps 16 s while the
    // confirmation is open, and the thread it then writes is still the
    // selection, since the write posts at Confirm, not at the choice.
    await page.evaluate(() => {
      const now = Date.now;
      Date.now = () => now() + 16_000;
    });
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);
    const supersedes = page.locator(`.yarn-chip[data-layer="spec"][data-edge-type="supersedes"][data-from="dc-1"][data-to="${WALL.ADR_REF}"]`);
    await expect(supersedes).toHaveCount(1);
    await expect(supersedes).toHaveAttribute("data-selected", "true");
    await expect(canvas(page)).toHaveAttribute("data-selection", "thread");
    await expect(pill(page).locator(".wall-status-id")).toHaveText("supersedes");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText(`dc-1 → ${WALL.ADR_REF}`);
    await page.reload();
    await expect(supersedes).toHaveCount(1);

    // A stub card's pin anchors its coverage yarn and starts no thread
    // (dc-3): dragging from it drags the paper, lights no target, opens no
    // picker, and writes no thread. The paper is released where it was,
    // and the server's drop resolution puts it back within a pixel.
    const stub = page.getByTestId(stubCardTestId(WALL.STUB_SLUG));
    const ac2 = page.getByTestId("card-ac-2");
    await stub.scrollIntoViewIfNeeded();
    const pin = (await stub.locator(".stub-pushpin").boundingBox())!;
    const start = { x: pin.x + pin.width / 2, y: pin.y + pin.height / 2 };
    const stubLeft = await stub.evaluate((el) => parseFloat((el as HTMLElement).style.left));
    const chipsBefore = await page.locator("#board-canvas .yarn-chip").count();
    await page.mouse.move(start.x, start.y);
    await page.mouse.down();
    const tb = (await ac2.boundingBox())!;
    await page.mouse.move(tb.x + tb.width / 2, tb.y + tb.height / 2, { steps: 6 });
    await expect(ac2).not.toHaveAttribute("data-drop-target", /./);
    await expect(picker).toBeHidden();
    await page.mouse.move(start.x, start.y, { steps: 6 });
    await page.mouse.up();
    await expect(picker).toBeHidden();
    await expectAutosaved(page);
    await expect(page.locator("#board-canvas .yarn-chip")).toHaveCount(chipsBefore);
    await expect(baseThreads(page)).toHaveCount(chipsBefore);
    expect(Math.abs((await stub.evaluate((el) => parseFloat((el as HTMLElement).style.left))) - stubLeft)).toBeLessThanOrEqual(1);

    // A sticky's attribution yarn and graduation drop keep their existing
    // paths and writes (dc-3): a story sticky's pin dropped on an AC claims
    // coverage through its own confirmation, never the picker, and writes
    // the scratch thread; graduating it writes the stub that claims the AC.
    const made = await page.request.post(WALL.WRITABLE_PATH + "/api/sticky", { data: { text: "retraction audit trail", type: "story" } });
    expect(made.status(), await made.text()).toBe(200);
    await page.reload();
    const proto = page.locator('[data-testid^="sticky-"][data-annotation-type="story"]').filter({ hasText: "retraction audit trail" });
    await expect(proto).toHaveCount(1);
    const protoId = (await proto.getAttribute("data-id"))!;
    await dragPin(page, protoId, ac2);
    await expect(picker).toBeHidden();
    const claim = page.getByRole("alertdialog", { name: "Claim coverage of ac-2" });
    await expect(claim).toBeVisible();
    await claim.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);
    await expect(page.locator(`.yarn-chip[data-layer="annotation"][data-edge-type="relates"][data-from="${protoId}"][data-to="ac-2"]`)).toHaveCount(1);
    await toolbarAction(page, proto, "Graduate");
    const graduate = page.locator("#edge-confirm");
    await expect(graduate).toBeVisible();
    await expect(graduate).toContainText("declares slug retraction-audit-trail");
    await page.locator("#edge-confirm-ok").click();
    await expectAutosaved(page);
    await expect(page.getByTestId(stubCardTestId("retraction-audit-trail"))).toHaveAttribute("data-acs", "ac-2");
    await expect(proto).toHaveCount(0);
  });

  test("Declaring in place and editing a card", async ({ page }) => {
    await openWritableWall(page);
    // Every typed write goes over one route; what each slot posts is read
    // here, so the id is proven the server's and the operation exactly one.
    const posted: Array<{ request: { operations: Array<Record<string, unknown>> } }> = [];
    await page.route("**/api/mutate_draft", async (route) => {
      posted.push(route.request().postDataJSON());
      await route.continue();
    });
    const kinds: Record<string, [string, string]> = {
      ac: ["acceptance criterion", "add-ac"],
      co: ["constraint", "add-constraint"],
      dc: ["decision", "add-decision"],
      oq: ["open question", "add-question"],
    };
    for (const prefix of ["ac", "co", "dc", "oq"]) {
      const [words, op] = kinds[prefix];
      const slot = page.getByTestId(`slot-${prefix}`);
      await slot.scrollIntoViewIfNeeded();
      // The slot sits at the column's foot: one row gap below the lowest
      // paper whose footprint overlaps the band (SI-350 (15)).
      const at = await slot.evaluate((el) => ({ left: (el as HTMLElement).offsetLeft, top: (el as HTMLElement).offsetTop }));
      const lowest = await page.evaluate((x) => {
        let bottom = 0;
        for (const p of Array.from(document.querySelectorAll<HTMLElement>("#board-canvas .objcard, #board-canvas .refcard, #board-canvas .stubcard, #board-canvas .sticky"))) {
          if (p.offsetLeft < x + 200 && x < p.offsetLeft + 200) bottom = Math.max(bottom, p.offsetTop + p.offsetHeight);
        }
        return bottom;
      }, at.left);
      expect(at.top, `${prefix}: the slot sits below the column's lowest paper`).toBe(lowest + 36);
      const nextId = (await canvas(page).getAttribute(`data-next-id-${prefix}`))!;
      await page.getByTestId(`slot-open-${prefix}`).click();
      await expect(slot).toHaveAttribute("data-open", "true");
      await expect(page.getByTestId(`slot-line-${prefix}`)).toHaveText(`${words} · will be declared as ${nextId}`);
      expect(await interactionLive(page), `${prefix}: an open slot holds the projection (co-2)`).toBe(true);
      const before = posted.length;
      const text = `declared from the ${words} slot`;
      await page.getByTestId(`slot-text-${prefix}`).fill(text);
      await page.keyboard.press("Enter");
      await expectAutosaved(page);
      expect(posted.length, `${prefix}: one request`).toBe(before + 1);
      expect(posted[before].request.operations, `${prefix}: one typed operation, with the server's next id`).toEqual([
        expect.objectContaining({ op, id: nextId, text, anchor: `#${nextId}` }),
      ]);
      await expect(page.getByTestId(`card-${nextId}`)).toContainText(text);
      await expect(slot).not.toHaveAttribute("data-open", /./);
      expect(await canvas(page).getAttribute(`data-next-id-${prefix}`)).not.toBe(nextId);
      // The new card took the slot's row; the slot moved one row down.
      expect(await slot.evaluate((el) => (el as HTMLElement).offsetTop)).toBe(at.top + 176);
    }

    // Escape cancels with nothing written.
    const acSlot = page.getByTestId("slot-ac");
    await acSlot.scrollIntoViewIfNeeded();
    await page.getByTestId("slot-open-ac").click();
    await expect(acSlot).toHaveAttribute("data-open", "true");
    await page.getByTestId("slot-text-ac").fill("never declared");
    const beforeEscape = posted.length;
    await page.keyboard.press("Escape");
    await expect(acSlot).not.toHaveAttribute("data-open", /./);
    expect(await interactionLive(page)).toBe(false);
    await expect(page.getByTestId("slot-open-ac")).toBeFocused();
    await page.reload();
    expect(posted.length).toBe(beforeEscape);
    await expect(page.locator("#board-canvas .objcard").filter({ hasText: "never declared" })).toHaveCount(0);

    // The existing add-object dialog stays for keyboard-only use.
    await page.locator("#asd-add-object").focus();
    await page.keyboard.press("Enter");
    await expect(page.locator("#asd-op-dialog")).toBeVisible();
    await expect(page.getByTestId("asd-op-text")).toBeFocused();
    const preview = (await page.getByTestId("asd-op-id-preview").textContent())!;
    const dialogId = preview.replace("will be declared as ", "").trim();
    expect(dialogId).toMatch(/^(ac|co|dc|oq)-\d+$/);
    await page.keyboard.type("declared from the keyboard dialog");
    for (let i = 0; i < 6; i++) {
      if ((await page.evaluate(() => document.activeElement?.id)) === "asd-op-ok") break;
      await page.keyboard.press("Tab");
    }
    expect(await page.evaluate(() => document.activeElement?.id)).toBe("asd-op-ok");
    const beforeDialog = posted.length;
    await page.keyboard.press("Enter");
    await expectAutosaved(page);
    expect(posted.length).toBe(beforeDialog + 1);
    expect(posted[beforeDialog].request.operations).toEqual([expect.objectContaining({ id: dialogId, text: "declared from the keyboard dialog" })]);
    await expect(page.getByTestId(`card-${dialogId}`)).toContainText("declared from the keyboard dialog");

    // Enter edits the selected card in place, Enter applying — with the
    // focus off the card, so it is the selection Enter acts on.
    const ac1 = page.getByTestId("card-ac-1");
    await ac1.scrollIntoViewIfNeeded();
    await ac1.click();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press("Enter");
    const editor = page.getByRole("textbox", { name: "Card text" });
    await expect(editor).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    expect(await interactionLive(page), "an open card edit holds the projection (co-2)").toBe(true);
    const original = await editor.inputValue();
    const edited = original + " — edited in place";
    const beforeEdit = posted.length;
    await editor.fill(edited);
    await page.keyboard.press("Enter");
    await expectAutosaved(page);
    await expect(editor).toHaveCount(0);
    expect(posted.length).toBe(beforeEdit + 1);
    expect(posted[beforeEdit].request.operations).toEqual([expect.objectContaining({ op: "edit-ac", id: "ac-1", text: edited })]);
    await expect(ac1.locator(".card-text")).toHaveText(edited);

    // A double click edits too and keeps the selection; Escape cancels
    // with nothing written.
    await ac1.dblclick();
    await expect(editor).toBeVisible();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await editor.fill("discarded by Escape");
    const beforeCancel = posted.length;
    await page.keyboard.press("Escape");
    await expect(editor).toHaveCount(0);
    expect(posted.length).toBe(beforeCancel);
    await expect(ac1.locator(".card-text")).toHaveText(edited);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await page.reload();
    await expect(page.getByTestId("card-ac-1").locator(".card-text")).toHaveText(edited);
  });

  test("a reopened add slot keeps its hold across a background change (co-2; Wave 6 §5.1)", async ({ page }) => {
    // A slot opened, cancelled and opened again keeps its hold: with text
    // typed and the focus gone to a card, it stays open and holds the
    // projection, so a change that lands meanwhile waits for the slot to
    // close, and the typed text survives it. The sticky is moved back
    // after, so the wall is as it was.
    await openWritableWall(page);
    const ac1 = page.getByTestId("card-ac-1");
    const coSlot = page.getByTestId("slot-co");
    await coSlot.scrollIntoViewIfNeeded();
    await page.getByTestId("slot-open-co").click();
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await page.keyboard.press("Escape");
    await expect(coSlot).not.toHaveAttribute("data-open", /./);
    await page.getByTestId("slot-open-co").click();
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await page.getByTestId("slot-text-co").fill("typed, then the focus left");
    await ac1.scrollIntoViewIfNeeded();
    await ac1.click();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(coSlot).toHaveAttribute("data-open", "true");
    expect(await interactionLive(page), "the reopened slot holds the projection").toBe(true);
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
    const at = await stickyPosition(sticky);
    await backgroundMove(page, WALL.STICKY_ID, at.x, at.y + 8);
    await expect(coSlot).toHaveAttribute("data-open", "true");
    await expect(page.getByTestId("slot-text-co")).toHaveValue("typed, then the focus left");
    await expect(sticky).toHaveCSS("top", `${at.y}px`);
    await page.getByTestId("slot-text-co").focus();
    await page.keyboard.press("Escape");
    await expect(coSlot).not.toHaveAttribute("data-open", /./);
    await expect(sticky).toHaveCSS("top", `${at.y + 8}px`);
    await backgroundMove(page, WALL.STICKY_ID, at.x, at.y);
    await expect(sticky).toHaveCSS("top", `${at.y}px`);
  });

  test("a background swap keeps the focus on the toolbar's action and on a chip (Wave 6 §5.1)", async ({ page }) => {
    // A change that lands from outside swaps the region under the toolbar
    // and the chips: the focus stays on the same action, and on the same
    // chip, each found again by its stable key. The sticky is moved and
    // moved back, so the wall is as it was.
    await openWritableWall(page);
    const toolbar = wallToolbar(page);
    const ac1 = page.getByTestId("card-ac-1");
    const sticky = page.getByTestId(`sticky-${WALL.STICKY_ID}`);
    await ac1.scrollIntoViewIfNeeded();
    await ac1.click();
    await expect(ac1).toHaveAttribute("data-selected", "true");
    const edit = toolbar.getByRole("button", { name: "Edit", exact: true });
    await edit.focus();
    await expect(edit).toBeFocused();
    const at = await stickyPosition(sticky);
    await backgroundMove(page, WALL.STICKY_ID, at.x, at.y + 8);
    await expect(sticky).toHaveCSS("top", `${at.y + 8}px`);
    await expect(ac1).toHaveAttribute("data-selected", "true");
    await expect(edit).toBeFocused();

    const exemptsChip = page.locator('.yarn-chip[data-edge-type="exempts"]');
    await exemptsChip.scrollIntoViewIfNeeded();
    await exemptsChip.focus();
    await expect(exemptsChip).toBeFocused();
    await backgroundMove(page, WALL.STICKY_ID, at.x, at.y);
    await expect(sticky).toHaveCSS("top", `${at.y}px`);
    await expect(exemptsChip).toBeFocused();
    await expect(ac1).toHaveAttribute("data-selected", "true");
  });
});
