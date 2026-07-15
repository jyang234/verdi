import { test, expect, type Page } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";
import { addSticky, expectAutosaved } from "./helpers";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P6, exit
// criterion 4: "layout stability (adding a new object never moves another
// object's stored layout.json position — the S8 proof re-verified at the
// UI layer)"; 05 §Workbench, "Layout: zoned, incremental, position-stable":
// "Stored coordinates are never moved by generation; landing a new object
// never re-flows the board ... Only the property binds: same inputs →
// same layout, stored positions never moved."
test.describe("V1-P6: layout stability at the UI layer", () => {
  // Snapshot every object card's rendered position, keyed by testid.
  const cardPositions = async (page: Page) => {
    const cards = page.locator('[data-testid^="card-"]');
    await expect(cards.first()).toBeVisible();
    const entries = await cards.evaluateAll((els) =>
      els.map((el) => [
        el.getAttribute("data-testid"),
        {
          left: (el as HTMLElement).style.left,
          top: (el as HTMLElement).style.top,
        },
      ]),
    );
    return new Map(entries as [string, { left: string; top: string }][]);
  };

  test("adding an object never moves an existing card", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const before = await cardPositions(page);
    expect(before.size).toBeGreaterThan(0);

    // Add a new object through the board's own affordance: a scratch
    // sticky graduated to a constraint (an ordinary edit, 05 §Workbench's
    // scratch tier) — the new object lands in its kind's zone at the next
    // free slot.
    const stickyText = "constraint: decline notices localize per region";
    const sticky = await addSticky(page, stickyText);
    await sticky.getByRole("button", { name: "Graduate" }).click();
    await page.getByRole("menuitem", { name: "Constraint" }).click();
    await expectAutosaved(page);

    const newCard = page
      .locator('[data-testid^="card-"][data-object-kind="constraint"]')
      .filter({ hasText: "localize per region" });
    await expect(newCard).toHaveCount(1);

    // Every pre-existing card sits exactly where it was — no re-flow.
    const after = await cardPositions(page);
    expect(after.size).toBe(before.size + 1);
    for (const [id, pos] of before) {
      expect(after.get(id), `card ${id} moved when a new object landed`).toEqual(
        pos,
      );
    }

    // And the stored positions truly never moved: a fresh projection from
    // the working tree (spec + layout.json) reproduces the same map.
    await page.reload();
    const reloaded = await cardPositions(page);
    expect(reloaded.size).toBe(after.size);
    for (const [id, pos] of before) {
      expect(
        reloaded.get(id),
        `card ${id}'s stored position changed across reload`,
      ).toEqual(pos);
    }
  });
});
