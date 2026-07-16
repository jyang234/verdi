import { test, expect } from "@playwright/test";
import { SHOWCASE } from "./fixtures";

// The advisory preview matrix page: GET /matrix/{story...} renders the
// same fold `verdi matrix --preview` computes (03 §Evidence records),
// always behind the mandatory PREVIEW — ADVISORY banner — a preview must
// never be mistaken for the gate's answer (internal/workbench/matrix.go).
//
// Fixture choice, disclosed: SHOWCASE.READONLY_SPEC's own derived
// snapshots pin fixturegit golden SHAs that the harness's fresh scratch
// repo does not contain, so ITS matrix URL renders the disclosed
// ancestry-error page (500), not a matrix — the record-backed target here
// is SHOWCASE.SLOT_WALL_SPEC instead, whose derived CI record and
// attestation are provisioned at the scratch repo's own main sha
// (cmd/e2eharness/provision_board.go), i.e. real fold-visible evidence.
// The committed-corpus render is proven on SHOWCASE.STORY_STUB_MATCHED,
// stale-decline's own realized story (implements spec/stale-decline#ac-2,
// story jira:LOAN-1482).

test("advisory preview matrix folds real records behind the mandatory banner", async ({ page }) => {
  await page.goto(`/matrix/spec/${SHOWCASE.SLOT_WALL_SPEC}`);
  await expect(page).toHaveTitle(/Advisory preview matrix:/);

  // The mandatory advisory banner (03 §Evidence records): impossible to
  // miss, and explicit that local evidence is never authoritative.
  const banner = page.locator(".preview-banner");
  await expect(banner).toBeVisible();
  await expect(banner).toContainText("PREVIEW");
  await expect(banner).toContainText("ADVISORY");
  await expect(banner).toContainText("never authoritative");

  // The folded per-AC table.
  const table = page.locator("table.matrix-table");
  await expect(table).toBeVisible();
  await expect(table.locator("thead")).toContainText("AC");
  await expect(table.locator("thead")).toContainText("Status");
  await expect(table.locator("thead")).toContainText("Evidence");

  // One declared AC, one row — carrying the REAL per-kind fold state: the
  // provisioned CI static record passes, the attestation file is present,
  // and behavioral has no record yet, so the AC folds to pending.
  const rows = table.locator("tbody tr");
  await expect(rows).toHaveCount(1);
  const row = rows.filter({ hasText: SHOWCASE.SLOT_WALL_AC });
  await expect(row).toContainText(`${SHOWCASE.SLOT_HELD_KIND}:pass`);
  await expect(row).toContainText(`${SHOWCASE.SLOT_ATTESTED_KIND}:present`);
  await expect(row).toContainText(`${SHOWCASE.SLOT_EMPTY_KIND}:none`);
  await expect(row.locator(".status-badge")).toHaveText("pending");

  // The story-level verdict line: no fail record → not violated; the
  // empty behavioral slot → not eligible.
  const summary = page.locator(".matrix-summary");
  await expect(summary).toContainText("story.violated: false");
  await expect(summary).toContainText("story.eligible: false");
});

test("advisory preview matrix renders a committed showcase story's fold", async ({ page }) => {
  await page.goto(`/matrix/spec/${SHOWCASE.STORY_STUB_MATCHED}`);

  const banner = page.locator(".preview-banner");
  await expect(banner).toBeVisible();
  await expect(banner).toContainText("PREVIEW");
  await expect(banner).toContainText("ADVISORY");

  // The committed story declares one AC with no derived records in this
  // store — the fold reads that honestly as no-signal, never a blank.
  const table = page.locator("table.matrix-table");
  await expect(table).toBeVisible();
  const rows = table.locator("tbody tr");
  await expect(rows).toHaveCount(1);
  const row = rows.filter({ hasText: "ac-1" });
  await expect(row).toContainText("PUT /applications/:id/update");
  await expect(row.locator(".status-badge")).toHaveText("no-signal");

  await expect(page.locator(".matrix-summary")).toContainText("story.violated:");
});
