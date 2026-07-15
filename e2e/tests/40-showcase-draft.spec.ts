import { test, expect } from "@playwright/test";
import {
  SHOWCASE_DRAFT_SPEC,
  SHOWCASE_DRAFT_BRANCH,
  SHOWCASE_DRAFT_PROBLEM_SNIPPET,
  SHOWCASE_DRAFT_OUTCOME_SNIPPET,
  SHOWCASE_DRAFT_ACS,
  SHOWCASE_DRAFT_OQ_ID,
  SHOWCASE_DRAFT_OQ_CARRIED,
  SHOWCASE_DRAFT_OQ_RESOLVED,
  SHOWCASE_DRAFT_DIAGRAM,
  INSPECT_URL,
  boardPath,
  branchBoardPath,
  worktreeDiagramPath,
} from "./fixtures";

// The showcase live-draft feature (public rollout design §4.3: "one live
// draft on a design branch"). payoff-quote-portal is authored on
// design/payoff-quote-portal and never committed to main (VL-004), so its
// authoring board exists only under the /b/ per-branch address — served
// from the branch's pre-cut, seeded managed worktree
// (cmd/e2eharness/provision_showcase_draft.go). This is the draft-surface
// coverage the committed-draft deletion (Task 1.3) left unrestored: the
// canonical live draft a README reader is pointed at.

function draftBoard(): string {
  return branchBoardPath(SHOWCASE_DRAFT_BRANCH, SHOWCASE_DRAFT_SPEC);
}

async function worktreeFile(
  request: import("@playwright/test").APIRequestContext,
  path: string,
): Promise<{ status: number; body: string }> {
  const resp = await request.get(`${INSPECT_URL}/file?path=${encodeURIComponent(path)}`);
  return { status: resp.status(), body: resp.ok() ? await resp.text() : "" };
}

// The draft is a live authoring wall under /b/, with its full object model
// — problem/outcome placards and both acceptance-criteria cards — plus the
// authoring affordance. It is NOT on the serving checkout: its unprefixed
// address has nothing to serve.
test("the payoff-quote-portal draft is a live authoring wall under /b/", async ({ page }) => {
  const unprefixed = await page.request.get(boardPath(SHOWCASE_DRAFT_SPEC));
  expect(unprefixed.status()).toBe(404);

  await page.goto(draftBoard());
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
  await expect(page.getByTestId("placard-problem")).toContainText(SHOWCASE_DRAFT_PROBLEM_SNIPPET);
  await expect(page.getByTestId("placard-outcome")).toContainText(SHOWCASE_DRAFT_OUTCOME_SNIPPET);

  for (const ac of SHOWCASE_DRAFT_ACS) {
    await expect(page.getByTestId(`card-${ac}`)).toBeVisible();
  }
  // The authoring affordance is present (a draft on its design branch).
  await expect(page.getByRole("button", { name: "Add sticky" })).toBeVisible();
});

// VL-017's TWO legal paths, both showcased on one wall: the open question
// carried onto the spec as a declared open_questions object (rendered as an
// oq card, its text byte-identical to the still-open question sticky), and a
// question settled in place as a status:resolved sticky.
test("the wall showcases VL-017 both paths: a carried open question and a resolved sticky", async ({
  page,
}) => {
  await page.goto(draftBoard());

  // Carried path: the declared open_questions object renders as its card.
  await expect(page.getByTestId(`card-${SHOWCASE_DRAFT_OQ_ID}`)).toContainText(
    SHOWCASE_DRAFT_OQ_CARRIED,
  );

  // The still-open question sticky carries the same text (the annotation the
  // carried object formalizes), and a second sticky is resolved in place.
  const stickies = page.locator('[data-testid^="sticky-"]');
  await expect(
    stickies.filter({ hasText: SHOWCASE_DRAFT_OQ_CARRIED }),
  ).toHaveCount(1);
  const resolved = stickies.filter({ hasText: SHOWCASE_DRAFT_OQ_RESOLVED });
  await expect(resolved).toHaveCount(1);
  await expect(resolved).toHaveAttribute("data-annotation-type", "question");
});

// The proposal-tier diagram is authored on the branch: a class: proposal
// diagram whose derived_from pins a real corpus diagram (VL-021). Read it
// out of the branch's managed worktree through the inspection server.
test("a proposal-tier diagram is authored on the draft branch", async ({ page }) => {
  await page.goto(draftBoard()); // ensure the worktree is cut and seeded

  const diagram = await worktreeFile(
    page.request,
    worktreeDiagramPath(SHOWCASE_DRAFT_SPEC, SHOWCASE_DRAFT_DIAGRAM),
  );
  expect(diagram.status).toBe(200);
  expect(diagram.body).toContain("class: proposal");
  expect(diagram.body).toContain("derived_from:");
});
