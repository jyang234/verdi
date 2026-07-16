import { test, expect, type Page } from "@playwright/test";
import {
  SHOWCASE,
  EDGE,
  CONTROL_URL,
  dirEntryTestId,
  dirGroupTestId,
  glanceEntryTestId,
  glanceGroupTestId,
  draftBoardHref,
} from "./fixtures";

// The home status glance (spec/home-status-glance): GET / leads with a new
// status-at-a-glance section, above the existing, unchanged Directory
// section (ac-2's no-loss bar) — every active spec, default-branch and
// design-branch alike, regroups into three fixed, actionable-first buckets
// (on-the-desk / in-flight / settling), each entry carrying only its
// status badge and working links (no source chip, no in-review chip, no
// evidence-bearing state — dc-3).
//
// The shared harness store already spans every status this store's schema
// legalizes and both zones (a design-branch draft, a default-branch
// accepted-pending-build feature, a default-branch active component, a
// default-branch superseded component still in the active zone, and an
// archive-zone closed feature) — this suite reuses those fixtures rather
// than provisioning a parallel store. Only the "closed awaiting archive"
// shape (parent workbench-legibility dc-4's own example: closed, but
// STILL physically in specs/active/) needed a new, minimal fixture, since
// no committed examples/showcase spec carries it — see
// EDGE.DIR_CLOSED_AWAITING_ARCHIVE (cmd/e2eharness/provision.go).
//
// The empty-bucket case (ac-3) cannot be proven against that same shared
// store: every glance bucket there is populated by real fixtures other
// suites depend on (in-flight alone backs stale-decline, escrow-autopay,
// and every dex-suite accepted-pending-build story/feature), so
// manufacturing an empty bucket would mean deleting fixtures other tests
// need. Instead it drives a SEPARATE, hermetic, isolated workbench
// instance the control server spawns on demand (CONTROL_URL's
// /empty-glance-fixture — cmd/e2eharness/emptyglance.go), backed by a REAL
// minimal store (git init + .verdi/verdi.yaml, zero specs) computed
// through the real refindex.ComputeIndex pipeline (co-1; Controller
// adjudication ADJ-40) — never touching the shared store, and never a
// canned index standing in for the pipeline.

const ACCEPTED_SPEC = "stale-decline"; // default-branch accepted-pending-build feature
const ACCEPTED_STORY = "jira:LOAN-1482";
const ACTIVE_SPEC = "store-layout-notes"; // default-branch active component
const TERMINAL_SPEC = "legacy-cache-policy"; // default-branch superseded component, still active-zone
const ARCHIVED_SPEC = "loan-refi-2023"; // archive-zone closed feature — glance-excluded (dc-2)
const CLOSED_AWAITING_ARCHIVE_STORY = "jira:LOAN-1901";

function glanceEntry(page: Page, name: string) {
  return page.getByTestId(glanceEntryTestId(name));
}
function glanceGroup(page: Page, slug: string) {
  return page.getByTestId(glanceGroupTestId(slug));
}

// AC-1: the three fixed buckets are the glance's organizing structure, in
// order; every fixture spec appears exactly once, under its correct
// bucket, status-chipped, and linked per dc-3's two grammars; matrix and
// verdict appear only on a default-branch feature entry carrying a story
// ref; the archive-zone entry is excluded entirely (dc-2).
test("the glance groups every fixture entry into its correct bucket, badged and linked per source and class", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Workbench/);

  // dc-5: the glance renders, structurally, above the exhaustive
  // Directory section.
  await expect(page.getByTestId("home-glance")).toBeVisible();
  const glanceBox = await page.getByTestId("home-glance").boundingBox();
  const dirBox = await page.locator(".home-directory").boundingBox();
  expect(glanceBox).not.toBeNull();
  expect(dirBox).not.toBeNull();
  expect(glanceBox!.y).toBeLessThan(dirBox!.y);

  // (a) the three dc-2 buckets, as sections, in ADJ-36's fixed order.
  const groups = ["on-the-desk", "in-flight", "settling"];
  for (const g of groups) {
    await expect(glanceGroup(page, g)).toBeVisible();
  }
  const rendered = await page
    .locator(".glance-group")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
  expect(rendered).toEqual(groups.map((g) => glanceGroupTestId(g)));

  // (b) every fixture spec appears exactly once, under its bucket.
  for (const name of [SHOWCASE.DESIGN_SPEC, SHOWCASE.DIR_LOCAL_DRAFT, SHOWCASE.DIR_REMOTE_DRAFT]) {
    await expect(glanceEntry(page, name)).toHaveCount(1);
    await expect(glanceGroup(page, "on-the-desk").getByTestId(glanceEntryTestId(name))).toBeVisible();
  }
  for (const [name, group] of [
    [ACCEPTED_SPEC, "in-flight"],
    [ACTIVE_SPEC, "settling"],
    [TERMINAL_SPEC, "settling"],
    [EDGE.DIR_CLOSED_AWAITING_ARCHIVE, "settling"],
  ] as const) {
    await expect(glanceEntry(page, name)).toHaveCount(1);
    await expect(glanceGroup(page, group).getByTestId(glanceEntryTestId(name))).toBeVisible();
  }

  // dc-2/ADJ-32 f1: the archive-zone entry never appears in the glance at
  // all (asserted absent-yet-present-below by the sibling no-loss test).
  await expect(glanceEntry(page, ARCHIVED_SPEC)).toHaveCount(0);

  // (c) every entry carries its REAL raw status badge.
  await expect(glanceEntry(page, SHOWCASE.DESIGN_SPEC).locator(".badge-draft")).toHaveText("draft");
  await expect(
    glanceEntry(page, ACCEPTED_SPEC).locator(".badge-accepted-pending-build"),
  ).toHaveText("accepted-pending-build");
  await expect(glanceEntry(page, ACTIVE_SPEC).locator(".badge-active")).toHaveText("active");
  await expect(glanceEntry(page, TERMINAL_SPEC).locator(".badge-superseded")).toHaveText("superseded");
  await expect(
    glanceEntry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(".badge-closed"),
  ).toHaveText("closed");

  // (d) link grammar per dc-3: the unprefixed default-branch board
  // address...
  for (const name of [ACCEPTED_SPEC, ACTIVE_SPEC, TERMINAL_SPEC, EDGE.DIR_CLOSED_AWAITING_ARCHIVE]) {
    await expect(glanceEntry(page, name).locator("a.glance-board")).toHaveAttribute(
      "href",
      `/board/spec/${name}`,
    );
  }
  // ...and each default-branch entry's TITLE anchor (the first link in the
  // card, writeGlanceDefaultEntry) carries its /a/spec/<name> corpus href —
  // ac-1's "title, linked exactly as its source already links it today"
  // register, proven in the browser per ac-1's declared Playwright evidence
  // shape ("asserts ... link targets"), not only by the Go unit test
  // TestGlanceLinks_MirrorDirectoryExactly (Controller adjudication ADJ-44,
  // 2026-07-16). The .first() locator pins the title specifically: a card
  // that stopped emitting the corpus link would fail this href assertion,
  // never fall through to the board anchor (/board/spec/<name>).
  for (const name of [ACCEPTED_SPEC, ACTIVE_SPEC, TERMINAL_SPEC, EDGE.DIR_CLOSED_AWAITING_ARCHIVE]) {
    await expect(glanceEntry(page, name).locator("a").first()).toHaveAttribute("href", `/a/spec/${name}`);
  }
  // ...and the /b/<branch-escaped>/ grammar for design-branch drafts — the
  // entry's title IS its one link (dc-3), no separate board anchor.
  for (const name of [SHOWCASE.DESIGN_SPEC, SHOWCASE.DIR_LOCAL_DRAFT, SHOWCASE.DIR_REMOTE_DRAFT]) {
    await expect(glanceEntry(page, name).locator("a")).toHaveAttribute("href", draftBoardHref(name));
  }

  // (e) matrix+verdict appear ONLY on a default-branch feature entry
  // carrying a story ref.
  await expect(glanceEntry(page, ACCEPTED_SPEC).locator(`a[href="/matrix/${ACCEPTED_STORY}"]`)).toHaveCount(1);
  await expect(glanceEntry(page, ACCEPTED_SPEC).locator(`a[href="/verdict/${ACCEPTED_STORY}"]`)).toHaveCount(1);
  await expect(
    glanceEntry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(
      `a[href="/matrix/${CLOSED_AWAITING_ARCHIVE_STORY}"]`,
    ),
  ).toHaveCount(1);
  await expect(
    glanceEntry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(
      `a[href="/verdict/${CLOSED_AWAITING_ARCHIVE_STORY}"]`,
    ),
  ).toHaveCount(1);
  // Negative: never on a component entry, never on a design-branch draft.
  for (const name of [ACTIVE_SPEC, TERMINAL_SPEC, SHOWCASE.DESIGN_SPEC]) {
    await expect(glanceEntry(page, name).locator('a[href^="/matrix/"]')).toHaveCount(0);
    await expect(glanceEntry(page, name).locator('a[href^="/verdict/"]')).toHaveCount(0);
  }

  // dc-3: the glance never carries a source chip or an in-review chip
  // anywhere, scoped to the glance section alone.
  await expect(page.getByTestId("home-glance").locator(".badge-src")).toHaveCount(0);
  await expect(page.getByTestId("home-glance").locator(".dir-inreview")).toHaveCount(0);

  // The unprefixed default-branch board link genuinely serves (live by
  // construction).
  await glanceEntry(page, ACCEPTED_SPEC).locator("a.glance-board").click();
  await expect(page.getByTestId("board")).toBeVisible();
});

// AC-2: every section and link the directory rendered before this story
// landed is still present, in the same place, carrying the same content —
// the glance is additive only, never a replacement.
test("every pre-existing directory section and link survives unchanged alongside the new glance", async ({
  page,
}) => {
  await page.goto("/");

  // The four pre-existing status groups, unchanged.
  for (const g of ["drafts-in-progress", "accepted-pending-build", "active-components", "terminal"]) {
    await expect(page.getByTestId(dirGroupTestId(g))).toBeVisible();
  }
  // Every fixture entry the glance also shows is STILL in the exhaustive
  // section too (additive, never a replacement).
  for (const name of [
    SHOWCASE.DESIGN_SPEC,
    ACCEPTED_SPEC,
    ACTIVE_SPEC,
    TERMINAL_SPEC,
    EDGE.DIR_CLOSED_AWAITING_ARCHIVE,
  ]) {
    await expect(page.getByTestId(dirEntryTestId(name))).toBeVisible();
  }

  // The archive-zone entry — absent from the glance — is still fully
  // present, unchanged, in the exhaustive section (dc-2's zone rule;
  // ac-2's no-loss bar): the SAME store, rendered two ways.
  await expect(page.getByTestId(dirEntryTestId(ARCHIVED_SPEC))).toBeVisible();
  await expect(
    page.getByTestId(dirGroupTestId("terminal")).getByTestId(dirEntryTestId(ARCHIVED_SPEC)),
  ).toBeVisible();
  await expect(page.getByTestId(dirEntryTestId(ARCHIVED_SPEC)).locator(".badge-closed")).toHaveText(
    "closed",
  );

  // A case 37-directory-home.spec.ts already covers end to end, re-proven
  // unchanged here: the disclosed no-draft-spec notice entry (ac-3's
  // degenerate branch) — never mutated by any other test in this suite,
  // so it is deterministic regardless of file execution order.
  const disclosed = page.getByTestId(dirEntryTestId(EDGE.DIR_EMPTY_BRANCH));
  await expect(disclosed).toBeVisible();
  await expect(disclosed).toContainText(`design/${EDGE.DIR_EMPTY_BRANCH}`);
  await expect(disclosed.locator(".dir-disclosed")).toBeVisible();
  await expect(disclosed.locator("a")).toHaveCount(0);

  // The other, unrelated home sections — unchanged.
  await expect(page.locator(".home-kinds")).toBeVisible();
  await expect(page.locator(".home-services")).toBeVisible();
  await expect(page.locator(".home-boards")).toBeVisible();
  await expect(page.locator(".store-root")).toBeVisible();
  await expect(page.locator(".home-disclosures")).toBeVisible();
});

// AC-3/DC-4/CO-1: a glance bucket with zero matching entries still renders
// its heading, its zero count, and an explicit empty-state notice — never a
// silently omitted bucket. Driven against a separate, isolated REAL store
// (git init + .verdi/verdi.yaml, zero specs) computed through the real
// refindex.ComputeIndex pipeline (Controller adjudication ADJ-40) — an empty
// store proves all three empty buckets at once through the true pipe. See
// this file's header comment for why this is isolated rather than mutating
// the shared corpus.
test("an isolated real store with zero specs renders every glance bucket's heading, zero count, and empty-state notice", async ({
  page,
}) => {
  const res = await page.request.get(`${CONTROL_URL}/empty-glance-fixture`);
  expect(res.ok()).toBe(true);
  const isolatedURL = (await res.text()).trim();
  expect(isolatedURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);

  await page.goto(isolatedURL);
  await expect(page.getByTestId("home-glance")).toBeVisible();

  // Zero specs in the store, so all three fixed buckets are empty at once —
  // the strongest proof of dc-4: an operator reads absence-of-work as an
  // explicit, deliberate fact in every bucket, not a broken render. (The
  // populated-bucket contrast dc-4 also values is proven by this file's
  // first test, over the shared store.)
  for (const slug of ["on-the-desk", "in-flight", "settling"]) {
    const group = glanceGroup(page, slug);
    await expect(group).toBeVisible();
    await expect(group).toContainText("(0)");
    await expect(group.locator(".empty")).toHaveText("None.");
  }
  // Nothing to badge or link: not a single glance entry renders through the
  // real pipeline over an empty store.
  await expect(page.getByTestId("home-glance").locator(".glance-entry")).toHaveCount(0);
});
