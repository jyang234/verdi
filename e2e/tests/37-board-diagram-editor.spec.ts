import { test, expect, type Page } from "@playwright/test";
import { SHOWCASE, EDGE, boardPath, diagramEditorPath } from "./fixtures";
import { dragToTrash, expectAutosaved, pinArtifact } from "./helpers";

// The diagram-proposal editor (spec/board-editor): code pane + live
// preview under the ONE vendored mermaid asset, structural operations as
// deterministic source-text edits, byte preservation through the full
// HTTP-write-to-disk-to-reload loop, mechanical before-peek/reset, and
// the consuming verification rail. The suite runs hermetically: the
// preview renders under /assets/mermaid.min.js only (co-4 — the harness
// has no network), and the rail's report is the harness's canned file.

function pane(page: Page) {
  return page.getByTestId("diagram-source");
}
function preview(page: Page) {
  return page.locator("#diagram-preview");
}
function previewNode(page: Page, id: string) {
  return page.locator(`#diagram-preview g.node[data-node-id="${id}"]`);
}

async function openEditor(page: Page, name: string) {
  await page.goto(diagramEditorPath(name));
  await expect(page.getByTestId("diagram-editor")).toBeVisible();
}

// Set the pane's exact content and wait for the SAVE ROUND-TRIP (not
// just the status text, which could still read "saved" from a previous
// action): the response is the write's receipt.
async function setPane(page: Page, name: string, text: string) {
  const saved = page.waitForResponse(
    (r) => r.url().includes(`/board/diagram/${name}/api/save`) && r.ok(),
  );
  await pane(page).fill(text);
  await saved;
}

test.describe("diagram editor: the drafting surface (ac-1)", () => {
  test("the pane holds the artifact's source and valid source renders an SVG under the vendored asset", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_PROPOSAL_BODY);
    await expect(preview(page).locator("svg")).toBeVisible();
    // The gesture layer's stable hooks are annotated onto the picture.
    await expect(previewNode(page, "loansvc")).toBeVisible();
    await expect(page.getByTestId("diagram-render-error")).toBeHidden();
  });

  test("rejected source paints the renderer's own error; the SVG is not retained; the pane is untouched", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    await expect(preview(page).locator("svg")).toBeVisible();

    const invalid = "flowchart TD\n  a --> --> b\n";
    await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, invalid);

    const errorBox = page.getByTestId("diagram-render-error");
    await expect(errorBox).toBeVisible();
    // The renderer's OWN message, not a house-made one.
    await expect(errorBox.locator(".diagram-render-error-msg")).toContainText(
      /parse error|expecting|syntax/i,
    );
    // Never a silently retained last-good picture.
    await expect(preview(page).locator("svg")).toHaveCount(0);
    // The pane text is exactly what was typed.
    await expect(pane(page)).toHaveValue(invalid);

    // Restore the fixture body for the suite's later journeys; the
    // error state clears the moment the source renders again.
    await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, SHOWCASE.DIAGRAM_PROPOSAL_BODY);
    await expect(errorBox).toBeHidden();
    await expect(preview(page).locator("svg")).toBeVisible();
  });
});

test.describe("diagram editor: the verification rail consumes (ac-5)", () => {
  test("a canned extractor report renders verbatim: tier and per-element findings", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    const rail = page.getByTestId("verification-rail");
    await expect(rail).toBeVisible();
    await expect(page.getByTestId("verification-tier")).toHaveAttribute(
      "data-tier",
      SHOWCASE.DIAGRAM_RAIL_TIER,
    );
    for (const [identity, kind] of SHOWCASE.DIAGRAM_RAIL_FINDINGS) {
      const finding = page.getByTestId(`finding-${identity}`);
      await expect(finding).toBeVisible();
      await expect(finding).toHaveAttribute("data-finding-kind", kind);
    }
    // The contradicted finding's candidate witness rides along, spoken
    // as a candidate (verification-extractor dc-4's candor).
    await expect(page.getByTestId("finding-audit-log")).toContainText(
      "candidate witness",
    );
  });

  test("without a report the rail discloses verification-unavailable — and neither state blocks a save", async ({
    page,
  }) => {
    // The out-of-subset fixture has no entry in the canned file: the
    // rail must disclose, never render an empty region or invent a tier.
    await openEditor(page, EDGE.DIAGRAM_OUTSIDE_OPS);
    const unavailable = page.getByTestId("verification-unavailable");
    await expect(unavailable).toBeVisible();
    await expect(unavailable).toContainText("verification unavailable");
    await expect(page.getByTestId("verification-tier")).toHaveCount(0);

    // Editing and saving succeed in the unavailable state...
    const edited = "sequenceDiagram\n  Applicant->>LoanSvc: apply\n  LoanSvc->>Applicant: decline (edited)\n";
    await setPane(page, EDGE.DIAGRAM_OUTSIDE_OPS, edited);
    await expectAutosaved(page);
    await page.reload();
    await expect(pane(page)).toHaveValue(edited);

    // ...and identically in the with-report state.
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, SHOWCASE.DIAGRAM_PROPOSAL_BODY + "%% rail never blocks\n");
    await expectAutosaved(page);
    await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, SHOWCASE.DIAGRAM_PROPOSAL_BODY);
  });
});

test.describe("diagram editor: structural operations (ac-2)", () => {
  test.beforeEach(async ({ page }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    // Make each journey a pure function of the fixture body, whatever an
    // earlier test wrote.
    if ((await pane(page).inputValue()) !== SHOWCASE.DIAGRAM_PROPOSAL_BODY) {
      await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, SHOWCASE.DIAGRAM_PROPOSAL_BODY);
    }
    await expect(preview(page).locator("svg")).toBeVisible();
  });

  test("add node appends one n<k> line; the pane text is the witness", async ({
    page,
  }) => {
    await page.getByTestId("add-node-btn").click();
    await page.locator("#add-node-label").fill("Rate limiter");
    await page.locator("#add-node-ok").click();
    await expect(pane(page)).toHaveValue(
      SHOWCASE.DIAGRAM_PROPOSAL_BODY + '  n1["Rate limiter"]\n',
    );
    // The new node joins the picture and the gesture layer.
    await expect(previewNode(page, "n1")).toBeVisible();
  });

  test("connect via click-click appends one edge line", async ({ page }) => {
    await previewNode(page, "billing").click();
    await expect(page.getByTestId("node-toolbox")).toBeVisible();
    await previewNode(page, "loansvc").click();
    await expect(pane(page)).toHaveValue(
      SHOWCASE.DIAGRAM_PROPOSAL_BODY + "  billing --> loansvc\n",
    );
  });

  test("connect via drag-to-connect appends one edge line and stores nothing spatial", async ({
    page,
  }) => {
    const from = await previewNode(page, "loansvc").boundingBox();
    const to = await previewNode(page, "billing").boundingBox();
    expect(from).not.toBeNull();
    expect(to).not.toBeNull();
    await page.mouse.move(from!.x + from!.width / 2, from!.y + from!.height / 2);
    await page.mouse.down();
    await page.mouse.move(to!.x + to!.width / 2, to!.y + to!.height / 2, {
      steps: 10,
    });
    await page.mouse.up();
    // The drag produced EXACTLY one appended edge line: the whole
    // document is asserted, so nothing spatial (and nothing else) can
    // have landed anywhere (co-2).
    await expect(pane(page)).toHaveValue(
      SHOWCASE.DIAGRAM_PROPOSAL_BODY + "  loansvc --> billing\n",
    );
  });

  test("a drag that connects nothing produces nothing", async ({ page }) => {
    const from = await previewNode(page, "loansvc").boundingBox();
    const stage = await preview(page).boundingBox();
    expect(from).not.toBeNull();
    expect(stage).not.toBeNull();
    await page.mouse.move(from!.x + from!.width / 2, from!.y + from!.height / 2);
    await page.mouse.down();
    // Release over empty stage paper, on no node.
    await page.mouse.move(stage!.x + stage!.width - 8, stage!.y + 8, { steps: 8 });
    await page.mouse.up();
    // Give any wrong-doing round-trip a moment to land, then assert
    // the document did not move.
    await page.waitForTimeout(400);
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_PROPOSAL_BODY);
  });

  test("rename inline rewrites only the label; the id is byte-identical", async ({
    page,
  }) => {
    await previewNode(page, "loansvc").click();
    await page.getByTestId("rename-node-btn").click();
    const input = page.getByTestId("rename-input");
    await expect(input).toBeVisible();
    await input.fill("Loan orchestrator");
    await input.press("Enter");
    await expect(pane(page)).toHaveValue(
      SHOWCASE.DIAGRAM_PROPOSAL_BODY.replace(
        'loansvc["Loan service"]',
        'loansvc["Loan orchestrator"]',
      ),
    );
  });

  test("delete removes the node's defining line and every edge line naming it", async ({
    page,
  }) => {
    await previewNode(page, "billing").click();
    await page.getByTestId("delete-node-btn").click();
    await expect(pane(page)).toHaveValue(
      'flowchart TD\n  loansvc["Loan service"]\n  %% drafted on the wall\n',
    );
  });

  test("delete edge removes that one line", async ({ page }) => {
    // The wide transparent hit twin is the edge's gesture surface — a
    // hairline curve is unclickable for a hand and a test alike.
    const edge = page.locator(
      '#diagram-preview path.diagram-edge-hit[data-from="loansvc"][data-to="billing"]',
    );
    await expect(edge).toHaveCount(1);
    await edge.click({ force: true });
    await page.getByTestId("delete-edge-btn").click();
    await expect(pane(page)).toHaveValue(
      SHOWCASE.DIAGRAM_PROPOSAL_BODY.replace("  loansvc --> billing\n", ""),
    );
  });

  test("outside the flowchart subset the ops are disclosed unavailable while the pane stays live", async ({
    page,
  }) => {
    await openEditor(page, EDGE.DIAGRAM_OUTSIDE_OPS);
    const disclosure = page.getByTestId("ops-unavailable");
    await expect(disclosure).toBeVisible();
    await expect(disclosure).toContainText("structural operations are unavailable");
    await expect(page.getByTestId("add-node-btn")).toBeDisabled();

    // The code pane stays fully live: typing edits and saves.
    const edited =
      "sequenceDiagram\n  Applicant->>LoanSvc: apply\n  LoanSvc->>Applicant: still editable\n";
    await setPane(page, EDGE.DIAGRAM_OUTSIDE_OPS, edited);
    await page.reload();
    await expect(pane(page)).toHaveValue(edited);
  });
});

test.describe("diagram editor: byte preservation through the page (ac-3)", () => {
  // Deliberately unusual, renderer-legal formatting: mixed indentation,
  // %% comments, blank lines, trailing spaces.
  const adversarial =
    "flowchart TD\n" +
    '  keeper["Trailing spaces "]   \n' +
    "\tmixed\n" +
    "\n" +
    "  %% a comment with trailing blanks   \n" +
    "      keeper --> mixed\n";

  test("a pasted diagram survives the save-reload loop bit-identical, and an op changes only its own lines", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, adversarial);
    await page.reload();
    // Bit-identical through HTTP write → disk → reload — toHaveValue is
    // an exact string equality, never a trimmed comparison.
    await expect(pane(page)).toHaveValue(adversarial);

    // One structural operation on the pasted source...
    await page.getByTestId("add-node-btn").click();
    await page.locator("#add-node-label").fill("Auditor");
    await page.locator("#add-node-ok").click();
    await expect(pane(page)).toHaveValue(adversarial + '  n1["Auditor"]\n');

    // ...and the RELOADED document differs from the pre-op text only on
    // the op's own line: every pre-existing byte is still in place.
    await page.reload();
    await expect(pane(page)).toHaveValue(adversarial + '  n1["Auditor"]\n');

    await setPane(page, SHOWCASE.DIAGRAM_PROPOSAL, SHOWCASE.DIAGRAM_PROPOSAL_BODY);
  });
});

test.describe("diagram editor: before-peek and reset (ac-4)", () => {
  test("before-peek renders the pinned base read-only beside the working preview, modifying nothing", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_DERIVED);
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_DERIVED_BODY);
    await page.getByTestId("peek-btn").click();
    const peek = page.getByTestId("peek-panel");
    await expect(peek).toBeVisible();
    await expect(peek.locator("svg")).toBeVisible();
    await expect(page.getByTestId("peek-failure")).toBeHidden();
    // The working preview and the pane are untouched; so is the disk
    // (proven through the page by the reload).
    await expect(preview(page).locator("svg")).toBeVisible();
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_DERIVED_BODY);
    await page.reload();
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_DERIVED_BODY);
  });

  test("reset replaces the working source with the base byte-for-byte through the ordinary save", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_DERIVED);
    await page.getByTestId("reset-btn").click();
    const confirm = page.locator("#reset-confirm");
    await expect(confirm).toBeVisible();
    await confirm.locator("#reset-confirm-ok").click();
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_BASE_BODY);
    // The reloaded artifact body equals it: the write landed on disk.
    await page.reload();
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_BASE_BODY);
  });

  test("a corrupted digest fails visible on both affordances and writes nothing", async ({
    page,
  }) => {
    await openEditor(page, EDGE.DIAGRAM_DERIVED_CORRUPT);
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_DERIVED_BODY);

    await page.getByTestId("peek-btn").click();
    const failure = page.getByTestId("peek-failure");
    await expect(failure).toBeVisible();
    await expect(failure).toContainText(/digest mismatch/i);
    await expect(page.getByTestId("peek-panel").locator("svg")).toHaveCount(0);

    await page.getByTestId("reset-btn").click();
    await page.locator("#reset-confirm-ok").click();
    await expect(failure).toBeVisible();
    await expect(failure).toContainText(/digest mismatch/i);

    // The artifact on disk is byte-identical before and after the
    // attempts — read back through the page.
    await page.reload();
    await expect(pane(page)).toHaveValue(SHOWCASE.DIAGRAM_DERIVED_BODY);
  });

  test("a from-scratch proposal does not offer the affordances at all", async ({
    page,
  }) => {
    await openEditor(page, SHOWCASE.DIAGRAM_PROPOSAL);
    await expect(page.getByTestId("peek-btn")).toHaveCount(0);
    await expect(page.getByTestId("reset-btn")).toHaveCount(0);
    await expect(page.getByTestId("peek-panel")).toHaveCount(0);
  });
});

test.describe("diagram editor: reachability (dc-1)", () => {
  test("the corpus page of a proposal links into the editor", async ({ page }) => {
    await page.goto(`/a/diagram/${SHOWCASE.DIAGRAM_PROPOSAL}`);
    const link = page.getByTestId("open-editor-link");
    await expect(link).toBeVisible();
    await link.click();
    await expect(page).toHaveURL(new RegExp(diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL)));
    await expect(page.getByTestId("diagram-editor")).toBeVisible();
  });

  test("a spec board's pinned diagram reference card opens the editor", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
    const ref = `diagram/${SHOWCASE.DIAGRAM_PROPOSAL}`;
    const card = await pinArtifact(page, ref, SHOWCASE.DIAGRAM_PROPOSAL);
    const editorLink = card.getByTestId("refcard-editor-link");
    await expect(editorLink).toBeVisible();
    await editorLink.click();
    await expect(page).toHaveURL(new RegExp(diagramEditorPath(SHOWCASE.DIAGRAM_PROPOSAL)));
    await expect(page.getByTestId("diagram-editor")).toBeVisible();

    // Housekeeping: the pin was this journey's scaffolding — a pure pin
    // dies from the trash without ceremony, restoring the wall.
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await dragToTrash(page, page.locator(`.refcard[data-ref="${ref}"]`));
    await expectAutosaved(page);
    await expect(page.locator(`.refcard[data-ref="${ref}"]`)).toHaveCount(0);
  });
});
