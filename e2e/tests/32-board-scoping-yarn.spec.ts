import { test, expect, type Locator, type Page } from "@playwright/test";
import { SHOWCASE, boardPath, stubCardTestId, coverageChipTestId } from "./fixtures";
import { expectAutosaved, dragToTrash, grabPoint } from "./helpers";

// The scoping yarn (owner directive, verbatim: "the yarn should be used
// to consistently represent the UI element. Story stubs are associated
// with acceptance criteria and spikes are associated with open
// questions") and stub-card drag (round 5.5 dc-6: a stub's position home
// is layout.json under "stub:<slug>"). This suite runs after 30/31 in
// the shared store, so SHOWCASE.DESIGN_SPEC already carries the stubs suite 30
// graduated: audit-decline-notice-log (covers ac-3) and the two probe
// spikes (both resolve oq-1).
//
// The scoping layer is a PROJECTION of the stubs block — not document
// links: no affordances, not gate material; the AC-side coverage
// receipts (ac-4) stay and complement the threads.

const STORY_STUB = "audit-decline-notice-log";
const SPIKE_STUBS = ["probe-legal-wording", "probe-policy-precedent"] as const;

const scopingChip = (page: Page, type: string, fromSlug: string, to: string) =>
  page.locator(
    `.yarn-chip--scoping[data-edge-type="${type}"]` +
      `[data-from="stub:${fromSlug}"][data-to="${to}"]`,
  );

const scopingThreads = (page: Page) =>
  page.locator("#board-canvas svg.yarn-svg path.yarn-thread--scoping");

const threadShapes = (page: Page) =>
  scopingThreads(page).evaluateAll((els) =>
    els.map((el) => el.getAttribute("d")).sort(),
  );

const position = (el: Locator) =>
  el.evaluate((node) => ({
    left: (node as HTMLElement).style.left,
    top: (node as HTMLElement).style.top,
  }));

test.describe("scoping yarn: stub attributions hang as basting threads", () => {
  test("the committed fixture's story stubs hang covers yarn to their acceptance criteria", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );

    // One covers thread per declared attribution — the fixture's stubs:
    // block verbatim (mandate-api→ac-1+ac-2, retry-policy→ac-2+ac-3).
    for (const [slug, ac] of [
      ["autopay-mandate-api", "ac-1"],
      ["autopay-mandate-api", "ac-2"],
      ["autopay-retry-policy", "ac-2"],
      ["autopay-retry-policy", "ac-3"],
    ] as const) {
      await expect(scopingChip(page, "covers", slug, ac)).toHaveCount(1);
    }
    await expect(page.locator(".yarn-chip--scoping")).toHaveCount(4);
    await expect(scopingThreads(page)).toHaveCount(4);

    // The stub card's chip list is retired — the yarn is the
    // representation now — while the AC-side coverage receipts stay
    // (ac-4: they complement the threads, they never duplicated them).
    await expect(page.locator(".stub-links, .stub-link-chip")).toHaveCount(0);
    await expect(page.getByTestId(coverageChipTestId("ac-2"))).toHaveText(
      "covered by 2 stubs",
    );

    // The yarn key names the planning thread present on this wall —
    // covers only: no spike stub, no scoping resolves row.
    const key = page.getByTestId("yarn-key");
    await expect(
      key.locator('li[data-layer="scoping"][data-edge-type="covers"]'),
    ).toBeVisible();
    await expect(
      key.locator('li[data-layer="scoping"][data-edge-type="covers"]'),
    ).toContainText("a planned story will deliver it");
    await expect(
      key.locator('li[data-layer="scoping"][data-edge-type="resolves"]'),
    ).toHaveCount(0);
  });

  test("scoping edges are projections: no graduate, delete, or retype affordance in any mode", async ({
    page,
  }) => {
    // The sealed record and the live wall alike: a scoping chip carries
    // no button — there is no spec edge behind it to edit (the Go render
    // test pins this per-mode; here the real DOM confirms it).
    for (const spec of [SHOWCASE.FEATURE_SPEC, SHOWCASE.DESIGN_SPEC]) {
      await page.goto(boardPath(spec));
      const chips = page.locator(".yarn-chip--scoping");
      expect(await chips.count()).toBeGreaterThan(0);
      await expect(page.locator(".yarn-chip--scoping button")).toHaveCount(0);
    }
  });

  test("the authoring wall's spike stubs hang resolves yarn, and the key lists both planning rows", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    await expect(scopingChip(page, "covers", STORY_STUB, SHOWCASE.AC_IDS[2])).toHaveCount(1);
    for (const slug of SPIKE_STUBS) {
      await expect(scopingChip(page, "resolves", slug, SHOWCASE.OQ_ID)).toHaveCount(1);
    }
    // The OQ's multi-spike smell badge stays — the yarn shows the ties,
    // the badge makes the count an observation (ac-5).
    await expect(page.getByTestId(`oq-claims-${SHOWCASE.OQ_ID}`)).toHaveText(
      "claimed by 2 spikes",
    );

    const key = page.getByTestId("yarn-key");
    await expect(
      key.locator('li[data-layer="scoping"][data-edge-type="covers"]'),
    ).toBeVisible();
    await expect(
      key.locator('li[data-layer="scoping"][data-edge-type="resolves"]'),
    ).toContainText("a planned spike will answer it");
  });

  test("a stub card drags like any card: threads re-anchor live, the drop persists reload-deterministically", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const stub = page.getByTestId(stubCardTestId(STORY_STUB));
    await expect(stub).toBeVisible();
    const before = await position(stub);
    const shapesBefore = await threadShapes(page);
    expect(shapesBefore.length).toBeGreaterThanOrEqual(3);

    // Drag toward open canvas, pausing mid-flight: the basting threads
    // must follow the paper DURING the drag, not only after the drop.
    const grip = await grabPoint(page, stub);
    await page.mouse.move(grip.x, grip.y);
    await page.mouse.down();
    await page.mouse.move(grip.x - 130, grip.y + 120, { steps: 8 });
    const shapesMidDrag = await threadShapes(page);
    expect(shapesMidDrag).not.toEqual(shapesBefore);
    await page.mouse.move(grip.x - 260, grip.y + 240, { steps: 8 });
    await page.mouse.up();
    await expectAutosaved(page);

    // Only the dragged card wrote; its threads still tie the same
    // endpoints from the new spot.
    const moved = page.getByTestId(stubCardTestId(STORY_STUB));
    const after = await position(moved);
    expect(after).not.toEqual(before);
    await expect(scopingChip(page, "covers", STORY_STUB, SHOWCASE.AC_IDS[2])).toHaveCount(1);
    const shapesAfter = await threadShapes(page);
    expect(shapesAfter).not.toEqual(shapesBefore);

    // A fresh projection reads the stored stub:<slug> coordinate back
    // verbatim and re-derives the identical threads.
    await page.reload();
    expect(await position(page.getByTestId(stubCardTestId(STORY_STUB)))).toEqual(
      after,
    );
    expect(await threadShapes(page)).toEqual(shapesAfter);
  });

  test("the trash refuses a declared stub in plain language — cancel writes nothing", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const stub = page.getByTestId(stubCardTestId(STORY_STUB));
    await expect(stub).toBeVisible();
    const before = await position(stub);

    await dragToTrash(page, stub);

    // The refusal speaks the picker's plain language: a declared stub is
    // spec content, removal from the wall is not built yet, and the way
    // out is named. No Confirm exists — this is a refusal, not a gate.
    const refusal = page.getByRole("alertdialog", { name: /this stub stays/i });
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText("declared stub");
    await expect(refusal).toContainText("stubs block");
    await expect(refusal).toContainText(/not built yet/);
    await expect(refusal).toContainText(/edit the spec document/);
    await expect(page.locator("#edge-confirm-ok")).toBeHidden();
    await refusal.getByRole("button", { name: "Cancel" }).click();
    await expect(refusal).toBeHidden();

    // The paper snapped home; nothing was written — the same wall, the
    // same stored position, the same yarn, across a fresh projection.
    expect(await position(page.getByTestId(stubCardTestId(STORY_STUB)))).toEqual(
      before,
    );
    await page.reload();
    expect(await position(page.getByTestId(stubCardTestId(STORY_STUB)))).toEqual(
      before,
    );
    await expect(scopingChip(page, "covers", STORY_STUB, SHOWCASE.AC_IDS[2])).toHaveCount(1);
  });

  test("a read-only wall refuses a stub drag with the sealed record's own words", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    const stub = page.getByTestId(stubCardTestId("autopay-mandate-api"));
    const before = await position(stub);

    const grip = await grabPoint(page, stub);
    await page.mouse.move(grip.x, grip.y);
    await page.mouse.down();
    await page.mouse.move(grip.x + 200, grip.y + 160, { steps: 6 });
    await page.mouse.up();

    const refusal = page.getByTestId("drag-refusal");
    await expect(refusal).toBeVisible();
    await expect(refusal).toHaveText(/frozen with the accepted spec/);
    expect(await position(stub)).toEqual(before);
  });
});
