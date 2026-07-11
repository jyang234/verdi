import { test, expect } from "@playwright/test";
import {
  REVIEW_SPEC,
  REVIEW_COMMENT_ANCHORED,
  REVIEW_COMMENT_TOKEN_FREE,
  REVIEW_COMMENT_UNRESOLVABLE,
  REVIEW_FEED_TOTAL,
  boardPath,
} from "./fixtures";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P6 (Goal: "the
// review-mode mirror") and Phase V1-P7 (Delivers: "review-sticky forge
// round-trip ([vd:<object-id>] tokens, inbox tray)"; exit criteria:
// "list_comments returns both anchored (resolvable-token) and unanchored
// comments, and the unanchored set renders in the inbox tray — never
// dropped"); 05 §Review stickies and forge round-trip: "comments carrying
// a resolvable [vd:<object-id>] token render anchored to their object as
// a review sticky; comments that carry no resolvable token render in an
// inbox tray — never dropped, never silently unattached"; 02 §Record
// schemas, "Comment-token grammar".
//
// The harness serves the MR comment feed through internal/forge's fake
// adapter double (PLAN-V1 §5 V1-P6 "Stubs") — no network, per CLAUDE.md.
test.describe("V1-P6/V1-P7: review mode mirrors the MR", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(REVIEW_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "review",
    );
  });

  // The token routes the comment to its object's CURRENT card — position
  // is a display hint, the token is the key (S6 finding, binding).
  test("a token-bearing comment renders anchored to its object card", async ({
    page,
  }) => {
    const anchored = page.locator(
      `[data-annotation-type="review"][data-anchor="${REVIEW_COMMENT_ANCHORED.objectId}"]`,
    );
    await expect(anchored).toHaveCount(1);
    await expect(anchored).toContainText(REVIEW_COMMENT_ANCHORED.body);

    // Anchored means anchored: it does not also sit in the inbox tray.
    await expect(
      page
        .getByRole("region", { name: "Inbox tray" })
        .filter({ hasText: REVIEW_COMMENT_ANCHORED.body }),
    ).toHaveCount(0);
  });

  // "never dropped, never silently unattached, always visible to whoever
  // is triaging the review" — both the token-free comment AND the
  // unresolvable-token comment land in the tray, and the whole feed is
  // accounted for on the board.
  test("token-free and unresolvable-token comments render in the inbox tray — never dropped", async ({
    page,
  }) => {
    const tray = page.getByRole("region", { name: "Inbox tray" });
    await expect(tray).toBeVisible();

    await expect(
      tray.locator('[data-annotation-type="review"]').filter({
        hasText: REVIEW_COMMENT_TOKEN_FREE.body,
      }),
    ).toHaveCount(1);
    await expect(
      tray.locator('[data-annotation-type="review"]').filter({
        hasText: REVIEW_COMMENT_UNRESOLVABLE.body,
      }),
    ).toHaveCount(1);

    // Conservation: every comment in the feed is on the board, anchored
    // or trayed — the count proves none was dropped.
    await expect(
      page.locator('[data-annotation-type="review"]'),
    ).toHaveCount(REVIEW_FEED_TOTAL);
  });

  // "the board becomes a mirror of the MR rather than an editing surface"
  // (05 §Workbench, "Review" bullet): the authoring affordances are gone.
  test("review mode is a mirror, not an editing surface", async ({ page }) => {
    await expect(
      page.getByRole("button", { name: "Commit & push" }),
    ).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Add sticky" })).toHaveCount(
      0,
    );
  });
});
