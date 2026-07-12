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

// Open the supply toolbox (board-polish, owner directive): the "Pin an
// artifact" tab at the screen's lower-left opens the corpus picker tray
// in one click. Returns the tray.
export async function openPinToolbox(page: Page): Promise<Locator> {
  await page.getByRole("button", { name: "Pin an artifact" }).click();
  const tray = page.getByRole("dialog", { name: "Pin an artifact" });
  await expect(tray).toBeVisible();
  return tray;
}

// Pin an artifact to the wall through the toolbox picker (02 §Record
// schemas: type pin; 05 §The scratch tier: pinned references). Contract:
// each picker row is a .pin-result button carrying data-ref; choosing
// one pins the artifact and the same-paper reference card appears with
// data-pin-id. Returns the pinned card.
export async function pinArtifact(
  page: Page,
  ref: string,
  searchTerm?: string,
): Promise<Locator> {
  const tray = await openPinToolbox(page);
  if (searchTerm) {
    await tray.getByRole("searchbox", { name: "Search artifacts" }).fill(searchTerm);
  }
  const result = tray.locator(`.pin-result[data-ref="${ref}"]`);
  await expect(result).toBeVisible();
  await result.click();
  await expectAutosaved(page);
  const card = page.locator(`.refcard[data-ref="${ref}"]`);
  await expect(card).toBeVisible();
  await expect(card).toHaveAttribute("data-pin-id", /a-/);
  return card;
}

// Find a grip on a wall element that a real hand could take: a point
// that elementFromPoint resolves INSIDE the element and not on any
// button (a yarn chip can park over a sticky's center — chips avoid
// cards, not stickies — and a chip's own center is its Graduate button).
export async function grabPoint(
  page: Page,
  el: Locator,
): Promise<{ x: number; y: number }> {
  // The element can sit past the canvas's visible edge (the canvas
  // scrolls); the grip must land on the element itself.
  await el.scrollIntoViewIfNeeded();
  const p = await el.evaluate((node) => {
    const r = node.getBoundingClientRect();
    const cands: Array<[number, number]> = [
      [0.5, 0.5], [0.25, 0.2], [0.75, 0.8], [0.2, 0.8],
      [0.8, 0.2], [0.5, 0.15], [0.15, 0.5], [0.85, 0.5],
    ];
    for (const [fx, fy] of cands) {
      const x = r.left + r.width * fx;
      const y = r.top + r.height * fy;
      const hit = document.elementFromPoint(x, y);
      if (hit && node.contains(hit) && !hit.closest("button, textarea, input")) {
        return { x, y };
      }
    }
    return null;
  });
  expect(p, "no clear grip on the element (fully covered?)").not.toBeNull();
  return p!;
}

// Drag a wall element onto the trash target (owner directive): nearing
// the viewport's lower-right raises the trash (is-armed), hovering it
// goes hot (is-hot), releasing drops the element on it. The mouse is
// released with the pointer over the trash; what happens next is the
// caller's tier to assert.
export async function dragToTrash(page: Page, el: Locator): Promise<void> {
  const grip = await grabPoint(page, el);
  await page.mouse.move(grip.x, grip.y);
  await page.mouse.down();

  const vp = page.viewportSize();
  expect(vp).not.toBeNull();
  // Approach the corner: the trash rises when the pointer nears it.
  await page.mouse.move(vp!.width - 180, vp!.height - 180, { steps: 12 });
  const trash = page.getByTestId("board-trash");
  await expect(trash).toHaveClass(/is-armed/);

  // Over the bin it goes unmistakably hot; release drops there.
  const tbox = await trash.boundingBox();
  expect(tbox, "trash target has no layout box").not.toBeNull();
  await page.mouse.move(tbox!.x + tbox!.width / 2, tbox!.y + tbox!.height / 2, {
    steps: 6,
  });
  await expect(trash).toHaveClass(/is-hot/);
  await page.mouse.up();
}

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
