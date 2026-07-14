import { test, expect, type Page } from "@playwright/test";
import { addSticky } from "./helpers";
import {
  SIZE_SMELL_SPEC,
  SIZE_FIT_SPEC,
  SIZE_SMELL_ESTIMATE,
  SIZE_SMELL_REFERENCE,
  EMPTY_SPEC,
  boardPath,
} from "./fixtures";

// spec/case-file-flags ac-2/ac-3 (and the parent wall-receipts ac-5): an
// acceptance-criteria column whose dc-1 ESTIMATE exceeds the declared
// reference-viewport-height constant raises the size-smell badge on the
// case file — an observation, never a rule — and the badge is INVARIANT
// TO THE CLIENT: same badge, same drawer content, at two distinct browser
// viewport sizes, with no rendered drawer value equal to either actual
// viewport height (the falsifiable form of "the drawer never cites a
// client viewport measurement").
//
// The two viewport heights deliberately straddle the reference constant
// (one shorter than 900, one taller) and are chosen so neither number
// can collide with a genuine drawer operand (40, 176, 140, 36, 900, 5,
// 920) even as a substring.
const SHORT_VIEWPORT = { width: 1880, height: 700 };
const TALL_VIEWPORT = { width: 1880, height: 1100 };

function sizeSmellStamp(page: Page) {
  return page
    .getByTestId("case-file-badges")
    .locator('.case-stamp[data-badge-source="observe:size-smell"]');
}

// Read the stamp's full serialized derivation record — the drawer's one
// content source (data-badge-record, the derivation-drawer opener
// contract).
async function stampRecord(page: Page): Promise<string> {
  const stamp = sizeSmellStamp(page);
  await expect(stamp).toBeVisible();
  const raw = await stamp.getAttribute("data-badge-record");
  expect(raw).toBeTruthy();
  return raw!;
}

test.describe("size-smell: an observation on the case file, invariant to the client", () => {
  test("the badge and its drawer content are identical at two viewport sizes, and cite no client measurement", async ({
    page,
  }) => {
    await page.setViewportSize(SHORT_VIEWPORT);
    await page.goto(boardPath(SIZE_SMELL_SPEC));
    const shortRecord = await stampRecord(page);

    await page.setViewportSize(TALL_VIEWPORT);
    await page.goto(boardPath(SIZE_SMELL_SPEC));
    const tallRecord = await stampRecord(page);

    // Byte-identical across viewports: the compute ran server-side over
    // pinned inputs; nothing client-side fed or rewrote it (ac-3).
    expect(tallRecord).toBe(shortRecord);

    const record = JSON.parse(shortRecord);
    expect(record.source).toBe("observe:size-smell");
    const records: string[] = record.records;
    const all = records.join("\n");

    // The derivation discloses every operand by name and value (dc-1):
    // the layout constants, the reference constant AS A CONSTANT, the AC
    // count, and the computed estimate.
    expect(all).toContain("boardlayout.ZoneOriginY");
    expect(all).toContain("boardlayout.RowPitch");
    expect(all).toContain(
      `wallbadge.ReferenceViewportHeight = ${SIZE_SMELL_REFERENCE}`,
    );
    expect(all).toContain("declared acceptance criteria: 5");
    expect(all).toContain(`${SIZE_SMELL_ESTIMATE}`);

    // No rendered drawer value equals either ACTUAL viewport height —
    // the obligation's falsifiable form of "never a client measurement".
    const numbers = (all.match(/\d+/g) ?? []).map(Number);
    expect(numbers.length).toBeGreaterThan(0);
    expect(numbers).not.toContain(SHORT_VIEWPORT.height);
    expect(numbers).not.toContain(TALL_VIEWPORT.height);
    expect(numbers).not.toContain(SHORT_VIEWPORT.width);

    // Observation register (dc-2): the copy observes — "worth a scoping
    // look" — it never speaks an error's voice.
    expect(all).toContain("worth a scoping look");
    expect(all.toLowerCase()).not.toContain("error");
  });

  test("an AC column that fits the reference viewport raises nothing, at either viewport", async ({
    page,
  }) => {
    for (const viewport of [SHORT_VIEWPORT, TALL_VIEWPORT]) {
      await page.setViewportSize(viewport);
      await page.goto(boardPath(SIZE_FIT_SPEC));
      // The wall renders (its ACs are on the board) but wears no
      // size-smell stamp: the estimate — not any client height — decides.
      await expect(page.getByTestId("card-ac-1")).toBeVisible();
      await expect(sizeSmellStamp(page)).toHaveCount(0);
    }
  });

  test("dragging an AC card never changes the badge — positions are not an operand", async ({
    page,
  }) => {
    await page.goto(boardPath(SIZE_SMELL_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
    const before = await stampRecord(page);

    // Drag ac-1 well away from its slot; the drag autosaves a stored
    // position into layout.json (a real pinned-input write — but not one
    // of dc-1's operands).
    const card = page.getByTestId("card-ac-1");
    const box = await card.boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
    await page.mouse.down();
    await page.mouse.move(
      box!.x + box!.width / 2 + 320,
      box!.y + box!.height / 2 + 60,
      { steps: 10 },
    );
    await page.mouse.up();
    await expect(page.getByTestId("autosave-status")).toHaveText("saved", {
      timeout: 5_000,
    });

    // A fresh server render over the mutated layout.json: same badge,
    // byte-identical derivation (dragging paper around must not create
    // or destroy an observation about the spec's size — dc-1).
    await page.reload();
    const after = await stampRecord(page);
    expect(after).toBe(before);
  });

  test("every write path still succeeds on the badged wall — an observation, never a rule", async ({
    page,
  }) => {
    await page.goto(boardPath(SIZE_SMELL_SPEC));
    await expect(sizeSmellStamp(page)).toBeVisible();

    // A real write on the badged wall succeeds unchanged (co-2: nothing
    // blocks, gates, or refuses on the smell), and the post-mutation
    // fragment still wears the badge.
    await addSticky(page, "five decline paths is worth a scoping look");
    await expect(sizeSmellStamp(page)).toBeVisible();
  });
});

// spec/case-file-flags ac-1/dc-4: the ladder's disclosed-unproven outcome
// on the case file. The harness's `verdi serve` runs with NO forge
// configured, so EMPTY_SPEC — a story with an implements edge — cannot
// have its open MRs enumerated: pending-supersession is disclosed-
// unproven, and it renders as a case-file disclosure LINE in the board's
// notice vocabulary, never a stamp (unproven is never dressed as a
// verdict in either direction) and never silence.
test.describe("case-file flags: disclosed-unproven is a line, never a stamp", () => {
  test("a story wall with no forge wears the pending-supersession disclosure line on its case file", async ({
    page,
  }) => {
    await page.goto(boardPath(EMPTY_SPEC));
    const line = page.getByTestId("case-file-disclosure");
    await expect(line).toBeVisible();
    await expect(line).toContainText("pending-supersession is disclosed-unproven");
    // The line sits INSIDE the case-file lockup and speaks the board's
    // notice vocabulary (the disclosed board-notice voice).
    await expect(
      page.locator(".case-file [data-testid='case-file-disclosure']"),
    ).toBeVisible();
    await expect(line).toHaveClass(/board-notice/);
    // Never a stamp: unproven must not dress as a verdict.
    await expect(
      page.locator('.case-stamp[data-badge-source="ladder:pending-supersession"]'),
    ).toHaveCount(0);
  });
});
