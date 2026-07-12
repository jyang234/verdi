import { test, expect } from "@playwright/test";
import {
  DESIGN_SPEC,
  READONLY_SPEC,
  REVIEW_SPEC,
  EMPTY_SPEC,
  boardPath,
} from "./fixtures";
import { addSticky } from "./helpers";

// The legibility contract (owner UAT: the board must read, at a glance,
// like a murder board — "put all the facts and entities on the board to
// draw relationships and keep track of context", human-legibly). These
// are the behavioral halves of that redesign: labeled zone bands, the
// teaching empty wall, the collapsed four-move guide (05 §Workbench
// "The four-concept minimum path" — everything else discoverable, never
// front-loaded), the yarn key that names only the threads present, and
// mode identity readable from the page chrome.

const ZONE_KINDS = [
  "acceptance-criterion",
  "constraint",
  "decision",
  "open-question",
  "reference",
  "scratch",
] as const;

test.describe("board legibility: the wall reads at a glance", () => {
  test("authoring labels every zone band; empty bands read as invitations", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    for (const kind of ZONE_KINDS) {
      await expect(page.getByTestId(`zone-label-${kind}`)).toBeVisible();
    }
    // Empty bands render as dimmed invitations, occupied bands as plain
    // tape — self-consistent with whatever earlier suite writes left on
    // the shared wall (graduations mint cards; the exact empty-band rule
    // is pinned deterministically in the Go render tests).
    for (const kind of [
      "acceptance-criterion",
      "constraint",
      "decision",
      "open-question",
    ]) {
      const occupied = await page
        .locator(`.objcard[data-object-kind="${kind}"]`)
        .count();
      const cls = await page
        .getByTestId(`zone-label-${kind}`)
        .getAttribute("class");
      expect(
        cls!.includes("zone-label--empty"),
        `${kind}: ${occupied} cards, class ${cls}`,
      ).toBe(occupied === 0);
    }

    // The labels are teaching chrome, never an interaction layer: a
    // pointer aimed at a card or the canvas must pass straight through.
    for (const kind of ZONE_KINDS) {
      const pe = await page
        .getByTestId(`zone-label-${kind}`)
        .evaluate((el) => getComputedStyle(el).pointerEvents);
      expect(pe, `zone label ${kind} intercepts the pointer`).toBe("none");
    }
  });

  test("a sealed record labels only the zones it occupies", async ({
    page,
  }) => {
    await page.goto(boardPath(READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );

    // READONLY_SPEC declares ACs and one document-level edge to an ADR
    // (a reference card) — nothing else, so nothing else is labeled.
    await expect(
      page.getByTestId("zone-label-acceptance-criterion"),
    ).toBeVisible();
    await expect(page.getByTestId("zone-label-reference")).toBeVisible();
    await expect(page.getByTestId("zone-label-decision")).toHaveCount(0);
    await expect(page.getByTestId("zone-label-constraint")).toHaveCount(0);
    await expect(page.locator(".zone-label--empty")).toHaveCount(0);
  });

  test("the four-move guide: collapsed in authoring, absent from mirror and record", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    const guide = page.getByTestId("board-guide");
    await expect(guide).toBeVisible();
    // Never front-loaded: closed until asked.
    await expect(guide).not.toHaveAttribute("open", "");
    await guide.locator("summary").click();
    await expect(guide).toHaveAttribute("open", "");
    // The four concepts, in the guide's own words.
    await expect(guide).toContainText("case file");
    await expect(guide).toContainText("acceptance criteria");
    await expect(guide).toContainText("yarn");
    await expect(guide).toContainText("Commit & push");

    for (const spec of [REVIEW_SPEC, READONLY_SPEC]) {
      await page.goto(boardPath(spec));
      await expect(page.getByTestId("board-guide")).toHaveCount(0);
    }
  });

  test("the yarn key names exactly the threads on the wall", async ({
    page,
  }) => {
    // The key lists each distinct thread type on the wall exactly once,
    // and nothing more — self-consistent with whatever annotation
    // threads earlier suite writes left in the shared store (the
    // canonical order and present-types-only rule are pinned
    // deterministically in the Go render tests). Checked on both a
    // sealed record and the live wall.
    const keyMatchesChips = async () => {
      const chipTypes = await page
        .locator(".yarn-chip")
        .evaluateAll((els) =>
          Array.from(
            new Set(els.map((el) => el.getAttribute("data-edge-type"))),
          ).sort(),
        );
      const keyTypes = await page
        .locator('[data-testid="yarn-key"] li')
        .evaluateAll((els) =>
          els.map((el) => el.getAttribute("data-edge-type")).sort(),
        );
      expect(keyTypes).toEqual(chipTypes);
    };

    await page.goto(boardPath(READONLY_SPEC));
    const key = page.getByTestId("yarn-key");
    await expect(key).toBeVisible();
    // The sealed fixture's own document-level implements edge is always
    // on this wall, whatever else leaked in.
    await expect(key.locator('li[data-edge-type="implements"]')).toBeVisible();
    await keyMatchesChips();

    // The document's own chip says whose edge it is — the document is
    // not a card, so its thread runs off the top of the wall.
    await expect(
      page.locator('.yarn-chip--doc[data-edge-type="implements"]'),
    ).toContainText("this spec");

    await page.goto(boardPath(DESIGN_SPEC));
    await keyMatchesChips();
  });

  test("an empty wall invites instead of voiding", async ({ page }) => {
    await page.goto(boardPath(EMPTY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const empty = page.getByTestId("board-empty");
    await expect(empty).toBeVisible();
    await expect(empty).toContainText("Nothing pinned yet");
    await expect(empty).toContainText("Add sticky");

    // No object cards — the leanest valid story spec still hangs its
    // implements thread (a reference card), and the wall still invites.
    await expect(page.locator('[data-testid^="card-"]')).toHaveCount(0);
    await expect(page.locator(".yarn-chip--doc")).toHaveCount(1);
    for (const kind of ZONE_KINDS) {
      const label = page.getByTestId(`zone-label-${kind}`);
      await expect(label).toBeVisible();
      if (kind !== "reference") {
        await expect(label).toHaveClass(/zone-label--empty/);
      }
    }

    // The invitation works: the case file is up, and Add sticky is live.
    await expect(page.getByTestId("placard-problem")).toContainText(
      "verified by hand",
    );
    await expect(page.getByRole("button", { name: "Add sticky" })).toBeVisible();
  });

  test("a new sticky lands at the bottom of its type's lane", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    // The landing policy, restated executable (boardspecapi.go
    // stickyLanePosition): lane x comes from the zone label's band;
    // y appends below every element footprint intersecting the band
    // (cards 140 high, refcards 72, stickies estimated 150), gap 24,
    // first slot 40 when the lane is empty.
    const laneBottom = (laneX: number) =>
      page.locator("#board-canvas").evaluate((canvas, left) => {
        const right = left + 200;
        let bottom = -1;
        const consider = (sel: string, h: number, fixed: boolean) => {
          for (const el of canvas.querySelectorAll<HTMLElement>(sel)) {
            const x = parseFloat(el.style.left);
            const y = parseFloat(el.style.top);
            const height = fixed ? h : Math.max(h, el.offsetHeight);
            if (x < right && left < x + 200 && y + height > bottom) {
              bottom = y + height;
            }
          }
        };
        consider(".objcard", 140, true);
        consider(".refcard", 72, true);
        consider(".sticky:not(.sticky-draft)", 150, true);
        return bottom;
      }, laneX);

    const laneXOf = async (kind: string) =>
      parseFloat(
        await page
          .getByTestId(`zone-label-${kind}`)
          .evaluate((el) => (el as HTMLElement).style.left),
      );

    // A question files beneath the open-questions column.
    const oqX = await laneXOf("open-question");
    const oqBottom = await laneBottom(oqX);
    const q = await addSticky(page, "legibility: does the lane hold?", "question");
    await expect(q).toHaveCSS("left", `${oqX}px`);
    const qTop = parseFloat(await q.evaluate((el) => (el as HTMLElement).style.top));
    expect(qTop).toBe(oqBottom < 0 ? 40 : oqBottom + 24);

    // A comment files into the scratch lane, appended below whatever
    // already sits there.
    const scratchX = await laneXOf("scratch");
    const scratchBottom = await laneBottom(scratchX);
    const c = await addSticky(page, "legibility: noted for the wall", "comment");
    await expect(c).toHaveCSS("left", `${scratchX}px`);
    const cTop = parseFloat(await c.evaluate((el) => (el as HTMLElement).style.top));
    expect(cTop).toBe(scratchBottom < 0 ? 40 : scratchBottom + 24);

    // The scratch lane's label is no longer an empty invitation.
    await expect(page.getByTestId("zone-label-scratch")).not.toHaveClass(
      /zone-label--empty/,
    );
  });

  test("mode identity is page chrome: three rooms, three stamps", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.locator(".board-mode-tag")).toHaveText(
      /authoring · live wall/,
    );
    await expect(page.locator("body")).toHaveClass(/mode-authoring/);

    await page.goto(boardPath(REVIEW_SPEC));
    await expect(page.locator(".board-mode-tag")).toHaveText(
      /review · mirror of the MR/,
    );
    await expect(page.locator("body")).toHaveClass(/mode-review/);
    // The mirror explains itself in the rail.
    await expect(page.locator(".mirror-note")).toContainText(
      "mirrors the merge request",
    );

    await page.goto(boardPath(READONLY_SPEC));
    await expect(page.locator(".board-mode-tag")).toHaveText(
      /read-only · sealed record/,
    );
    await expect(page.locator("body")).toHaveClass(/mode-readonly/);
    await expect(page.locator(".sealed-panel")).toContainText("supersession");
  });
});
