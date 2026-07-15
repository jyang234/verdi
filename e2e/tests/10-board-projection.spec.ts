import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath, refCardTestId } from "./fixtures";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P6 (Goal:
// "internal/workbench becomes a pure projection renderer"); 05 §Workbench
// "Board as projection" and "Element taxonomy". The authoring-mode board
// must render the spec's parsed object model — attribute placards, one
// object card per frontmatter-declared object, typed yarn per declared
// edge — as a deterministic projection: nothing board-native except
// position.
test.describe("V1-P6: board renders the spec's object model (projection fidelity)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  // 05 §Workbench element taxonomy row 1: "attribute placards | the spec's
  // problem statement and outcome | the required problem:/outcome:
  // frontmatter attributes".
  test("attribute placards render the spec's problem and outcome", async ({
    page,
  }) => {
    await expect(page.getByTestId("placard-problem")).toContainText(
      SHOWCASE.PROBLEM_SNIPPET,
    );
    await expect(page.getByTestId("placard-outcome")).toContainText(
      SHOWCASE.OUTCOME_SNIPPET,
    );
  });

  // 05 §Workbench element taxonomy row 2: object cards are "frontmatter-
  // declared objects with body anchors ... a deterministic parse of
  // frontmatter plus resolved anchors, never inferred from prose".
  test("one object card per declared object, typed by kind", async ({
    page,
  }) => {
    for (const acId of SHOWCASE.AC_IDS) {
      const card = page.getByTestId(`card-${acId}`);
      await expect(card).toBeVisible();
      await expect(card).toHaveAttribute(
        "data-object-kind",
        "acceptance-criterion",
      );
    }

    const constraint = page.getByTestId(`card-${SHOWCASE.CONSTRAINT_ID}`);
    await expect(constraint).toBeVisible();
    await expect(constraint).toHaveAttribute("data-object-kind", "constraint");

    for (const dcId of [SHOWCASE.DECISION_WITH_EXEMPTS, SHOWCASE.DECISION_PLAIN]) {
      const decision = page.getByTestId(`card-${dcId}`);
      await expect(decision).toBeVisible();
      await expect(decision).toHaveAttribute("data-object-kind", "decision");
    }
  });

  // 05 §Workbench element taxonomy row 3: yarn is "the spec's typed edges
  // ... closed enum", stored in frontmatter links. The fixture decision's
  // declared exempts edge (PLAN-V1 §4: "one [decision] carrying a
  // links: [{type: exempts, ...}] edge against an ADR") must project as
  // spec-layer yarn, with its external target rendered as a reference card.
  test("declared edges project as typed spec-layer yarn with a visible target", async ({
    page,
  }) => {
    await expect(page.getByTestId(refCardTestId(SHOWCASE.ADR_REF))).toBeVisible();

    const exemptsYarn = page.locator(
      `[data-edge-type="exempts"][data-from="${SHOWCASE.DECISION_WITH_EXEMPTS}"]`,
    );
    await expect(exemptsYarn).toHaveCount(1);
    await expect(exemptsYarn).toHaveAttribute("data-layer", "spec");
    await expect(exemptsYarn).toHaveAttribute("data-to", SHOWCASE.ADR_REF);
  });

  // 05 §Workbench "Board as projection": "Generation is a pure function of
  // four inputs ... Same four inputs, same board — no LLM and no randomness
  // anywhere in generation." A reload with unchanged inputs must reproduce
  // the identical card set at identical positions.
  test("same inputs render the same board across reloads", async ({
    page,
  }) => {
    const cardPositions = async () => {
      const cards = page.locator('[data-testid^="card-"]');
      await expect(cards.first()).toBeVisible();
      return cards.evaluateAll((els) =>
        els
          .map((el) => ({
            id: el.getAttribute("data-testid"),
            left: (el as HTMLElement).style.left,
            top: (el as HTMLElement).style.top,
          }))
          .sort((a, b) => (a.id! < b.id! ? -1 : 1)),
      );
    };

    const first = await cardPositions();
    expect(first.length).toBeGreaterThan(0);

    await page.reload();
    const second = await cardPositions();

    expect(second).toEqual(first);
  });
});
