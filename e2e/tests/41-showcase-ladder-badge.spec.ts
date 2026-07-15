import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// spec/badge-computes ac-3 (wall badges, story-class case-file concern) +
// spec/derivation-drawer ac-1/ac-2 (the opener contract), proven TOGETHER
// on REAL, committed showcase content rather than 37-board-wall-badges.spec.ts
// / 38-derivation-drawer.spec.ts's EDGE-zoned, deliberately-synthesized
// VL-violation rigs (showcase-coverage Task 3.4; those two files stay
// exactly as they are — a lint violation is not showcase material by
// definition, and EDGE fixtures are never coverage evidence, ledger L-B).
//
// examples/showcase's own committed borrower-update-mobile carries a
// genuine accepted-deviation whose finding targets its own declared ac-1
// (decisionsweep.SpecStale's trigger (a) — the exact scar
// public-rollout-plan Task 1.4 authored and cli_showcase_test.go's `audit`
// subtest already proves from the CLI side). Because borrower-update-mobile
// is class: story, its board's case file computes and wears a REAL
// ladder:spec-stale badge (internal/wallbadge/ladder.go's SpecStaleBadge —
// the exact entry point internal/dex/lens.go's own ladder-badge computation
// calls, so this is the same fold the dex's spec-stale flag renders, proven
// here on the workbench board instead). Opening it exercises the derivation
// drawer's full contract against genuine content: the real deviation
// report's own path and covers-sha as the pinned input, and the real
// accepted-deviation finding id as a firing record — never a canned or
// synthesized record.
test.describe("real showcase ladder badge: spec-stale on borrower-update-mobile", () => {
  test("the case file wears a real ladder:spec-stale badge whose derivation drawer cites the real deviation report", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.STORY_WITH_SPEC_STALE));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );

    const stamp = page
      .getByTestId("case-file-badges")
      .locator('.case-stamp[data-badge-source="ladder:spec-stale"]');
    await expect(stamp).toBeVisible();
    // dc-4's opener contract, shared by every wall badge: a real BUTTON.
    expect(await stamp.evaluate((el) => el.tagName)).toBe("BUTTON");

    // The record is byte-inspectable before the drawer is ever opened —
    // real deviation-report path, a real commit-sha revision (never a
    // canned digest), and the real finding id among the firing records.
    const raw = await stamp.getAttribute("data-badge-record");
    expect(raw).toBeTruthy();
    const record = JSON.parse(raw!);
    expect(record.source).toBe("ladder:spec-stale");
    expect(record.label).toBe("spec-stale");
    expect(record.inputs.length).toBe(1);
    expect(record.inputs[0].name).toBe("deviation-report");
    expect(record.inputs[0].path).toBe(
      `.verdi/specs/active/${SHOWCASE.STORY_WITH_SPEC_STALE}/deviation-report.md`,
    );
    expect(record.inputs[0].revision).toMatch(/^[0-9a-f]{7,40}$/);
    expect(record.records).toContain("ac-1");

    // ac-1's full opener loop: open by pointer, read the SAME content back
    // out of the rendered drawer (never merely a second copy of the
    // attribute), close.
    await stamp.click();
    const drawer = page.locator(".badge-drawer:not([hidden])");
    await expect(drawer).toBeVisible();
    await expect(drawer).toHaveAttribute("role", "dialog");
    await expect(drawer.locator(".drawer-source")).toHaveText(
      "ladder:spec-stale",
    );
    await expect(drawer.locator(".drawer-record").first()).toContainText(
      "ac-1",
    );

    // ac-4: no rendered date or time anywhere in the drawer — every
    // citation is a digest/sha/pinned field, never wall-clock.
    expect(await drawer.innerText()).not.toMatch(/\d{4}-\d{2}-\d{2}/);

    await drawer.locator(".drawer-close").click();
    await expect(page.locator(".badge-drawer:not([hidden])")).toHaveCount(0);
  });
});
