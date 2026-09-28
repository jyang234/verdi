import { test, expect, request, type Locator, type Page } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";

// Closed-spec object supersession on the board's Document tab (design
// docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-design.md
// §6, §8; spec/spec-documents ac-6; SI-280; whole-wave review F-8). The
// tab renders the one document the docs site, `verdi spec doc`, and MCP
// get_document render (cmd/verdi's TestDocumentParity_ClosedSpecObject-
// Supersession pins the bytes), so on the SERVING board it carries the
// successor's decision view — "supersedes spec/T#<object>", in force,
// linking the closed object's corpus page — and, for a closed spec, the
// object's ORIGINAL text followed by §6's lines: "governed spec/T's
// completed work (closed <date>)" and "superseded since <date> by
// spec/S#<decision-id>" linking S's decision and the conflict, every link
// at the corpus address the serving checkout answers. A closed spec's
// board itself 404s on the archive zone (ADJ-39); its Document tab is the
// archived reading's one board-side surface. On a PER-BRANCH board
// (/b/<branch>) the same lines render with the unservable /a/ hrefs
// omitted (SI-280; ADJ-70): same test ids, same states, same text, no
// corpus link — and the trailing conflict anchor, a link or nothing, is
// omitted as it is on per-branch cards (83-objsupersede-board.spec.ts's
// conflictHref === null posture).
//
// Every board is one the control server's objsupersede fixture hands out
// (cmd/e2eharness/objsupersedefixture.go). The successor's tab is reached
// through the board's own tab link; a closed spec has no board, so its
// tab is addressed by the tab's route, `<board>/document`, composed from
// the fixture's board URL (documentTabOf). Assertions are on test ids,
// data-state attributes, and link targets — never the innerText of a
// whole page; the line texts are pinned by the Go tests
// (internal/workbench/objsupersedefix_test.go, internal/dex/objsupersede_test.go).

const FIXTURE_URL = `${CONTROL_URL}/objsupersede-fixture`;

// The closed objects' declared texts, from the committed scenario records
// (testdata/objsupersede/records/specs/closed-*.md) — pinned here
// independently of any rendered surface.
const OBJECT_TEXT: Record<string, string> = {
  "spec/closed-feature#dc-1": "the governed records are listed newest first",
  "spec/closed-story#ac-1": "the record list renders every governed record",
};

// The two closed objects the accepted scenario's records supersede, in
// the fixture's order (the M-3 guard: a loop over an empty list proves
// nothing).
const CLOSED_OBJECTS = ["spec/closed-feature#dc-1", "spec/closed-story#ac-1"];

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

// The fixture's cold start builds the binary and seven stores; warm it
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

// objectId is "spec/closed-feature#dc-1" -> "dc-1"; specOf is the
// object's spec, "spec/closed-feature".
function objectId(ref: string): string {
  return ref.slice(ref.indexOf("#") + 1);
}
function specOf(ref: string): string {
  return ref.slice(0, ref.indexOf("#"));
}

// decisionStem is a decision view's line-testid stem on the successor's
// document: "<decision-id>-<object ref flattened>", as internal/specdoc's
// supersessionStem flattens it.
function decisionStem(decision: string, object: string): string {
  return `${objectId(decision)}-${object.replace(/[/#]/g, "-")}`;
}

// corpusHref is the one address every document consumer serves for a ref
// the §6 lines name (internal/specdocload's SupersessionLink): the
// artifact's corpus page, "/a/<kind>/<name>", with the object's anchor
// for a spec's object.
function corpusHref(ref: string): string {
  const hash = ref.indexOf("#");
  return hash < 0 ? `/a/${ref}` : `/a/${ref.slice(0, hash)}#${ref.slice(hash + 1)}`;
}

// documentTabOf is the Document tab's route for spec on the board whose
// URL the fixture handed out: the same mount (serving or /b/<branch>),
// the spec segment swapped, "/document" appended — the address the board
// page's own tab link carries (boardspec.go: EscapedPath + "/document").
function documentTabOf(boardURL: string, spec: string): string {
  expect(boardURL).toMatch(/\/board\/spec\/[^/]+$/);
  return boardURL.replace(/\/board\/spec\/[^/]+$/, `/board/spec/${spec.replace(/^spec\//, "")}/document`);
}

// The object's own text renders unchanged at its anchor: the h3 (a
// decision) or the top-level list item (a criterion, whose anchor sits
// inside the item's paragraph) that holds `<a id="<id>">`.
function objectText(region: Locator, id: string) {
  return region.locator(`h3:has(a#${id}), li:has(a#${id})`).first();
}

// expectServedHref proves a rendered link carries the corpus address of
// the ref it names AND that the serving checkout answers it (200).
async function expectServedHref(page: Page, link: Locator, ref: string) {
  await expect(link).toBeVisible();
  await expect(link).toHaveAttribute("href", corpusHref(ref));
  const res = await page.request.get(new URL(corpusHref(ref).replace(/#.*$/, ""), page.url()).href);
  expect(res.status(), `GET ${corpusHref(ref)}`).toBe(200);
}

// lineFacts reads every §6 LINE span in the document — its test id, its
// state, and its text — in document order. The trailing conflict anchor
// is not a line span (it is an <a>), so it is read separately.
type LineFact = [testid: string, state: string, text: string];
function lineFacts(region: Locator): Promise<LineFact[]> {
  return region.locator('span[data-testid^="objsupersede-"]').evaluateAll((els) =>
    els.map((el) => [el.getAttribute("data-testid") ?? "", el.getAttribute("data-state") ?? "", el.textContent ?? ""] as LineFact),
  );
}

test.describe("closed-spec object supersession on the board's Document tab", () => {
  test("serving board: the successor's tab carries the decision view in force, linking the closed object's corpus page", async ({ page }) => {
    const st = store(await fixture(page), "accepted");
    expect(st.supersessions.map((s) => s.object)).toEqual(CLOSED_OBJECTS);
    // Through the board's own tab link, never a composed address.
    await page.goto(st.boards.checkout.url);
    await page.getByTestId("board-tab-document").click();
    await expect(page).toHaveURL(/\/board\/spec\/successor\/document$/);
    await expect(page.getByTestId("document-kind-spec")).toHaveAttribute("aria-current", "page");
    const region = page.getByTestId("document-region");
    for (const s of st.supersessions) {
      const stem = decisionStem(s.decision, s.object);
      const edge = region.getByTestId(`objsupersede-${stem}-edge`);
      await expect(edge).toHaveAttribute("data-state", "in-force");
      await expect(edge).toHaveText(`supersedes ${s.object}`);
      await expectServedHref(page, edge.getByRole("link", { name: s.object, exact: true }), s.object);
      // The establishing successor heads its chain: no carrying line, and
      // never a reason.
      await expect(region.getByTestId(`objsupersede-${stem}-carries`)).toHaveCount(0);
      await expect(region.getByTestId(`objsupersede-${stem}-not-established`)).toHaveCount(0);
    }
  });

  test("serving board: a closed spec's tab carries the object's text and §6 lines with live links, though its board 404s (ADJ-39)", async ({ page }) => {
    const st = store(await fixture(page), "accepted");
    expect(st.supersessions.map((s) => s.object)).toEqual(CLOSED_OBJECTS);
    for (const s of st.supersessions) {
      const tab = documentTabOf(st.boards.checkout.url, specOf(s.object));
      // The archived spec has no board (ADJ-39) …
      expect((await page.request.get(tab.replace(/\/document$/, ""))).status()).toBe(404);
      // … and its Document tab is the archived reading.
      await page.goto(tab);
      const region = page.getByTestId("document-region");
      const id = objectId(s.object);
      await expect(objectText(region, id)).toContainText(OBJECT_TEXT[s.object]);

      const governed = region.getByTestId(`objsupersede-${id}-governed`);
      await expect(governed).toHaveAttribute("data-state", "superseded");
      await expect(governed).toHaveText(`governed ${specOf(s.object)}'s completed work (closed 2024-01-10)`);

      const since = region.getByTestId(`objsupersede-${id}-since`);
      await expect(since).toHaveAttribute("data-state", "superseded");
      await expect(since).toHaveText(`superseded since 2024-02-15 by ${s.establishing_decision}`);
      expect(s.establishing_decision.startsWith(st.establishing_successor + "#")).toBe(true);
      await expectServedHref(page, since.getByRole("link", { name: s.establishing_decision, exact: true }), s.establishing_decision);

      const conflict = region.getByTestId(`objsupersede-${id}-conflict`);
      await expect(conflict).toHaveText(s.conflict);
      await expectServedHref(page, conflict, s.conflict);

      // The establishing successor heads its chain: no carrying line.
      await expect(region.getByTestId(`objsupersede-${id}-carry`)).toHaveCount(0);
    }
  });

  test("per-branch board: the same lines with the unservable corpus hrefs omitted (SI-280)", async ({ page }) => {
    const st = store(await fixture(page), "accepted");
    expect(st.boards.design.url).toContain(`/b/${encodeURIComponent(st.design_branch)}/`);
    // The per-branch board offers its Document tab through the same link,
    // under the branch mount.
    await page.goto(st.boards.design.url);
    await page.getByTestId("board-tab-document").click();
    await expect(page).toHaveURL(new RegExp(`/b/${encodeURIComponent(st.design_branch)}/board/spec/successor/document$`));

    // The successor's document and both closed specs': the serving tab's
    // lines, links live, then the branch tab's — identical spans, no /a/
    // href anywhere in the document, no trailing conflict anchor.
    const specs = [st.successor, ...CLOSED_OBJECTS.map(specOf)];
    for (const spec of specs) {
      await page.goto(documentTabOf(st.boards.checkout.url, spec));
      const serving = page.getByTestId("document-region");
      const want = await lineFacts(serving);
      expect(want.length, `${spec}'s serving tab carries lines`).toBeGreaterThan(0);
      expect(await serving.locator('a[href^="/a/"]').count(), `${spec}'s serving tab links corpus pages`).toBeGreaterThan(0);
      const servingConflicts = await serving.locator('a[data-testid$="-conflict"]').count();

      await page.goto(documentTabOf(st.boards.design.url, spec));
      const branch = page.getByTestId("document-region");
      expect(await lineFacts(branch)).toEqual(want);
      await expect(branch.locator('a[href^="/a/"]')).toHaveCount(0);
      await expect(branch.locator('span[data-testid^="objsupersede-"] a')).toHaveCount(0);
      // The link-only trailing conflict anchor is omitted with its href
      // (spec 83's per-branch cards carry none either); the closed specs
      // have one on the serving tab, the successor's document none.
      expect(servingConflicts).toBe(spec === st.successor ? 0 : 1);
      await expect(branch.locator('a[data-testid$="-conflict"]')).toHaveCount(0);
    }
  });
});
