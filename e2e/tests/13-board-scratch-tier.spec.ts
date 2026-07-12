import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  AC_IDS,
  DECISION_PLAIN,
  ADR_REF,
  boardPath,
  refCardTestId,
} from "./fixtures";
import { addSticky, drawYarn, edgeTypePicker, expectAutosaved, uncommittedIndicator } from "./helpers";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P6, exit
// criterion 2: "an authoring-mode open-question sticky → graduation
// (becomes a declared object via an ordinary edit, or a relates thread
// graduates to a typed edge via the picker)"; 05 §Workbench, "The scratch
// tier": free-floating stickies and untyped relates threads are
// "mutable-zone, never entering the spec document"; graduation is an
// ordinary edit. 02 §Record schemas: `type: relates` "never enters the
// spec document; graduation to a real object edge ... is an ordinary spec
// edit, not an automatic promotion".
test.describe("V1-P6: scratch tier", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  // Each test creates its own sticky (11-board-git-affordance's
  // establish-your-own-state discipline), so every test here stays
  // meaningful run in isolation. Texts are unique per test: the harness
  // re-provisions the scratch store at startup, but within one run the
  // hasText filters must never collide.

  // Free-floating sticky: annotation layer only. The sharpest observable
  // of "never entering the spec document" is the git affordance itself —
  // a sticky is mutable-zone (data/mutable/, never committed), so creating
  // one must NOT dirty the spec's working tree.
  test("a free-floating open-question sticky is annotation-layer and survives reload", async ({
    page,
  }) => {
    const stickyText = "open question: what about partial refunds?";
    await expect(uncommittedIndicator(page)).toBeHidden();

    const sticky = await addSticky(page, stickyText);
    await expect(sticky).toHaveAttribute("data-annotation-type", "question");

    // Mutable zone, not the document: the spec working tree stays clean.
    await expect(uncommittedIndicator(page)).toBeHidden();

    // But it durably persisted (the annotation stream), not page state.
    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: stickyText }),
    ).toHaveCount(1);
  });

  // "Graduation is an ordinary edit: a sticky becomes a real object
  // (decision, constraint, AC, declared open question) ... or they die"
  // (05 §Workbench). Graduating IS a spec edit, so the indicator must rise.
  test("a sticky graduates to a declared open-question object", async ({
    page,
  }) => {
    const stickyText = "open question: do declined co-borrowers get notices?";
    const sticky = await addSticky(page, stickyText);

    // The working-tree-honesty pair, first half: the sticky alone is
    // mutable-zone — creating it left the spec's working tree clean.
    await expect(uncommittedIndicator(page)).toBeHidden();

    await sticky.getByRole("button", { name: "Graduate" }).click();
    await page.getByRole("menuitem", { name: "Open question" }).click();
    await expectAutosaved(page);

    // The sticky is gone; a declared object card carries its text now.
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: stickyText }),
    ).toHaveCount(0);
    const graduated = page
      .locator('[data-testid^="card-"][data-object-kind="open-question"]')
      .filter({ hasText: "co-borrowers" });
    await expect(graduated).toHaveCount(1);

    // The honesty pair, second half: becoming a declared object edited
    // the spec document — the working tree is dirty where the sticky
    // alone left it clean.
    await expect(uncommittedIndicator(page)).toBeVisible();

    // Declared means durable: the object re-projects from the document.
    await page.reload();
    await expect(
      page
        .locator('[data-testid^="card-"][data-object-kind="open-question"]')
        .filter({ hasText: "co-borrowers" }),
    ).toHaveCount(1);
  });

  // An untyped relates thread between two elements stays annotation-layer
  // — never in the document — until graduated to a typed edge.
  test("an untyped relates thread stays annotation-layer across reloads", async ({
    page,
  }) => {
    await drawYarn(page, AC_IDS[0], page.getByTestId(`card-${AC_IDS[1]}`));
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /relates \(scratch\)/ }).click();
    await expectAutosaved(page);

    const relatesYarn = page.locator(
      `[data-edge-type="relates"][data-from="${AC_IDS[0]}"][data-to="${AC_IDS[1]}"]`,
    );
    await expect(relatesYarn).toHaveCount(1);
    await expect(relatesYarn).toHaveAttribute("data-layer", "annotation");

    // Persisted in the annotation stream, still annotation-layer after a
    // fresh projection — it never entered the spec document.
    await page.reload();
    await expect(relatesYarn).toHaveCount(1);
    await expect(relatesYarn).toHaveAttribute("data-layer", "annotation");
  });

  // The other graduation arm: "a relates-thread becomes a typed edge"
  // via the picker (05 §Workbench; PLAN-V1 V1-P6 exit criterion 2) — with
  // the gate-bearing confirmation honoured on the way (exempts).
  test("a relates thread graduates to a typed edge via the picker", async ({
    page,
  }) => {
    // Scratch first: an untyped thread decision→ADR.
    await drawYarn(
      page,
      DECISION_PLAIN,
      page.getByTestId(refCardTestId(ADR_REF)),
    );
    const picker = edgeTypePicker(page);
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /relates \(scratch\)/ }).click();
    await expectAutosaved(page);

    const scratchThread = page.locator(
      `[data-edge-type="relates"][data-from="${DECISION_PLAIN}"]`,
    );
    await expect(scratchThread).toHaveCount(1);
    await expect(scratchThread).toHaveAttribute("data-layer", "annotation");

    // Graduate it: the same context-sensitive picker, now over the
    // existing thread's (source, target) pair.
    await scratchThread.getByRole("button", { name: "Graduate" }).click();
    await expect(picker).toBeVisible();
    await picker.getByRole("menuitem", { name: /^exempts/ }).click();

    // exempts is gate-bearing: explicit confirmation required.
    const confirm = page.getByRole("alertdialog", { name: /confirm exempts/i });
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Confirm" }).click();
    await expectAutosaved(page);

    // The scratch thread is gone; a typed spec-layer edge replaced it.
    await expect(scratchThread).toHaveCount(0);
    const typedYarn = page.locator(
      `[data-edge-type="exempts"][data-from="${DECISION_PLAIN}"]`,
    );
    await expect(typedYarn).toHaveCount(1);
    await expect(typedYarn).toHaveAttribute("data-layer", "spec");

    // In the document now: a fresh projection re-renders it from the spec.
    await page.reload();
    await expect(typedYarn).toHaveCount(1);
    await expect(typedYarn).toHaveAttribute("data-layer", "spec");
  });
});
