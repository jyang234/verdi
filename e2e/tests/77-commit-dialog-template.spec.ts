import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// The board's "Commit & push" dialog (spec/uat-round-1 ac-9, co-4; closes
// UAT-019: the proposal commit was typed as "accepted F13 spec", a
// lifecycle state no design-branch commit can make true — acceptance is
// the owner's merge). Two mechanical aids, neither of which changes what
// is committed or pushed:
//
//   1. every open prefills "Propose spec/<name>: " with the caret at the
//      end; the author appends a summary or replaces the whole line;
//   2. while the message contains a lifecycle word the commit cannot make
//      true (accepted/closed/merged/superseded and their verb forms, as
//      whole words, any case), a note beside the field says so. It never
//      blocks: the Commit button stays enabled; it leaves when the words do.
//
// The pattern the browser compiles is the server's own (data-pattern on
// the note), the same text the Go matcher's word-boundary table proves; this
// spec is the proof that text is a valid ECMAScript RegExp and drives the
// note. Nothing here commits: spec 11 already exercises the real commit
// path on this fixture and its own fill() replaces the prefill wholesale.
// State assertions ride roles, testids and values, never screenshots
// (recording stays off).
test.describe("commit dialog: proposal template and lifecycle note", () => {
  const template = `Propose spec/${SHOWCASE.DESIGN_SPEC}: `;

  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
  });

  test("prefills a proposal naming the spec, notes a lifecycle word without blocking, and forgets on reopen", async ({
    page,
  }) => {
    const openButton = page.getByRole("button", { name: "Commit & push" });
    const dialog = page.getByRole("dialog", { name: "Commit & push" });
    const message = dialog.getByRole("textbox", { name: "Commit message" });
    const note = dialog.getByTestId("commit-lifecycle-note");
    const commit = dialog.getByRole("button", { name: "Commit", exact: true });

    await openButton.click();
    await expect(dialog).toBeVisible();
    await expect(message).toHaveValue(template);
    await expect(message).toBeFocused();
    await expect(note).toBeHidden();
    await expect(commit).toBeEnabled();

    // The UAT-019 message, typed after the template: the note appears and
    // names the correction; the button is still live.
    await message.pressSequentially("accepted F13 spec");
    await expect(message).toHaveValue(template + "accepted F13 spec");
    await expect(note).toBeVisible();
    await expect(note).toContainText("owner");
    await expect(note).toContainText("proposes");
    await expect(commit).toBeEnabled();

    // A summary without the word silences it.
    await message.fill(template + "add the F13 summary");
    await expect(note).toBeHidden();

    // Whole words only: the near-misses stay silent ...
    await message.fill("unacceptable disclosure of the acceptance criteria");
    await expect(note).toBeHidden();
    // ... and the whole word fires in any case and position, even when the
    // author has replaced the template entirely.
    await message.fill("MERGE spec now");
    await expect(note).toBeVisible();
    await expect(commit).toBeEnabled();

    // Cancel commits nothing; reopening forgets the edit and starts from the
    // template again, note hidden.
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
    await openButton.click();
    await expect(dialog).toBeVisible();
    await expect(message).toHaveValue(template);
    await expect(note).toBeHidden();

    // Still escapable (owner UAT round 6: never a modal you can't get out of).
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("the empty message is still refused client-side exactly as before", async ({ page }) => {
    const dialog = page.getByRole("dialog", { name: "Commit & push" });
    const message = dialog.getByRole("textbox", { name: "Commit message" });

    await page.getByRole("button", { name: "Commit & push" }).click();
    await message.fill("   ");
    await dialog.getByRole("button", { name: "Commit", exact: true }).click();
    // Unchanged rule: a blank message keeps the dialog open with focus on
    // the field and sends nothing (the server's own rule is untouched).
    await expect(dialog).toBeVisible();
    await expect(message).toBeFocused();
    await expect(dialog.getByTestId("commit-lifecycle-note")).toBeHidden();
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });
});
