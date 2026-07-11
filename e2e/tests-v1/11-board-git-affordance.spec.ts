import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  MAIN_BRANCH,
  boardPath,
  AC_IDS,
} from "./fixtures";
import { editCard, uncommittedIndicator } from "./helpers";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P6, exit
// criterion 1: "author edit on a design branch → commit (the git
// affordance commits/pushes the working tree, the uncommitted-changes
// indicator clears, a branch switch mid-edit is guarded)"; 05 §Workbench,
// authoring-mode bullet: "the board owns the git affordance: a commit/push
// button (message prompt, executes git on the design branch underneath), a
// persistent uncommitted-changes indicator, and a branch-switch guard in
// `verdi serve` — a PM or designer must be able to author and durably save
// without git fluency".
//
// Each test establishes its own dirty working tree with a card edit, so
// the tests stay meaningful independent of ordering.
test.describe("V1-P6: board-owned git affordance", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  // Authoring is bidirectional: "changes autosave to the working tree"
  // (05 §Workbench); the dirty tree surfaces as the persistent indicator.
  test("card edit autosaves and raises the uncommitted-changes indicator", async ({
    page,
  }) => {
    await editCard(page, AC_IDS[0], (text) => `${text} [edited-by-e2e]`);

    // The edit reached the spec's object, not a board-local copy.
    await expect(page.getByTestId(`card-${AC_IDS[0]}`)).toContainText(
      "[edited-by-e2e]",
    );
    await expect(uncommittedIndicator(page)).toBeVisible();
  });

  // "a branch-switch guard in `verdi serve` ... an hour of board work
  // evaporating in someone else's working tree is exactly the silent loss
  // this system exists to forbid" (05 §Workbench).
  test("branch-switch guard blocks a mid-edit switch", async ({ page }) => {
    await editCard(page, AC_IDS[1], (text) => `${text} [guard-e2e]`);
    await expect(uncommittedIndicator(page)).toBeVisible();

    await page.getByTestId("branch-switcher").click();
    await page.getByRole("menuitem", { name: MAIN_BRANCH }).click();

    // The guard interrupts instead of switching.
    const guard = page.getByRole("alertdialog", { name: "Uncommitted changes" });
    await expect(guard).toBeVisible();
    await guard.getByRole("button", { name: "Stay on branch" }).click();
    await expect(guard).toBeHidden();

    // Still authoring the same spec on the design branch; nothing was lost.
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
    await expect(page.getByTestId(`card-${AC_IDS[1]}`)).toContainText(
      "[guard-e2e]",
    );
    await expect(uncommittedIndicator(page)).toBeVisible();
  });

  // "Commits are explicit ... a commit/push button (message prompt,
  // executes git on the design branch underneath)" (05 §Workbench); the
  // indicator clearing is the V1-P6 exit criterion's observable proof.
  test("commit/push prompts for a message and clears the indicator", async ({
    page,
  }) => {
    await editCard(page, AC_IDS[2], (text) => `${text} [commit-e2e]`);
    await expect(uncommittedIndicator(page)).toBeVisible();

    await page.getByRole("button", { name: "Commit & push" }).click();

    const dialog = page.getByRole("dialog", { name: "Commit & push" });
    await expect(dialog).toBeVisible();
    await dialog
      .getByRole("textbox", { name: "Commit message" })
      .fill("board: e2e authoring edits");
    await dialog.getByRole("button", { name: "Commit" }).click();

    await expect(dialog).toBeHidden();
    await expect(uncommittedIndicator(page)).toBeHidden();

    // Durable: a reload still shows a clean tree and the committed edit.
    await page.reload();
    await expect(page.getByTestId(`card-${AC_IDS[2]}`)).toContainText(
      "[commit-e2e]",
    );
    await expect(uncommittedIndicator(page)).toBeHidden();
  });
});
