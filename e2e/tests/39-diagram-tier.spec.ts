import { test, expect, type Page, type Locator } from "@playwright/test";
import { SHOWCASE, DEX_BASE, dexSpecPath, boardPath } from "./fixtures";
import { pinArtifact } from "./helpers";

// spec/illustrative-class — the diagram-tier disclosure. Every mermaid
// render arrives from internal/render's ONE seam wrapped in a badged
// <figure data-diagram-tier=...>: a fenced body figure and a
// non-proposal diagram-kind artifact are illustrative (dc-2), badged
// "illustrative · not deterministically verifiable" (dc-1); a
// class: proposal carries the extractor-computed coverage tier instead
// and is NEVER painted illustrative (ac-2's negative case). One vendored
// pinned mermaid asset (the dex-embedded copy the workbench re-serves)
// renders every surface — no CDN, no network (co-2) — and no surface
// shows a body-figure diagram unbadged (ac-3, "never silently blended").

// -- hermeticity + vendored-asset accounting --------------------------------
//
// Each test routes ALL requests: anything bound for a non-loopback origin
// is aborted AND recorded, so a diagram renderer fetched from a CDN both
// breaks the render (the svg assertion fails) and fails the explicit
// zero-external-requests assertion (the ac-1 obligation: "a run that
// fetches any diagram renderer from a remote origin fails"). Loads of the
// vendored /assets/mermaid.min.js are recorded too, so each test can
// prove the SVG it saw came from the one pinned local asset.

let externalRequests: string[];
let mermaidAssetLoads: string[];

test.beforeEach(async ({ page }) => {
  externalRequests = [];
  mermaidAssetLoads = [];
  await page.route("**/*", (route) => {
    const url = new URL(route.request().url());
    if (url.hostname !== "127.0.0.1" && url.hostname !== "localhost") {
      externalRequests.push(url.href);
      return route.abort();
    }
    if (url.pathname === "/assets/mermaid.min.js") {
      mermaidAssetLoads.push(url.href);
    }
    return route.continue();
  });
});

test.afterEach(() => {
  expect(externalRequests, "the suite is network-free (co-2)").toEqual([]);
});

// The badged illustrative figure: the SAME figure carries the
// machine-readable tier marker, the client-rendered SVG, and the visible
// figcaption chip — the badge sits on the diagram, not elsewhere on the
// page (the ac-2 obligation's same-figure requirement).
async function expectIllustrativeFigure(scope: Page | Locator) {
  const figure = (scope as Page | Locator).locator(SHOWCASE.ILLUSTRATIVE_FIGURE).first();
  await expect(figure).toBeVisible();
  // mermaid swaps the pre's text for an <svg> asynchronously — poll.
  await expect(figure.locator("pre.mermaid svg")).toBeVisible({
    timeout: 10_000,
  });
  await expect(figure.locator("figcaption.diagram-tier-badge")).toHaveText(
    SHOWCASE.ILLUSTRATIVE_CHIP,
  );
}

// The ac-3 sweep: every <pre class="mermaid"> anywhere on the exercised
// page sits inside a badged figure (data-diagram-tier present, either
// tier) — no body-figure diagram renders unbadged.
async function expectNoUnbadgedMermaid(page: Page) {
  const orphans = await page.$$eval(
    "pre.mermaid",
    (els) =>
      els.filter((el) => !el.closest("figure[data-diagram-tier]")).length,
  );
  expect(orphans, "every mermaid pre sits inside a badged figure").toBe(0);
}

function expectVendoredAssetLoaded(origin: string) {
  expect(
    mermaidAssetLoads,
    `the renderer is the vendored asset served by ${origin}`,
  ).toContain(`${origin}/assets/mermaid.min.js`);
}

// ---------------------------------------------------------------------------
// Dex — the static site (ac-1, ac-2)
// ---------------------------------------------------------------------------

test("dex spec page: the fenced body figure renders to an SVG inside the illustrative badged figure", async ({
  page,
}) => {
  await page.goto(dexSpecPath(SHOWCASE.MERMAID_SPEC));
  await expectIllustrativeFigure(page);
  expectVendoredAssetLoaded(DEX_BASE);
  await expectNoUnbadgedMermaid(page);
});

test("dex diagram page: a non-proposal diagram-kind artifact wears the illustrative badge", async ({
  page,
}) => {
  await page.goto(`${DEX_BASE}/a/${SHOWCASE.ILLUSTRATIVE_DIAGRAM}/`);
  await expectIllustrativeFigure(page);
  expectVendoredAssetLoaded(DEX_BASE);
  await expectNoUnbadgedMermaid(page);
});

test("dex proposal page: the extractor-computed tier, never the illustrative badge — and the tiers are distinct", async ({
  page,
}) => {
  // The illustrative tier's marker, read for the distinctness assertion.
  await page.goto(`${DEX_BASE}/a/${SHOWCASE.ILLUSTRATIVE_DIAGRAM}/`);
  const illustrativeTier = await page
    .locator("figure[data-diagram-tier]")
    .first()
    .getAttribute("data-diagram-tier");

  // The proposal's page: the extractor-computed coverage tier (its body
  // sits inside the declared grammar → "full", computed by
  // internal/diagramverify, never re-derived client-side), rendered to a
  // real SVG under the same vendored asset, wearing its own visible chip.
  await page.goto(`${DEX_BASE}/a/${SHOWCASE.PROPOSAL_DIAGRAM}/`);
  const figure = page.locator(SHOWCASE.PROPOSAL_FULL_FIGURE).first();
  await expect(figure).toBeVisible();
  await expect(figure.locator("pre.mermaid svg")).toBeVisible({
    timeout: 10_000,
  });
  await expect(figure.locator("figcaption.diagram-tier-badge")).toHaveText(
    "proposal · full coverage",
  );

  // ac-2's negative case: the proposal render path is NEVER painted with
  // the illustrative badge or marker — anywhere on its page.
  await expect(page.locator("main")).not.toContainText("illustrative");
  await expect(page.locator(SHOWCASE.ILLUSTRATIVE_FIGURE)).toHaveCount(0);

  // ac-3 distinctness: two different, non-empty DOM tier markers — a
  // regression that collapses the tiers fails here, never silently.
  const proposalTier = await figure.getAttribute("data-diagram-tier");
  expect(illustrativeTier).toBe("illustrative");
  expect(proposalTier).toBe("full");
  expect(proposalTier).not.toBe(illustrativeTier);

  expectVendoredAssetLoaded(DEX_BASE);
  await expectNoUnbadgedMermaid(page);
});

// ---------------------------------------------------------------------------
// Workbench — the board's spec-body surfaces (ac-1, ac-2, ac-3)
// ---------------------------------------------------------------------------

test("workbench corpus page: the same fixture spec renders the badged SVG under the re-served vendored asset", async ({
  page,
  baseURL,
}) => {
  await page.goto(`/a/${SHOWCASE.MERMAID_SPEC_REF}`);
  await expectIllustrativeFigure(page);
  expectVendoredAssetLoaded(baseURL!);
  await expectNoUnbadgedMermaid(page);
});

test("board placard body dialog: the outcome section's body figure renders badged inside the dialog", async ({
  page,
  baseURL,
}) => {
  await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
  await expect(page.getByTestId("board")).toHaveAttribute(
    "data-board-mode",
    "authoring",
  );

  // SHOWCASE.DESIGN_SPEC's `## Outcome` body carries the fixture's fenced mermaid
  // block (cmd/e2eharness/provision_board.go). Opening the placard's
  // expand dialog injects the server-rendered body — the badge arrived in
  // that HTML from internal/render's seam; the client only hands the pre
  // to the lazily-loaded vendored asset (dc-1: no client-side badge
  // computation).
  await page.getByTestId("placard-outcome").locator(".placard-text").click();
  const dialog = page.getByTestId("expand-dialog");
  await expect(dialog).toBeVisible();
  await expectIllustrativeFigure(dialog);
  expectVendoredAssetLoaded(baseURL!);
  await expectNoUnbadgedMermaid(page);
});

test("board reference peek: both tiers peek badged, distinct, never blended", async ({
  page,
  baseURL,
}) => {
  await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
  await expect(page.getByTestId("board")).toHaveAttribute(
    "data-board-mode",
    "authoring",
  );
  const peek = page.getByTestId("ref-peek");

  // The illustrative tier through the peek: pin the fenced-body fixture
  // spec and peek it — the injected fragment carries the badged figure
  // and the vendored asset turns it into an SVG.
  const specCard = await pinArtifact(page, SHOWCASE.MERMAID_SPEC_REF, "mermaid");
  await specCard.click();
  await expect(peek).toBeVisible();
  await expectIllustrativeFigure(peek);
  await page.keyboard.press("Escape");
  await expect(peek).toHaveCount(0);

  // The verified tier through the same surface: peek the class: proposal
  // diagram — the extractor-computed tier marker, its own chip, and no
  // illustrative paint anywhere in the fragment (ac-2's negative case).
  const proposalCard = await pinArtifact(page, SHOWCASE.PROPOSAL_DIAGRAM, "future");
  await proposalCard.click();
  await expect(peek).toBeVisible();
  const proposalFigure = peek.locator(SHOWCASE.PROPOSAL_FULL_FIGURE).first();
  await expect(proposalFigure).toBeVisible();
  await expect(proposalFigure.locator("pre.mermaid svg")).toBeVisible({
    timeout: 10_000,
  });
  await expect(
    proposalFigure.locator("figcaption.diagram-tier-badge"),
  ).toHaveText("proposal · full coverage");
  await expect(peek.locator(SHOWCASE.ILLUSTRATIVE_FIGURE)).toHaveCount(0);
  await expect(peek.locator(".peek-body")).not.toContainText("illustrative");

  expectVendoredAssetLoaded(baseURL!);
  await expectNoUnbadgedMermaid(page);
});
