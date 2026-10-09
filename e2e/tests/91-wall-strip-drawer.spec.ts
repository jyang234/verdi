import { test, expect, type Page, type Locator, type Request } from "@playwright/test";
import { SHOWCASE, EDGE, boardPath, dexSpecPath } from "./fixtures";
import { addSticky } from "./helpers";

// spec/wall-strip-and-drawer-v2 — the case-file strip (ac-1) and its chips
// (ac-2). The two tests here are the producers their obligations name
// (.verdi/obligations/wall-strip-and-drawer-v2/ac-1--behavioral.md and
// ac-2--behavioral.md), titled as the claims spell them; the file passes
// when run alone (BL-98). Lane F3b adds the branch menu, the pill, the
// drawer and the rail's homes (ac-4 to ac-6) to this file.
//
// State assertions ride roles, test ids, attributes and request bodies —
// never screenshots (recording stays off).

// The typed mutation transport's one endpoint: every in-place edit is one
// mutate_draft envelope through it (boardspecasd.js's mutate).
const MUTATE = "/api/mutate_draft";

function isMutate(r: Request): boolean {
  return r.method() === "POST" && r.url().includes(MUTATE);
}

// operationsOf reads the typed operations an envelope carries.
function operationsOf(r: Request): { op: string; text: string; anchor: string }[] {
  const body = r.postDataJSON() as { request: { operations: { op: string; text: string; anchor: string }[] } };
  return body.request.operations;
}

// expectAppliedClean waits for the transport's clean result after a write.
async function expectAppliedClean(page: Page): Promise<void> {
  await expect(page.getByTestId("asd-last-result")).toHaveAttribute("data-result-kind", "clean", { timeout: 10_000 });
}

// editInPlace clicks a half's text, replaces the statement and applies it
// with Enter; it returns the one mutate_draft request the Enter sent.
async function editInPlace(page: Page, which: "problem" | "outcome", text: string): Promise<Request> {
  const half = page.getByTestId(`placard-${which}`);
  await half.locator(".placard-text").click();
  const area = page.getByTestId(`case-strip-text-${which}`);
  await expect(area).toBeFocused();
  await area.fill(text);
  const sent = page.waitForRequest(isMutate);
  await area.press("Enter");
  const request = await sent;
  await expectAppliedClean(page);
  return request;
}

// chipOf locates one badge chip in the strip's chips by its source.
function chipOf(page: Page, source: string): Locator {
  return page.getByTestId("case-file-badges").locator(`.case-stamp[data-badge-source="${source}"]`);
}

// expectBadgeChip: ac-2's kept properties on one chip — a button carrying
// data-badge-source and its serialized derivation record, whose activation
// opens the derivation drawer, read back and closed clean.
async function expectBadgeChip(page: Page, chip: Locator, source: string): Promise<void> {
  await expect(chip).toBeVisible();
  expect(await chip.evaluate((el) => el.tagName)).toBe("BUTTON");
  const raw = await chip.getAttribute("data-badge-record");
  expect(raw).toBeTruthy();
  const record = JSON.parse(raw!);
  expect(record.source).toBe(source);
  expect(record.inputs.length).toBeGreaterThan(0);
  await chip.click();
  const drawer = page.locator(".badge-drawer:not([hidden])");
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveAttribute("role", "dialog");
  await expect(drawer.locator(".drawer-source")).toHaveText(source);
  await drawer.locator(".drawer-close").click();
  await expect(page.locator(".badge-drawer:not([hidden])")).toHaveCount(0);
  await expect(chip).toBeFocused();
}

test.describe("wall-strip-and-drawer", () => {
  test("The one-line case-file strip, edited in place", async ({ page }) => {
    test.setTimeout(120_000);
    const wall = boardPath(SHOWCASE.DESIGN_SPEC);
    await page.goto(wall);
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");

    // The strip: the problem and the outcome, one line each, in the one
    // strip above the wall, each half naming its typed operation.
    const strip = page.getByTestId("case-strip");
    await expect(strip).toBeVisible();
    const problem = strip.getByTestId("placard-problem");
    const outcome = strip.getByTestId("placard-outcome");
    await expect(problem).toContainText(SHOWCASE.PROBLEM_SNIPPET);
    await expect(outcome).toContainText(SHOWCASE.OUTCOME_SNIPPET);
    await expect(problem).toHaveAttribute("data-strip-op", "set-problem");
    await expect(outcome).toHaveAttribute("data-strip-op", "set-outcome");
    for (const half of [problem, outcome]) {
      const line = await half.locator(".placard-text").evaluate((el) => {
        const cs = getComputedStyle(el);
        return { lines: el.getClientRects().length > 0 ? Math.round(el.getBoundingClientRect().height / parseFloat(cs.lineHeight)) : 0 };
      });
      expect(line.lines, "one line each").toBe(1);
    }
    const originalProblem = (await problem.locator(".placard-text").textContent())!.trim();
    const originalOutcome = (await outcome.locator(".placard-text").textContent())!.trim();
    const problemAnchor = await problem.getAttribute("data-strip-anchor");
    const outcomeAnchor = await outcome.getAttribute("data-strip-anchor");

    try {
      // Escape cancels with nothing written: no envelope leaves the page,
      // the statement stands, and the editor is gone.
      const mutations: Request[] = [];
      page.on("request", (r) => {
        if (isMutate(r)) mutations.push(r);
      });
      await problem.locator(".placard-text").click();
      const editor = page.getByTestId("case-strip-editor-problem");
      await expect(editor).toBeVisible();
      await expect(editor).toHaveAttribute("data-holds-projection", "true");
      const area = page.getByTestId("case-strip-text-problem");
      await expect(area).toBeFocused();
      await expect(area).toHaveValue(originalProblem);
      await area.fill("a statement Escape must never write [91]");
      await area.press("Escape");
      await expect(editor).toHaveCount(0);
      await expect(problem.locator(".placard-text")).toBeFocused();
      await expect(problem.locator(".placard-text")).toHaveText(originalProblem);
      await page.waitForTimeout(500);
      expect(mutations, "Escape sent no operation").toEqual([]);
      await page.reload();
      await expect(page.getByTestId("placard-problem").locator(".placard-text")).toHaveText(originalProblem);

      // The open editor holds the swap (SI-368 (13): [data-holds-projection]
      // in interactionLive). An outside write that moves the revision — a
      // scratch sticky posted through the wall's API while the hand is in
      // the editor — lands nothing on the wall past two poll ticks: the
      // editor, its text and its focus survive, and the sticky is not
      // there. Escape closes the editor, and the held refresh lands.
      const holdDraft = "a statement the poll must never yank away [91-hold]";
      const holdSticky = "an outside write during an open edit [91-hold]";
      await page.getByTestId("placard-problem").locator(".placard-text").click();
      await expect(page.getByTestId("case-strip-editor-problem")).toBeVisible();
      await page.getByTestId("case-strip-text-problem").fill(holdDraft);
      const made = await page.request.post(wall + "/api/sticky", { data: { text: holdSticky, type: "comment" } });
      expect(made.status(), await made.text()).toBe(200);
      const held = page.locator('[data-testid^="sticky-"]').filter({ hasText: holdSticky });
      await page.waitForTimeout(4_500);
      await expect(page.getByTestId("case-strip-editor-problem"), "the editor survives the poll").toBeVisible();
      await expect(page.getByTestId("case-strip-text-problem")).toHaveValue(holdDraft);
      await expect(page.getByTestId("case-strip-text-problem")).toBeFocused();
      await expect(held, "the swap is held while the editor is open").toHaveCount(0);
      await page.getByTestId("case-strip-text-problem").press("Escape");
      await expect(page.getByTestId("case-strip-editor-problem")).toHaveCount(0);
      await expect(held, "the held refresh lands on Escape").toHaveCount(1, { timeout: 8_000 });
      await expect(page.getByTestId("placard-problem").locator(".placard-text")).toHaveText(originalProblem);
      expect(mutations, "the hold wrote no operation").toEqual([]);
      const heldID = (await held.getAttribute("data-id"))!;
      const unheld = await page.request.post(wall + "/api/annotation-delete", { data: { ids: [heldID] } });
      expect(unheld.status(), await unheld.text()).toBe(200);
      await expect(held).toHaveCount(0, { timeout: 8_000 });

      // Enter applies exactly one set-problem — the typed operation with
      // the server's own anchor — and the strip shows the new statement.
      const problemText = `${originalProblem} [91-problem]`;
      const problemRequest = await editInPlace(page, "problem", problemText);
      const problemOps = operationsOf(problemRequest);
      expect(problemOps).toHaveLength(1);
      expect(problemOps[0].op).toBe("set-problem");
      expect(problemOps[0].text).toBe(problemText);
      expect(problemOps[0].anchor).toBe(problemAnchor);
      await expect(page.getByTestId("placard-problem").locator(".placard-text")).toHaveText(problemText);
      await expect(page.getByTestId("case-strip-editor-problem")).toHaveCount(0);

      // And one set-outcome.
      const outcomeText = `${originalOutcome} [91-outcome]`;
      const outcomeRequest = await editInPlace(page, "outcome", outcomeText);
      const outcomeOps = operationsOf(outcomeRequest);
      expect(outcomeOps).toHaveLength(1);
      expect(outcomeOps[0].op).toBe("set-outcome");
      expect(outcomeOps[0].text).toBe(outcomeText);
      expect(outcomeOps[0].anchor).toBe(outcomeAnchor);
      await expect(page.getByTestId("placard-outcome").locator(".placard-text")).toHaveText(outcomeText);

      // Both edits reached the spec document: a reload reads them back.
      await page.reload();
      await expect(page.getByTestId("placard-problem").locator(".placard-text")).toHaveText(problemText);
      await expect(page.getByTestId("placard-outcome").locator(".placard-text")).toHaveText(outcomeText);

      // The full case file is one action away: the outcome's control
      // opens the existing expand dialog on the rendered body prose (the
      // fixture's richer `## Outcome` section), read-only.
      await page.getByTestId("placard-outcome").locator(".placard-more").click();
      const dialog = page.getByTestId("expand-dialog");
      await expect(dialog).toBeVisible();
      await expect(dialog.locator(".expand-kind")).toHaveText("OUTCOME");
      await expect(page.getByTestId("expand-text")).toContainText("single source of decline truth");
      await expect(page.getByTestId("autosave-status")).toHaveText("");
      await page.keyboard.press("Escape");
      await expect(dialog).toHaveCount(0);
      // The keyboard reaches the editor too: Enter on the focused headline.
      await page.getByTestId("placard-problem").locator(".placard-text").focus();
      await page.keyboard.press("Enter");
      await expect(page.getByTestId("case-strip-editor-problem")).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(page.getByTestId("case-strip-editor-problem")).toHaveCount(0);
    } finally {
      // The wall as it was: the shared store's statements restored through
      // the same one-operation path.
      await page.goto(wall);
      if ((await page.getByTestId("placard-problem").locator(".placard-text").textContent())!.trim() !== originalProblem) {
        await editInPlace(page, "problem", originalProblem);
      }
      if ((await page.getByTestId("placard-outcome").locator(".placard-text").textContent())!.trim() !== originalOutcome) {
        await editInPlace(page, "outcome", originalOutcome);
      }
      await page.reload();
      await expect(page.getByTestId("placard-problem").locator(".placard-text")).toHaveText(originalProblem);
      await expect(page.getByTestId("placard-outcome").locator(".placard-text")).toHaveText(originalOutcome);
    }

    // The badges, flags and disclosures as chips in the strip: a flag chip
    // (size-smell on a wall declaring criteria), a disclosure chip (the
    // pending-supersession the harness's forge-less serve cannot prove),
    // and the class tag beside them, all inside the strip.
    await page.goto(boardPath(EDGE.SIZE_SMELL_SPEC));
    const smell = page.getByTestId("case-strip").locator('.case-stamp[data-badge-source="observe:size-smell"]');
    await expect(smell).toBeVisible();
    await expect(smell).toHaveText("size-smell");
    await expect(page.getByTestId("case-strip").getByTestId("case-class-tag")).toBeVisible();
    await page.goto(boardPath(SHOWCASE.EMPTY_SPEC));
    const disclosure = page.getByTestId("case-strip").getByTestId("case-file-disclosure");
    await expect(disclosure).toBeVisible();
    await expect(disclosure).toHaveClass(/case-chip--disclosed/);
    await expect(disclosure).toContainText("disclosed-unproven");
  });

  test("The strip's chips keep the kept badge and flag properties", async ({ page }) => {
    test.setTimeout(120_000);
    // A feature wall in every board mode (the badge rigs: authoring,
    // review, sealed read-only): the spec-level chip is a button carrying
    // its source and record, and it opens the derivation drawer.
    for (const [spec, mode] of [
      [EDGE.BADGE_WALL_SPEC, "authoring"],
      [EDGE.BADGE_REVIEW_SPEC, "review"],
      [EDGE.BADGE_SEALED_SPEC, "readonly"],
    ] as const) {
      await page.goto(boardPath(spec));
      await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", mode);
      await expect(page.getByTestId("case-strip").getByTestId("case-file-badges")).toBeVisible();
      await expectBadgeChip(page, chipOf(page, "lint:VL-003"), "lint:VL-003");
    }

    // No write is refused because a badge is present: a scratch write on
    // the badged authoring wall succeeds, and the strip still wears the
    // chip after the swap.
    await page.goto(boardPath(EDGE.BADGE_WALL_SPEC));
    const sticky = await addSticky(page, "a chip never blocks a write [91]");
    await expect(chipOf(page, "lint:VL-003")).toBeVisible();
    const id = (await sticky.getAttribute("data-id"))!;
    const gone = await page.request.post(boardPath(EDGE.BADGE_WALL_SPEC) + "/api/annotation-delete", { data: { ids: [id] } });
    expect(gone.status(), await gone.text()).toBe(200);

    // A story wall wears its ladder flag, named as the dex story lens
    // names it: the real showcase story's spec-stale, on its read-only
    // wall, and on the dex page the same word.
    await page.goto(boardPath(SHOWCASE.STORY_WITH_SPEC_STALE));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    const stale = chipOf(page, "ladder:spec-stale");
    await expectBadgeChip(page, stale, "ladder:spec-stale");
    const flagName = (await stale.textContent())!.trim();
    expect(flagName).toBe("spec-stale");
    await page.goto(dexSpecPath(SHOWCASE.STORY_WITH_SPEC_STALE));
    await expect(page.getByTestId("badge-spec-stale")).toContainText(flagName);

    // Size-smell on a wall declaring acceptance criteria, as a chip in
    // the strip, named as its compute names it.
    await page.goto(boardPath(EDGE.SIZE_SMELL_SPEC));
    const smell = chipOf(page, "observe:size-smell");
    await expectBadgeChip(page, smell, "observe:size-smell");
    await expect(smell).toHaveText("size-smell");

    // A disclosed-unproven value is drawn with the disclosure class, not
    // the flag class, and no ladder flag dresses it as a verdict.
    await page.goto(boardPath(SHOWCASE.EMPTY_SPEC));
    const disclosure = page.getByTestId("case-strip").getByTestId("case-file-disclosure");
    await expect(disclosure).toBeVisible();
    await expect(disclosure).toHaveClass(/case-chip--disclosed/);
    await expect(disclosure).not.toHaveClass(/case-stamp/);
    await expect(disclosure).toContainText("[gate:pending-supersession]");
    await expect(page.locator('.case-stamp[data-badge-source="ladder:pending-supersession"]')).toHaveCount(0);
    expect(await disclosure.evaluate((el) => el.tagName)).not.toBe("BUTTON");
  });
});
