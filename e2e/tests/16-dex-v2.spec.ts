import { test, expect } from "@playwright/test";
import {
  FEATURE_SPEC,
  STORY_STUB_MATCHED,
  STORY_WITH_SPEC_STALE,
  STORY_WITH_PENDING_SUPERSESSION,
  ADR_NAME,
  dexSpecPath,
  dexAdrExemptionsPath,
} from "./fixtures";

// EXECUTABLE ACCEPTANCE CRITERIA — PLAN-V1.md §5 Phase V1-P8, exit
// criteria: "the feature page renders the stub list paired with the
// computed live mapping under the 'acceptance-time plan; current mapping
// computed below' banner, never the frozen stubs alone; the exemption page
// lists the fixture ADR's active exemptions with the exempting specs
// named; a story page carries a spec-stale badge and a
// pending-supersession badge on the fixture stories that carry those
// flags"; 05 §Lenses (feature lens, story-lens ladder badges, the per-ADR
// exemption page) and §Verdi-dex (the read-only, main-only editions,
// "computed the same way — no separate logic path").
test.describe("V1-P8: dex feature page — stubs paired with the live mapping", () => {
  test("stub plan and computed live mapping render together under the banner", async ({
    page,
  }) => {
    await page.goto(dexSpecPath(FEATURE_SPEC));

    // The banner is the honesty device: the frozen stubs are an
    // acceptance-time plan, the current mapping is computed below.
    const banner = page.getByTestId("acceptance-plan-banner");
    await expect(banner).toBeVisible();
    await expect(banner).toContainText("acceptance-time plan");
    await expect(banner).toContainText("current mapping computed below");

    // The frozen stubs (the fixture feature carries three — PLAN-V1 §4)...
    const stubPlan = page.getByTestId("stub-plan");
    await expect(stubPlan).toBeVisible();
    await expect(stubPlan.locator('[data-testid^="stub-"]')).toHaveCount(3);
    await expect(stubPlan).toContainText(STORY_STUB_MATCHED);

    // ...are PAIRED with the computed live mapping (the inverse of the
    // stories' implements edges — the feature is downward-blind, 02 §Link
    // taxonomy), never rendered alone.
    const liveMapping = page.getByTestId("live-mapping");
    await expect(liveMapping).toBeVisible();
    await expect(liveMapping).toContainText(STORY_STUB_MATCHED);
  });
});

test.describe("V1-P8: per-ADR exemption page", () => {
  // 05 §Lenses: "A per-ADR exemption page (the human face of verdi audit)
  // lists an ADR's active exemptions and the exempting specs, computed and
  // countable — 'ADR-7: 9 active exemptions.'"
  test("the exemption page lists active exemptions with the exempting specs named, countably", async ({
    page,
  }) => {
    await page.goto(dexAdrExemptionsPath(ADR_NAME));

    const heading = page.getByRole("heading", { name: /active exemption/i });
    await expect(heading).toBeVisible();

    const items = page
      .getByTestId("exemption-list")
      .locator('[data-testid^="exemption-"]');
    const itemCount = await items.count();
    expect(itemCount).toBeGreaterThan(0);

    // Countable means the stated count IS the list's length.
    const headingText = (await heading.textContent()) ?? "";
    const stated = headingText.match(/(\d+)\s+active exemption/i);
    expect(stated, `heading "${headingText}" carries no count`).not.toBeNull();
    expect(Number(stated![1])).toBe(itemCount);

    // The exempting spec is named: the fixture feature's decision carries
    // the exempts edge against this ADR (PLAN-V1 §4).
    await expect(
      items.filter({ hasText: FEATURE_SPEC }),
    ).not.toHaveCount(0);
  });
});

test.describe("V1-P8: story-page ladder badges", () => {
  // 05 §Lenses (story lens): "ladder state: spec-stale and
  // pending-supersession flags (§3b of the concept) surfaced alongside AC
  // and story status"; §Verdi-dex: story pages carry ladder state
  // "read-only, computed identically to the workbench story lens".
  test("a story flagged spec-stale renders the spec-stale badge", async ({
    page,
  }) => {
    await page.goto(dexSpecPath(STORY_WITH_SPEC_STALE));
    const badge = page.getByTestId("badge-spec-stale");
    await expect(badge).toBeVisible();
    await expect(badge).toContainText("spec-stale");
  });

  test("a story flagged pending-supersession renders the pending-supersession badge", async ({
    page,
  }) => {
    await page.goto(dexSpecPath(STORY_WITH_PENDING_SUPERSESSION));
    const badge = page.getByTestId("badge-pending-supersession");
    await expect(badge).toBeVisible();
    await expect(badge).toContainText("pending-supersession");
  });
});
