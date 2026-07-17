import { test, expect } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";

// Vocabulary surfaces (spec/vocabulary-surfaces ac-2): a store carrying a
// vocab-rename model.yaml renders the model's display names in a REAL
// browser's DOM — the rename reaches the served chrome, not merely a
// Go-level render function returning the right string.
//
// The store is the hermetic vocab-rename fixture the control server
// spawns on demand (cmd/e2eharness/provision_vocab.go, following the
// empty-glance fixture's ADJ-40 convention): its .verdi/model.yaml is
// internal/model/testdata/vocab-rename.yaml reused verbatim
// (accepted-pending-build -> "Ready to build", feature -> "Initiative"),
// and its one spec, vocab-probe, is an accepted-pending-build feature. A
// SEPARATE store because a rename is store-wide by design — planting the
// model in the shared store would rename every bare-id label other suites
// pin.
//
// Ids never move: the chip's badge-accepted-pending-build CSS class, the
// class tag's case-class-tag--feature modifier, and the /board/spec/
// vocab-probe address all keep the bare id while only the visible words
// rename — asserted here against the same elements that carry the
// renamed text.

test.describe("vocabulary surfaces (spec/vocabulary-surfaces)", () => {
  test("a vocab-rename store's served chrome reads the model's display names, never the bare ids", async ({
    page,
  }) => {
    // Discover the isolated vocab-rename workbench (started lazily by the
    // control server; the URL is stable across calls).
    const res = await page.request.get(`${CONTROL_URL}/vocab-fixture`);
    expect(res.ok()).toBeTruthy();
    const vocabBase = (await res.text()).trim();

    // The home page: vocab-probe's status chip — the column chip the
    // glance's "In flight" bucket and the directory's grouped listing
    // both render — reads the RENAMED state label, and the bare id
    // appears nowhere as visible text on the page.
    await page.goto(vocabBase);
    const glanceChip = page
      .getByTestId("glance-entry-vocab-probe")
      .locator(".badge.badge-accepted-pending-build");
    await expect(glanceChip).toBeVisible();
    await expect(glanceChip).toHaveText("Ready to build");
    await expect(
      page.getByText("accepted-pending-build", { exact: true }),
    ).toHaveCount(0);

    // The served board: the case-file class tag reads the renamed class
    // word while its testid and CSS modifier keep the bare id, and the
    // board's address itself is the unrenamed spec ref.
    await page.goto(`${vocabBase}board/spec/vocab-probe`);
    const classTag = page.getByTestId("case-class-tag");
    await expect(classTag).toBeVisible();
    await expect(classTag).toHaveText("Initiative");
    await expect(classTag).toHaveClass(/case-class-tag--feature/);
    await expect(
      page.getByText("accepted-pending-build", { exact: true }),
    ).toHaveCount(0);
  });
});
