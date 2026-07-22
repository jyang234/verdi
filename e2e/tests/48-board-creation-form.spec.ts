import { test, expect, type Page } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";

// The board creation form (spec/creation-form ac-3, implementing
// spec/creation-surfaces#ac-2): a sealed accepted feature wall grows a
// creation affordance whose form fields are GENERATED from the story
// class's own resolved template placeholders — here the vocab-rename
// store's own .verdi/templates/custom-story.md override, so the browser
// proof covers the L-M12 override property end to end.
//
// The store is the hermetic vocab-rename fixture (cmd/e2eharness/
// provision_vocab.go): story -> "Workstream", accept -> "Sign off",
// feature -> "Initiative". The form's labels must speak those display
// words — the affordance, the dialog, and the receipt (whose verb word
// routes through DisplayVerb, spec/creation-surfaces ac-5's discipline)
// — while the identity layer (the design/<name> branch, data-* values,
// testids) stays bare.
//
// The landed artifact is verified OUTSIDE the browser's own claims,
// through the control server's read-only /vocab-fixture/show window
// (`git show <branch>:<path>` against the fixture's real repository):
// correct class, the chosen implements edge, submitted statements
// verbatim with no TODO residue, unfilled fields keeping their disclosed
// placeholders, and the override template's own body shape.

test.describe("board creation form (spec/creation-form)", () => {
  test("the sealed wall's creation affordance and dialog speak the store's display words", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);
    await page.goto(`${vocabBase}board/spec/vocab-probe`);

    // The affordance speaks the renamed class word; its testid stays bare.
    const btn = page.getByTestId("create-spec-btn");
    await expect(btn).toBeVisible();
    await expect(btn).toHaveText(/New Workstream/);

    await btn.click();
    const dialog = page.locator("#create-dialog");
    await expect(dialog).toBeVisible();

    // The dialog's heading and action speak the display word...
    await expect(dialog.locator("h2")).toHaveText("New Workstream");
    await expect(page.getByTestId("create-ok")).toHaveText("Create Workstream");
    // ...the template-generated fields are present (custom-story.md's
    // input/statement placeholders)...
    for (const testid of [
      "create-name",
      "create-field-Title",
      "create-field-Problem",
      "create-field-Outcome",
      "create-field-StoryRef",
    ]) {
      await expect(page.getByTestId(testid)).toBeAttached();
    }
    // ...the wall's declared AC is offered as a coverage choice...
    await expect(page.getByTestId("create-ac-ac-1")).toBeAttached();
    // ...and no bare class id renders as visible dialog text (the
    // identity layer lives in attributes, never labels).
    await expect(dialog).not.toContainText(/\b(story|spike|feature)\b/);

    // The branch tab is the identity being minted: it live-updates from
    // the name field, in the machine's own mono voice.
    await page.getByTestId("create-name").fill("quote-pricing");
    await expect(dialog.locator("#create-branch-tab")).toHaveText(
      "design/quote-pricing",
    );

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("submitting the form lands a TODO-free story on its own design branch", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);
    await page.goto(`${vocabBase}board/spec/vocab-probe`);
    await page.getByTestId("create-spec-btn").click();

    await page.getByTestId("create-name").fill("priced-quote-flow");
    await page
      .getByTestId("create-field-Problem")
      .fill("Quotes are hand-priced today");
    await page
      .getByTestId("create-field-Outcome")
      .fill("Quotes price themselves from the declared model");
    await page.getByTestId("create-ac-ac-1").check();
    await page.getByTestId("create-ok").click();

    // The receipt names the branch and speaks display words: the class
    // word and the DisplayVerb-routed verb word ("Sign off", never a
    // hand-written bare "accept").
    const receipt = page.locator("#edge-confirm");
    await expect(receipt).toBeVisible();
    await expect(receipt.locator("h2")).toHaveText("Workstream created");
    await expect(receipt).toContainText("design/priced-quote-flow");
    await expect(receipt).toContainText("Sign off");
    await expect(receipt).not.toContainText(/\baccept\b/);
    await page.keyboard.press("Escape");

    // The landed artifact, read from the fixture repository itself —
    // the branch is real, the spec is the override template's render.
    const res = await page.request.get(
      `${CONTROL_URL}/vocab-fixture/show?ref=design/priced-quote-flow&path=.verdi/specs/active/priced-quote-flow/spec.md`,
    );
    expect(res.ok()).toBeTruthy();
    const spec = await res.text();
    expect(spec).toContain("id: spec/priced-quote-flow");
    expect(spec).toContain("class: story");
    // Submitted statements land verbatim — TODO-free where filled.
    expect(spec).toContain(
      'problem: { text: "Quotes are hand-priced today", anchor: problem }',
    );
    expect(spec).toContain(
      'outcome: { text: "Quotes price themselves from the declared model", anchor: outcome }',
    );
    expect(spec).not.toContain("TODO: replace with the real problem");
    expect(spec).not.toContain("TODO: replace with the real outcome");
    // The unfilled fields keep their disclosed placeholder defaults.
    expect(spec).toContain("story: todo:REPLACE-ME");
    expect(spec).toContain('title: "Priced Quote Flow"');
    expect(spec).toContain("owners: [unassigned]");
    // The chosen AC became a real implements edge.
    expect(spec).toContain(
      '- { type: implements, ref: "spec/vocab-probe#ac-1" }',
    );
    // The override template's own shape reached the created spec (the
    // L-M12 property, in a browser).
    expect(spec).toContain("## Delivery Notes");
  });

  test("a filled tracker ref is never called a placeholder by the receipt (state-resolved copy)", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);
    await page.goto(`${vocabBase}board/spec/vocab-probe`);
    await page.getByTestId("create-spec-btn").click();

    await page.getByTestId("create-name").fill("tracked-quote-flow");
    await page
      .getByTestId("create-field-Problem")
      .fill("Tracker refs go stale by hand");
    await page
      .getByTestId("create-field-Outcome")
      .fill("The tracker ref is real from birth");
    await page.getByTestId("create-field-StoryRef").fill("jira:QUOTE-9");
    await page.getByTestId("create-ac-ac-1").check();
    await page.getByTestId("create-ok").click();

    // The receipt asserts only what is true of the landed artifact
    // (judged-create-receipt-storyref-claim): the tracker ref was
    // filled, so the placeholder sentence must not appear.
    const receipt = page.locator("#edge-confirm");
    await expect(receipt).toBeVisible();
    await expect(receipt).toContainText("design/tracked-quote-flow");
    await expect(receipt).not.toContainText("todo:REPLACE-ME");
    await expect(receipt).not.toContainText("placeholder");
    await page.keyboard.press("Escape");

    // And the landed spec really carries the submitted ref.
    const res = await page.request.get(
      `${CONTROL_URL}/vocab-fixture/show?ref=design/tracked-quote-flow&path=.verdi/specs/active/tracked-quote-flow/spec.md`,
    );
    expect(res.ok()).toBeTruthy();
    const spec = await res.text();
    expect(spec).toContain("story: jira:QUOTE-9");
    expect(spec).not.toContain("todo:REPLACE-ME");
  });

  test("empty statement fields refuse visibly and nothing is created", async ({
    page,
  }) => {
    const vocabBase = await vocabFixtureBase(page);
    await page.goto(`${vocabBase}board/spec/vocab-probe`);
    await page.getByTestId("create-spec-btn").click();

    await page.getByTestId("create-name").fill("empty-probe");
    await page.getByTestId("create-ac-ac-1").check();
    await page.getByTestId("create-ok").click();

    // The refusal is visible, names the missing statements, and the form
    // (with everything typed) stays open — no silent default, nothing
    // lost.
    const error = page.getByTestId("create-error");
    await expect(error).toBeVisible();
    await expect(error).toContainText("Problem");
    await expect(error).toContainText("Outcome");
    await expect(page.locator("#create-dialog")).toBeVisible();

    // Nothing landed: the branch does not exist in the fixture repo.
    const res = await page.request.get(
      `${CONTROL_URL}/vocab-fixture/show?ref=design/empty-probe&path=.verdi/specs/active/empty-probe/spec.md`,
    );
    expect(res.ok()).toBeFalsy();
  });
});

// vocabFixtureBase discovers the isolated vocab-rename workbench through
// the control server (started lazily; the URL is stable across calls).
async function vocabFixtureBase(page: Page): Promise<string> {
  const res = await page.request.get(`${CONTROL_URL}/vocab-fixture`);
  expect(res.ok()).toBeTruthy();
  return (await res.text()).trim();
}
