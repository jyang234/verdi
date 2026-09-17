import { test, expect, type Page } from "@playwright/test";
import fs from "node:fs";
import path from "node:path";
import {
  SPEC_IMPORT_FIXTURE_URL,
  SPEC_IMPORT_FILES,
  SPEC_IMPORT_F13_PRIMARY_SHA256,
  importRecordPath,
  importSourceId,
} from "./fixtures";

// The import record page names its format and, for a reference profile,
// the pinned primary digest it was bound to (spec/uat-round-1 ac-4, wave-1
// ledger R-1; closes UAT-005). Both imports here go through the real
// `verdi serve` import transport of the isolated import store
// (cmd/e2eharness/specimportfixture.go) at request level — the browser
// import journey itself is 72-spec-import.spec.ts — and the RECORD PAGE
// is what the browser then reads. State assertions ride testids and text,
// never screenshots (recording stays off).
//
// Disclosed as unproven here: the "not recorded (pre-ac-4 record)" value.
// The harness produces records only through the current importer, which
// always persists a format, and its one tamper endpoint truncates a
// record rather than rewriting its fields, so no fixture reaches a
// pre-ac-4 record from the browser. That value is pinned at render level
// by internal/workbench's TestRenderSpecImportRecord_FormatFacts.

type AppliedBody = { status: string; branch: string; commit: string };

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

function source(file: string): { id: string; label: string; data: string } {
  const label = path.basename(file);
  return { id: importSourceId(label), label, data: fs.readFileSync(file).toString("base64") };
}

// importAtRequestLevel previews and applies one request exactly as the
// page's own transport does (preview digest bound into apply's header).
async function importAtRequestLevel(page: Page, base: string, request: Record<string, unknown>): Promise<AppliedBody> {
  const json = { "Content-Type": "application/json" };
  const body = JSON.stringify(request);
  const previewed = await page.request.post(at(base, "/design/import/preview"), { headers: json, data: body });
  expect(previewed.status(), await previewed.text()).toBe(200);
  const preview = await previewed.json();
  expect(preview.ready, JSON.stringify(preview.findings)).toBe(true);
  const applied = await page.request.post(at(base, "/design/import/apply"), {
    headers: { ...json, "X-Verdi-Import-Preview": preview.digest },
    data: body,
  });
  expect(applied.status(), await applied.text()).toBe(200);
  const result = (await applied.json()) as AppliedBody;
  expect(result.status).toBe("created");
  return result;
}

test.describe("import record page: format and profile primary digest", () => {
  test("an f13-reference-v1 record shows its format and the pinned primary digest", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const slug = "gatekeeper-record-facts";
    const sources = [SPEC_IMPORT_FILES.F13_PRIMARY, ...SPEC_IMPORT_FILES.F13_SUPPORTS].map(source);
    const mappings = Array.from({ length: 8 }, (_, i) => ({ target: `ac-${i + 1}`, evidence: ["attestation"] }));
    const created = await importAtRequestLevel(page, base, {
      schema: "verdi.spec-import-request/v1",
      target: { slug, class: "feature", title: "Bounded gatekeeper state machine" },
      format: "f13-reference-v1",
      primary: sources[0].id,
      sources,
      defer_statements: true,
      retain_unmapped: true,
      mappings,
    });

    const response = await page.goto(at(base, importRecordPath(created.branch, slug)));
    expect(response?.status()).toBe(200);
    const record = page.getByTestId("import-record");
    await expect(record).toHaveAttribute("data-import-commit", created.commit);
    await expect(record.getByTestId("record-format")).toHaveText("f13-reference-v1");
    await expect(record.getByTestId("record-profile-primary-digest")).toHaveText(SPEC_IMPORT_F13_PRIMARY_SHA256);
    // Both facts are rows of the Verified import block, labelled.
    await expect(record.locator("dt", { hasText: /^Format$/ })).toHaveCount(1);
    await expect(record.locator("dt", { hasText: /^Profile primary digest$/ })).toHaveCount(1);
  });

  test("a markdown-v1 record shows its format and states the profile digest as not applicable", async ({ page }) => {
    test.setTimeout(90_000);
    const base = await importBase(page);
    const slug = "labeled-record-facts";
    const labeled = source(SPEC_IMPORT_FILES.LABELED);
    const created = await importAtRequestLevel(page, base, {
      schema: "verdi.spec-import-request/v1",
      target: { slug, class: "feature", title: "Labeled record facts" },
      format: "markdown-v1",
      primary: labeled.id,
      sources: [labeled],
      defer_statements: false,
      retain_unmapped: true,
      mappings: [1, 2, 3].map((i) => ({ target: `ac-${i}`, evidence: ["attestation"] })),
    });

    const response = await page.goto(at(base, importRecordPath(created.branch, slug)));
    expect(response?.status()).toBe(200);
    const record = page.getByTestId("import-record");
    await expect(record.getByTestId("record-format")).toHaveText("markdown-v1");
    await expect(record.getByTestId("record-profile-primary-digest")).toHaveText("not applicable");
    // Never a blank cell and never a fabricated digest.
    await expect(record.getByTestId("record-profile-primary-digest")).not.toHaveText(/[0-9a-f]{64}/);
  });
});
