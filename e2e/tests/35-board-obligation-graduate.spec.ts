import { test, expect, type Page } from "@playwright/test";
import {
  OBLIGATION_STORY_SPEC,
  OBLIGATION_STORY_AC,
  OBLIGATION_STORY_NON_AC,
  boardPath,
} from "./fixtures";
import { addSticky, drawYarn, expectAutosaved } from "./helpers";

// Obligation authoring (spec/obligation-artifact ac-3): a sticky graduates
// into an evidence-obligation artifact by being dropped on a STORY
// acceptance criterion — the story wall's counterpart to the feature wall's
// scoping canvas (a proto-sticky graduating into a stub). The sticky's yarn
// dropped on an AC card opens the evidence-kind picker; choosing a kind
// seeds the obligation's for_kind and its verifies edge (→ the whole story
// spec) and writes the obligation file. A drop on anything that is not an
// AC is refused legibly, and nothing is written.
//
// The scratch store is ephemeral and unmounted to this test, so the
// obligation file itself is asserted at the Go level
// (internal/workbench/obligationauthor_test.go). Here we prove the BROWSER
// flow through board state: the sticky graduates and stays gone, and a
// second graduation onto the same AC/kind is refused because the file the
// first one wrote already exists — the board surfacing the record's
// existence.

// The for_kind picker the yarn drop opens (reuses the edge picker's dialog).
const forKindPicker = (page: Page) => page.locator("#edge-picker");
const pickerPair = (page: Page) => page.locator("#edge-picker-pair");
const notYetDialog = (page: Page) => page.locator("#edge-confirm");

async function newObligationSticky(page: Page, text: string): Promise<string> {
  const sticky = await addSticky(page, text, "comment");
  const id = await sticky.getAttribute("data-id");
  expect(id, "sticky has no data-id").not.toBeNull();
  return id!;
}

test.describe("obligation authoring: a sticky graduates on a story AC", () => {
  test("dropping a sticky's yarn on a story AC authors its evidence obligation", async ({
    page,
  }) => {
    await page.goto(boardPath(OBLIGATION_STORY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const stickyId = await newObligationSticky(
      page,
      "the stale-decline retry is proven end to end",
    );

    // The drop on the story AC: the sticky's yarn to the AC card opens the
    // evidence-kind picker — the one choice the (sticky, AC) pair leaves.
    await drawYarn(
      page,
      stickyId,
      page.getByTestId(`card-${OBLIGATION_STORY_AC}`),
    );
    await expect(forKindPicker(page)).toBeVisible();
    await expect(pickerPair(page)).toHaveText(
      `obligation → ${OBLIGATION_STORY_AC}`,
    );
    for (const kind of ["static", "behavioral", "runtime", "attestation"]) {
      await expect(
        forKindPicker(page).locator(`[data-forkind="${kind}"]`),
      ).toBeVisible();
    }

    // Choose behavioral: the sticky graduates into the obligation and its
    // handwriting is consumed — the sticky no longer renders.
    await forKindPicker(page).locator('[data-forkind="behavioral"]').click();
    await expectAutosaved(page);
    await expect(page.getByTestId(`sticky-${stickyId}`)).toHaveCount(0);

    // Declared means durable: the sticky stays graduated across a fresh
    // projection (a graduated record, not page state).
    await page.reload();
    await expect(page.getByTestId(`sticky-${stickyId}`)).toHaveCount(0);

    // Proof the FILE was written, surfaced through the board: a second
    // sticky dropped on the SAME AC with the SAME kind is refused because
    // the obligation the first drop authored already exists.
    const secondId = await newObligationSticky(page, "a duplicate obligation");
    await drawYarn(
      page,
      secondId,
      page.getByTestId(`card-${OBLIGATION_STORY_AC}`),
    );
    await forKindPicker(page).locator('[data-forkind="behavioral"]').click();

    await expect(notYetDialog(page)).toBeVisible();
    await expect(page.locator("#edge-confirm-title")).toHaveText(
      "Not yet an obligation",
    );
    await expect(page.locator("#edge-confirm-consequence")).toContainText(
      "already exists",
    );
    await expect(page.locator("#edge-confirm-ok")).toBeHidden();

    // The refused sticky survives, unauthored; tidy the wall for a clean
    // shared store.
    await page.keyboard.press("Escape");
    await expect(page.getByTestId(`sticky-${secondId}`)).toBeVisible();
    await page
      .getByTestId(`sticky-${secondId}`)
      .locator('[data-delete="sticky"]')
      .click();
    await expectAutosaved(page);
  });

  test("a sticky dropped on a non-AC target is refused legibly and nothing is authored", async ({
    page,
  }) => {
    await page.goto(boardPath(OBLIGATION_STORY_SPEC));
    const stickyId = await newObligationSticky(page, "misaimed obligation");

    // The drop lands on a decision card, not an acceptance criterion: the
    // picker never opens — a plain-language refusal names the wrong target.
    await drawYarn(
      page,
      stickyId,
      page.getByTestId(`card-${OBLIGATION_STORY_NON_AC}`),
    );
    const refusal = page.getByTestId("proto-yarn-refusal");
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText("story acceptance criterion");
    await expect(refusal).toContainText(OBLIGATION_STORY_NON_AC);

    // No evidence-kind picker, so nothing could be authored; the sticky is
    // still parked, its handwriting intact.
    await expect(
      forKindPicker(page).locator("[data-forkind]"),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");
    await expect(page.getByTestId(`sticky-${stickyId}`)).toBeVisible();

    // Tidy the wall (scratch dies without ceremony).
    await page
      .getByTestId(`sticky-${stickyId}`)
      .locator('[data-delete="sticky"]')
      .click();
    await expectAutosaved(page);
    await expect(page.getByTestId(`sticky-${stickyId}`)).toHaveCount(0);
  });
});
