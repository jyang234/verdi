import { test, expect } from "@playwright/test";
import { DESIGN_SPEC, boardPath } from "./fixtures";
import { addSticky } from "./helpers";

// Owner UAT (round 6, item 2): "it starts a purple question sticky…
// It should either have a default blank state or you should be able to
// pick before entering the content." The sticky draft now starts
// neutral and offers the four creatable annotation types as an inline
// segmented control — choosing is part of creating, no second modal,
// nothing defaults silently. (Amends the R4-I-31 question-by-default
// invention and the binding addSticky gesture in helpers.ts.)
test.describe("board sticky creation: the author picks the type", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("the draft offers the four creatable types and honors the choice", async ({
    page,
  }) => {
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    await expect(draft).toBeVisible();

    // All four creatable types, none preselected — a blank state until
    // the author chooses.
    for (const label of ["Comment", "Question", "Decision needed", "Agent task"]) {
      await expect(draft.getByRole("button", { name: label })).toBeVisible();
    }
    await expect(draft.locator('[aria-pressed="true"]')).toHaveCount(0);

    const text = "flag the copy team about decline wording";
    await draft.getByRole("textbox", { name: "Sticky text" }).fill(text);
    await draft.getByRole("button", { name: "Comment" }).click();
    await expect(draft.getByRole("button", { name: "Comment" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    // Commit by leaving the draft (the binding gesture: focus out of the
    // draft as a whole — refocus the editor first since the type click
    // moved focus to the type button).
    await draft.getByRole("textbox", { name: "Sticky text" }).focus();
    await draft.getByRole("textbox", { name: "Sticky text" }).blur();
    await expect(page.getByTestId("autosave-status")).toHaveText("saved");

    const sticky = page
      .locator('[data-testid^="sticky-"]')
      .filter({ hasText: text });
    await expect(sticky).toHaveCount(1);
    await expect(sticky).toHaveAttribute("data-annotation-type", "comment");

    // Durable, and still the chosen type after a fresh projection.
    await page.reload();
    await expect(
      page
        .locator('[data-testid^="sticky-"]')
        .filter({ hasText: text }),
    ).toHaveAttribute("data-annotation-type", "comment");
  });

  test("text without a chosen type never saves; Escape discards the draft", async ({
    page,
  }) => {
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    const editor = draft.getByRole("textbox", { name: "Sticky text" });
    const text = "an untyped thought that must not autosave";
    await editor.fill(text);

    // Leaving the draft without a type keeps it (with a visible hint)
    // instead of silently defaulting or silently discarding the text.
    await editor.blur();
    await expect(draft).toBeVisible();
    await expect(draft.getByTestId("sticky-type-hint")).toBeVisible();
    await expect(draft.getByTestId("sticky-type-hint")).toHaveText(/type/i);

    // Escape is the explicit way out: the draft dies, nothing was saved.
    await editor.focus();
    await page.keyboard.press("Escape");
    await expect(draft).toHaveCount(0);

    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(0);
  });

  test("the amended addSticky gesture carries any creatable type", async ({
    page,
  }) => {
    const sticky = await addSticky(
      page,
      "agent: sweep the decline templates",
      "agent-task",
    );
    await expect(sticky).toHaveAttribute("data-annotation-type", "agent-task");
  });
});
