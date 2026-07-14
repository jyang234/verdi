import { test, expect, type Page } from "@playwright/test";
import {
  CONTROL_URL,
  DESIGN_SPEC,
  DIR_DOOMED_DRAFT,
  DIR_EMPTY_BRANCH,
  DIR_INREVIEW_SPEC,
  DIR_LOCAL_DRAFT,
  DIR_REMOTE_DRAFT,
  dirEntryTestId,
  dirGroupTestId,
  draftBoardHref,
} from "./fixtures";

// The whole-store directory home (spec/directory-home): GET / at the one
// serve address renders the computed directory index — every spec on the
// default branch and every draft on a design branch — grouped by status
// (workbench-directory dc-2), chipped, linked per the ratified grammars,
// disclosed by source, chipped in-review from the forge feed, and
// degrading every absence to a disclosed notice.
//
// ORDER MATTERS within this file (the suite is serial, one shared store):
// the delete-branch test permanently removes DIR_DOOMED_DRAFT's branch and
// the outage test permanently downs the open-MR feed, so both run last, in
// that order.

const ALL_DRAFTS = [
  DESIGN_SPEC,
  DIR_LOCAL_DRAFT,
  DIR_REMOTE_DRAFT,
  DIR_DOOMED_DRAFT,
  DIR_EMPTY_BRANCH,
];

// One representative default-branch spec per remaining group (statuses per
// testdata/corpus): accepted-pending-build, active component, terminal —
// both the active-zone superseded shape and the archive-zone closed shape.
const ACCEPTED_SPEC = "stale-decline";
const ACTIVE_SPEC = "store-layout-notes";
const TERMINAL_SPEC = "legacy-cache-policy";
const ARCHIVED_SPEC = "loan-refi-2023";

function entry(page: Page, name: string) {
  return page.getByTestId(dirEntryTestId(name));
}

// AC-1: the four status groups are the page's organizing structure; every
// fixture spec appears exactly once (the design-branch drafts included —
// the entries the old single-checkout home could not show), status-chipped,
// board-linked per dc-3's two grammars.
test("home renders the four status groups, each entry once, chipped and linked", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Workbench/);

  // (a) the four dc-2 groups, as sections, in order.
  const groups = [
    "drafts-in-progress",
    "accepted-pending-build",
    "active-components",
    "terminal",
  ];
  for (const g of groups) {
    await expect(page.getByTestId(dirGroupTestId(g))).toBeVisible();
  }
  const rendered = await page.locator(".dir-group").evaluateAll((els) =>
    els.map((el) => el.getAttribute("data-testid")),
  );
  expect(rendered).toEqual(groups.map((g) => `dir-group-${g}`));

  // (b) every fixture spec appears exactly once, under its group.
  for (const name of ALL_DRAFTS) {
    await expect(entry(page, name)).toHaveCount(1);
    await expect(
      page.getByTestId(dirGroupTestId("drafts-in-progress")).getByTestId(dirEntryTestId(name)),
    ).toBeVisible();
  }
  for (const [name, group] of [
    [ACCEPTED_SPEC, "accepted-pending-build"],
    [ACTIVE_SPEC, "active-components"],
    [TERMINAL_SPEC, "terminal"],
    [ARCHIVED_SPEC, "terminal"],
  ] as const) {
    await expect(entry(page, name)).toHaveCount(1);
    await expect(
      page.getByTestId(dirGroupTestId(group)).getByTestId(dirEntryTestId(name)),
    ).toBeVisible();
  }

  // (c) every ordinary entry carries a visible status chip.
  await expect(entry(page, DESIGN_SPEC).locator(".badge-draft")).toHaveText("draft");
  await expect(
    entry(page, ACCEPTED_SPEC).locator(".badge-accepted-pending-build"),
  ).toHaveText("accepted-pending-build");
  await expect(entry(page, ACTIVE_SPEC).locator(".badge-active")).toHaveText("active");
  await expect(entry(page, TERMINAL_SPEC).locator(".badge-superseded")).toHaveText(
    "superseded",
  );
  await expect(entry(page, ARCHIVED_SPEC).locator(".badge-closed")).toHaveText("closed");

  // (d) board links per dc-3: unprefixed for a default-branch entry; the
  // /b/<branch-escaped>/ grammar for design-branch drafts (href only —
  // the routes belong to the draft-boards story).
  await expect(entry(page, ACCEPTED_SPEC).locator("a.dir-board")).toHaveAttribute(
    "href",
    `/board/spec/${ACCEPTED_SPEC}`,
  );
  for (const name of [DESIGN_SPEC, DIR_LOCAL_DRAFT, DIR_REMOTE_DRAFT]) {
    await expect(entry(page, name).locator("a.dir-board")).toHaveAttribute(
      "href",
      draftBoardHref(name),
    );
  }

  // The unprefixed default-branch board link still serves (live by
  // construction): the accepted spec's board opens.
  await entry(page, ACCEPTED_SPEC).locator("a.dir-board").click();
  await expect(page.getByTestId("board")).toBeVisible();
});

// AC-2 (sources): every entry is disclosed by source — a local design
// branch, a remote-tracking one, a both-sided one, and a default-branch
// spec are all visibly distinguished.
test("entries are disclosed by source: local, remote-tracking, both, default", async ({
  page,
}) => {
  await page.goto("/");

  await expect(entry(page, DIR_LOCAL_DRAFT).locator(".badge-src")).toHaveText(
    "local branch",
  );
  await expect(entry(page, DIR_REMOTE_DRAFT).locator(".badge-src")).toHaveText(
    "remote-tracking",
  );
  await expect(entry(page, DESIGN_SPEC).locator(".badge-src")).toHaveText(
    "local + remote",
  );
  await expect(entry(page, ACCEPTED_SPEC).locator(".badge-src")).toHaveText(
    "default branch",
  );
});

// AC-2 (chip): the branch with an open MR — and only that one — is chipped
// "in review" from the forge feed, and the second source is disclosed.
test("an open MR chips its entry in review, and only that entry", async ({ page }) => {
  await page.goto("/");

  await expect(entry(page, DIR_INREVIEW_SPEC).locator(".dir-inreview")).toHaveText(
    "in review",
  );
  await expect(page.locator(".dir-inreview")).toHaveCount(1);
  await expect(page.locator(".dir-provenance")).toContainText(
    "a second source beside the refs",
  );
});

// AC-3 (empty branch): a design branch with no draft spec renders as a
// disclosed notice entry naming the branch — listed, explained, never
// linked as if a board existed.
test("a design branch with no draft spec is a disclosed notice entry, not a link", async ({
  page,
}) => {
  await page.goto("/");

  const notice = entry(page, DIR_EMPTY_BRANCH);
  await expect(notice).toBeVisible();
  await expect(notice).toContainText(`design/${DIR_EMPTY_BRANCH}`);
  await expect(notice.locator(".dir-disclosed")).toBeVisible();
  await expect(notice.locator("a")).toHaveCount(0);

  // One unresolvable entry never takes down the page: the rest of the
  // directory renders around it.
  await expect(entry(page, ACCEPTED_SPEC)).toBeVisible();
  await expect(entry(page, DESIGN_SPEC)).toBeVisible();
});

// AC-3 (deleted mid-session): the fixture deletes a listed design branch
// AFTER the directory renders; clicking that entry's link resolves to a
// rendered disclosed notice page — HTTP 404, a body naming what vanished,
// and a working link back to the directory. Never a bare NotFound.
test("a branch deleted mid-session resolves to a disclosed 404 with a way back", async ({
  page,
}) => {
  await page.goto("/");
  const link = entry(page, DIR_DOOMED_DRAFT).locator("a.dir-board");
  await expect(link).toHaveAttribute("href", draftBoardHref(DIR_DOOMED_DRAFT));

  // The branch vanishes between render and click.
  const del = await page.request.post(
    `${CONTROL_URL}/delete-branch?branch=${encodeURIComponent(`design/${DIR_DOOMED_DRAFT}`)}`,
  );
  expect(del.ok()).toBe(true);

  const responsePromise = page.waitForResponse(
    (r) => r.request().isNavigationRequest() && r.url().includes(DIR_DOOMED_DRAFT),
  );
  await link.click();
  const response = await responsePromise;
  expect(response.status()).toBe(404);

  const notice = page.getByTestId("stale-entry-notice");
  await expect(notice).toBeVisible();
  await expect(notice).toContainText(`design/${DIR_DOOMED_DRAFT}`);

  // The way back works — and the re-rendered directory honestly no longer
  // lists the vanished branch, while everything else still renders.
  await page.getByTestId("back-to-directory").click();
  await expect(page.getByTestId(dirGroupTestId("drafts-in-progress"))).toBeVisible();
  await expect(entry(page, DIR_DOOMED_DRAFT)).toHaveCount(0);
  await expect(entry(page, DESIGN_SPEC)).toBeVisible();
});

// AC-2 (degradation) — LAST: with the forge feed unreachable, the SAME
// surface renders a disclosed "MR status unavailable" notice in place of
// the chip while the refs-computed directory still renders fully.
test("an unreachable forge degrades to a disclosed notice while the directory renders fully", async ({
  page,
}) => {
  const outage = await page.request.post(`${CONTROL_URL}/outage`);
  expect(outage.ok()).toBe(true);

  await page.goto("/");

  const notice = page.getByTestId("mr-status-unavailable");
  await expect(notice).toBeVisible();
  await expect(notice).toContainText("MR status unavailable");

  // No fabricated chips, and the directory is complete — not blocked, not
  // partial: every group and every surviving fixture entry still renders.
  await expect(page.locator(".dir-inreview")).toHaveCount(0);
  for (const g of [
    "drafts-in-progress",
    "accepted-pending-build",
    "active-components",
    "terminal",
  ]) {
    await expect(page.getByTestId(dirGroupTestId(g))).toBeVisible();
  }
  for (const name of [
    DESIGN_SPEC,
    DIR_LOCAL_DRAFT,
    DIR_REMOTE_DRAFT,
    DIR_EMPTY_BRANCH,
    ACCEPTED_SPEC,
    ACTIVE_SPEC,
    TERMINAL_SPEC,
    ARCHIVED_SPEC,
  ]) {
    await expect(entry(page, name)).toBeVisible();
  }
});
