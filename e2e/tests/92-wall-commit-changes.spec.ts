import { test, expect, type Page } from "@playwright/test";
import { uncommittedIndicator } from "./helpers";

// spec/wall-strip-and-drawer-v2 ac-3 — Commit and push shows the
// uncommitted changes in three distinct states. The test is the producer
// its obligation names (.verdi/obligations/wall-strip-and-drawer-v2/
// ac-3--behavioral.md), titled as the claim spells it; the file passes
// when run alone (BL-98).
//
// The walls are cmd/e2eharness/provision_wallstrip.go's: four authoring
// walls on their own namesake branches, each served at its writable path
// from a pre-cut worktree whose working tree holds one change state. The
// names and paths are copied here, not imported (fixtures.ts stays F7's),
// and pinned by the harness's TestWallStripPaths.
const WALLS = {
  typed: "/b/design%2Fdecline-changes-typed/board/spec/decline-changes-typed",
  unclassified: "/b/design%2Fdecline-changes-unclassified/board/spec/decline-changes-unclassified",
  mixed: "/b/design%2Fdecline-changes-mixed/board/spec/decline-changes-mixed",
  unreadable: "/b/design%2Fdecline-changes-unreadable/board/spec/decline-changes-unreadable",
} as const;

// The untracked note the unclassified and mixed walls hold (the harness's
// wallStripNotePath).
const NOTE = "notes/retraction-copy.md";

function commit(page: Page) {
  return page.getByTestId("wall-commit");
}

// openPopover opens Commit and push's changes popover from its count and
// returns the changes body.
async function openPopover(page: Page) {
  const details = page.getByTestId("wall-commit-popover");
  const count = page.getByTestId("wall-commit-count");
  const body = page.getByTestId("wall-commit-changes");
  // Never on hover alone: hovering the count shows nothing.
  await count.hover();
  await page.waitForTimeout(300);
  await expect(details).not.toHaveJSProperty("open", true);
  await expect(body).toBeHidden();
  await count.click();
  await expect(details).toHaveJSProperty("open", true);
  await expect(body).toBeVisible();
  return body;
}

test.describe("wall-strip-and-drawer", () => {
  test("Commit and push shows the three change states", async ({ page }) => {
    test.setTimeout(120_000);

    // Typed changes, with their badges: the problem replaced reads
    // "edited" and the added criterion "added", each with its raw kind.
    await page.goto(WALLS.typed);
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    await expect(commit(page)).toHaveAttribute("data-changes", "typed");
    await expect(page.getByTestId("wall-commit-count")).toHaveText(/^\d+ changes?$/);
    await expect(uncommittedIndicator(page)).toBeVisible();
    let body = await openPopover(page);
    await expect(body.getByTestId("wall-commit-typed")).toBeVisible();
    await expect(body.getByTestId("wall-commit-unclassified")).toHaveCount(0);
    await expect(body.getByTestId("wall-commit-unreadable")).toHaveCount(0);
    const problem = body.locator('[data-target="problem"]');
    await expect(problem).toHaveAttribute("data-change", "replaced");
    await expect(problem.locator(".wall-commit-badge")).toHaveText("edited");
    const added = body.locator('[data-target="ac-2"]');
    await expect(added).toHaveAttribute("data-change", "added");
    await expect(added.locator(".wall-commit-badge")).toHaveText("added");
    for (const badge of await body.locator(".wall-commit-badge").allTextContents()) {
      expect(["added", "changed", "edited"], "a ruled badge word").toContain(badge);
    }
    // Escape closes it and returns the focus to the count; the keyboard
    // opens it again from the focused count.
    await page.keyboard.press("Escape");
    await expect(body).toBeHidden();
    await expect(page.getByTestId("wall-commit-count")).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(body).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(body).toBeHidden();

    // Unclassified changes only: a prose edit the comparison cannot type,
    // listed with its path and reason — and the indicator stays set,
    // since a comparison with zero recognized operations never clears it
    // while a change remains.
    await page.goto(WALLS.unclassified);
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    await expect(commit(page)).toHaveAttribute("data-changes", "unclassified");
    await expect(uncommittedIndicator(page)).toBeVisible();
    await expect(page.getByTestId("wall-commit-count")).toHaveText(/^\d+ changes?$/);
    body = await openPopover(page);
    await expect(body.getByTestId("wall-commit-unclassified")).toBeVisible();
    await expect(body.getByTestId("wall-commit-typed")).toHaveCount(0);
    await expect(body.getByTestId("wall-commit-unreadable")).toHaveCount(0);
    const prose = body.locator('[data-reason="prose-or-body-text"]');
    await expect(prose).toHaveCount(1);
    await expect(prose.locator(".wall-commit-reason")).toHaveText("prose or body text");
    await expect(body.locator(`[data-path="${NOTE}"]`)).toHaveAttribute("data-reason", "untracked-file");
    // Over the next polls the indicator is still set: nothing clears it.
    await page.waitForTimeout(2_600);
    await expect(uncommittedIndicator(page)).toBeVisible();
    await expect(commit(page)).toHaveAttribute("data-changes", "unclassified");

    // Mixed: typed and unclassified side by side, each in its own section.
    await page.goto(WALLS.mixed);
    await expect(commit(page)).toHaveAttribute("data-changes", "mixed");
    await expect(uncommittedIndicator(page)).toBeVisible();
    body = await openPopover(page);
    await expect(body.getByTestId("wall-commit-typed")).toBeVisible();
    await expect(body.getByTestId("wall-commit-unclassified")).toBeVisible();
    await expect(body.getByTestId("wall-commit-unreadable")).toHaveCount(0);
    await expect(body.locator('[data-target="problem"] .wall-commit-badge')).toHaveText("edited");
    await expect(body.locator(`[data-path="${NOTE}"]`)).toHaveAttribute("data-reason", "untracked-file");

    // Unreadable: the spec was never committed, so there is no HEAD
    // revision to compare against — disclosed, with no typed list and no
    // count claimed.
    await page.goto(WALLS.unreadable);
    await expect(commit(page)).toHaveAttribute("data-changes", "unreadable");
    await expect(page.getByTestId("wall-commit-count")).toHaveText("unreadable");
    await expect(uncommittedIndicator(page)).toBeVisible();
    body = await openPopover(page);
    const unreadable = body.getByTestId("wall-commit-unreadable");
    await expect(unreadable).toBeVisible();
    await expect(unreadable).toContainText("The comparison with HEAD is unreadable:");
    await expect(body.getByTestId("wall-commit-typed")).toHaveCount(0);
    // An outside press closes the popover.
    await page.mouse.click(8, 8);
    await expect(body).toBeHidden();
  });
});
