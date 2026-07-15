import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// spec/feature-supersession-state ac-2 on the BOARD surface: a superseded
// spec's terminal `status` is legible on its own wall — a `superseded` status
// badge stamped on the board head beside the mode tag — at BOTH rungs (a
// superseded feature and a superseded story), so finding the terminal state
// never requires reading raw frontmatter or chasing a superseded-by backlink
// (03 §rung 3; co-3). The badge reuses the `.badge-superseded` vocabulary the
// index and dex surfaces already carry, so status reads the same everywhere a
// spec is rendered.

for (const { rung, spec } of [
  { rung: "feature", spec: SHOWCASE.SUPERSEDED_FEATURE_SPEC },
  { rung: "story", spec: SHOWCASE.SUPERSEDED_STORY_SPEC },
]) {
  test(`a superseded ${rung}'s board head wears the superseded status badge`, async ({
    page,
  }) => {
    await page.goto(boardPath(spec));

    // A superseded spec is not a draft, so its wall is the sealed record.
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );

    const badge = page.getByTestId("board-status-badge");
    await expect(badge).toBeVisible();
    await expect(badge).toHaveText("superseded");
    // The same status vocabulary the index list and dex page carry.
    await expect(badge).toHaveClass(/badge-superseded/);
  });
}

test("an accepted-pending-build board head wears no status badge", async ({
  page,
}) => {
  // Only the terminal `superseded` state is stamped (ac-2 scope; the mode tag
  // already speaks an accepted spec's read-only lifecycle, and `closed` is
  // deferred by dc-2).
  await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
  await expect(page.getByTestId("board")).toHaveAttribute(
    "data-board-mode",
    "readonly",
  );
  await expect(page.getByTestId("board-status-badge")).toHaveCount(0);
});
