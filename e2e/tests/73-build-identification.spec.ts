import { test, expect, type Page } from "@playwright/test";
import { SHOWCASE, SPEC_IMPORT_FIXTURE_URL, importPagePath } from "./fixtures";

// Build identification in the workbench footer (spec/uat-round-1 ac-1,
// co-4; closes UAT-003: "Two builds were live during the UAT ... nothing
// identified which one produced a result"). Every workbench page — the
// shared read-page shell, the v0 board, the v1 board — carries one footer
// element whose text is internal/buildinfo.Line(), the SAME string `verdi
// version` prints and `verdi serve` logs.
//
// The browser cannot read the binary's own build info, so the assertion is
// the line's closed grammar plus agreement: the harness builds ONE binary
// from this tree, so every page it serves — including the isolated import
// store's separate `verdi serve` process — must carry the identical string.
// From a linked-worktree build that string is the honest "verdi (devel)"
// (dc-8); the pattern accepts that and a stamped "verdi vX rev=... [modified]"
// alike, and nothing else. State assertions ride testids and text, never
// screenshots (recording stays off).

// buildinfo.Line()'s grammar: "verdi <version>[ rev=<≤12 hex>][ modified]",
// where <version> is a module version or Go's "(devel)" placeholder.
const BUILD_LINE = /^verdi (\(devel\)|v\S+)( rev=[0-9a-f]{1,12})?( modified)?$/;

const DRAFT_BOARD = () =>
  "/b/" + encodeURIComponent(SHOWCASE.SHOWCASE_DRAFT_BRANCH) + "/board/spec/" + SHOWCASE.SHOWCASE_DRAFT_SPEC;

async function footerLine(page: Page): Promise<string> {
  const footer = page.getByTestId("build-footer");
  await expect(footer).toHaveCount(1);
  const line = footer.getByTestId("build-identification");
  await expect(line).toBeVisible();
  const text = (await line.textContent()) ?? "";
  expect(text, "footer text must follow buildinfo.Line()'s grammar").toMatch(BUILD_LINE);
  return text;
}

test.describe("build identification footer", () => {
  test("home, the v0 board and the v1 board all carry the same build line in their footer", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveTitle(/Workbench/);
    const home = await footerLine(page);

    await page.goto("/board/STORY-1482");
    await expect(page.locator(".page-header h1")).toHaveText("Board: STORY-1482");
    expect(await footerLine(page)).toBe(home);

    await page.goto(DRAFT_BOARD());
    await expect(page.getByTestId("board")).toBeVisible();
    expect(await footerLine(page)).toBe(home);
  });

  test("the isolated import store's serve carries the same build line", async ({ page }) => {
    // The import fixture is a SEPARATE `verdi serve` process of the same
    // built binary (cmd/e2eharness/specimportfixture.go); its read-page
    // shell must identify the same build as the shared store's pages.
    await page.goto("/");
    const shared = await footerLine(page);

    const res = await page.request.get(SPEC_IMPORT_FIXTURE_URL);
    expect(res.ok(), await res.text()).toBe(true);
    const base = (await res.text()).trim();
    await page.goto(base + importPagePath().replace(/^\//, ""));
    await expect(page.getByTestId("import-form")).toBeVisible();
    expect(await footerLine(page)).toBe(shared);
  });
});
