import { test, expect, type Page } from "@playwright/test";
import path from "node:path";
import { SPEC_IMPORT_FIXTURE_URL, SPEC_IMPORT_FILES, importPagePath, importSourceId } from "./fixtures";

// Selection-driven mapping in the import dialog (spec/uat-round-1 ac-8,
// closing UAT-006: nobody maps by typing byte offsets, and three of four
// UAT sources stayed at zero mapped bytes). Each selected source is
// rendered read-only beneath its row; selecting a passage and choosing
// "Map selection" creates a source-backed mapping whose start/end are
// UTF-8 BYTE offsets into that source's exact bytes, with the target
// chosen from a picker (statements, existing objects, or a new object of
// any of the four kinds). The server is the oracle for correctness: the
// preview's field card must carry the exact phrase and the coverage table
// must count exactly its bytes as mapped. Every source here carries
// multi-byte characters BEFORE the selected phrase, so a JS-string-index
// offset would be wrong and refused (or would map the wrong bytes).
//
// Runs against the isolated clean-main import store the 72 spec uses
// (cmd/e2eharness/specimportfixture.go). State assertions ride testids,
// data attributes and text — never screenshots (recording stays off).

const PLAN_NAME = "widget-plan.md";
const PROBLEM_PHRASE = "Operators can’t bring “existing” plans onto a board — they retype them, and the naïve copy loses the wording.";
const CONSTRAINT_PHRASE = "Every import must preserve the source bytes exactly; nothing is reworded.";
const PLAN_TEXT = [
  "# Widget plan — working notes",
  "",
  "## Problem",
  "",
  PROBLEM_PHRASE,
  "",
  "## Constraints",
  "",
  CONSTRAINT_PHRASE,
  "",
].join("\n");
const PLAN_BYTES = Buffer.from(PLAN_TEXT, "utf8");

const NOTES_NAME = "notes.md";
// The clef is an astral character (two UTF-16 units, four UTF-8 bytes):
// a selection boundary inside it is not a character boundary.
const NOTES_TEXT = "Ünrelated notes — a second source, 𝄞 clef.\n";

// byteRange is the server's coordinate system: UTF-8 byte offsets into the
// exact bytes, half-open.
function byteRange(phrase: string): { start: number; end: number } {
  const start = PLAN_BYTES.indexOf(Buffer.from(phrase, "utf8"));
  expect(start).toBeGreaterThan(0);
  return { start, end: start + Buffer.byteLength(phrase, "utf8") };
}

async function importBase(page: Page): Promise<string> {
  const res = await page.request.get(SPEC_IMPORT_FIXTURE_URL);
  expect(res.ok(), await res.text()).toBe(true);
  const url = (await res.text()).trim();
  expect(url).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return url;
}

async function openImport(page: Page, base: string): Promise<void> {
  await page.goto(base + importPagePath().replace(/^\//, ""));
  await expect(page.getByTestId("import-form")).toBeVisible();
}

async function addInline(page: Page, name: string, text: string): Promise<string> {
  await page.locator("#import-files").setInputFiles({ name, mimeType: "text/markdown", buffer: Buffer.from(text, "utf8") });
  const id = importSourceId(name);
  await expect(page.getByTestId(`import-source-${id}`)).toBeVisible();
  return id;
}

// selectIn places the browser's live selection over one phrase inside one
// rendered source, the way a drag would; the phrase is located by
// UTF-16 index in the rendered text, which is deliberately NOT the byte
// offset the mapping must carry.
async function selectIn(page: Page, sourceId: string, phrase: string): Promise<void> {
  await page.evaluate(
    ({ sourceId, phrase }) => {
      const pre = document.querySelector(`[data-testid="import-source-text-${sourceId}"]`);
      if (!pre || !pre.firstChild) throw new Error("no rendered source text for " + sourceId);
      const node = pre.firstChild;
      const i = (node.textContent || "").indexOf(phrase);
      if (i < 0) throw new Error("phrase not in rendered source");
      const range = document.createRange();
      range.setStart(node, i);
      range.setEnd(node, i + phrase.length);
      const sel = window.getSelection()!;
      sel.removeAllRanges();
      sel.addRange(range);
    },
    { sourceId, phrase },
  );
}

// selectUnits places the selection over a raw UTF-16 unit range of one
// rendered source — used to reach whitespace-only and mid-character spans
// a phrase search cannot express.
async function selectUnits(page: Page, sourceId: string, start: number, length: number): Promise<void> {
  await page.evaluate(
    ({ sourceId, start, length }) => {
      const pre = document.querySelector(`[data-testid="import-source-text-${sourceId}"]`);
      if (!pre || !pre.firstChild) throw new Error("no rendered source text for " + sourceId);
      const range = document.createRange();
      range.setStart(pre.firstChild, start);
      range.setEnd(pre.firstChild, start + length);
      const sel = window.getSelection()!;
      sel.removeAllRanges();
      sel.addRange(range);
    },
    { sourceId, start, length },
  );
}

async function clearSelection(page: Page): Promise<void> {
  await page.evaluate(() => window.getSelection()!.removeAllRanges());
}

async function mapSelection(page: Page, sourceId: string, target: string) {
  await page.getByTestId(`import-map-target-${sourceId}`).selectOption(target);
  await page.getByTestId(`import-map-selection-${sourceId}`).click();
  return page.getByTestId(`import-map-note-${sourceId}`);
}

async function preview(page: Page): Promise<number> {
  const response = page.waitForResponse((r) => r.url().includes("/design/import/preview"));
  await page.getByTestId("import-preview-btn").click();
  const resp = await response;
  if (resp.status() === 200) {
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "false");
  }
  return resp.status();
}

type Mapping = { target: string; source_id?: string; start?: number; end?: number; transform?: string; text?: string; evidence?: string[] };

async function previewedMappings(page: Page): Promise<Mapping[]> {
  const request = await page.evaluate(() => (window as unknown as { __verdiImport: { state(): { request: string } } }).__verdiImport.state().request);
  return (JSON.parse(request) as { mappings?: Mapping[] }).mappings || [];
}

test.describe("spec import: mapping by selection", () => {
  test("a phrase after multi-byte text maps its exact UTF-8 byte range to a picked target; the preview and coverage agree", async ({ page }) => {
    test.setTimeout(90_000);
    // The test discriminates: the constraint's UTF-16 index and its byte
    // offset differ because an em-dash, curly quotes and a diaeresis
    // precede it.
    const constraint = byteRange(CONSTRAINT_PHRASE);
    const problem = byteRange(PROBLEM_PHRASE);
    expect(constraint.start).not.toBe(PLAN_TEXT.indexOf(CONSTRAINT_PHRASE));
    expect(constraint.end - constraint.start).toBe(CONSTRAINT_PHRASE.length); // ASCII phrase: only its offset shifts
    expect(problem.end - problem.start).toBeGreaterThan(PROBLEM_PHRASE.length); // multi-byte phrase: its length shifts too

    const base = await importBase(page);
    await openImport(page, base);
    const sourceId = await addInline(page, PLAN_NAME, PLAN_TEXT);

    // (1) The source is rendered read-only, whitespace preserved, in a
    // monospace block, and is selectable.
    const text = page.getByTestId(`import-source-text-${sourceId}`);
    await expect(text).toBeVisible();
    expect(await text.evaluate((el) => el.textContent)).toBe(PLAN_TEXT);
    expect(await text.evaluate((el) => getComputedStyle(el).whiteSpace)).toBe("pre-wrap");
    expect(await text.evaluate((el) => getComputedStyle(el).userSelect)).toBe("text");
    expect(await text.evaluate((el) => (el as HTMLElement).isContentEditable)).toBe(false);
    await expect(page.locator(`#import-source-list textarea, #import-source-list [contenteditable="true"]`)).toHaveCount(0);

    // (3) The picker offers both statements and a new object of each of
    // the four kinds, naming the id the new object would take.
    const picker = page.getByTestId(`import-map-target-${sourceId}`);
    for (const value of ["problem", "outcome", "new:ac-", "new:co-", "new:dc-", "new:oq-"]) {
      await expect(picker.locator(`option[value="${value}"]`)).toHaveCount(1);
    }
    // Before any preview the id is the NEXT one, not proven new.
    await expect(picker.locator('option[value="new:co-"]')).toHaveText(/^constraint co-1 \(next id; no current preview/);
    await expect(picker.locator('option[value="new:ac-"]')).toHaveText(/^acceptance criterion ac-1 \(next id/);

    await page.locator("#import-format").selectOption("manual-v1");
    await page.locator("#import-slug").fill("selection-widget");
    await page.locator("#import-title").fill("Selection widget");

    // (2) Problem statement from a multi-byte phrase.
    await selectIn(page, sourceId, PROBLEM_PHRASE);
    const note1 = await mapSelection(page, sourceId, "problem");
    await expect(note1).toHaveAttribute("data-refused", "false");
    await expect(note1).toContainText(`[${problem.start},${problem.end})`);
    await expect(note1).toContainText("problem");

    // A new constraint from an ASCII phrase whose OFFSET is shifted by the
    // multi-byte text before it.
    await selectIn(page, sourceId, CONSTRAINT_PHRASE);
    const note2 = await mapSelection(page, sourceId, "new:co-");
    await expect(note2).toHaveAttribute("data-refused", "false");
    await expect(note2).toContainText("co-1");
    await expect(note2).toContainText(`[${constraint.start},${constraint.end})`);

    // (4) The mapping list shows the covered text beside the byte range,
    // and the Advanced offsets are the computed bytes.
    await expect(page.locator("#import-mapping-count")).toHaveText("2");
    const row = page.locator('#import-mapping-list li[data-mapping-target="co-1"]');
    await expect(row).toHaveCount(1);
    await expect(row.locator(".import-mapping-target")).toHaveValue("co-1");
    await expect(row.locator(".import-mapping-source")).toHaveValue(sourceId);
    await expect(row.locator(".import-mapping-start")).toHaveValue(String(constraint.start));
    await expect(row.locator(".import-mapping-end")).toHaveValue(String(constraint.end));
    await expect(row.locator(".import-mapping-transform")).toHaveValue("identity");
    await expect(row.locator(".import-mapping-excerpt")).toContainText(CONSTRAINT_PHRASE);
    await expect(row.locator(".import-mapping-excerpt")).toContainText(`[${constraint.start},${constraint.end})`);
    const problemRow = page.locator('#import-mapping-list li[data-mapping-target="problem"]');
    await expect(problemRow.locator(".import-mapping-excerpt")).toContainText(PROBLEM_PHRASE);

    // (5) The server accepts exactly those bytes: the field cards carry the
    // exact phrases and the coverage counts exactly their bytes.
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    await expect(page.getByTestId("import-field-co-1")).toHaveAttribute("data-origin", "copied-source");
    expect(await page.getByTestId("import-field-text-co-1").evaluate((el) => el.textContent)).toBe(CONSTRAINT_PHRASE);
    await expect(page.getByTestId("import-field-spans-co-1")).toContainText(`[${constraint.start},${constraint.end})`);
    await expect(page.getByTestId("import-field-problem")).toHaveAttribute("data-origin", "copied-source");
    expect(await page.getByTestId("import-field-text-problem").evaluate((el) => el.textContent)).toBe(PROBLEM_PHRASE);
    await expect(page.getByTestId("import-field-spans-problem")).toContainText(`[${problem.start},${problem.end})`);
    const coverage = page.getByTestId(`import-coverage-${sourceId}`);
    await expect(coverage).toHaveAttribute("data-total", String(PLAN_BYTES.length));
    await expect(coverage).toHaveAttribute("data-mapped", String(problem.end - problem.start + (constraint.end - constraint.start)));
    await expect(coverage).toHaveAttribute("data-unresolved", "0");
    await expect(page.getByTestId("import-group-count-co")).toHaveText("1 constraint");

    // The wire shape is the contract's Mapping, nothing more: a
    // source-backed mapping with identity transform, no text, no evidence.
    const mappings = await previewedMappings(page);
    expect(mappings).toEqual([
      { target: "problem", source_id: sourceId, start: problem.start, end: problem.end, transform: "identity" },
      { target: "co-1", source_id: sourceId, start: constraint.start, end: constraint.end, transform: "identity" },
    ]);

    // After the preview the picker offers the existing co-1 and moves the
    // new-constraint id on; changing the picker is not an edit of the
    // request, so the preview stays current.
    await expect(picker.locator('option[value="co-1"]')).toHaveCount(1);
    await expect(picker.locator('option[value="new:co-"]')).toHaveText(/\(co-2\)/);
    await picker.selectOption("co-1");
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "false");
  });

  test("an empty selection, a selection outside the sources, and one spanning two sources are refused visibly and create no mapping", async ({ page }) => {
    test.setTimeout(60_000);
    const base = await importBase(page);
    await openImport(page, base);
    const planId = await addInline(page, PLAN_NAME, PLAN_TEXT);
    const notesId = await addInline(page, NOTES_NAME, NOTES_TEXT);
    await expect(page.getByTestId(`import-source-text-${notesId}`)).toBeVisible();

    // Empty.
    await clearSelection(page);
    const note = await mapSelection(page, planId, "new:dc-");
    await expect(note).toHaveAttribute("data-refused", "true");
    await expect(note).toContainText(/select/i);
    await expect(page.locator("#import-mapping-count")).toHaveText("0");
    await expect(page.locator("#import-mapping-list li")).toHaveCount(0);

    // Outside any source: the step's own hint text.
    await page.evaluate(() => {
      const hint = document.querySelector("#import-form .field-hint")!;
      const range = document.createRange();
      range.selectNodeContents(hint);
      const sel = window.getSelection()!;
      sel.removeAllRanges();
      sel.addRange(range);
    });
    const outside = await mapSelection(page, planId, "new:dc-");
    await expect(outside).toHaveAttribute("data-refused", "true");
    await expect(page.locator("#import-mapping-list li")).toHaveCount(0);

    // Spanning two sources.
    await page.evaluate(
      ({ planId, notesId }) => {
        const a = document.querySelector(`[data-testid="import-source-text-${planId}"]`)!.firstChild!;
        const b = document.querySelector(`[data-testid="import-source-text-${notesId}"]`)!.firstChild!;
        const range = document.createRange();
        range.setStart(a, 5);
        range.setEnd(b, 6);
        const sel = window.getSelection()!;
        sel.removeAllRanges();
        sel.addRange(range);
      },
      { planId, notesId },
    );
    const spanning = await mapSelection(page, planId, "new:dc-");
    await expect(spanning).toHaveAttribute("data-refused", "true");
    await expect(spanning).toContainText(/one source/i);
    await expect(page.locator("#import-mapping-count")).toHaveText("0");
    await expect(page.locator("#import-mapping-list li")).toHaveCount(0);

    // Whitespace only: the blank line after the Problem heading.
    await selectUnits(page, planId, PLAN_TEXT.indexOf("## Problem") + "## Problem".length, 2);
    const blank = await mapSelection(page, planId, "new:dc-");
    await expect(blank).toHaveAttribute("data-refused", "true");
    await expect(blank).toContainText(/whitespace/i);
    await expect(page.locator("#import-mapping-list li")).toHaveCount(0);

    // Boundary mismatch: a selection starting inside the clef's surrogate
    // pair falls on no byte boundary of the source.
    const clef = NOTES_TEXT.indexOf("𝄞");
    expect(clef).toBeGreaterThan(0);
    await selectUnits(page, notesId, clef + 1, 3);
    const split = await mapSelection(page, notesId, "new:dc-");
    await expect(split).toHaveAttribute("data-refused", "true");
    await expect(split).toContainText(/character boundaries/i);
    await expect(page.locator("#import-mapping-list li")).toHaveCount(0);

    // A selection in the OTHER source, mapped from this source's controls,
    // is refused too: the controls belong to one source.
    await selectIn(page, notesId, "second source");
    const other = await mapSelection(page, planId, "new:dc-");
    await expect(other).toHaveAttribute("data-refused", "true");
    await expect(page.locator("#import-mapping-list li")).toHaveCount(0);
    // ...and mapped from its own controls it succeeds, byte-shifted by the
    // leading multi-byte characters.
    const notesBytes = Buffer.from(NOTES_TEXT, "utf8");
    const start = notesBytes.indexOf(Buffer.from("second source", "utf8"));
    expect(start).not.toBe(NOTES_TEXT.indexOf("second source"));
    await selectIn(page, notesId, "second source");
    const own = await mapSelection(page, notesId, "new:dc-");
    await expect(own).toHaveAttribute("data-refused", "false");
    await expect(own).toContainText(`dc-1`);
    await expect(own).toContainText(`[${start},${start + "second source".length})`);
    await expect(page.locator("#import-mapping-count")).toHaveText("1");

    // The note survives a list re-render: adding a third file rebuilds the
    // source rows, and the notes source's last outcome is still shown.
    await addInline(page, "third.md", "third\n");
    await expect(page.getByTestId(`import-map-note-${notesId}`)).toContainText("dc-1");
    await expect(page.getByTestId(`import-map-note-${notesId}`)).toHaveAttribute("data-refused", "false");
  });

  test("a 'new' id is only called new once a current preview proves it absent; before that the label and note disclose the possible override, which the preview then shows", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    await openImport(page, base);
    // The labeled fixture's automatic recognition owns ac-1..ac-3.
    await page.locator("#import-files").setInputFiles([SPEC_IMPORT_FILES.LABELED]);
    const sourceId = importSourceId(path.basename(SPEC_IMPORT_FILES.LABELED));
    await expect(page.getByTestId(`import-source-${sourceId}`)).toBeVisible();
    await page.locator("#import-slug").fill("override-widget");
    await page.locator("#import-title").fill("Override widget");
    const picker = page.getByTestId(`import-map-target-${sourceId}`);
    const newAC = picker.locator('option[value="new:ac-"]');

    // Before any preview: next id, no novelty claim, the override named.
    await expect(newAC).toHaveText(/ac-1 \(next id; no current preview/);
    await expect(newAC).not.toHaveText(/New/);
    const overridden = "This wastes their afternoon.";
    await selectIn(page, sourceId, overridden);
    const note = await mapSelection(page, sourceId, "new:ac-");
    await expect(note).toHaveAttribute("data-refused", "false");
    await expect(note).toContainText("to ac-1 (next id, unverified until preview; replaces any automatically recognized ac-1)");
    await expect(note).not.toContainText("(new)");

    // The preview shows the override: ac-1 now carries the mapped text, the
    // automatic ac-2 and ac-3 remain.
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    expect(await page.getByTestId("import-field-text-ac-1").evaluate((el) => el.textContent)).toBe(overridden);
    await expect(page.getByTestId("import-field-ac-1")).toHaveAttribute("data-origin", "copied-source");
    await expect(page.getByTestId("import-criteria-count")).toHaveText("3 acceptance criteria");
    await expect(page.getByTestId("import-field-text-ac-2")).toContainText("preserves exact wording");

    // With a current preview the next id is proven absent: "New" is honest,
    // and the mapped id is called new.
    await expect(newAC).toHaveText("New acceptance criterion (ac-4)");
    await expect(picker.locator('option[value="ac-2"]')).toHaveCount(1);
    const added = "The importer reports missing fields.";
    await selectIn(page, sourceId, added);
    const note2 = await mapSelection(page, sourceId, "new:ac-");
    await expect(note2).toContainText("to ac-4 (new)");
    // The edit made the preview stale, so novelty is unverified again.
    await expect(newAC).toHaveText(/ac-5 \(next id; no current preview/);
    expect(await preview(page)).toBe(200);
    expect(await page.getByTestId("import-field-text-ac-4").evaluate((el) => el.textContent)).toBe(added);
    await expect(page.getByTestId("import-criteria-count")).toHaveText("4 acceptance criteria");
    await expect(newAC).toHaveText("New acceptance criterion (ac-5)");
  });
});
