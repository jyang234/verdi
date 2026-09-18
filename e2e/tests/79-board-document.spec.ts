import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath, branchBoardPath } from "./fixtures";

// The board's Document tab (spec/spec-documents ac-4): a reading of the
// wall's spec rendered through the shared loader beside the board, with a
// conditional /snapshot refresh (Wave 6 rules), a Markdown download, and a
// copy control. READONLY_SPEC (stale-decline) is the sealed accepted wall:
// its working-tree bytes ARE the accepted bytes, so its document is the
// accepted reading. DESIGN_SPEC lives only on DESIGN_BRANCH, so its
// document renders as proposed. Never authority (co-2); GET-only.

const docPath = (spec: string) => `${boardPath(spec)}/document`;

test.describe("board document tab", () => {
  test("the sealed wall offers a Document tab that renders the accepted reading", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await page.getByTestId("board-tab-document").click();
    await expect(page).toHaveURL(new RegExp(`${docPath(SHOWCASE.READONLY_SPEC)}$`));
    const region = page.getByTestId("document-region");
    await expect(region).toContainText("Identity");
    await expect(region).toContainText("not authority");
    await expect(region).not.toContainText("Proposed, not accepted");
    // The document's own h1 is the page's one h1.
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    // The kind switch is a plain link: the plan document has Decisions and
    // no Problem section (the spec kind's own section).
    await page.getByTestId("document-kind-plan").click();
    await expect(page).toHaveURL(new RegExp(`${docPath(SHOWCASE.READONLY_SPEC)}\\?kind=plan$`));
    await expect(region).toContainText("Decisions");
    await expect(region).not.toContainText("Problem");
    await expect(page.getByTestId("document-kind-plan")).toHaveAttribute("aria-current", "page");
    // The way back is the Board tab.
    await page.getByTestId("document-tab-board").click();
    await expect(page).toHaveURL(new RegExp(`${boardPath(SHOWCASE.READONLY_SPEC)}$`));
  });

  test("a design-branch draft renders as proposed under the branch mount", async ({ page }) => {
    await page.goto(`${branchBoardPath(SHOWCASE.DESIGN_BRANCH, SHOWCASE.DESIGN_SPEC)}/document`);
    const region = page.getByTestId("document-region");
    await expect(region).toContainText("Proposed, not accepted");
    await expect(region).toContainText("not authority");
    // Every sibling link keeps the branch mount's encoded segment.
    const branchBase = branchBoardPath(SHOWCASE.DESIGN_BRANCH, SHOWCASE.DESIGN_SPEC);
    await expect(page.getByTestId("document-tab-board")).toHaveAttribute("href", branchBase);
    await expect(page.getByTestId("document-kind-tasks")).toHaveAttribute("href", `${branchBase}/document?kind=tasks`);
    await expect(page.getByTestId("document-download")).toHaveAttribute("href", `${branchBase}/document?format=md&kind=spec`);
  });

  test("snapshot answers 304 for an unchanged revision token", async ({ page }) => {
    const first = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`);
    expect(first.status()).toBe(200);
    const etag = first.headers()["etag"];
    expect(etag).toBeTruthy();
    const second = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`, {
      headers: { "If-None-Match": etag },
    });
    expect(second.status()).toBe(304);
    // A different kind is a different revision, never a 304 to the spec's token.
    const plan = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot?kind=plan`, {
      headers: { "If-None-Match": etag },
    });
    expect(plan.status()).toBe(200);
  });

  test("hidden tabs pause polling; visibility resumes with one immediate refresh", async ({ page }) => {
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    // Count snapshot fetches from inside the page AND simulate a hidden
    // tab in the same evaluate, so no 2 s tick can land between the two
    // (the poll loop reads document.hidden live).
    await page.evaluate(() => {
      const w = window as unknown as { __snapCount: number; fetch: typeof fetch };
      w.__snapCount = 0;
      const real = w.fetch.bind(window);
      w.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
        if (String(input).includes("/document/snapshot")) w.__snapCount++;
        return real(input, init);
      }) as typeof fetch;
      Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await page.waitForTimeout(4_500);
    const whileHidden = await page.evaluate(
      () => (window as unknown as { __snapCount: number }).__snapCount,
    );
    expect(whileHidden).toBe(0);
    // Visible again: the immediate conditional refresh fires.
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await expect
      .poll(
        () => page.evaluate(() => (window as unknown as { __snapCount: number }).__snapCount),
        { timeout: 3_000 },
      )
      .toBeGreaterThan(0);
  });

  test("the manual Refresh control reports an unchanged document and leaves it untouched", async ({ page }) => {
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    const region = page.getByTestId("document-region");
    const before = await region.getAttribute("data-revision");
    expect(before).toBeTruthy();
    await page.getByTestId("document-refresh").focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("status")).toHaveText("Up to date");
    await expect(region).toHaveAttribute("data-revision", before as string);
  });

  test("copy and download hand over the same Markdown the snapshot carries", async ({ page, context }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    const snap = await (await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`)).json();
    expect(snap.markdown.startsWith("# ")).toBe(true);
    await page.getByTestId("document-copy").click();
    await expect(page.getByRole("status")).toHaveText("Copied Markdown");
    const clip = await page.evaluate(() => navigator.clipboard.readText());
    expect(clip).toBe(snap.markdown);
    const dl = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}?format=md`);
    expect(dl.status()).toBe(200);
    expect(dl.headers()["content-type"]).toContain("text/markdown");
    expect(dl.headers()["content-disposition"]).toContain(`${SHOWCASE.READONLY_SPEC}-spec.md`);
    expect(await dl.text()).toBe(snap.markdown);
    await expect(page.getByTestId("document-download")).toHaveAttribute("download", `${SHOWCASE.READONLY_SPEC}-spec.md`);
  });

  test("keyboard reaches every control", async ({ page }) => {
    await page.goto(docPath(SHOWCASE.READONLY_SPEC));
    const ids = ["document-tab-board", "document-kind-plan", "document-kind-tasks", "document-refresh", "document-copy", "document-download"];
    const seen = new Set<string>();
    for (let i = 0; i < 20 && seen.size < ids.length; i++) {
      await page.keyboard.press("Tab");
      const id = await page.evaluate(() => document.activeElement?.getAttribute("data-testid"));
      if (id && ids.includes(id)) seen.add(id);
    }
    expect([...seen].sort()).toEqual([...ids].sort());
  });

  test("the document routes are GET-only and refuse an unknown kind", async ({ page }) => {
    const post = await page.request.post(docPath(SHOWCASE.READONLY_SPEC));
    expect(post.status()).toBe(405);
    const postSnap = await page.request.post(`${docPath(SHOWCASE.READONLY_SPEC)}/snapshot`);
    expect(postSnap.status()).toBe(405);
    const bad = await page.request.get(`${docPath(SHOWCASE.READONLY_SPEC)}?kind=chapter`);
    expect(bad.status()).toBe(400);
  });
});
