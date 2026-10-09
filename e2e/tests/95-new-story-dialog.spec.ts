import { test, expect, type Page } from "@playwright/test";
import { CONTROL_URL } from "./fixtures";

// The New story dialog (spec/new-story-dialog-v2; ledger SI-369): the
// branch preview, the inline grammar report and the gated Create (ac-1),
// and the criteria listed with their coverage, claimable (ac-2).
//
// The store is the isolated new-story fixture (cmd/e2eharness/
// newstoryfixture.go; SI-369 (10)): one sealed feature, escrow-analysis,
// whose ac-1 a declared stub covers, ac-2 only an accepted story implements
// (escrow-analysis-notice), and ac-3 nothing covers, under the plain
// vocabulary preset, which calls a story a "planned story". It starts once
// per harness run and is never reset, so no test here presses Create:
// only the file's last test, F6b's ac-4 test, does (SI-369 (17)). Each
// test passes with this file run alone (BL-98).

// The fixture's names, kept here as newstoryfixture.go asks: change them
// together.
const NEW_STORY_FIXTURE_URL = `${CONTROL_URL}/newstory-fixture`;
const FEATURE = "escrow-analysis";
const STORY_REF = "spec/escrow-analysis-notice";
const CRITERIA = ["ac-1", "ac-2", "ac-3"] as const;
// The plain preset's story word (initwizard.PlainPreset).
const STORY_WORD = "planned story";

// Names the server's grammar accepts and refuses: createnamepattern_test.go
// holds the same shipped pattern to ValidateSuccessorName on each of them,
// so the browser's verdicts below are the server's (SI-369 (19)(c)).
const SERVER_ACCEPTS = ["quote-pricing", "a", "v2-quote-3"];
const SERVER_REFUSES = [
  "Quote-pricing",
  "QUOTE",
  "quote#ac-1",
  "quote@abc1234",
  "quote/pricing",
  "quote--pricing",
  "-quote",
  "quote-",
  "",
  "café-quote",
  "quote pricing",
  "quote_pricing",
  "quote\n",
];

test.describe("new-story-dialog", () => {
  test("The branch preview, inline grammar report, and gated Create", async ({ page }) => {
    await openFeatureWall(page);
    await page.getByTestId("create-spec-btn").click();
    const dialog = page.locator("#create-dialog");
    await expect(dialog).toBeVisible();
    const name = page.getByTestId("create-name");
    const preview = dialog.locator("#create-branch-tab");
    const hint = page.getByTestId("create-name-hint");
    const create = page.getByTestId("create-ok");
    const status = page.getByTestId("create-status");

    // Opened: nothing named and nothing claimed, so Create waits and the
    // status line asks for the name first.
    await expect(preview).toHaveText("design/…");
    await expect(hint).toHaveText("spec/<name> · design/<name>");
    await expect(name).toHaveAttribute("aria-describedby", "create-name-hint");
    await expect(create).toBeDisabled();
    await expect(create).toHaveAttribute("aria-describedby", "create-status");
    await expect(status).toHaveAttribute("role", "status");
    await expect(status).toHaveText(`name the ${STORY_WORD} to continue`);

    // The grammar the browser compiles is the server's: data-pattern,
    // compiled here as the dialog's script compiles it, accepts exactly the
    // names the server accepts.
    const verdicts = await name.evaluate(
      (input, names) => {
        const grammar = new RegExp(input.getAttribute("data-pattern") ?? "(?!)");
        return names.map((n) => grammar.test(n));
      },
      [...SERVER_ACCEPTS, ...SERVER_REFUSES],
    );
    expect(verdicts).toEqual([...SERVER_ACCEPTS.map(() => true), ...SERVER_REFUSES.map(() => false)]);

    // A valid name: the preview shows the branch it will cut, and the hint
    // the ref and branch it becomes. Still no claim, so still gated.
    await name.fill("escrow-shortage-notice");
    await expect(preview).toHaveText("design/escrow-shortage-notice");
    await expect(hint).toHaveText("spec/escrow-shortage-notice · design/escrow-shortage-notice");
    await expect(name).not.toHaveAttribute("aria-invalid", "true");
    await expect(page.getByTestId("create-field-Title")).toHaveAttribute("placeholder", "Escrow Shortage Notice");
    await expect(create).toBeDisabled();
    await expect(status).toHaveText("claim at least one acceptance criterion");

    // A name that breaks the grammar is reported inline, quoting the
    // pattern, and never lowercased into a valid one: the preview goes back
    // to design/… and Create waits for the name again.
    await name.fill("Escrow-Shortage");
    await expect(hint).toHaveText('"Escrow-Shortage" is not kebab-case; the grammar is ^[a-z0-9]+(?:-[a-z0-9]+)*$');
    await expect(name).toHaveAttribute("aria-invalid", "true");
    await expect(preview).toHaveText("design/…");
    await expect(create).toBeDisabled();
    await expect(status).toHaveText(`name the ${STORY_WORD} to continue`);

    // A claim does not lift the gate while the name is broken.
    await page.getByTestId("create-ac-ac-3").check();
    await expect(create).toBeDisabled();
    await expect(status).toHaveText(`name the ${STORY_WORD} to continue`);

    // A valid name and a claim: Create is enabled, and the status line
    // names the branch, the claim, and the empty required fields, dropping
    // each once it is filled.
    await name.fill("escrow-shortage-notice");
    await expect(create).toBeEnabled();
    await expect(status).toHaveText(
      "cuts design/escrow-shortage-notice · claims ac-3 · Problem and Outcome are required",
    );
    await page.getByTestId("create-field-Problem").fill("a shortage reaches the borrower only through a changed payment");
    await expect(status).toHaveText("cuts design/escrow-shortage-notice · claims ac-3 · Outcome is required");
    await page.getByTestId("create-field-Outcome").fill("the borrower is told of a shortage before the payment changes");
    await expect(status).toHaveText("cuts design/escrow-shortage-notice · claims ac-3");
    await expect(create).toBeEnabled();

    // Unclaimed again: gated again, and the status line asks for a claim.
    await page.getByTestId("create-ac-ac-3").uncheck();
    await expect(create).toBeDisabled();
    await expect(status).toHaveText("claim at least one acceptance criterion");

    // The index's opener (wallnewstory.js) clicks the button and checks a
    // criterion without dispatching an event; the gate counts that claim
    // (SI-369 (9)). Escape keeps what was typed.
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
    await page.evaluate(() => {
      document.getElementById("create-spec-btn")!.click();
      const box = document.querySelector<HTMLInputElement>('#create-dialog [data-create-ac="ac-1"]')!;
      box.checked = true;
    });
    await expect(dialog).toBeVisible();
    await expect(create).toBeEnabled();
    await expect(status).toHaveText("cuts design/escrow-shortage-notice · claims ac-1");
    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });

  test("Criteria listed with their coverage, claimable", async ({ page }) => {
    await openFeatureWall(page);

    // The wall's chips, read before the dialog opens: ac-1 covered by its
    // stub, ac-2 and ac-3 by none (ac-2's coverage is its story's).
    const chips = new Map<string, string>();
    for (const ac of CRITERIA) {
      chips.set(ac, ((await page.getByTestId(`coverage-${ac}`).textContent()) ?? "").trim());
    }
    expect([...chips.values()]).toEqual(["covered by 1 stub", "no stub", "no stub"]);

    await page.getByTestId("create-spec-btn").click();
    const dialog = page.locator("#create-dialog");
    await expect(dialog).toBeVisible();

    // Every criterion the feature declares, in order, each with the wall
    // chip's own text — the stub half, from the same coverage function.
    await expect(dialog.locator("[data-create-ac]")).toHaveCount(CRITERIA.length);
    expect(
      await dialog.locator("[data-create-ac]").evaluateAll((boxes) => boxes.map((b) => b.getAttribute("data-create-ac"))),
    ).toEqual([...CRITERIA]);
    for (const ac of CRITERIA) {
      await expect(page.getByTestId(`create-coverage-${ac}`)).toHaveText(chips.get(ac)!);
    }

    // The story half, apart from it: ac-2 is claimed by the accepted story
    // (its ref in the title), ac-3 by nothing — the one criterion the
    // legend counts as unclaimed — and ac-1's stub needs no note.
    await expect(page.getByTestId("create-claims-ac-2")).toHaveText(`claimed by 1 ${STORY_WORD}`);
    await expect(page.getByTestId("create-claims-ac-2")).toHaveAttribute("title", STORY_REF);
    await expect(page.getByTestId("create-claims-ac-3")).toHaveText("unclaimed");
    await expect(page.getByTestId("create-claims-ac-1")).toHaveCount(0);
    await expect(page.getByTestId("create-acs-count")).toHaveText("1 AC unclaimed");
    await expect(criterionRow(page, "ac-3")).toHaveAttribute("data-state", "uncovered");
    await expect(criterionRow(page, "ac-2")).toHaveAttribute("data-state", "covered");
    await expect(criterionRow(page, "ac-1")).toHaveAttribute("data-state", "covered");

    // Claimable: the box checks and unchecks, and so does the row's own
    // text, the whole row being the box's label.
    const box = page.getByTestId("create-ac-ac-3");
    await expect(box).not.toBeChecked();
    await box.check();
    await expect(box).toBeChecked();
    await expect(dialog.locator("[data-create-ac]:checked")).toHaveCount(1);
    await box.uncheck();
    await expect(box).not.toBeChecked();
    await expect(dialog.locator("[data-create-ac]:checked")).toHaveCount(0);

    const ac2 = page.getByTestId("create-ac-ac-2");
    await criterionRow(page, "ac-2").locator(".create-ac-text").click();
    await expect(ac2).toBeChecked();
    await criterionRow(page, "ac-2").locator(".create-ac-text").click();
    await expect(ac2).not.toBeChecked();
    await expect(dialog.locator("[data-create-ac]:checked")).toHaveCount(0);

    await page.keyboard.press("Escape");
    await expect(dialog).toBeHidden();
  });
});

// openFeatureWall opens the fixture feature's sealed wall, discovering the
// isolated workbench through the control server (started on first use; the
// URL is stable thereafter).
async function openFeatureWall(page: Page): Promise<void> {
  const res = await page.request.get(NEW_STORY_FIXTURE_URL);
  expect(res.ok()).toBeTruthy();
  const base = (await res.text()).trim();
  await page.goto(`${base}board/spec/${FEATURE}`);
  await expect(page.getByTestId("create-spec-btn")).toBeVisible();
}

// criterionRow is the dialog row whose checkbox claims ac.
function criterionRow(page: Page, ac: string) {
  return page.locator("#create-dialog label.create-ac", { has: page.getByTestId(`create-ac-${ac}`) });
}
