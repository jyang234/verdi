import { test, expect, type Locator, type Page } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";
import { addSticky, drawYarn, edgeTypePicker, expectAutosaved, toolbarAction, uncommittedIndicator, wallToolbar } from "./helpers";

// Owner UAT (round 6, item 3 + the mid-pass retype directive): scratch
// stickies and untyped threads die from the annotation layer; a
// spec-layer typed edge is removable (the inverse of drawing it, with
// the gate-bearing confirmation mirrored) and its type is updatable IN
// PLACE. Every negative path proves cancel changes nothing, reload
// included. None of these affordances exist outside authoring mode.
// Since spec/wall-canvas-v2 ac-3 (SI-350 (5)) the affordances are the
// contextual toolbar's for the selected sticky or thread (toolbarAction),
// over the same paths and confirmations.

const dc1Chip = (page: Page, type: string): Locator =>
  page.locator(
    `.yarn-chip[data-edge-type="${type}"][data-from="${SHOWCASE.DECISION_WITH_EXEMPTS}"]`,
  );

test.describe("board: scratch records die, spec edges retype and remove", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("a scratch sticky dies from the board without touching the spec", async ({
    page,
  }) => {
    const text = "doomed: does the decline email localize?";
    const sticky = await addSticky(page, text, "question");
    // Order-independent working-tree honesty: earlier suite files may
    // have left the tree dirty (drags write positions), so assert the
    // deletion CHANGES nothing — the mutable stream is not the spec.
    const dirtyBefore = await uncommittedIndicator(page).isVisible();

    await toolbarAction(page, sticky, "Delete");
    await expectAutosaved(page);
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(0);
    // The record died from the mutable stream; the spec tree's state is
    // exactly what it was.
    expect(await uncommittedIndicator(page).isVisible()).toBe(dirtyBefore);

    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(0);
  });

  test("an untyped relates thread dies from the annotation layer", async ({
    page,
  }) => {
    await drawYarn(page, SHOWCASE.AC_IDS[0], page.getByTestId(`card-${SHOWCASE.AC_IDS[2]}`));
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /relates \(scratch\)/ }).click();
    await expectAutosaved(page);

    const thread = page.locator(
      `.yarn-chip[data-edge-type="relates"][data-from="${SHOWCASE.AC_IDS[0]}"][data-to="${SHOWCASE.AC_IDS[2]}"]`,
    );
    await expect(thread).toHaveCount(1);

    await toolbarAction(page, thread, "Delete thread");
    await expectAutosaved(page);
    await expect(thread).toHaveCount(0);

    await page.reload();
    await expect(thread).toHaveCount(0);
  });

  test("retype: cancelling the confirmation changes nothing", async ({
    page,
  }) => {
    const chip = dc1Chip(page, "exempts");
    await expect(chip).toHaveCount(1);
    await toolbarAction(page, chip, "Retype");

    // The picker opens over the same pair, offering the OTHER legal type
    // only — no scratch option on a typed edge.
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await expect(picker.getByRole("menuitem", { name: /^supersedes/ })).toBeVisible();
    await expect(picker.getByRole("menuitem", { name: /^exempts/ })).toHaveCount(0);
    await expect(picker.getByRole("menuitem", { name: /relates/ })).toHaveCount(0);

    await picker.getByRole("menuitem", { name: /^supersedes/ }).click();
    const confirm = page.getByRole("alertdialog", { name: /confirm supersedes/i });
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Cancel" }).click();

    await expect(dc1Chip(page, "exempts")).toHaveCount(1);
    await expect(dc1Chip(page, "supersedes")).toHaveCount(0);
    await page.reload();
    await expect(dc1Chip(page, "exempts")).toHaveCount(1);
  });

  test("the relationship's type is updatable in place (owner directive)", async ({
    page,
  }) => {
    const chip = dc1Chip(page, "exempts");
    await toolbarAction(page, chip, "Retype");
    const picker = edgeTypePicker(page);
    await picker.getByRole("menuitem", { name: /^supersedes/ }).click();
    const confirm = page.getByRole("alertdialog", { name: /confirm supersedes/i });
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);

    await expect(dc1Chip(page, "supersedes")).toHaveCount(1);
    await expect(dc1Chip(page, "exempts")).toHaveCount(0);
    await expect(dc1Chip(page, "supersedes")).toHaveAttribute("data-to", SHOWCASE.ADR_REF);
    await expect(uncommittedIndicator(page)).toBeVisible();

    await page.reload();
    await expect(dc1Chip(page, "supersedes")).toHaveCount(1);
    await expect(dc1Chip(page, "exempts")).toHaveCount(0);
  });

  test("removing a gate-bearing edge asks first; cancel removes nothing", async ({
    page,
  }) => {
    // dc-1's edge is supersedes now (retyped by the previous test in the
    // serial suite; in isolation this test still finds the fixture's
    // exempts edge and exercises the same ritual).
    const chip = page
      .locator(`.yarn-chip[data-layer="spec"][data-from="${SHOWCASE.DECISION_WITH_EXEMPTS}"]`)
      .first();
    await expect(chip).toBeVisible();
    const type = await chip.getAttribute("data-edge-type");

    await toolbarAction(page, chip, `Remove ${type} edge`);
    const confirm = page.getByRole("alertdialog", {
      name: new RegExp(`remove ${type}`, "i"),
    });
    await expect(confirm).toBeVisible();
    // The removal consequence speaks, then Cancel wins.
    await expect(confirm.locator("#edge-confirm-consequence")).not.toBeEmpty();
    await confirm.getByRole("button", { name: "Cancel" }).click();

    await expect(dc1Chip(page, type!)).toHaveCount(1);
    await page.reload();
    await expect(dc1Chip(page, type!)).toHaveCount(1);
  });

  test("a spec-layer edge is removable — the inverse of drawing it", async ({
    page,
  }) => {
    const chip = page
      .locator(`.yarn-chip[data-layer="spec"][data-from="${SHOWCASE.DECISION_WITH_EXEMPTS}"]`)
      .first();
    const type = await chip.getAttribute("data-edge-type");

    await toolbarAction(page, chip, `Remove ${type} edge`);
    const confirm = page.getByRole("alertdialog", {
      name: new RegExp(`remove ${type}`, "i"),
    });
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);

    await expect(
      page.locator(`.yarn-chip[data-layer="spec"][data-from="${SHOWCASE.DECISION_WITH_EXEMPTS}"]`),
    ).toHaveCount(0);
    // Removing a declared edge IS a spec edit.
    await expect(uncommittedIndicator(page)).toBeVisible();

    await page.reload();
    await expect(
      page.locator(`.yarn-chip[data-layer="spec"][data-from="${SHOWCASE.DECISION_WITH_EXEMPTS}"]`),
    ).toHaveCount(0);
  });

  test("no deletion or retype affordance exists outside authoring", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    await expect(page.locator(".yarn-chip")).not.toHaveCount(0);
    await expect(page.locator(".delete-btn")).toHaveCount(0);
    await expect(page.locator("[data-retype]")).toHaveCount(0);
    await expect(page.locator(".graduate-btn")).toHaveCount(0);
    // Nor does the server stamp any action the toolbar could project, and
    // the toolbar offers none for a selected thread (ac-3).
    await expect(page.locator("[data-can-delete], [data-can-retype], [data-can-graduate]")).toHaveCount(0);
    const chip = page.locator(".yarn-chip").first();
    await chip.scrollIntoViewIfNeeded();
    await chip.click();
    await expect(chip).toHaveAttribute("data-selected", "true");
    await expect(wallToolbar(page).getByRole("button", { name: /Delete|Remove|Retype|Graduate/ })).toHaveCount(0);
  });
});
