import { test, expect, Page } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// spec/spec-documents Wave 4 Task 4 — guidance-first concern cards (ac-12,
// R-W4-7) and the policy guide's verb pointer (ac-10, R-W4-8).
//
// On the wall shell and the /readiness cockpit each concern card shows its
// guidance sentence as the primary line when it carries one (otherwise the
// summary), keeps the fact visible as a secondary .asd-fact line, and files
// the concern id, timing and blocking flag in the existing Technical
// details disclosure. The four area labels and the plain-word triad are
// unchanged; nothing here changes a derivation. State assertions ride
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
// indexes: the first primary line, the state chip, the secondary fact,
// and which testid the primary line carries.
async function cardShape(row: ReturnType<Page["locator"]>) {
  return row.locator(".readiness-copy").evaluate((copy) => {
    const kids = Array.from(copy.children);
    const idx = (sel: string) => kids.findIndex((k) => k.matches(sel));
    return {
      stage: idx("p.readiness-stage"),
      primary: idx("p.readiness-summary"),
      guidance: idx('p.readiness-summary[data-testid^="asd-guidance-"]'),
      summary: idx('p.readiness-summary[data-testid^="asd-summary-"]'),
      chip: idx(".readiness-state"),
      fact: idx('p.asd-fact[data-testid^="asd-fact-"]'),
      summaries: kids.filter((k) => k.matches("p.readiness-summary")).length,
    };
  });
}

test("wall cards lead with guidance, keep the fact visible, and file timing under Technical details", async ({
  page,
}) => {
  await page.goto(DESIGN());
  await openRemainder(page);
  const shell = page.getByTestId("asd-shell");
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

    // Primary line first, then the chip; the fact only after the chip and
    // only when guidance leads. Exactly one primary line per card.
    const shape = await cardShape(row);
    expect(shape.stage, id).toBe(0);
    expect(shape.primary, id).toBeGreaterThan(shape.stage);
    expect(shape.chip, id).toBeGreaterThan(shape.primary);
    expect(shape.summaries, id).toBe(1);
    if (shape.guidance >= 0) {
      expect(shape.guidance, id).toBe(shape.primary);
      expect(shape.fact, id).toBeGreaterThan(shape.chip);
      guided++;
    } else {
      expect(shape.summary, id).toBe(shape.primary);
      expect(shape.fact, id).toBe(-1);
    }

    // Timing: now on the focus area, later after it, none otherwise or
    // on a proven row — as a data attribute and a disclosure row, never
    // on the stage line.
    let expected: "now" | "later" | null = null;
    if (state !== "proven" && focus) {
      if (area === focus) expected = "now";
      else if (AREA_ORDER.indexOf(area) > AREA_ORDER.indexOf(focus))
        expected = "later";
    }
    if (expected) {
      await expect(row).toHaveAttribute("data-timing", expected);
    } else {
      expect(await row.getAttribute("data-timing"), id).toBeNull();
    }
    await row.locator(".readiness-tech summary").click();
    await expect(row.locator('dt:text-is("Concern") + dd')).toHaveText(id);
    await expect(row.locator('dt:text-is("Blocking") + dd')).toHaveText(
      /^(true|false)$/,
    );
    const timing = row.locator('dt:text-is("Timing") + dd');
    if (expected === "now") {
      await expect(timing).toHaveText("now");
      now++;
    } else if (expected === "later") {
      await expect(timing).toHaveText(/^later — waits on /);
      await expect(timing).toHaveText(
        `later — waits on ${AREA_LABEL[focus!]}`,
      );
    } else {
      await expect(timing).toHaveCount(0);
    }
  }
  // The fixture exercises both shapes and the "now" row.
  expect(guided).toBeGreaterThan(0);
  expect(now).toBeGreaterThan(0);
});

test("the four area labels and the three plain state words are unchanged on the wall", async ({
  page,
}) => {
  await page.goto(DESIGN());
  await openRemainder(page);
  const shell = page.getByTestId("asd-shell");
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
    await expect(row.locator(".readiness-stage")).toHaveText(AREA_LABEL[area]);
    await expect(row.locator(".readiness-state")).toHaveText(
      PLAIN_LABELS[state],
    );
  }
  for (const word of await shell.locator(".readiness-state").allTextContents()) {
    expect(Object.values(PLAIN_LABELS)).toContain(word.trim());
  }
});

test("/readiness keeps the summary primary and Concern, Timing, Blocking in the disclosure", async ({
  page,
}) => {
  await page.goto("/readiness");
  await openRemainder(page);
  const rows = page.locator("article[data-concern-id]");
  const n = await rows.count();
  expect(n).toBeGreaterThan(0);
  for (let i = 0; i < n; i++) {
    const row = rows.nth(i);
    const id = (await row.getAttribute("data-concern-id"))!;
    const shape = await cardShape(row);
    expect(shape.stage, id).toBe(0);
    expect(shape.primary, id).toBe(1);
    expect(shape.chip, id).toBeGreaterThan(shape.primary);
    await row.locator(".readiness-tech summary").click();
    await expect(row.locator('dt:text-is("Concern") + dd')).toHaveText(id);
    await expect(row.locator('dt:text-is("Timing") + dd')).toHaveCount(1);
    await expect(row.locator('dt:text-is("Blocking") + dd')).toHaveText(
      /^(true|false)$/,
    );
  }
});

test("the policy setup guide points at verdi policy adopt --starter and stays read-only", async ({
  page,
}) => {
  await page.goto(DRAFT_B());
  const guide = page.getByTestId("asd-policy-guide");
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
  // Inspect-first still precedes the verb, in the guide and on the row.
  const text = (await guide.textContent()) ?? "";
  expect(text.indexOf("Inspect first")).toBeGreaterThanOrEqual(0);
  expect(text.indexOf("Inspect first")).toBeLessThan(
    text.indexOf("verdi policy adopt"),
  );
  await openRemainder(page);
  const rowGuidance = page.getByTestId("asd-guidance-context/policy");
  await expect(rowGuidance).toContainText("verdi policy adopt --starter");
  const row = (await rowGuidance.textContent()) ?? "";
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
      const bodyBg = getComputedStyle(document.body).backgroundColor;
      return {
        bg: luminance(bodyBg),
        summary: ink("#asd-shell article[data-concern-id] .readiness-summary"),
        fact: ink("#asd-shell article[data-concern-id] .asd-fact"),
        summaryLum: luminance(
          ink("#asd-shell article[data-concern-id] .readiness-summary"),
        ),
        factLum: luminance(
          ink("#asd-shell article[data-concern-id] .asd-fact"),
        ),
      };
    });

  await page.emulateMedia({ colorScheme: "light" });
  await page.goto(DESIGN());
  await openRemainder(page);
  const light = await palette();

  await page.emulateMedia({ colorScheme: "dark" });
  await page.reload();
  await openRemainder(page);
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
