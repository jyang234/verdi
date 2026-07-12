import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  AC_IDS,
  DECISION_PLAIN,
  ADR_REF,
  boardPath,
  refCardTestId,
} from "./fixtures";
import { addSticky, drawYarn, edgeTypePicker } from "./helpers";

// Owner UAT (round 6, item 1): "difficult to understand what it's even
// trying to say, and the user can't close out of it easily." Every board
// dialog must close three ways — Escape, clicking the backdrop, and a
// visible Cancel affordance — and a picker with no legal typed edge must
// SAY so in plain language instead of presenting a menu of nothing.
test.describe("board dialogs: always escapable, always legible", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("the edge picker closes via Cancel, backdrop, and Escape", async ({
    page,
  }) => {
    const picker = edgeTypePicker(page);
    const open = async () => {
      await drawYarn(page, DECISION_PLAIN, page.getByTestId(refCardTestId(ADR_REF)));
      await expect(picker).toBeVisible();
    };

    // A visible affordance.
    await open();
    await picker.getByRole("button", { name: "Cancel" }).click();
    await expect(picker).toBeHidden();

    // The backdrop.
    await open();
    await page.locator("#modal-backdrop").click({ position: { x: 8, y: 8 } });
    await expect(picker).toBeHidden();

    // Escape.
    await open();
    await page.keyboard.press("Escape");
    await expect(picker).toBeHidden();

    // Cancelling committed nothing.
    await page.reload();
    await expect(
      page.locator(`[data-edge-type="supersedes"][data-from="${DECISION_PLAIN}"][data-to="${ADR_REF}"]`),
    ).toHaveCount(0);
  });

  test("the graduate menu closes via Cancel and backdrop", async ({ page }) => {
    const sticky = await addSticky(page, "question: does the notice localize?");
    const menu = page.locator("#graduate-menu");

    await sticky.getByRole("button", { name: "Graduate" }).click();
    await expect(menu).toBeVisible();
    await menu.getByRole("button", { name: "Cancel" }).click();
    await expect(menu).toBeHidden();

    await sticky.getByRole("button", { name: "Graduate" }).click();
    await expect(menu).toBeVisible();
    await page.locator("#modal-backdrop").click({ position: { x: 8, y: 8 } });
    await expect(menu).toBeHidden();

    // Nothing graduated: the sticky is still a sticky.
    await expect(sticky).toBeVisible();
  });

  test("the commit dialog closes via backdrop", async ({ page }) => {
    await page.getByRole("button", { name: "Commit & push" }).click();
    const dialog = page.getByRole("dialog", { name: "Commit & push" });
    await expect(dialog).toBeVisible();
    await page.locator("#modal-backdrop").click({ position: { x: 8, y: 8 } });
    await expect(dialog).toBeHidden();
  });

  // The flow the owner actually hit: between two acceptance criteria no
  // typed edge exists. Fresh yarn must explain that in plain words (the
  // scratch thread is still offered); graduating an existing AC↔AC
  // relates thread — where "relates" isn't on the menu because the thread
  // already IS one — must explain instead of showing an empty menu.
  test("a pair with no legal typed edge gets plain language, not an empty menu", async ({
    page,
  }) => {
    // Fresh yarn AC → AC: explanation + the scratch option. (This draw
    // doubles as the buried-pin regression proof: in the full suite a
    // wide chip legitimately parks over ac-2's pushpin, and the grab
    // must resolve geometrically through it.)
    await drawYarn(page, AC_IDS[1], page.getByTestId(`card-${AC_IDS[2]}`));
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    const note = picker.getByTestId("picker-no-typed-edge");
    await expect(note).toBeVisible();
    await expect(note).toHaveText(/No typed edge exists between two acceptance criteria/);
    await expect(note).toHaveText(/scratch thread/);
    await expect(
      picker.getByRole("menuitem", { name: /relates \(scratch\)/ }),
    ).toBeVisible();

    // Take the scratch thread, then try to graduate it: no menu of
    // nothing — the same explanation, and a way out.
    await picker.getByRole("menuitem", { name: /relates \(scratch\)/ }).click();
    const thread = page.locator(
      `[data-edge-type="relates"][data-from="${AC_IDS[1]}"][data-to="${AC_IDS[2]}"]`,
    );
    await expect(thread).toHaveCount(1);

    await thread.getByRole("button", { name: "Graduate" }).click();
    await expect(picker).toBeVisible();
    await expect(picker.getByTestId("picker-no-typed-edge")).toBeVisible();
    await expect(picker.getByRole("menuitem")).toHaveCount(0);
    await picker.getByRole("button", { name: "Cancel" }).click();
    await expect(picker).toBeHidden();

    // The thread is untouched by the aborted graduation.
    await expect(thread).toHaveCount(1);
    await expect(thread).toHaveAttribute("data-layer", "annotation");
  });
});
