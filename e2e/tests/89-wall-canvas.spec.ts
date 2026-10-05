import { test, expect, type Page, type Locator } from "@playwright/test";
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
const pill = (page: Page) => page.getByTestId("wall-status");
const overlayThreads = (page: Page) => page.locator("#board-canvas svg.yarn-overlay path.yarn-thread");
const baseThreads = (page: Page) => page.locator("#board-canvas svg.yarn-svg path.yarn-thread");

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
  // stays unproven, never a pass.
  test.fixme(
    "Cards at the new footprint with their receipts, and yarn in two layers",
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
    const exemptsChip = page.locator('.yarn-chip[data-edge-type="exempts"]');
    const coversChip = page.locator('.yarn-chip[data-edge-type="covers"]');
    const opacityOf = (el: Locator) => el.evaluate((node) => Number(getComputedStyle(node).opacity));

    // Clicking a card selects it: the card and every card threaded to it
    // are emphasized, everything else recedes.
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await expect(ref).toHaveAttribute("data-linked", "exempts");
    await expect(canvas(page)).toHaveAttribute("data-selection", "card");
    await expect(exemptsChip).toHaveAttribute("data-hot", "true");
    await expect(coversChip).not.toHaveAttribute("data-hot", /./);
    await expect.poll(() => opacityOf(dc1)).toBe(1);
    await expect.poll(() => opacityOf(ref)).toBe(1);
    await expect.poll(() => opacityOf(co1)).toBeCloseTo(0.32, 2);
    await expect.poll(() => opacityOf(coversChip)).toBeCloseTo(0.25, 2);
    await expect.poll(() => opacityOf(exemptsChip)).toBe(1);
    // The status pill names the card and its threads.
    await expect(pill(page)).toHaveAttribute("role", "status");
    await expect(pill(page).locator(".wall-status-id")).toHaveText("dc-1");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText(`exempts → ${WALL.ADR_REF}`);

    // A card with no threads says so — and this wall offers no pin to
    // drag, so the pill does not mention one (SI-350 (14) as amended).
    await co1.click();
    await expect(co1).toHaveAttribute("data-selected", "true");
    await expect(dc1).not.toHaveAttribute("data-selected", /./);
    await expect(page.locator("#board-canvas [data-linked]")).toHaveCount(0);
    await expect(pill(page).locator(".wall-status-id")).toHaveText("co-1");
    await expect(pill(page).locator(".wall-status-summary")).toHaveText("no threads yet");

    // Clicking the wall clears the selection.
    await canvas(page).click({ position: { x: 300, y: 520 } });
    await expect(page.locator("#board-canvas [data-selected]")).toHaveCount(0);
    await expect(canvas(page)).not.toHaveAttribute("data-selection", /./);
    await expect(pill(page)).toBeEmpty();
    await expect.poll(() => opacityOf(co1)).toBe(1);

    // Clicking the same card clears it too (a beat apart: two quick clicks
    // are a double click, which edits).
    await dc1.click();
    await expect(dc1).toHaveAttribute("data-selected", "true");
    await page.waitForTimeout(600);
    await dc1.click();
    await expect(dc1).not.toHaveAttribute("data-selected", /./);
    await expect(pill(page)).toBeEmpty();

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
  });
});
