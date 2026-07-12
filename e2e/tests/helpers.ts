// tests-v1/helpers.ts — shared interaction helpers for the v1 acceptance
// specs. Gestures defined here are part of the binding UI contract
// (tests-v1/README.md): the V1-P6 implementer must make these exact
// interactions work.

import { expect, type Locator, type Page } from "@playwright/test";

// Draw yarn from an object card's yarn handle to a target element
// (05 §Workbench, element taxonomy "yarn" row: "drawing yarn opens a
// context-sensitive type picker"). The gesture is a pointer drag from
// [data-testid="yarn-handle-<object-id>"] to the target's center.
export async function drawYarn(
  page: Page,
  fromObjectId: string,
  target: Locator,
): Promise<void> {
  const handle = page.getByTestId(`yarn-handle-${fromObjectId}`);
  await expect(handle).toBeVisible();
  await expect(target).toBeVisible();

  const from = await handle.boundingBox();
  const to = await target.boundingBox();
  expect(from, `yarn handle for ${fromObjectId} has no layout box`).not.toBeNull();
  expect(to, "yarn target has no layout box").not.toBeNull();

  await page.mouse.move(from!.x + from!.width / 2, from!.y + from!.height / 2);
  await page.mouse.down();
  await page.mouse.move(to!.x + to!.width / 2, to!.y + to!.height / 2, {
    steps: 12,
  });
  await page.mouse.up();
}

// The context-sensitive edge-type picker that drawing yarn opens
// (05 §Workbench: "only the edge types legal for the (source kind, target
// kind) pair, each with a one-line consequence label").
export function edgeTypePicker(page: Page): Locator {
  return page.getByRole("dialog", { name: "Edge type" });
}

// Open an object card's inline editor (authoring mode is bidirectional:
// "editing a card ... *is* editing the spec's objects" — 05 §Workbench).
// Contract: double-click opens a textbox labelled "Card text"; the edit
// commits on blur and autosaves to the working tree.
export async function editCard(
  page: Page,
  objectId: string,
  mutate: (current: string) => string,
): Promise<void> {
  await page.getByTestId(`card-${objectId}`).dblclick();
  const editor = page.getByRole("textbox", { name: "Card text" });
  await expect(editor).toBeVisible();
  const current = await editor.inputValue();
  await editor.fill(mutate(current));
  await editor.blur();
  await expectAutosaved(page);
}

// Autosave completion: [data-testid="autosave-status"] reads "saved"
// (same signal the v0 board exposed; carried forward as binding contract).
export async function expectAutosaved(page: Page): Promise<void> {
  await expect(page.getByTestId("autosave-status")).toHaveText("saved", {
    timeout: 5_000,
  });
}

// The board-owned git affordance's uncommitted-changes indicator
// (05 §Workbench, authoring-mode bullet: "a persistent uncommitted-changes
// indicator").
export function uncommittedIndicator(page: Page): Locator {
  return page.getByTestId("uncommitted-indicator");
}

// Create a free-floating scratch sticky (05 §Workbench, "The scratch
// tier": "free-floating stickies ... mutable-zone, never entering the spec
// document"). Contract (AMENDED, owner UAT round 6 item 2 — choosing the
// type is part of creating the sticky): "Add sticky" opens a draft with
// an inline type control (one button per creatable annotation type) and
// a "Sticky text" textbox; the author picks a type, writes the text, and
// the sticky commits when focus leaves the draft.
export type StickyType =
  | "comment"
  | "question"
  | "decision-needed"
  | "agent-task";

const stickyTypeLabels: Record<StickyType, string> = {
  comment: "Comment",
  question: "Question",
  "decision-needed": "Decision needed",
  "agent-task": "Agent task",
};

export async function addSticky(
  page: Page,
  text: string,
  type: StickyType = "question",
): Promise<Locator> {
  await page.getByRole("button", { name: "Add sticky" }).click();
  const draft = page.locator(".sticky-draft");
  await expect(draft).toBeVisible();
  await draft.getByRole("button", { name: stickyTypeLabels[type] }).click();
  const editor = draft.getByRole("textbox", { name: "Sticky text" });
  await editor.fill(text); // fill focuses the editor…
  await editor.blur(); // …so this blur leaves the draft and commits
  await expectAutosaved(page);
  const sticky = page
    .locator('[data-testid^="sticky-"]')
    .filter({ hasText: text });
  await expect(sticky).toHaveCount(1);
  return sticky.first();
}
