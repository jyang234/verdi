import { test, expect, type Page } from "@playwright/test";
import path from "node:path";
import {
  CONTROL_URL,
  SPEC_IMPORT_FIXTURE_URL,
  SPEC_IMPORT_FILES,
  SPEC_IMPORT_F13_PRIMARY_BYTES,
  SPEC_IMPORT_TRACKER_SCHEME,
  SPEC_IMPORT_PARENT_FEATURE,
  importPagePath,
  importRecordPath,
  importSourceId,
} from "./fixtures";

// Mechanical spec import — the browser adoption path (docs/superpowers/
// specs/2026-09-14-spec-import-contract.md "Errors and browser behavior";
// plan Task 4 UI). Every case here drives the REAL `verdi serve` binary
// built from this tree over an ISOLATED clean-main store with a synthetic
// default-branch proof and no policy/model/forge/tracker configuration
// (cmd/e2eharness/specimportfixture.go) — the shared store's serving
// checkout is dirty by the time this file runs and the importer's own
// clean-context gate would refuse it. The human import proceeds with zero
// model or provider calls; the writer lock is serve's own lifetime lock.
//
// State assertions ride data attributes, testids and text — never
// screenshots (recording stays off).

async function importBase(page: Page): Promise<string> {
  const res = await page.request.get(SPEC_IMPORT_FIXTURE_URL);
  expect(res.ok(), await res.text()).toBe(true);
  const url = (await res.text()).trim();
  expect(url).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return url;
}

function at(base: string, p: string): string {
  return base + p.replace(/^\//, "");
}

async function openImport(page: Page, base: string): Promise<void> {
  await page.goto(at(base, importPagePath()));
  await expect(page.getByTestId("import-form")).toBeVisible();
}

// addFiles uploads explicit files and waits for each source row (keyed by
// the page's mechanical id of the file NAME) to appear.
async function addFiles(page: Page, files: readonly string[]): Promise<string[]> {
  await page.locator("#import-files").setInputFiles([...files]);
  const ids: string[] = [];
  for (const f of files) {
    const id = importSourceId(path.basename(f));
    await expect(page.getByTestId(`import-source-${id}`)).toBeVisible();
    ids.push(id);
  }
  return ids;
}

async function fillTarget(page: Page, slug: string, title: string): Promise<void> {
  await page.locator("#import-slug").fill(slug);
  await page.locator("#import-title").fill(title);
}

// preview clicks Preview, waits for the response and for the page to mark
// the result as current (never stale), and returns the response status.
async function preview(page: Page): Promise<number> {
  const response = page.waitForResponse((r) => r.url().includes("/design/import/preview"));
  await page.getByTestId("import-preview-btn").click();
  const resp = await response;
  if (resp.status() === 200) {
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "false");
  }
  return resp.status();
}

function findings(page: Page, code: string) {
  return page.locator(`#import-findings li[data-code="${code}"]`);
}

async function expectReady(page: Page, ready: boolean): Promise<void> {
  await expect(page.getByTestId("import-ready")).toHaveAttribute("data-ready", String(ready));
}

async function confirmAndApply(page: Page): Promise<{ status: number; body: Record<string, unknown> }> {
  await expect(page.getByTestId("import-confirm")).toBeEnabled();
  await page.getByTestId("import-confirm").check();
  await expect(page.getByTestId("import-apply-btn")).toBeEnabled();
  const response = page.waitForResponse((r) => r.url().includes("/design/import/apply"));
  await page.getByTestId("import-apply-btn").click();
  const resp = await response;
  return { status: resp.status(), body: await resp.json() };
}

// readyLabeled drives the labeled happy path up to a READY preview
// (retained, every criterion carrying attestation), leaving confirmation
// and creation to the caller.
async function readyLabeled(page: Page, base: string, slug: string): Promise<void> {
  await openImport(page, base);
  await addFiles(page, [SPEC_IMPORT_FILES.LABELED]);
  await fillTarget(page, slug, "Widget Import");
  await page.getByTestId("import-retain").check();
  expect(await preview(page)).toBe(200);
  // attestation: the accepted candidate lint's evidence floor for a
  // feature criterion (VL-006), reported by the preview until selected.
  await page.getByTestId("import-evidence-ac-1-attestation").check();
  await page.getByTestId("import-evidence-all-ac-1").click();
  expect(await preview(page)).toBe(200);
  await expectReady(page, true);
}

type CreatedBody = { board_path: string; branch: string; spec_ref: string; commit: string; preview_digest: string; status: string };

// importLabeledViaUI runs the labeled happy path to a created result.
async function importLabeledViaUI(page: Page, base: string, slug: string): Promise<CreatedBody> {
  await readyLabeled(page, base, slug);
  const applied = await confirmAndApply(page);
  expect(applied.status, JSON.stringify(applied.body)).toBe(200);
  await expect(page.getByTestId("import-created")).toHaveAttribute("data-status", "created");
  return applied.body as CreatedBody;
}

// addMapping appends one explicit mapping row and fills it.
async function addMapping(
  page: Page,
  fields: { target: string; source?: string; start?: number; end?: number; transform?: string; text?: string; evidence?: string[] },
) {
  await page.getByTestId("import-add-mapping").click();
  const row = page.locator("#import-mapping-list li").last();
  await row.locator(".import-mapping-target").fill(fields.target);
  if (fields.source) {
    await row.locator(".import-mapping-source").selectOption(fields.source);
    await row.locator(".import-mapping-start").fill(String(fields.start));
    await row.locator(".import-mapping-end").fill(String(fields.end));
    await row.locator(".import-mapping-transform").selectOption(fields.transform || "identity");
  }
  if (fields.text !== undefined) await row.locator(".import-mapping-text").fill(fields.text);
  for (const kind of fields.evidence || []) await row.locator(`.import-mapping-evidence input[data-kind="${kind}"]`).check();
  return row;
}

test.describe("spec import: discoverability", () => {
  test("home offers 'Import existing spec' before any statement is requested", async ({ page }) => {
    await page.goto("/");
    const link = page.getByTestId("home-import-link");
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute("href", importPagePath());
    await expect(link).toHaveText("Import existing spec");
    // Discoverable BEFORE the directory/glance sections, which are where
    // new proposals (and their statements) are otherwise reached from.
    const before = await page.evaluate(() => {
      const a = document.querySelector('[data-testid="home-import-link"]')!;
      const g = document.querySelector('[data-testid="home-glance"]') || document.querySelector(".home-directory");
      return g ? Boolean(a.compareDocumentPosition(g) & Node.DOCUMENT_POSITION_FOLLOWING) : true;
    });
    expect(before).toBe(true);
    await link.click();
    await expect(page).toHaveURL(/\/design\/import$/);
    await expect(page.getByTestId("import-form")).toBeVisible();
    // Explicit files only: no directory, archive, URL or command inputs.
    await expect(page.locator("#import-files")).toHaveAttribute("multiple", "");
    await expect(page.locator("#import-files")).not.toHaveAttribute("webkitdirectory", /.*/);
    await expect(page.locator('input[type="url"]')).toHaveCount(0);
    // Every driven control is labeled.
    for (const id of ["import-files", "import-format", "import-slug", "import-class", "import-title", "import-story"]) {
      await expect(page.locator(`label[for="${id}"]`)).toHaveCount(1);
    }
    await expect(page.getByTestId("import-next-action")).toContainText("Add at least one source file");
  });
});

test.describe("spec import: labeled Markdown journey", () => {
  test("imports end to end with truthful findings, evidence selection, retained acknowledgement, a mapped edit, the ordinary board and the verified record", async ({
    page,
  }) => {
    test.setTimeout(120_000);
    const base = await importBase(page);
    const slug = "widget-import";
    await openImport(page, base);
    const [sourceId] = await addFiles(page, [SPEC_IMPORT_FILES.LABELED]);
    // The label is the file NAME, never a path; the first file is primary.
    await expect(page.getByTestId(`import-source-${sourceId}`).locator(".import-source-label")).toHaveText("positive-basic.md");
    await expect(page.getByTestId(`import-primary-${sourceId}`)).toBeChecked();
    await fillTarget(page, slug, "Widget Import");
    await expect(page.locator("#import-format")).toHaveValue("markdown-v1");

    // Preview 1: nothing retained, no evidence — a completed preview with
    // its blocking findings, never a fabricated success.
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    await expect(findings(page, "unresolved-coverage")).toHaveCount(1);
    await expect(findings(page, "unresolved-coverage").first()).toHaveAttribute("data-blocking", "true");
    await expect(findings(page, "missing-evidence")).toHaveCount(3);
    await expect(page.getByTestId("import-confirm")).toBeDisabled();
    await expect(page.getByTestId("import-apply-btn")).toBeDisabled();
    await expect(page.getByTestId("import-next-action")).toContainText("blocking finding");
    // Automatic positive fields with their source origins.
    await expect(page.getByTestId("import-field-problem")).toHaveAttribute("data-origin", "copied-source");
    await expect(page.getByTestId("import-field-text-problem")).toContainText("Operators currently retype every requirement by hand.");
    await expect(page.getByTestId("import-field-spans-problem")).toContainText("positive-basic.md");
    await expect(page.getByTestId("import-field-outcome")).toHaveAttribute("data-origin", "copied-source");
    for (const ac of ["ac-1", "ac-2", "ac-3"]) {
      await expect(page.getByTestId(`import-field-${ac}`)).toHaveAttribute("data-origin", "copied-source");
    }
    // Coverage is byte accounting, disclosed as such.
    const coverage = page.getByTestId(`import-coverage-${sourceId}`);
    await expect(coverage).toHaveAttribute("data-total", "322");
    expect(Number(await coverage.getAttribute("data-unresolved"))).toBeGreaterThan(0);
    await expect(page.getByTestId("import-coverage")).toContainText("not semantic completeness");

    // Retained acknowledgement: an explicit disposition, and any edit
    // invalidates the preview until a fresh one.
    await page.getByTestId("import-retain").check();
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "true");
    expect(await preview(page)).toBe(200);
    await expect(findings(page, "unresolved-coverage")).toHaveCount(0);
    await expect(coverage).toHaveAttribute("data-unresolved", "0");
    expect(Number(await coverage.getAttribute("data-retained"))).toBeGreaterThan(0);

    // Evidence selection: current model kinds, applied as user selection
    // to one criterion and then to every criterion together. The
    // candidate lint's feature floor (VL-006) needs attestation among the
    // kinds; the preview reports that truthfully as invalid-candidate
    // until it is selected.
    await page.getByTestId("import-evidence-ac-1-static").check();
    await page.getByTestId("import-evidence-ac-1-attestation").check();
    await page.getByTestId("import-evidence-all-ac-1").click();
    await expect(page.getByTestId("import-evidence-ac-3-attestation")).toBeChecked();
    // A meaningful mapped edit of ac-2's own text.
    await page.getByTestId("import-edit-ac-2").click();
    await page.getByTestId("import-edit-text-ac-2").fill("The importer preserves exact wording (edited by the reviewer).");
    await expect(page.locator('#import-mapping-list [data-mapping-target="ac-2"]')).toHaveCount(1);
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    await expect(findings(page, "missing-evidence")).toHaveCount(0);
    await expect(page.getByTestId("import-field-ac-2")).toHaveAttribute("data-origin", "user-edited-source");
    await expect(page.getByTestId("import-field-text-ac-2")).toContainText("(edited by the reviewer)");
    await expect(page.getByTestId("import-field-ac-1")).toContainText("static");
    await expect(page.getByTestId("import-digest")).toHaveText(/^[0-9a-f]{64}$/);
    await expect(page.getByTestId("import-next-action")).toContainText("confirm");

    // Confirm this exact preview and create.
    const applied = await confirmAndApply(page);
    expect(applied.status, JSON.stringify(applied.body)).toBe(200);
    const created = page.getByTestId("import-created");
    await expect(created).toHaveAttribute("data-status", "created");
    await expect(created).toContainText("spec/" + slug);
    await expect(created).toContainText("design/" + slug);
    const boardHref = `/b/design%2F${slug}/board/spec/${slug}`;
    await expect(page.getByTestId("import-board-link")).toHaveAttribute("href", boardHref);
    await expect(page.getByTestId("import-record-link")).toHaveAttribute("href", importRecordPath("design/" + slug, slug));
    expect(applied.body.board_path).toBe(boardHref);
    expect(applied.body.statements_deferred).toBe(false);

    // The ordinary branch board, authoring, with the source-record link
    // beside the semantic review panel explaining the import origin apart
    // from ASD history and acceptance.
    await page.goto(at(base, boardHref));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    const origin = page.getByTestId("asd-import-origin");
    await expect(origin).toBeVisible();
    await expect(origin.locator("a")).toHaveAttribute("href", importRecordPath("design/" + slug, slug));
    await expect(origin).toContainText("not");
    await expect(origin).toContainText("acceptance");
    await expect(origin).toContainText("unclassified");
    const adjacent = await page.evaluate(() => {
      const review = document.querySelector('[data-testid="asd-review"]')!;
      const origin = document.querySelector('[data-testid="asd-import-origin"]')!;
      return review.parentElement === origin.parentElement && Boolean(review.compareDocumentPosition(origin) & Node.DOCUMENT_POSITION_FOLLOWING);
    });
    expect(adjacent).toBe(true);
    await expect(page.locator('[data-testid="placard-problem"] .placard-text')).toContainText("retype every requirement");

    // An ordinary supported edit through the typed form, committed on the
    // branch, survives a reload.
    await page.locator("#asd-set-outcome").click();
    await page.getByTestId("asd-op-text").fill("Operators import existing specs directly into a board [72-edit].");
    const mutated = page.waitForResponse((r) => r.url().includes("/api/mutate_draft") && r.ok());
    await page.getByTestId("asd-op-ok").click();
    await mutated;
    await expect(page.locator('[data-testid="placard-outcome"] .placard-text')).toContainText("[72-edit]", { timeout: 10_000 });
    await page.getByRole("button", { name: "Commit & push" }).click();
    const dialog = page.getByRole("dialog", { name: "Commit & push" });
    await dialog.getByRole("textbox", { name: "Commit message" }).fill("board: ordinary edit after import");
    await dialog.getByRole("button", { name: "Commit" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByTestId("uncommitted-indicator")).toBeHidden({ timeout: 10_000 });
    await page.reload();
    await expect(page.locator('[data-testid="placard-outcome"] .placard-text')).toContainText("[72-edit]");
    await expect(page.getByTestId("asd-import-origin")).toBeVisible();

    // The record remains verifiable and honest: the ORIGINAL import
    // verified against committed bytes, the current spec disclosed as
    // changed — never as corrupted provenance.
    await page.getByTestId("asd-import-origin").locator("a").click();
    const record = page.getByTestId("import-record");
    await expect(record).toBeVisible();
    await expect(record).toHaveAttribute("data-current-spec-matches", "false");
    await expect(record).toHaveAttribute("data-import-commit", applied.body.commit as string);
    const changed = page.getByTestId("record-current-spec-changed");
    await expect(changed).toBeVisible();
    await expect(changed).toContainText("current-spec-changed");
    await expect(changed).toContainText("ORIGINAL imported revision");
    await expect(page.getByTestId("import-record-unavailable")).toHaveCount(0);
    await expect(record).toContainText("(edited by the reviewer)");
    await expect(record).toContainText("unauthenticated");
    await expect(record).toContainText("not-applicable");
    await expect(record).toContainText("positive-basic.md");
    await expect(record).toContainText("not");
    await expect(record).toContainText("acceptance");
  });
});

test.describe("spec import: confirmation is bound to the exact current preview", () => {
  test("any edit clears the confirmation; a stale in-flight preview response cannot reinstate it", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    await openImport(page, base);
    const [sourceId] = await addFiles(page, [SPEC_IMPORT_FILES.LABELED]);
    await fillTarget(page, "stale-widget", "Stale widget");
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    const firstDigest = await page.getByTestId("import-digest").textContent();

    // The race: a preview issued for the current inputs answers AFTER a
    // range edit; the response must be discarded — the shown preview stays
    // stale and nothing can be confirmed against it.
    await page.route("**/design/import/preview", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 1500));
      await route.continue();
    });
    const delayed = page.waitForResponse((r) => r.url().includes("/design/import/preview"));
    await page.getByTestId("import-preview-btn").click();
    await page.waitForTimeout(300);
    await page.locator(`[data-testid="import-start-${sourceId}"]`).fill("1");
    await page.locator(`[data-testid="import-end-${sourceId}"]`).fill("10");
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "true");
    await delayed;
    await page.waitForTimeout(300);
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "true");
    await expect(page.getByTestId("import-digest")).toHaveText(firstDigest!);
    await expect(page.getByTestId("import-confirm")).not.toBeChecked();
    await expect(page.getByTestId("import-confirm")).toBeDisabled();
    await expect(page.getByTestId("import-apply-btn")).toBeDisabled();
    await page.unroute("**/design/import/preview");

    // A fresh preview over the changed range is honest about it: the
    // selected slice is smaller and the digest differs.
    expect(await preview(page)).toBe(200);
    await expect(page.getByTestId(`import-coverage-${sourceId}`)).not.toHaveAttribute("data-total", "322");
    await expect(page.getByTestId("import-digest")).not.toHaveText(firstDigest!);

    // Back to the whole file, evidence for every criterion, ready, confirmed.
    await page.locator(`[data-testid="import-start-${sourceId}"]`).fill("");
    await page.locator(`[data-testid="import-end-${sourceId}"]`).fill("");
    expect(await preview(page)).toBe(200);
    await page.getByTestId("import-evidence-ac-1-attestation").check();
    await page.getByTestId("import-evidence-all-ac-1").click();
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    await page.getByTestId("import-confirm").check();
    await expect(page.getByTestId("import-apply-btn")).toBeEnabled();

    // A target edit clears and disables the confirmation.
    await page.locator("#import-title").fill("Stale widget, retitled");
    await expect(page.getByTestId("import-confirm")).not.toBeChecked();
    await expect(page.getByTestId("import-confirm")).toBeDisabled();
    await expect(page.getByTestId("import-apply-btn")).toBeDisabled();
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "true");
    await expect(page.getByTestId("import-next-action")).toContainText("preview again");

    // Evidence and deferral edits invalidate just the same.
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    await page.getByTestId("import-confirm").check();
    await page.getByTestId("import-evidence-ac-2-runtime").check();
    await expect(page.getByTestId("import-confirm")).not.toBeChecked();
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "true");
    expect(await preview(page)).toBe(200);
    await page.getByTestId("import-confirm").check();
    await page.getByTestId("import-defer").check();
    await expect(page.getByTestId("import-confirm")).toBeDisabled();
    await expect(page.getByTestId("import-result")).toHaveAttribute("data-stale", "true");
  });
});

test.describe("spec import: the F13 reference profile", () => {
  test("absent labels are separate from the eight missing evidence declarations; pair deferral discloses TODO placeholders; created and already-created keep the disclosures", async ({
    page,
  }) => {
    test.setTimeout(120_000);
    const base = await importBase(page);
    await openImport(page, base);
    const ids = await addFiles(page, [SPEC_IMPORT_FILES.F13_PRIMARY, ...SPEC_IMPORT_FILES.F13_SUPPORTS]);
    const primary = ids[0];
    await expect(page.getByTestId(`import-primary-${primary}`)).toBeChecked();
    await page.locator("#import-format").selectOption("f13-reference-v1");
    await fillTarget(page, "gatekeeper-flight", "Bounded gatekeeper state machine");
    await page.getByTestId("import-retain").check();

    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    // Two absent labels and eight unset evidence declarations, each its
    // own blocking finding (the candidate lint's own blocking splice
    // finding rides beside them until evidence exists).
    await expect(findings(page, "missing-statement")).toHaveCount(2);
    await expect(findings(page, "missing-statement").first()).toContainText("Problem");
    await expect(page.locator('#import-findings li[data-code="missing-statement"][data-blocking="true"]')).toHaveCount(2);
    await expect(findings(page, "missing-evidence")).toHaveCount(8);
    await expect(page.locator('#import-findings li[data-code="missing-evidence"][data-blocking="true"]')).toHaveCount(8);
    for (let i = 1; i <= 8; i++) {
      await expect(page.getByTestId(`import-field-ac-${i}`)).toHaveAttribute("data-origin", "copied-source");
      await expect(page.getByTestId(`import-field-spans-ac-${i}`)).toContainText("primary-f13.md");
    }
    await expect(page.getByTestId("import-field-problem")).toHaveCount(0);
    const primaryCoverage = page.getByTestId(`import-coverage-${primary}`);
    await expect(primaryCoverage).toHaveAttribute("data-total", String(SPEC_IMPORT_F13_PRIMARY_BYTES));
    await expect(primaryCoverage).toHaveAttribute("data-unresolved", "0");
    for (const support of ids.slice(1)) {
      const row = page.getByTestId(`import-coverage-${support}`);
      expect(await row.getAttribute("data-retained")).toBe(await row.getAttribute("data-total"));
    }

    // Explicit pair deferral: TODO placeholders, disclosed, never source text.
    await page.getByTestId("import-defer").check();
    expect(await preview(page)).toBe(200);
    await expect(findings(page, "missing-statement")).toHaveCount(0);
    await expect(findings(page, "statements-deferred")).toHaveCount(2);
    await expect(findings(page, "statements-deferred").first()).toHaveAttribute("data-blocking", "false");
    await expect(findings(page, "missing-evidence")).toHaveCount(8);
    await expect(page.getByTestId("import-field-problem")).toHaveAttribute("data-origin", "generated-deferral");
    await expect(page.getByTestId("import-field-text-problem")).toContainText("TODO");
    await expect(page.getByTestId("import-field-outcome")).toHaveAttribute("data-origin", "generated-deferral");

    // Evidence for every criterion, then create.
    await page.getByTestId("import-evidence-ac-1-attestation").check();
    await page.getByTestId("import-evidence-all-ac-1").click();
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    const applied = await confirmAndApply(page);
    expect(applied.status, JSON.stringify(applied.body)).toBe(200);
    await expect(page.getByTestId("import-created")).toHaveAttribute("data-status", "created");
    expect(applied.body.statements_deferred).toBe(true);
    await expect(page.locator('#import-disclosures li[data-code="statements-deferred"]')).toHaveCount(2);

    // An identical retry (the page's own exact request and digest) is the
    // same already-created result with its disclosures preserved.
    const state = await page.evaluate(() => (window as unknown as { __verdiImport: { state: () => { request: string; digest: string } } }).__verdiImport.state());
    const retry = await page.request.post(at(base, "/design/import/apply"), {
      headers: { "Content-Type": "application/json", "X-Verdi-Import-Preview": state.digest },
      data: state.request,
    });
    expect(retry.status()).toBe(200);
    const again = await retry.json();
    expect(again.status).toBe("already-created");
    expect(again.commit).toBe(applied.body.commit);
    expect(again.statements_deferred).toBe(true);
    expect(again.disclosures.filter((d: { code: string }) => d.code === "statements-deferred")).toHaveLength(2);
  });
});

test.describe("spec import: corrections", () => {
  test("an ambiguous heading is a blocking finding that an explicit mapping corrects; the importer never chooses", async ({ page }) => {
    const base = await importBase(page);
    await openImport(page, base);
    await addFiles(page, [SPEC_IMPORT_FILES.AMBIGUOUS]);
    await fillTarget(page, "ambiguous-widget", "Ambiguous widget");
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    const ambiguous = findings(page, "ambiguous-field");
    await expect(ambiguous).toHaveCount(1);
    await expect(ambiguous.first()).toHaveAttribute("data-target", "problem");
    await expect(ambiguous.first()).toHaveAttribute("data-blocking", "true");
    await expect(ambiguous.first()).toContainText("explicit mapping");
    await expect(page.getByTestId("import-field-problem")).toHaveCount(0);

    await page.getByTestId("import-add-mapping").click();
    const row = page.locator("#import-mapping-list li").last();
    await row.locator(".import-mapping-target").fill("problem");
    await row.locator(".import-mapping-text").fill("Operators currently retype every requirement by hand.");
    expect(await preview(page)).toBe(200);
    // The mapping supplies the value; the source-structure gap stays
    // disclosed, no longer blocking — never silently vanished.
    await expect(page.locator('#import-findings li[data-code="ambiguous-field"][data-blocking="true"]')).toHaveCount(0);
    await expect(page.locator('#import-findings li[data-code="ambiguous-field"][data-blocking="false"]')).toHaveCount(1);
    await expect(page.getByTestId("import-field-problem")).toHaveAttribute("data-origin", "user-added");
    await expect(page.getByTestId("import-field-text-problem")).toContainText("retype every requirement");
  });
});

test.describe("spec import: escaping", () => {
  test("source text, labels and error details render as text; nothing from the source executes", async ({ page }) => {
    const base = await importBase(page);
    await openImport(page, base);
    await addFiles(page, [SPEC_IMPORT_FILES.XSS]);
    const hostile = '<img src=x onerror="window.__xss=3">.md';
    await page.locator("#import-files").setInputFiles({
      name: hostile,
      mimeType: "text/markdown",
      buffer: Buffer.from("# Hostile name\n\n## Problem\n\nx\n\n## Outcome\n\ny\n"),
    });
    const hostileRow = page.locator(`#import-source-list li[data-testid="import-source-${importSourceId(hostile)}"]`);
    await expect(hostileRow.locator(".import-source-label")).toHaveText(hostile);
    await expect(page.locator("#import-source-list img")).toHaveCount(0);
    await fillTarget(page, "escaped-import", "<b>Escaped</b> title");
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    await expect(page.getByTestId("import-field-text-problem")).toContainText("<script>window.__xss = 1</script>");
    await expect(page.getByTestId("import-field-text-problem")).toContainText('<img src=x onerror="window.__xss = 2">');
    await expect(page.locator("#import-fields img, #import-fields script, #import-result img, #import-result script")).toHaveCount(0);
    expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();

    // A failure detail is text too.
    await page.route("**/design/import/preview", (route) =>
      route.fulfill({
        status: 400,
        contentType: "application/json; charset=utf-8",
        body: JSON.stringify({ code: "invalid-request", error: '<img src=x onerror="window.__xss=4"> refused' }),
      }),
    );
    expect(await preview(page)).toBe(400);
    const error = page.getByTestId("import-error");
    await expect(error).toBeVisible();
    await expect(error).toHaveAttribute("data-code", "invalid-request");
    await expect(error).toContainText('<img src=x onerror="window.__xss=4"> refused');
    await expect(error.locator("img")).toHaveCount(0);
    expect(await page.evaluate(() => (window as unknown as { __xss?: number }).__xss)).toBeUndefined();
    await page.unroute("**/design/import/preview");
  });
});

test.describe("spec import: transport refusals and the hermetic store", () => {
  test("strict body grammar, the raw envelope cap, method and same-origin guards, and the apply digest binding", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const previewURL = at(base, "/design/import/preview");
    const applyURL = at(base, "/design/import/apply");
    const json = { "Content-Type": "application/json" };
    const labeled = require("node:fs").readFileSync(SPEC_IMPORT_FILES.LABELED);
    const valid = JSON.stringify({
      schema: "verdi.spec-import-request/v1",
      target: { slug: "transport-probe", class: "feature", title: "Transport probe" },
      format: "markdown-v1",
      primary: "widget",
      sources: [{ id: "widget", label: "positive-basic.md", data: labeled.toString("base64") }],
      defer_statements: false,
      retain_unmapped: true,
    });

    for (const body of [`{"schema":"verdi.spec-import-request/v1","bogus":1}`, `null`, `[]`, `${valid}{}`, ``]) {
      const resp = await page.request.post(previewURL, { headers: json, data: body });
      expect(resp.status(), body.slice(0, 40)).toBe(400);
      expect((await resp.json()).code).toBe("invalid-request");
    }
    const oversize = Buffer.alloc(12 * 1024 * 1024 + 1, " ");
    oversize.write(valid, 0, "utf8");
    const big = await page.request.post(previewURL, { headers: json, data: oversize });
    expect(big.status()).toBe(413);
    expect((await big.json()).code).toBe("invalid-request");

    expect((await page.request.get(previewURL)).status()).toBe(405);
    expect((await page.request.put(applyURL, { headers: json, data: valid })).status()).toBe(405);
    expect((await page.request.post(at(base, "/design/import"), { headers: json, data: "{}" })).status()).toBe(405);

    const crossOrigin: Record<string, string>[] = [{ "Sec-Fetch-Site": "cross-site" }, { Origin: "http://evil.invalid" }];
    for (const headers of crossOrigin) {
      const resp = await page.request.post(previewURL, { headers: { ...json, ...headers }, data: valid });
      expect(resp.status(), JSON.stringify(headers)).toBe(403);
    }
    const ok = await page.request.post(previewURL, { headers: { ...json, Origin: base.replace(/\/$/, ""), "Sec-Fetch-Site": "same-origin" }, data: valid });
    expect(ok.status()).toBe(200);
    const previewed = await ok.json();
    expect(previewed.ready).toBe(false);

    const noDigest = await page.request.post(applyURL, { headers: json, data: valid });
    expect(noDigest.status()).toBe(400);
    expect((await noDigest.json()).code).toBe("invalid-request");
    const stale = await page.request.post(applyURL, { headers: { ...json, "X-Verdi-Import-Preview": "a".repeat(64) }, data: valid });
    expect(stale.status()).toBe(409);
    expect((await stale.json()).code).toBe("stale-preview");
    const unresolved = await page.request.post(applyURL, { headers: { ...json, "X-Verdi-Import-Preview": previewed.digest }, data: valid });
    expect(unresolved.status()).toBe(409);
    expect((await unresolved.json()).code).toBe("unresolved");

    // The record view distinguishes a malformed query from unavailable proof.
    const invalid = await page.request.get(at(base, "/design/import/record?branch=-x&spec=transport-probe"));
    expect(invalid.status()).toBe(400);
    expect(await invalid.text()).toContain('data-testid="import-record-invalid"');
    const missing = await page.request.get(at(base, importRecordPath("design/never-imported", "never-imported")));
    expect(missing.status()).toBe(409);
    const missingText = await missing.text();
    expect(missingText).toContain('data-testid="import-record-unavailable"');
    expect(missingText).toContain("provenance-mismatch");
    expect(missingText).not.toContain("fatal:");
    expect(missingText).not.toContain("gitx:");

    // No configuration dependency: the served store is manifest-only, has
    // no adopted policy or model override, and its serve runs under the
    // stripped environment — the human import above needed none of them.
    const info = await (await page.request.get(`${CONTROL_URL}/spec-import-fixture/info`)).json();
    expect(info.url).toBe(base);
    // The manifest is the layout schema plus ONE synthetic, test-only
    // tracker provider (never contacted); no forge, policy or model override.
    expect(info.manifest).toContain("schema: verdi.layout/v1\n");
    expect(info.manifest).toContain("providers:\n  " + SPEC_IMPORT_TRACKER_SCHEME + ":");
    expect(info.manifest).not.toContain("forge");
    expect(info.synthetic_tracker).toBe(SPEC_IMPORT_TRACKER_SCHEME);
    expect(info.parent_feature).toBe(SPEC_IMPORT_PARENT_FEATURE);
    expect(info.policy_adopted).toBe(false);
    expect(info.model_override).toBe(false);
    expect(info.stripped_env).toEqual(expect.arrayContaining(["VERDI_REVIEW_FEED", "VERDI_OPENMR_FEED", "VERDI_DIAGRAM_VERIFICATION", "CI_DEFAULT_BRANCH"]));
    expect(info.branch).toBe("main");
    expect(info.porcelain).toBe("");
  });
});

test.describe("spec import: corrupted record", () => {
  test("a tampered record is disclosed as unavailable proof, never as the original and never as a mere current-spec change", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const slug = "tamper-widget";
    const created = await importLabeledViaUI(page, base, slug);
    const before = await page.goto(at(base, importRecordPath(created.branch, slug)));
    expect(before?.status()).toBe(200);
    await expect(page.getByTestId("import-record")).toHaveAttribute("data-current-spec-matches", "true");

    const tamper = await page.request.post(`${CONTROL_URL}/spec-import-fixture/tamper?branch=${encodeURIComponent(created.branch)}&spec=${slug}`);
    expect(tamper.status(), await tamper.text()).toBe(204);

    const after = await page.goto(at(base, importRecordPath(created.branch, slug)));
    expect(after?.status()).toBe(409);
    const unavailable = page.getByTestId("import-record-unavailable");
    await expect(unavailable).toBeVisible();
    await expect(unavailable).toContainText("provenance-mismatch");
    await expect(page.getByTestId("record-current-spec-changed")).toHaveCount(0);
    await expect(page.getByTestId("import-record")).toHaveCount(0);
    await expect(page.locator("body")).not.toContainText("fatal:");

    // The board over that branch (its worktree cut AFTER the tamper) keeps
    // the source-record link — presence of a record file — but must not
    // assert verification from that presence: verification is the record
    // view's own successful read, which here discloses the corruption.
    await page.goto(at(base, created.board_path));
    const origin = page.getByTestId("asd-import-origin");
    await expect(origin).toBeVisible();
    await expect(origin).not.toContainText("verified against");
    await expect(origin).toContainText("not verified here");
    await expect(origin).toContainText("acceptance");
    await expect(origin).toContainText("unclassified");
    await origin.locator("a").click();
    await expect(page.getByTestId("import-record-unavailable")).toBeVisible();
  });
});

test.describe("spec import: apply recovery", () => {
  test("a lost apply response is recoverable by a visible retry that reports already-created without a duplicate", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const slug = "lost-response";
    await readyLabeled(page, base, slug);
    // The request reaches the server and publishes; the response never
    // reaches the page.
    await page.route("**/design/import/apply", async (route) => {
      await route.fetch();
      await route.abort("failed");
    });
    await page.getByTestId("import-confirm").check();
    await page.getByTestId("import-apply-btn").click();
    const retry = page.getByTestId("import-retry");
    await expect(retry).toBeVisible();
    await expect(retry).toContainText("outcome is unknown");
    await expect(page.getByTestId("import-retry-btn")).toBeEnabled();
    await expect(page.getByTestId("import-created")).toBeHidden();
    await expect(page.getByTestId("import-next-action")).toContainText("Retry");
    await page.unroute("**/design/import/apply");

    // The visible retry resends the SAME request bytes and digest; the
    // server reconciles to the publication it already made.
    const response = page.waitForResponse((r) => r.url().includes("/design/import/apply"));
    await page.getByTestId("import-retry-btn").click();
    const resp = await response;
    expect(resp.status()).toBe(200);
    const body = (await resp.json()) as CreatedBody;
    expect(body.status).toBe("already-created");
    await expect(page.getByTestId("import-created")).toHaveAttribute("data-status", "already-created");
    await expect(page.getByTestId("import-created")).toContainText(body.commit);
    await expect(page.getByTestId("import-created")).toContainText("design/" + slug);
    await expect(retry).toBeHidden();
    // No duplicate: the branch's verified import commit IS that commit, and
    // one more identical apply still reconciles to it.
    const record = await page.goto(at(base, importRecordPath(body.branch, slug)));
    expect(record?.status()).toBe(200);
    await expect(page.getByTestId("import-record")).toHaveAttribute("data-import-commit", body.commit);
  });

  test("an edit while apply is in flight never silently loses the publication notice", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const slug = "late-notice";
    await readyLabeled(page, base, slug);
    await page.route("**/design/import/apply", async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 1500));
      await route.continue();
    });
    await page.getByTestId("import-confirm").check();
    const response = page.waitForResponse((r) => r.url().includes("/design/import/apply"));
    await page.getByTestId("import-apply-btn").click();
    await page.waitForTimeout(300);
    // An edit lands while the publication is in flight.
    await page.locator("#import-title").fill("Late notice, retitled");
    const resp = await response;
    expect(resp.status()).toBe(200);
    await page.unroute("**/design/import/apply");
    const created = page.getByTestId("import-created");
    await expect(created).toBeVisible();
    await expect(created).toHaveAttribute("data-status", "created");
    await expect(created).toHaveAttribute("data-late", "true");
    await expect(created).toContainText("design/" + slug);
    await expect(created).toContainText("earlier");
    await expect(page.getByTestId("import-confirm")).toBeDisabled();
    await expect(page.getByTestId("import-next-action")).toContainText("earlier");

    // The edited inputs are not that proposal: re-creating under the same
    // name is refused as target-exists, with the existing board offered.
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    await page.getByTestId("import-confirm").check();
    const refused = page.waitForResponse((r) => r.url().includes("/design/import/apply"));
    await page.getByTestId("import-apply-btn").click();
    expect((await refused).status()).toBe(409);
    const error = page.getByTestId("import-error");
    await expect(error).toHaveAttribute("data-code", "target-exists");
    await expect(page.getByTestId("import-existing-board-link")).toHaveAttribute("href", `/b/design%2F${slug}/board/spec/${slug}`);
  });
});

test.describe("spec import: native, manual and story surfaces", () => {
  test("native: exact eligible bytes import byte-identically; an identity mismatch is refused and corrected; mappings and deferral are refused", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    await openImport(page, base);
    await addFiles(page, [SPEC_IMPORT_FILES.NATIVE]);
    await page.locator("#import-format").selectOption("native");
    await fillTarget(page, "native-gadget", "Native Widget");
    expect(await preview(page)).toBe(400);
    const error = page.getByTestId("import-error");
    await expect(error).toBeVisible();
    await expect(error).toHaveAttribute("data-code", "invalid-request");
    await expect(error).toContainText("does not match target slug");

    await page.locator("#import-slug").fill("native-widget");
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    // Native content is byte-identical: no per-field views, no edit or
    // evidence controls (the validator refuses every explicit mapping for
    // native), and the preview says so rather than reading as "no fields".
    await expect(page.locator("#import-fields article")).toHaveCount(0);
    await expect(page.getByTestId("import-fields")).toContainText("byte for byte");
    await expect(page.locator("#import-fields .import-edit")).toHaveCount(0);
    await expect(page.locator("#import-fields .import-evidence")).toHaveCount(0);
    const coverage = page.getByTestId(`import-coverage-${importSourceId("native-widget.md")}`);
    expect(await coverage.getAttribute("data-mapped")).toBe(await coverage.getAttribute("data-total"));
    const state = await page.evaluate(() => (window as unknown as { __verdiImport: { state: () => { request: string } } }).__verdiImport.state());
    const previewed = await (
      await page.request.post(at(base, "/design/import/preview"), { headers: { "Content-Type": "application/json" }, data: state.request })
    ).json();
    expect(previewed.candidate).toBe(previewed.sources[0].data);

    const applied = await confirmAndApply(page);
    expect(applied.status, JSON.stringify(applied.body)).toBe(200);
    await expect(page.getByTestId("import-created")).toHaveAttribute("data-status", "created");
    await page.getByTestId("import-record-link").click();
    await expect(page.getByTestId("import-record")).toHaveAttribute("data-current-spec-matches", "true");
    await expect(page.getByTestId(`record-source-${importSourceId("native-widget.md")}`)).toBeVisible();

    // Explicit controls are refused for native, named by the validator.
    await page.goBack();
    await expect(page.getByTestId("import-form")).toBeVisible();
    await addFiles(page, [SPEC_IMPORT_FILES.NATIVE]);
    await page.locator("#import-format").selectOption("native");
    await fillTarget(page, "native-widget", "Native Widget");
    await addMapping(page, { target: "problem", text: "rewritten" });
    expect(await preview(page)).toBe(400);
    await expect(page.getByTestId("import-error")).toContainText("refuses explicit mappings");
  });

  test("manual: source spans and user-authored text with explicit evidence make a ready candidate; coverage counts exactly the mapped bytes", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const bytes: Buffer = require("node:fs").readFileSync(SPEC_IMPORT_FILES.LABELED);
    const problemStart = bytes.indexOf("Operators currently");
    const problemEnd = bytes.indexOf("\n\n## Outcome");
    const acText = "The importer reads a Markdown file.";
    const acStart = bytes.indexOf(acText);
    const acEnd = acStart + acText.length;
    expect(problemStart).toBeGreaterThan(0);
    expect(acStart).toBeGreaterThan(problemEnd);

    await openImport(page, base);
    const [sourceId] = await addFiles(page, [SPEC_IMPORT_FILES.LABELED]);
    await page.locator("#import-format").selectOption("manual-v1");
    await fillTarget(page, "manual-widget", "Manual Widget");
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    // No automatic fields at all: the statements are missing findings.
    await expect(findings(page, "missing-statement")).toHaveCount(2);
    await expect(page.locator("#import-fields article")).toHaveCount(0);

    await addMapping(page, { target: "problem", source: sourceId, start: problemStart, end: problemEnd, transform: "identity" });
    await addMapping(page, { target: "outcome", text: "Operators bring existing specs onto a board without retyping them." });
    await addMapping(page, { target: "ac-1", source: sourceId, start: acStart, end: acEnd, transform: "identity", evidence: ["attestation"] });
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    await expect(page.getByTestId("import-field-problem")).toHaveAttribute("data-origin", "copied-source");
    await expect(page.getByTestId("import-field-text-problem")).toContainText("This wastes their afternoon.");
    await expect(page.getByTestId("import-field-spans-problem")).toContainText(`[${problemStart},${problemEnd})`);
    await expect(page.getByTestId("import-field-outcome")).toHaveAttribute("data-origin", "user-added");
    await expect(page.getByTestId("import-field-spans-outcome")).toContainText("no source span");
    await expect(page.getByTestId("import-field-ac-1")).toHaveAttribute("data-origin", "copied-source");
    await expect(page.getByTestId("import-evidence-ac-1-attestation")).toBeChecked();
    const coverage = page.getByTestId(`import-coverage-${sourceId}`);
    await expect(coverage).toHaveAttribute("data-mapped", String(problemEnd - problemStart + (acEnd - acStart)));
    await expect(coverage).toHaveAttribute("data-unresolved", "0");

    const applied = await confirmAndApply(page);
    expect(applied.status, JSON.stringify(applied.body)).toBe(200);
    await page.getByTestId("import-record-link").click();
    await expect(page.getByTestId("record-field-outcome")).toHaveAttribute("data-origin", "user-added");
    await expect(page.getByTestId("record-field-ac-1")).toHaveAttribute("data-origin", "copied-source");
    await expect(page.getByTestId("import-record")).toContainText("without retyping them");
  });

  test("story: tracker, parent and implements link are selected explicitly; a missing edge, a dangling ref and an unconfigured scheme are corrected in place", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    await openImport(page, base);
    await addFiles(page, [SPEC_IMPORT_FILES.LABELED]);
    await page.locator("#import-class").selectOption("story");
    await fillTarget(page, "widget-story", "Widget Import");
    await page.locator("#import-story").fill("bogus:WID-1");
    await page.getByTestId("import-retain").check();
    expect(await preview(page)).toBe(200);
    await page.getByTestId("import-evidence-ac-1-static").check();
    await page.getByTestId("import-evidence-all-ac-1").click();
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    await expect(page.locator("#import-findings li[data-blocking='true']").filter({ hasText: "implements edge" })).toHaveCount(1);

    // A declared link to a parent that does not exist is refused by name.
    await page.getByTestId("import-add-link").click();
    const link = page.locator("#import-link-list li").last();
    await link.locator(".import-link-type").selectOption("implements");
    await link.locator(".import-link-ref").fill("spec/no-such-feature#ac-1");
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    await expect(page.locator("#import-findings li[data-blocking='true']").filter({ hasText: "VL-003" })).toHaveCount(1);

    // The landed parent resolves; the tracker scheme is still unconfigured.
    await link.locator(".import-link-ref").fill(`spec/${SPEC_IMPORT_PARENT_FEATURE}#ac-1`);
    expect(await preview(page)).toBe(200);
    await expectReady(page, false);
    await expect(page.locator("#import-findings li[data-blocking='true']").filter({ hasText: "VL-003" })).toHaveCount(0);
    await expect(page.locator("#import-findings li[data-blocking='true']").filter({ hasText: "VL-005" })).toHaveCount(1);

    await page.locator("#import-story").fill(`${SPEC_IMPORT_TRACKER_SCHEME}:WID-1`);
    expect(await preview(page)).toBe(200);
    await expectReady(page, true);
    const applied = await confirmAndApply(page);
    expect(applied.status, JSON.stringify(applied.body)).toBe(200);
    expect(applied.body.spec_ref).toBe("spec/widget-story");
    await page.getByTestId("import-board-link").click();
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    await expect(page.locator("body")).toContainText(SPEC_IMPORT_PARENT_FEATURE);
  });
});
