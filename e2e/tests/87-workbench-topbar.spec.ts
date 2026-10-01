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
// existing axe check and the Wave 6 page budget.
//
// Each test here is the producer its obligation names
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
  // title: the bar's page title, when the fixture pins it.
  title?: string | RegExp;
  // status: the response status the page is served with (404, 400).
  status?: number;
}

// ac-1's page list, each over the fixture stores: the wall on the default
// branch and on a design branch, the Document page, the diagram editor,
// the readiness page, the index, and every shared-layout page — matrix,
// verdict, disclosures, spec import, corpus, not found, error — plus the
// v0 board (SI-323 (4)).
const PAGES: WorkbenchPage[] = [
  { name: "wall on the default branch", path: () => boardPath(SHOWCASE.DESIGN_SPEC), spec: true },
  {
    name: "wall on a design branch",
    path: () => branchBoardPath(SHOWCASE.SHOWCASE_DRAFT_BRANCH, SHOWCASE.SHOWCASE_DRAFT_SPEC),
    spec: true,
  },
  { name: "Document page", path: () => boardPath(SHOWCASE.DESIGN_SPEC) + "/document", spec: true },
  { name: "diagram editor", path: () => diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL), spec: false },
  { name: "readiness page", path: (page) => readinessPageURL(page), spec: false, title: "Readiness" },
  { name: "index", path: () => "/", spec: false, title: "Workbench" },
  { name: "matrix", path: () => `/matrix/spec/${SHOWCASE.SLOT_WALL_SPEC}`, spec: false },
  { name: "verdict", path: () => `/verdict/spec/${SHOWCASE.READONLY_SPEC}`, spec: false },
  { name: "disclosures", path: () => "/disclosures", spec: false },
  { name: "spec import", path: () => importPagePath(), spec: false },
  {
    name: "corpus",
    path: () => `/a/spec/${SHOWCASE.READONLY_SPEC}`,
    spec: false,
    title: "Stale decline handling (fixture)",
  },
  { name: "not found", path: () => "/no-such-page-87", spec: false, title: "Not found", status: 404 },
  // A commit pin in ?spec= is refused before any store read (readiness.go):
  // a deterministic 400 on the workbench's own error shell.
  { name: "error", path: () => "/readiness?spec=no-such-spec@deadbeef", spec: false, title: "Error", status: 400 },
  { name: "v0 board", path: () => V0_BOARD, spec: false, title: `Board: STORY-1482` },
];

// The pages on the shared store the ac-5 sweep covers at 320 px, at 200 %
// zoom, without script, under axe, and within budget.
const SWEEP: WorkbenchPage[] = PAGES.filter((p) => p.name !== "readiness page");

// The board pages, whose bodies fit the viewport at 320 px and at 200 %
// zoom today (50-design-workbench proves the wall): there the whole page
// is held to no horizontal scroll. The shared-layout pages' bodies
// overflow at 320 px before this story (long unbroken code, tables, the
// index's glance grid), which is not the bar's doing and not this story's
// to change; there the proof is differential — the bar adds no overflow —
// and the body's own overflow is logged as a disclosure.
const BODY_FITS = new Set(["wall on the default branch", "wall on a design branch", "diagram editor", "v0 board"]);

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

// expectBarFits: the bar sits inside the viewport, scrolls nothing
// inside itself, and adds no horizontal overflow to the page (measured
// with the bar shown and hidden); on a page whose body fits, the whole
// page has no horizontal scroll.
async function expectBarFits(page: Page, p: WorkbenchPage, what: string): Promise<void> {
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
  if (BODY_FITS.has(p.name)) {
    expect(withBar, `${what}: horizontal overflow`).toBeLessThanOrEqual(1);
  } else if (withoutBar > 1) {
    console.log(`disclosed: ${what}: the page body overflows by ${withoutBar} px without the bar (pre-existing, not the bar's)`);
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

// postureFact reads one fact row of the opened posture: its text and the
// three-valued state its data attribute carries.
async function postureFact(page: Page, slug: string): Promise<{ text: string; state: string | null }> {
  const dd = page.getByTestId(`asd-posture-${slug}`);
  await expect(dd).toBeVisible();
  return { text: (await dd.textContent())?.trim() ?? "", state: await dd.getAttribute("data-state") };
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
  await expect(bytesEl, `${what}: bytes text`).toContainText(`displayed bytes: ${bytes.word}`);
  const treeEl = bar(page).getByTestId("asd-posture-tree");
  await expect(treeEl, `${what}: tree`).toBeVisible();
  await expect(treeEl, `${what}: tree state`).toHaveAttribute("data-dirty", tree.state);
  await expect(treeEl, `${what}: tree text`).toContainText(`working tree: ${tree.text}`);
}

test.describe("chrome-and-tokens", () => {
  test("One top bar on every workbench page, and none of the old header rows", async ({ page }) => {
    test.setTimeout(120_000);
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
      // The page title.
      const title = topbar.getByTestId("topbar-title");
      await expect(title, `${p.name}: title`).toBeVisible();
      await expect(title, `${p.name}: title text`).not.toHaveText("");
      if (p.title) await expect(title, `${p.name}: title`).toHaveText(p.title);
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
  });

  test("The bar keeps the displayed bytes and clean or dirty state, with the full posture one action away", async ({
    page,
    browser,
  }) => {
    test.setTimeout(120_000);
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
    const snap = await (await page.request.get(design + "/snapshot")).json();
    const mutate = await page.request.post(design + "/api/mutate_draft", {
      data: {
        request: {
          schema: "verdi.draftmutation/v1",
          spec: "spec/" + SHOWCASE.DESIGN_SPEC,
          base_digest: snap.base_digest,
          base_spec_b64: snap.base_spec_b64,
          expected: snap.expected,
          operations: [
            {
              op: "edit-ac",
              id: SHOWCASE.AC_IDS[1],
              text: "the bar states the tree's state honestly [87-dirty]",
              evidence: ["attestation"],
              anchor: "#" + SHOWCASE.AC_IDS[1],
            },
          ],
        },
      },
    });
    expect(mutate.status(), await mutate.text()).toBe(200);
    expect((await mutate.json()).result, "the typed mutation landed").toBeTruthy();
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
    const refreshSnap = await (await page.request.get(design + "/snapshot")).json();
    const refreshMutate = await page.request.post(design + "/api/mutate_draft", {
      data: {
        request: {
          schema: "verdi.draftmutation/v1",
          spec: "spec/" + SHOWCASE.DESIGN_SPEC,
          base_digest: refreshSnap.base_digest,
          base_spec_b64: refreshSnap.base_spec_b64,
          expected: refreshSnap.expected,
          operations: [
            {
              op: "edit-ac",
              id: SHOWCASE.AC_IDS[1],
              text: "a background refresh keeps the open posture open [87-refresh]",
              evidence: ["attestation"],
              anchor: "#" + SHOWCASE.AC_IDS[1],
            },
          ],
        },
      },
    });
    expect(refreshMutate.status(), await refreshMutate.text()).toBe(200);
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[1])).toContainText("[87-refresh]", { timeout: 8_000 });
    await expect(page.locator(".asd-posture-tech")).toHaveAttribute("open", "");
    await expect(bar(page).getByTestId("asd-posture-tree")).toHaveAttribute("data-dirty", "dirty");
    await expect(page.getByTestId("asd-posture-accepted-branch")).toBeVisible();

    // With JavaScript disabled, one activation of the posture text reveals
    // the full posture: a native <details>, so the summary click is the
    // browser's own toggle, no script involved.
    const noJS = await browser.newContext({ javaScriptEnabled: false });
    try {
      const quiet = await noJS.newPage();
      await quiet.goto(unproven);
      const details = quiet.locator(".asd-posture-tech");
      await expect(details).toHaveCount(1);
      await expect(details).not.toHaveAttribute("open", "");
      await quiet.getByTestId("topbar-posture").click();
      await expect(details).toHaveAttribute("open", "");
      // Proven facts carry their text; the facts this store cannot prove
      // are disclosed-unproven with the row's own reason.
      expect((await postureFact(quiet, "checkout")).state).toBe("proven");
      const branch = await postureFact(quiet, "branch");
      expect(branch.state).toBe("proven");
      expect(branch.text).not.toBe("");
      const head = await postureFact(quiet, "worktree-head");
      expect(head.state).toBe("proven");
      expect(head.text).toMatch(/^[0-9a-f]{40}$/);
      const acceptedBranch = await postureFact(quiet, "accepted-branch");
      expect(acceptedBranch.state).toBe("unproven");
      expect(acceptedBranch.text).toBe("unproven: the default branch could not be resolved");
      const acceptedHead = await postureFact(quiet, "accepted-head");
      expect(acceptedHead.state).toBe("unproven");
      expect(acceptedHead.text).toMatch(/^unproven: /);
      const aheadBehind = await postureFact(quiet, "ahead-behind");
      expect(aheadBehind.state).toBe("unproven");
      expect(aheadBehind.text).toMatch(/^unproven: the accepted branch could not be resolved/);

      // The design wall proves every one of them.
      await quiet.goto(WORKBENCH + design);
      await quiet.getByTestId("topbar-posture").click();
      await expect(quiet.locator(".asd-posture-tech")).toHaveAttribute("open", "");
      expect((await postureFact(quiet, "checkout")).state).toBe("proven");
      expect(await postureFact(quiet, "branch")).toEqual({ text: SHOWCASE.DESIGN_BRANCH, state: "proven" });
      expect((await postureFact(quiet, "worktree-head")).text).toMatch(/^[0-9a-f]{40}$/);
      expect(await postureFact(quiet, "accepted-branch")).toEqual({ text: SHOWCASE.MAIN_BRANCH, state: "proven" });
      expect((await postureFact(quiet, "accepted-head")).text).toMatch(/^[0-9a-f]{40}$/);
      const ab = await postureFact(quiet, "ahead-behind");
      expect(ab.state).toBe("proven");
      expect(ab.text).toMatch(new RegExp(`^\\d+ ahead, \\d+ behind ${SHOWCASE.MAIN_BRANCH}$`));
    } finally {
      await noJS.close();
    }
  });

  test("The bar at 320 px and 200 % zoom, without JavaScript, by keyboard, within budget", async ({
    page,
    browser,
  }) => {
    test.setTimeout(180_000);
    // 320 px: no horizontal scroll from the bar, every bar control visible.
    await page.setViewportSize({ width: 320, height: 800 });
    for (const p of SWEEP) {
      await gotoPage(page, p);
      await expectBarFits(page, p, `${p.name} @320`);
      await expectBarControlsVisible(page, `${p.name} @320`);
    }
    // 200 % zoom at a laptop width: the same.
    await page.setViewportSize({ width: 1280, height: 800 });
    for (const p of SWEEP) {
      await gotoPage(page, p);
      await page.evaluate(() => {
        (document.body.style as unknown as { zoom: string }).zoom = "200%";
      });
      await expectBarFits(page, p, `${p.name} @200%`);
      await expectBarControlsVisible(page, `${p.name} @200%`);
    }

    // Without JavaScript: the bar is in the initial server response, and a
    // script-less browser renders it.
    const sizes: Record<string, number> = {};
    for (const p of SWEEP) {
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
    for (const p of SWEEP) {
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
    for (const asset of ["/assets/boardspecasd.js", "/assets/specdocument.js"]) {
      const size = (await (await page.request.get(asset)).text()).length;
      console.log(`budget: ${asset} js=${size} bytes`);
      expect(size, `${asset}: asset budget`).toBeLessThanOrEqual(64 * 1024);
    }
  });
});
