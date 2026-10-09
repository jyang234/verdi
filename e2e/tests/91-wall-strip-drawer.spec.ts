import { test, expect, type Page, type Locator, type Request } from "@playwright/test";
import { SHOWCASE, EDGE, CONTROL_URL, boardPath, branchBoardPath, dexSpecPath, stubCardTestId } from "./fixtures";
import { addSticky, clearWallSelection, editCard, selectStub, stickyAction, uncommittedIndicator, wallToolbar } from "./helpers";

// spec/wall-strip-and-drawer-v2 — the case-file strip (ac-1) and its chips
// (ac-2), the branch menu, the readiness pill and the ⋯ menu's on-demand
// counts (ac-4), and the record drawer's tabs (ac-5). Each test here is the
// producer its obligation names (.verdi/obligations/wall-strip-and-
// drawer-v2/ac-<n>--behavioral.md), titled as the claim spells it; the
// file passes when run alone (BL-98). The rail is gone, and its homes
// (ac-6) are proven by the last test.
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

// The F3-go2 harness walls (cmd/e2eharness/provision_wallstrip.go), each an
// authoring wall on its own namesake branch, copied here as 92 copies its
// own (fixtures.ts stays F7's) and pinned by TestWallStripPaths: a spec
// whose drawer projections are empty (no provenance record, no yarn, no
// pinned context), and one whose committed provenance record does not
// decode, so its Provenance and Review projections are unavailable.
const DRAWER_EMPTY_WALL = "/b/design%2Fdecline-drawer-empty/board/spec/decline-drawer-empty";
const DRAWER_UNAVAILABLE_WALL = "/b/design%2Fdecline-drawer-unavailable/board/spec/decline-drawer-unavailable";
// And the wall whose working tree holds unclassified changes only, whose
// spec no typed operation ever wrote, so its review packet carries an
// unclassified edit.
const CHANGES_UNCLASSIFIED_WALL = "/b/design%2Fdecline-changes-unclassified/board/spec/decline-changes-unclassified";

// branchOf is a /b/ wall's branch, decoded from its address.
function branchOf(wall: string): string {
  return decodeURIComponent(wall.split("/")[2]);
}

// The design reads the drawer and the ⋯ menu make on demand (co-1).
const DESIGN_READS = /\/api\/(get_design_provenance|prepare_design_review|get_design_context|get_design_capabilities)$/;

// readsOf records every on-demand design read and every Readiness tab
// load the page makes from now on.
function readsOf(page: Page): string[] {
  const reads: string[] = [];
  page.on("request", (r) => {
    const op = DESIGN_READS.exec(r.url());
    if (op && r.method() === "POST") reads.push(op[1]);
    if (r.method() === "GET" && /\/board\/spec\/[^/]+\/readiness$/.test(r.url())) reads.push("readiness");
  });
  return reads;
}

// pollsOf records the status of every conditional poll the wall's
// transport makes from now on (GET <wall>/snapshot, 304 when nothing
// changed).
function pollsOf(page: Page): number[] {
  const polls: number[] = [];
  page.on("response", (r) => {
    if (r.request().method() === "GET" && new URL(r.url()).pathname.endsWith("/snapshot")) polls.push(r.status());
  });
  return polls;
}

// revisionOf reads the revision the wall's transport holds.
function revisionOf(page: Page): Promise<string> {
  return page.evaluate(() => (window as unknown as { __verdiASD: { state: () => { revision: string } } }).__verdiASD.state().revision);
}

// expectProposal: the Review tab's last section names the branch and
// SI-368 (12)'s forge-agnostic command, and holds no control that opens
// anything (co-2).
async function expectProposal(panel: Locator, branch: string): Promise<void> {
  const command = panel.getByTestId("record-review-command");
  await expect(command).toContainText(branch);
  await expect(command).toContainText("Push the branch, then open a pull request on your forge:");
  await expect(command.locator("code")).toHaveText(`git push -u origin ${branch}`);
  await expect(command.locator("a, button, form")).toHaveCount(0);
}

// JSON_TEXT matches raw JSON printed as text: an object's opening brace
// before a key, or a quoted key and its colon.
const JSON_TEXT = /\{\s*"|"[\w-]+"\s*:/;

// openTab opens one drawer tab through its own opener: the pill for
// Readiness, the ⋯ menu for every other — or, with the drawer already
// open, its tab — and returns its panel once its body has settled.
async function openTab(page: Page, tab: string): Promise<Locator> {
  const drawer = page.getByTestId("record-drawer");
  if (await drawer.isHidden()) {
    if (tab === "readiness") await page.getByTestId("readiness-pill").click();
    else {
      await page.getByTestId("wall-more").click();
      await page.getByTestId(`wall-more-${tab}`).click();
    }
  } else {
    await page.getByTestId(`record-tab-${tab}`).click();
  }
  await expect(page.getByTestId(`record-tab-${tab}`)).toHaveAttribute("aria-selected", "true");
  const panel = page.getByTestId(`record-panel-${tab}`);
  await expect(panel).toBeVisible();
  await expect(panel.locator('[data-record-body][aria-busy="true"]')).toHaveCount(0, { timeout: 15_000 });
  return panel;
}

// expectProse: a tab renders the sections named, each a heading, and no
// raw JSON text anywhere in it.
async function expectProse(panel: Locator, sections: string[]): Promise<void> {
  for (const name of sections) {
    await expect(panel.getByRole("heading", { name, exact: true }), `section ${name}`).toBeVisible();
  }
  expect(await panel.innerText(), "a tab prints no raw JSON").not.toMatch(JSON_TEXT);
}

// postTypedEdit posts one typed edit-ac on the design wall from outside the
// page, against the wall's current base, so its provenance holds an entry.
async function postTypedEdit(page: Page, id: string, text: string, evidence: string[]): Promise<void> {
  const wall = boardPath(SHOWCASE.DESIGN_SPEC);
  const snap = await (await page.request.get(wall + "/snapshot")).json();
  const resp = await page.request.post(wall + "/api/mutate_draft", {
    data: {
      request: {
        schema: "verdi.draftmutation/v1",
        spec: "spec/" + SHOWCASE.DESIGN_SPEC,
        base_digest: snap.base_digest,
        base_spec_b64: snap.base_spec_b64,
        expected: snap.expected,
        operations: [{ op: "edit-ac", id, text, evidence, anchor: "#" + id }],
      },
    },
  });
  expect(resp.status(), await resp.text()).toBe(200);
  expect((await resp.json()).result, "the typed mutation landed").toBeTruthy();
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

  test("The branch menu, the readiness pill, and the menu's on-demand counts", async ({ page }) => {
    test.setTimeout(150_000);
    const wall = boardPath(SHOWCASE.DESIGN_SPEC);
    const ac = SHOWCASE.AC_IDS[1];
    const reads = readsOf(page);
    await page.goto(wall);
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");

    try {
      // The branch menu on the branch text in the top bar: the switcher
      // opens it, and over uncommitted changes the guard still refuses a
      // switch — the wall stays on its branch, the edit kept.
      await editCard(page, ac, (text) => `${text} [91-guard]`);
      await expect(uncommittedIndicator(page)).toBeVisible();
      const switcher = page.getByTestId("topbar").getByTestId("branch-switcher");
      await expect(switcher).toHaveText(SHOWCASE.DESIGN_BRANCH);
      await switcher.click();
      await expect(switcher).toHaveAttribute("aria-expanded", "true");
      const menu = page.locator("#branch-menu");
      await expect(menu).toBeVisible();
      await menu.getByRole("menuitem", { name: SHOWCASE.MAIN_BRANCH, exact: true }).click();
      const guard = page.getByRole("alertdialog", { name: "Uncommitted changes" });
      await expect(guard).toBeVisible();
      await guard.getByRole("button", { name: "Stay on branch" }).click();
      await expect(guard).toBeHidden();
      await expect(page.getByTestId("topbar").getByTestId("branch-switcher")).toHaveText(SHOWCASE.DESIGN_BRANCH);
      await expect(page.getByTestId(`card-${ac}`)).toContainText("[91-guard]");

      // Nothing the drawer reads is fetched before it is asked for: no
      // count, no projection, no readiness load.
      await page.waitForTimeout(1_000);
      expect(reads, "nothing is read before the pill or the menu opens").toEqual([]);

      // The readiness pill opens the drawer's Readiness tab — one readiness
      // load, when the tab opens — and its link stays the page's address
      // for a browser without JavaScript.
      const pill = page.getByTestId("readiness-pill");
      await expect(pill).toHaveAttribute("href", `/readiness?spec=${SHOWCASE.DESIGN_SPEC}`);
      await pill.click();
      await expect(page).toHaveURL(new RegExp(`${wall}$`));
      const drawer = page.getByTestId("record-drawer");
      await expect(drawer).toBeVisible();
      await expect(page.getByTestId("record-tab-readiness")).toHaveAttribute("aria-selected", "true");
      await expect(page.getByTestId("record-tab-readiness")).toBeFocused();
      await expect(page.getByTestId("record-panel-readiness").getByTestId("readiness-tab")).toBeVisible({ timeout: 15_000 });
      expect(reads.filter((r) => r === "readiness").length, "the pill loads the readiness").toBeGreaterThan(0);
      expect(reads.filter((r) => r !== "readiness"), "the pill reads nothing else").toEqual([]);
      // Shut, the focus goes back to the pill.
      await page.keyboard.press("Escape");
      await expect(drawer).toBeHidden();
      await expect(pill).toBeFocused();

      // The ⋯ menu opens the other tabs; its counts load when it opens and
      // only then — one read each for Provenance and Review, none for Repo
      // (the bar's own ahead fact) — and each item shows its count.
      reads.length = 0;
      await page.waitForTimeout(500);
      expect(reads, "no count is read before the menu opens").toEqual([]);
      const more = page.getByTestId("wall-more");
      await more.click();
      await expect(more).toHaveAttribute("aria-expanded", "true");
      const items = page.getByTestId("wall-more-menu");
      await expect(items).toBeVisible();
      const counts = {
        provenance: /^\d+ (entry|entries)$/,
        review: /^\d+ needs? a human eye$/,
        repo: /^\d+ ahead$/,
      };
      for (const [tab, words] of Object.entries(counts)) {
        const count = page.getByTestId(`wall-more-count-${tab}`);
        await expect(count, `${tab}'s count`).toHaveText(words, { timeout: 15_000 });
        await expect(count).not.toHaveAttribute("data-count-state", "unavailable");
      }
      expect([...reads].sort(), "the menu's counts cost one read each").toEqual(["get_design_provenance", "prepare_design_review"]);
      for (const [tab, label] of [
        ["provenance", "Provenance"],
        ["review", "Semantic review"],
        ["context", "Design context"],
        ["repo", "Repository details"],
        ["moves", "Four moves"],
        ["keys", "Keyboard"],
      ]) {
        await expect(items.getByTestId(`wall-more-${tab}`)).toContainText(label);
      }
      // The keyboard runs the menu: the first item has the focus, the
      // arrows move it, Escape shuts the menu and gives it back to ⋯.
      await expect(items.getByTestId("wall-more-provenance")).toBeFocused();
      await page.keyboard.press("ArrowDown");
      await expect(items.getByTestId("wall-more-review")).toBeFocused();
      await page.keyboard.press("Escape");
      await expect(items).toBeHidden();
      await expect(more).toBeFocused();
      await expect(more).toHaveAttribute("aria-expanded", "false");
      // An item opens its tab.
      await more.click();
      await items.getByTestId("wall-more-repo").click();
      await expect(page.getByTestId("record-tab-repo")).toHaveAttribute("aria-selected", "true");
      await page.getByTestId("record-drawer-close").click();
      await expect(drawer).toBeHidden();
    } finally {
      await page.goto(wall);
      const now = (await page.getByTestId(`card-${ac}`).textContent()) ?? "";
      if (now.includes("[91-guard]")) await editCard(page, ac, (text) => text.replace(" [91-guard]", ""));
    }

    // A count that cannot be read says "unavailable", never 0: the wall
    // whose provenance record does not decode.
    await page.goto(DRAWER_UNAVAILABLE_WALL);
    await page.getByTestId("wall-more").click();
    for (const tab of ["provenance", "review"]) {
      const count = page.getByTestId(`wall-more-count-${tab}`);
      await expect(count, `${tab}'s failed count`).toHaveText("unavailable", { timeout: 15_000 });
      await expect(count).toHaveAttribute("data-count-state", "unavailable");
      expect(await count.getAttribute("title"), `${tab}'s failure names its reason`).toMatch(/\S/);
    }

    // The Review count is SI-368 (11)'s sum, the inferred or unresolved
    // objects plus the unclassified edits (SI-368 (28)(a)), on a wall whose
    // review packet carries unclassified edits.
    await page.goto(CHANGES_UNCLASSIFIED_WALL);
    const packet = await page.request.post(CHANGES_UNCLASSIFIED_WALL + "/api/prepare_design_review", { data: {} });
    expect(packet.status(), await packet.text()).toBe(200);
    const review = (await packet.json()) as { inferred_or_unresolved: unknown[]; unclassified_edits: unknown[] };
    expect(review.unclassified_edits.length, "the wall's packet carries unclassified edits").toBeGreaterThan(0);
    const sum = review.inferred_or_unresolved.length + review.unclassified_edits.length;
    await page.getByTestId("wall-more").click();
    await expect(page.getByTestId("wall-more-count-review"), "the Review count").toHaveText(sum === 1 ? "1 needs a human eye" : `${sum} need a human eye`, { timeout: 15_000 });
  });

  test("Every drawer tab renders prose and tables, never JSON", async ({ page }) => {
    test.setTimeout(180_000);
    const wall = boardPath(SHOWCASE.DESIGN_SPEC);
    const ac = SHOWCASE.AC_IDS[1];
    // One typed operation on the wall, and its undo, so its provenance
    // holds entries whatever ran before.
    await page.goto(wall);
    const card = page.getByTestId(`card-${ac}`);
    const text = (await card.locator(".card-text").getAttribute("title"))!;
    const evidence = (await card.getAttribute("data-evidence"))!.split(",").filter(Boolean);
    expect(text, "the criterion's own text").toBeTruthy();
    await postTypedEdit(page, ac, `${text} [91-ac5]`, evidence);
    await page.reload();
    await expect(card).toContainText("[91-ac5]");

    try {
      // Readiness: the shared readiness body in the wall's words, each
      // Focus next item guidance first — its fact, timing and blocking flag
      // filed in its technical disclosure — and an item's click selects its
      // card on the wall.
      const readiness = await openTab(page, "readiness");
      const body = readiness.getByTestId("readiness-tab");
      await expect(body).toBeVisible();
      await expect(body.locator(".readiness-station")).toHaveCount(4);
      await expect(body.getByRole("heading", { name: /^Focus next/ })).toBeVisible();
      expect(await readiness.innerText()).not.toMatch(JSON_TEXT);
      const items = body.locator("#readiness-focus > .readiness-queue-list > li > .readiness-card");
      expect(await items.count(), "the current step lists its items").toBeGreaterThan(0);
      for (const item of await items.all()) {
        const copy = item.locator(".readiness-copy");
        const first = copy.locator(":scope > *").first();
        await expect(first, "the item's first line is its primary line").toHaveClass("readiness-primary");
        const primary = first.locator("p.readiness-summary");
        await expect(primary).toHaveClass(/readiness-guidance/);
        const tech = copy.locator("details.readiness-tech");
        await tech.locator("summary").click();
        const facts = tech.locator(".readiness-tech-facts");
        for (const label of ["Fact", "Timing", "Blocking"]) {
          await expect(facts.locator("dt", { hasText: new RegExp(`^${label}$`) }), `${label} in the disclosure`).toHaveCount(1);
        }
        expect((await facts.locator("dd.readiness-fact").textContent())?.trim(), "the fact is filed, not the primary line").not.toBe((await primary.textContent())?.trim());
        await tech.locator("summary").click();
      }
      const target = body.locator('article.readiness-card[data-target-kind="object"]').first();
      const id = (await target.getAttribute("data-target"))!;
      await target.locator("p.readiness-summary").click();
      await expect(page.getByTestId(`card-${id}`), `the item selects ${id}'s card`).toHaveAttribute("data-selected", "true");
      await expect(target).toHaveAttribute("data-found", "true");
      await expect(page.getByTestId("record-drawer")).toBeVisible();

      // Readiness loads again only when the revision moves (SI-368 (21),
      // (28)(a)): open across three polls that change nothing, it makes no
      // readiness request.
      const loads: string[] = readsOf(page);
      const polls = pollsOf(page);
      const revision = await revisionOf(page);
      await expect.poll(() => polls.length, { timeout: 20_000 }).toBeGreaterThanOrEqual(3);
      expect(polls.every((status) => status === 304), `each poll found nothing changed: ${polls}`).toBe(true);
      expect(await revisionOf(page), "no poll moved the revision").toBe(revision);
      expect(loads, "no readiness request while the revision stands").toEqual([]);
      await expect(page.getByTestId("record-tab-readiness")).toHaveAttribute("aria-selected", "true");

      // The drawer lives outside the swapped region (SI-368 (9)): an outside
      // write that moves the revision swaps the wall under the open tab,
      // and the drawer stays, its tab chosen, the Readiness loaded again for
      // the new revision (SI-368 (21)).
      const swapText = "an outside write under the open drawer [91-ac5]";
      const made = await page.request.post(wall + "/api/sticky", { data: { text: swapText, type: "comment" } });
      expect(made.status(), await made.text()).toBe(200);
      const swapped = page.locator('[data-testid^="sticky-"]').filter({ hasText: swapText });
      await expect(swapped, "the poll swapped the wall").toHaveCount(1, { timeout: 10_000 });
      await expect(page.getByTestId("record-drawer")).toBeVisible();
      await expect(page.getByTestId("record-tab-readiness")).toHaveAttribute("aria-selected", "true");
      await expect.poll(() => loads.filter((r) => r === "readiness").length, { timeout: 15_000 }).toBeGreaterThan(0);
      await expect(readiness.getByTestId("readiness-tab")).toBeVisible({ timeout: 15_000 });
      const swapID = (await swapped.getAttribute("data-id"))!;
      const gone = await page.request.post(wall + "/api/annotation-delete", { data: { ids: [swapID] } });
      expect(gone.status(), await gone.text()).toBe(200);

      // Provenance: the typed operations as a timeline of rows, and the
      // unclassified direct edits; the Review packet's sections, naming the
      // branch and the command, with no control that opens a pull request;
      // the bounded Context with its capabilities; the bar's posture as the
      // Repo tab; the Moves and the Keys.
      const provenance = await openTab(page, "provenance");
      await expectProse(provenance, ["Typed operations", "Unclassified direct edits"]);
      await expect(provenance.locator(".record-row").filter({ hasText: `edit-ac ${ac}` }).first()).toBeVisible();
      const review = await openTab(page, "review");
      await expectProse(review, ["Review base", "Semantic changes", "Needs a human eye", "Material warnings", "Policy in force", "Open a pull request"]);
      await expectProposal(review, SHOWCASE.DESIGN_BRANCH);
      await expect(page.getByTestId("record-drawer").locator("a, button, [role=button], [role=link]").filter({ hasText: /pull request|merge request/i })).toHaveCount(0);
      const context = await openTab(page, "context");
      await expectProse(context, ["Current spec", "Parent feature", "Applicable policy", "What agents may do", "Pinned context", "Verdi-go findings", "Digests"]);
      const repo = await openTab(page, "repo");
      await expectProse(repo, ["Working tree", "Accepted record"]);
      await expect(repo.locator(".record-row").filter({ hasText: "branch" }).first()).toContainText(SHOWCASE.DESIGN_BRANCH);
      const moves = await openTab(page, "moves");
      await expectProse(moves, ["The minimum path", "Kept as they were"]);
      const keys = await openTab(page, "keys");
      await expectProse(keys, ["Selection", "Acting on the selection", "Yarn key"]);
      await page.getByTestId("record-drawer-close").click();
    } finally {
      await postTypedEdit(page, ac, text, evidence);
    }

    // An honest empty state on a wall without the projection: no typed
    // operation recorded, no pinned context, no structural warning, no
    // yarn — each said in a sentence, never an empty list or a zero
    // dressed as a value.
    await page.goto(DRAWER_EMPTY_WALL);
    const empties: [string, string, RegExp][] = [
      ["provenance", "Typed operations", /No typed operation is recorded/],
      ["review", "Material warnings", /None\. The diff since the review base raised no structural warning/],
      ["context", "Pinned context", /declares no pinned context/],
      ["keys", "Yarn key", /No yarn on this wall yet/],
    ];
    for (const [tab, section, words] of empties) {
      const panel = await openTab(page, tab);
      await expect(panel.locator(".record-section").filter({ has: page.getByRole("heading", { name: section, exact: true }) }).locator(".record-empty")).toHaveText(words);
      expect(await panel.innerText()).not.toMatch(JSON_TEXT);
    }
    // The posture reason where a projection is unavailable: this wall's
    // branch is not the serving checkout's, so its readiness cannot be
    // derived here, and the tab says why.
    const fixed = await openTab(page, "readiness");
    await expect(fixed.getByTestId("readiness-unavailable")).toHaveText(/^Readiness is unavailable: .+\.$/);
    await page.getByTestId("record-drawer-close").click();

    // On a wall that takes edits the Review tab names the branch, the bar's
    // own fact, and the command even when the review packet cannot be read,
    // its reason beside them (SI-368 (12), (28)(a)): the payoff-quote-portal
    // draft, whose project has adopted no policy.
    await page.goto(branchBoardPath(SHOWCASE.SHOWCASE_DRAFT_BRANCH, SHOWCASE.SHOWCASE_DRAFT_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    const draftReview = await openTab(page, "review");
    await expect(draftReview.getByTestId("record-unavailable")).toHaveText(/^The review packet is unavailable: policy-forbidden: .+/);
    await expectProposal(draftReview, SHOWCASE.SHOWCASE_DRAFT_BRANCH);
    expect(await draftReview.innerText()).not.toMatch(JSON_TEXT);
    await page.getByTestId("record-drawer-close").click();

    // Unavailable, with the reason: a provenance record that does not
    // decode leaves the Provenance and Review projections unreadable.
    // Provenance has nothing beside its reason; Review keeps the one
    // section its wall proposes from, the branch and the command.
    await page.goto(DRAWER_UNAVAILABLE_WALL);
    for (const [tab, subject, sections] of [
      ["provenance", "Provenance is", 0],
      ["review", "The review packet is", 1],
    ] as const) {
      const panel = await openTab(page, tab);
      await expect(panel.getByTestId("record-unavailable")).toHaveText(new RegExp(`^${subject} unavailable: .*decoding design provenance`));
      await expect(panel.locator(".record-section")).toHaveCount(sections);
    }
    await expectProposal(page.getByTestId("record-panel-review"), branchOf(DRAWER_UNAVAILABLE_WALL));
    await page.getByTestId("record-drawer-close").click();
    // A design context the core cannot compile: the sealed record whose
    // pinned context does not resolve in the hermetic history.
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    const sealedContext = await openTab(page, "context");
    await expect(sealedContext.getByTestId("record-unavailable")).toHaveText(/^The design context is unavailable: .+/);
    // On a wall that takes no edit the Review tab proposes nothing (SI-368
    // (28)(b), F3BR-3): the sealed record and the review mirror each say no
    // pull request is proposed from them, and show no push command.
    for (const [wall, mode] of [
      [boardPath(SHOWCASE.READONLY_SPEC), "readonly"],
      [boardPath(SHOWCASE.REVIEW_SPEC), "review"],
    ] as const) {
      await page.goto(wall);
      await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", mode);
      const proposes = (await openTab(page, "review")).getByTestId("record-review-command");
      await expect(proposes.locator(".record-empty"), `${mode}: proposes nothing`).toHaveText(/^No pull request is proposed from this wall: /);
      await expect(proposes.locator("code"), `${mode}: no push command`).toHaveCount(0);
      expect(await proposes.innerText(), `${mode}: no push command`).not.toContain("git push");
      await page.getByTestId("record-drawer-close").click();
    }
    // A repository posture that cannot be proven: the store whose default
    // branch cannot be resolved names each fact's reason.
    const res = await page.request.get(`${CONTROL_URL}/unproven-board-fixture`);
    expect(res.ok(), await res.text()).toBe(true);
    await page.goto(`${(await res.text()).trim()}board/spec/${EDGE.UNPROVEN_BOARD_SPEC}`);
    const unproven = await openTab(page, "repo");
    const accepted = unproven.locator(".record-row").filter({ has: page.locator("dt", { hasText: /^accepted branch$/ }) });
    await expect(accepted.locator(".record-badge")).toHaveText("unproven");
    await expect(accepted.locator(".record-note")).toHaveText(/\S/);
  });

  test("The rail is gone and every item has a home", async ({ page }) => {
    test.setTimeout(240_000);
    const unprovenRes = await page.request.get(`${CONTROL_URL}/unproven-board-fixture`);
    expect(unprovenRes.ok(), await unprovenRes.text()).toBe(true);
    const unprovenWall = `${(await unprovenRes.text()).trim()}board/spec/${EDGE.UNPROVEN_BOARD_SPEC}`;

    // No rail element on any wall, in any mode or room: the rail, the shell
    // beside it, its JSON panels, its scratch and typed-operation panels,
    // the four-move guide, a visible yarn key, Instantiate inside a stub
    // card — and no column is kept for it: the wall frame takes the
    // region's width.
    const walls: [string, string, string][] = [
      ["the live authoring wall", boardPath(SHOWCASE.DESIGN_SPEC), "authoring"],
      ["the empty story wall", boardPath(SHOWCASE.EMPTY_SPEC), "authoring"],
      ["the policy-less draft", branchBoardPath(SHOWCASE.SHOWCASE_DRAFT_BRANCH, SHOWCASE.SHOWCASE_DRAFT_SPEC), "authoring"],
      ["an authoring wall under its domain refusal", boardPath(EDGE.STATUSLESS_DRAFT_SPEC), "authoring"],
      ["the review mirror", boardPath(SHOWCASE.REVIEW_SPEC), "review"],
      ["the sealed record with stubs", boardPath(SHOWCASE.FEATURE_SPEC), "readonly"],
      ["a read-only wall not yet accepted", branchBoardPath(SHOWCASE.DB_SAME_SPEC_BRANCH, SHOWCASE.DB_SAME_SPEC), "readonly"],
      ["a read-only wall whose lifecycle is unproven", unprovenWall, "readonly"],
    ];
    const rail = [".board-side", "#asd-shell", '[data-testid="asd-shell"]', "[data-asd-panel]", ".asd-panel-json", "#add-sticky-btn", ".scratch-panel", "#asd-forms", "#asd-set-problem", "#asd-set-outcome", '[data-testid="board-guide"]', ".stub-instantiate", "#board-canvas [data-instantiate]"];
    for (const [name, wall, mode] of walls) {
      await page.goto(wall);
      await expect(page.getByTestId("board"), name).toHaveAttribute("data-board-mode", mode);
      for (const sel of rail) {
        await expect(page.locator(sel), `${name}: ${sel}`).toHaveCount(0);
      }
      await expect(page.getByRole("button", { name: "Add sticky" }), `${name}: the rail's Add sticky`).toHaveCount(0);
      // The rail's scratch panel's home: the toolbar's Sticky, offered
      // exactly where the scratch tier is live.
      if (mode === "authoring") await expect(stickyAction(page), `${name}: the toolbar's Sticky`).toBeVisible();
      else await expect(stickyAction(page), `${name}: no Sticky outside authoring`).toHaveCount(0);
      await expect(page.locator('[data-testid="yarn-key"]:visible'), `${name}: a visible yarn key`).toHaveCount(0);
      const gap = await page.evaluate(() => {
        const region = document.getElementById("boardv2-region")!.getBoundingClientRect();
        const frame = document.querySelector('[data-testid="wall-frame"]')!.getBoundingClientRect();
        return region.right - frame.right;
      });
      expect(gap, `${name}: the frame takes the region's width, with no rail column`).toBeLessThanOrEqual(24);
      const empty = page.getByTestId("board-empty");
      if (await empty.count()) await expect(empty, `${name}: the empty wall names no rail`).not.toContainText("rail");
    }

    // The review-mode inbox tray stays docked and visible: directly below
    // the wall frame, in the region, keeping role=region "Inbox tray".
    await page.goto(boardPath(SHOWCASE.REVIEW_SPEC));
    const tray = page.getByRole("region", { name: "Inbox tray" });
    await expect(tray).toHaveCount(1);
    await tray.scrollIntoViewIfNeeded();
    await expect(tray).toBeVisible();
    await expect(page.locator("#boardv2-region").getByRole("region", { name: "Inbox tray" })).toHaveCount(1);
    const docked = await page.evaluate(() => {
      const frame = document.querySelector('[data-testid="wall-frame"]')!.getBoundingClientRect();
      const t = document.querySelector('[aria-label="Inbox tray"]')!.getBoundingClientRect();
      return { below: t.top - frame.bottom, left: Math.abs(t.left - frame.left) };
    });
    expect(docked.below, "the tray sits below the wall frame").toBeGreaterThanOrEqual(0);
    expect(docked.below, "the tray is docked to the wall frame").toBeLessThanOrEqual(32);
    expect(docked.left, "the tray lines up with the wall frame").toBeLessThanOrEqual(2);
    await expect(tray.locator('[data-annotation-type="review"]').first()).toBeVisible();
    // The mirror explains itself among the notices.
    await expect(page.locator(".board-notices .mirror-note")).toContainText("mirrors the merge request");

    // On a sealed wall, New story and Revise are the top bar's primary
    // action; their notes sit among the notices beside the sealed record's.
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    const bar = page.getByTestId("topbar");
    const newStory = bar.getByRole("button", { name: /^New story$/ });
    await expect(newStory).toBeVisible();
    await expect(newStory).toHaveClass(/btn-primary/);
    await expect(bar.getByRole("button", { name: /^Revise this feature$/ })).toBeVisible();
    await expect(page.locator("#boardv2-region").getByRole("button", { name: /New story|Revise/ })).toHaveCount(0);
    const notices = page.locator(".board-notices");
    await expect(notices.locator(".sealed-panel")).toContainText("This spec is accepted");
    await expect(notices.getByTestId("create-panel")).toBeVisible();
    await expect(notices.getByTestId("revise-panel")).toBeVisible();

    // Instantiate is on the stub card's toolbar: never inside the card, not
    // offered with nothing selected, offered for the selected stub with its
    // test id and its consequence, and the card keeps to its footprint.
    const slug = SHOWCASE.STUB_SLUGS[0];
    const stub = page.getByTestId(stubCardTestId(slug));
    await expect(stub).toBeVisible();
    await expect(stub.locator("[data-instantiate], .stub-instantiate")).toHaveCount(0);
    await expect(stub).not.toContainText("Instantiate");
    await expect(wallToolbar(page).locator("[data-instantiate]")).toHaveCount(0);
    await selectStub(page, slug);
    const instantiate = wallToolbar(page).getByTestId(`instantiate-${slug}`);
    await expect(instantiate).toBeVisible();
    await expect(instantiate).toHaveText("Instantiate story");
    await expect(instantiate).toHaveAttribute("data-instantiate", slug);
    const overflow = await stub.evaluate((card) => {
      const box = card.getBoundingClientRect();
      return Math.max(0, ...Array.from(card.children).map((c) => c.getBoundingClientRect().bottom - box.bottom));
    });
    expect(overflow, "the stub card's contents keep to its footprint (BL-167)").toBeLessThanOrEqual(0.5);
    await instantiate.click();
    const confirm = page.locator("#edge-confirm");
    await expect(confirm).toBeVisible();
    await expect(confirm).toContainText(`Instantiate story “${slug}”`);
    await expect(confirm).toContainText(`design/${slug}`);
    await page.locator("#edge-confirm-cancel").click();
    await expect(confirm).toBeHidden();
    await expect(stub).toHaveAttribute("data-selected", "true");
    // Its label is the one the server writes on the card in the store's
    // words — a renamed store's is pinned in Go (vocabulary_render_test.go;
    // SI-368 (30)) — and the toolbar shows it as written, with no class
    // word of its own: a card that carries other words is named with them.
    await expect(stub).toHaveAttribute("data-instantiate-label", "Instantiate story");
    await clearWallSelection(page);
    await expect(wallToolbar(page).locator("[data-instantiate]")).toHaveCount(0);
    await stub.evaluate((card) => card.setAttribute("data-instantiate-label", "Instantiate Workstream"));
    await selectStub(page, slug);
    await expect(wallToolbar(page).getByTestId(`instantiate-${slug}`)).toHaveText("Instantiate Workstream");

    // The policy setup guide is in the drawer's Readiness tab, its id once
    // on the page, and nowhere on the wall.
    await page.goto(branchBoardPath(SHOWCASE.SHOWCASE_DRAFT_BRANCH, SHOWCASE.SHOWCASE_DRAFT_SPEC));
    const readiness = await openTab(page, "readiness");
    await expect(readiness.getByTestId("asd-policy-guide")).toBeVisible();
    await expect(page.locator("#asd-policy-guide")).toHaveCount(1);
    await expect(page.locator('#boardv2-region [data-testid="asd-policy-guide"]')).toHaveCount(0);
    await page.getByTestId("record-drawer-close").click();

    // Every readiness target resolves to an element that exists: a card, a
    // stub, a strip half or a slot the wall draws, and each row without one
    // offers nothing to find; every in-page link the tab carries lands. The
    // criteria row finds its slot only where the wall draws the slot.
    for (const [name, wall, slot] of [
      ["the live authoring wall", boardPath(SHOWCASE.DESIGN_SPEC), true],
      ["the review mirror", boardPath(SHOWCASE.REVIEW_SPEC), false],
      ["the sealed record with stubs", boardPath(SHOWCASE.FEATURE_SPEC), false],
      ["the sealed record", boardPath(SHOWCASE.READONLY_SPEC), false],
    ] as const) {
      await page.goto(wall);
      const tab = await openTab(page, "readiness");
      const rows = tab.locator("article[data-target-kind]");
      expect(await rows.count(), `${name}: the tab lists its readiness`).toBeGreaterThan(0);
      for (const row of await rows.all()) {
        const id = await row.getAttribute("data-concern-id");
        const kind = await row.getAttribute("data-target-kind");
        const value = (await row.getAttribute("data-target")) ?? "";
        const target = {
          object: `#board-canvas [data-testid="card-${value}"]`,
          stub: `#board-canvas [data-testid="stub-card-${value}"]`,
          strip: `#boardv2-region [data-testid="placard-${value}"]`,
          slot: `#boardv2-region .wall-slot[data-slot-kind="${value}"]`,
        }[kind ?? ""];
        if (kind === "none") {
          await expect(row.locator(".readiness-target"), `${name}: ${id} offers nothing to find`).toHaveCount(0);
          continue;
        }
        expect(target, `${name}: ${id}'s target kind ${kind}`).toBeTruthy();
        await expect(page.locator(target!), `${name}: ${id} → ${kind} ${value}`).toHaveCount(1);
      }
      const criteria = tab.locator('article[data-concern-id="success/criteria"]');
      await expect(criteria, `${name}: the criteria row`).toHaveCount(1);
      await expect(criteria, `${name}: the criteria row's target`).toHaveAttribute("data-target-kind", slot ? "slot" : "none");
      const dangling = await tab.evaluate((panel) =>
        Array.from(panel.querySelectorAll<HTMLAnchorElement>('a[href^="#"]'))
          .map((a) => a.getAttribute("href")!.slice(1))
          .filter((id) => !document.getElementById(decodeURIComponent(id))),
      );
      expect(dangling, `${name}: every in-page link lands`).toEqual([]);
      await page.getByTestId("record-drawer-close").click();
    }
    // No retired rail anchor is pointed at from anywhere on a wall.
    await expect(page.locator('a[href="#asd-forms"], a[href="#asd-git"], a[href="#asd-policy-guide"]')).toHaveCount(0);
  });
});
