import { test, expect, type Page } from "@playwright/test";
import {
  FEATURE_SPEC,
  STUB_SLUGS,
  INSTANTIATE_SLUG,
  OQ_ID,
  boardPath,
  stubCardTestId,
  coverageChipTestId,
} from "./fixtures";

// Instantiate-story-from-stub (spec/scoping-canvas ac-6) on the sealed
// wall: FEATURE_SPEC is escrow-autopay on main (the harness
// store is a real git repository), so its board is READ-ONLY — and the
// one live affordance a sealed record permits is Instantiate. The action
// cuts design/<slug> via no-checkout plumbing: the serving checkout
// never moves, which these journeys assert from the browser's side.

const confirmDialog = (page: Page) => page.locator("#edge-confirm");
const confirmOk = (page: Page) => page.locator("#edge-confirm-ok");
const confirmCancel = (page: Page) => page.locator("#edge-confirm-cancel");

test.describe("scoping canvas: the sealed wall instantiates its stubs", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
  });

  test("the committed corpus fixture renders its declared stubs as scoping cards", async ({
    page,
  }) => {
    // The fixture's stubs: frontmatter, verbatim as first-class cards
    // (ac-3) — this is one of the two committed corpus fixtures with
    // stubs, so the cards come for free and are asserted for free.
    for (const slug of STUB_SLUGS) {
      const stub = page.getByTestId(stubCardTestId(slug));
      await expect(stub).toBeVisible();
      await expect(stub.locator(".stub-tab")).toHaveText(slug);
      await expect(stub.locator(".card-kind-label")).toHaveText("story stub");
    }
    // Coverage is computed from the same frontmatter (ac-4, co-2):
    // ac-1 is claimed by the api and ui stubs, ac-2 by ui, ac-3 by the
    // audit log.
    await expect(page.getByTestId(coverageChipTestId("ac-1"))).toHaveText(
      "covered by 2 stubs",
    );
    await expect(page.getByTestId(coverageChipTestId("ac-2"))).toHaveText(
      "covered by 1 stub",
    );
    await expect(page.getByTestId(coverageChipTestId("ac-3"))).toHaveText(
      "covered by 1 stub",
    );
    // No spike stub claims oq-1 — no smell, and no badge at all.
    await expect(page.getByTestId(`oq-claims-${OQ_ID}`)).toHaveCount(0);
  });

  test("instantiate: consequence first, then the branch — and the serving wall never moves", async ({
    page,
  }) => {
    const button = page.getByTestId(`instantiate-${INSTANTIATE_SLUG}`);
    await expect(button).toBeVisible();

    // Consequence-labeled before firing: the dialog names the branch
    // and says the serving checkout stays put.
    await button.click();
    await expect(confirmDialog(page)).toBeVisible();
    await expect(confirmDialog(page)).toContainText(
      `design/${INSTANTIATE_SLUG}`,
    );
    await expect(confirmDialog(page)).toContainText(/serving checkout never moves/i);

    // A first cancel leaves everything standing.
    await confirmCancel(page).click();
    await expect(confirmDialog(page)).toBeHidden();

    // Fire it. The receipt names the branch and the tracker-ref
    // placeholder the operator must fill.
    await button.click();
    await confirmOk(page).click();
    await expect(confirmDialog(page)).toContainText("Story instantiated");
    await expect(confirmDialog(page)).toContainText(
      `design/${INSTANTIATE_SLUG}`,
    );
    await expect(confirmDialog(page)).toContainText("todo:REPLACE-ME");
    await confirmCancel(page).click();

    // The serving wall did not move: still the sealed record, stubs and
    // all — and the scaffolded story spec is NOT in the working tree
    // (its branch was never checked out), so its board does not exist.
    await page.reload();
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    await expect(
      page.getByTestId(stubCardTestId(INSTANTIATE_SLUG)),
    ).toBeVisible();
    const resp = await page.request.get(boardPath(INSTANTIATE_SLUG));
    expect(resp.status()).toBe(404);
  });

  test("a second instantiate is refused plainly: the branch already exists", async ({
    page,
  }) => {
    await page.getByTestId(`instantiate-${INSTANTIATE_SLUG}`).click();
    await confirmOk(page).click();
    await expect(confirmDialog(page)).toContainText("Could not instantiate");
    await expect(confirmDialog(page)).toContainText(
      `design/${INSTANTIATE_SLUG} already exists`,
    );
    await expect(confirmOk(page)).toBeHidden();
    await confirmCancel(page).click();
    await expect(confirmDialog(page)).toBeHidden();
  });
});
