import { test, expect } from "@playwright/test";
import { DEX_BASE, SHOWCASE } from "./fixtures";

// spec/disclosures-panel (spec/disclosure-legibility ac-1/ac-2): the
// operator's "what is verdi not proving right now" surface — one view,
// two editions, one compute path (internal/disclosureview over the
// internal/disclosure seam; 05 §Lenses: "the dex ships their read-only,
// main-only editions, computed the same way — no separate logic path").
//
// The two editions deliberately show DIFFERENT checkout states here, and
// both are the truth of their own moment of compute:
//   - the workbench serves live: the harness store names its `forge:`
//     (SHOWCASE.FORGE_KIND) in examples/showcase's own committed
//     verdi.yaml and exports no credentials, so the serving process's one
//     real disclosed state is the review-feed-unavailable disclosure (the
//     same mcp:review-feed value list_annotations discloses) — the seeded
//     disclosure this suite asserts, sourced from the real showcase
//     manifest rather than an arbitrary literal (showcase-coverage
//     Task 3.4; SHOWCASE.FORGE_KIND's own doc comment in fixtures.ts).
//   - the dex baked main at build time: the mutable zone was present and
//     no process context applies, so its enumeration is honestly empty —
//     which is exactly the empty-state path (a positive claim, never a
//     blank page).

test.describe("workbench /disclosures: the live edition", () => {
  test("renders the checkout's seeded disclosure with every seam field (ac-1, ac-2)", async ({
    page,
  }) => {
    await page.goto("/disclosures");

    const view = page.locator("section.disclosures-view");
    await expect(view).toBeVisible();
    await expect(view).toHaveAttribute("data-count", "1");

    const item = view.locator(".disclosure-item");
    await expect(item).toHaveCount(1);

    // ac-2: source, text, severity, stable id — one consistent rendering.
    await expect(item.locator(".disclosure-severity")).toHaveText(
      "disclosed-unproven",
    );
    await expect(item.locator(".disclosure-source")).toHaveText(
      "mcp:review-feed",
    );
    await expect(item.locator(".disclosure-text")).toHaveText(
      `forge "${SHOWCASE.FORGE_KIND}" is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown`,
    );
    await expect(item).toHaveAttribute(
      "data-disclosure-id",
      "mcp:review-feed",
    );
  });

  test("states its compute honesty: fresh per render, never persisted (ac-1)", async ({
    page,
  }) => {
    await page.goto("/disclosures");
    const note = page.locator(".disclosures-note");
    await expect(note).toContainText("never persisted");
  });

  test("is discoverable from the workbench home page", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("link", { name: "Disclosures" }).click();
    await expect(page).toHaveURL(/\/disclosures$/);
    await expect(page.locator("section.disclosures-view")).toBeVisible();
  });
});

test.describe("dex /disclosures/: the read-only edition (ac-3)", () => {
  test("ships the same view structure with the honest empty state", async ({
    page,
  }) => {
    await page.goto(`${DEX_BASE}/disclosures/`);

    // Same shared container the workbench edition renders — the one
    // compute path's own markup, not a dex-private lookalike.
    const view = page.locator("section.disclosures-view");
    await expect(view).toBeVisible();
    await expect(view).toHaveAttribute("data-count", "0");

    // "No current disclosures" is a positive claim and reads like one.
    const empty = view.locator(".disclosures-empty");
    await expect(empty).toBeVisible();
    await expect(empty.locator(".disclosures-empty-claim")).toHaveText(
      "No current disclosures.",
    );
    await expect(empty.locator(".disclosures-empty-detail")).toContainText(
      "a computed claim, not a silent pass",
    );
    await expect(view.locator(".disclosure-item")).toHaveCount(0);
  });

  test("carries the dex's temporal honesty and no editing affordance", async ({
    page,
  }) => {
    await page.goto(`${DEX_BASE}/disclosures/`);

    // Living-gated build stamp: this edition claims build-time currency,
    // never live currency (05 §Verdi-dex temporal classes).
    const banner = page.locator(".temporal-banner");
    await expect(banner).toBeVisible();
    await expect(banner).toContainText("main @");

    // Read-only edition: no forms, no buttons, nothing editable.
    await expect(page.locator("form, button, [contenteditable]")).toHaveCount(
      0,
    );

    // And it is discoverable from the dex home hub.
    await page.goto(`${DEX_BASE}/`);
    await page.getByRole("link", { name: "Disclosures", exact: true }).click();
    await expect(page.locator("section.disclosures-view")).toBeVisible();
  });
});

test.describe("cross-edition parity: one vocabulary, one markup contract", () => {
  test("both editions render the shared view with identical structure and vocabulary", async ({
    page,
  }) => {
    // The workbench item's vocabulary fields...
    await page.goto("/disclosures");
    const wbSeverity = await page
      .locator(".disclosure-severity")
      .first()
      .textContent();
    const wbViewClass = await page
      .locator("section.disclosures-view")
      .getAttribute("class");

    // ...and the dex edition's container come from the one shared
    // renderer: same container class; and the severity token equals the
    // seam's one severity value — the same word the CLI's rendered lines
    // carry ("disclosed-unproven [<source>]: <text>"), so recognizing a
    // disclosure on one surface teaches all of them (feature ac-1).
    await page.goto(`${DEX_BASE}/disclosures/`);
    const dexViewClass = await page
      .locator("section.disclosures-view")
      .getAttribute("class");

    expect(wbSeverity).toBe("disclosed-unproven");
    expect(dexViewClass).toBe(wbViewClass);

    // Both editions carry the shared note element (each edition's own
    // honest compute-provenance line — the one legitimate difference).
    await expect(page.locator(".disclosures-note")).toBeVisible();
  });
});
