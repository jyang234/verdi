import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";
import { expectAutosaved } from "./helpers";

// Click-to-expand (owner directive: "the output is truncated. Clicking on
// it should open a dialog that expands it."). Two affordances share the one
// read-only expand dialog (the existing board-dialog chrome: × / backdrop /
// Escape), and this file pins both:
//
//   1. OBJECT CARDS / STUB TITLES / STICKIES keep the CLAMP-TRIGGERED
//      affordance (unchanged): the element fades its last line, marks itself
//      with a quiet "⋯", and expands to its OWN text — but ONLY when the
//      text measurably overflows. A short one stays crisp and inert.
//
//   2. CASE-FILE PLACARDS (problem / outcome) are ALWAYS expandable — the
//      board-polish pass's fix for the old width-dependence bug. Each wears
//      a persistent, quiet dog-ear (`.placard-more`) independent of whether
//      its headline currently clamps, and a click reads the FULL case file:
//      the spec's rendered `## Problem`/`## Outcome` body prose
//      (`.placard-full`) when it carried one, else a fall-back to the
//      headline. The ONE exception is the degenerate placard — a short
//      headline with no body section — which has nothing more to show and
//      so gets no affordance at all. When the headline ALSO clamps, the
//      fade+⋯ still layers on top of the dog-ear.
//
// The fixtures (cmd/e2eharness/provisionv2.go): SHOWCASE.DESIGN_SPEC
// (refi-decline-flow) carries a deliberately long problem headline that
// overflows the three-line clamp with an EMPTY `## Problem` body (the
// no-body → headline-fallback path) and a short outcome headline with a
// RICHER-than-headline `## Outcome` body (the always-expandable + show-body
// path, and — since the outcome headline does not clamp at the wide e2e
// viewport — the width-independence proof); SHOWCASE.REVIEW_SPEC
// (stale-decline-notices) carries a long problem headline, proving the
// affordance works in a non-authoring room; SHOWCASE.EMPTY_SPEC (income-verification)
// keeps a one-line problem headline with no body section — the degenerate
// negative case.

test.describe("board expand: truncated text opens a read-only dialog", () => {
  test("a clamped placard shows the hint and expands to its full text", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const placard = page.getByTestId("placard-problem");
    await expect(placard).toBeVisible();
    // The HEADLINE paragraph, targeted unambiguously by its own class — the
    // seam's hidden `.placard-full` body would otherwise make a bare `p`
    // match two elements.
    const p = placard.locator(".placard-text");

    // The hint is present because the headline is measurably clamped: it
    // wears .is-clamped, advertises itself as expandable, changes the
    // cursor, and grows a "⋯" mark. The always-present dog-ear sits beside
    // it (this placard is expandable regardless — see the width-independence
    // test below).
    await expect(p).toHaveClass(/is-clamped/);
    await expect(p).toHaveAttribute("data-expandable", "");
    await expect(p).toHaveCSS("cursor", "zoom-in");
    await expect(placard.locator(".clamp-more")).toHaveCount(1);
    await expect(placard.locator(".placard-more")).toHaveCount(1);

    // The full headline is in the DOM already (the clamp only hides it);
    // capture it to compare against what the dialog reads back. This spec's
    // `## Problem` body section is empty, so the placard carries no
    // `.placard-full` and the dialog falls back to the headline.
    await expect(placard.getByTestId("placard-full-problem")).toHaveCount(0);
    const full = (await p.textContent())!.trim();
    expect(full.length).toBeGreaterThan(200); // genuinely long

    // A click opens the read-only expand dialog: header names the element
    // ("PROBLEM"), body is the FULL headline text, in the existing chrome.
    await p.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog).toHaveClass(/board-dialog/); // the shared chrome
    await expect(dialog.locator(".expand-kind")).toHaveText("PROBLEM");
    await expect(page.getByTestId("expand-text")).toHaveText(full);

    // Read-only: opening it writes NOTHING (no autosave fired).
    await expect(page.getByTestId("autosave-status")).toHaveText("");

    // Closes on Escape, and it is reload-proof — nothing was persisted.
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
    await page.reload();
    await expect(
      page.getByTestId("placard-problem").locator(".placard-text"),
    ).toHaveText(full);
    await expect(
      page.getByTestId("placard-problem").locator(".placard-text"),
    ).toHaveClass(/is-clamped/);
  });

  test("the expand dialog closes from × and the backdrop too", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const p = page.getByTestId("placard-problem").locator(".placard-text");

    // × closes.
    await p.click();
    let dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(dialog).toHaveCount(0);

    // The backdrop closes (a soft-scrim click, the same exit every board
    // dialog offers).
    await p.click();
    dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    // Click the scrim at a corner clear of the centered dialog.
    await page.locator("#expand-backdrop").click({ position: { x: 8, y: 8 } });
    await expect(dialog).toHaveCount(0);
  });

  test("a short placard with no body section gets no affordance", async ({
    page,
  }) => {
    // SHOWCASE.EMPTY_SPEC's one-line problem headline fits AND its `## Problem` body
    // section is empty — the one degenerate case: nothing more to show than
    // the three lines on its face, so the always-on dog-ear is suppressed.
    // No clamp, no body, no affordance, and a click does nothing.
    await page.goto(boardPath(SHOWCASE.EMPTY_SPEC));
    const placard = page.getByTestId("placard-problem");
    const p = placard.locator(".placard-text");
    await expect(p).toBeVisible();
    await expect(p).not.toHaveClass(/is-clamped/);
    await expect(p).not.toHaveAttribute("data-expandable", "");
    await expect(p).toHaveCSS("cursor", "auto");
    await expect(placard.locator(".clamp-more")).toHaveCount(0);
    // The degenerate signature: no body section, not marked expandable, no
    // dog-ear.
    await expect(placard.getByTestId("placard-full-problem")).toHaveCount(0);
    await expect(placard).not.toHaveClass(/placard--expandable/);
    await expect(placard.locator(".placard-more")).toHaveCount(0);

    await p.click();
    // A generous beat past the expand delay: the dialog never appears.
    await page.waitForTimeout(400);
    await expect(page.getByTestId("expand-dialog")).toHaveCount(0);
  });

  test("the affordance works in a non-authoring (review) room too", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.REVIEW_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "review",
    );
    const placard = page.getByTestId("placard-problem");
    const p = placard.locator(".placard-text");
    await expect(p).toHaveClass(/is-clamped/);
    // The always-on dog-ear is present in the review mirror, exactly as on
    // the live wall — legibility does not depend on which room you stand in.
    await expect(placard.locator(".placard-more")).toHaveCount(1);
    // This spec carries no `## Problem` body, so the dialog falls back to the
    // headline.
    await expect(placard.getByTestId("placard-full-problem")).toHaveCount(0);
    const full = (await p.textContent())!.trim();

    await p.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId("expand-text")).toHaveText(full);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  });

  test("a placard expands to its body prose even when its headline does not clamp (width-independent)", async ({
    page,
  }) => {
    // The width-dependence fix, pinned: a case-file placard is expandable
    // because it HAS a fuller file to open, not because the viewport happened
    // to clamp its headline. SHOWCASE.DESIGN_SPEC's OUTCOME headline is short — at the
    // config's wide viewport (1880px) it does NOT overflow — yet the placard
    // carries a rendered `## Outcome` body and stays expandable.
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const placard = page.getByTestId("placard-outcome");
    const p = placard.locator(".placard-text");
    await expect(p).toBeVisible();

    // Width-independence premise: the headline fits, so it is NOT clamped and
    // wears no truncation ⋯ …
    await expect(p).not.toHaveClass(/is-clamped/);
    await expect(placard.locator(".clamp-more")).toHaveCount(0);
    // … and yet the placard is expandable and wears its dog-ear.
    await expect(placard).toHaveClass(/placard--expandable/);
    await expect(placard.locator(".placard-more")).toHaveCount(1);
    await expect(placard).toHaveCSS("cursor", "zoom-in");

    // The server rendered the body section into a hidden `.placard-full`.
    const bodyEl = placard.getByTestId("placard-full-outcome");
    await expect(bodyEl).toHaveCount(1);
    await expect(bodyEl).toBeHidden();

    // A click reads the FULL case file: the dialog shows the body as rendered
    // HTML — a distinctive phrase that lives in the `## Outcome` body but NOT
    // in the short headline, plus real markup (a 3-item list, emphasis)
    // proving it is the rendered section, not the headline read back.
    await p.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog).toHaveClass(/board-dialog/);
    await expect(dialog.locator(".expand-kind")).toHaveText("OUTCOME");
    const body = page.getByTestId("expand-text");
    await expect(body).toHaveClass(/expand-text--rich/);
    await expect(body).toContainText("single source of decline truth");
    await expect(body.locator("li")).toHaveCount(3);
    await expect(body.locator("strong").first()).toBeVisible();
    // The distinctive phrase is genuinely body-only: the placard's own
    // headline never contained it.
    await expect(p).not.toContainText("single source of decline truth");

    // Read-only, and closes on the backdrop.
    await expect(page.getByTestId("autosave-status")).toHaveText("");
    await page.locator("#expand-backdrop").click({ position: { x: 8, y: 8 } });
    await expect(dialog).toHaveCount(0);
  });

  test("a placard with no body section falls back to its headline", async ({
    page,
  }) => {
    // SHOWCASE.DESIGN_SPEC's PROBLEM placard has an empty `## Problem` body: the seam
    // emits no `.placard-full`, so the dog-ear (present because the long
    // headline clamps) opens a plain-text dialog reading the headline back —
    // the documented no-body fallback, never an empty dialog.
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const placard = page.getByTestId("placard-problem");
    await expect(placard.getByTestId("placard-full-problem")).toHaveCount(0);
    await expect(placard).toHaveClass(/placard--expandable/);
    await expect(placard.locator(".placard-more")).toHaveCount(1);

    const headline = (
      await placard.locator(".placard-text").textContent()
    )!.trim();

    // Open via the dog-ear button itself — the keyboard-reachable affordance.
    await placard.locator(".placard-more").click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    const body = page.getByTestId("expand-text");
    // Plain fallback: NOT the rich body-prose variant, and it reads back the
    // full headline.
    await expect(body).not.toHaveClass(/expand-text--rich/);
    await expect(body).toHaveText(headline);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  });

  test("a card double-click still edits; a single click expands", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    // Double-click still opens the inline editor (the click-to-expand must
    // not eat the dblclick) — no expand dialog appears.
    const card = page.getByTestId(`card-${SHOWCASE.AC_IDS[0]}`);
    await card.dblclick();
    const editor = page.getByRole("textbox", { name: "Card text" });
    await expect(editor).toBeVisible();
    await expect(page.getByTestId("expand-dialog")).toHaveCount(0);
    // Leave the editor without changing anything (blur commits nothing).
    await editor.blur();

    // Make this card's text long enough to clamp, then a single click on
    // the text opens the expand dialog (not the editor).
    await card.dblclick();
    const editor2 = page.getByRole("textbox", { name: "Card text" });
    const longText =
      "a declined applicant sees the current decline reason within a minute of a data change, across web and mobile, and the reason shown matches the servicing system of record exactly";
    await editor2.fill(longText);
    await editor2.blur();
    await expectAutosaved(page);

    const cardText = card.locator(".card-text");
    await expect(cardText).toHaveClass(/is-clamped/);
    await cardText.click();
    const dialog = page.getByTestId("expand-dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId("expand-text")).toHaveText(longText);
    await expect(dialog.locator(".expand-kind")).toContainText(SHOWCASE.AC_IDS[0]);
    await page.keyboard.press("Escape");
    await expect(dialog).toHaveCount(0);
  });

  test("a drag is still a drag, not an expand", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const card = page.getByTestId(`card-${SHOWCASE.AC_IDS[0]}`);
    await card.scrollIntoViewIfNeeded();

    const before = await card.boundingBox();
    expect(before).not.toBeNull();

    // A real drag past the slop: press on the card text, move, release.
    const text = card.locator(".card-text");
    const tb = await text.boundingBox();
    expect(tb).not.toBeNull();
    await page.mouse.move(tb!.x + tb!.width / 2, tb!.y + tb!.height / 2);
    await page.mouse.down();
    await page.mouse.move(tb!.x + 160, tb!.y + 140, { steps: 12 });
    await page.mouse.up();
    await expectAutosaved(page);

    // The card moved (a drag), and the drag's click tail did NOT open the
    // expand dialog.
    await page.waitForTimeout(400);
    await expect(page.getByTestId("expand-dialog")).toHaveCount(0);
    const after = await card.boundingBox();
    expect(Math.abs(after!.x - before!.x) + Math.abs(after!.y - before!.y)).toBeGreaterThan(
      40,
    );
  });
});
