import { test, expect, type Page, type Locator } from "@playwright/test";
import {
  SHOWCASE,
  EDGE,
  INDEX_COLUMNS,
  EMPTY_INDEX_FIXTURE_URL,
  INDEX_DATES_FIXTURE_URL,
  FORGE_OUTAGE_URL,
  FORGE_OUTAGE_RESET_URL,
  dirEntryTestId,
  dirGroupTestId,
  draftBoardHref,
  directoryFixtureTitle,
  indexDatedTitle,
} from "./fixtures";

// spec/index-v2 (jira:VERDI-WR-10), ac-1 and ac-2: the index shows every
// spec on the default branch and every draft on a design branch exactly
// once, in four columns — On the desk, Accepted, Active components, On
// the shelf (parent workbench-redesign dc-12) — each with its heading,
// its count and an explicit empty state; and each card shows its title,
// ref, status badge, working links, age, source, the in-review chip, its
// next move and any disclosure, under the dir-group and dir-entry test
// ids (SI-366 (1), (2), (3), (13), (14), (18), (20)).
//
// The two tests are the producers their obligations name
// (.verdi/obligations/index-v2/ac-1--behavioral.md and
// ac-2--behavioral.md), titled exactly as each claim spells it. The file
// passes when run alone (BL-98). Three real stores back it: the shared
// harness store (every status this store's schema legalizes, both zones,
// and the directory fixture drafts — provision_directory.go,
// provision_board.go, examples/showcase), the isolated empty store
// (cmd/e2eharness/emptyglance.go: zero specs through the real
// refindex pipeline, so every column is empty at once; ADJ-40), and the
// isolated dated store (indexdates.go: a `verdi serve` under VERDI_NOW,
// so every age is exact). On the shared store the ages are asserted by
// pattern only: its commits carry one fixed date against the wall clock,
// and other suites may commit to the design branch mid-run. State rides
// test ids, data attributes and text — never a screenshot (recording
// stays off).
//
// The filters, the forge-unavailable chip, the other-records strip and the
// archived fold are this file's third and fifth tests (ac-3, ac-5; lane
// F7b), and the New story call to action its fourth (ac-4; lane F7c). The
// list view, the keyboard and the honest failure are 97-index-list's
// (ac-6).

// READONLY_SPEC's and DIR_CLOSED_AWAITING_ARCHIVE's `story:` tracker
// refs, the matrix and verdict addresses (SI-366 (20)).
const ACCEPTED_STORY = "jira:LOAN-1482";
const CLOSED_AWAITING_ARCHIVE_STORY = "jira:LOAN-1901";

// An age chip's copy (SI-366 (13)): today, n d ago, or quiet n d.
const AGE = /^(today|\d+ d ago|quiet \d+ d)$/;

const DESK = "drafts-in-progress";
const ACCEPTED = "accepted-pending-build";
const ACTIVE = "active-components";
const SHELF = "terminal";

function card(page: Page, name: string): Locator {
  return page.getByTestId(dirEntryTestId(name));
}
function column(page: Page, group: string): Locator {
  return page.getByTestId(dirGroupTestId(group));
}

// openArchived opens the shelf's collapsed archived fold (ac-5; SI-366 (8))
// so its cards can be seen; a page whose shelf has no archived entry has
// no fold to open.
async function openArchived(page: Page): Promise<void> {
  const summary = column(page, SHELF).locator("details.dir-archived > summary");
  if ((await summary.count()) > 0) {
    await summary.click();
  }
}

// isolatedBase asks the control server for an isolated store's base URL.
async function isolatedBase(page: Page, url: string): Promise<string> {
  const res = await page.request.get(url);
  expect(res.ok(), await res.text()).toBe(true);
  const base = (await res.text()).trim();
  expect(base).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return base;
}

// expectColumns asserts ac-1's column set on the current page: the four
// columns, in order, each visible with its identity heading, a count that
// equals the cards it holds, and an explicit empty state exactly when it
// holds none.
async function expectColumns(page: Page): Promise<void> {
  const rendered = await page
    .locator(".dir-group")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
  expect(rendered).toEqual(INDEX_COLUMNS.map(([group]) => dirGroupTestId(group)));
  for (const [group, heading] of INDEX_COLUMNS) {
    const col = column(page, group);
    await expect(col).toBeVisible();
    await expect(col.locator("h2")).toContainText(heading);
    const cards = await col.locator(".dir-entry").count();
    await expect(col.locator("h2 .count")).toHaveText(String(cards));
    if (cards === 0) {
      await expect(col.locator(".dir-empty")).toBeVisible();
      await expect(col.locator(".dir-empty")).not.toHaveText("");
    } else {
      await expect(col.locator(".dir-empty")).toHaveCount(0);
    }
  }
}

// expectIn asserts a card renders exactly once on the page, inside the
// named column.
async function expectIn(page: Page, group: string, name: string): Promise<void> {
  await expect(card(page, name)).toHaveCount(1);
  await expect(column(page, group).getByTestId(dirEntryTestId(name))).toBeVisible();
}

test("index › Four columns, every spec once, counts and empty states", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Workbench/);
  await openArchived(page);

  // (a) the four columns, in order, with their headings, counts and no
  // empty state beside cards.
  await expectColumns(page);

  // (b) every spec exactly once: no dir-entry test id repeats anywhere on
  // the page — the retired glance's second rendering is gone (dc-12).
  const ids = await page
    .locator(".dir-entry")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
  expect(ids.length).toBeGreaterThan(0);
  expect(new Set(ids).size, `repeated cards among ${ids.join(", ")}`).toBe(ids.length);
  await expect(page.getByTestId("home-glance")).toHaveCount(0);

  // (c) each fixture spec sits in its column: every design-branch draft
  // on the desk (the degenerate no-draft branch included), the accepted
  // feature, the active component, and — on the shelf — the superseded
  // component, the closed feature still in the active zone, and the
  // archive-zone feature. The archived spec never leads the page: the
  // shelf is the last column.
  for (const name of [
    SHOWCASE.DESIGN_SPEC,
    SHOWCASE.DIR_LOCAL_DRAFT,
    SHOWCASE.DIR_REMOTE_DRAFT,
    EDGE.DIR_EMPTY_BRANCH,
  ]) {
    await expectIn(page, DESK, name);
  }
  await expectIn(page, ACCEPTED, SHOWCASE.READONLY_SPEC);
  await expectIn(page, ACTIVE, SHOWCASE.NO_CASEFILE_SPEC);
  for (const name of [SHOWCASE.DIR_TERMINAL_SPEC, EDGE.DIR_CLOSED_AWAITING_ARCHIVE, SHOWCASE.DIR_ARCHIVED_SPEC]) {
    await expectIn(page, SHELF, name);
  }

  // (c2) completeness (SI-366 (22)(c)): every committed default-branch
  // spec — every class, both zones — appears exactly once, in the column
  // its status puts it in; a render that dropped a class of cards would
  // keep the counts honest and still fail here.
  for (const [name, group] of Object.entries(SHOWCASE.INDEX_COMMITTED_SPECS)) {
    await expectIn(page, group, name);
  }

  // (d) the empty state, on the isolated real store with zero specs:
  // every column renders its heading, a zero count and its explicit
  // empty state, and not one card, through the real pipeline.
  const empty = await isolatedBase(page, EMPTY_INDEX_FIXTURE_URL);
  await page.goto(empty);
  await expectColumns(page);
  for (const [group] of INDEX_COLUMNS) {
    await expect(column(page, group).locator("h2 .count")).toHaveText("0");
    await expect(column(page, group).locator(".dir-empty")).toBeVisible();
  }
  await expect(page.locator(".dir-entry")).toHaveCount(0);
});

test("index › Each card's facts, links, and test ids", async ({ page }) => {
  await page.goto("/");
  await openArchived(page);

  // The default-branch feature with stories: its title links its corpus
  // page (the card's first link), then the ref, the badge, every working
  // link — board, matrix and verdict — the age, the source, no in-review
  // chip (a default-branch spec has no branch in review), its next move,
  // and no disclosure.
  const feature = card(page, SHOWCASE.READONLY_SPEC);
  await expectIn(page, ACCEPTED, SHOWCASE.READONLY_SPEC);
  await expect(feature.locator("a.dir-title")).toHaveText(SHOWCASE.READONLY_SPEC_TITLE);
  await expect(feature.locator("a.dir-title")).toHaveAttribute("href", `/a/spec/${SHOWCASE.READONLY_SPEC}`);
  await expect(feature.locator("a").first()).toHaveAttribute("href", `/a/spec/${SHOWCASE.READONLY_SPEC}`);
  await expect(feature.locator(".dir-ref")).toHaveText(`spec/${SHOWCASE.READONLY_SPEC}`);
  await expect(feature.locator(".badge-accepted-pending-build")).toHaveText("accepted-pending-build");
  await expect(feature.locator("a.dir-board")).toHaveAttribute("href", `/board/spec/${SHOWCASE.READONLY_SPEC}`);
  await expect(feature.locator(`a[href="/matrix/${ACCEPTED_STORY}"]`)).toHaveCount(1);
  await expect(feature.locator(`a[href="/verdict/${ACCEPTED_STORY}"]`)).toHaveCount(1);
  await expect(feature.locator(".dir-age")).toHaveText(AGE);
  await expect(feature.locator(".badge-src")).toHaveText("default branch");
  await expect(feature.locator(".dir-inreview")).toHaveCount(0);
  await expect(feature.locator(".dir-move")).toHaveText("→ sealed wall");
  await expect(feature.locator(".dir-disclosed, .dir-unproven")).toHaveCount(0);
  await expect(feature).toHaveAttribute("data-source", "default");
  await expect(feature).toHaveAttribute("data-review", "not-open");
  await expect(feature).toHaveAttribute("data-disclosed", "false");

  // The active component: a board link, never matrix or verdict (SI-366
  // (20): those belong to a feature with a `story:` field), obligations.
  const component = card(page, SHOWCASE.NO_CASEFILE_SPEC);
  await expectIn(page, ACTIVE, SHOWCASE.NO_CASEFILE_SPEC);
  await expect(component.locator("a.dir-title")).toHaveText(SHOWCASE.NO_CASEFILE_SPEC_TITLE);
  await expect(component.locator("a.dir-title")).toHaveAttribute("href", `/a/spec/${SHOWCASE.NO_CASEFILE_SPEC}`);
  await expect(component.locator(".dir-ref")).toHaveText(`spec/${SHOWCASE.NO_CASEFILE_SPEC}`);
  await expect(component.locator(".badge-active")).toHaveText("active");
  await expect(component.locator("a.dir-board")).toHaveAttribute("href", `/board/spec/${SHOWCASE.NO_CASEFILE_SPEC}`);
  await expect(component.locator('a[href^="/matrix/"]')).toHaveCount(0);
  await expect(component.locator('a[href^="/verdict/"]')).toHaveCount(0);
  await expect(component.locator(".dir-age")).toHaveText(AGE);
  await expect(component.locator(".badge-src")).toHaveText("default branch");
  await expect(component.locator(".dir-move")).toHaveText("→ obligations");
  await expect(component.locator(".dir-disclosed, .dir-unproven")).toHaveCount(0);

  // The archived spec: title and corpus link, its closed badge, no board
  // link (dc-3: only addresses the routing serves), so no matrix or
  // verdict either, its age and source, and no next move (SI-366 (2)).
  const archived = card(page, SHOWCASE.DIR_ARCHIVED_SPEC);
  await expectIn(page, SHELF, SHOWCASE.DIR_ARCHIVED_SPEC);
  await expect(archived.locator("a.dir-title")).toHaveText(SHOWCASE.DIR_ARCHIVED_TITLE);
  await expect(archived.locator("a.dir-title")).toHaveAttribute("href", `/a/spec/${SHOWCASE.DIR_ARCHIVED_SPEC}`);
  await expect(archived.locator(".badge-closed")).toHaveText("closed");
  await expect(archived.locator("a.dir-board")).toHaveCount(0);
  await expect(archived.locator('a[href^="/matrix/"]')).toHaveCount(0);
  await expect(archived.locator('a[href^="/verdict/"]')).toHaveCount(0);
  await expect(archived.locator(".dir-age")).toHaveText(AGE);
  await expect(archived.locator(".badge-src")).toHaveText("default branch");
  await expect(archived.locator(".dir-move")).toHaveCount(0);
  // Its ref is shown whether or not its board is servable, and a proven
  // archived spec carries no disclosure (SI-366 (22)(c)).
  await expect(archived.locator(".dir-ref")).toHaveText(`spec/${SHOWCASE.DIR_ARCHIVED_SPEC}`);
  await expect(archived.locator(".dir-disclosed, .dir-unproven")).toHaveCount(0);
  await expect(archived).toHaveAttribute("data-disclosed", "false");

  // The superseded component still in the active zone: see successor,
  // linked to the spec whose supersedes edge names it, through the one
  // corpus index built per render.
  const superseded = card(page, SHOWCASE.DIR_TERMINAL_SPEC);
  await expectIn(page, SHELF, SHOWCASE.DIR_TERMINAL_SPEC);
  await expect(superseded.locator(".badge-superseded")).toHaveText("superseded");
  await expect(superseded.locator(".dir-move")).toHaveText("→ see successor");
  await expect(superseded.locator(".dir-move a")).toHaveAttribute("href", `/a/spec/${SHOWCASE.DIR_TERMINAL_SUCCESSOR}`);

  // The closed feature still in the active zone carries the projector's
  // compatibility disclosure beside its proven badge, matrix and verdict
  // for its story, and the archive move.
  const closed = card(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE);
  await expectIn(page, SHELF, EDGE.DIR_CLOSED_AWAITING_ARCHIVE);
  await expect(closed.locator(".badge-closed")).toHaveText("closed");
  await expect(closed.locator(".dir-disclosed")).toContainText("compatibility reading");
  await expect(closed).toHaveAttribute("data-disclosed", "true");
  await expect(closed.locator(`a[href="/matrix/${CLOSED_AWAITING_ARCHIVE_STORY}"]`)).toHaveCount(1);
  await expect(closed.locator(`a[href="/verdict/${CLOSED_AWAITING_ARCHIVE_STORY}"]`)).toHaveCount(1);
  await expect(closed.locator(".dir-move")).toHaveText("→ archive");

  // The local and the remote-tracking drafts: the decoded title is the
  // card's one link, its per-branch board address (SI-366 (1), dc-3);
  // then the ref, the draft badge, the source, the age, no in-review
  // chip, open the wall, and no corpus link, matrix or verdict.
  for (const [name, source] of [
    [SHOWCASE.DIR_LOCAL_DRAFT, "local branch"],
    [SHOWCASE.DIR_REMOTE_DRAFT, "remote-tracking"],
  ] as const) {
    const draft = card(page, name);
    await expectIn(page, DESK, name);
    await expect(draft.locator("a.dir-board")).toHaveText(directoryFixtureTitle(name));
    await expect(draft.locator("a.dir-board")).toHaveAttribute("href", draftBoardHref(name));
    await expect(draft.locator("a")).toHaveCount(1);
    await expect(draft.locator(".dir-ref")).toHaveText(`spec/${name}`);
    await expect(draft.locator(".badge-draft")).toHaveText("draft");
    await expect(draft.locator(".badge-src")).toHaveText(source);
    await expect(draft.locator(".dir-age")).toHaveText(AGE);
    await expect(draft.locator(".dir-inreview")).toHaveCount(0);
    await expect(draft.locator(".dir-move")).toHaveText("→ open the wall");
    await expect(draft.locator(".dir-disclosed, .dir-unproven")).toHaveCount(0);
    await expect(draft).toHaveAttribute("data-review", "not-open");
    await expect(draft).toHaveAttribute("data-disclosed", "false");
  }

  // The draft whose branch has an open pull request — and only that one
  // — carries the in-review chip, and its next move is awaiting merge.
  const inReview = card(page, SHOWCASE.DIR_INREVIEW_SPEC);
  await expectIn(page, DESK, SHOWCASE.DIR_INREVIEW_SPEC);
  await expect(inReview.locator("a.dir-board")).toHaveText(SHOWCASE.DIR_INREVIEW_TITLE);
  await expect(inReview.locator("a.dir-board")).toHaveAttribute("href", draftBoardHref(SHOWCASE.DIR_INREVIEW_SPEC));
  await expect(inReview.locator(".badge-src")).toHaveText("local + remote");
  await expect(inReview.locator(".dir-inreview")).toHaveText(SHOWCASE.DIR_INREVIEW_CHIP);
  await expect(inReview.locator(".dir-move")).toHaveText("→ awaiting merge");
  await expect(inReview).toHaveAttribute("data-review", "open");
  await expect(page.locator(".dir-inreview")).toHaveCount(1);

  // The branch with no draft spec: its disclosure names the branch, the
  // card carries no link at all, its move is inspect the branch, and it
  // reads disclosed.
  const notice = card(page, EDGE.DIR_EMPTY_BRANCH);
  await expectIn(page, DESK, EDGE.DIR_EMPTY_BRANCH);
  await expect(notice.locator(".dir-ref")).toHaveText(`spec/${EDGE.DIR_EMPTY_BRANCH}`);
  await expect(notice.locator(".dir-disclosed")).toBeVisible();
  await expect(notice.locator(".dir-disclosed")).toContainText(`design/${EDGE.DIR_EMPTY_BRANCH}`);
  await expect(notice.locator("a")).toHaveCount(0);
  await expect(notice.locator(".dir-age")).toHaveText(AGE);
  await expect(notice.locator(".dir-move")).toHaveText("→ inspect the branch");
  await expect(notice).toHaveAttribute("data-disclosed", "true");

  // Exact ages, on the isolated dated store (SI-366 (13)): each card's
  // age is the known number of days before that serve's fixed clock,
  // quiet past the fourteen-day boundary (exclusive) on the desk only;
  // the dated drafts' titles are the ones decoded from their content
  // through the real pipeline; and with no forge configured there is no
  // in-review chip and no unavailable notice.
  const dated = await isolatedBase(page, INDEX_DATES_FIXTURE_URL);
  await page.goto(dated);
  await expectColumns(page);
  // Every column's exact card list, in document order (SI-366 (22)(c)):
  // 4 / 0 / 2 / 0 — nothing dropped, nothing invented, nothing moved.
  for (const [group, names] of Object.entries(SHOWCASE.INDEX_DATED_COLUMNS)) {
    const rendered = await column(page, group)
      .locator(".dir-entry")
      .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
    expect(rendered, `${group}'s cards`).toEqual(names.map((name) => dirEntryTestId(name)));
  }
  for (const [name, age] of Object.entries(SHOWCASE.INDEX_DATED_AGES)) {
    const c = card(page, name);
    await expect(c).toHaveCount(1);
    await expect(c.locator(".dir-age")).toHaveText(age);
    if (age.startsWith("quiet")) {
      await expect(c.locator(".dir-age")).toHaveClass(/dir-age-quiet/);
      await expect(c).toHaveAttribute("data-quiet", "true");
    } else {
      await expect(c.locator(".dir-age")).not.toHaveClass(/dir-age-quiet/);
    }
  }
  for (const name of [SHOWCASE.INDEX_DATED_DRAFT_13, SHOWCASE.INDEX_DATED_DRAFT_14, SHOWCASE.INDEX_DATED_DRAFT_15]) {
    await expectIn(page, DESK, name);
    await expect(card(page, name).locator("a.dir-board")).toHaveText(indexDatedTitle(name));
    await expect(card(page, name).locator("a.dir-board")).toHaveAttribute("href", draftBoardHref(name));
  }
  await expect(card(page, SHOWCASE.INDEX_DATED_DRAFT_13)).toHaveAttribute("data-quiet", "false");
  await expect(card(page, SHOWCASE.INDEX_DATED_DRAFT_14)).toHaveAttribute("data-quiet", "false");
  await expectIn(page, DESK, SHOWCASE.INDEX_DATED_DESK_COMPONENT);
  await expect(card(page, SHOWCASE.INDEX_DATED_DESK_COMPONENT).locator("a.dir-title")).toHaveText(
    indexDatedTitle(SHOWCASE.INDEX_DATED_DESK_COMPONENT),
  );
  for (const name of [SHOWCASE.INDEX_DATED_LANDED, SHOWCASE.INDEX_DATED_EDITED]) {
    await expectIn(page, ACTIVE, name);
    await expect(card(page, name)).not.toHaveAttribute("data-quiet", /.*/);
  }
  await expect(page.locator(".dir-inreview")).toHaveCount(0);
  await expect(page.getByTestId("mr-status-unavailable")).toHaveCount(0);
});

// The filter pills, in the row's order (ac-3; SI-366 (6)).
const FILTERS = ["everything", "quiet", "in-review", "disclosed"] as const;

function pill(page: Page, id: (typeof FILTERS)[number]): Locator {
  return page.getByTestId(`dir-filter-${id}`);
}

// Shared-store assertions in the two tests below are by membership and
// by counts relative to the cards actually rendered: other lanes add
// design branches to the shared store, so no exact shared-store count or
// card list is pinned. Exact lists belong to the isolated stores.

test("index › On the shelf, the other-records strip, and the kept directory notices", async ({ page, browser }) => {
  await page.goto("/");

  // (a) On the shelf: the terminal specs still in the active zone are
  // cards in the column's body; every archive-zone spec sits in the
  // collapsed archived fold at the column's foot, its count equal to the
  // cards it holds, and the column's own count includes them (SI-366
  // (8)). Opening the fold shows the archived card; the shelf is the
  // last column (ac-1), so archived specs never lead the page.
  const shelf = column(page, SHELF);
  const fold = shelf.locator("details.dir-archived");
  await expect(fold).toHaveCount(1);
  await expect(fold).not.toHaveAttribute("open", /.*/);
  expect(await shelf.evaluate((el) => el.lastElementChild?.classList.contains("dir-archived"))).toBe(true);
  for (const name of [SHOWCASE.DIR_TERMINAL_SPEC, EDGE.DIR_CLOSED_AWAITING_ARCHIVE]) {
    await expect(shelf.locator("> .dir-cards").getByTestId(dirEntryTestId(name))).toBeVisible();
    await expect(fold.getByTestId(dirEntryTestId(name))).toHaveCount(0);
  }
  for (const name of SHOWCASE.INDEX_ARCHIVED_SPECS) {
    await expect(card(page, name)).toHaveCount(1);
    await expect(fold.getByTestId(dirEntryTestId(name))).toHaveCount(1);
    await expect(card(page, name)).toBeHidden();
  }
  const folded = await fold.locator(".dir-entry").count();
  const inBody = await shelf.locator("> .dir-cards > .dir-entry").count();
  expect(folded).toBeGreaterThan(0);
  await expect(fold.locator("summary .count")).toHaveText(String(folded));
  await expect(shelf.locator("h2 .count")).toHaveText(String(inBody + folded));
  await fold.locator("summary").click();
  await expect(fold).toHaveAttribute("open", "");
  await expect(card(page, SHOWCASE.DIR_ARCHIVED_SPEC)).toBeVisible();
  await expect(card(page, SHOWCASE.DIR_ARCHIVED_SPEC).locator("a.dir-board")).toHaveCount(0);

  // (b) The other-records strip, below the columns: the other kinds, the
  // services and the boards, each a collapsed fold under its kept class,
  // its summary carrying the count of what it lists, its listing hidden
  // until opened.
  const strip = page.locator(".home-strip");
  await expect(strip).toBeVisible();
  expect(
    await page.evaluate(() => {
      const columns = document.querySelector(".dir-columns");
      const s = document.querySelector(".home-strip");
      return Boolean(columns && s && columns.compareDocumentPosition(s) & Node.DOCUMENT_POSITION_FOLLOWING);
    }),
  ).toBe(true);
  for (const cls of ["home-kinds", "home-services", "home-boards"]) {
    const section = strip.locator(`details.${cls}`);
    await expect(section).toHaveCount(1);
    await expect(section).toBeVisible();
    await expect(section).not.toHaveAttribute("open", /.*/);
    const items = await section.locator("li").count();
    expect(items, `${cls} lists something`).toBeGreaterThan(0);
    await expect(section.locator("summary .count")).toHaveText(String(items));
    await expect(section.locator("li").first()).toBeHidden();
  }
  expect(await strip.locator('details.home-kinds a[href^="/a/adr/"]').count()).toBeGreaterThan(0);
  await expect(strip.locator("details.home-services")).toContainText("svcfix");
  await expect(strip.locator('details.home-boards a[href="/board/STORY-1482"]')).toHaveCount(1);

  // ...each expandable to its full listing WITHOUT JavaScript, as is the
  // archived fold; and before any script runs every card is shown.
  const noJS = await browser.newContext({ javaScriptEnabled: false });
  try {
    const quiet = await noJS.newPage();
    await quiet.goto("/");
    expect(await quiet.locator(".dir-entry").count()).toBeGreaterThan(0);
    await expect(quiet.locator(".dir-entry[hidden]")).toHaveCount(0);
    for (const cls of ["home-kinds", "home-services", "home-boards"]) {
      const section = quiet.locator(`details.${cls}`);
      await expect(section).not.toHaveAttribute("open", /.*/);
      await expect(section.locator("li").first()).toBeHidden();
      await section.locator("summary").click();
      await expect(section).toHaveAttribute("open", "");
      await expect(section.locator("li").first()).toBeVisible();
      await expect(section.locator("li")).toHaveCount(await strip.locator(`details.${cls} li`).count());
    }
    await expect(quiet.locator('details.home-boards a[href="/board/STORY-1482"]')).toBeVisible();
    const quietFold = quiet.locator("details.dir-archived");
    await expect(quietFold.getByTestId(dirEntryTestId(SHOWCASE.DIR_ARCHIVED_SPEC))).toBeHidden();
    await quietFold.locator("summary").click();
    await expect(quietFold.getByTestId(dirEntryTestId(SHOWCASE.DIR_ARCHIVED_SPEC))).toBeVisible();
  } finally {
    await noJS.close();
  }

  // (c) The Disclosures toggle (SI-366 (5)): the bar's one Disclosures
  // link, carrying the disclosures count as its carrier and showing it,
  // leading to /disclosures — and, separately, the "disclosed" filter
  // pill over the cards' own disclosures, whose count is the disclosed
  // cards and which hides every other card.
  const disclosures = page.getByTestId("topbar").locator("a.home-disclosures");
  await expect(disclosures).toBeVisible();
  await expect(disclosures).toHaveAttribute("href", "/disclosures");
  await expect(page.getByRole("link", { name: "Disclosures" })).toHaveCount(1);
  await expect(disclosures).toHaveAttribute("data-disclosures-count", /^\d+$/);
  const count = (await disclosures.getAttribute("data-disclosures-count")) ?? "";
  await expect(disclosures.locator(".count")).toHaveText(count);
  await expect(page.locator("p.home-disclosures")).toHaveCount(0);
  const disclosed = pill(page, "disclosed");
  await expect(disclosed).toHaveAttribute("aria-pressed", "false");
  const disclosedCards = page.locator('.dir-entry[data-disclosed="true"]');
  expect(await disclosedCards.count()).toBeGreaterThan(0);
  await expect(disclosed.locator(".count")).toHaveText(String(await disclosedCards.count()));
  await disclosed.click();
  await expect(disclosed).toHaveAttribute("aria-pressed", "true");
  await expect(card(page, EDGE.DIR_EMPTY_BRANCH)).toBeVisible();
  await expect(card(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE)).toBeVisible();
  await expect(card(page, SHOWCASE.DIR_LOCAL_DRAFT)).toBeHidden();
  await expect(card(page, SHOWCASE.READONLY_SPEC)).toBeHidden();
  await pill(page, "everything").click();
  await expect(card(page, SHOWCASE.DIR_LOCAL_DRAFT)).toBeVisible();

  // (d) The disclosed entry for a branch with no draft spec stays: on the
  // desk, naming its branch, disclosed, with no link at all.
  const notice = card(page, EDGE.DIR_EMPTY_BRANCH);
  await expectIn(page, DESK, EDGE.DIR_EMPTY_BRANCH);
  await expect(notice.locator(".dir-disclosed")).toBeVisible();
  await expect(notice.locator(".dir-disclosed")).toContainText(`design/${EDGE.DIR_EMPTY_BRANCH}`);
  await expect(notice.locator("a")).toHaveCount(0);
  await expect(notice).toHaveAttribute("data-disclosed", "true");

  // (e) The notice page for a deleted branch stays: a board address whose
  // design branch resolves to no ref renders the disclosed 404 notice
  // naming the branch, with a working way back to the directory (37
  // proves the same page after a real mid-session deletion).
  const gone = await page.goto(draftBoardHref(EDGE.DIR_VANISHED_BRANCH));
  expect(gone?.status()).toBe(404);
  const stale = page.getByTestId("stale-entry-notice");
  await expect(stale).toBeVisible();
  await expect(stale).toContainText(`design/${EDGE.DIR_VANISHED_BRANCH}`);
  await page.getByTestId("back-to-directory").click();
  await expect(column(page, DESK)).toBeVisible();
  await expect(card(page, EDGE.DIR_VANISHED_BRANCH)).toHaveCount(0);

  // ...and the Disclosures link leads where it says.
  await disclosures.click();
  await expect(page).toHaveURL(/\/disclosures$/);
});

test("index › The New story call to action", async ({ page }) => {
  await page.goto("/");

  // The accepted feature with unclaimed criteria: its card carries the
  // call to action beside its move, naming the unclaimed count and the
  // first unclaimed criterion in declared order, speaking the store's
  // story word, and linking the feature's wall with the criterion as
  // the query the wall's opener honours (SI-366 (10), (11)); no unproven
  // mark, since coverage was read.
  const feature = card(page, SHOWCASE.INDEX_CTA_FEATURE);
  await expectIn(page, ACCEPTED, SHOWCASE.INDEX_CTA_FEATURE);
  const cta = feature.getByTestId("dir-cta");
  await expect(cta).toHaveCount(1);
  await expect(cta).toBeVisible();
  await expect(cta).toHaveText(`${SHOWCASE.INDEX_CTA_UNCLAIMED} AC unclaimed · ${SHOWCASE.INDEX_CTA_CRITERION} · New story`);
  await expect(cta).toHaveAttribute("href", `/board/spec/${SHOWCASE.INDEX_CTA_FEATURE}?new-story=${SHOWCASE.INDEX_CTA_CRITERION}`);
  await expect(cta).toHaveAttribute("data-cta-ac", SHOWCASE.INDEX_CTA_CRITERION);
  await expect(cta).toHaveAttribute("data-cta-unclaimed", String(SHOWCASE.INDEX_CTA_UNCLAIMED));
  await expect(feature.locator(".dir-cta-unproven")).toHaveCount(0);
  await expect(feature.locator(".dir-move")).toHaveText("→ sealed wall");

  // The feature every criterion of which a stub claims shows no call to
  // action — and no "coverage unproven" either: its coverage was read
  // and found complete. No call to action anywhere counts zero.
  const claimed = card(page, SHOWCASE.INDEX_CTA_CLAIMED_FEATURE);
  await expectIn(page, ACCEPTED, SHOWCASE.INDEX_CTA_CLAIMED_FEATURE);
  await expect(claimed.locator(".dir-cta")).toHaveCount(0);
  await expect(claimed.locator(".dir-unproven")).toHaveCount(0);
  const counts = await page.locator(".dir-cta-link").evaluateAll((els) => els.map((el) => Number(el.getAttribute("data-cta-unclaimed"))));
  expect(counts.length).toBeGreaterThan(0);
  for (const n of counts) {
    expect(n).toBeGreaterThan(0);
  }
  // Only accepted features carry one: never a desk draft, a component or
  // a shelved spec.
  for (const group of [DESK, ACTIVE, SHELF]) {
    await expect(column(page, group).locator(".dir-cta")).toHaveCount(0);
  }

  // Following it opens the feature's wall with the New story dialog
  // open and exactly that criterion claimed, through the dialog's own
  // button and checkbox (the hooks the contract names).
  await cta.click();
  await expect(page).toHaveURL(new RegExp(`/board/spec/${SHOWCASE.INDEX_CTA_FEATURE}\\?new-story=${SHOWCASE.INDEX_CTA_CRITERION}$`));
  const dialog = page.locator("#create-dialog");
  await expect(dialog).toBeVisible();
  await expect(page.getByTestId(`create-ac-${SHOWCASE.INDEX_CTA_CRITERION}`)).toBeChecked();
  await expect(dialog.locator("[data-create-ac]:checked")).toHaveCount(1);
  await expect(page.getByTestId("create-error")).toBeHidden();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();

  // The opener honours the criterion it is named, not a position: the
  // wall's later uncovered criterion (loan-workflow-v2 declares ac-1 then
  // ac-3 and nothing claims either, the two INDEX_CTA_UNCLAIMED counts —
  // fixtures.ts) opens the dialog with exactly that box checked, never
  // the dialog's first box (F7CR-1).
  const later = "ac-3";
  await page.goto(`/board/spec/${SHOWCASE.INDEX_CTA_FEATURE}?new-story=${later}`);
  await expect(dialog).toBeVisible();
  const boxes = await dialog.locator("[data-create-ac]").evaluateAll((els) => els.map((el) => el.getAttribute("data-create-ac")));
  expect(boxes.indexOf(later), `${later} is declared after the dialog's first box, among ${boxes.join(", ")}`).toBeGreaterThan(0);
  await expect(page.getByTestId(`create-ac-${later}`)).toBeChecked();
  await expect(dialog.locator("[data-create-ac]:checked")).toHaveCount(1);
  await expect(dialog.locator(`[data-create-ac="${boxes[0]}"]`)).not.toBeChecked();
  await expect(page.getByTestId("create-error")).toBeHidden();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();

  // The same wall opened without the query shows no dialog: the opener
  // acts on the contract alone.
  await page.goto(`/board/spec/${SHOWCASE.INDEX_CTA_FEATURE}`);
  await expect(page.getByTestId("create-spec-btn")).toBeVisible();
  await expect(dialog).toBeHidden();

  // A criterion this wall does not declare: the dialog opens with
  // nothing claimed and its error line names the criterion — never a
  // silent open, never a different box checked.
  await page.goto(`/board/spec/${SHOWCASE.INDEX_CTA_FEATURE}?new-story=ac-99`);
  await expect(dialog).toBeVisible();
  await expect(dialog.locator("[data-create-ac]:checked")).toHaveCount(0);
  await expect(page.getByTestId("create-error")).toBeVisible();
  await expect(page.getByTestId("create-error")).toContainText("ac-99");
});

// LAST in this file (SI-366 (16)): the forge outage is one-way until the
// harness's reset route is POSTed, which this test does before it ends.
test("index › Filters, and review status disclosed when the forge is unreachable", async ({ page }) => {
  await page.goto("/");

  // The filter row: four pills in order, everything pressed first with
  // every card counted and shown.
  const filters = page.getByTestId("dir-filters");
  await expect(filters).toBeVisible();
  expect(await filters.locator(".dir-filter").evaluateAll((els) => els.map((el) => el.getAttribute("data-filter")))).toEqual(
    [...FILTERS],
  );
  const total = await page.locator(".dir-entry").count();
  expect(total).toBeGreaterThan(0);
  await expect(pill(page, "everything")).toHaveAttribute("aria-pressed", "true");
  await expect(pill(page, "everything").locator(".count")).toHaveText(String(total));
  await expect(page.locator(".dir-entry[hidden]")).toHaveCount(0);
  const columnCounts = await page.locator(".dir-group h2 .count").allTextContents();
  // Each column's cards, by id and in order, before anything is done to
  // the page (F7BR-2): the outage below must leave every one where it is.
  const columnCards: Record<string, (string | null)[]> = {};
  for (const [group] of INDEX_COLUMNS) {
    columnCards[group] = await column(page, group)
      .locator(".dir-entry")
      .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
  }

  // applyFilter presses one pill and asserts what ac-3 asks of it: every
  // card the filter's own attribute selects stays shown, every other card
  // is hidden, the pill's count is the selected cards, only that pill is
  // pressed, and the column counts stay the totals (SI-366 (19)). The
  // archived fold's summary says how many of its folded cards the filter
  // selects, so a match the closed fold hides is never silent, and says
  // nothing while everything is shown (F7BR-1).
  const foldMark = column(page, SHELF).locator("details.dir-archived > summary .dir-archived-matched");
  async function applyFilter(id: (typeof FILTERS)[number], attr: string): Promise<void> {
    await pill(page, id).click();
    for (const other of FILTERS) {
      await expect(pill(page, other)).toHaveAttribute("aria-pressed", String(other === id));
    }
    await expect(page.locator(`.dir-entry[${attr}][hidden]`)).toHaveCount(0);
    await expect(page.locator(`.dir-entry:not([${attr}]):not([hidden])`)).toHaveCount(0);
    await expect(pill(page, id).locator(".count")).toHaveText(String(await page.locator(`.dir-entry[${attr}]`).count()));
    expect(await page.locator(".dir-group h2 .count").allTextContents()).toEqual(columnCounts);
    if (id === "everything") {
      await expect(foldMark).toBeHidden();
      await expect(foldMark).toHaveText("");
    } else {
      await expect(foldMark).toBeVisible();
      const folded = await column(page, SHELF).locator(`details.dir-archived .dir-entry[${attr}]`).count();
      await expect(foldMark).toHaveText(`· ${folded} match`);
    }
  }
  // Quiet drafts: the shared store's drafts all read quiet (one fixed
  // commit date against the wall clock), a default-branch spec never.
  await applyFilter("quiet", 'data-quiet="true"');
  await expect(card(page, SHOWCASE.DIR_LOCAL_DRAFT)).toBeVisible();
  await expect(card(page, SHOWCASE.READONLY_SPEC)).toBeHidden();
  // Drafts in review: the branch with the open pull request.
  await applyFilter("in-review", 'data-review="open"');
  await expect(card(page, SHOWCASE.DIR_INREVIEW_SPEC)).toBeVisible();
  await expect(card(page, SHOWCASE.DIR_LOCAL_DRAFT)).toBeHidden();
  await expect(card(page, SHOWCASE.READONLY_SPEC)).toBeHidden();
  // Disclosed entries: the no-draft branch and the compatibility-noted
  // closed feature; a proven card is hidden.
  await applyFilter("disclosed", 'data-disclosed="true"');
  await expect(card(page, EDGE.DIR_EMPTY_BRANCH)).toBeVisible();
  await expect(card(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE)).toBeVisible();
  await expect(card(page, SHOWCASE.READONLY_SPEC)).toBeHidden();
  // Everything again: nothing hidden.
  await applyFilter("everything", "data-testid");
  await expect(page.locator(".dir-entry[hidden]")).toHaveCount(0);
  // The fold's mark counts rather than merely reading 0 (F7BR-1): the
  // shared store's archived cards match no filter today, so one folded
  // card is marked disclosed in this page's DOM alone — the attribute
  // the script reads — and the disclosed filter must then say 1 while
  // the card stays folded; everything clears the mark again.
  const folded = page.locator("details.dir-archived .dir-entry").first();
  await folded.evaluate((el) => el.setAttribute("data-disclosed", "true"));
  await pill(page, "disclosed").click();
  await expect(foldMark).toHaveText("· 1 match");
  await expect(folded).toBeHidden();
  await folded.evaluate((el) => el.setAttribute("data-disclosed", "false"));
  await pill(page, "everything").click();
  await expect(foldMark).toBeHidden();

  // The isolated dated store (exact): quiet selects exactly the quiet
  // cards, and with no forge configured the in-review pill says so —
  // never a zero — and no card carries a review chip.
  const dated = await isolatedBase(page, INDEX_DATES_FIXTURE_URL);
  await page.goto(dated);
  const quietNames = Object.entries(SHOWCASE.INDEX_DATED_AGES)
    .filter(([, age]) => age.startsWith("quiet"))
    .map(([name]) => dirEntryTestId(name))
    .sort();
  expect(quietNames.length).toBeGreaterThan(0);
  await expect(pill(page, "quiet").locator(".count")).toHaveText(String(quietNames.length));
  await pill(page, "quiet").click();
  expect(
    await page.locator(".dir-entry:not([hidden])").evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")).sort()),
  ).toEqual(quietNames);
  await expect(pill(page, "in-review")).toHaveText("in review · no forge configured");
  await expect(pill(page, "in-review")).toBeDisabled();
  await expect(pill(page, "in-review")).not.toHaveText(/\d/);
  await expect(page.locator(".dir-inreview, .dir-review-unavailable")).toHaveCount(0);
  await expect(page.getByTestId("mr-status-unavailable")).toHaveCount(0);

  // The forge unreachable (SI-366 (4)): the notice stays; every
  // design-branch card's chip says review status is unavailable, in a
  // class no in-review pin counts; a default-branch card, with no branch
  // to be in review, carries no chip; the in-review pill says unavailable
  // with no number; and every card stays in its column, the column
  // counts unchanged.
  const outage = await page.request.post(FORGE_OUTAGE_URL);
  expect(outage.ok()).toBe(true);
  try {
    await page.goto("/");
    await expect(page.getByTestId("mr-status-unavailable")).toBeVisible();
    await expect(page.getByTestId("mr-status-unavailable")).toContainText("MR status unavailable");
    await expect(page.locator(".dir-inreview")).toHaveCount(0);
    for (const name of [SHOWCASE.DESIGN_SPEC, SHOWCASE.DIR_LOCAL_DRAFT, SHOWCASE.DIR_REMOTE_DRAFT, EDGE.DIR_EMPTY_BRANCH]) {
      await expectIn(page, DESK, name);
      await expect(card(page, name).locator(".dir-review-unavailable")).toHaveText("review status unavailable");
      await expect(card(page, name)).toHaveAttribute("data-review", "unavailable");
    }
    await expect(page.locator('.dir-entry:not([data-source="default"]):not(:has(.dir-review-unavailable))')).toHaveCount(0);
    for (const [name, group] of [
      [SHOWCASE.READONLY_SPEC, ACCEPTED],
      [SHOWCASE.NO_CASEFILE_SPEC, ACTIVE],
      [SHOWCASE.DIR_TERMINAL_SPEC, SHELF],
      [EDGE.DIR_CLOSED_AWAITING_ARCHIVE, SHELF],
    ] as const) {
      await expectIn(page, group, name);
      await expect(card(page, name).locator(".dir-review-unavailable")).toHaveCount(0);
      await expect(card(page, name)).toHaveAttribute("data-review", "not-open");
    }
    await expect(column(page, SHELF).locator("details.dir-archived").getByTestId(dirEntryTestId(SHOWCASE.DIR_ARCHIVED_SPEC))).toHaveCount(1);
    await expect(pill(page, "in-review")).toHaveText("in review · unavailable");
    await expect(pill(page, "in-review")).toBeDisabled();
    await expect(pill(page, "in-review")).not.toHaveText(/\d/);
    await expect(pill(page, "everything").locator(".count")).toHaveText(String(await page.locator(".dir-entry").count()));
    expect(await page.locator(".dir-group h2 .count").allTextContents()).toEqual(columnCounts);
    await expect(page.locator(".dir-entry[hidden]")).toHaveCount(0);
    // ...and every column holds exactly the cards it held before, in the
    // same order (F7BR-2): the forge never moves a card.
    for (const [group] of INDEX_COLUMNS) {
      const after = await column(page, group)
        .locator(".dir-entry")
        .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
      expect(after, `${group}'s cards after the outage`).toEqual(columnCards[group]);
    }
  } finally {
    const reset = await page.request.post(FORGE_OUTAGE_RESET_URL);
    expect(reset.ok()).toBe(true);
  }

  // After the reset the forge answers again: the chip and the count are
  // back, so the file leaves the store as it found it.
  await page.goto("/");
  await expect(page.getByTestId("mr-status-unavailable")).toHaveCount(0);
  await expect(card(page, SHOWCASE.DIR_INREVIEW_SPEC).locator(".dir-inreview")).toHaveText(SHOWCASE.DIR_INREVIEW_CHIP);
  await expect(pill(page, "in-review").locator(".count")).toHaveText(String(await page.locator('.dir-entry[data-review="open"]').count()));
});
