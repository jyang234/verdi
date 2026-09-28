import { test, expect, request, type Page } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";

// Closed-spec object supersession on the docs site (design
// docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-design.md
// §6, §8; SI-263): a closed spec's document renders a superseded
// criterion or decision's ORIGINAL text unchanged, then the objsupersede
// views' lines — "governed spec/T's completed work (closed <date>)",
// "superseded since <date> by spec/S#<decision-id>" linking to S's
// decision and to the conflict, and the carrying line when a later
// revision carries or drops the replacement — and a successor's decision
// renders its decision view. Default-branch surfaces compute from
// default-branch records only: a successor not yet accepted shows nothing
// on the closed object, and records that do not match read "supersession
// not established: <reason>" on the successor's decision, never as a
// supersession.
//
// Every store is one of the six scenario repositories the control server
// provisions on demand (cmd/e2eharness/objsupersedefixture.go, lane L3d):
// each with its own `verdi serve` and a docs site built from main. The
// fixture JSON hands out every URL; nothing here composes one. Assertions
// are on test ids, data-state attributes, and link targets — never the
// innerText of a whole page. The texts themselves are pinned exactly by
// the Go render tests (internal/dex/objsupersede_test.go); here the
// browser proves the same markup reaches the DOM with its links live.

const FIXTURE_URL = `${CONTROL_URL}/objsupersede-fixture`;

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
  conflicts: string[];
  supersessions: Supersession[];
  boards: { checkout: BoardView; design: BoardView; main: BoardView };
  docs: Record<string, string>;
}

interface Fixture {
  stores: Record<string, Store>;
}

// The first GET builds the verdi binary and provisions six stores (each a
// materialized repository, a docs build, and a serve) — tens of seconds
// cold — so warm the fixture ONCE under its own allowance, the pattern
// 49-readiness-pilot.spec.ts uses; every test then finds it up.
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

// objectId is the object's id: "spec/closed-feature#dc-1" -> "dc-1".
function objectId(ref: string): string {
  return ref.slice(ref.indexOf("#") + 1);
}

// decisionStem is the data-testid stem of a decision view on the
// successor's document: "<decision-id>-<object ref flattened>", as
// internal/specdoc's supersessionStem flattens it.
function decisionStem(decision: string, object: string): string {
  return `${objectId(decision)}-${object.replace(/[/#]/g, "-")}`;
}

// The object's own text renders unchanged at its anchor: the h3 (a
// decision) or the top-level list item (a criterion, whose anchor sits
// inside the item's paragraph) that holds `<a id="<id>">`.
function objectText(page: Page, id: string) {
  return page.locator(`h3:has(a#${id}), li:has(a#${id})`).first();
}

// corpusHref is the one address every document consumer serves for a
// ref the §6 lines name (internal/specdocload's SupersessionLink; the
// links must be identical across the four renders, spec/spec-documents
// ac-6): the artifact's corpus page, "/a/<kind>/<name>", with the
// object's anchor for a spec's object.
function corpusHref(ref: string): string {
  const hash = ref.indexOf("#");
  return hash < 0 ? `/a/${ref}` : `/a/${ref.slice(0, hash)}#${ref.slice(hash + 1)}`;
}

// expectSameTarget compares a rendered href (site-absolute) with the
// corpus address of the ref it links, and proves the address serves: the
// docs site's directory-form permalink answers it (by its redirect).
async function expectSameTarget(page: Page, link: ReturnType<Page["locator"]>, ref: string) {
  await expect(link).toHaveAttribute("href", corpusHref(ref));
  const res = await page.request.get(new URL(corpusHref(ref).replace(/#.*$/, ""), page.url()).href);
  expect(res.status(), `GET ${corpusHref(ref)}`).toBe(200);
}

// expectSuperseded asserts §6's lines on one closed object's document:
// original text, governed, since/by linking to S's decision and to the
// conflict, and nothing else unless the caller asks for the carry line.
async function expectSuperseded(page: Page, s: Supersession, establishingSpec: string) {
  const id = objectId(s.object);
  await page.goto(s.object_docs_url);
  await expect(objectText(page, id)).toBeVisible();

  const governed = page.getByTestId(`objsupersede-${id}-governed`);
  await expect(governed).toHaveAttribute("data-state", "superseded");
  const closedSpec = s.object.slice(0, s.object.indexOf("#"));
  await expect(governed).toContainText(`governed ${closedSpec}'s completed work (closed `);

  const since = page.getByTestId(`objsupersede-${id}-since`);
  await expect(since).toHaveAttribute("data-state", "superseded");
  await expect(since).toContainText("superseded since ");
  await expect(since).toContainText(` by ${s.establishing_decision}`);
  expect(s.establishing_decision.startsWith(establishingSpec + "#")).toBe(true);
  const decisionLink = since.getByRole("link", { name: s.establishing_decision, exact: true });
  await expect(decisionLink).toBeVisible();
  await expectSameTarget(page, decisionLink, s.establishing_decision);

  const conflictLink = page.getByTestId(`objsupersede-${id}-conflict`);
  await expect(conflictLink).toHaveText(s.conflict);
  await expectSameTarget(page, conflictLink, s.conflict);
}

// The two closed objects every scenario's records supersede, in the
// fixture's order (the M-3 guard: a loop over an empty list proves
// nothing).
const CLOSED_OBJECTS = ["spec/closed-feature#dc-1", "spec/closed-story#ac-1"];

test.describe("closed-spec object supersession on the docs site", () => {
  test("after acceptance: original text, governed, since/by with links to S's decision and the conflict", async ({ page }) => {
    const st = store(await fixture(page), "accepted");
    expect(st.supersessions.map((s) => s.object)).toEqual(CLOSED_OBJECTS);
    for (const s of st.supersessions) {
      await expectSuperseded(page, s, st.establishing_successor);
      // No carrying line: the establishing successor heads its chain.
      await expect(page.getByTestId(`objsupersede-${objectId(s.object)}-carry`)).toHaveCount(0);
      // Both links land: S's decision heading exists on its artifact page
      // (the permalink is directory-form, so the site answers the corpus
      // address with a trailing slash, the anchor kept), and the conflict
      // page names the conflict.
      await page.getByTestId(`objsupersede-${objectId(s.object)}-since`).getByRole("link").click();
      const decisionId = objectId(s.establishing_decision);
      await expect.poll(() => new URL(page.url()).pathname.replace(/\/$/, "") + new URL(page.url()).hash).toBe(corpusHref(s.establishing_decision));
      await expect(page.locator(`#${decisionId}`)).toHaveCount(1);
      await page.goto(new URL(corpusHref(s.conflict), page.url()).href);
      // The artifact page's shell and body each carry an h1 naming the conflict.
      await expect(page.getByRole("heading", { level: 1 }).first()).toContainText(s.conflict.slice("conflict/".length));
    }
    // The successor's decisions carry the decision view, in force, linking
    // to the closed object.
    for (const s of st.supersessions) {
      await page.goto(s.decision_docs_url);
      const edge = page.getByTestId(`objsupersede-${decisionStem(s.decision, s.object)}-edge`);
      await expect(edge).toHaveAttribute("data-state", "in-force");
      await expect(edge).toHaveText(`supersedes ${s.object}`);
      await expectSameTarget(page, edge.getByRole("link", { name: s.object, exact: true }), s.object);
      await expect(page.getByTestId(`objsupersede-${decisionStem(s.decision, s.object)}-carries`)).toHaveCount(0);
    }
  });

  test("carried through revisions: S1's date and conflict kept, carried by the latest revision", async ({ page }) => {
    const st = store(await fixture(page), "chain");
    expect(st.successor).toBe("spec/successor-v3");
    expect(st.supersessions.map((s) => s.object)).toEqual(CLOSED_OBJECTS);
    for (const s of st.supersessions) {
      await expectSuperseded(page, s, st.establishing_successor);
      const carry = page.getByTestId(`objsupersede-${objectId(s.object)}-carry`);
      await expect(carry).toHaveAttribute("data-state", "superseded");
      await expect(carry).toHaveText(`carried by ${st.successor}`);
      await expectSameTarget(page, carry.getByRole("link", { name: st.successor, exact: true }), st.successor);
      // The revision's own decision reads in force and names what it carries.
      await page.goto(s.decision_docs_url);
      const stem = decisionStem(s.decision, s.object);
      await expect(page.getByTestId(`objsupersede-${stem}-edge`)).toHaveAttribute("data-state", "in-force");
      const carries = page.getByTestId(`objsupersede-${stem}-carries`);
      await expect(carries).toHaveAttribute("data-state", "in-force");
      await expect(carries).toContainText(`carries the replacement established by ${st.establishing_successor} (${s.conflict}, since `);
      await expectSameTarget(page, carries.getByRole("link", { name: st.establishing_successor, exact: true }), st.establishing_successor);
      await expectSameTarget(page, carries.getByRole("link", { name: s.conflict, exact: true }), s.conflict);
    }
  });

  test("a revision drops the edge: still superseded, no longer carried by the current revision", async ({ page }) => {
    const st = store(await fixture(page), "chain-drop");
    const dropped = st.supersessions.find((s) => s.decision === "");
    const kept = st.supersessions.find((s) => s.decision !== "");
    expect(dropped?.object).toBe("spec/closed-feature#dc-1");
    expect(kept?.object).toBe("spec/closed-story#ac-1");
    await expectSuperseded(page, dropped as Supersession, st.establishing_successor);
    const droppedCarry = page.getByTestId(`objsupersede-${objectId((dropped as Supersession).object)}-carry`);
    await expect(droppedCarry).toHaveAttribute("data-state", "superseded");
    await expect(droppedCarry).toHaveText(`no longer carried by the current revision (${st.successor})`);
    await expectSameTarget(page, droppedCarry.getByRole("link", { name: st.successor, exact: true }), st.successor);
    await expectSuperseded(page, kept as Supersession, st.establishing_successor);
    await expect(page.getByTestId(`objsupersede-${objectId((kept as Supersession).object)}-carry`)).toHaveText(`carried by ${st.successor}`);
  });

  for (const name of ["proposed", "no-conflict"]) {
    test(`not yet accepted (${name}): the default branch's documents show nothing on the closed objects`, async ({ page }) => {
      const st = store(await fixture(page), name);
      expect(st.checkout).toBe(st.design_branch);
      expect(st.supersessions.map((s) => s.object)).toEqual(CLOSED_OBJECTS);
      for (const s of st.supersessions) {
        await page.goto(s.object_docs_url);
        await expect(objectText(page, objectId(s.object))).toBeVisible();
        await expect(page.locator('[data-testid^="objsupersede-"]')).toHaveCount(0);
        // Main carries no successor: its document is not on the site.
        expect(s.decision_docs_url).toBe("");
      }
      const res = await page.request.get(`${st.docs_url}a/${st.successor}/document/`);
      expect(res.status()).toBe(404);
    });
  }

  test("not established after acceptance: the reason on S's decision, never a supersession on the object", async ({ page }) => {
    const st = store(await fixture(page), "chain-not-in-force");
    const byObject = new Map(st.supersessions.map((s) => [s.object, s]));
    // The feature's objects: nothing, on both the decision and the criterion.
    for (const object of ["spec/closed-feature#dc-1", "spec/closed-feature#ac-1"]) {
      const s = byObject.get(object) as Supersession;
      await page.goto(s.object_docs_url);
      await expect(objectText(page, objectId(object))).toBeVisible();
      await expect(page.locator(`[data-testid^="objsupersede-${objectId(object)}-"]`)).toHaveCount(0);
    }
    // The story's criterion: superseded by the establishing successor's
    // matching edge.
    await expectSuperseded(page, byObject.get("spec/closed-story#ac-1") as Supersession, st.establishing_successor);
    // The establishing successor's decisions on main: the non-matching
    // records read as reasons, never as supersessions.
    for (const object of ["spec/closed-feature#dc-1", "spec/closed-feature#ac-1"]) {
      const s = byObject.get(object) as Supersession;
      await page.goto(s.establishing_decision_docs_url);
      const stem = decisionStem(s.establishing_decision, s.object);
      const notEstablished = page.getByTestId(`objsupersede-${stem}-not-established`);
      await expect(notEstablished).toHaveAttribute("data-state", "not-established");
      await expect(notEstablished).toContainText("supersession not established: ");
      await expect(notEstablished).toContainText("no conflict challenges spec/closed-feature#ac-1");
      await expect(page.getByTestId(`objsupersede-${stem}-edge`)).toHaveCount(0);
      await expect(page.locator(`[data-testid^="objsupersede-${stem}-"][data-state="in-force"]`)).toHaveCount(0);
    }
  });
});
