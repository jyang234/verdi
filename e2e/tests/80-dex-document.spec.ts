import { test, expect } from "@playwright/test";
import { SHOWCASE, dexSpecPath } from "./fixtures";

// The docs site's Document view (spec/spec-documents ac-4): a spec page
// links to a reading of its objects rendered at the site's build commit,
// which says it is not authority before the content and offers the three
// Markdown files — spec.md, plan.md, tasks.md — written beside the page.
// FEATURE_SPEC is the showcase corpus's accepted feature spec on main.
test.describe("dex document view", () => {
  test("a spec page links to its document, which renders and offers the three files", async ({ page }) => {
    await page.goto(dexSpecPath(SHOWCASE.FEATURE_SPEC));
    const link = page.getByRole("link", { name: /read as a document/i });
    await expect(link).toBeVisible();
    await link.click();
    await expect(page).toHaveURL(new RegExp(`/a/spec/${SHOWCASE.FEATURE_SPEC}/document/$`));
    await expect(page.getByRole("heading", { level: 2, name: "Identity" })).toBeVisible();
    // The provenance note precedes the content (co-2: never authority); the
    // rendered document's own footer repeats the sentence, so target the note.
    const note = page.locator(".document-note");
    await expect(note).toBeVisible();
    await expect(note).toContainText("not authority");
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
    for (const file of ["spec.md", "plan.md", "tasks.md"]) {
      const fileLink = page.locator(".document-files").getByRole("link", { name: file });
      await expect(fileLink).toBeVisible();
      await expect(fileLink).toHaveAttribute("download", "");
      const res = await page.request.get(`${dexSpecPath(SHOWCASE.FEATURE_SPEC)}${file}`);
      expect(res.status()).toBe(200);
      const body = await res.text();
      expect(body.startsWith("# ")).toBe(true);
      expect(body).toContain("not authority");
    }
    // The way back: the artifact page keeps the verbatim body.
    await page.locator(".document-note").getByRole("link", { name: "artifact page" }).click();
    await expect(page).toHaveURL(new RegExp(`/a/spec/${SHOWCASE.FEATURE_SPEC}/$`));
  });
});
