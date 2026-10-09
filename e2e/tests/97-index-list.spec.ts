import { test, expect, type Page, type Locator } from "@playwright/test";
import { SHOWCASE, EDGE, INDEX_FAILURE_FIXTURE_URL, dirEntryTestId, dirGroupTestId, draftBoardHref } from "./fixtures";

// spec/index-v2 (jira:VERDI-WR-10), ac-6: a list view shows the same four
// groups with the same entries and labels, the keyboard moves between
// cards and opens one with Enter, and when the index computation fails
// the page discloses it once and neither view renders a partial or
// invented group (SI-366 (6), (7), (14), (15), (21)(f)).
//
// The one test is the producer its obligation names
// (.verdi/obligations/index-v2/ac-6--behavioral.md), titled exactly as
// the claim spells it. The file passes when run alone (BL-98). Two real
// stores back it: the shared harness store, whose cards the two views
// must agree on and whose no-draft branch is the card the arrows reach
// without a link, and the isolated failing store
// (cmd/e2eharness/indexfailure.go: an undecodable default-branch spec
// beside an intact one and a valid draft on a design branch, so a render
// that showed any partial index would have something to show; SI-366
// (15)). Shared-store assertions are by membership and by what the page
// itself renders — other lanes add design branches to the shared store,
// so no exact shared-store count or card list is pinned. State rides
// test ids, data attributes, text and geometry — never a screenshot
// (recording stays off).

const DESK = "drafts-in-progress";
const SHELF = "terminal";

function card(page: Page, name: string): Locator {
  return page.getByTestId(dirEntryTestId(name));
}
function column(page: Page, group: string): Locator {
  return page.getByTestId(dirGroupTestId(group));
}

// isolatedBase asks the control server for an isolated store's base URL.
async function isolatedBase(page: Page, url: string): Promise<string> {
  const res = await page.request.get(url);
  expect(res.ok(), await res.text()).toBe(true);
  const base = (await res.text()).trim();
  expect(base).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return base;
}

// groupSignatures reads what the page shows for every group and card —
// the labels the two views must share: each column's heading, count,
// where and move lines, empty state and archived summary, and each card's
// id, title, ref, badges and chips, every link's text and address, its
// move, its call to action and its disclosure. The age chip is read by
// its class only: its text is a wall-clock age on the shared store.
async function groupSignatures(page: Page): Promise<unknown> {
  return page.locator(".dir-group").evaluateAll((groups) =>
    groups.map((g) => ({
      id: g.getAttribute("data-testid"),
      heading: g.querySelector("h2")?.textContent?.trim() ?? null,
      where: g.querySelector(".dir-group-where")?.textContent?.trim() ?? null,
      move: g.querySelector(".dir-group-move")?.textContent?.trim() ?? null,
      notes: Array.from(g.querySelectorAll(".dir-group-note")).map((n) => n.textContent?.trim()),
      empty: g.querySelector(".dir-empty")?.textContent?.trim() ?? null,
      archived: g.querySelector("details.dir-archived > summary")?.textContent?.trim() ?? null,
      cards: Array.from(g.querySelectorAll(".dir-entry")).map((c) => ({
        id: c.getAttribute("data-testid"),
        title: c.querySelector(".dir-card-title")?.textContent?.trim() ?? null,
        ref: c.querySelector(".dir-ref")?.textContent?.trim() ?? null,
        badges: Array.from(c.querySelectorAll(".dir-meta .badge, .dir-meta .dir-unproven")).map((b) => b.textContent?.trim()),
        age: c.querySelector(".dir-age")?.className ?? null,
        links: Array.from(c.querySelectorAll("a")).map((a) => [a.textContent?.trim(), a.getAttribute("href")]),
        move: c.querySelector(".dir-move")?.textContent?.trim() ?? null,
        cta: c.querySelector(".dir-cta")?.textContent?.trim() ?? null,
        disclosed: c.querySelector(".dir-disclosed")?.textContent?.trim() ?? null,
      })),
    })),
  );
}

// columnBoxes reads each column's box, so the two views can be told apart
// by geometry: side by side in the pipeline, stacked in the list.
async function columnBoxes(page: Page): Promise<{ top: number; bottom: number; left: number }[]> {
  return page.locator(".dir-group").evaluateAll((els) =>
    els.map((el) => {
      const r = el.getBoundingClientRect();
      return { top: r.top, bottom: r.bottom, left: r.left };
    }),
  );
}

async function expectStacked(page: Page): Promise<void> {
  const boxes = await columnBoxes(page);
  expect(boxes).toHaveLength(4);
  for (let i = 1; i < boxes.length; i++) {
    expect(boxes[i].top, `column ${i} sits below column ${i - 1}`).toBeGreaterThanOrEqual(boxes[i - 1].bottom - 1);
    expect(boxes[i].left, `column ${i} shares the left edge`).toBeCloseTo(boxes[0].left, 0);
  }
}

async function expectSideBySide(page: Page): Promise<void> {
  const boxes = await columnBoxes(page);
  expect(boxes).toHaveLength(4);
  expect(boxes[1].top, "the first two columns share a row").toBeCloseTo(boxes[0].top, 0);
  expect(boxes[1].left, "the second column sits to the right").toBeGreaterThan(boxes[0].left);
}

// reachableCards lists the cards a reader can see, in document order: not
// hidden by a filter and not inside a closed fold — the cards the arrows
// must visit, each exactly once.
async function reachableCards(page: Page): Promise<string[]> {
  return page.locator(".dir-entry").evaluateAll((els) =>
    els
      .filter((el) => {
        if (el.hasAttribute("hidden")) return false;
        for (let p = el.parentElement; p; p = p.parentElement) {
          if (p.classList.contains("home-directory")) break;
          if (p.tagName === "DETAILS" && !(p as HTMLDetailsElement).open) return false;
        }
        return true;
      })
      .map((el) => el.getAttribute("data-testid") ?? ""),
  );
}

// focusPrimary focuses a card's primary link — the board link where one
// is served, else the corpus link — or the card itself when it has none.
async function focusPrimary(c: Locator): Promise<void> {
  const board = c.locator("a.dir-board");
  if ((await board.count()) > 0) {
    await board.first().focus();
    return;
  }
  const title = c.locator("a.dir-title");
  if ((await title.count()) > 0) {
    await title.first().focus();
    return;
  }
  await c.focus();
}

// focusedCard is the test id of the card whose primary link holds focus —
// its a.dir-board, else its a.dir-title, else the card itself when it has
// no link at all (SI-366 (14)) — or null when focus is in no card. Focus
// on any other element of a card reads as a witness naming that element,
// never as the card's id, so a walk that lands beside the primary link
// fails (F7CR-2).
async function focusedCard(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    const el = document.activeElement;
    const c = el?.closest(".dir-entry");
    if (!el || !c) return null;
    const id = c.getAttribute("data-testid");
    const primary = c.querySelector("a.dir-board") ?? c.querySelector("a.dir-title") ?? (c.querySelector("a") ? null : c);
    if (el === primary) return id;
    return `${id}: focus on <${el.tagName.toLowerCase()} class="${el.getAttribute("class") ?? ""}">, not its primary link`;
  });
}

// walk presses key from the current focus until focus stops moving, and
// returns every card visited, the starting one first.
async function walk(page: Page, key: "ArrowDown" | "ArrowUp" | "ArrowRight" | "ArrowLeft"): Promise<(string | null)[]> {
  const visited = [await focusedCard(page)];
  for (let i = 0; i < 500; i++) {
    await page.keyboard.press(key);
    const now = await focusedCard(page);
    if (now === visited[visited.length - 1]) break;
    visited.push(now);
  }
  return visited;
}

// expectHonestFailure asserts ac-6's failure shape on the current page:
// the index-failure notice exactly once, naming the undecodable spec, and
// no column, heading, count, filter, card list, fold or card in the
// directory — nothing partial, nothing invented — while the page is still
// the workbench and the other-records strip survives with its own
// disclosure of the same unreadable file (SI-366 (15), (21)(f)).
async function expectHonestFailure(page: Page, view: "pipeline" | "list"): Promise<void> {
  await expect(page).toHaveTitle(/Workbench/);
  const dir = page.locator(".home-directory");
  await expect(dir).toBeVisible();
  await expect(dir).toHaveAttribute("data-view", view);
  await expect(page.getByTestId("index-view-toggle").locator(`a[data-view="${view}"]`)).toHaveAttribute("aria-current", "page");
  const notice = page.locator(".dir-index-failed");
  await expect(notice).toHaveCount(1);
  await expect(notice).toBeVisible();
  await expect(notice).toContainText("Could not compute the directory index");
  await expect(notice).toContainText(EDGE.INDEX_FAILURE_UNDECODABLE);
  for (const partial of [
    ".dir-group",
    ".dir-group-head",
    ".dir-filters",
    ".dir-filter",
    ".dir-cards",
    ".dir-entry",
    ".dir-archived",
    ".dir-empty",
    ".home-directory .count",
    '[data-testid^="dir-group-"]',
    '[data-testid^="dir-entry-"]',
    '[data-testid^="glance-"]',
  ]) {
    await expect(page.locator(partial), `${partial} on the failed index`).toHaveCount(0);
  }
  await expect(page.locator(".home-strip")).toBeVisible();
  await expect(page.locator("details.home-kinds > summary")).toContainText("count unproven");
}

test("index › The list view, keyboard movement, and an honest failure", async ({ page, browser }) => {
  // (a) The list view, switched from the bar: the pipeline is drawn
  // first, its columns side by side, and the toggle names it current;
  // pressing List switches the directory to the list view in place, the
  // address records it, the toggle follows, the same four groups with the
  // same entries and labels render — every card exactly once — and the
  // groups now stack as rows (SI-366 (6), (7)).
  await page.goto("/");
  await expect(page).toHaveTitle(/Workbench/);
  const dir = page.locator(".home-directory");
  await expect(dir).toHaveAttribute("data-view", "pipeline");
  const toggle = page.getByTestId("index-view-toggle");
  await expect(toggle.locator("a")).toHaveCount(2);
  await expect(toggle.locator("button, [disabled]")).toHaveCount(0);
  await expect(toggle.locator('a[data-view="pipeline"]')).toHaveAttribute("aria-current", "page");
  await expect(toggle.locator('a[data-view="pipeline"]')).toHaveAttribute("href", "/");
  await expect(toggle.locator('a[data-view="list"]')).toHaveAttribute("href", "/?view=list");
  await expect(toggle.locator('a[data-view="list"]')).not.toHaveAttribute("aria-current", /.*/);
  const pipeline = await groupSignatures(page);
  expect((pipeline as { cards: unknown[] }[]).map((g) => g.cards.length).reduce((a, b) => a + b, 0)).toBeGreaterThan(0);
  await expectSideBySide(page);
  const feature = card(page, SHOWCASE.READONLY_SPEC);
  const stacked = async () => {
    const title = await feature.locator(".dir-card-title").boundingBox();
    const meta = await feature.locator(".dir-meta").boundingBox();
    return Boolean(title && meta && meta.y >= title.y + title.height - 1);
  };
  expect(await stacked(), "a pipeline card stacks its title above its chips").toBe(true);

  await toggle.locator('a[data-view="list"]').click();
  await expect(dir).toHaveAttribute("data-view", "list");
  await expect(page).toHaveURL(/\/\?view=list$/);
  await expect(toggle.locator('a[data-view="list"]')).toHaveAttribute("aria-current", "page");
  await expect(toggle.locator('a[data-view="pipeline"]')).not.toHaveAttribute("aria-current", /.*/);
  expect(await groupSignatures(page), "the list view's groups, entries and labels").toEqual(pipeline);
  const ids = await page.locator(".dir-entry").evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
  expect(new Set(ids).size, `repeated cards among ${ids.join(", ")}`).toBe(ids.length);
  await expectStacked(page);
  expect(await stacked(), "a list row sets its chips beside its title").toBe(false);
  await expect(toggle.locator('a[data-view="pipeline"]')).toHaveAttribute("href", "/");
  await toggle.locator('a[data-view="pipeline"]').click();
  await expect(dir).toHaveAttribute("data-view", "pipeline");
  await expect(page).toHaveURL(/\/$/);
  await expectSideBySide(page);

  // (b) ...and without JavaScript: ?view=list is honoured server-side,
  // the toggle's links lead between the views, and a view the page does
  // not know draws the pipeline and says so.
  const noJS = await browser.newContext({ javaScriptEnabled: false });
  try {
    const quiet = await noJS.newPage();
    await quiet.goto("/?view=list");
    await expect(quiet.locator(".home-directory")).toHaveAttribute("data-view", "list");
    await expect(quiet.getByTestId("index-view-toggle").locator('a[data-view="list"]')).toHaveAttribute("aria-current", "page");
    expect(await groupSignatures(quiet), "the server-rendered list view").toEqual(pipeline);
    await expectStacked(quiet);
    await quiet.getByTestId("index-view-toggle").locator('a[data-view="pipeline"]').click();
    await expect(quiet).toHaveURL(/\/$/);
    await expect(quiet.locator(".home-directory")).toHaveAttribute("data-view", "pipeline");
    await expectSideBySide(quiet);
    await quiet.goto("/?view=grid");
    await expect(quiet.locator(".home-directory")).toHaveAttribute("data-view", "pipeline");
    await expect(quiet.getByTestId("index-view-toggle").locator('a[data-view="pipeline"]')).toHaveAttribute("aria-current", "page");
    await expect(quiet.locator(".dir-entry[hidden]")).toHaveCount(0);
  } finally {
    await noJS.close();
  }

  // (c) The keyboard, in both views (SI-366 (14)): from the first card
  // Down visits every card a reader can see, in document order, each
  // once, landing on its primary link, and stops at the end; Up returns;
  // Right and Left step the same way; Tab leaves the cards (no trap).
  // The no-draft branch's card has no link, so it is focusable itself,
  // the arrows reach it, focus stays visible, and Enter does nothing
  // there; Enter on a card with a link, reached by an arrow, follows the
  // link natively. A filter's hidden cards and a closed fold's cards are
  // skipped, and join the walk once shown.
  for (const view of ["pipeline", "list"] as const) {
    await page.goto(view === "list" ? "/?view=list" : "/");
    await expect(dir).toHaveAttribute("data-view", view);
    const expected = await reachableCards(page);
    expect(expected.length).toBeGreaterThan(3);
    expect(expected).toContain(dirEntryTestId(EDGE.DIR_EMPTY_BRANCH));
    expect(expected).not.toContain(dirEntryTestId(SHOWCASE.DIR_ARCHIVED_SPEC));
    await focusPrimary(page.getByTestId(expected[0]));
    expect(await focusedCard(page)).toBe(expected[0]);
    expect(await walk(page, "ArrowDown"), `${view}: Down visits every visible card once`).toEqual(expected);
    expect(await walk(page, "ArrowUp"), `${view}: Up returns the same way`).toEqual([...expected].reverse());
    await page.keyboard.press("ArrowRight");
    expect(await focusedCard(page)).toBe(expected[1]);
    await page.keyboard.press("ArrowLeft");
    expect(await focusedCard(page)).toBe(expected[0]);
    const held = await page.evaluate(() => document.activeElement?.outerHTML ?? "");
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => document.activeElement?.outerHTML ?? ""), `${view}: Tab leaves the focused link`).not.toBe(held);
    expect(await page.evaluate(() => document.activeElement === document.body)).toBe(false);

    // The no-draft card: reached by an arrow from its neighbour, itself
    // the focused element with tabindex -1, its focus ring visible, and
    // Enter leaves the page where it is.
    const notice = card(page, EDGE.DIR_EMPTY_BRANCH);
    await expect(notice).toHaveAttribute("tabindex", "-1");
    await expect(notice.locator("a")).toHaveCount(0);
    const at = expected.indexOf(dirEntryTestId(EDGE.DIR_EMPTY_BRANCH));
    expect(at).toBeGreaterThanOrEqual(0);
    if (at > 0) {
      await focusPrimary(page.getByTestId(expected[at - 1]));
      await page.keyboard.press("ArrowDown");
    } else {
      await focusPrimary(page.getByTestId(expected[at + 1]));
      await page.keyboard.press("ArrowUp");
    }
    expect(await page.evaluate(() => document.activeElement?.getAttribute("data-testid"))).toBe(dirEntryTestId(EDGE.DIR_EMPTY_BRANCH));
    expect(await notice.evaluate((el) => getComputedStyle(el).outlineStyle)).not.toBe("none");
    const here = page.url();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(here);
    expect(await focusedCard(page)).toBe(dirEntryTestId(EDGE.DIR_EMPTY_BRANCH));

    // A hidden card is skipped: under the disclosed filter the walk is
    // exactly the visible disclosed cards; with the archived fold open
    // its cards join the walk at the shelf's end.
    await page.getByTestId("dir-filter-disclosed").click();
    const disclosedOnly = await reachableCards(page);
    expect(disclosedOnly.length).toBeGreaterThan(1);
    expect(disclosedOnly.length).toBeLessThan(expected.length);
    await focusPrimary(page.getByTestId(disclosedOnly[0]));
    expect(await walk(page, "ArrowDown"), `${view}: Down skips the filtered-out cards`).toEqual(disclosedOnly);
    await page.getByTestId("dir-filter-everything").click();
    await column(page, SHELF).locator("details.dir-archived > summary").click();
    const withArchived = await reachableCards(page);
    expect(withArchived).toContain(dirEntryTestId(SHOWCASE.DIR_ARCHIVED_SPEC));
    expect(withArchived.length).toBeGreaterThan(expected.length);
    await focusPrimary(page.getByTestId(expected[expected.length - 1]));
    expect(await walk(page, "ArrowDown"), `${view}: Down reaches the opened fold's cards`).toEqual(withArchived.slice(withArchived.indexOf(expected[expected.length - 1])));

    // Enter on a linked card reached by an arrow follows its link
    // natively: the arrow lands on the card's primary link, so Enter has
    // that link to follow (F7CR-2).
    const draft = dirEntryTestId(SHOWCASE.DIR_LOCAL_DRAFT);
    const from = withArchived.indexOf(draft);
    expect(from, `${view}: the local draft's card is reachable`).toBeGreaterThanOrEqual(0);
    if (from > 0) {
      await focusPrimary(page.getByTestId(withArchived[from - 1]));
      await page.keyboard.press("ArrowDown");
    } else {
      await focusPrimary(page.getByTestId(withArchived[from + 1]));
      await page.keyboard.press("ArrowUp");
    }
    expect(await focusedCard(page), `${view}: an arrow reaches the local draft's primary link`).toBe(draft);
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(new RegExp(draftBoardHref(SHOWCASE.DIR_LOCAL_DRAFT).replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "$"));
  }
  // The desk column names the branch with no draft in its copy, so its
  // fixed copy is true of that card too (F7BR-4).
  await page.goto("/");
  await expect(column(page, DESK).locator(".dir-group-where")).toContainText("or no draft yet");
  await expect(column(page, DESK).getByTestId("dir-group-nodraft")).toContainText("no draft spec yet");

  // (d) The honest failure, on the isolated failing store: each view,
  // reached by its address, by the toggle in place, and without
  // JavaScript, discloses the failure exactly once and draws no partial
  // or invented group (SI-366 (15), (21)(f)).
  const failing = await isolatedBase(page, INDEX_FAILURE_FIXTURE_URL);
  await page.goto(failing);
  await expectHonestFailure(page, "pipeline");
  await page.getByTestId("index-view-toggle").locator('a[data-view="list"]').click();
  await expectHonestFailure(page, "list");
  await page.goto(`${failing}?view=list`);
  await expectHonestFailure(page, "list");
  const noJSFailing = await browser.newContext({ javaScriptEnabled: false });
  try {
    const quiet = await noJSFailing.newPage();
    await quiet.goto(`${failing}?view=list`);
    await expectHonestFailure(quiet, "list");
    await quiet.goto(failing);
    await expectHonestFailure(quiet, "pipeline");
  } finally {
    await noJSFailing.close();
  }
});
