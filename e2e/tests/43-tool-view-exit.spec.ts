import { test, expect, type Page } from "@playwright/test";
import { SHOWCASE, boardPath, diagramEditorPath } from "./fixtures";
import { dragToTrash, expectAutosaved, pinArtifact } from "./helpers";

// spec/tool-view-exit ac-1: the workbench's one board tool view — the
// diagram designer — renders an explicit, visible exit affordance in its
// page chrome, and the Escape key does the same. Entered via a spec
// board's pinned diagram reference card (dc-2: the card's own href carries
// a board=<spec-name> query parameter, request-scoped, never persisted),
// both the affordance and Escape return to that exact board, fully
// rendered. Entered with no originating board known — a direct URL, or the
// corpus page's editor link, neither of which is a board — both instead
// disclose that honestly and fall back to the index (dc-3), never a
// broken link (parent wl co-2).

const ref = `diagram/${SHOWCASE.DIAGRAM_PROPOSAL}`;

// boarddiagram.js registers the Escape binding only once it has finished
// loading and executing — which happens strictly after the vendored
// mermaid.min.js script tag ahead of it in the page, since both are
// ordinary synchronous <script src> tags. The server-rendered editor
// region (data-testid="diagram-editor") becomes visible via the browser's
// incremental HTML parsing well before that — a real, deterministic gap,
// not mere flakiness. The live preview's rendered SVG is the same
// readiness signal 37-board-diagram-editor.spec.ts's own first test
// already waits on (it also depends on boarddiagram.js having run), so
// waiting for it here is what actually proves the page is ready to
// receive Escape, rather than merely present on screen.
async function waitForEditorReady(page: Page): Promise<void> {
  await expect(page.getByTestId("diagram-editor")).toBeVisible();
  await expect(page.locator("#diagram-preview svg")).toBeVisible();
}

// Enters the editor from SHOWCASE.DESIGN_SPEC's board via a freshly pinned
// diagram reference card — the one path that carries a board= parameter.
async function enterEditorFromBoard(page: Page): Promise<void> {
  await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
  const card = await pinArtifact(page, ref, SHOWCASE.DIAGRAM_PROPOSAL);
  await card.getByTestId("refcard-editor-link").click();
  await waitForEditorReady(page);
}

// The originating board, confirmed restored — fully rendered, not blank
// or broken (ac-1's own bar): the exact URL, the authoring board itself,
// and its real problem-placard content.
async function expectDesignSpecBoardRestored(page: Page): Promise<void> {
  await expect(page).toHaveURL(new RegExp(`${boardPath(SHOWCASE.DESIGN_SPEC)}$`));
  await expect(page.getByTestId("board")).toHaveAttribute(
    "data-board-mode",
    "authoring",
  );
  await expect(page.getByTestId("placard-problem")).toContainText(
    SHOWCASE.PROBLEM_SNIPPET,
  );
}

// Housekeeping: the pin was scaffolding for entering the editor, not a
// fact under test — drop it and confirm the wall is clean again, mirroring
// 37-board-diagram-editor.spec.ts's own reachability journey.
async function unpinScaffolding(page: Page): Promise<void> {
  await dragToTrash(page, page.locator(`.refcard[data-ref="${ref}"]`));
  await expectAutosaved(page);
  await expect(page.locator(`.refcard[data-ref="${ref}"]`)).toHaveCount(0);
}

test.describe("tool view exit: the diagram designer's exit affordance and Escape (ac-1)", () => {
  test("the exit affordance, entered from a board's pinned reference card, returns to that exact board — fully rendered", async ({
    page,
  }) => {
    await enterEditorFromBoard(page);

    // Visible, in the page chrome, and distinct from the existing
    // index/artifact nav links (a different element, class, and location).
    const exit = page.getByTestId("diagram-exit");
    await expect(exit).toBeVisible();
    await expect(exit).toHaveAttribute("href", boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(exit).toContainText(SHOWCASE.DESIGN_SPEC);
    await expect(page.locator(".site-nav")).not.toContainText("back to board");

    await exit.click();
    await expectDesignSpecBoardRestored(page);

    await unpinScaffolding(page);
  });

  test("Escape, entered from a board's pinned reference card, returns to that exact board too — the same exit, a second path", async ({
    page,
  }) => {
    await enterEditorFromBoard(page);

    await page.keyboard.press("Escape");
    await expectDesignSpecBoardRestored(page);

    await unpinScaffolding(page);
  });

  // Controller adjudication ADJ-38 (2026-07-16), finding
  // escape-during-inline-rename-also-exits: an in-editor overlay owns Escape
  // while it is open (dc-1's framing — overlays close in place without
  // navigating away). One Escape used to cancel the inline rename AND exit
  // the tool view, because the page-level exit handler fired after the
  // rename input had already detached itself; the fix stops the rename's own
  // Escape from bubbling to the page. One Escape cancels the rename only; a
  // second Escape then exits.
  test("Escape during an active inline rename cancels only the rename and stays in the editor; a second Escape then exits", async ({
    page,
  }) => {
    await enterEditorFromBoard(page);

    // Open the inline rename over a node (an authoring gesture): select the
    // node, then Rename in its toolbox.
    await page
      .locator('#diagram-preview g.node[data-node-id="loansvc"]')
      .click();
    await page.getByTestId("rename-node-btn").click();
    const renameInput = page.getByTestId("rename-input");
    await expect(renameInput).toBeVisible();

    // First Escape: the rename's own handler cancels it and MUST NOT bubble
    // to the page-level exit. The input is gone; the editor is still here —
    // it did not navigate away to the board.
    await page.keyboard.press("Escape");
    await expect(renameInput).toHaveCount(0);
    await expect(page.getByTestId("diagram-editor")).toBeVisible();
    await expect(page.getByTestId("board")).toHaveCount(0);

    // Second Escape: no rename open now — the page-level exit fires and
    // returns to the originating board, fully rendered.
    await page.keyboard.press("Escape");
    await expectDesignSpecBoardRestored(page);

    await unpinScaffolding(page);
  });

  test("with no originating board known, the affordance and Escape both disclose that honestly and fall back to the index — never a broken link", async ({
    page,
  }) => {
    // A direct URL carries no board= parameter — the same state the corpus
    // page's own editor link leaves the editor in (dc-3's second case).
    await page.goto(diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL));
    await waitForEditorReady(page);

    const exit = page.getByTestId("diagram-exit");
    await expect(exit).toBeVisible();
    await expect(exit).toHaveAttribute("href", "/");
    await expect(exit).toContainText(/no originating board is known/i);

    await exit.click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page).toHaveTitle(/workbench/i);
    // The index itself renders fully, not a blank or broken landing.
    await expect(page.locator(".home-directory")).toBeVisible();

    // A fresh visit, this time leaving via Escape: the identical fallback.
    await page.goto(diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL));
    await waitForEditorReady(page);
    const exitAgain = page.getByTestId("diagram-exit");
    await expect(exitAgain).toContainText(/no originating board is known/i);

    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/\/$/);
    await expect(page).toHaveTitle(/workbench/i);
    await expect(page.locator(".home-directory")).toBeVisible();
  });
});
