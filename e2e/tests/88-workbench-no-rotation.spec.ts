import { test, expect, type Page, type Locator } from "@playwright/test";
import { SHOWCASE, EDGE, boardPath } from "./fixtures";
import { addSticky, dragToTrash, expectAutosaved, pinArtifact } from "./helpers";

// spec/chrome-and-tokens-v2 ac-3: no workbench element is rotated, and the
// stamps are drawn as chips — status, class, and mode marks render as
// chips — and the hand font appears only on a story or spike sticky
// parked in the stubs band; every other card, badge, and label uses the
// body or monospace face. The test is the producer its obligation names
// (.verdi/obligations/chrome-and-tokens-v2/ac-3--behavioral.md), titled as
// the claim spells it; it passes when run alone (BL-98).
//
// It reads computed styles — transform, border radius, font family — never
// a screenshot (recording stays off).

// rotated reports whether a computed transform carries a rotation (or a
// skew): a 2-D matrix with an off-diagonal term, a 3-D matrix with any
// off-diagonal term in its upper 3×3, or a transform in a form the test
// does not read (never a silent pass).
function rotated(transform: string): boolean {
  if (!transform || transform === "none") return false;
  const m2 = transform.match(/^matrix\(([^)]+)\)$/);
  if (m2) {
    const [, b, c] = m2[1].split(",").map((v) => Number(v.trim()));
    return Math.abs(b) > 1e-6 || Math.abs(c) > 1e-6;
  }
  const m3 = transform.match(/^matrix3d\(([^)]+)\)$/);
  if (m3) {
    const v = m3[1].split(",").map((s) => Number(s.trim()));
    return [v[1], v[2], v[4], v[6], v[8], v[9]].some((x) => Math.abs(x) > 1e-6);
  }
  return true;
}

interface Styled {
  what: string;
  transform: string;
  radius: string;
  upper: string;
  font: string;
  border: string;
}

// stylesOf reads the chip-relevant computed styles of an element, or of
// one of its pseudo-elements.
async function stylesOf(el: Locator, pseudo?: string): Promise<Styled> {
  return el.evaluate(
    (node, p) => {
      const c = getComputedStyle(node, p || null);
      return {
        what: node.tagName.toLowerCase() + (node.className ? "." + String(node.className).split(" ").join(".") : "") + (p || ""),
        transform: c.transform,
        radius: c.borderTopLeftRadius,
        upper: c.textTransform,
        font: c.fontFamily,
        border: c.borderTopWidth,
      };
    },
    pseudo,
  );
}

// expectChip: the handoff's chip — no rotation, a 3 px radius, uppercase
// mono lettering, at most a 1 px edge.
async function expectChip(s: Styled): Promise<void> {
  expect(rotated(s.transform), `${s.what}: transform ${s.transform}`).toBe(false);
  expect(s.radius, `${s.what}: radius`).toBe("3px");
  expect(s.upper, `${s.what}: case`).toBe("uppercase");
  expect(s.font, `${s.what}: face`).toMatch(/mono|menlo|consolas/i);
  expect(parseFloat(s.border), `${s.what}: edge`).toBeLessThanOrEqual(1);
}

// rotatedElements scans every element on the page and its ::before and
// ::after pseudo-elements for a rotation.
async function rotatedElements(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const out: string[] = [];
    const describe = (el: Element, pseudo: string) =>
      el.tagName.toLowerCase() +
      (el.id ? "#" + el.id : "") +
      (typeof el.className === "string" && el.className ? "." + el.className.split(" ").join(".") : "") +
      pseudo;
    const rot = (t: string): boolean => {
      if (!t || t === "none") return false;
      const m2 = t.match(/^matrix\(([^)]+)\)$/);
      if (m2) {
        const v = m2[1].split(",").map((x) => Number(x.trim()));
        return Math.abs(v[1]) > 1e-6 || Math.abs(v[2]) > 1e-6;
      }
      const m3 = t.match(/^matrix3d\(([^)]+)\)$/);
      if (m3) {
        const v = m3[1].split(",").map((x) => Number(x.trim()));
        return [v[1], v[2], v[4], v[6], v[8], v[9]].some((x) => Math.abs(x) > 1e-6);
      }
      return true;
    };
    for (const el of Array.from(document.querySelectorAll("*"))) {
      if (rot(getComputedStyle(el).transform)) out.push(describe(el, ""));
      for (const pseudo of ["::before", "::after"]) {
        const c = getComputedStyle(el, pseudo);
        if (c.content !== "none" && c.content !== "normal" && rot(c.transform)) out.push(describe(el, pseudo));
      }
    }
    return out;
  });
}

// handElements: every element whose computed font family is the hand
// face (the --hand token's cursive stack), with the sticky it sits in.
async function handElements(page: Page): Promise<{ what: string; sticky: string | null }[]> {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll("*"))
      .filter((el) => /bradley hand|marker felt|cursive/i.test(getComputedStyle(el).fontFamily))
      .map((el) => ({
        what:
          el.tagName.toLowerCase() +
          (typeof el.className === "string" && el.className ? "." + el.className.split(" ").join(".") : ""),
        sticky: el.closest(".sticky")?.getAttribute("class") ?? null,
      })),
  );
}

test.describe("chrome-and-tokens", () => {
  test("Nothing rotated, stamps drawn as chips, handwriting only on a parked sticky", async ({ page }) => {
    test.setTimeout(120_000);
    const wall = boardPath(SHOWCASE.DESIGN_SPEC);
    await page.goto(wall);
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");

    // A wall carrying every card kind: the fixture's object cards (acceptance
    // criteria, constraints, decisions, open questions) and stub cards, a
    // pinned reference card (pinned here when the wall carries none), and
    // a story sticky parked in the stubs band — a fresh sticky lands in the
    // scratch corner, so the position API parks it in the band, exactly as
    // a drag would.
    let pinnedHere = false;
    if ((await page.locator(".refcard").count()) === 0) {
      await pinArtifact(page, SHOWCASE.PIN_ADR, SHOWCASE.PIN_ADR);
      pinnedHere = true;
    }
    for (const kind of ["acceptance-criterion", "constraint", "decision", "open-question"]) {
      expect(await page.locator(`.objcard[data-object-kind="${kind}"]`).count(), kind).toBeGreaterThan(0);
    }
    expect(await page.locator(".stubcard").count(), "stub cards").toBeGreaterThan(0);
    expect(await page.locator(".refcard").count(), "reference cards").toBeGreaterThan(0);

    const sticky = await addSticky(page, "a story thought parked in the band [88]", "story");
    const id = await sticky.getAttribute("data-id");
    expect(id).toBeTruthy();
    const band = await page.getByTestId("zone-label-stub").evaluate((el) => ({
      left: parseFloat((el as HTMLElement).style.left),
      width: parseFloat((el as HTMLElement).style.width),
    }));
    const parkedX = band.left + 8;
    const park = await page.request.post(wall + "/api/sticky-position", { data: { id, x: parkedX, y: 420 } });
    expect(park.status(), await park.text()).toBe(200);
    await page.reload();
    const parked = page.getByTestId("sticky-" + id);
    await expect(parked).toBeVisible();
    await expect(parked).toHaveClass(/sticky--story/);
    const x = await parked.evaluate((el) => parseFloat((el as HTMLElement).style.left));
    expect(x).toBeGreaterThanOrEqual(band.left);
    expect(x).toBeLessThan(band.left + band.width);

    try {
      // No workbench element is rotated: every element and pseudo-element.
      expect(await rotatedElements(page)).toEqual([]);

      // Status, class, and mode marks render as chips: the bar's mode and
      // class chips, and the case file's class tag.
      await expectChip(await stylesOf(page.locator(".board-mode-tag")));
      await expectChip(await stylesOf(page.getByTestId("topbar-class-chip")));
      await expectChip(await stylesOf(page.getByTestId("case-class-tag")));

      // The hand font appears only on the parked story sticky; every other
      // card, badge, and label uses the body or monospace face.
      const hand = await handElements(page);
      expect(hand.length, "the parked sticky is handwritten").toBeGreaterThan(0);
      for (const h of hand) {
        expect(h.sticky, `${h.what} wears the hand face outside a parked story or spike sticky`).toMatch(
          /sticky--(story|spike)/,
        );
      }
      const body = await stylesOf(parked.locator(".sticky-body"));
      expect(body.font, "the parked sticky's body").toMatch(/bradley hand|marker felt|cursive/i);
      for (const other of [".objcard .card-text", ".stubcard", ".refcard", ".zone-label", ".placard", ".badge-chip"]) {
        const n = await page.locator(other).count();
        for (let i = 0; i < n; i++) {
          const s = await stylesOf(page.locator(other).nth(i));
          expect(s.font, `${other} #${i} uses the body or monospace face`).not.toMatch(/bradley hand|marker felt|cursive/i);
        }
      }
    } finally {
      await dragToTrash(page, parked);
      await expectAutosaved(page);
      await expect(page.getByTestId("sticky-" + id)).toHaveCount(0);
      if (pinnedHere) {
        await dragToTrash(page, page.locator(`.refcard[data-ref="${SHOWCASE.PIN_ADR}"]`));
        await expectAutosaved(page);
      }
    }

    // The sealed record's stamp is a chip too (the readonly wall's
    // ::after), and so is the terminal status badge on a superseded wall
    // and the case file's spec-level stamp on a badged wall; nothing on
    // those walls is rotated either.
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.locator("body")).toHaveClass(/mode-readonly/);
    await expectChip(await stylesOf(page.locator(".boardv2-canvas"), "::after"));
    expect(await rotatedElements(page)).toEqual([]);

    await page.goto(boardPath(SHOWCASE.SUPERSEDED_FEATURE_SPEC));
    await expectChip(await stylesOf(page.getByTestId("board-status-badge")));
    expect(await rotatedElements(page)).toEqual([]);

    await page.goto(boardPath(EDGE.BADGE_WALL_SPEC));
    const stamp = await stylesOf(page.getByTestId("case-file-badges").locator(".case-stamp").first());
    expect(rotated(stamp.transform), `${stamp.what}: transform ${stamp.transform}`).toBe(false);
    expect(stamp.radius, `${stamp.what}: radius`).toBe("3px");
    expect(stamp.font, `${stamp.what}: face`).toMatch(/mono|menlo|consolas/i);
    expect(await rotatedElements(page)).toEqual([]);
  });
});
