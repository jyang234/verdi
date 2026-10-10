import { test, expect, Page, type Locator } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";
import { openRecordTab } from "./helpers";

// spec/spec-documents Wave 4 Task 4 — guidance-first concern cards (ac-12,
// R-W4-7) and the policy guide's verb pointer (ac-10, R-W4-8).
//
// On the wall and the /readiness cockpit each concern card shows its
// guidance sentence as the primary line when it carries one (otherwise the
// summary), keeps the fact in the existing Technical details disclosure
// with the concern id, timing and blocking flag. The four area labels and
// the plain-word triad are unchanged; nothing here changes a derivation.
// The wall's readiness is the record drawer's Readiness tab since the
// wall shell retired (spec/wall-strip-and-drawer-v2 ac-5, ac-7; SI-368
// (14), (24)(a)): the wall's assertions read the tab's shared shape —
// the primary line first, the fact, timing (the snapshot's current or
// eventual, SI-339 (3)) and blocking flag in the disclosure — in the
// wall's own triad words, its titles unchanged. State assertions ride
// classes and data attributes, never free innerText (lane rule).

const DESIGN = () => boardPath(SHOWCASE.DESIGN_SPEC);
const DRAFT_B = () =>
  "/b/" +
  encodeURIComponent(SHOWCASE.SHOWCASE_DRAFT_BRANCH) +
  "/board/spec/" +
  SHOWCASE.SHOWCASE_DRAFT_SPEC;

// The four stations in snapshot order: id, plain label (the formal states
// pinned per surface in 49-readiness-pilot.spec.ts; here the labels and
// the id order are what ac-12 holds fixed).
const RAIL: Array<[id: string, label: string]> = [
  ["shape-proposal", "Define the work"],
  ["show-success", "Define success"],
  ["check-context", "Check constraints"],
  ["request-review", "Get approval"],
];
const AREA_ORDER = RAIL.map(([id]) => id);
const AREA_LABEL = Object.fromEntries(RAIL) as Record<string, string>;

const PLAIN_LABELS: Record<string, string> = {
  proven: "Ready",
  "violated-with-witness": "Needs attention",
  unproven: "Not enough evidence yet",
};

// openRemainder expands the focus queue's exact-count remainder so every
// queued row is in the rendered tree (SI-125 keeps it lossless).

// wallReadiness opens the wall's readiness — the record drawer's
// Readiness tab — and returns its body.
async function wallReadiness(page: Page): Promise<Locator> {
  return (await openRecordTab(page, "readiness")).getByTestId("readiness-tab");
}

// stageLabel reads a row's step label as the label's own text, the
// step-relation span ("now", "later — waits on …") set apart (SI-368
// (24)(a)).
async function stageLabel(row: Locator): Promise<string> {
  return row.locator(".readiness-stage").evaluate((el) =>
    Array.from(el.childNodes)
      .filter((n) => n.nodeType === Node.TEXT_NODE)
      .map((n) => n.textContent ?? "")
      .join("")
      .trim(),
  );
}
async function openRemainder(page: Page) {
  const more = page.locator(
    '[data-testid="asd-more"] > summary, details.readiness-more > summary',
  );
  if (await more.count()) await more.first().click();
}

// formalState reads the article's own state class — never the chip text.
async function formalState(row: ReturnType<Page["locator"]>) {
  const cls = (await row.getAttribute("class")) ?? "";
  const m = cls.match(/readiness-concern--([a-z-]+)/);
  expect(m, `state class on ${cls}`).not.toBeNull();
  return m![1];
}

// cardShape reports the child order inside one card's .readiness-copy as
// indexes — the step label, the first primary line and the state chip —
// and how many primary lines the card carries.
async function cardShape(row: ReturnType<Page["locator"]>) {
  return row.locator(".readiness-copy").evaluate((copy) => {
    const kids = Array.from(copy.children);
    const idx = (sel: string) => kids.findIndex((k) => k.matches(sel));
    return {
      stage: idx("p.readiness-stage"),
      primary: idx("p.readiness-summary"),
      chip: idx(".readiness-state"),
      summaries: kids.filter((k) => k.matches("p.readiness-summary")).length,
    };
  });
}

test("wall cards lead with guidance, keep the fact visible, and file timing under Technical details", async ({
  page,
}) => {
  await page.goto(DESIGN());
  const shell = await wallReadiness(page);
  await openRemainder(page);
  // The inline stage-line timing span is gone everywhere.
  await expect(shell.locator(".asd-timing")).toHaveCount(0);

  // The current focus is the rail's aria-current station (absent when
  // all four steps are complete) — the same condition the timing uses.
  const focusStation = shell.locator(
    '.readiness-station:has(a[aria-current="step"])',
  );
  const focus = (await focusStation.count())
    ? await focusStation.getAttribute("data-area-id")
    : null;

  const rows = shell.locator("article[data-concern-id]");
  const n = await rows.count();
  expect(n).toBeGreaterThan(0);
  let guided = 0;
  let now = 0;
  for (let i = 0; i < n; i++) {
    const row = rows.nth(i);
    const id = (await row.getAttribute("data-concern-id"))!;
    const area = (await row.getAttribute("data-area-id"))!;
    const state = await formalState(row);

    // The primary line leads the card, then the step label, then the
    // chip: exactly one primary line per card — the guidance sentence when
    // the row carries one (an unresolved row), its fact otherwise (a
    // proven row) — and the fact itself kept in the disclosure.
    const copy = row.locator(".readiness-copy");
    await expect(copy.locator(":scope > *").first(), id).toHaveClass("readiness-primary");
    const primary = copy.locator(":scope > .readiness-primary > p.readiness-summary");
    await expect(primary, id).toHaveCount(1);
    expect(await copy.locator("p.readiness-summary").count(), id).toBe(1);
    const order = await copy.evaluate((el) => {
      const kids = Array.from(el.children);
      return {
        primary: kids.findIndex((k) => k.matches(".readiness-primary")),
        stage: kids.findIndex((k) => k.matches("p.readiness-stage")),
        chip: kids.findIndex((k) => k.matches(".readiness-state")),
      };
    });
    expect(order.stage, id).toBeGreaterThan(order.primary);
    expect(order.chip, id).toBeGreaterThan(order.stage);
    const guidance = (await primary.getAttribute("class"))!.includes("readiness-guidance");
    if (state === "proven") expect(guidance, `${id}: a proven row leads with its fact`).toBe(false);
    if (guidance) guided++;

    // Timing: the step relation as a data attribute — "later" after the
    // current step, "now" for every other unresolved row, none on a
    // proven row (SI-346 (2)) — and the snapshot's own current or
    // eventual in the disclosure, never on the stage line.
    let expected: "now" | "later" | null = null;
    if (state !== "proven") {
      expected = focus && AREA_ORDER.indexOf(area) > AREA_ORDER.indexOf(focus) ? "later" : "now";
    }
    if (expected) {
      await expect(row).toHaveAttribute("data-timing", expected);
      if (expected === "now") now++;
    } else {
      expect(await row.getAttribute("data-timing"), id).toBeNull();
    }
    await row.locator(".readiness-tech summary").click();
    const fact = row.locator('dt:text-is("Fact") + dd');
    await expect(fact, `${id}: the fact kept in the disclosure`).toBeVisible();
    await expect(fact).not.toHaveText("");
    await expect(row.locator('dt:text-is("Concern") + dd')).toHaveText(id);
    await expect(row.locator('dt:text-is("Blocking") + dd')).toHaveText(
      /^(true|false)$/,
    );
    await expect(row.locator('dt:text-is("Timing") + dd')).toHaveText(/^(current|eventual)$/);
    expect(await stageLabel(row), id).toBe(AREA_LABEL[area]);
  }
  // The fixture exercises both shapes and the "now" row.
  expect(guided).toBeGreaterThan(0);
  expect(now).toBeGreaterThan(0);
});

test("the four area labels and the three plain state words are unchanged on the wall", async ({
  page,
}) => {
  await page.goto(DESIGN());
  const shell = await wallReadiness(page);
  await openRemainder(page);
  const stations = shell.locator(".readiness-rail .readiness-station");
  await expect(stations).toHaveCount(4);
  for (let i = 0; i < RAIL.length; i++) {
    const [id, label] = RAIL[i];
    await expect(stations.nth(i)).toHaveAttribute("data-area-id", id);
    await expect(
      stations.nth(i).locator(".readiness-station-label"),
    ).toHaveText(label);
    const formal = (await stations.nth(i).getAttribute("data-state"))!;
    expect(Object.keys(PLAIN_LABELS)).toContain(formal);
    await expect(stations.nth(i).locator(".readiness-state")).toHaveText(
      PLAIN_LABELS[formal],
    );
  }
  // Every card: the stage line is exactly its area label (no timing
  // suffix), the chip is exactly the plain word for its formal state.
  const rows = shell.locator("article[data-concern-id]");
  const n = await rows.count();
  expect(n).toBeGreaterThan(0);
  for (let i = 0; i < n; i++) {
    const row = rows.nth(i);
    const area = (await row.getAttribute("data-area-id"))!;
    const state = await formalState(row);
    expect(AREA_ORDER).toContain(area);
    expect(Object.keys(PLAIN_LABELS)).toContain(state);
    expect(await stageLabel(row), (await row.getAttribute("data-concern-id"))!).toBe(AREA_LABEL[area]);
    await expect(row.locator(".readiness-state")).toHaveText(
      PLAIN_LABELS[state],
    );
  }
  for (const word of await shell.locator(".readiness-state").allTextContents()) {
    expect(Object.values(PLAIN_LABELS)).toContain(word.trim());
  }
});

test("/readiness leads with guidance and keeps Fact, Concern, Timing, Blocking in the disclosure", async ({
  page,
}) => {
  await page.goto("/readiness");
  await openRemainder(page);
  const rows = page.locator("article[data-concern-id]");
  const n = await rows.count();
  // R-RR1-18's order-independence pin. Playwright runs these suites
  // alphabetically, so suite 50 has already edited the served store's
  // spec bytes by the time this one reads /readiness back. The cached
  // policy-conflict report the server warmed up at startup was computed
  // over the PRE-edit bytes, which used to make the loader fail and the
  // page render no rows at all. A digest mismatch is now a cache miss, so
  // the page still derives: rows are present, and the check-context area
  // carries its verdict row in the unproven state. State is read from the
  // article's own state class and id attribute, never innerText (lane
  // rule); no harness fixture is touched by this pin.
  //
  // spec/readiness-page-v2 ac-2 (SI-339 (2)): an unresolved row's one
  // primary line is its guidance, its fact filed in the disclosure; a
  // proven row, which carries no guidance, leads with its fact. A
  // human-review row carries its plain label between the step label and
  // the primary line.
  expect(n).toBeGreaterThan(0);
  const unprovenContext = page.locator(
    'article[data-concern-id^="context/"].readiness-concern--unproven',
  );
  expect(await unprovenContext.count()).toBeGreaterThan(0);
  let guided = 0;
  for (let i = 0; i < n; i++) {
    const row = rows.nth(i);
    const id = (await row.getAttribute("data-concern-id"))!;
    const state = await formalState(row);
    const shape = await cardShape(row);
    expect(shape.stage, id).toBe(0);
    expect(shape.summaries, id).toBe(1);
    // The primary line is the copy block's second child — its third
    // exactly when the plain human-review label sits between the step
    // label and it; nothing else ever precedes it (readiness-page-v2
    // ac-2: the base pin `primary === 1`, adapted to the human-review
    // label).
    const lead = await row.locator(".readiness-copy").evaluate((copy) => {
      const kids = Array.from(copy.children);
      const primary = kids.findIndex((k) => k.matches("p.readiness-summary"));
      return kids.slice(0, Math.max(primary, 0)).map((k) => k.className);
    });
    const humanReview = await row.locator(".readiness-human-review").count();
    expect(lead, id).toEqual(
      humanReview ? ["readiness-stage", "readiness-human-review"] : ["readiness-stage"],
    );
    expect(shape.primary, id).toBe(1 + humanReview);
    expect(shape.chip, id).toBeGreaterThan(shape.primary);
    const primary = row.locator(".readiness-copy > p.readiness-summary");
    if (state === "proven") {
      await expect(primary, id).not.toHaveClass(/readiness-guidance/);
    } else {
      await expect(primary, id).toHaveClass(/readiness-guidance/);
      guided++;
    }
    await row.locator(".readiness-tech summary").click();
    await expect(row.locator('dt:text-is("Fact") + dd')).not.toHaveText("");
    await expect(row.locator('dt:text-is("Concern") + dd')).toHaveText(id);
    await expect(row.locator('dt:text-is("Timing") + dd')).toHaveText(
      /^(current|eventual)$/,
    );
    await expect(row.locator('dt:text-is("Blocking") + dd')).toHaveText(
      /^(true|false)$/,
    );
  }
  expect(guided).toBeGreaterThan(0);
});

test("the policy setup guide points at verdi policy adopt --starter and stays read-only", async ({
  page,
}) => {
  await page.goto(DRAFT_B());
  const readiness = await openRecordTab(page, "readiness");
  const guide = readiness.getByTestId("asd-policy-guide");
  await expect(guide).toHaveAttribute("data-policy-guide", "not-adopted");
  await expect(guide).toContainText(
    "verdi policy adopt --starter [--profile solo|team]",
  );
  await expect(guide).not.toContainText("setup wizard");
  // Still exactly the four read-only check blocks: the verb is a pointer,
  // never a fifth copyable command.
  const blocks = guide.locator("pre.asd-policy-guide-cmd");
  await expect(blocks).toHaveCount(4);
  for (let i = 0; i < 4; i++) {
    await expect(blocks.nth(i)).not.toContainText("policy adopt");
  }
  await expect(
    guide.locator("form, button, input, select, textarea, [data-asd-panel]"),
  ).toHaveCount(0);
  // Inspect-first still precedes the verb, in the guide and on the row —
  // the context/policy row's home being the guide itself, under the
  // Readiness tab's Capabilities label (SI-368 (3)).
  const text = (await guide.textContent()) ?? "";
  expect(text.indexOf("Inspect first")).toBeGreaterThanOrEqual(0);
  expect(text.indexOf("Inspect first")).toBeLessThan(
    text.indexOf("verdi policy adopt"),
  );
  const capabilities = readiness.getByTestId("readiness-tab-capabilities");
  await expect(capabilities).toContainText("verdi policy adopt --starter");
  const row = (await capabilities.textContent()) ?? "";
  expect(row.indexOf("Capabilities"), "the guide sits under the Capabilities label").toBeGreaterThanOrEqual(0);
  expect(row.indexOf("Capabilities")).toBeLessThan(row.indexOf("Policy setup guide"));
  expect(row.indexOf("Inspect")).toBeGreaterThanOrEqual(0);
  expect(row.indexOf("Inspect")).toBeLessThan(row.indexOf("verdi policy adopt"));
});

test("light and dark schemes ink both the primary line and the secondary fact", async ({
  page,
}) => {
  const palette = async () =>
    page.evaluate(() => {
      const luminance = (color: string) => {
        const m = color.match(/\d+(\.\d+)?/g)!.map(Number);
        return (0.2126 * m[0] + 0.7152 * m[1] + 0.0722 * m[2]) / 255;
      };
      const ink = (sel: string) =>
        getComputedStyle(document.querySelector(sel)!).color;
      // The lines' ground is the record drawer's, where the wall's
      // readiness is read.
      const ground = getComputedStyle(document.querySelector('[data-testid="record-drawer"]')!).backgroundColor;
      const row = '[data-testid="readiness-tab"] article[data-concern-id]';
      return {
        bg: luminance(ground),
        summary: ink(row + " .readiness-summary"),
        fact: ink(row + " dd.readiness-fact"),
        summaryLum: luminance(ink(row + " .readiness-summary")),
        factLum: luminance(ink(row + " dd.readiness-fact")),
      };
    });

  // The secondary fact is read where the tab keeps it: in the first row's
  // opened disclosure.
  const openFirstFact = async () => {
    const shell = await wallReadiness(page);
    await openRemainder(page);
    await shell.locator("article[data-concern-id] .readiness-tech summary").first().click();
    await expect(shell.locator("article[data-concern-id] dd.readiness-fact").first()).toBeVisible();
  };
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto(DESIGN());
  await openFirstFact();
  const light = await palette();

  await page.emulateMedia({ colorScheme: "dark" });
  await page.reload();
  await openFirstFact();
  const dark = await palette();

  // The selected palette is actually used on both lines, and each stays
  // legible against its ground.
  expect(light.bg).toBeGreaterThan(0.5);
  expect(dark.bg).toBeLessThan(0.5);
  expect(light.summary).not.toBe(dark.summary);
  expect(light.fact).not.toBe(dark.fact);
  expect(Math.abs(light.summaryLum - light.bg)).toBeGreaterThan(0.3);
  expect(Math.abs(dark.summaryLum - dark.bg)).toBeGreaterThan(0.3);
  expect(Math.abs(light.factLum - light.bg)).toBeGreaterThan(0.3);
  expect(Math.abs(dark.factLum - dark.bg)).toBeGreaterThan(0.3);
});
