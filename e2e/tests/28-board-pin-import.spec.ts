import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  READONLY_SPEC,
  REVIEW_SPEC,
  DECISION_PLAIN,
  PIN_ADR,
  PIN_DIAGRAM,
  boardPath,
  refCardTestId,
} from "./fixtures";
import {
  drawYarn,
  edgeTypePicker,
  expectAutosaved,
  grabPoint,
  openPinToolbox,
  pinArtifact,
} from "./helpers";

// The import/pin surface (02 §Record schemas round-5.2: type pin;
// 05 §The scratch tier: pinned references; owner directive: starting a
// spec fresh means bringing in ADRs, diagrams, etc. to plan freely,
// through a toolbox that is quiet at rest and one click away). The
// journey: import → pin → peek → relate → graduate.

test.describe("board: the supply toolbox pins planning material", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("the toolbox rests quiet; one click opens the picker; it closes without residue", async ({
    page,
  }) => {
    // At rest: the tab is present and the tray is closed — zero clutter.
    const tab = page.getByRole("button", { name: "Pin an artifact" });
    await expect(tab).toBeVisible();
    await expect(tab).toHaveAttribute("aria-expanded", "false");
    const tray = page.getByRole("dialog", { name: "Pin an artifact" });
    await expect(tray).toBeHidden();

    // One click: the picker, populated from the corpus, search focused.
    await tab.click();
    await expect(tray).toBeVisible();
    await expect(tab).toHaveAttribute("aria-expanded", "true");
    await expect(
      tray.getByRole("searchbox", { name: "Search artifacts" }),
    ).toBeFocused();
    await expect(tray.locator(".pin-result").first()).toBeVisible();
    // Rows show kind and title, not bare refs.
    const firstRow = tray.locator(".pin-result").first();
    await expect(firstRow.locator(".pin-result-kind")).not.toBeEmpty();
    await expect(firstRow.locator(".pin-result-title")).not.toBeEmpty();

    // Escape closes it without residue; so does an outside click.
    await page.keyboard.press("Escape");
    await expect(tray).toBeHidden();
    await expect(tab).toHaveAttribute("aria-expanded", "false");
    await tab.click();
    await expect(tray).toBeVisible();
    await page.getByTestId("board").click({ position: { x: 30, y: 30 } });
    await expect(tray).toBeHidden();
  });

  test("the picker never offers this board's own spec or refs already on the wall", async ({
    page,
  }) => {
    const tray = await openPinToolbox(page);
    await expect(tray.locator(".pin-result").first()).toBeVisible();
    await expect(
      tray.locator(`.pin-result[data-ref="spec/${DESIGN_SPEC}"]`),
    ).toHaveCount(0);
    // adr/0001-outbox-events has a card (the fixture's exempts edge — or
    // an earlier suite file's), so it is not offered again.
    await expect(
      page.getByTestId(refCardTestId("adr/0001-outbox-events")),
    ).toBeVisible();
    await expect(
      tray.locator(`.pin-result[data-ref="adr/0001-outbox-events"]`),
    ).toHaveCount(0);
    await page.keyboard.press("Escape");
  });

  test("import → pin → peek → relate → graduate", async ({ page }) => {
    // IMPORT: search narrows the picker; choosing pins the ADR.
    const card = await pinArtifact(page, PIN_ADR, "outbox");

    // PIN: the same reference-card paper, wearing the pin marking, one
    // card only, and it survives a fresh projection.
    await expect(card).toHaveClass(/refcard--pinned/);
    await expect(page.locator(`.refcard[data-ref="${PIN_ADR}"]`)).toHaveCount(1);
    await page.reload();
    await expect(page.locator(`.refcard[data-ref="${PIN_ADR}"]`)).toHaveCount(1);
    await expect(
      page.locator(`.refcard[data-ref="${PIN_ADR}"]`),
    ).toHaveAttribute("data-pin-id", /a-/);

    // PEEK: a pinned card peeks like any reference card.
    await page.locator(`.refcard[data-ref="${PIN_ADR}"]`).click();
    const peek = page.getByTestId("ref-peek");
    await expect(peek).toBeVisible();
    await expect(peek.getByTestId("ref-peek-open")).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(peek).toBeHidden();

    // DRAG: pins drag like stickies — the position survives reload.
    const pinned = page.locator(`.refcard[data-ref="${PIN_ADR}"]`);
    const grip = await grabPoint(page, pinned);
    await page.mouse.move(grip.x, grip.y);
    await page.mouse.down();
    await page.mouse.move(grip.x - 140, grip.y + 220, { steps: 10 });
    await page.mouse.up();
    await expectAutosaved(page);
    const moved = await pinned.evaluate((node) => ({
      left: (node as HTMLElement).style.left,
      top: (node as HTMLElement).style.top,
    }));
    await page.reload();
    expect(
      await page
        .locator(`.refcard[data-ref="${PIN_ADR}"]`)
        .evaluate((node) => ({
          left: (node as HTMLElement).style.left,
          top: (node as HTMLElement).style.top,
        })),
    ).toEqual(moved);

    // RELATE/GRADUATE: drawing a typed edge to the pinned target IS the
    // pin's graduation (02) — the card stays, the edge holds it now.
    await drawYarn(
      page,
      DECISION_PLAIN,
      page.locator(`.refcard[data-ref="${PIN_ADR}"]`),
    );
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /^exempts/ }).click();
    const confirm = page.getByRole("alertdialog", { name: /confirm exempts/i });
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);

    const typed = page.locator(
      `.yarn-chip[data-edge-type="exempts"][data-from="${DECISION_PLAIN}"][data-to="${PIN_ADR}"]`,
    );
    await expect(typed).toHaveCount(1);
    const survivor = page.locator(`.refcard[data-ref="${PIN_ADR}"]`);
    await expect(survivor).toHaveCount(1);
    // Graduated: the pin marking is gone; the record's edge projects it.
    await expect(survivor).not.toHaveAttribute("data-pin-id", /.+/);
    await page.reload();
    await expect(page.locator(`.refcard[data-ref="${PIN_ADR}"]`)).toHaveCount(1);
    await expect(
      page.locator(
        `.yarn-chip[data-edge-type="exempts"][data-from="${DECISION_PLAIN}"][data-to="${PIN_ADR}"]`,
      ),
    ).toHaveCount(1);
  });

  test("a diagram pins beside the ADR — planning material of every kind", async ({
    page,
  }) => {
    const card = await pinArtifact(page, PIN_DIAGRAM, "topology");
    await expect(card).toHaveAttribute("data-ref-kind", "diagram");
    await page.reload();
    await expect(
      page.locator(`.refcard[data-ref="${PIN_DIAGRAM}"]`),
    ).toHaveCount(1);
  });

  test("no toolbox exists outside authoring", async ({ page }) => {
    await page.goto(boardPath(READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    await expect(page.getByTestId("pin-toolbox")).toHaveCount(0);

    await page.goto(boardPath(REVIEW_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "review",
    );
    await expect(page.getByTestId("pin-toolbox")).toHaveCount(0);
  });
});
