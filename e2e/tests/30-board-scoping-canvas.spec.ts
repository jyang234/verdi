import { test, expect, type Page } from "@playwright/test";
import { SHOWCASE, boardPath, stubCardTestId, coverageChipTestId } from "./fixtures";
import { addSticky, drawYarn, expectAutosaved } from "./helpers";

// The scoping canvas (spec/scoping-canvas ac-2/ac-3/ac-4/ac-5, dc-1/
// dc-5/dc-6): story and spike proto-stickies are the stub authoring
// surface — they park handwritten in the stubs band, their yarn to
// acceptance criteria / open questions is the coverage / resolution
// claim, and graduation typesets them in place as declared stub cards.
// Every journey here runs on the harness's feature fixture (SHOWCASE.DESIGN_SPEC,
// authoring mode on its design branch); the shared store carries each
// test's graduated stubs forward, so the specs run strictly in order.

const confirmDialog = (page: Page) => page.locator("#edge-confirm");
const confirmOk = (page: Page) => page.locator("#edge-confirm-ok");
const confirmCancel = (page: Page) => page.locator("#edge-confirm-cancel");

// Draw attribution yarn from a proto-sticky's pushpin and confirm the
// thread's stated meaning (the calm confirmation replaces the picker:
// the endpoint pair has exactly one reading).
async function drawAttribution(
  page: Page,
  sticky: import("@playwright/test").Locator,
  targetId: string,
  meaning: RegExp,
) {
  const stickyId = await sticky.getAttribute("data-id");
  expect(stickyId).not.toBeNull();
  await drawYarn(page, stickyId!, page.getByTestId(`card-${targetId}`));
  await expect(confirmDialog(page)).toBeVisible();
  await expect(confirmDialog(page)).toContainText(meaning);
  await confirmOk(page).click();
  await expectAutosaved(page);
  // The thread projects as an untyped relates chip whose endpoint is
  // the sticky's own annotation id (round 5.4).
  await expect(
    page.locator(
      `.yarn-chip--annotation[data-from="${stickyId}"][data-to="${targetId}"], ` +
        `.yarn-chip--annotation[data-from="${targetId}"][data-to="${stickyId}"]`,
    ),
  ).toHaveCount(1);
  return stickyId!;
}

// Graduate a proto-sticky through the register ceremony's confirm.
async function graduateProto(page: Page, sticky: import("@playwright/test").Locator) {
  await sticky.locator('[data-graduate="stub"]').click();
  await expect(confirmDialog(page)).toBeVisible();
  await expect(confirmDialog(page)).toContainText(/typesets this sticky in place/i);
  await confirmOk(page).click();
  await expectAutosaved(page);
}

test.describe("scoping canvas: the feature wall authors its stubs", () => {
  test("story and spike are offered exactly where the server accepts them", async ({
    page,
  }) => {
    // The feature wall's draft control offers all six types.
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await page.getByRole("button", { name: "Add sticky" }).click();
    const draft = page.locator(".sticky-draft");
    await expect(draft).toBeVisible();
    for (const label of ["Story", "Spike", "Comment", "Question"]) {
      await expect(draft.getByRole("button", { name: label })).toBeVisible();
    }
    await page.keyboard.press("Escape");
    await expect(draft).toHaveCount(0);

    // A story-class wall never offers the proto types (the same gate
    // the server enforces — the menu mirrors the refusal).
    await page.goto(boardPath(SHOWCASE.EMPTY_SPEC));
    await page.getByRole("button", { name: "Add sticky" }).click();
    const storyDraft = page.locator(".sticky-draft");
    await expect(storyDraft).toBeVisible();
    await expect(storyDraft.getByRole("button", { name: "Comment" })).toBeVisible();
    await expect(storyDraft.getByRole("button", { name: "Story" })).toHaveCount(0);
    await expect(storyDraft.getByRole("button", { name: "Spike" })).toHaveCount(0);
    await page.keyboard.press("Escape");

    // Non-authoring modes have no sticky control at all — the sealed
    // record's one live affordance is instantiate, nothing else writes.
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    await expect(page.getByRole("button", { name: "Add sticky" })).toHaveCount(0);

    // And a draft wall offers no instantiate: only an accepted record
    // cuts story branches.
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.locator("[data-instantiate]")).toHaveCount(0);
  });

  test("story sticky → coverage yarn → graduation typesets the stub in place", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    // Before any stub, every AC wears the quietly-insistent chip.
    const ac3Chip = page.getByTestId(coverageChipTestId(SHOWCASE.AC_IDS[2]));
    await expect(ac3Chip).toHaveText("no stub");
    await expect(ac3Chip).toHaveAttribute("data-coverage", "0");

    // The story sticky parks handwritten in the stubs band (dc-6): its
    // left edge is the band's x — the same lane the stub label tapes.
    const sticky = await addSticky(page, "audit decline notice log", "story");
    await expect(sticky).toHaveAttribute("data-annotation-type", "story");
    const stubLabelX = await page
      .getByTestId("zone-label-stub")
      .evaluate((el) => (el as HTMLElement).style.left);
    await expect(sticky).toHaveCSS("left", stubLabelX);

    // Yarn to ac-3: one reading, no picker — a calm confirmation of the
    // coverage claim, minted as an untyped relates thread.
    await drawAttribution(page, sticky, SHOWCASE.AC_IDS[2], /coverage claim/i);

    // Graduate: the sticky typesets into its stub card in the same band.
    const stickyId = await sticky.getAttribute("data-id");
    await graduateProto(page, sticky);
    const stub = page.getByTestId(stubCardTestId("audit-decline-notice-log"));
    await expect(stub).toBeVisible();
    await expect(stub.locator(".stub-tab")).toHaveText("audit-decline-notice-log");
    await expect(stub.locator(".card-kind-label")).toHaveText("story stub");
    // AMENDED (scoping yarn): the coverage attribution is no longer a
    // chip ON the card — it hangs as scoping yarn tying the stub card
    // to the AC it covers (owner directive; 32-board-scoping-yarn
    // proves the full thread contract).
    await expect(
      page.locator(
        `.yarn-chip--scoping[data-edge-type="covers"]` +
          `[data-from="stub:audit-decline-notice-log"][data-to="${SHOWCASE.AC_IDS[2]}"]`,
      ),
    ).toHaveCount(1);
    // The handwriting is gone — the record took its place — and so is
    // the consumed attribution thread.
    await expect(page.getByTestId(`sticky-${stickyId}`)).toHaveCount(0);
    await expect(
      page.locator(`.yarn-chip--annotation[data-from="${stickyId}"]`),
    ).toHaveCount(0);

    // The coverage chip updated: computed, never declared (ac-4).
    await expect(ac3Chip).toHaveText("covered by 1 stub");
    await expect(ac3Chip).toHaveAttribute("data-coverage", "1");

    // Durable across a fresh projection.
    await page.reload();
    await expect(
      page.getByTestId(stubCardTestId("audit-decline-notice-log")),
    ).toBeVisible();
    await expect(page.getByTestId(coverageChipTestId(SHOWCASE.AC_IDS[2]))).toHaveText(
      "covered by 1 stub",
    );
  });

  test("zero-yarn graduation is refused legibly and the sticky survives", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    const sticky = await addSticky(page, "orphan claim", "story");
    await sticky.locator('[data-graduate="stub"]').click();
    await confirmOk(page).click();

    // The server's refusal surfaces in the dialog's own voice — a
    // designed state, not a raw error toast.
    await expect(confirmDialog(page)).toBeVisible();
    await expect(confirmDialog(page)).toContainText("Not yet a stub");
    await expect(confirmDialog(page)).toContainText(/draw coverage yarn first/);
    await expect(confirmOk(page)).toBeHidden();
    await confirmCancel(page).click();
    await expect(confirmDialog(page)).toBeHidden();

    // Nothing graduated, nothing died: the sticky is still parked.
    const stickyId = await sticky.getAttribute("data-id");
    await expect(page.getByTestId(`sticky-${stickyId}`)).toBeVisible();

    // Tidy the wall for the next journey (scratch dies without ceremony).
    await sticky.locator('[data-delete="sticky"]').click();
    await expectAutosaved(page);
    await expect(page.getByTestId(`sticky-${stickyId}`)).toHaveCount(0);
  });

  test("spike stickies → resolution yarn → spike stubs; two claims raise the smell", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));

    // First spike: no smell at one claim (one spike answering questions
    // is the norm).
    const first = await addSticky(page, "probe legal wording", "spike");
    await expect(first).toHaveAttribute("data-annotation-type", "spike");
    await drawAttribution(page, first, SHOWCASE.OQ_ID, /attribution/i);
    await graduateProto(page, first);
    const firstStub = page.getByTestId(stubCardTestId("probe-legal-wording"));
    await expect(firstStub).toBeVisible();
    await expect(firstStub).toHaveClass(/stubcard--spike/);
    await expect(firstStub.locator(".card-kind-label")).toHaveText("spike stub");
    // AMENDED (scoping yarn): the resolution attribution hangs as
    // scoping yarn to the open question, not as a chip on the card.
    await expect(
      page.locator(
        `.yarn-chip--scoping[data-edge-type="resolves"]` +
          `[data-from="stub:probe-legal-wording"][data-to="${SHOWCASE.OQ_ID}"]`,
      ),
    ).toHaveCount(1);
    await expect(page.getByTestId(`oq-claims-${SHOWCASE.OQ_ID}`)).toHaveCount(0);

    // Second spike claiming the SAME question: the multi-claim smell
    // appears — a norm-shaped observation, never an error (ac-5).
    const second = await addSticky(page, "probe policy precedent", "spike");
    await drawAttribution(page, second, SHOWCASE.OQ_ID, /attribution/i);
    await graduateProto(page, second);
    await expect(
      page.getByTestId(stubCardTestId("probe-policy-precedent")),
    ).toBeVisible();
    const smell = page.getByTestId(`oq-claims-${SHOWCASE.OQ_ID}`);
    await expect(smell).toHaveText("claimed by 2 spikes");
    await expect(smell).toHaveAttribute("data-claims", "2");
  });

  test("illegal pairs get the picker's plain-language refusal", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));

    // A story sticky's thread has one meaning: coverage. An open
    // question is the spike's target, and the refusal says so.
    const story = await addSticky(page, "misaimed story", "story");
    const storyId = await story.getAttribute("data-id");
    await drawYarn(page, storyId!, page.getByTestId(`card-${SHOWCASE.OQ_ID}`));
    const refusal = page.getByTestId("proto-yarn-refusal");
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText(/spike sticky instead/);
    await page.keyboard.press("Escape");

    // Neither proto type ties to anything else — a decision card here
    // (co-1 was legitimately trashed off the wall by the earlier trash
    // suite; dc-2 is the wall's stable bystander).
    await drawYarn(page, storyId!, page.getByTestId(`card-${SHOWCASE.DECISION_PLAIN}`));
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText(/one meaning/);
    await page.keyboard.press("Escape");

    // A spike sticky refuses acceptance criteria symmetrically.
    const spike = await addSticky(page, "misaimed spike", "spike");
    const spikeId = await spike.getAttribute("data-id");
    await drawYarn(page, spikeId!, page.getByTestId(`card-${SHOWCASE.AC_IDS[0]}`));
    await expect(refusal).toBeVisible();
    await expect(refusal).toContainText(/story sticky instead/);
    await page.keyboard.press("Escape");

    // No thread was minted by any refusal.
    for (const id of [storyId, spikeId]) {
      await expect(
        page.locator(
          `.yarn-chip--annotation[data-from="${id}"], .yarn-chip--annotation[data-to="${id}"]`,
        ),
      ).toHaveCount(0);
    }

    // Tidy the wall.
    for (const id of [storyId, spikeId]) {
      await page
        .getByTestId(`sticky-${id}`)
        .locator('[data-delete="sticky"]')
        .click();
      await expectAutosaved(page);
    }
  });
});
