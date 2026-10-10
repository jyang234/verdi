import { test, expect, type Page, type Locator } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { resolvePorts } from "../ports";
import {
  SHOWCASE,
  EDGE,
  CONTROL_URL,
  boardPath,
  branchBoardPath,
  diagramEditorPath,
  importPagePath,
} from "./fixtures";

// spec/chrome-and-tokens-v2 (jira:VERDI-WR-1), ac-1, ac-2, and ac-5: every
// workbench page opens with ONE top bar (data-testid topbar) and none of
// the old site header, board header, or posture row; on a page about one
// spec the bar shows the displayed bytes and the working tree's state at
// every width, with the full posture one activation away — a native
// <details> disclosure, so it opens before any script runs; and the bar
// holds at 320 px and 200 % zoom, by keyboard in reading order, under the
// existing axe check and the Wave 6 page budget. A fourth case proves the
// posture popover's enhancement (dc-2; ledger SI-331) and the live swap of
// the posture group (SI-323 (3)).
//
// The first three tests are the producers their obligations name
// (.verdi/obligations/chrome-and-tokens-v2/), titled exactly as the
// obligation's claim spells it, and the file passes when run alone
// (BL-98). Every case drives the REAL `verdi serve` subprocess; state
// assertions ride test ids, data attributes, and text — never screenshots
// (recording stays off).

const WORKBENCH = `http://127.0.0.1:${resolvePorts().workbench}`;
const TOPBAR = '[data-testid="topbar"]';
const OLD_ROWS = [".site-head", ".board-head", ".asd-posture"];

// The v0 board the harness provisions (00-home.spec.ts clicks through to
// it by this same key); SI-323 (4): it is a page verdi serve renders, so
// it gets the bar too.
const V0_BOARD = "/board/STORY-1482";

// The fixtures' page titles, as cmd/e2eharness provisions them
// (provision_board.go, provision_showcase_draft.go, provision_diagram.go)
// and as the handlers title their pages (matrix.go, verdict.go,
// disclosures.go, specimportrender.go, readinessrender.go, index.go).
const TITLES = {
  design: "Refinancing decline flow",
  draft: "Payoff quote portal",
  diagram: "Editor proposal",
  // The sealed accepted feature (examples/showcase's escrow-autopay) and
  // the never-committed changes wall (cmd/e2eharness/provision_wallstrip.go).
  feature: "Escrow autopay enrollment",
  unreadable: "Decline retraction (decline-changes-unreadable)",
};

// The changes wall whose comparison with HEAD is unreadable: F3-go2's
// harness wall (provision_wallstrip.go), on its namesake branch, copied
// here as 92-wall-commit-changes copies it (fixtures.ts stays F7's) and
// pinned by the harness's TestWallStripPaths.
const UNREADABLE_CHANGES_WALL = "/b/design%2Fdecline-changes-unreadable/board/spec/decline-changes-unreadable";
// Its sibling whose working tree holds typed changes (the same harness
// file), whose bar carries the longest Commit and push suffix.
const TYPED_CHANGES_WALL = "/b/design%2Fdecline-changes-typed/board/spec/decline-changes-typed";

// The readiness page lives on the readiness-pilot fixture's isolated
// serve (49-readiness-pilot.spec.ts's own path to it): started lazily by
// the control server on first use and reused thereafter.
async function readinessPageURL(page: Page): Promise<string> {
  const res = await page.request.get(`${CONTROL_URL}/readiness-pilot-fixture`);
  expect(res.ok(), await res.text()).toBe(true);
  const base = (await res.text()).trim();
  expect(base).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return base + "readiness";
}

// The unproven-board fixture (51-board-unproven-lifecycle.spec.ts): a
// REAL no-remote store whose default branch cannot be resolved, served by
// the shipped binary — the one fixture where a posture fact is honestly
// disclosed-unproven, and whose working tree stays clean (every write to
// it is refused).
async function unprovenWallURL(page: Page): Promise<string> {
  const res = await page.request.get(`${CONTROL_URL}/unproven-board-fixture`);
  expect(res.ok(), await res.text()).toBe(true);
  const base = (await res.text()).trim();
  expect(base).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return `${base}board/spec/${EDGE.UNPROVEN_BOARD_SPEC}`;
}

interface WorkbenchPage {
  name: string;
  path: (page: Page) => string | Promise<string>;
  // spec: a page about one spec (class and mode chips, displayed bytes).
  spec: boolean;
  // title: the bar's page title, pinned from the fixtures.
  title: string;
  // status: the response status the page is served with (404, 400).
  status?: number;
  // overflow: the page body's own horizontal overflow before this story,
  // in CSS px at 320 px and at 200 % zoom (ledger SI-332 (1): the 320 px
  // figures at base 4cbe1073, the readiness and 200 % figures from the
  // review's base probe at the same commit). A body that overflowed
  // before this story is the next story's to fix; it may not grow here.
  overflow?: { at320: number; at200: number };
}

// ac-1's page list, each over the fixture stores: the wall on the default
// branch and on a design branch, the Document page, the diagram editor,
// the readiness page, the index, and every shared-layout page — matrix,
// verdict, disclosures, spec import, corpus, not found, error — plus the
// v0 board (SI-323 (4)).
const PAGES: WorkbenchPage[] = [
  { name: "wall on the default branch", path: () => boardPath(SHOWCASE.DESIGN_SPEC), spec: true, title: TITLES.design },
  {
    name: "wall on a design branch",
    path: () => branchBoardPath(SHOWCASE.SHOWCASE_DRAFT_BRANCH, SHOWCASE.SHOWCASE_DRAFT_SPEC),
    spec: true,
    title: TITLES.draft,
  },
  {
    // spec/wall-strip-and-drawer-v2 (SI-368 (26)(a), F3AR-1 and F3AR-3):
    // the sealed accepted feature's wall, whose bar carries New story and
    // Revise as its primary actions and whose strip wears a long badge
    // chip — the widest bar and the widest chips row the fixtures hold.
    name: "sealed feature wall",
    path: () => boardPath(SHOWCASE.FEATURE_SPEC),
    spec: true,
    title: TITLES.feature,
  },
  {
    // The authoring wall with the long title and the pill that reads
    // "readiness unavailable" (its comparison with HEAD is unreadable).
    name: "unreadable changes wall",
    path: () => UNREADABLE_CHANGES_WALL,
    spec: true,
    title: TITLES.unreadable,
  },
  {
    // spec/document-page-v2 ac-4 fixed the Document body's own overflow
    // (354 px at 320, 99 px at 200 % before it): no base figure, so the
    // page has no horizontal scroll at all.
    name: "Document page",
    path: () => boardPath(SHOWCASE.DESIGN_SPEC) + "/document",
    spec: true,
    title: TITLES.design,
  },
  { name: "diagram editor", path: () => diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL), spec: false, title: TITLES.diagram },
  {
    name: "readiness page",
    path: (page) => readinessPageURL(page),
    spec: false,
    title: "Readiness",
    overflow: { at320: 8, at200: 18 },
  },
  { name: "index", path: () => "/", spec: false, title: "Workbench", overflow: { at320: 129, at200: 0 } },
  {
    name: "matrix",
    path: () => `/matrix/spec/${SHOWCASE.SLOT_WALL_SPEC}`,
    spec: false,
    // matrix.go titles the page by the story's tracker ref: the slot
    // wall's story field, as cmd/e2eharness provisions it.
    title: "Advisory preview matrix: jira:LOAN-2204",
    overflow: { at320: 114, at200: 0 },
  },
  {
    name: "verdict",
    path: () => `/verdict/spec/${SHOWCASE.READONLY_SPEC}`,
    spec: false,
    title: `Verdict viewer: spec/${SHOWCASE.READONLY_SPEC}`,
    overflow: { at320: 106, at200: 0 },
  },
  { name: "disclosures", path: () => "/disclosures", spec: false, title: "Disclosures" },
  { name: "spec import", path: () => importPagePath(), spec: false, title: "Import existing spec", overflow: { at320: 373, at200: 138 } },
  {
    name: "corpus",
    path: () => `/a/spec/${SHOWCASE.READONLY_SPEC}`,
    spec: false,
    title: "Stale decline handling (fixture)",
    overflow: { at320: 52, at200: 0 },
  },
  { name: "not found", path: () => "/no-such-page-87", spec: false, title: "Not found", status: 404 },
  // A commit pin in ?spec= is refused before any store read (readiness.go):
  // a deterministic 400 on the workbench's own error shell.
  { name: "error", path: () => "/readiness?spec=no-such-spec@deadbeef", spec: false, title: "Error", status: 400 },
  { name: "v0 board", path: () => V0_BOARD, spec: false, title: `Board: STORY-1482` },
];

async function gotoPage(page: Page, p: WorkbenchPage): Promise<void> {
  const resp = await page.goto(await p.path(page));
  expect(resp, p.name).not.toBeNull();
  expect(resp!.status(), `${p.name}: status`).toBe(p.status ?? 200);
}

function bar(page: Page): Locator {
  return page.locator(TOPBAR);
}

// barControls: the bar's focusable controls in DOM (reading) order —
// links, buttons, and the posture disclosure's summary — as rendered.
async function barControls(page: Page): Promise<string[]> {
  return bar(page).evaluate((el) =>
    Array.from(el.querySelectorAll<HTMLElement>('a[href],button:not([disabled]),summary,[tabindex]:not([tabindex="-1"])'))
      .filter((c) => !c.hidden && c.offsetParent !== null)
      .map((c) => c.getAttribute("data-testid") || c.id || c.className || c.tagName),
  );
}

// pageOverflow is the page's horizontal overflow in CSS px.
async function pageOverflow(page: Page): Promise<number> {
  return page.evaluate(() => {
    const el = document.scrollingElement!;
    return el.scrollWidth - el.clientWidth;
  });
}

// expectBarFits (SI-332 (1)): the bar sits inside the viewport, scrolls
// nothing inside itself, and adds no horizontal overflow to the page
// (measured with the bar shown and hidden); the page body's own overflow
// does not exceed its figure before this story, and a page whose body had
// none has no horizontal scroll at all.
async function expectBarFits(page: Page, p: WorkbenchPage, zoom: "at320" | "at200", what: string): Promise<void> {
  const box = await bar(page).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { left: r.left, right: r.right, scrollWidth: el.scrollWidth, clientWidth: el.clientWidth, viewport: document.documentElement.clientWidth };
  });
  expect(box.left, `${what}: bar left edge`).toBeGreaterThanOrEqual(-1);
  expect(box.right, `${what}: bar right edge`).toBeLessThanOrEqual(box.viewport + 1);
  expect(box.scrollWidth, `${what}: bar inner overflow`).toBeLessThanOrEqual(box.clientWidth + 1);
  const withBar = await pageOverflow(page);
  await bar(page).evaluate((el) => ((el as HTMLElement).style.display = "none"));
  const withoutBar = await pageOverflow(page);
  await bar(page).evaluate((el) => ((el as HTMLElement).style.display = ""));
  expect(withBar, `${what}: the bar adds horizontal overflow (page ${withoutBar} px without it)`).toBeLessThanOrEqual(Math.max(withoutBar, 0) + 1);
  const base = p.overflow?.[zoom] ?? 0;
  expect(withoutBar, `${what}: the page body overflows past its base figure ${base} px`).toBeLessThanOrEqual(base + 1);
  if (base === 0) {
    expect(withBar, `${what}: horizontal overflow`).toBeLessThanOrEqual(1);
  } else if (withoutBar > 1) {
    console.log(`disclosed: ${what}: the page body overflows by ${withoutBar} px without the bar (pre-existing; base ${base} px)`);
  }
}

// Every bar control visible or reachable: each focusable control in the
// bar is rendered on screen (the closed posture panel's facts are
// reachable through its summary, which is one of them).
async function expectBarControlsVisible(page: Page, what: string): Promise<void> {
  const controls = bar(page).locator('a[href],button:not([disabled]),summary');
  const n = await controls.count();
  expect(n, `${what}: bar controls`).toBeGreaterThan(0);
  for (let i = 0; i < n; i++) {
    await expect(controls.nth(i), `${what}: control ${i}`).toBeVisible();
  }
}

// postureFact reads one fact of the opened posture: its test-id element's
// text — exactly today's row's — and the state its data attribute carries.
async function postureFact(page: Page, slug: string): Promise<{ text: string; state: string | null }> {
  const dd = page.getByTestId(`asd-posture-${slug}`);
  await expect(dd).toBeVisible();
  return { text: (await dd.textContent())?.trim() ?? "", state: await dd.getAttribute("data-state") };
}

// postureWhy reads a fact's disclosed reason, the sibling element beside
// its test-id element (SI-332 (2)), or null when the fact carries none.
async function postureWhy(page: Page, slug: string): Promise<string | null> {
  const why = page.getByTestId(`asd-posture-${slug}-why`);
  if ((await why.count()) === 0) return null;
  await expect(why).toBeVisible();
  return (await why.textContent())?.trim() ?? "";
}

async function checkoutOf(page: Page, path: string): Promise<string> {
  const snap = await (await page.request.get(path + "/snapshot")).json();
  expect(typeof snap.expected?.checkout, `${path}: the snapshot names its checkout`).toBe("string");
  return snap.expected.checkout as string;
}

// expectWordsPainted (ac-2, closure ruling): the displayed-bytes word and
// the clean/dirty word are painted, never clipped — each fact's visible
// text (its first text node) has a box inside the posture summary's box
// and inside the viewport, and a hit test at its last character lands
// inside the summary.
async function expectWordsPainted(page: Page, what: string): Promise<void> {
  const m = await page.evaluate(() => {
    const s = document.querySelector(".topbar-posture > summary") as HTMLElement;
    const sb = s.getBoundingClientRect();
    const words: Record<string, { text: string; left: number; right: number; top: number; bottom: number; hitInside: boolean }> = {};
    for (const sel of [".asd-posture-bytes", ".asd-posture-tree"]) {
      const e = s.querySelector(sel);
      if (!e) continue;
      const t = e.firstChild;
      if (!t || t.nodeType !== Node.TEXT_NODE) throw new Error(sel + ": its first child is not its visible text");
      const txt = (t.textContent || "").replace(/\s+$/, "");
      const whole = document.createRange();
      whole.setStart(t, 0);
      whole.setEnd(t, txt.length);
      const r = whole.getBoundingClientRect();
      const last = document.createRange();
      last.setStart(t, txt.length - 1);
      last.setEnd(t, txt.length);
      const c = last.getBoundingClientRect();
      const hit = document.elementFromPoint(c.left + c.width / 2, c.top + c.height / 2);
      words[sel] = { text: txt, left: r.left, right: r.right, top: r.top, bottom: r.bottom, hitInside: !!hit && s.contains(hit) };
    }
    return { summary: { left: sb.left, right: sb.right, top: sb.top, bottom: sb.bottom }, viewport: document.documentElement.clientWidth, words };
  });
  expect(Object.keys(m.words), `${what}: both words present`).toEqual([".asd-posture-bytes", ".asd-posture-tree"]);
  for (const [sel, w] of Object.entries(m.words)) {
    const tag = `${what} ${sel} "${w.text}"`;
    expect(w.left, `${tag}: starts left of the summary`).toBeGreaterThanOrEqual(m.summary.left - 0.5);
    expect(w.right, `${tag}: clipped by the summary's right edge`).toBeLessThanOrEqual(m.summary.right + 0.5);
    expect(w.top, `${tag}: above the summary`).toBeGreaterThanOrEqual(m.summary.top - 0.5);
    expect(w.bottom, `${tag}: below the summary`).toBeLessThanOrEqual(m.summary.bottom + 0.5);
    expect(w.left, `${tag}: left of the viewport`).toBeGreaterThanOrEqual(-0.5);
    expect(w.right, `${tag}: right of the viewport`).toBeLessThanOrEqual(m.viewport + 0.5);
    expect(w.hitInside, `${tag}: its last character is not painted inside the summary`).toBe(true);
  }
}

async function expectBytesAndTree(
  page: Page,
  what: string,
  bytes: { state: string; word: string },
  tree: { state: string; text: string },
): Promise<void> {
  const bytesEl = bar(page).getByTestId("asd-posture-bytes");
  await expect(bytesEl, `${what}: bytes`).toBeVisible();
  await expect(bytesEl, `${what}: bytes state`).toHaveAttribute("data-state", bytes.state);
  await expect(bytesEl, `${what}: bytes text`).toHaveText(`displayed bytes: ${bytes.word} (${bytes.state})`);
  const treeEl = bar(page).getByTestId("asd-posture-tree");
  await expect(treeEl, `${what}: tree`).toBeVisible();
  await expect(treeEl, `${what}: tree state`).toHaveAttribute("data-dirty", tree.state);
  await expect(treeEl, `${what}: tree text`).toHaveText(`working tree: ${tree.text}`);
}

// postTypedEdit posts one typed edit-ac on the design wall from outside
// the page, against the wall's current base — what 50-design-workbench's
// postMutate does.
async function postTypedEdit(page: Page, text: string): Promise<void> {
  const design = boardPath(SHOWCASE.DESIGN_SPEC);
  const snap = await (await page.request.get(design + "/snapshot")).json();
  const resp = await page.request.post(design + "/api/mutate_draft", {
    data: {
      request: {
        schema: "verdi.draftmutation/v1",
        spec: "spec/" + SHOWCASE.DESIGN_SPEC,
        base_digest: snap.base_digest,
        base_spec_b64: snap.base_spec_b64,
        expected: snap.expected,
        operations: [
          { op: "edit-ac", id: SHOWCASE.AC_IDS[1], text, evidence: ["attestation"], anchor: "#" + SHOWCASE.AC_IDS[1] },
        ],
      },
    },
  });
  expect(resp.status(), await resp.text()).toBe(200);
  expect((await resp.json()).result, "the typed mutation landed").toBeTruthy();
}

// activeKey names the focused element by test id, id, or tag; "BODY"
// when focus is nowhere.
async function activeKey(page: Page): Promise<string> {
  return page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    return el && el !== document.body ? el.getAttribute("data-testid") || el.id || el.tagName : "BODY";
  });
}

test.describe("chrome-and-tokens", () => {
  test("One top bar on every workbench page, and none of the old header rows", async ({ page }) => {
    test.setTimeout(150_000);
    for (const p of PAGES) {
      await gotoPage(page, p);
      const topbar = bar(page);
      await expect(topbar, `${p.name}: one bar`).toHaveCount(1);
      for (const old of OLD_ROWS) {
        await expect(page.locator(old), `${p.name}: ${old}`).toHaveCount(0);
      }
      // The wordmark links to the index.
      const wordmark = topbar.getByTestId("topbar-wordmark");
      await expect(wordmark, `${p.name}: wordmark`).toBeVisible();
      await expect(wordmark, `${p.name}: wordmark href`).toHaveAttribute("href", "/");
      await expect(wordmark, `${p.name}: wordmark text`).toContainText("verdi");
      // The page title, pinned from the fixtures.
      const title = topbar.getByTestId("topbar-title");
      await expect(title, `${p.name}: title`).toBeVisible();
      await expect(title, `${p.name}: title text`).toHaveText(p.title);
      // The branch and posture text, as two controls.
      await expect(topbar.getByTestId("topbar-branch"), `${p.name}: branch`).toBeVisible();
      await expect(topbar.getByTestId("topbar-posture"), `${p.name}: posture`).toBeVisible();
      // The class and mode chips on a page about one spec; neither elsewhere.
      const classChip = topbar.getByTestId("topbar-class-chip");
      const modeChip = topbar.locator(".board-mode-tag");
      if (p.spec) {
        await expect(classChip, `${p.name}: class chip`).toBeVisible();
        await expect(classChip, `${p.name}: class chip`).toHaveText("feature");
        await expect(modeChip, `${p.name}: mode chip`).toBeVisible();
      } else {
        await expect(classChip, `${p.name}: no class chip`).toHaveCount(0);
        await expect(topbar.getByTestId("asd-posture-bytes"), `${p.name}: no bytes`).toHaveCount(0);
      }
      // No literal dot text nodes between the bar's links (its nav and tabs).
      const dots = await topbar.evaluate((el) =>
        Array.from(el.querySelectorAll(".topbar-nav, .topbar-tabs")).flatMap((group) =>
          Array.from(group.childNodes).filter((n) => n.nodeType === Node.TEXT_NODE && /[·•]/.test(n.textContent ?? "")).map((n) => n.textContent),
        ),
      );
      expect(dots, `${p.name}: dot text nodes`).toEqual([]);
    }

    // The diagram editor's bar carries its explicit exit, distinct from the
    // index and artifact links (tool-view-exit ac-1): a direct URL knows no
    // originating board, so the exit discloses that and falls back to the
    // index — a different element from the nav's own index link.
    await page.goto(diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL));
    const topbar = bar(page);
    const exit = topbar.getByTestId("diagram-exit");
    await expect(exit).toBeVisible();
    await expect(exit).toHaveAttribute("href", "/");
    await expect(exit).toContainText(/no originating board is known/i);
    const indexLink = topbar.locator('.topbar-nav a[href="/"]');
    const artifactLink = topbar.locator(`.topbar-nav a[href="/a/diagram/${SHOWCASE.DIAGRAM_PROPOSAL}"]`);
    await expect(indexLink).toHaveCount(1);
    await expect(artifactLink).toHaveCount(1);
    await expect(topbar.locator(".topbar-nav")).not.toContainText("no originating board");
    expect(await exit.evaluate((el) => !!el.closest(".topbar-nav"))).toBe(false);
    await expect(topbar.locator(".board-mode-tag")).toBeVisible();

    // The controls slot (dc-3): Commit & push on the authoring wall, inside
    // the slot and nowhere else; absent in the other modes. The Wall and
    // Document switch in the slot too, on both pages, with the same labels.
    const design = boardPath(SHOWCASE.DESIGN_SPEC);
    await page.goto(design);
    await expect(page.locator("#commit-push-btn")).toHaveCount(1);
    await expect(page.locator('[data-testid="topbar-controls"] #commit-push-btn')).toHaveCount(1);
    const wallTabs = bar(page).locator('[data-testid="topbar-controls"] .topbar-tabs');
    await expect(wallTabs.locator(".current")).toHaveText("Wall");
    await expect(wallTabs.getByTestId("board-tab-document")).toHaveText("Document");
    await expect(wallTabs.getByTestId("board-tab-document")).toHaveAttribute("href", design + "/document");
    await page.goto(design + "/document");
    const docTabs = bar(page).locator('[data-testid="topbar-controls"] .topbar-tabs');
    await expect(docTabs.getByTestId("document-tab-board")).toHaveText("Wall");
    await expect(docTabs.getByTestId("document-tab-board")).toHaveAttribute("href", design);
    await expect(docTabs.getByTestId("document-tab-document")).toHaveText("Document");
    await expect(docTabs.locator(".current")).toHaveText("Document");
    for (const spec of [SHOWCASE.READONLY_SPEC, SHOWCASE.REVIEW_SPEC]) {
      await page.goto(boardPath(spec));
      await expect(page.getByTestId("board"), spec).not.toHaveAttribute("data-board-mode", "authoring");
      await expect(page.locator("#commit-push-btn"), `${spec}: no Commit & push`).toHaveCount(0);
    }
  });

  test("The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away", async ({
    page,
    browser,
  }) => {
    test.setTimeout(300_000);
    // A clean working tree, on a fixture whose default branch cannot be
    // resolved: the wall and its Document page, at 1440 px and at 320 px.
    const unproven = await unprovenWallURL(page);
    for (const width of [1440, 320]) {
      await page.setViewportSize({ width, height: 900 });
      for (const path of [unproven, unproven + "/document"]) {
        await page.goto(path);
        await expectBytesAndTree(
          page,
          `${path} @${width}`,
          { state: "unproven", word: "unproven" },
          { state: "clean", text: "clean" },
        );
      }
    }

    // A dirty working tree: one typed mutation on the design wall leaves
    // the spec file uncommitted; the wall and its Document page say so.
    const design = boardPath(SHOWCASE.DESIGN_SPEC);
    await postTypedEdit(page, "the bar states the tree's state honestly [87-dirty]");
    for (const width of [1440, 320]) {
      await page.setViewportSize({ width, height: 900 });
      for (const path of [design, design + "/document"]) {
        await page.goto(path);
        await expectBytesAndTree(
          page,
          `${path} @${width}`,
          { state: "proposed", word: "proposed" },
          { state: "dirty", text: "uncommitted changes" },
        );
      }
    }

    // A background snapshot refresh not caused by a click keeps the open
    // posture open (SI-323 (3); SI-331): with the disclosure open, a typed
    // mutation posted outside the page changes the revision, the visible
    // page's next poll tick applies it, and the swapped-in posture group
    // is still expanded and current.
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(design);
    await page.getByTestId("topbar-posture").click();
    await expect(page.locator(".asd-posture-tech")).toHaveAttribute("open", "");
    await postTypedEdit(page, "a background refresh keeps the open posture open [87-refresh]");
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[1])).toContainText("[87-refresh]", { timeout: 8_000 });
    await expect(page.locator(".asd-posture-tech")).toHaveAttribute("open", "");
    await expect(bar(page).getByTestId("asd-posture-tree")).toHaveAttribute("data-dirty", "dirty");
    await expect(page.getByTestId("asd-posture-accepted-branch")).toBeVisible();

    // With JavaScript disabled, one activation of the posture text reveals
    // the full posture: a native <details>, so the summary click is the
    // browser's own toggle, no script involved. Each fact's test-id
    // element carries exactly the text today's row showed; a disclosed
    // reason the row did not print sits beside it (SI-332 (2)).
    // The checkout fact's text: on the authoring wall, the checkout the
    // snapshot's expected identity names; on the read-only fixture, whose
    // snapshot names no expected checkout, the isolated store the
    // fixture's serve runs on — an absolute path ending in its store.
    const designCheckout = await checkoutOf(page, design);
    expect(designCheckout, "the authoring wall's snapshot names its checkout").toMatch(/^\//);
    const noJS = await browser.newContext({ javaScriptEnabled: false });
    try {
      const quiet = await noJS.newPage();
      await quiet.goto(unproven);
      const details = quiet.locator(".asd-posture-tech");
      await expect(details).toHaveCount(1);
      await expect(details).not.toHaveAttribute("open", "");
      await quiet.getByTestId("topbar-posture").click();
      await expect(details).toHaveAttribute("open", "");
      const unprovenCheckout = await postureFact(quiet, "checkout");
      expect(unprovenCheckout.state).toBe("proven");
      expect(unprovenCheckout.text).toMatch(/^\/.+\/store$/);
      const branch = await postureFact(quiet, "branch");
      expect(branch.state).toBe("proven");
      expect(branch.text).not.toBe("");
      const head = await postureFact(quiet, "worktree-head");
      expect(head.state).toBe("proven");
      expect(head.text).toMatch(/^[0-9a-f]{40}$/);
      expect(await postureFact(quiet, "accepted-branch")).toEqual({ text: "unproven", state: "unproven" });
      expect(await postureWhy(quiet, "accepted-branch")).toBe("the default branch could not be resolved");
      expect(await postureFact(quiet, "accepted-head")).toEqual({ text: "unproven", state: "unproven" });
      expect(await postureWhy(quiet, "accepted-head")).toMatch(/could not be resolved/);
      // Today's row printed ahead/behind's reason itself; no sibling repeats it.
      expect(await postureFact(quiet, "ahead-behind")).toEqual({
        text: "unproven: the accepted branch could not be resolved",
        state: "unproven",
      });
      expect(await postureWhy(quiet, "ahead-behind")).toBeNull();

      // The design wall proves every one of them.
      await quiet.goto(WORKBENCH + design);
      await quiet.getByTestId("topbar-posture").click();
      await expect(quiet.locator(".asd-posture-tech")).toHaveAttribute("open", "");
      expect(await postureFact(quiet, "checkout")).toEqual({ text: designCheckout, state: "proven" });
      expect(await postureFact(quiet, "branch")).toEqual({ text: SHOWCASE.DESIGN_BRANCH, state: "proven" });
      expect((await postureFact(quiet, "worktree-head")).text).toMatch(/^[0-9a-f]{40}$/);
      expect(await postureFact(quiet, "accepted-branch")).toEqual({ text: SHOWCASE.MAIN_BRANCH, state: "proven" });
      expect((await postureFact(quiet, "accepted-head")).text).toMatch(/^[0-9a-f]{40}$/);
      const ab = await postureFact(quiet, "ahead-behind");
      expect(ab.state).toBe("proven");
      expect(ab.text).toMatch(new RegExp(`^\\d+ ahead, \\d+ behind ${SHOWCASE.MAIN_BRANCH}$`));
      for (const slug of ["checkout", "branch", "worktree-head", "accepted-branch", "accepted-head", "ahead-behind"]) {
        expect(await postureWhy(quiet, slug), `${slug}: a proven fact carries no reason`).toBeNull();
      }
    } finally {
      await noJS.close();
    }

    // The words are painted, never clipped, at every width from 320 px
    // (closure ruling): the box of the bytes word and of the tree word
    // lies inside the summary's box and the viewport, hit-tested, on
    // every wall kind and the Document page, with script on and off —
    // at the ruling's widths, at 1280, and on both sides of the 640 px
    // breakpoint.
    const sealedRemote = branchBoardPath(`design/${SHOWCASE.DB_SEALED_REMOTE}`, SHOWCASE.DB_SEALED_REMOTE);
    const walls = [
      design,
      design + "/document",
      boardPath(SHOWCASE.REVIEW_SPEC),
      boardPath(SHOWCASE.READONLY_SPEC),
      boardPath(SHOWCASE.SUPERSEDED_FEATURE_SPEC),
      branchBoardPath(SHOWCASE.SHOWCASE_DRAFT_BRANCH, SHOWCASE.SHOWCASE_DRAFT_SPEC),
      sealedRemote,
      unproven,
    ];
    for (const scripted of [true, false]) {
      const ctx = await browser.newContext({ javaScriptEnabled: scripted });
      try {
        const p = await ctx.newPage();
        for (const width of [1440, 1280, 1024, 800, 640, 639, 320]) {
          await p.setViewportSize({ width, height: 900 });
          for (const path of walls) {
            await p.goto(path.startsWith("http") ? path : WORKBENCH + path);
            await expectWordsPainted(p, `${path.replace(/^https?:\/\/[^/]+/, "")} @${width} script ${scripted ? "on" : "off"}`);
          }
        }
      } finally {
        await ctx.close();
      }
    }
  });

  test("The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget", async ({
    page,
    browser,
  }) => {
    test.setTimeout(240_000);
    // 320 px: no horizontal scroll from the bar, every bar control visible.
    await page.setViewportSize({ width: 320, height: 800 });
    for (const p of PAGES) {
      await gotoPage(page, p);
      await expectBarFits(page, p, "at320", `${p.name} @320`);
      await expectBarControlsVisible(page, `${p.name} @320`);
    }
    // 200 % zoom at a laptop width: the same.
    await page.setViewportSize({ width: 1280, height: 800 });
    for (const p of PAGES) {
      await gotoPage(page, p);
      await page.evaluate(() => {
        (document.body.style as unknown as { zoom: string }).zoom = "200%";
      });
      await expectBarFits(page, p, "at200", `${p.name} @200%`);
      await expectBarControlsVisible(page, `${p.name} @200%`);
    }

    // The bar is one row at desktop widths (handoff "Global chrome"): at
    // 1440 on every page, and on every wall from 1280 up (spec/wall-strip-
    // and-drawer-v2, F3a closure N1) — the walls of this list and the
    // other rooms' and change states' walls, the busiest bars the fixtures
    // hold — with no bar control pushed past the viewport.
    const walls = [
      ...PAGES.filter((p) => p.spec && p.name !== "Document page"),
      { name: "story wall", path: () => boardPath(SHOWCASE.EMPTY_SPEC), spec: true, title: "" },
      { name: "sealed record wall", path: () => boardPath(SHOWCASE.READONLY_SPEC), spec: true, title: "" },
      { name: "sealed story wall", path: () => boardPath(SHOWCASE.STORY_WITH_SPEC_STALE), spec: true, title: "" },
      { name: "badged sealed wall", path: () => boardPath(EDGE.BADGE_SEALED_SPEC), spec: true, title: "" },
      { name: "typed changes wall", path: () => TYPED_CHANGES_WALL, spec: true, title: "" },
    ];
    for (const width of [1280, 1366, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      for (const p of width === 1440 ? [...PAGES, ...walls.slice(-5)] : walls) {
        await gotoPage(page, p);
        const row = await bar(page).evaluate((el) => {
          const vw = document.documentElement.clientWidth;
          const past = Array.from(el.querySelectorAll<HTMLElement>("a[href],button,summary"))
            .filter((c) => {
              const r = c.getBoundingClientRect();
              return r.width > 0 && (r.left < -1 || r.right > vw + 1);
            })
            .map((c) => c.getAttribute("data-testid") || c.id || c.className);
          return { h: el.querySelector(".topbar-row")!.getBoundingClientRect().height, past };
        });
        console.log(`bar height: ${p.name} @${width} = ${row.h} px`);
        expect(row.h, `${p.name} @${width}: one 52 px row`).toBeLessThanOrEqual(56);
        expect(row.past, `${p.name} @${width}: bar controls past the viewport`).toEqual([]);
      }
    }

    // The sealed wall's New story and Revise open their dialogs from a
    // press anywhere on them: the wall's script dispatches on the pressed
    // element's id, so the glyph and the words after the verb, each in its
    // own span, never take the pointer. At 1440 those spans are folded
    // away from the eye, the names stay whole, and the press lands on the
    // visible verb; at the project's 1880 px they show, and the press
    // lands on each of them.
    const sealedWall = PAGES.find((p) => p.name === "sealed feature wall")!;
    const sealedActions = [
      { testid: "create-spec-btn", dialog: "#create-dialog", cancel: "#create-cancel", name: "New story", verb: "New" },
      { testid: "revise-spec-btn", dialog: "#revise-dialog", cancel: "#revise-cancel", name: "Revise this feature", verb: "Revise" },
    ];
    const pressAt = async (at: { x: number; y: number }, a: (typeof sealedActions)[number], where: string) => {
      await page.mouse.click(at.x, at.y);
      await expect(page.locator(a.dialog), `${a.name}: a press on ${where} opens its dialog`).toBeVisible();
      await page.locator(a.cancel).click();
      await expect(page.locator(a.dialog)).toBeHidden();
    };
    await gotoPage(page, sealedWall);
    for (const a of sealedActions) {
      const btn = bar(page).getByTestId(a.testid);
      await expect(btn).toHaveAccessibleName(a.name);
      for (const part of [".wall-action-glyph", ".wall-action-rest"]) {
        const w = await btn.locator(part).evaluate((el) => el.getBoundingClientRect().width);
        expect(w, `${a.name} @1440: ${part} folded away from the eye`).toBeLessThanOrEqual(1);
      }
      const verbAt = await btn.evaluate((el, verb) => {
        const node = Array.from(el.childNodes).find((n) => n.nodeType === Node.TEXT_NODE && n.textContent?.trim() === verb);
        if (!node) return null;
        const range = document.createRange();
        range.selectNodeContents(node);
        const r = range.getBoundingClientRect();
        return { x: r.x + r.width / 2, y: r.y + r.height / 2 };
      }, a.verb);
      expect(verbAt, `${a.name} @1440: the visible verb "${a.verb}"`).not.toBeNull();
      await pressAt(verbAt!, a, `its visible verb @1440`);
    }
    await page.setViewportSize({ width: 1880, height: 1000 });
    await gotoPage(page, sealedWall);
    for (const a of sealedActions) {
      const btn = bar(page).getByTestId(a.testid);
      await expect(btn).toHaveAccessibleName(a.name);
      for (const part of [".wall-action-glyph", ".wall-action-rest"]) {
        const r = await btn.locator(part).evaluate((el) => {
          const b = el.getBoundingClientRect();
          return { x: b.x + b.width / 2, y: b.y + b.height / 2, w: b.width };
        });
        expect(r.w, `${a.name} @1880: ${part} shows`).toBeGreaterThan(1);
        await pressAt(r, a, `${part} @1880`);
      }
    }
    await page.setViewportSize({ width: 1440, height: 900 });

    // Without JavaScript: the bar is in the initial server response, and a
    // script-less browser renders it.
    const sizes: Record<string, number> = {};
    for (const p of PAGES) {
      const resp = await page.request.get(await p.path(page));
      const body = await resp.text();
      sizes[p.name] = body.length;
      expect(body, `${p.name}: bar in the initial response`).toContain('data-testid="topbar"');
    }
    const noJS = await browser.newContext({ javaScriptEnabled: false });
    try {
      const quiet = await noJS.newPage();
      await quiet.goto(WORKBENCH + boardPath(SHOWCASE.DESIGN_SPEC));
      await expect(quiet.locator(TOPBAR)).toHaveCount(1);
      await expect(quiet.getByTestId("topbar-title")).toBeVisible();
      await expect(quiet.getByTestId("topbar-posture")).toBeVisible();
    } finally {
      await noJS.close();
    }

    // Keyboard: Tab runs through the bar's controls in reading (DOM)
    // order, on the wall and on the Document page.
    for (const path of [boardPath(SHOWCASE.DESIGN_SPEC), boardPath(SHOWCASE.DESIGN_SPEC) + "/document"]) {
      await page.goto(path);
      const order = await barControls(page);
      expect(order.length, `${path}: bar controls`).toBeGreaterThan(2);
      await bar(page).getByTestId("topbar-wordmark").focus();
      const seen: string[] = [];
      for (let i = 0; i < order.length; i++) {
        seen.push(
          await page.evaluate(() => {
            const el = document.activeElement as HTMLElement | null;
            return el ? el.getAttribute("data-testid") || el.id || el.className || el.tagName : "";
          }),
        );
        await page.keyboard.press("Tab");
      }
      expect(seen, `${path}: tab order`).toEqual(order);
    }

    // The existing axe check (50-design-workbench's wcag2a/aa scan), on
    // each page: the bar itself is clean, and the page's findings are the
    // same with the bar as without it — the bar introduces none. The
    // wall, the existing check's page, stays clean whole; a body finding
    // that predates this story (the index's badge contrast and
    // link-in-text-block, the v0 board's optional label) is logged as a
    // disclosure, never hidden.
    for (const p of PAGES) {
      await gotoPage(page, p);
      const scan = async (scope: (b: AxeBuilder) => AxeBuilder) =>
        (await scope(new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa"])).analyze()).violations.map((v) => ({
          id: v.id,
          impact: v.impact,
          nodes: v.nodes.length,
          targets: v.nodes.slice(0, 3).map((n) => n.target.join(" ")),
        }));
      const inBar = await scan((b) => b.include(TOPBAR));
      expect(inBar, `${p.name}: bar violations ${JSON.stringify(inBar, null, 2)}`).toEqual([]);
      const whole = await scan((b) => b);
      const withoutBar = await scan((b) => b.exclude(TOPBAR));
      const summary = (vs: { id: string; nodes: number }[]) => vs.map((v) => `${v.id}×${v.nodes}`).sort();
      expect(summary(whole), `${p.name}: the bar changes the page's findings ${JSON.stringify(whole, null, 2)}`).toEqual(summary(withoutBar));
      if (p.name === "wall on the default branch") {
        expect(whole, `${p.name}: ${JSON.stringify(whole, null, 2)}`).toEqual([]);
      } else if (whole.length > 0) {
        console.log(`disclosed: ${p.name}: pre-existing page findings outside the bar ${JSON.stringify(summary(whole))}`);
      }
    }

    // The Wave 6 budget the existing budget test uses (SI-168): each page's
    // HTML within 512 KiB; the route-scoped assets within 64 KiB each.
    for (const [name, size] of Object.entries(sizes)) {
      console.log(`budget: ${name} html=${size} bytes`);
      expect(size, `${name}: page budget`).toBeLessThanOrEqual(512 * 1024);
    }
    for (const asset of ["/assets/boardspecasd.js", "/assets/specdocument.js", "/assets/topbar.js"]) {
      const size = (await (await page.request.get(asset)).text()).length;
      console.log(`budget: ${asset} js=${size} bytes`);
      expect(size, `${asset}: asset budget`).toBeLessThanOrEqual(64 * 1024);
    }
  });

  test("The posture popover closes by Escape or an outside press, keeps its focus, and survives the wall's refresh", async ({
    page,
  }) => {
    test.setTimeout(150_000);
    const design = boardPath(SHOWCASE.DESIGN_SPEC);
    const details = page.locator(".asd-posture-tech");
    const summary = page.getByTestId("topbar-posture");
    const mark = () => page.evaluate(() => document.getElementById("asd-posture")!.setAttribute("data-probe", "before-swap"));
    const marked = page.locator("#asd-posture[data-probe]");

    await page.setViewportSize({ width: 1280, height: 500 });
    await page.goto(design);
    // Opened by keyboard, the summary keeps focus.
    await summary.focus();
    await page.keyboard.press("Enter");
    await expect(details).toHaveAttribute("open", "");
    expect(await activeKey(page)).toBe("topbar-posture");

    // A snapshot refresh not caused by a click swaps the posture group in
    // whole (SI-323 (3)): the mark set on the group is gone, the group is
    // current (the tree reads dirty), the disclosure stays open, and the
    // focused summary stays focused across the swap.
    await mark();
    await expect(marked).toHaveCount(1);
    await postTypedEdit(page, "the posture group is swapped by a background refresh [87-swap]");
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[1])).toContainText("[87-swap]", { timeout: 8_000 });
    await expect(marked).toHaveCount(0);
    await expect(bar(page).getByTestId("asd-posture-tree")).toHaveAttribute("data-dirty", "dirty");
    await expect(details).toHaveAttribute("open", "");
    expect(await activeKey(page)).toBe("topbar-posture");

    // A press on Refresh — inside the posture group — keeps it open; with
    // a fresh change to fetch, the refresh it asks for (or the poll tick
    // that races it) swaps the group again, and it is still open after.
    // (An unchanged wall answers a manual Refresh with 304 and swaps
    // nothing, so the change comes first.)
    await mark();
    await postTypedEdit(page, "a press on Refresh keeps the open posture open [87-refresh-press]");
    await page.getByTestId("asd-refresh").click();
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[1])).toContainText("[87-refresh-press]", { timeout: 8_000 });
    await expect(marked).toHaveCount(0, { timeout: 8_000 });
    await expect(details).toHaveAttribute("open", "");

    // Escape closes it and returns focus to the summary.
    await page.keyboard.press("Escape");
    await expect(details).not.toHaveAttribute("open", "");
    expect(await activeKey(page)).toBe("topbar-posture");

    // An outside press on something that is not focusable closes it and
    // returns focus to the summary, and the page does not scroll for it
    // (SI-331): the wall's bare cork, in view, on a scrolled page (the
    // shell's step text this test pressed is retired with the shell,
    // spec/wall-strip-and-drawer-v2 ac-6).
    await summary.click();
    await expect(details).toHaveAttribute("open", "");
    await page.evaluate(() => window.scrollTo(0, 200));
    const y0 = await page.evaluate(() => window.scrollY);
    const cork = await page.evaluate(() => {
      const c = document.getElementById("board-canvas")!;
      const r = c.getBoundingClientRect();
      const top = Math.max(r.top, 0) + 4;
      const bottom = Math.min(r.bottom, window.innerHeight) - 4;
      for (let y = top; y < bottom; y += 12) {
        for (let x = r.left + 4; x < Math.min(r.right, window.innerWidth) - 4; x += 12) {
          if (document.elementFromPoint(x, y) === c) return { x, y };
        }
      }
      return null;
    });
    expect(cork, "the wall's bare cork is on screen").not.toBeNull();
    await page.mouse.click(cork!.x, cork!.y);
    await expect(details).not.toHaveAttribute("open", "");
    await expect.poll(() => activeKey(page)).toBe("topbar-posture");
    expect(Math.abs((await page.evaluate(() => window.scrollY)) - y0)).toBeLessThanOrEqual(1);

    // A press on a focusable control closes it and keeps that control's
    // focus: an object card on the wall (focusable, and its press selects
    // it and navigates nowhere; the shell's disclosure this test pressed
    // is retired with the shell, spec/wall-strip-and-drawer-v2 ac-6).
    await summary.click();
    await expect(details).toHaveAttribute("open", "");
    const control = page.getByTestId("card-" + SHOWCASE.AC_IDS[0]);
    await control.scrollIntoViewIfNeeded();
    await expect(control).toBeVisible();
    await control.click();
    await expect(details).not.toHaveAttribute("open", "");
    await expect.poll(() => activeKey(page)).toBe("card-" + SHOWCASE.AC_IDS[0]);

    // On the diagram editor, whose page-level Escape is its exit
    // (tool-view-exit ac-1), the first Escape closes only the popover and
    // stays on the editor; a second Escape exits.
    await page.goto(diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL));
    await expect(page.getByTestId("diagram-editor")).toBeVisible();
    await expect(page.locator("#diagram-preview svg")).toBeVisible();
    await summary.click();
    await expect(details).toHaveAttribute("open", "");
    await page.keyboard.press("Escape");
    await expect(details).not.toHaveAttribute("open", "");
    await page.waitForTimeout(500);
    await expect(page).toHaveURL(new RegExp(`${diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL)}$`));
    expect(await activeKey(page)).toBe("topbar-posture");
    await page.keyboard.press("Escape");
    await expect(page).toHaveURL(/\/$/);
    await expect(page.locator(".home-directory")).toBeVisible();
  });
});
