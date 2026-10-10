import { test, expect, type Page } from "@playwright/test";
import {
  SHOWCASE,
  EDGE,
  INDEX_COLUMNS,
  EMPTY_INDEX_FIXTURE_URL,
  dirEntryTestId,
  dirGroupTestId,
  draftBoardHref,
} from "./fixtures";

// The home status glance's kept properties (spec/home-status-glance ac-1
// to ac-3, dc-1 to dc-5 — superseded by spec/workbench-redesign dc-12 and
// carried by spec/index-v2 dc-1): GET / shows every active spec,
// default-branch and design-branch alike, exactly once in actionable-
// first order, each entry status-badged and linked per its source and
// class, with counts and explicit empty states from one index
// computation per render. dc-12 merged the separate three-bucket glance
// into the directory's four columns, so this file's assertions address
// the columns and their cards under the dir-group and dir-entry test ids
// (the glance-group and glance-entry test ids gave way to them); its
// three titles stay verbatim (index-v2 co-2; SI-366 (9)).
//
// The shared harness store already spans every status this store's schema
// legalizes and both zones (a design-branch draft, a default-branch
// accepted-pending-build feature, a default-branch active component, a
// default-branch superseded component still in the active zone, and an
// archive-zone closed feature) — this suite reuses those fixtures rather
// than provisioning a parallel store. Only the "closed awaiting archive"
// shape (closed, but STILL physically in specs/active/) needed a new,
// minimal fixture, since no committed examples/showcase spec carries it —
// see EDGE.DIR_CLOSED_AWAITING_ARCHIVE (cmd/e2eharness/provision.go).
//
// The empty-column case cannot be proven against that same shared store:
// every column there is populated by real fixtures other suites depend
// on, so manufacturing an empty column would mean deleting fixtures other
// tests need. Instead it drives a SEPARATE, hermetic, isolated workbench
// instance the control server spawns on demand (EMPTY_INDEX_FIXTURE_URL —
// cmd/e2eharness/emptyglance.go), backed by a REAL minimal store (git
// init + .verdi/verdi.yaml, zero specs) computed through the real
// refindex.ComputeIndex pipeline (co-1; Controller adjudication ADJ-40)
// — never touching the shared store, and never a canned index standing
// in for the pipeline.

const ACCEPTED_SPEC = "stale-decline"; // default-branch accepted-pending-build feature
const ACCEPTED_STORY = "jira:LOAN-1482";
const ACTIVE_SPEC = "store-layout-notes"; // default-branch active component
const TERMINAL_SPEC = "legacy-cache-policy"; // default-branch superseded component, still active-zone
const ARCHIVED_SPEC = "loan-refi-2023"; // archive-zone closed feature — on the shelf, never leading
const CLOSED_AWAITING_ARCHIVE_STORY = "jira:LOAN-1901";

function entry(page: Page, name: string) {
  return page.getByTestId(dirEntryTestId(name));
}
function group(page: Page, slug: string) {
  return page.getByTestId(dirGroupTestId(slug));
}

// AC-1 (kept by dc-12): the four fixed columns are the page's organizing
// structure, in actionable-first order; every fixture spec appears
// exactly once, under its correct column, status-chipped, and linked per
// dc-3's two grammars; matrix and verdict appear only on a default-branch
// feature entry carrying a story ref; the archive-zone entry sits on the
// shelf, never leading, with no board link.
test("the glance groups every fixture entry into its correct bucket, badged and linked per source and class", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page).toHaveTitle(/Workbench/);

  // dc-12: the columns ARE the directory — one rendering, no separate
  // glance section above it.
  await expect(page.getByTestId("home-glance")).toHaveCount(0);
  await expect(page.locator(".home-directory")).toBeVisible();

  // (a) the four columns, as sections, in dc-2's fixed order.
  const groups = INDEX_COLUMNS.map(([slug]) => slug);
  for (const g of groups) {
    await expect(group(page, g)).toBeVisible();
  }
  const rendered = await page
    .locator(".dir-group")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-testid")));
  expect(rendered).toEqual(groups.map((g) => dirGroupTestId(g)));

  // (b) every fixture spec appears exactly once, under its column. The
  // archive-zone entry sits in the shelf's collapsed archived fold
  // (spec/index-v2 ac-5; SI-366 (8)): open it so the entry can be seen.
  await group(page, "terminal").locator("details.dir-archived > summary").click();
  for (const name of [SHOWCASE.DESIGN_SPEC, SHOWCASE.DIR_LOCAL_DRAFT, SHOWCASE.DIR_REMOTE_DRAFT]) {
    await expect(entry(page, name)).toHaveCount(1);
    await expect(group(page, "drafts-in-progress").getByTestId(dirEntryTestId(name))).toBeVisible();
  }
  for (const [name, g] of [
    [ACCEPTED_SPEC, "accepted-pending-build"],
    [ACTIVE_SPEC, "active-components"],
    [TERMINAL_SPEC, "terminal"],
    // Merge-signaled acceptance, legacy-compatibility reading (Task 6
    // fix round 2, finding 4): a LANDED spec carrying a legacy explicit
    // `status: closed` projects Closed — the projector's compatibility
    // rows preserve a legacy terminal artifact's existing meaning (with
    // a disclosure) rather than silently re-deriving a weaker state —
    // so the closed-awaiting-archive shape still settles on the shelf.
    [EDGE.DIR_CLOSED_AWAITING_ARCHIVE, "terminal"],
    // dc-12: the archive-zone entry is on the shelf, the last column —
    // it never leads the page.
    [ARCHIVED_SPEC, "terminal"],
  ] as const) {
    await expect(entry(page, name)).toHaveCount(1);
    await expect(group(page, g).getByTestId(dirEntryTestId(name))).toBeVisible();
  }

  // (c) every entry carries its REAL raw status badge.
  await expect(entry(page, SHOWCASE.DESIGN_SPEC).locator(".badge-draft")).toHaveText("draft");
  await expect(entry(page, ACCEPTED_SPEC).locator(".badge-accepted-pending-build")).toHaveText(
    "accepted-pending-build",
  );
  await expect(entry(page, ACTIVE_SPEC).locator(".badge-active")).toHaveText("active");
  await expect(entry(page, TERMINAL_SPEC).locator(".badge-superseded")).toHaveText("superseded");
  // The badge speaks the EFFECTIVE state — which, under the projector's
  // legacy-compatibility rows, honors the landed explicit terminal field
  // (see the column note above).
  await expect(entry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(".badge-closed")).toHaveText("closed");

  // (c2, final fix wave I5) the projector's legacy-compatibility reading
  // is DISCLOSED beside the proven badge, in the shared disclosure
  // vocabulary — the closed-awaiting-archive spec's state was honored
  // from its persisted legacy field alone, and that compatibility note
  // must reach the browser, never be stripped by the index seam.
  await expect(entry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(".dir-disclosed")).toContainText(
    "compatibility reading",
  );
  // A clean legacy-accepted entry carries no such note (the disclosure is
  // the projector's own, never a blanket decoration).
  await expect(entry(page, ACCEPTED_SPEC).locator(".dir-disclosed")).toHaveCount(0);

  // (d) link grammar per dc-3: the unprefixed default-branch board
  // address on every servable default-branch entry...
  for (const name of [ACCEPTED_SPEC, ACTIVE_SPEC, TERMINAL_SPEC, EDGE.DIR_CLOSED_AWAITING_ARCHIVE]) {
    await expect(entry(page, name).locator("a.dir-board")).toHaveAttribute("href", `/board/spec/${name}`);
  }
  // ...none on the archive-zone entry (only addresses the routing serves)...
  await expect(entry(page, ARCHIVED_SPEC).locator("a.dir-board")).toHaveCount(0);
  // ...and each default-branch entry's TITLE anchor (the first link in the
  // card) carries its /a/spec/<name> corpus href — ac-1's "title, linked
  // exactly as its source already links it today" register, proven in the
  // browser (Controller adjudication ADJ-44, 2026-07-16). The .first()
  // locator pins the title specifically: a card that stopped emitting the
  // corpus link would fail this href assertion, never fall through to the
  // board anchor (/board/spec/<name>).
  for (const name of [ACCEPTED_SPEC, ACTIVE_SPEC, TERMINAL_SPEC, EDGE.DIR_CLOSED_AWAITING_ARCHIVE, ARCHIVED_SPEC]) {
    await expect(entry(page, name).locator("a").first()).toHaveAttribute("href", `/a/spec/${name}`);
  }
  // ...and the /b/<branch-escaped>/ grammar for design-branch drafts — the
  // entry's title IS its one link (dc-3), no separate board anchor.
  for (const name of [SHOWCASE.DESIGN_SPEC, SHOWCASE.DIR_LOCAL_DRAFT, SHOWCASE.DIR_REMOTE_DRAFT]) {
    await expect(entry(page, name).locator("a")).toHaveAttribute("href", draftBoardHref(name));
  }

  // (e) matrix+verdict appear ONLY on a default-branch feature entry
  // carrying a story ref.
  await expect(entry(page, ACCEPTED_SPEC).locator(`a[href="/matrix/${ACCEPTED_STORY}"]`)).toHaveCount(1);
  await expect(entry(page, ACCEPTED_SPEC).locator(`a[href="/verdict/${ACCEPTED_STORY}"]`)).toHaveCount(1);
  await expect(
    entry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(`a[href="/matrix/${CLOSED_AWAITING_ARCHIVE_STORY}"]`),
  ).toHaveCount(1);
  await expect(
    entry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(`a[href="/verdict/${CLOSED_AWAITING_ARCHIVE_STORY}"]`),
  ).toHaveCount(1);
  // Negative: never on a component entry, never on a design-branch draft,
  // never on the archive-zone entry whose board the routing does not serve.
  for (const name of [ACTIVE_SPEC, TERMINAL_SPEC, SHOWCASE.DESIGN_SPEC, ARCHIVED_SPEC]) {
    await expect(entry(page, name).locator('a[href^="/matrix/"]')).toHaveCount(0);
    await expect(entry(page, name).locator('a[href^="/verdict/"]')).toHaveCount(0);
  }

  // The unprefixed default-branch board link genuinely serves (live by
  // construction).
  await entry(page, ACCEPTED_SPEC).locator("a.dir-board").click();
  await expect(page.getByTestId("board")).toBeVisible();
});

// AC-2 (kept by dc-12 as the no-loss bar): every entry and section the
// directory rendered before the glance landed is still present, carrying
// the same content — the columns hold every entry exactly once, and the
// other home sections survive beneath them.
test("every pre-existing directory section and link survives unchanged alongside the new glance", async ({
  page,
}) => {
  await page.goto("/");

  // The four status groups, now the columns.
  for (const [g] of INDEX_COLUMNS) {
    await expect(group(page, g)).toBeVisible();
  }
  // Every fixture entry is in the columns, exactly once — the former
  // glance's second rendering of the same entries is gone (dc-12).
  for (const name of [
    SHOWCASE.DESIGN_SPEC,
    ACCEPTED_SPEC,
    ACTIVE_SPEC,
    TERMINAL_SPEC,
    EDGE.DIR_CLOSED_AWAITING_ARCHIVE,
  ]) {
    await expect(entry(page, name)).toHaveCount(1);
    await expect(entry(page, name)).toBeVisible();
  }

  // The archive-zone entry — once absent from the glance — is a card on
  // the shelf, in the SAME columns as everything else (dc-12's zone rule:
  // archived specs never lead; the no-loss bar: still listed), folded in
  // the shelf's collapsed archived list (spec/index-v2 ac-5; SI-366 (8)),
  // which is opened here so the card can be seen.
  await expect(entry(page, ARCHIVED_SPEC)).toHaveCount(1);
  await group(page, "terminal").locator("details.dir-archived > summary").click();
  await expect(group(page, "terminal").getByTestId(dirEntryTestId(ARCHIVED_SPEC))).toBeVisible();
  await expect(entry(page, ARCHIVED_SPEC).locator(".badge-closed")).toHaveText("closed");

  // Final fix wave I5: the card carries the compatibility note beside its
  // badge — one disclosure, never stripped on a proven state.
  await expect(entry(page, EDGE.DIR_CLOSED_AWAITING_ARCHIVE).locator(".dir-disclosed")).toContainText(
    "compatibility reading",
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

// AC-3/DC-4/CO-1 (kept by dc-12 and index-v2 ac-1): a column with zero
// matching entries still renders its heading, its zero count, and an
// explicit empty state — never a silently omitted column. Driven against
// a separate, isolated REAL store (git init + .verdi/verdi.yaml, zero
// specs) computed through the real refindex.ComputeIndex pipeline
// (Controller adjudication ADJ-40) — an empty store proves all four empty
// columns at once through the true pipe. See this file's header comment
// for why this is isolated rather than mutating the shared corpus.
test("an isolated real store with zero specs renders every glance bucket's heading, zero count, and empty-state notice", async ({
  page,
}) => {
  const res = await page.request.get(EMPTY_INDEX_FIXTURE_URL);
  expect(res.ok()).toBe(true);
  const isolatedURL = (await res.text()).trim();
  expect(isolatedURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);

  await page.goto(isolatedURL);
  await expect(page.locator(".home-directory")).toBeVisible();

  // Zero specs in the store, so all four columns are empty at once — the
  // strongest proof of the kept dc-4: an operator reads absence-of-work as
  // an explicit, deliberate fact in every column, not a broken render.
  // (The populated-column contrast dc-4 also values is proven by this
  // file's first test, over the shared store.)
  for (const [slug, heading] of INDEX_COLUMNS) {
    const col = group(page, slug);
    await expect(col).toBeVisible();
    await expect(col.locator("h2")).toContainText(heading);
    await expect(col.locator(".count")).toHaveText("0");
    await expect(col.locator(".dir-empty")).toBeVisible();
  }
  // Nothing to badge or link: not a single card renders through the real
  // pipeline over an empty store.
  await expect(page.locator(".dir-entry")).toHaveCount(0);
});
