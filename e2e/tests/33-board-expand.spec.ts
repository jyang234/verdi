import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  EMPTY_SPEC,
  REVIEW_SPEC,
  AC_IDS,
  boardPath,
} from "./fixtures";
import { expectAutosaved } from "./helpers";

// Click-to-expand (owner directive: "the output is truncated. Clicking on
// it should open a dialog that expands it."). The wall clamps text to keep
// every paper a bounded footprint; a clamp that actually cuts text must be
// visible, not silent — the element fades its last line, marks itself with
// a quiet "⋯", and opens a READ-ONLY expand dialog on a click. The dialog
// is the existing board-dialog chrome (× / backdrop / Escape), and the
// affordance appears ONLY when the text measurably overflows.
//
// The fixtures: DESIGN_SPEC (refi-decline-flow) carries a deliberately long
// problem that overflows the placard's three-line clamp (authoring room);
// REVIEW_SPEC (stale-decline-notices) carries a long problem too, proving
// the affordance works in a non-authoring room; EMPTY_SPEC
// (income-verification) keeps a one-line problem — the negative case.

test.describe("board expand: truncated text opens a read-only dialog", () => {
  test("a clamped placard shows the hint and expands to its full text", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    const placard = page.getByTestId("placard-problem");
    await expect(placard).toBeVisible();
    const p = placard.locator("p");

    // The hint is only-when-truncated: the placard's own paragraph is
    // measurably clamped, so it wears .is-clamped, advertises itself as
    // expandable, changes the cursor, and grows a "⋯" mark.
    await expect(p).toHaveClass(/is-clamped/);
    await expect(p).toHaveAttribute("data-expandable", "");
    await expect(p).toHaveCSS("cursor", "zoom-in");
    await expect(placard.locator(".clamp-more")).toHaveCount(1);

    // The full string is in the DOM already (the clamp only hides it);
    // capture it to compare against what the dialog reads back.
    const full = (await p.textContent())!.trim();
    expect(full.length).toBeGreaterThan(200); // genuinely long

    // A click opens the read-only expand dialog: header names the element
    // ("PROBLEM"), body is the FULL text, in the existing dialog chrome.
    await p.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog).toHaveClass(/board-dialog/); // the shared chrome
    await expect(dialog.locator(".expand-kind")).toHaveText("PROBLEM");
    await expect(page.getByTestId("expand-text")).toHaveText(full);

    // Read-only: opening it writes NOTHING (no autosave fired).
    await expect(page.getByTestId("autosave-status")).toHaveText("");

    // Closes on Escape, and it is reload-proof — nothing was persisted.
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await page.reload();
    await expect(page.getByTestId("placard-problem").locator("p")).toHaveText(
      full,
    );
    await expect(
      page.getByTestId("placard-problem").locator("p"),
    ).toHaveClass(/is-clamped/);
  });

  test("the expand dialog closes from × and the backdrop too", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    const p = page.getByTestId("placard-problem").locator("p");

    // × closes.
    await p.click();
    let dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(dialog).toHaveCount(0);

    // The backdrop closes (a soft-scrim click, the same exit every board
    // dialog offers).
    await p.click();
    dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    // Click the scrim at a corner clear of the centered dialog.
    await page.locator("#expand-backdrop").click({ position: { x: 8, y: 8 } });
    await expect(dialog).toHaveCount(0);
  });

  test("a short placard gets no affordance", async ({ page }) => {
    // EMPTY_SPEC's one-line problem fits — no clamp, no hint, no cursor
    // change, and a click does nothing.
    await page.goto(boardPath(EMPTY_SPEC));
    const p = page.getByTestId("placard-problem").locator("p");
    await expect(p).toBeVisible();
    await expect(p).not.toHaveClass(/is-clamped/);
    await expect(p).not.toHaveAttribute("data-expandable", "");
    await expect(p).toHaveCSS("cursor", "auto");
    await expect(page.getByTestId("placard-problem").locator(".clamp-more")).toHaveCount(
      0,
    );

    await p.click();
    // A generous beat past the expand delay: the dialog never appears.
    await page.waitForTimeout(400);
    await expect(page.getByTestId("expand-dialog")).toHaveCount(0);
  });

  test("the affordance works in a non-authoring (review) room too", async ({
    page,
  }) => {
    await page.goto(boardPath(REVIEW_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "review",
    );
    const p = page.getByTestId("placard-problem").locator("p");
    await expect(p).toHaveClass(/is-clamped/);
    const full = (await p.textContent())!.trim();

    await p.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId("expand-text")).toHaveText(full);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  });

  test("a card double-click still edits; a single click expands", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    // Double-click still opens the inline editor (the click-to-expand must
    // not eat the dblclick) — no expand dialog appears.
    const card = page.getByTestId(`card-${AC_IDS[0]}`);
    await card.dblclick();
    const editor = page.getByRole("textbox", { name: "Card text" });
    await expect(editor).toBeVisible();
    await expect(page.getByTestId("expand-dialog")).toHaveCount(0);
    // Leave the editor without changing anything (blur commits nothing).
    await editor.blur();

    // Make this card's text long enough to clamp, then a single click on
    // the text opens the expand dialog (not the editor).
    await card.dblclick();
    const editor2 = page.getByRole("textbox", { name: "Card text" });
    const longText =
      "a declined applicant sees the current decline reason within a minute of a data change, across web and mobile, and the reason shown matches the servicing system of record exactly";
    await editor2.fill(longText);
    await editor2.blur();
    await expectAutosaved(page);

    const cardText = card.locator(".card-text");
    await expect(cardText).toHaveClass(/is-clamped/);
    await cardText.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId("expand-text")).toHaveText(longText);
    await expect(dialog.locator(".expand-kind")).toContainText(AC_IDS[0]);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  });

  test("a drag is still a drag, not an expand", async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    const card = page.getByTestId(`card-${AC_IDS[0]}`);
    await card.scrollIntoViewIfNeeded();

    const before = await card.boundingBox();
    expect(before).not.toBeNull();

    // A real drag past the slop: press on the card text, move, release.
    const text = card.locator(".card-text");
    const tb = await text.boundingBox();
    expect(tb).not.toBeNull();
    await page.mouse.move(tb!.x + tb!.width / 2, tb!.y + tb!.height / 2);
    await page.mouse.down();
    await page.mouse.move(tb!.x + 160, tb!.y + 140, { steps: 12 });
    await page.mouse.up();
    await expectAutosaved(page);

    // The card moved (a drag), and the drag's click tail did NOT open the
    // expand dialog.
    await page.waitForTimeout(400);
    await expect(page.getByTestId("expand-dialog")).toHaveCount(0);
    const after = await card.boundingBox();
    expect(Math.abs(after!.x - before!.x) + Math.abs(after!.y - before!.y)).toBeGreaterThan(
      40,
    );
  });
});
