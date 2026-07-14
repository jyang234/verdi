import { test, expect, type Page } from "@playwright/test";
import { addSticky } from "./helpers";
import {
  BADGE_WALL_SPEC,
  BADGE_REVIEW_SPEC,
  BADGE_SEALED_SPEC,
  BADGE_DECISION,
  BADGE_STUB_SLUG,
  boardPath,
  stubCardTestId,
} from "./fixtures";

// spec/badge-computes ac-5 (the ac-5--behavioral obligation): badges render
// as chips on their cards and stamps on the case file in EVERY board mode —
// authoring, review, and read-only alike — and never block: no board write
// path refuses an action because a badge is present (co-2).
//
// The fixture walls carry REAL badge-triggering state (lint findings the
// store actually computes at render time — never a canned badge):
//   - stub "badge-orphan" names undeclared ac-99  → VL-006 chip on the stub
//     card (dc-3's object-anchored bucket, keyed stub:<slug>);
//   - decision dc-1 exempts a nonexistent ADR     → VL-003 chip on dc-1;
//   - a top-level depends-on to a nonexistent spec → VL-003 stamp on the
//     case-file lockup (dc-3's spec-level bucket).
//
// Every badge element is a BUTTON carrying data-badge-source and its
// serialized derivation record (dc-4 — the derivation-drawer story's opener
// contract; the drawer itself is NOT this story).

// assertBadgedWall asserts the full badge surface on one wall: the two
// object-anchored chips inside their own cards, the case-file stamp beside
// the class tag, and dc-4's button/attribute contract on each.
async function assertBadgedWall(page: Page): Promise<void> {
  // The stub card wears its VL-006 chip (a dangling stub ref anchors to
  // the STUB's own card, never the case file).
  const stubCard = page.getByTestId(stubCardTestId(BADGE_STUB_SLUG));
  await expect(stubCard).toBeVisible();
  const stubChip = stubCard.locator('.badge-chip[data-badge-source="lint:VL-006"]');
  await expect(stubChip).toBeVisible();

  // The decision card wears its VL-003 chip (a decision's own dangling
  // link badges exactly that card).
  const decisionCard = page.getByTestId(`card-${BADGE_DECISION}`);
  await expect(decisionCard).toBeVisible();
  const decisionChip = decisionCard.locator('.badge-chip[data-badge-source="lint:VL-003"]');
  await expect(decisionChip).toBeVisible();

  // The case file wears its spec-level stamp, beside the class tag on the
  // same lockup (dc-4's second form).
  const stampRow = page.getByTestId("case-file-badges");
  await expect(stampRow).toBeVisible();
  const stamp = stampRow.locator('.case-stamp[data-badge-source="lint:VL-003"]');
  await expect(stamp).toBeVisible();
  await expect(stampRow.getByTestId("case-class-tag")).toBeVisible();

  // dc-4's opener contract, on every badge element: a real BUTTON whose
  // data-badge-record is the full serialized derivation record — source
  // matching data-badge-source, pinned inputs each carrying a content-
  // digest revision (dc-5, never wall-clock), and the firing records.
  for (const badge of [stubChip, decisionChip, stamp]) {
    expect(await badge.evaluate((el) => el.tagName)).toBe("BUTTON");
    const raw = await badge.getAttribute("data-badge-record");
    expect(raw).toBeTruthy();
    const record = JSON.parse(raw!);
    expect(record.source).toBe(await badge.getAttribute("data-badge-source"));
    expect(record.inputs.length).toBeGreaterThan(0);
    for (const input of record.inputs) {
      expect(input.revision).toMatch(/^sha256:[0-9a-f]{64}$/);
    }
    expect(record.records.length).toBeGreaterThan(0);
  }
}

test.describe("wall badges render in every board mode and never block", () => {
  test("authoring: chips + stamp render, and a write path succeeds on the badged wall", async ({
    page,
  }) => {
    await page.goto(boardPath(BADGE_WALL_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
    await assertBadgedWall(page);

    // Never blocking (co-2, the obligation's write-path proof): on this
    // badged authoring wall a real write path — add a sticky — succeeds
    // unchanged (addSticky itself asserts the autosave receipt). No write
    // handler consults badge state.
    await addSticky(page, "badges are receipts, not gates");

    // And the wall still wears its badges after the post-mutation fragment
    // swap (the fragment shares the one renderer with the page).
    await assertBadgedWall(page);
  });

  test("review: the badged wall renders chips + stamp in the MR mirror", async ({
    page,
  }) => {
    await page.goto(boardPath(BADGE_REVIEW_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "review",
    );
    await assertBadgedWall(page);
  });

  test("read-only: the sealed badged wall renders chips + stamp, and badge presence disables nothing", async ({
    page,
  }) => {
    await page.goto(boardPath(BADGE_SEALED_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    await assertBadgedWall(page);

    // The sealed accepted-pending-build wall's one live affordance
    // (Instantiate, spec/scoping-canvas ac-6) sits on the SAME stub card
    // that wears the VL-006 chip — and stays enabled: a badge never
    // disables an action (co-2's disclosure-not-refusal, on the exact
    // card where receipt and affordance meet).
    const instantiate = page.getByTestId(`instantiate-${BADGE_STUB_SLUG}`);
    await expect(instantiate).toBeVisible();
    await expect(instantiate).toBeEnabled();
  });
});
