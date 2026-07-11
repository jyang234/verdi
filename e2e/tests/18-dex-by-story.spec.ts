import { test, expect } from "@playwright/test";
import {
  DEX_BASE,
  ADR_NAME,
  ARCHIVED_STORY_ROUND4,
  ARCHIVED_STORY_GRANDFATHERED,
  dexByStoryPath,
  dexAdrExemptionsPath,
} from "./fixtures";

// V1-P8's dex behavior beyond the 16-dex-v2 contract (CLAUDE.md: every
// browser-facing behavioral path gets a Playwright spec): the by-story
// axis — 05 §Verdi-dex IA "by story (the archived quartet: spec, board,
// rollup, deviation report)" — including the round-four board-slot rule
// (00 §Glossary "the quartet"; 03 §Alignment report round-four note:
// layout.json freezes in the board slot; board.json is the grandfathered
// v0 form), plus the ADR permalink's link to its exemption page.
test.describe("V1-P8: dex by-story axis", () => {
  test("the hub lists both archived story records", async ({ page }) => {
    await page.goto(`${DEX_BASE}/by-story/`);
    await expect(
      page.locator(`a[href="/by-story/${ARCHIVED_STORY_ROUND4}/"]`),
    ).toBeVisible();
    await expect(
      page.locator(`a[href="/by-story/${ARCHIVED_STORY_GRANDFATHERED}/"]`),
    ).toBeVisible();
  });

  test("a round-four quartet renders layout.json in the board slot", async ({
    page,
  }) => {
    await page.goto(dexByStoryPath(ARCHIVED_STORY_ROUND4));
    const body = page.locator("main.content");
    // The coordinate sidecar, labeled as what it is...
    await expect(body).toContainText("layout.json");
    await expect(body).toContainText("verdi.boardlayout/v1");
    // ...never the v0 frozen-board form a round-four archive doesn't have.
    await expect(body).not.toContainText("board.json");
    // The rest of the quartet is present and the spec permalink links out.
    await expect(body).toContainText("rollup.json");
    await expect(
      page.getByRole("heading", { name: "Deviation report" }),
    ).toBeVisible();
    await expect(
      page.locator(`a[href="/a/spec/${ARCHIVED_STORY_ROUND4}/"]`),
    ).toBeVisible();
  });

  test("a grandfathered v0 quartet keeps its frozen board.json, labeled", async ({
    page,
  }) => {
    await page.goto(dexByStoryPath(ARCHIVED_STORY_GRANDFATHERED));
    const body = page.locator("main.content");
    await expect(body).toContainText("board.json");
    await expect(body).toContainText("verdi.board/v1");
    await expect(body).toContainText("grandfathered");
  });
});

test.describe("V1-P8: ADR page links its exemption audit face", () => {
  test("the ADR permalink page links to the per-ADR exemption page", async ({
    page,
  }) => {
    await page.goto(`${DEX_BASE}/a/adr/${ADR_NAME}/`);
    const link = page.locator(
      `a[href="/a/adr/${ADR_NAME}/exemptions/"]`,
    );
    await expect(link).toBeVisible();
    await link.click();
    await expect(
      page.getByRole("heading", { name: /active exemption/i }),
    ).toBeVisible();
    expect(page.url()).toBe(dexAdrExemptionsPath(ADR_NAME));
  });
});
