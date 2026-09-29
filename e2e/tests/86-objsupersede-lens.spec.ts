import { test, expect, request, type Locator, type Page } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";

// Closed-spec object supersession on the dex feature lens (design
// docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-design.md
// §6, §8; whole-wave review F-8; lane L7's stopped item, unblocked by lane
// L6's `feature-criterion` store): a closed FEATURE's artifact page carries
// the feature lens (05 §Lenses), whose "Current mapping" table has one row
// per acceptance criterion. A superseded criterion's row renders the
// criterion's ORIGINAL text, unchanged, then the objsupersede views' §6
// lines from the same views its document renders (internal/dex/featurelens.go,
// objectSupersessionHTML): "governed spec/T's completed work (closed
// <date>)" and "superseded since <date> by spec/S#<decision-id>" linking
// S's decision and the conflict. A criterion the views do not supersede
// renders its bare row.
//
// The store is the control server's `feature-criterion` objsupersede
// store (cmd/e2eharness/objsupersedefixture.go): the accepted successor's
// dc-3 supersedes the closed feature's ac-1, in force. The page is the
// artifact permalink `{docs_url}a/spec/closed-feature/` (05 §Verdi-dex
// "Mechanics", the grammar fixtures.ts's dexSpecPath encodes), composed
// from the fixture's docs_url as 82-objsupersede-docs.spec.ts composes
// its 404 probe; the lens lives on the artifact page, not the document
// page the fixture's object_docs_url names.
//
// Paired negative — a disclosed deviation from the brief. The brief asks
// for "another criterion row on the same page, not superseded". The
// closed feature declares exactly ONE acceptance criterion (ac-1;
// testdata/objsupersede/records/specs/closed-feature.md), so its lens
// has one row and no second row exists on that page; test 1 pins the
// row count at 1 so the fact is witnessed, not assumed. The negatives
// used instead are the closest the fixture offers without a fixture
// change: the SAME row on the SAME page in the `accepted` store, where
// ac-1 is not superseded (same renderer, same page, different records:
// no lines), and the other feature's lens row in the same store. Both
// prove the `lens-supersession-*` selector's absence is meaningful on a
// row the same renderer produced.
//
// Assertions are on test ids, data-state attributes, link targets, and
// the row's own text — never the innerText of a whole page; the line
// texts are pinned by internal/dex/objsupersede_test.go's
// TestFeatureLensHTML_RendersSupersededCriterionLines.

const FIXTURE_URL = `${CONTROL_URL}/objsupersede-fixture`;

// The closed feature's criterion text, from the committed scenario record
// (testdata/objsupersede/records/specs/closed-feature.md) — pinned here
// independently of any rendered surface.
const CLOSED_FEATURE = "spec/closed-feature";
const CRITERION = `${CLOSED_FEATURE}#ac-1`;
const CRITERION_TEXT = "an operator can read the governed records";

interface BoardView {
  branch: string;
  spec: string;
  url: string;
  not_a_surface: string;
}

interface Supersession {
  object: string;
  object_docs_url: string;
  object_board: BoardView;
  decision: string;
  decision_docs_url: string;
  establishing_decision: string;
  establishing_decision_docs_url: string;
  conflict: string;
  conflict_docs_url: string;
}

interface Store {
  scenario: string;
  url: string;
  docs_url: string;
  docs_commit: string;
  checkout: string;
  main_branch: string;
  design_branch: string;
  successor: string;
  establishing_successor: string;
  supersessions: Supersession[];
  boards: { checkout: BoardView; design: BoardView; main: BoardView };
  docs: Record<string, string>;
}

interface Fixture {
  stores: Record<string, Store>;
}

// The fixture's cold start builds the binary and eight stores; warm it
// once under its own allowance (the 49-readiness-pilot pattern).
test.beforeAll(async () => {
  test.setTimeout(240_000);
  const api = await request.newContext();
  try {
    const res = await api.get(FIXTURE_URL, { timeout: 240_000 });
    expect(res.ok(), await res.text()).toBe(true);
  } finally {
    await api.dispose();
  }
});

async function fixture(page: Page): Promise<Fixture> {
  const res = await page.request.get(FIXTURE_URL);
  expect(res.ok(), await res.text()).toBe(true);
  return (await res.json()) as Fixture;
}

function store(f: Fixture, name: string): Store {
  const s = f.stores[name];
  expect(s, `fixture has no store ${name}`).toBeTruthy();
  return s;
}

// artifactPage is a spec's artifact permalink on the store's docs site:
// "/a/spec/<name>/", where the feature lens renders.
function artifactPage(st: Store, spec: string): string {
  return `${st.docs_url}a/${spec}/`;
}

// corpusHref is the address every document consumer serves for a ref the
// §6 lines name (internal/specdocload's SupersessionLink): "/a/<kind>/<name>",
// with the object's anchor for a spec's object.
function corpusHref(ref: string): string {
  const hash = ref.indexOf("#");
  return hash < 0 ? `/a/${ref}` : `/a/${ref.slice(0, hash)}#${ref.slice(hash + 1)}`;
}

// lensRows are the "Current mapping" table's criterion rows; lensRow is
// the one whose AC cell reads exactly id.
function lensRows(page: Page): Locator {
  return page.getByTestId("live-mapping").locator("tbody tr");
}
function lensRow(page: Page, id: string): Locator {
  return lensRows(page).filter({ has: page.locator("td:first-child code", { hasText: new RegExp(`^${id}$`) }) });
}

// ownText is the cell's own text — its text nodes and inline children,
// any nested list (the §6 lines) left out, whitespace collapsed — so the
// criterion's original text is pinned exactly, not merely contained.
function ownText(el: Locator): Promise<string> {
  return el.evaluate((node) =>
    Array.from(node.childNodes)
      .filter((n) => n.nodeName !== "UL" && n.nodeName !== "OL")
      .map((n) => n.textContent ?? "")
      .join("")
      .replace(/\s+/g, " ")
      .trim(),
  );
}

// expectServed proves a link carries the corpus address of the ref it
// names and that the docs site answers that address (200; the site's
// directory-form permalink answers the address without its slash).
async function expectServed(page: Page, link: Locator, ref: string) {
  await expect(link).toBeVisible();
  await expect(link).toHaveAttribute("href", corpusHref(ref));
  const res = await page.request.get(new URL(corpusHref(ref).replace(/#.*$/, ""), page.url()).href);
  expect(res.status(), `GET ${corpusHref(ref)}`).toBe(200);
}

test.describe("closed-spec object supersession on the dex feature lens", () => {
  test("feature-criterion: the closed feature's lens row keeps the criterion's text and carries the §6 lines, the since link resolving to S's decision", async ({ page }) => {
    const st = store(await fixture(page), "feature-criterion");
    const s = st.supersessions.find((x) => x.object === CRITERION);
    expect(s, "the fixture supersedes the closed feature's criterion").toBeTruthy();
    expect((s as Supersession).establishing_decision).toBe(`${st.establishing_successor}#dc-3`);
    const since = s as Supersession;

    await page.goto(artifactPage(st, CLOSED_FEATURE));
    await expect(page.getByTestId("acceptance-plan-banner")).toBeVisible();
    // The closed feature declares one criterion: one row (see the header's
    // paired-negative note).
    await expect(lensRows(page)).toHaveCount(1);
    const row = lensRow(page, "ac-1");
    await expect(row).toHaveCount(1);

    // The criterion's original text, unchanged, is the text cell's own
    // text; the lines follow it inside the cell.
    const textCell = row.locator("td").nth(1);
    expect(await ownText(textCell)).toBe(CRITERION_TEXT);
    const lines = textCell.getByTestId("lens-supersession-ac-1");
    await expect(lines).toHaveCount(1);
    await expect(lines).toHaveAttribute("data-state", "superseded");
    await expect(lines.locator("li")).toHaveCount(2);

    const governed = lines.getByTestId("objsupersede-ac-1-governed");
    await expect(governed).toHaveAttribute("data-state", "superseded");
    await expect(governed).toHaveText(`governed ${CLOSED_FEATURE}'s completed work (closed 2024-01-10)`);

    const sinceLine = lines.getByTestId("objsupersede-ac-1-since");
    await expect(sinceLine).toHaveAttribute("data-state", "superseded");
    await expect(sinceLine).toHaveText(`superseded since 2024-02-15 by ${since.establishing_decision}`);
    const decisionLink = sinceLine.getByRole("link", { name: since.establishing_decision, exact: true });
    await expectServed(page, decisionLink, since.establishing_decision);

    const conflict = lines.getByTestId("objsupersede-ac-1-conflict");
    await expect(conflict).toHaveText(since.conflict);
    await expectServed(page, conflict, since.conflict);

    // Exactly those three elements: no carry line (the establishing
    // successor heads its chain). The count of 3 is the positive control
    // for the 0-count on the same prefix.
    await expect(lines.locator('[data-testid^="objsupersede-ac-1-"]')).toHaveCount(3);
    await expect(lines.getByTestId("objsupersede-ac-1-carry")).toHaveCount(0);

    // The since link lands: S's artifact page answers the corpus address
    // (directory-form permalink, anchor kept) and carries the decision's
    // anchor.
    await decisionLink.click();
    await expect.poll(() => new URL(page.url()).pathname.replace(/\/$/, "") + new URL(page.url()).hash).toBe(corpusHref(since.establishing_decision));
    await expect(page.locator("#dc-3")).toHaveCount(1);
  });

  test("paired negative (deviation: the closed feature has one criterion row, so a second row on that page does not exist): the same row unsuperseded, and another feature's row, carry no lens lines", async ({ page }) => {
    const f = await fixture(page);

    // The same page, the same row, records that supersede only the
    // feature's decision and the story's criterion (the accepted store):
    // the row renders its bare text and no lines.
    const acc = store(f, "accepted");
    expect(acc.supersessions.map((x) => x.object)).not.toContain(CRITERION);
    await page.goto(artifactPage(acc, CLOSED_FEATURE));
    await expect(page.getByTestId("live-mapping")).toBeVisible();
    await expect(lensRows(page)).toHaveCount(1);
    const bare = lensRow(page, "ac-1");
    await expect(bare).toHaveCount(1);
    expect(await ownText(bare.locator("td").nth(1))).toBe(CRITERION_TEXT);
    await expect(bare.locator('[data-testid^="lens-supersession-"]')).toHaveCount(0);
    await expect(page.locator('[data-testid^="lens-supersession-"], [data-testid^="objsupersede-"]')).toHaveCount(0);

    // The same store as test 1, another feature's lens: rows, no lines.
    const fc = store(f, "feature-criterion");
    await page.goto(artifactPage(fc, "spec/other-feature"));
    await expect(page.getByTestId("live-mapping")).toBeVisible();
    expect(await lensRows(page).count()).toBeGreaterThan(0);
    await expect(page.locator('[data-testid^="lens-supersession-"]')).toHaveCount(0);
  });
});
