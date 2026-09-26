import { test, expect, request, type Locator, type Page } from "@playwright/test";
import { CONTROL_URL, refCardTestId } from "./fixtures";

// Closed-spec object supersession on the board (design
// docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-design.md
// §6, §8; SI-278): on any board whose spec links a closed spec's object,
// that object's reference card renders the object's ORIGINAL text and the
// objsupersede views' lines computed from default-branch records —
// "governed spec/T's completed work (closed <date>)", "superseded since
// <date> by spec/S#<decision-id>" linking to S's decision and to the
// conflict, and the carrying line when a later revision carries or drops
// the replacement — while the successor's own decision card carries the
// decision view from the board's own tree: "supersedes …" in force,
// "proposed — … when spec/S is accepted" on its design branch, or
// "supersession not established: <reason>", never a supersession. Every
// line shows AT REST — no hover, no focus, no clipping — and the layout
// reserves the height a grown card takes, so it covers no other card.
//
// Every board is one the control server's objsupersede fixture hands out
// (cmd/e2eharness/objsupersedefixture.go): each scenario store is its own
// repository with its own `verdi serve`; the fixture's `boards` and each
// supersession's `object_board` are the surfaces (a board 404s by design
// where `not_a_surface` says so, and is never visited). Assertions are on
// test ids, data-state attributes, link targets and bounding boxes; the
// texts are pinned by the Go tests (internal/workbench/objsupersede_test.go)
// and the closed objects' texts by the committed records below.

const FIXTURE_URL = `${CONTROL_URL}/objsupersede-fixture`;

// The closed objects' declared texts, from the committed scenario records
// (testdata/objsupersede/records/specs/closed-*.md) — pinned here
// independently of any rendered surface.
const OBJECT_TEXT: Record<string, string> = {
  "spec/closed-feature#dc-1": "the governed records are listed newest first",
  "spec/closed-feature#ac-1": "an operator can read the governed records",
  "spec/closed-story#ac-1": "the record list renders every governed record",
};

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

// The fixture's cold start builds the binary and six stores; warm it once
// under its own allowance (the 49-readiness-pilot pattern).
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

// objectId is "spec/closed-feature#dc-1" -> "dc-1".
function objectId(ref: string): string {
  return ref.slice(ref.indexOf("#") + 1);
}

// refStem is a reference card's line-testid stem: the ref with "/" and
// "#" flattened, as internal/workbench's refCardSupersessionStem does.
function refStem(ref: string): string {
  return ref.replace(/[/#]/g, "-");
}

// decisionStem is a decision card's line-testid stem:
// "<decision-id>-<object ref flattened>" (specdoc.SupersessionStem).
function decisionStem(decision: string, object: string): string {
  return `${objectId(decision)}-${refStem(object)}`;
}

async function expectHref(link: Locator, want: string) {
  await expect(link).toBeVisible();
  await expect(link).toHaveAttribute("href", want);
}

interface Box {
  x: number;
  y: number;
  width: number;
  height: number;
}

function inside(inner: Box, outer: Box, tolerance = 1): boolean {
  return (
    inner.x >= outer.x - tolerance &&
    inner.y >= outer.y - tolerance &&
    inner.x + inner.width <= outer.x + outer.width + tolerance &&
    inner.y + inner.height <= outer.y + outer.height + tolerance
  );
}

function intersects(a: Box, b: Box): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

// expectUnclipped proves the lines are readable AT REST: every line's box
// lies inside the card's box (nothing hidden by overflow or clamped
// height), and the card's box overlaps no other card's — the layout
// reserved the height the lines take. Boxes come from the browser's
// layout, with no hover or focus applied.
async function expectUnclipped(page: Page, card: Locator) {
  const cardBox = (await card.boundingBox()) as Box;
  expect(cardBox, "the card has a box").toBeTruthy();
  const lines = card.locator(".objsupersede-lines li");
  const n = await lines.count();
  expect(n, "the card carries lines").toBeGreaterThan(0);
  for (let i = 0; i < n; i++) {
    const box = (await lines.nth(i).boundingBox()) as Box;
    expect(box, `line ${i} has a box`).toBeTruthy();
    expect(inside(box, cardBox), `line ${i} ${JSON.stringify(box)} lies inside the card ${JSON.stringify(cardBox)}`).toBe(true);
  }
  const self = await card.getAttribute("data-testid");
  const others = page.locator(".objcard, .refcard, .stubcard");
  const m = await others.count();
  for (let i = 0; i < m; i++) {
    const other = others.nth(i);
    if ((await other.getAttribute("data-testid")) === self) continue;
    const box = await other.boundingBox();
    if (!box) continue;
    expect(intersects(cardBox, box), `${self} ${JSON.stringify(cardBox)} overlaps ${await other.getAttribute("data-testid")} ${JSON.stringify(box)}`).toBe(false);
  }
}

// expectSuperseded asserts the reference card of a closed object on the
// current page: the object modifier, the original text (the committed
// record's), §6's governed and since/by lines with the deciding decision
// and the conflict linked, and the whole card readable at rest.
async function expectSuperseded(page: Page, s: Supersession, decisionHref: string, conflictHref: string | null) {
  const card = page.getByTestId(refCardTestId(s.object));
  await expect(card).toBeVisible();
  await expect(card).toHaveClass(/refcard--object/);
  const text = card.getByTestId("refcard-object-text");
  await expect(text).toHaveText(OBJECT_TEXT[s.object]);
  const lines = card.getByTestId("refcard-supersession");
  await expect(lines).toHaveAttribute("data-state", "superseded");

  const stem = refStem(s.object);
  const closedSpec = s.object.slice(0, s.object.indexOf("#"));
  const governed = card.getByTestId(`objsupersede-${stem}-governed`);
  await expect(governed).toHaveAttribute("data-state", "superseded");
  await expect(governed).toContainText(`governed ${closedSpec}'s completed work (closed `);
  const since = card.getByTestId(`objsupersede-${stem}-since`);
  await expect(since).toHaveAttribute("data-state", "superseded");
  await expect(since).toContainText("superseded since ");
  await expect(since).toContainText(` by ${s.establishing_decision}`);
  await expectHref(since.getByRole("link", { name: s.establishing_decision, exact: true }), decisionHref);
  const conflict = card.getByTestId(`objsupersede-${stem}-conflict`);
  if (conflictHref === null) {
    await expect(conflict).toHaveCount(0);
  } else {
    await expect(conflict).toHaveText(s.conflict);
    await expectHref(conflict, conflictHref);
  }
  await expectUnclipped(page, card);
}

// expectUntouched asserts a closed object's reference card renders as
// every other reference card: no object text, no lines.
async function expectUntouched(page: Page, object: string) {
  const card = page.getByTestId(refCardTestId(object));
  await expect(card).toBeVisible();
  await expect(card).not.toHaveClass(/refcard--object/);
  await expect(card.getByTestId("refcard-object-text")).toHaveCount(0);
  await expect(page.locator(`[data-testid^="objsupersede-${refStem(object)}-"]`)).toHaveCount(0);
}

test.describe("closed-spec object supersession on the board", () => {
  test("after acceptance: the reference card shows text, governed, since/by with links; the decision card shows supersedes", async ({ page }) => {
    const st = store(await fixture(page), "accepted");
    expect(st.supersessions.map((s) => s.object)).toEqual(["spec/closed-feature#dc-1", "spec/closed-story#ac-1"]);
    await page.goto(st.boards.checkout.url);
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    for (const s of st.supersessions) {
      // S's decision is a card on this very board; the conflict has its
      // corpus page on the serving checkout.
      const decisionId = objectId(s.establishing_decision);
      await expectSuperseded(page, s, `#obj-${decisionId}`, `/a/${s.conflict}`);
      await expect(page.locator(`#obj-${decisionId}`)).toHaveCount(1);
      await expect(page.getByTestId(refCardTestId(s.object)).getByTestId(`objsupersede-${refStem(s.object)}-carry`)).toHaveCount(0);
      const conflictPage = await page.request.get(new URL(`/a/${s.conflict}`, page.url()).href);
      expect(conflictPage.status()).toBe(200);

      // The successor's decision card: the view in force, linking to the
      // archived object's corpus page (the board route 404s on the
      // archive zone) at the object's declared anchor; readable at rest.
      const card = page.getByTestId(`card-${decisionId}`);
      const lines = card.getByTestId(`card-supersession-${decisionId}`);
      await expect(lines).toBeVisible();
      const edge = lines.getByTestId(`objsupersede-${decisionStem(s.decision, s.object)}-edge`);
      await expect(edge).toHaveAttribute("data-state", "in-force");
      await expect(edge).toHaveText(`supersedes ${s.object}`);
      const closedSpec = s.object.slice(0, s.object.indexOf("#"));
      await expectHref(edge.getByRole("link", { name: s.object, exact: true }), `/a/${closedSpec}#${objectId(s.object)}`);
      const objectPage = await page.request.get(new URL(`/a/${closedSpec}`, page.url()).href);
      expect(objectPage.status()).toBe(200);
      await expect(lines.getByTestId(`objsupersede-${decisionStem(s.decision, s.object)}-carries`)).toHaveCount(0);
      await expectUnclipped(page, card);
    }
  });

  test("carried by S_n: the latest revision's board keeps S1's date and says it carries", async ({ page }) => {
    const st = store(await fixture(page), "chain");
    expect(st.successor).toBe("spec/successor-v3");
    expect(st.supersessions.map((s) => s.object)).toEqual(["spec/closed-feature#dc-1", "spec/closed-story#ac-1"]);
    await page.goto(st.boards.checkout.url);
    for (const s of st.supersessions) {
      const decisionId = objectId(s.establishing_decision);
      // S1 is another, active spec on main: its own board, at the card.
      await expectSuperseded(page, s, `/board/${st.establishing_successor}#obj-${decisionId}`, `/a/${s.conflict}`);
      const card = page.getByTestId(refCardTestId(s.object));
      const carry = card.getByTestId(`objsupersede-${refStem(s.object)}-carry`);
      await expect(carry).toHaveAttribute("data-state", "superseded");
      await expect(carry).toHaveText(`carried by ${st.successor}`);
      // The revision's own decision: in force, naming what it carries.
      const stem = decisionStem(s.decision, s.object);
      const lines = page.getByTestId(`card-supersession-${objectId(s.decision)}`);
      await expect(lines.getByTestId(`objsupersede-${stem}-edge`)).toHaveAttribute("data-state", "in-force");
      const carries = lines.getByTestId(`objsupersede-${stem}-carries`);
      await expect(carries).toHaveAttribute("data-state", "in-force");
      await expect(carries).toContainText(`carries the replacement established by ${st.establishing_successor} (${s.conflict}, since `);
      await expectHref(carries.getByRole("link", { name: st.establishing_successor, exact: true }), `/board/${st.establishing_successor}`);
      await expectHref(carries.getByRole("link", { name: s.conflict, exact: true }), `/a/${s.conflict}`);
      await expectUnclipped(page, page.getByTestId(`card-${objectId(s.decision)}`));
    }
    // The since link lands on S1's board, where the deciding card exists —
    // clicked at rest, with no hover: the lines are never clipped.
    const first = st.supersessions[0];
    await page.getByTestId(refCardTestId(first.object)).getByRole("link", { name: first.establishing_decision, exact: true }).click();
    await expect.poll(() => new URL(page.url()).pathname + new URL(page.url()).hash).toBe(`/board/${st.establishing_successor}#obj-${objectId(first.establishing_decision)}`);
    await expect(page.locator(`#obj-${objectId(first.establishing_decision)}`)).toHaveCount(1);
  });

  test("no longer carried: the establishing successor's board (the fixture's object_board) says so", async ({ page }) => {
    const st = store(await fixture(page), "chain-drop");
    const dropped = st.supersessions.find((s) => s.decision === "") as Supersession;
    expect(dropped.object).toBe("spec/closed-feature#dc-1");
    expect(dropped.object_board.url).not.toBe("");
    expect(dropped.object_board.spec).toBe(st.establishing_successor);
    await page.goto(dropped.object_board.url);
    const decisionId = objectId(dropped.establishing_decision);
    await expectSuperseded(page, dropped, `#obj-${decisionId}`, `/a/${dropped.conflict}`);
    const carry = page.getByTestId(refCardTestId(dropped.object)).getByTestId(`objsupersede-${refStem(dropped.object)}-carry`);
    await expect(carry).toHaveAttribute("data-state", "superseded");
    await expect(carry).toHaveText(`no longer carried by the current revision (${st.successor})`);
    await expectHref(carry.getByRole("link", { name: st.successor, exact: true }), `/board/${st.successor}`);
    // The establishing decision on this board still reads in force.
    await expect(page.getByTestId(`objsupersede-${decisionStem(dropped.establishing_decision, dropped.object)}-edge`)).toHaveAttribute("data-state", "in-force");
  });

  test("not yet accepted: the design board's decision card reads proposed; the object's reference card is untouched", async ({ page }) => {
    const st = store(await fixture(page), "proposed");
    expect(st.boards.main.not_a_surface).not.toBe("");
    expect(st.supersessions.map((s) => s.object)).toEqual(["spec/closed-feature#dc-1", "spec/closed-story#ac-1"]);
    await page.goto(st.boards.design.url);
    for (const s of st.supersessions) {
      await expectUntouched(page, s.object);
      const stem = decisionStem(s.decision, s.object);
      const edge = page.getByTestId(`card-supersession-${objectId(s.decision)}`).getByTestId(`objsupersede-${stem}-edge`);
      await expect(edge).toHaveAttribute("data-state", "proposed");
      await expect(edge).toHaveText(`proposed — supersedes ${s.object} when ${st.successor} is accepted`);
      await expect(page.getByTestId(`objsupersede-${stem}-not-established`)).toHaveCount(0);
      await expectUnclipped(page, page.getByTestId(`card-${objectId(s.decision)}`));
    }
  });

  test("not established: the reason appears on the decision card, never a supersession on the reference card", async ({ page }) => {
    const f = await fixture(page);
    // Before acceptance, on the design board: one edge has no conflict.
    const nc = store(f, "no-conflict");
    await page.goto(nc.boards.design.url);
    const missing = nc.supersessions.find((s) => s.conflict === "") as Supersession;
    expect(missing.object).toBe("spec/closed-feature#dc-1");
    await expectUntouched(page, missing.object);
    const ncStem = decisionStem(missing.decision, missing.object);
    const reason = page.getByTestId(`card-supersession-${objectId(missing.decision)}`).getByTestId(`objsupersede-${ncStem}-not-established`);
    await expect(reason).toHaveAttribute("data-state", "not-established");
    await expect(reason).toHaveText(`supersession not established: no conflict challenges ${missing.object}`);
    await expect(page.getByTestId(`objsupersede-${ncStem}-edge`)).toHaveCount(0);
    const matched = nc.supersessions.find((s) => s.conflict !== "") as Supersession;
    await expect(page.getByTestId(`objsupersede-${decisionStem(matched.decision, matched.object)}-edge`)).toHaveAttribute("data-state", "proposed");
    await expectUnclipped(page, page.getByTestId(`card-${objectId(missing.decision)}`));

    // After acceptance, on the DESIGN board of the accepted but
    // non-matching successor's revision (the serving checkout is
    // design/successor-v2): the two-index rule — the story's reference
    // card reads DEFAULT-BRANCH records (superseded, S1's date, no carry
    // line: main carries no successor-v2), while the revision's own
    // decision card reads the branch's tree (proposed, carrying S1's
    // replacement); the feature's objects stay untouched and their
    // decisions read the reasons.
    const ci = store(f, "chain-not-in-force");
    expect(ci.checkout).toBe(ci.design_branch);
    await page.goto(ci.boards.checkout.url);
    const byObject = new Map(ci.supersessions.map((s) => [s.object, s]));
    for (const object of ["spec/closed-feature#dc-1", "spec/closed-feature#ac-1"]) {
      const s = byObject.get(object) as Supersession;
      await expectUntouched(page, object);
      const stem = decisionStem(s.decision, s.object);
      const line = page.getByTestId(`card-supersession-${objectId(s.decision)}`).getByTestId(`objsupersede-${stem}-not-established`);
      await expect(line).toHaveAttribute("data-state", "not-established");
      await expect(line).toContainText("supersession not established: ");
      await expect(line).toContainText("no conflict challenges spec/closed-feature#ac-1");
      await expect(page.getByTestId(`objsupersede-${stem}-edge`)).toHaveCount(0);
    }
    const story = byObject.get("spec/closed-story#ac-1") as Supersession;
    // S1 (spec/successor) is active on this checkout: its board, at the
    // deciding card; the serving board serves the conflict's corpus page.
    await expectSuperseded(page, story, `/board/${ci.establishing_successor}#obj-${objectId(story.establishing_decision)}`, `/a/${story.conflict}`);
    await expect(page.getByTestId(refCardTestId(story.object)).getByTestId(`objsupersede-${refStem(story.object)}-carry`)).toHaveCount(0);
    const storyStem = decisionStem(story.decision, story.object);
    const storyLines = page.getByTestId(`card-supersession-${objectId(story.decision)}`);
    await expect(storyLines.getByTestId(`objsupersede-${storyStem}-edge`)).toHaveAttribute("data-state", "proposed");
    await expect(storyLines.getByTestId(`objsupersede-${storyStem}-carries`)).toContainText(`carries the replacement established by ${ci.establishing_successor} (`);
    await expectUnclipped(page, page.getByTestId(`card-${objectId(story.decision)}`));

    // The same store's /b/main board: the default-branch board of the
    // establishing successor, where no corpus page is provably served for
    // a per-branch tree, so the conflict carries no link.
    expect(ci.boards.main.url).not.toBe("");
    await page.goto(ci.boards.main.url);
    await expectSuperseded(page, story, `#obj-${objectId(story.establishing_decision)}`, null);
  });
});
