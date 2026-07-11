import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  AC_IDS,
  DECISION_PLAIN,
  ADR_REF,
  boardPath,
  refCardTestId,
} from "./fixtures";
import { drawYarn, edgeTypePicker } from "./helpers";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P6, exit
// criterion 3: "drawing yarn toward a supersedes target opens the
// context-sensitive picker restricted to legal edge types with a
// consequence label and requires an explicit confirmation step before the
// edge commits"; 05 §Workbench, element-taxonomy yarn row: "drawing yarn
// opens a context-sensitive type picker: only the edge types legal for the
// (source kind, target kind) pair, each with a one-line consequence label
// ... and a confirmation step on gate-bearing types (supersedes, exempts)
// — a menu misclick must not summon an org-wide supersession flow".
test.describe("V1-P6: context-sensitive edge-type picker", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  // decision → ADR is the pair 02 §Link taxonomy admits supersedes and
  // exempts for ("decisions objects may carry their own links: ... for
  // supersedes/exempts edges against ADRs or other decisions"); implements
  // (story → feature-AC) and resolves (spike → open-question) are illegal
  // here and must not be offered.
  test("picker offers only the legal types for a decision→ADR pair, each with a consequence label", async ({
    page,
  }) => {
    await drawYarn(
      page,
      DECISION_PLAIN,
      page.getByTestId(refCardTestId(ADR_REF)),
    );

    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();

    await expect(
      picker.getByRole("menuitem", { name: /^supersedes/ }),
    ).toBeVisible();
    await expect(
      picker.getByRole("menuitem", { name: /^exempts/ }),
    ).toBeVisible();
    // The scratch tier's untyped thread is always available between any
    // two elements (05 §Workbench, "The scratch tier").
    await expect(
      picker.getByRole("menuitem", { name: /relates \(scratch\)/ }),
    ).toBeVisible();

    // Illegal for this (source kind, target kind) pair — never offered.
    await expect(
      picker.getByRole("menuitem", { name: /^implements/ }),
    ).toHaveCount(0);
    await expect(
      picker.getByRole("menuitem", { name: /^resolves/ }),
    ).toHaveCount(0);

    // "each with a one-line consequence label (e.g. 'supersedes: amends
    // the ADR for everyone; requires quorum')" (05 §Workbench).
    await expect(picker.getByTestId("consequence-supersedes")).toBeVisible();
    await expect(picker.getByTestId("consequence-supersedes")).not.toBeEmpty();
    await expect(picker.getByTestId("consequence-exempts")).toBeVisible();
    await expect(picker.getByTestId("consequence-exempts")).not.toBeEmpty();

    await page.keyboard.press("Escape");
    await expect(picker).toBeHidden();
  });

  // The gate-bearing negative path: a supersedes pick without its explicit
  // confirmation commits nothing — no yarn, no document edit, nothing to
  // find after a reload.
  test("a cancelled supersedes confirmation commits nothing", async ({
    page,
  }) => {
    const supersedesYarn = page.locator(
      `[data-edge-type="supersedes"][data-from="${DECISION_PLAIN}"]`,
    );
    await expect(supersedesYarn).toHaveCount(0);

    await drawYarn(
      page,
      DECISION_PLAIN,
      page.getByTestId(refCardTestId(ADR_REF)),
    );
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /^supersedes/ }).click();

    // The explicit confirmation step, restating the consequence.
    const confirm = page.getByRole("alertdialog", {
      name: /confirm supersedes/i,
    });
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Cancel" }).click();
    await expect(confirm).toBeHidden();
    await expect(picker).toBeHidden();

    // Nothing committed: no spec-layer edge now...
    await expect(supersedesYarn).toHaveCount(0);
    // ...and none materializes from the document on a fresh projection.
    await page.reload();
    await expect(supersedesYarn).toHaveCount(0);
  });

  // Negative path for context sensitivity itself: no typed edge in the
  // closed five-value vocabulary (02 §Link taxonomy) is legal between two
  // acceptance criteria of the same spec — the picker must offer no typed
  // edge for that pair, only the scratch tier's untyped thread.
  test("an illegal (source,target) pair offers no typed edge", async ({
    page,
  }) => {
    await drawYarn(page, AC_IDS[0], page.getByTestId(`card-${AC_IDS[1]}`));

    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();

    await expect(
      picker.getByRole("menuitem", { name: /relates \(scratch\)/ }),
    ).toBeVisible();
    for (const edgeType of [
      /^implements/,
      /^resolves/,
      /^supersedes/,
      /^exempts/,
      /^depends-on/,
    ]) {
      await expect(
        picker.getByRole("menuitem", { name: edgeType }),
      ).toHaveCount(0);
    }

    await page.keyboard.press("Escape");
    await expect(picker).toBeHidden();
  });
});
