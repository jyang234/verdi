import { test, expect, type Page } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";
import { drawYarn, expectAutosaved } from "./helpers";

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
//
// The store's second spec, vocab-draft (a DRAFT feature on its own
// design branch, with the serving checkout left on that branch), serves
// its board in AUTHORING mode — the half that proves the rename reaches
// CLIENT-side JS prose (judged-client-js-prose-has-no-browser-proof):
// boardspec.js builds the sticky type menu's story/spike labels
// (STICKY_TYPES) and the proto-yarn dialog/refusal copy from the
// embedded words payload, so only a real browser executing that JS can
// prove the seam. The fixture model renames story -> "Workstream" and
// spike -> "Timebox" (the L-M13 pseudo-class carve), and the identity
// layer stays bare throughout: data-sticky-type values, the sticky's
// data-annotation-type, and the api() payloads all keep the enum ids.

test.describe("vocabulary surfaces (spec/vocabulary-surfaces)", () => {
  test("a vocab-rename store's served chrome reads the model's display names, never the bare ids", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);

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

  test("the sticky type menu's story/spike labels speak the renamed words (STICKY_TYPES words payload)", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);
    await page.goto(`${vocabBase}board/spec/vocab-draft`);
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    // The inline type control is built ENTIRELY by boardspec.js
    // (startStickyEditor over STICKY_TYPES): its story/spike labels are
    // classWordCap over the embedded words payload — the client-side
    // words seam, executed by a real browser here.
    await page.getByRole("button", { name: "Add sticky" }).click();
    const picker = page.locator(".sticky-draft .sticky-type-picker");
    await expect(picker).toBeVisible();
    const storyBtn = picker.locator('[data-sticky-type="story"]');
    const spikeBtn = picker.locator('[data-sticky-type="spike"]');
    await expect(storyBtn).toHaveText("Workstream");
    await expect(spikeBtn).toHaveText("Timebox");
    // The bare ids never surface as menu text (the value attribute — the
    // identity layer the server receives — is where they live).
    await expect(picker).not.toContainText(/\b(story|spike)\b/i);
    // The non-class taxonomy labels are not class words and stay verbatim.
    await expect(
      picker.locator('[data-sticky-type="question"]'),
    ).toHaveText("Question");

    // Discard the draft: nothing was written by opening the menu.
    await page.keyboard.press("Escape");
    await expect(page.locator(".sticky-draft")).toHaveCount(0);
  });

  test("a misaimed spike thread's refusal copy speaks the renamed words, never the bare ids", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);
    await page.goto(`${vocabBase}board/spec/vocab-draft`);
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    // Create a spike proto-sticky through the renamed control. The type
    // VALUE sent to the server is the bare enum id (data-sticky-type),
    // and the committed sticky's data-annotation-type stays bare too —
    // ids never move while the visible words rename.
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    await expect(draft).toBeVisible();
    await draft.locator('[data-sticky-type="spike"]').click();
    const editor = draft.getByRole("textbox", { name: "Sticky text" });
    await editor.fill("misaimed timebox probe");
    await editor.blur();
    await expectAutosaved(page);
    const sticky = page
      .locator('.sticky[data-annotation-type="spike"]')
      .filter({ hasText: "misaimed timebox probe" })
      .first();
    await expect(sticky).toBeVisible();

    // Drop its attribution yarn on the acceptance criterion — the wrong
    // target for a spike thread, so routeProtoYarn answers with the
    // picker's plain-language refusal. That copy is assembled in
    // boardspec.js from classWord("spike")/classWord("story"): the
    // renamed words must appear, the bare ids must not.
    const stickyId = await sticky.getAttribute("data-id");
    expect(stickyId).not.toBeNull();
    await drawYarn(page, stickyId!, page.getByTestId("card-ac-1"));
    const refusal = page.getByTestId("proto-yarn-refusal");
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText(
      "A Timebox sticky's thread claims an answer",
    );
    await expect(refusal).toContainText(
      "it wants to be a Workstream sticky instead",
    );
    await expect(refusal).not.toContainText(/\b(story|spike)\b/i);
    await page.keyboard.press("Escape");

    // The refusal minted nothing: no thread hangs from the sticky.
    await expect(
      page.locator(
        `.yarn-chip--annotation[data-from="${stickyId}"], .yarn-chip--annotation[data-to="${stickyId}"]`,
      ),
    ).toHaveCount(0);
  });
});

// vocabFixtureBase discovers the isolated vocab-rename workbench through
// the control server (started lazily; the URL is stable across calls).
async function vocabFixtureBase(page: Page): Promise<string> {
  const res = await page.request.get(`${CONTROL_URL}/vocab-fixture`);
  expect(res.ok()).toBeTruthy();
  return (await res.text()).trim();
}
