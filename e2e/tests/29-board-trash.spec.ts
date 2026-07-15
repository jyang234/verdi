import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";
import {
  addSticky,
  dragToTrash,
  drawYarn,
  grabPoint,
  edgeTypePicker,
  expectAutosaved,
  pinArtifact,
  uncommittedIndicator,
} from "./helpers";

// The trash (owner directive, verbatim: "dragging an artifact or sticky
// to the lower right hand of the screen can bring up a positionally-
// sensitive small overlay of a trash can icon which will remove it from
// the board and disconnect any existing relationship yarn"). Removal is
// per tier: scratch dies without ceremony; record edits confirm first,
// naming what goes; cancel or Escape mid-confirm removes nothing.

// The pure-pin fixture and the wall's edge-holding ADR (the fixture's
// exempts edge, or an earlier suite file's re-drawn one — either way a
// spec-layer edge).
const PURE_PIN = SHOWCASE.PIN_TRASH_ADR;
const EDGED_REF = SHOWCASE.ADR_REF;

test.describe("board: the trash removes per tier", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("the trash sleeps until a drag nears the corner; a sticky dies in it without ceremony", async ({
    page,
  }) => {
    const trash = page.getByTestId("board-trash");
    // At rest: present in authoring, invisible, inert.
    await expect(trash).toHaveCSS("opacity", "0");
    await expect(trash).toHaveCSS("pointer-events", "none");
    await expect(trash).not.toHaveClass(/is-armed/);

    const text = "doomed: drag me to the bin";
    const sticky = await addSticky(page, text, "question");
    const dirtyBefore = await uncommittedIndicator(page).isVisible();

    await dragToTrash(page, sticky);
    await expectAutosaved(page);

    // No ceremony for scratch: no dialog, the record is gone, the spec
    // tree is exactly as it was, and the trash sleeps again.
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(0);
    expect(await uncommittedIndicator(page).isVisible()).toBe(dirtyBefore);
    await expect(trash).not.toHaveClass(/is-armed/);

    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(0);
  });

  test("a drag ending anywhere else behaves exactly as today — even armed, off the bin nothing dies", async ({
    page,
  }) => {
    const text = "survivor: parked near the corner";
    const sticky = await addSticky(page, text, "question");

    const grip = await grabPoint(page, sticky);
    const vp = page.viewportSize()!;
    await page.mouse.move(grip.x, grip.y);
    await page.mouse.down();
    // Near the corner: the trash arms...
    await page.mouse.move(vp.width - 200, vp.height - 200, { steps: 12 });
    const trash = page.getByTestId("board-trash");
    await expect(trash).toHaveClass(/is-armed/);
    await expect(trash).not.toHaveClass(/is-hot/);
    // ...but the drop lands off the bin: a plain reposition.
    await page.mouse.up();
    await expectAutosaved(page);

    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(1);
    await expect(trash).not.toHaveClass(/is-armed/);
    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(1);
  });

  test("an untyped relates thread dies in the bin", async ({ page }) => {
    await drawYarn(page, SHOWCASE.AC_IDS[0], page.getByTestId(`card-${SHOWCASE.AC_IDS[2]}`));
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /relates \(scratch\)/ }).click();
    await expectAutosaved(page);

    const thread = page.locator(
      `.yarn-chip[data-edge-type="relates"][data-from="${SHOWCASE.AC_IDS[0]}"][data-to="${SHOWCASE.AC_IDS[2]}"]`,
    );
    await expect(thread).toHaveCount(1);

    await dragToTrash(page, thread);
    await expectAutosaved(page);
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
    await expect(thread).toHaveCount(0);
    await page.reload();
    await expect(thread).toHaveCount(0);
  });

  test("a pure pin dies without ceremony — its own scratch threads go with it", async ({
    page,
  }) => {
    const card = await pinArtifact(page, PURE_PIN, "retry policy");
    await drawYarn(page, SHOWCASE.AC_IDS[0], card);
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /relates \(scratch\)/ }).click();
    await expectAutosaved(page);
    const thread = page.locator(
      `.yarn-chip[data-edge-type="relates"][data-to="${PURE_PIN}"]`,
    );
    await expect(thread).toHaveCount(1);
    const dirtyBefore = await uncommittedIndicator(page).isVisible();

    await dragToTrash(page, page.locator(`.refcard[data-ref="${PURE_PIN}"]`));
    await expectAutosaved(page);

    await expect(page.getByRole("alertdialog")).toHaveCount(0);
    await expect(page.locator(`.refcard[data-ref="${PURE_PIN}"]`)).toHaveCount(0);
    await expect(thread).toHaveCount(0);
    expect(await uncommittedIndicator(page).isVisible()).toBe(dirtyBefore);
    await page.reload();
    await expect(page.locator(`.refcard[data-ref="${PURE_PIN}"]`)).toHaveCount(0);
    await expect(thread).toHaveCount(0);
  });

  test("a reference card with typed edges confirms — cancel removes nothing, confirm removes the edges", async ({
    page,
  }) => {
    const card = page.locator(`.refcard[data-ref="${EDGED_REF}"]`);
    await expect(card).toBeVisible();
    const edgeChips = page.locator(
      `.yarn-chip[data-layer="spec"][data-to="${EDGED_REF}"]`,
    );
    const edgeCount = await edgeChips.count();
    expect(edgeCount).toBeGreaterThan(0);

    // CANCEL: the ritual names the edges, then changes nothing.
    await dragToTrash(page, card);
    const confirm = page.getByRole("alertdialog", {
      name: /take .* off the wall/i,
    });
    await expect(confirm).toBeVisible();
    // Plain language names what goes; the gate-bearing exempts restates
    // its removal consequence.
    await expect(confirm.locator("#edge-confirm-consequence")).toContainText(
      "from the spec document",
    );
    await expect(confirm.locator("#edge-confirm-consequence")).toContainText(
      "exempts",
    );
    await confirm.getByRole("button", { name: "Cancel" }).click();
    await expect(confirm).toBeHidden();
    await expect(card).toHaveCount(1);
    await expect(edgeChips).toHaveCount(edgeCount);
    await page.reload();
    await expect(page.locator(`.refcard[data-ref="${EDGED_REF}"]`)).toHaveCount(1);

    // CONFIRM: the named edges leave the spec; the card follows them.
    await dragToTrash(page, page.locator(`.refcard[data-ref="${EDGED_REF}"]`));
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);
    await expect(page.locator(`.refcard[data-ref="${EDGED_REF}"]`)).toHaveCount(0);
    await expect(edgeChips).toHaveCount(0);
    await expect(uncommittedIndicator(page)).toBeVisible();
    await page.reload();
    await expect(page.locator(`.refcard[data-ref="${EDGED_REF}"]`)).toHaveCount(0);
  });

  test("an object card confirms — Escape removes nothing; confirm removes the declaration, never the prose", async ({
    page,
  }) => {
    const card = page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`);
    await expect(card).toBeVisible();

    // ESCAPE mid-confirm: nothing is removed.
    await dragToTrash(page, card);
    const confirm = page.getByRole("alertdialog", {
      name: new RegExp(`remove ${SHOWCASE.CONSTRAINT_ID} from the spec`, "i"),
    });
    await expect(confirm).toBeVisible();
    await expect(confirm.locator("#edge-confirm-consequence")).toContainText(
      `Removes ${SHOWCASE.CONSTRAINT_ID} from the spec document`,
    );
    await expect(confirm.locator("#edge-confirm-consequence")).toContainText(
      "prose stays in the document",
    );
    await page.keyboard.press("Escape");
    await expect(confirm).toBeHidden();
    await expect(page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`)).toHaveCount(1);
    await page.reload();
    await expect(page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`)).toHaveCount(1);

    // CONFIRM: the declaration goes; the document's prose section stays.
    await dragToTrash(page, page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`));
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);
    await expect(page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`)).toHaveCount(0);
    await expect(uncommittedIndicator(page)).toBeVisible();
    await page.reload();
    await expect(page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`)).toHaveCount(0);

    // Prose is never silently destroyed: the artifact page still renders
    // the removed object's body section.
    await page.goto(`/a/spec/${SHOWCASE.DESIGN_SPEC}`);
    await expect(
      page.getByRole("heading", { name: SHOWCASE.CONSTRAINT_ID, exact: true }),
    ).toBeVisible();
  });

  test("no trash target exists outside authoring", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    await expect(page.getByTestId("board-trash")).toHaveCount(0);
  });
});
