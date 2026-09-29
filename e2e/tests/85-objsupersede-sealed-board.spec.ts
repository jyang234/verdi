import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath, branchBoardPath } from "./fixtures";

// The sealed board's disclosure for closed-spec object supersession
// (design docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-
// design.md §6; whole-wave review F-8; the board closure's C-5, pinned in
// Go by internal/workbench's TestLoadSealed_DisclosesMissingLines). A
// /b/<branch> board whose branch resolves only to a remote-tracking ref
// renders SEALED from the ref's committed content (draft-boards dc-4;
// 38-draft-boards.spec.ts): no worktree is cut, so the objsupersede views
// — which read a working tree — are not computed on that render. The
// board says so in its notice, never silence; a board over a local
// checkout carries no such disclosure. The remote-only branch is the
// shared store's design/sealed-remote (cmd/e2eharness/provision_draftboards.go).
// Assertions are on test ids and element counts.
//
// What the browser proves here is the NOTICE. The zero-markup assertions
// in test 1 are a regression guard only: the sealed-remote spec declares
// no decision, no `supersedes` edge, and links no closed object
// (provision_draftboards.go), so no supersession markup could render on
// its board even if the views WERE computed, and those assertions cannot
// fail for the reason the notice names. The suppression itself — a
// sealed render over records with real edges computing no views — is
// proven in Go by internal/workbench's TestLoadSealed_DisclosesMissingLines
// (the proposed scenario, made remote-only), not in this browser test.

const NOT_COMPUTED = "closed-spec object supersession lines (design §6) are not computed on this sealed render";

test.describe("closed-spec object supersession on the sealed board", () => {
  test("a remote-only branch's sealed board discloses that the §6 lines are not computed (the suppression itself is Go-proven)", async ({ page }) => {
    await page.goto(branchBoardPath(`design/${SHOWCASE.DB_SEALED_REMOTE}`, SHOWCASE.DB_SEALED_REMOTE));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    const notice = page.getByTestId("board-notice").filter({ hasText: NOT_COMPUTED });
    await expect(notice).toHaveCount(1);
    await expect(notice).toBeVisible();
    // The disclosure rides the remoteness notice and names where the
    // lines are shown instead.
    await expect(notice).toContainText(`remote-tracking ref origin/design/${SHOWCASE.DB_SEALED_REMOTE}`);
    await expect(notice).toContainText("the docs site built from the default branch, or the branch fetched as a local branch, shows them");
    // Regression guard, not a proof (see the file header): this store has
    // nothing to supersede, so these counts are 0 whatever the render
    // does; TestLoadSealed_DisclosesMissingLines proves the suppression.
    await expect(page.locator(".objsupersede-lines")).toHaveCount(0);
    await expect(
      page.locator('[data-testid^="objsupersede-"], [data-testid^="card-supersession-"], [data-testid="refcard-supersession"]'),
    ).toHaveCount(0);
  });

  test("a board over a local checkout carries no such disclosure", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    await expect(page.getByTestId("board-notice").filter({ hasText: NOT_COMPUTED })).toHaveCount(0);
  });
});
