import { test, expect } from "@playwright/test";
import { EDGE, boardPath, branchBoardPath, refCardTestId } from "./fixtures";

// ADJ-70 (Phase-5 review, closing judged-fbl-r4-5): on a per-branch board
// every family href stays inside the branch it was resolved from. Before
// this fix both family directions emitted root-relative hrefs that ejected
// the operator to the serving checkout — and 404ed outright for a
// branch-only target, the exact journey driven here: FL_PAIR_BRANCH
// carries a feature and its implementing story that exist NOWHERE else.

const pairStoryBoard = branchBoardPath(EDGE.FL_PAIR_BRANCH, EDGE.FL_PAIR_STORY);
const pairFeatureBoard = branchBoardPath(EDGE.FL_PAIR_BRANCH, EDGE.FL_PAIR_FEATURE);

test.describe("branch family links (ADJ-70)", () => {
  test("a branch-only story board's parent-feature affordance stays inside the branch — the pre-fix 404 journey now round-trips", async ({
    page,
  }) => {
    // The family is branch-only: the unprefixed addresses have nothing to
    // serve (this is what made the old root-relative href a hard 404).
    const unprefixedStory = await page.request.get(boardPath(EDGE.FL_PAIR_STORY));
    expect(unprefixedStory.status()).toBe(404);
    const unprefixedFeature = await page.request.get(boardPath(EDGE.FL_PAIR_FEATURE));
    expect(unprefixedFeature.status()).toBe(404);

    await page.goto(pairStoryBoard); // first open may pay the lazy worktree cut
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      EDGE.FL_PAIR_STORY,
    );

    const card = page.getByTestId(
      refCardTestId(`spec/${EDGE.FL_PAIR_FEATURE}#ac-1`),
    );
    await expect(card).toBeVisible();
    const link = card.getByTestId("refcard-board-link");
    await expect(link).toHaveAttribute("href", pairFeatureBoard);
    await expect(link).toHaveAttribute("data-archived", "false");

    // Following it lands on the FEATURE's board on the SAME branch —
    // rendered fully, not a 404, not the serving checkout.
    await link.click();
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      EDGE.FL_PAIR_FEATURE,
    );
    expect(page.url()).toContain(pairFeatureBoard);
  });

  test("the feature board's stub-story link points back into the branch and round-trips", async ({
    page,
  }) => {
    await page.goto(pairFeatureBoard);
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      EDGE.FL_PAIR_FEATURE,
    );

    const storySlug = `spec-${EDGE.FL_PAIR_STORY}`;
    const link = page.getByTestId(
      `stub-story-link-${EDGE.FL_PAIR_STORY}-${storySlug}`,
    );
    await expect(link).toHaveAttribute("href", pairStoryBoard);
    await link.click();
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      EDGE.FL_PAIR_STORY,
    );
    expect(page.url()).toContain(pairStoryBoard);
  });

  test("the inverse case on a branch board discloses, never ejects: an implements target absent from the branch tree renders the notice, no href", async ({
    page,
  }) => {
    // design/family-links-instantiated-child was cut from main, so its tree
    // does NOT carry FL_PARENT (which lives only on the serving branch) —
    // the branch-rooted index cannot resolve the story's implements target.
    await page.goto(
      branchBoardPath(
        `design/${EDGE.FL_INSTANTIATED_CHILD}`,
        EDGE.FL_INSTANTIATED_CHILD,
      ),
    );
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      EDGE.FL_INSTANTIATED_CHILD,
    );
    const card = page.getByTestId(refCardTestId(`spec/${EDGE.FL_PARENT}#ac-2`));
    await expect(card).toBeVisible();
    await expect(card.getByTestId("refcard-unresolved-notice")).toContainText(
      "does not resolve in this checkout's store",
    );
    await expect(card.getByTestId("refcard-board-link")).toHaveCount(0);
  });
});
