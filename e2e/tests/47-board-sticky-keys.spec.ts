import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// Owner feature request (2026-07-19): in the sticky creation surface,
// pressing Enter creates/commits the sticky; Shift+Enter inserts a
// newline. This matches the surrounding idiom — leaving the draft
// commits it, Escape discards it, and nothing defaults silently: Enter
// with no chosen type shows the same pick-a-type hint the focus-out
// path shows, never a silent default and never silent loss.
test.describe("sticky editor keys: Enter commits, Shift+Enter breaks the line", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("Enter commits the typed sticky", async ({ page }) => {
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    await expect(draft).toBeVisible();
    await draft.getByRole("button", { name: "Comment" }).click();

    const editor = draft.getByRole("textbox", { name: "Sticky text" });
    const text = "enter pins this thought";
    await editor.fill(text);
    await editor.press("Enter");

    await expect(draft).toHaveCount(0);
    await expect(page.getByTestId("autosave-status")).toHaveText("saved");
    const sticky = page
      .locator('[data-testid^="sticky-"]')
      .filter({ hasText: text });
    await expect(sticky).toHaveCount(1);
    await expect(sticky).toHaveAttribute("data-annotation-type", "comment");

    // Durable across a fresh projection.
    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(1);
  });

  test("Shift+Enter writes a newline; Enter commits the multi-line sticky, which persists", async ({
    page,
  }) => {
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    await draft.getByRole("button", { name: "Question" }).click();

    const editor = draft.getByRole("textbox", { name: "Sticky text" });
    await editor.pressSequentially("first thought line");
    await editor.press("Shift+Enter");
    await editor.pressSequentially("second thought line");

    // Shift+Enter stayed in the draft and produced a real line break.
    await expect(draft).toBeVisible();
    expect(await editor.inputValue()).toBe(
      "first thought line\nsecond thought line",
    );

    await editor.press("Enter");
    await expect(draft).toHaveCount(0);
    await expect(page.getByTestId("autosave-status")).toHaveText("saved");

    const sticky = page
      .locator('[data-testid^="sticky-"]')
      .filter({ hasText: "first thought line" });
    await expect(sticky).toHaveCount(1);

    // The sticky READS as two lines (the stored newline renders as a
    // break, not a collapsed space) — and still does on a fresh load.
    const renderedLines = () =>
      sticky
        .locator(".sticky-body")
        .evaluate((el) => (el as HTMLElement).innerText.split("\n"));
    expect(await renderedLines()).toEqual([
      "first thought line",
      "second thought line",
    ]);
    await page.reload();
    expect(await renderedLines()).toEqual([
      "first thought line",
      "second thought line",
    ]);
  });

  test("Enter without a chosen type keeps the draft and shows the hint (no silent default)", async ({
    page,
  }) => {
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    const editor = draft.getByRole("textbox", { name: "Sticky text" });
    const text = "an untyped thought Enter must not save";
    await editor.fill(text);
    await editor.press("Enter");

    // The draft stays, with the same visible hint the focus-out path
    // shows; nothing was written.
    await expect(draft).toBeVisible();
    await expect(draft.getByTestId("sticky-type-hint")).toBeVisible();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(0);

    // Choosing a type completes the ritual: Enter now commits (the type
    // buttons refuse focus, so the editor keeps it across the click).
    await draft.getByRole("button", { name: "Decision needed" }).click();
    await editor.press("Enter");
    await expect(draft).toHaveCount(0);
    await expect(page.getByTestId("autosave-status")).toHaveText("saved");
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveAttribute("data-annotation-type", "decision-needed");
  });
});
