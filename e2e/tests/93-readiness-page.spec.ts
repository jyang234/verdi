import { test, expect, request, type Page, type Locator } from "@playwright/test";
import { CONTROL_URL, SHOWCASE, branchBoardPath } from "./fixtures";

// spec/readiness-page-v2 (jira:VERDI-WR-5), ac-1, ac-2, and ac-3: the
// readiness page in the design's layout (handoff screen 3b, as SI-339
// reads it). Where you are — the spec, its class chip, its branch — then
// the four-step stepper with each step's state and one line of count and
// reason, the sentence explaining the order, Focus next, Known problems in
// later steps, and Completed checks; "Open the wall →" in the bar's
// controls slot and the per-request derivation stamp in the body. Focus
// next lists every concern of the current step, marked "now", and the
// later steps' concerns behind one inline disclosure, marked "later" with
// what they wait on; each item leads with its guidance, its fact, timing
// and blocking flag in the technical disclosure. Human-review work is
// labeled plainly with the formal obligation as secondary text; every
// state reads as Proven, Violated (its witness in the disclosure), or Not
// enough evidence yet; on a solo-author profile the broader-role duties
// stay visible, human review, and unsatisfied, with their derived text.
//
// The three tests are the producers their obligations name
// (.verdi/obligations/readiness-page-v2/ac-{1,2,3}--behavioral.md), titled
// exactly as each claim spells it. Every page here is an ISOLATED serve
// (BL-98): the readiness-page fixture (cmd/e2eharness/readinesspagefixture.go
// — the real workbench handler over five derived snapshots: the page at
// steps 2, 3 and 4, the solo-author profile, and the three states) and the
// readiness-pilot fixture (49-readiness-pilot.spec.ts's step-1 store); both
// are warmed in beforeAll, and the shared /readiness is never read, so the
// file passes alone. State assertions ride classes and data attributes,
// never free innerText (lane rule); recording stays off.

const PAGE_FIXTURE_URL = `${CONTROL_URL}/readiness-page-fixture`;
const PILOT_FIXTURE_URL = `${CONTROL_URL}/readiness-pilot-fixture`;

// The plain triad (SI-339 (10)), by formal state.
const PLAIN: Record<string, string> = {
  proven: "Proven",
  "violated-with-witness": "Violated",
  unproven: "Not enough evidence yet",
};

const AREAS: Array<[id: string, label: string]> = [
  ["shape-proposal", "Define the work"],
  ["show-success", "Define success"],
  ["check-context", "Check constraints"],
  ["request-review", "Get approval"],
];
const AREA_LABEL = Object.fromEntries(AREAS) as Record<string, string>;

// One page per step. Pinned from the fixtures' own inputs
// (readinesspagefixture.go's step inputs; the pilot store's RAIL in
// 49-readiness-pilot.spec.ts) and the derivation's rules: the current
// focus is the first area with a blocking unresolved row; a proven step
// is complete, the focus is the current focus, any other step waits on
// the focus (SI-339 (8)).
type StepPage = {
  step: number;
  fixture: "pilot" | string;
  title: string;
  cls: string;
  spec: string;
  branch: string;
  states: string[];
  reasons: string[];
};
const STEP_PAGES: StepPage[] = [
  {
    step: 1, fixture: "pilot", title: "Refinancing decline flow", cls: "feature",
    spec: SHOWCASE.DESIGN_SPEC, branch: SHOWCASE.DESIGN_BRANCH,
    states: ["unproven", "proven", "unproven", "violated-with-witness"],
    reasons: ["current-focus", "complete", "waits", "waits"],
  },
  {
    step: 2, fixture: "readiness-page-step-2", title: "Decline notice refresh", cls: "feature",
    spec: "readiness-page-step-2", branch: "design/readiness-page-step-2",
    states: ["proven", "violated-with-witness", "unproven", "proven"],
    reasons: ["complete", "current-focus", "waits", "complete"],
  },
  {
    step: 3, fixture: "readiness-page-step-3", title: "Decline audit trail", cls: "story",
    spec: "readiness-page-step-3", branch: "design/readiness-page-step-3",
    states: ["proven", "proven", "unproven", "proven"],
    reasons: ["complete", "complete", "current-focus", "complete"],
  },
  {
    step: 4, fixture: "readiness-page-step-4", title: "Reversal propagation", cls: "story",
    spec: "readiness-page-step-4", branch: "design/readiness-page-step-4",
    states: ["proven", "proven", "proven", "violated-with-witness"],
    reasons: ["complete", "complete", "complete", "current-focus"],
  },
];

// The step-2 fixture's complete concern set, pinned from its input
// (readinessPageStep2Input: criteria ac-1..ac-3 with ac-3 uncovered, a
// current obligation-quality blocker for ac-2/runtime, an eventual
// outcome-floor blocker for ac-2, no context request) and the derivation's
// closed grammar and comparators (the focus area first; then blocking
// before non-blocking, current before eventual, area order, id).
const STEP2 = {
  focus: "show-success",
  now: ["success/blocker/obligation-quality/ac-2/runtime", "success/coverage/ac-3"],
  later: ["context/verdict", "review/blocker/outcome-floor/ac-2"],
  completed: [
    "shape/board", "shape/mutation", "shape/outcome", "shape/problem", "shape/provenance",
    "success/contributor/static", "success/criteria",
    "review/action", "review/eventual-derivation",
  ],
  known: ["review/blocker/outcome-floor/ac-2"],
  // Guidance the derivation's templates produce (ledger SI-342).
  guidance: {
    "success/coverage/ac-3": "Plan the delivery: graduate a story sticky into a stub claiming ac-3.",
    "context/verdict": "Run the context-conflict verb with this spec's context request, then resolve what it reports.",
  } as Record<string, string>,
  timing: {
    "success/blocker/obligation-quality/ac-2/runtime": ["true", "current"],
    "success/coverage/ac-3": ["false", "current"],
    "context/verdict": ["true", "current"],
    "review/blocker/outcome-floor/ac-2": ["false", "eventual"],
  } as Record<string, [blocking: string, timing: string]>,
};

// The solo-author fixture's duties (readinessPageSoloAuthorInput): the
// broader-role duty — a countersign on close — and the unresolved
// principal for merge, each human review by SI-338 (3); the author's own
// vouch is judgmental work, not human review. Their derived texts are the
// SI-342 role template and the blockers' own clearing conditions.
const SOLO = {
  humanReview: [
    "review/role/close/attestation/countersign",
    "review/role/merge/attestation/author-vouch",
    "review/blocker/obligation-countersign-unproven/close/attestation/countersign",
    "review/blocker/principal-resolution-unproven/merge",
  ],
  governance: [
    "review/blocker/obligation-countersign-unproven/close/attestation/countersign",
    "review/blocker/principal-resolution-unproven/merge",
  ],
  judgmental: "review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch",
  earlierStepRow: "context/disclosure/solo-principal-collapse",
  text: {
    "review/role/close/attestation/countersign":
      "Before close, a principal entitled to give attestation/countersign must provide it; verdi journey shows the requirement.",
    "review/role/merge/attestation/author-vouch":
      "Before merge, a principal entitled to give attestation/author-vouch must provide it; verdi journey shows the requirement.",
    "review/blocker/obligation-countersign-unproven/close/attestation/countersign":
      "obligation attestation/countersign is proven for transition close",
    "review/blocker/principal-resolution-unproven/merge": "the required principals resolve as authenticated",
  } as Record<string, string>,
};

async function fixtureBase(url: string): Promise<string> {
  const api = await request.newContext();
  try {
    const res = await api.get(url);
    expect(res.ok(), await res.text()).toBe(true);
    const base = (await res.text()).trim();
    expect(base).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
    return base;
  } finally {
    await api.dispose();
  }
}

function formalState(cls: string): string {
  const m = cls.match(/readiness-concern--(proven|violated-with-witness|unproven)/);
  expect(m, `state class on ${cls}`).not.toBeNull();
  return m![1];
}

function row(page: Page, id: string): Locator {
  return page.locator(`article[data-concern-id="${id}"]`);
}

async function idsOf(scope: Locator): Promise<string[]> {
  return scope.evaluateAll((els) => els.map((el) => el.getAttribute("data-concern-id") ?? ""));
}

// leadOf: what a row's copy block renders before its one primary line —
// the class names of the children ahead of it — plus whether the chip and
// the disclosure follow it. Guidance-first means the lead-in is exactly
// the step label, and the human-review label exactly when the row is
// human review (F4P-1).
async function leadOf(r: Locator): Promise<{ before: string[]; primaries: number; chipAfter: boolean; techAfter: boolean }> {
  return r.locator(".readiness-copy").evaluate((copy) => {
    const kids = Array.from(copy.children);
    const primary = kids.findIndex((k) => k.matches("p.readiness-summary"));
    return {
      before: kids.slice(0, Math.max(primary, 0)).map((k) => k.className),
      primaries: kids.filter((k) => k.matches("p.readiness-summary")).length,
      chipAfter: kids.findIndex((k) => k.matches(".readiness-state")) > primary,
      techAfter: kids.findIndex((k) => k.matches("details.readiness-tech")) > primary,
    };
  });
}

// isBefore: a precedes b in DOM order.
async function isBefore(a: Locator, b: Locator): Promise<boolean> {
  const bh = await b.elementHandle();
  return a.evaluate(
    (el, other) => !!other && !!(el.compareDocumentPosition(other) & Node.DOCUMENT_POSITION_FOLLOWING),
    bh,
  );
}

test.describe("readiness-page", () => {
  let pageBase = "";
  let pilotBase = "";

  // The pilot fixture provisions a whole store and builds the binary on
  // first use — seconds on a warm cache, longer cold — and the page
  // fixture derives five snapshots; warm both ONCE here, under their own
  // allowance, so every test finds its serve up (BL-98).
  test.beforeAll(async () => {
    test.setTimeout(120_000);
    pageBase = await fixtureBase(PAGE_FIXTURE_URL);
    pilotBase = await fixtureBase(PILOT_FIXTURE_URL);
  });

  const urlOf = (p: { fixture: string }) =>
    p.fixture === "pilot" ? `${pilotBase}readiness` : `${pageBase}readiness?spec=${p.fixture}`;

  test("The readiness page in the design's layout", async ({ page }) => {
    test.setTimeout(90_000);
    for (const p of STEP_PAGES) {
      await page.goto(urlOf(p));
      const what = `step ${p.step}`;
      const focusArea = AREAS[p.step - 1][0];

      // Where you are: the eyebrow, the exact title, the class chip, the
      // spec ref and the branch, then the current step.
      await expect(page.locator(".readiness-eyebrow"), what).toHaveText("Where you are");
      await expect(page.locator("h2.readiness-title"), what).toHaveText(p.title);
      const chip = page.getByTestId("readiness-class-chip");
      await expect(chip, what).toBeVisible();
      await expect(chip, what).toHaveAttribute("data-class", p.cls);
      await expect(chip, `${what}: the chip speaks the class's display word`).not.toHaveText("");
      await expect(page.getByTestId("readiness-target-ref"), what).toHaveText(`spec/${p.spec}`);
      await expect(page.getByTestId("readiness-branch"), what).toHaveText(p.branch);
      await expect(page.locator(".readiness-step"), what).toHaveText(
        `Step ${p.step} of 4 — ${AREA_LABEL[focusArea]}`,
      );

      // The four-step stepper: each station's state, formal id, and one
      // line of count and reason; the current step alone is aria-current.
      const stations = page.locator(".readiness-rail .readiness-station");
      await expect(stations, what).toHaveCount(4);
      for (let i = 0; i < 4; i++) {
        const [id, label] = AREAS[i];
        const st = stations.nth(i);
        const tag = `${what} station ${id}`;
        await expect(st, tag).toHaveAttribute("data-area-id", id);
        await expect(st, tag).toHaveAttribute("data-state", p.states[i]);
        await expect(st, tag).toHaveAttribute("data-reason", p.reasons[i]);
        await expect(st.locator(".readiness-station-num"), tag).toHaveText(String(i + 1));
        await expect(st.locator(".readiness-station-label"), tag).toHaveText(label);
        await expect(st.locator(".readiness-station-id"), tag).toHaveText(id);
        const stateChip = st.locator(".readiness-state");
        await expect(stateChip, tag).toHaveClass(new RegExp(`readiness-state--${p.states[i]}(\\s|$)`));
        await expect(stateChip, tag).toHaveText(PLAIN[p.states[i]]);
        const reasonText = { "current-focus": /— current focus$/, complete: /— complete$/, waits: new RegExp(`— waits on ${AREA_LABEL[focusArea]}$`) }[p.reasons[i]]!;
        await expect(st.locator(".readiness-station-line"), tag).toHaveText(/^\d+ (violated|not enough evidence yet|proven)(, \d+ (violated|not enough evidence yet|proven))* — /);
        await expect(st.locator(".readiness-station-line"), tag).toHaveText(reasonText);
      }
      await expect(page.locator('.readiness-rail a[aria-current="step"]'), what).toHaveCount(1);
      await expect(stations.nth(p.step - 1).locator('a[aria-current="step"]'), what).toHaveCount(1);

      // The ordering sentence sits between the stepper and Focus next.
      const order = page.getByTestId("readiness-order");
      await expect(order, what).toBeVisible();
      await expect(order, what).toHaveText(/^The four steps run in this order: define the work, define success, check constraints, then get approval\./);
      expect(await isBefore(page.locator(".readiness-rail"), order), `${what}: stepper before the sentence`).toBe(true);
      expect(await isBefore(order, page.locator("#readiness-focus")), `${what}: sentence before Focus next`).toBe(true);

      // The three sections, in the design's order.
      const focus = page.locator('#readiness-focus[aria-label="Focus next"]');
      const known = page.locator('#readiness-known[aria-label="Known problems in later steps"]');
      const completed = page.locator('#readiness-completed[aria-label="Completed checks"]');
      for (const [name, section] of [["Focus next", focus], ["Known problems", known], ["Completed checks", completed]] as const) {
        await expect(section, `${what}: ${name}`).toBeVisible();
        await expect(section.locator("h2.readiness-heading"), `${what}: ${name} heading`).toContainText(name);
      }
      expect(await isBefore(focus, known), `${what}: Focus next before Known problems`).toBe(true);
      expect(await isBefore(known, completed), `${what}: Known problems before Completed checks`).toBe(true);

      // "Open the wall →" in the bar's controls slot, with its own class;
      // the bar's title stays the page's h1, "Readiness".
      const bar = page.getByTestId("topbar");
      await expect(bar.getByTestId("topbar-title"), what).toHaveText("Readiness");
      const wall = bar.getByTestId("topbar-controls").getByTestId("readiness-wall-link");
      await expect(wall, what).toBeVisible();
      await expect(wall, what).toHaveText(/^Open the wall\s*→$/);
      await expect(wall, what).toHaveAttribute("href", branchBoardPath(p.branch, p.spec));
      await expect(bar.locator(".readiness-board-link"), `${what}: the wall link is not a destination link`).toHaveCount(0);

      // The per-request stamp stays in the body with its hook, and the
      // design's startup-snapshot copy is nowhere.
      const stamp = page.locator("main.content .readiness-stale[data-readiness-stale]");
      await expect(stamp, what).toBeVisible();
      await expect(stamp, what).toHaveAttribute("aria-label", "Derivation stamp");
      await expect(stamp, what).toContainText(/Derived at HEAD [0-9a-f]+ for this request\./);
      const text = (await page.locator("body").textContent()) ?? "";
      expect(text.toLowerCase(), `${what}: no startup-snapshot copy`).not.toContain("startup snapshot");
      expect(text.toLowerCase(), `${what}: no restart copy`).not.toContain("restart verdi serve");
    }

    // 320 px: the page's own content neither clips nor runs past the
    // viewport (Wave 6 §5.2); step labels wrap rather than truncate.
    await page.setViewportSize({ width: 320, height: 900 });
    await page.goto(urlOf(STEP_PAGES[1]));
    const narrow = await page.evaluate(() => {
      const root = document.querySelector(".readiness-standalone")!;
      const vw = document.documentElement.clientWidth;
      const past = Array.from(root.querySelectorAll<HTMLElement>("*"))
        .filter((el) => el.getBoundingClientRect().right > vw + 1)
        .map((el) => el.className)
        .slice(0, 5);
      const clipped = Array.from(root.querySelectorAll<HTMLElement>(".readiness-station-link, .readiness-card, .readiness-row, .readiness-known-link, .readiness-tech-facts"))
        .filter((el) => el.scrollWidth > el.clientWidth + 1)
        .map((el) => el.className)
        .slice(0, 5);
      const labels = Array.from(root.querySelectorAll<HTMLElement>(".readiness-station-label")).map((el) => ({
        text: el.textContent, lines: Math.round(el.getBoundingClientRect().height / parseFloat(getComputedStyle(el).lineHeight)),
        style: getComputedStyle(el).textOverflow + "/" + getComputedStyle(el).whiteSpace,
      }));
      return { past, clipped, labels };
    });
    expect(narrow.past, "elements past the 320 px viewport").toEqual([]);
    expect(narrow.clipped, "clipped elements at 320 px").toEqual([]);
    for (const l of narrow.labels) {
      expect(l.style, `step label ${l.text} wraps`).toBe("clip/normal");
    }
  });

  test("Focus next ranks every concern, guidance first", async ({ page }) => {
    await page.goto(urlOf({ fixture: "readiness-page-step-2" }));
    const focus = page.locator("#readiness-focus");
    const all = [...STEP2.now, ...STEP2.later];

    // The current step's concerns first, each marked with its step and
    // "now", visible before any expansion; then the waiting ones, each
    // marked "later" with what it waits on, behind one closed disclosure.
    expect(await idsOf(focus.locator("[data-concern-id]"))).toEqual(all);
    expect(await idsOf(focus.locator("[data-concern-id]:visible"))).toEqual(STEP2.now);
    for (const id of STEP2.now) {
      const r = row(page, id);
      await expect(r, id).toHaveAttribute("data-timing", "now");
      await expect(r, id).toHaveAttribute("data-area-id", STEP2.focus);
      await expect(r.locator(".readiness-stage"), id).toContainText(AREA_LABEL[STEP2.focus]);
      await expect(r.locator(".readiness-stage .readiness-when--now"), id).toHaveText("now");
    }
    const later = focus.locator('details.readiness-more[data-testid="readiness-later"]');
    await expect(later).toHaveCount(1);
    await expect(later).not.toHaveAttribute("open", "");
    await expect(later.locator(".readiness-more-closed")).toBeVisible();
    await expect(later.locator(".readiness-more-closed")).toHaveText(
      `${STEP2.later.length} more, waiting on ${AREA_LABEL[STEP2.focus]}`,
    );
    for (const id of STEP2.later) {
      await expect(row(page, id), `${id} hidden before expansion`).toBeHidden();
    }

    // Expanding shows them inline — inside Focus next, after the current
    // step's rows, ranked on from them.
    const lastNow = row(page, STEP2.now[STEP2.now.length - 1]);
    const laterSummary = later.locator("summary.readiness-more-summary");
    await laterSummary.click();
    await expect(later).toHaveAttribute("open", "");
    expect(await idsOf(later.locator("[data-concern-id]"))).toEqual(STEP2.later);
    for (const id of STEP2.later) {
      const r = row(page, id);
      await expect(r, id).toBeVisible();
      await expect(r, id).toHaveAttribute("data-timing", "later");
      await expect(r.locator(".readiness-stage .readiness-when--later"), id).toHaveText(
        `later — waits on ${AREA_LABEL[STEP2.focus]}`,
      );
      expect(await isBefore(lastNow, r), `${id} after the current step's rows`).toBe(true);
      const lastNowBox = (await lastNow.boundingBox())!;
      const box = (await r.boundingBox())!;
      expect(box.y, `${id} rendered in place below the current step's rows`).toBeGreaterThanOrEqual(lastNowBox.y + lastNowBox.height - 1);
    }
    const ranks = await focus.locator(".readiness-rank").allTextContents();
    expect(ranks).toEqual(all.map((_, i) => String(i + 1)));

    // Each item's first line is its guidance; its fact, timing and
    // blocking flag sit in the technical disclosure.
    for (const id of all) {
      const r = row(page, id);
      const primary = r.locator(".readiness-copy > p.readiness-summary");
      await expect(primary, id).toHaveCount(1);
      await expect(primary, id).toHaveClass(/readiness-guidance/);
      await expect(primary, id).not.toHaveText("");
      if (STEP2.guidance[id]) {
        await expect(primary, id).toHaveText(STEP2.guidance[id]);
      }
      // Exactly the step label precedes the guidance (no row here is
      // human review); one primary line; chip and disclosure follow.
      const lead = await leadOf(r);
      expect(lead.before, `${id}: only the step label before the guidance`).toEqual(["readiness-stage"]);
      expect(lead.primaries, `${id}: one primary line`).toBe(1);
      expect(lead.chipAfter, `${id}: chip after the guidance`).toBe(true);
      expect(lead.techAfter, `${id}: disclosure after the guidance`).toBe(true);
      const tech = r.locator("details.readiness-tech");
      await expect(tech.locator("dl.readiness-tech-facts"), id).toBeHidden();
      await tech.locator("summary").click();
      await expect(tech.locator('dt:text-is("Fact") + dd'), id).not.toHaveText("");
      await expect(tech.locator('dt:text-is("Concern") + dd'), id).toHaveText(id);
      const [blocking, timing] = STEP2.timing[id];
      await expect(tech.locator('dt:text-is("Blocking") + dd'), id).toHaveText(blocking);
      await expect(tech.locator('dt:text-is("Timing") + dd'), id).toHaveText(timing);
    }

    // The rendered concerns equal the facts' complete set: Focus next and
    // Completed checks together, each id exactly once.
    expect(await idsOf(page.locator("#readiness-completed [data-concern-id]"))).toEqual(STEP2.completed);
    const rendered = await idsOf(page.locator("[data-concern-id]"));
    expect(rendered.length).toBe(all.length + STEP2.completed.length);
    expect(new Set(rendered).size).toBe(rendered.length);
    expect([...rendered].sort()).toEqual([...all, ...STEP2.completed].sort());
    for (const id of STEP2.completed) {
      expect(formalState((await row(page, id).getAttribute("class"))!), id).toBe("proven");
    }

    // Known problems in later steps: the later violated rows only, each a
    // link to its one row, which the link reveals and reaches.
    const knownLinks = page.locator("#readiness-known .readiness-known-link");
    expect(await knownLinks.evaluateAll((els) => els.map((el) => el.getAttribute("data-known-concern")))).toEqual(STEP2.known);
    // An entry is exactly its step label, its fact and its state chip —
    // nothing of the row itself (F4P-2).
    const knownSection = page.locator("#readiness-known");
    for (const sel of ["[data-concern-id]", "details", ".readiness-guidance", ".readiness-dest", ".readiness-tech", ".readiness-summary"]) {
      await expect(knownSection.locator(sel), `known problems hold no ${sel}`).toHaveCount(0);
    }
    for (const id of STEP2.known) {
      const entry = knownSection.locator(`.readiness-known-link[data-known-concern="${id}"]`);
      expect(await entry.evaluate((el) => Array.from(el.children).map((k) => k.className)), id).toEqual([
        "readiness-stage", "readiness-known-text", "readiness-state readiness-state--violated-with-witness",
      ]);
      const area = (await row(page, id).getAttribute("data-area-id"))!;
      await expect(entry.locator(".readiness-stage"), id).toHaveText(AREA_LABEL[area]);
      await expect(entry.locator(".readiness-known-text"), id).not.toHaveText("");
      await expect(entry.locator(".readiness-state"), id).toHaveText(PLAIN["violated-with-witness"]);
    }
    await laterSummary.click();
    await expect(later).not.toHaveAttribute("open", "");
    await expect(knownLinks.first()).toHaveAttribute("href", `#concern-${STEP2.known[0]}`);
    await knownLinks.first().click();
    await expect(later).toHaveAttribute("open", "");
    await expect(row(page, STEP2.known[0])).toBeVisible();
    await expect(row(page, STEP2.known[0])).toBeInViewport();
    expect(await row(page, STEP2.known[0]).getAttribute("id")).toBe(`concern-${STEP2.known[0]}`);
  });

  test("Human review labeled plainly, three-valued states, solo-author language", async ({ page }) => {
    // The three states side by side: a proven, a violated-with-witness,
    // and a disclosed-unproven item, each labeled so.
    await page.goto(urlOf({ fixture: "readiness-page-three-states" }));
    const proven = row(page, "shape/problem");
    await expect(proven).toHaveClass(/readiness-concern--proven/);
    await expect(proven.locator(".readiness-state")).toHaveClass(/readiness-state--proven/);
    await expect(proven.locator(".readiness-state")).toHaveText(PLAIN.proven);
    await expect(page.locator("#readiness-completed").locator(proven)).toHaveCount(1);
    const violated = row(page, "shape/outcome");
    await expect(violated).toHaveClass(/readiness-concern--violated-with-witness/);
    await expect(violated.locator(".readiness-state")).toHaveClass(/readiness-state--violated-with-witness/);
    await expect(violated.locator(".readiness-state")).toHaveText(PLAIN["violated-with-witness"]);
    await violated.locator("details.readiness-tech summary").click();
    await expect(violated.locator(".readiness-witnesses li").first()).toBeVisible();
    await expect(violated.locator(".readiness-witnesses li").first()).not.toHaveText("");
    const unproven = row(page, "shape/question/oq-1");
    await expect(unproven).toHaveClass(/readiness-concern--unproven/);
    await expect(unproven.locator(".readiness-state")).toHaveClass(/readiness-state--unproven/);
    await expect(unproven.locator(".readiness-state")).toHaveText(PLAIN.unproven);
    for (const word of await page.locator(".readiness-state").allTextContents()) {
      expect(Object.values(PLAIN), `chip reads the triad: ${word}`).toContain(word.trim());
    }

    // The solo-author profile: each broader-role duty stays visible,
    // labeled human review with the formal obligation as secondary text,
    // in a state other than proven, with its derived text verbatim; the
    // author's own vouch is not human review.
    await page.goto(urlOf({ fixture: "readiness-page-solo-author" }));
    for (const id of SOLO.humanReview) {
      const r = row(page, id);
      await expect(r, id).toBeVisible();
      await expect(r, id).toHaveAttribute("data-timing", "now");
      await expect(r, id).toHaveClass(/readiness-concern--human-review/);
      expect(formalState((await r.getAttribute("class"))!), id).not.toBe("proven");
      const label = r.locator(".readiness-human-review");
      await expect(label, id).toBeVisible();
      await expect(label, id).toHaveText(/^Human review/);
      const formal = label.locator(".readiness-human-review-formal");
      await expect(formal, id).toContainText(id);
      if (SOLO.governance.includes(id)) {
        await expect(formal, id).toContainText("governance");
      }
      await expect(r.locator(".readiness-copy > p.readiness-summary.readiness-guidance"), id).toHaveText(SOLO.text[id]);
      // The human-review label sits between the step label and the
      // guidance, and nothing else does (F4P-1).
      const lead = await leadOf(r);
      expect(lead.before, `${id}: step label, then the human-review label, then the guidance`).toEqual(["readiness-stage", "readiness-human-review"]);
      expect(lead.primaries, `${id}: one primary line`).toBe(1);
      expect(lead.chipAfter && lead.techAfter, `${id}: chip and disclosure after the guidance`).toBe(true);
    }
    const judgmental = row(page, SOLO.judgmental);
    await expect(judgmental).toBeVisible();
    await expect(judgmental.locator(".readiness-human-review")).toHaveCount(0);
    await expect(judgmental).not.toHaveClass(/readiness-concern--human-review/);
    expect((await leadOf(judgmental)).before, "judgmental row: only the step label before its guidance").toEqual(["readiness-stage"]);
    // An unresolved non-blocking row of an earlier, complete step waits on
    // nothing: it stays visible and reads "now" (a reading beyond SI-339
    // (3), disclosed in the lane's report).
    const earlier = row(page, SOLO.earlierStepRow);
    await expect(earlier).toBeVisible();
    await expect(earlier).toHaveAttribute("data-timing", "now");
    await expect(page.locator("#readiness-known .readiness-known-empty")).toBeVisible();
  });
});
