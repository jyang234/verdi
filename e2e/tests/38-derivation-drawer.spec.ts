import { test, expect, type Locator, type Page } from "@playwright/test";
import { EDGE, boardPath } from "./fixtures";

// spec/derivation-drawer — the wall's receipts, opened:
//   ac-1: every wall badge is an opener (pointer AND keyboard) whose
//         drawer names the source rule id, the pinned inputs with their
//         revisions, and the firing records; closing restores the wall
//         untouched — proven on BOTH surfaces (a card chip and a
//         case-file stamp).
//   ac-2: the drawer renders from the badge's attached derivation record
//         alone — byte-identical across renders, field-for-field equal to
//         data-badge-record, nothing in the drawer without a record source.
//   ac-3: judged findings render on the case file wearing their sweep
//         provenance, and a stale or partial sweep LOOKS stale: the
//         fresh/stale/partial fixture walls carry real committed
//         decision-conflict reports (harness sweepSpec/sweepReport).
//   ac-4: every citation in drawer markup is a digest/sha/pinned field —
//         no rendered date or time (the clock-read proof itself is the Go
//         render test TestWriteBadgeDrawer_PureFunctionAcrossTime).

const REVISION_RE = /^(sha256:[0-9a-f]{64}|[0-9a-f]{7,40})$/;

// The one open drawer (boardspec.js moves the server-rendered element to
// the body while open; only ever one at a time).
function openDrawerEl(page: Page): Locator {
  return page.locator(".badge-drawer:not([hidden])");
}

function regionHTML(page: Page): Promise<string> {
  return page.locator("#boardv2-region").innerHTML();
}

// assertDrawerContent reads the open drawer's rule id, pinned inputs, and
// firing records — the ac-1 obligation's "reads its content, not merely
// its existence" bar — plus ac-4's citation shape on every revision cell.
async function assertDrawerContent(page: Page, source: string): Promise<void> {
  const drawer = openDrawerEl(page);
  await expect(drawer).toBeVisible();
  await expect(drawer).toHaveAttribute("role", "dialog");
  await expect(drawer.locator(".drawer-source")).toHaveText(source);

  const revs = drawer.locator(".drawer-input-rev");
  expect(await revs.count()).toBeGreaterThan(0);
  for (const rev of await revs.allTextContents()) {
    expect(rev).toMatch(REVISION_RE); // non-empty, and a digest/sha — never a date
  }
  expect(await drawer.locator(".drawer-record").count()).toBeGreaterThan(0);

  // ac-4: no rendered date or time anywhere in the drawer's own text.
  // (The clock-of-day pattern needs both colons — a bare \d{2}:\d{2}
  // would false-positive inside "…56:29…" of a sha256: digest.)
  expect(await drawer.innerText()).not.toMatch(/\d{4}-\d{2}-\d{2}|\b\d{1,2}:\d{2}:\d{2}\b/);
}

// openInspectClose drives ac-1's full loop on one badge: open by pointer,
// inspect, close by the close control; then open by keyboard, close by
// Escape (focus restored to the opener) — and after each close the wall's
// markup is byte-identical to what it was before the drawer ever opened.
async function openInspectClose(page: Page, badge: Locator, source: string): Promise<void> {
  const before = await regionHTML(page);

  // Pointer open → close control.
  await badge.click();
  await assertDrawerContent(page, source);
  await openDrawerEl(page).locator(".drawer-close").click();
  await expect(openDrawerEl(page)).toHaveCount(0);
  expect(await regionHTML(page)).toBe(before);

  // Keyboard open (the badge is a real button: Enter activates it) →
  // Escape close, focus restored to the opener (dc-4).
  await badge.focus();
  await page.keyboard.press("Enter");
  await assertDrawerContent(page, source);
  await page.keyboard.press("Escape");
  await expect(openDrawerEl(page)).toHaveCount(0);
  await expect(badge).toBeFocused();
  expect(await regionHTML(page)).toBe(before);
}

test.describe("ac-1: every wall badge opens its derivation drawer", () => {
  test("card chip and case-file stamp both open, read, and close clean", async ({ page }) => {
    await page.goto(boardPath(EDGE.BADGE_WALL_SPEC));

    // A card badge's drawer (the decision card's VL-003 chip)…
    const chip = page
      .getByTestId(`card-${EDGE.BADGE_DECISION}`)
      .locator('.badge-chip[data-badge-source="lint:VL-003"]');
    await openInspectClose(page, chip, "lint:VL-003");

    // …and a case-file badge's drawer (the spec-level VL-003 stamp).
    const stamp = page
      .getByTestId("case-file-badges")
      .locator('.case-stamp[data-badge-source="lint:VL-003"]');
    await openInspectClose(page, stamp, "lint:VL-003");
  });

  test("drawers open on a sealed read-only wall too (dc-4: reading a receipt is never a write)", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.BADGE_SEALED_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    const chip = page
      .getByTestId(`card-${EDGE.BADGE_DECISION}`)
      .locator('.badge-chip[data-badge-source="lint:VL-003"]');
    await openInspectClose(page, chip, "lint:VL-003");
  });
});

test.describe("ac-2: the drawer is a pure function of the attached record", () => {
  test("same record, same drawer bytes — and every drawer line has a record source", async ({
    page,
  }) => {
    // The hidden drawer body is the badge button's next sibling; read it
    // at rest (server-rendered, before any interaction).
    const chipDrawer = (p: Page) =>
      p
        .getByTestId(`card-${EDGE.BADGE_DECISION}`)
        .locator('.badge-chip[data-badge-source="lint:VL-003"] + .badge-drawer');

    await page.goto(boardPath(EDGE.BADGE_WALL_SPEC));
    const first = await chipDrawer(page).evaluate((el) => el.outerHTML);
    await page.reload();
    const second = await chipDrawer(page).evaluate((el) => el.outerHTML);
    expect(second).toBe(first); // byte-identical across two renders

    // Field-for-field correspondence with the serialized record the badge
    // itself carries (ADJ-8's opener contract): every input/record/
    // disclosure/provenance entry appears, and the drawer holds EXACTLY as
    // many entries as the record — nothing without a record source.
    const chip = page
      .getByTestId(`card-${EDGE.BADGE_DECISION}`)
      .locator('.badge-chip[data-badge-source="lint:VL-003"]');
    const record = JSON.parse((await chip.getAttribute("data-badge-record"))!);
    const drawer = chipDrawer(page);

    expect(await drawer.locator(".drawer-input").count()).toBe(record.inputs.length);
    for (const input of record.inputs) {
      const row = drawer.locator(".drawer-input", { hasText: input.name });
      expect(await row.locator(".drawer-input-path").textContent()).toBe(input.path);
      expect(await row.locator(".drawer-input-rev").textContent()).toBe(input.revision);
    }
    expect(await drawer.locator(".drawer-record").count()).toBe(record.records.length);
    for (const [i, rec] of record.records.entries()) {
      expect(await drawer.locator(".drawer-record").nth(i).textContent()).toBe(rec);
    }
    expect(await drawer.locator(".drawer-disclosure").count()).toBe(
      record.disclosures?.length ?? 0,
    );
    expect(await drawer.locator(".drawer-provenance-line").count()).toBe(
      record.provenance?.length ?? 0,
    );
    expect(await drawer.locator(".drawer-source").textContent()).toBe(record.source);
  });
});

test.describe("ac-3: judged findings wear their sweep provenance; stale looks stale", () => {
  // Opens the case file's judged chip and returns the open drawer.
  async function openJudgedDrawer(page: Page, spec: string): Promise<Locator> {
    await page.goto(boardPath(spec));
    const chip = page
      .getByTestId("case-file-badges")
      .locator('.case-stamp[data-badge-source="align:judged-sweep"]');
    await expect(chip).toBeVisible();
    await expect(chip).toHaveText("2 judged findings");
    await chip.click();
    const drawer = openDrawerEl(page);
    await expect(drawer).toBeVisible();
    return drawer;
  }

  // Every fixture's drawer shows finding disposition states: the
  // dispositioned finding with its note, the undispositioned one
  // disclosed as such (the obligation's per-fixture demand).
  async function assertDispositionStates(drawer: Locator): Promise<void> {
    const records = drawer.locator(".drawer-record");
    await expect(records).toHaveCount(2);
    const first = (await records.nth(0).textContent())!;
    expect(first).toContain("judged-dcf-1 [no-conflict]");
    expect(first).toContain("note: reviewed against the corpus");
    expect((await records.nth(1).textContent())!).toContain("judged-dcf-2 [undispositioned]");
  }

  test("fresh, complete sweep: provenance stamped, no mismatch line", async ({ page }) => {
    const drawer = await openJudgedDrawer(page, EDGE.SWEEP_FRESH_SPEC);

    const provenance = drawer.locator(".drawer-provenance-line");
    await expect(provenance).toHaveCount(3);
    expect((await provenance.nth(0).textContent())!).toMatch(/^sweep covers [0-9a-f]{40}$/);
    expect((await provenance.nth(1).textContent())!).toMatch(
      /^adr_corpus_digest sha256:[0-9a-f]{64}$/,
    );
    expect((await provenance.nth(2).textContent())!).toBe(
      `decisions_scanned: spec/${EDGE.SWEEP_FRESH_SPEC}#dc-1, spec/${EDGE.SWEEP_FRESH_SPEC}#dc-2`,
    );

    // Fresh and complete: NO mismatch/disclosure line at all.
    await expect(drawer.locator(".drawer-disclosure")).toHaveCount(0);
    await assertDispositionStates(drawer);
  });

  test("stale sweep: the covers contrast is disclosed visibly", async ({ page }) => {
    const drawer = await openJudgedDrawer(page, EDGE.SWEEP_STALE_SPEC);
    const disclosures = drawer.locator(".drawer-disclosure");
    await expect(disclosures).toHaveCount(1);
    expect((await disclosures.first().textContent())!).toMatch(
      /^sweep covers [0-9a-f]{40}; this wall renders sha256:[0-9a-f]{64}$/,
    );
    await assertDispositionStates(drawer);
  });

  test("partial sweep: the missing declared decision id is named", async ({ page }) => {
    const drawer = await openJudgedDrawer(page, EDGE.SWEEP_PARTIAL_SPEC);
    const disclosures = drawer.locator(".drawer-disclosure");
    await expect(disclosures).toHaveCount(1);
    await expect(disclosures.first()).toHaveText(
      `${EDGE.SWEEP_MISSING_DECISION} is not in decisions_scanned`,
    );
    await assertDispositionStates(drawer);
  });
});
