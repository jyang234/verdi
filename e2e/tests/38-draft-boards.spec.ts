import { test, expect } from "@playwright/test";
import {
  SHOWCASE,
  INSPECT_URL,
  boardPath,
  branchBoardPath,
  worktreeSpecPath,
} from "./fixtures";
import { editCard } from "./helpers";

// Per-branch draft boards (spec/draft-boards): one address grammar,
// /b/<branch-escaped>/board/spec/<name>, reaches every draft's own design
// branch tree through a serve-managed worktree — the existing board,
// rooted per branch, under the unchanged mode law. The per-draft port
// pattern retires (dc-3): everything here runs against the ONE serve on
// :4173.
//
// The serving checkout sits on SHOWCASE.DESIGN_BRANCH (the board suite's authoring
// fixture) and earlier specs in this serial suite legitimately dirty its
// working tree with their own autosaves — so the clean-serving-checkout
// law (ac-2) is witnessed as INVARIANCE: the porcelain status before the
// two-tab exchange is byte-identical after it, and no draft-board path
// ever appears in it.

function draftBoard(name: string): string {
  return branchBoardPath(`design/${name}`, name);
}

async function porcelain(request: import("@playwright/test").APIRequestContext) {
  const resp = await request.get(`${INSPECT_URL}/porcelain`);
  expect(resp.ok()).toBe(true);
  return (await resp.json()) as { branch: string; porcelain: string };
}

async function worktreeFile(
  request: import("@playwright/test").APIRequestContext,
  path: string,
): Promise<{ status: number; body: string }> {
  const resp = await request.get(`${INSPECT_URL}/file?path=${encodeURIComponent(path)}`);
  return { status: resp.status(), body: resp.ok() ? await resp.text() : "" };
}

// AC-1: a draft opens under /b/ as its authoring wall, served from the
// design branch's managed worktree (the first open pays the lazy cut,
// dc-2 — within ordinary timeouts), and the board's sub-routes work
// beneath the prefix: a mutation through the prefixed api lands and the
// prefixed fragment re-renders it.
test("a draft under /b/ is its authoring wall and its sub-routes work beneath the prefix", async ({
  page,
}) => {
  // The spec is NOT on the serving checkout's tree: its unprefixed
  // address has nothing to serve — the content below can only come from
  // the design branch's own tree.
  const unprefixed = await page.request.get(boardPath(SHOWCASE.DB_TAB_A));
  expect(unprefixed.status()).toBe(404);

  await page.goto(draftBoard(SHOWCASE.DB_TAB_A)); // first open: the lazy worktree cut
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
  await expect(page.getByTestId("placard-problem")).toContainText("tab A problem");

  // A board mutation through the prefixed api route (editCard autosaves
  // via POST .../api/edit-text relative to the page's own mount).
  const edited = "criterion edited beneath the /b/ prefix";
  await editCard(page, "ac-1", () => edited);
  await expect(page.getByTestId("card-ac-1")).toContainText(edited);

  // The prefixed fragment route returns the re-rendered region
  // reflecting the mutation.
  const fragment = await page.request.get(`${draftBoard(SHOWCASE.DB_TAB_A)}/fragment`);
  expect(fragment.ok()).toBe(true);
  expect(await fragment.text()).toContain(edited);

  // And the edit landed in the managed worktree's own tree.
  const wt = await worktreeFile(page.request, worktreeSpecPath(SHOWCASE.DB_TAB_A, SHOWCASE.DB_TAB_A));
  expect(wt.status).toBe(200);
  expect(wt.body).toContain(edited);
});

// AC-2 — the two-tab isolation proof: two draft boards from two design
// branches are open and usable in two tabs SIMULTANEOUSLY; an authoring
// edit through each lands only in its own branch's managed worktree; the
// serving checkout's working tree is untouched by the whole exchange.
test("two draft boards in two tabs: edits are isolated and the serving checkout is undisturbed", async ({
  page,
  context,
}) => {
  const before = await porcelain(page.request);
  expect(before.branch).toBe(SHOWCASE.DESIGN_BRANCH);

  const tabA = page;
  const tabB = await context.newPage();
  await tabA.goto(draftBoard(SHOWCASE.DB_TAB_A));
  await tabB.goto(draftBoard(SHOWCASE.DB_TAB_B));
  await expect(tabA.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
  await expect(tabB.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");

  // Board B's rendered region before A's edit — the byte-for-byte baseline.
  const bBefore = await (await tabB.request.get(`${draftBoard(SHOWCASE.DB_TAB_B)}/fragment`)).text();

  // Edit through tab A while tab B stays open.
  const editA = "edited only in tab A (two-tab proof)";
  await editCard(tabA, "ac-1", () => editA);

  // Tab B, re-fetched, is byte-for-byte unaffected by A's edit...
  const bAfter = await (await tabB.request.get(`${draftBoard(SHOWCASE.DB_TAB_B)}/fragment`)).text();
  expect(bAfter).toBe(bBefore);

  // ...and stays USABLE, not merely visible: an edit through B succeeds
  // now, without reloading — simultaneous, not alternate.
  const editB = "edited only in tab B (two-tab proof)";
  await editCard(tabB, "ac-1", () => editB);
  await expect(tabB.getByTestId("card-ac-1")).toContainText(editB);
  await expect(tabA.getByTestId("card-ac-1")).toContainText(editA);

  // Each edit lives in its own branch's managed worktree only.
  const wtA = await worktreeFile(page.request, worktreeSpecPath(SHOWCASE.DB_TAB_A, SHOWCASE.DB_TAB_A));
  const wtB = await worktreeFile(page.request, worktreeSpecPath(SHOWCASE.DB_TAB_B, SHOWCASE.DB_TAB_B));
  expect(wtA.body).toContain(editA);
  expect(wtA.body).not.toContain(editB);
  expect(wtB.body).toContain(editB);
  expect(wtB.body).not.toContain(editA);

  // The serving checkout: same branch, and its working-tree status is
  // byte-identical to before the exchange — opening and editing drafts
  // never disturbs it (feature dc-1's no-surprise-mutation law). No
  // draft-board path ever appears in its porcelain.
  const after = await porcelain(page.request);
  expect(after.branch).toBe(SHOWCASE.DESIGN_BRANCH);
  expect(after.porcelain).toBe(before.porcelain);
  expect(after.porcelain).not.toContain(SHOWCASE.DB_TAB_A);
  expect(after.porcelain).not.toContain(SHOWCASE.DB_TAB_B);

  await tabB.close();
});

// AC-3: the mode law unchanged — the SAME spec renders as the sealed
// read-only record at its unprefixed (serving checkout) address and as an
// authoring wall at its design-branch /b/ address, both reachable in one
// session, neither toggling the other.
test("the same spec is sealed unprefixed and authoring under /b/, simultaneously", async ({
  page,
}) => {
  await page.goto(boardPath(SHOWCASE.DB_SAME_SPEC));
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
  await expect(page.locator(".board-mode-tag")).toHaveText("read-only · sealed record");
  await expect(page.locator("body")).not.toContainText(SHOWCASE.DB_SAME_SPEC_DRAFT_SNIPPET);
  // No authoring affordances on the sealed record.
  await expect(page.getByRole("button", { name: "Add sticky" })).toHaveCount(0);

  await page.goto(branchBoardPath(SHOWCASE.DB_SAME_SPEC_BRANCH, SHOWCASE.DB_SAME_SPEC));
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
  await expect(page.getByTestId("placard-outcome")).toContainText(SHOWCASE.DB_SAME_SPEC_DRAFT_SNIPPET);
  await expect(page.getByRole("button", { name: "Add sticky" })).toBeVisible();

  // Back at the unprefixed address: still the sealed record — two
  // simultaneous truths of one spec, not a toggle.
  await page.goto(boardPath(SHOWCASE.DB_SAME_SPEC));
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
  await expect(page.locator("body")).not.toContainText(SHOWCASE.DB_SAME_SPEC_DRAFT_SNIPPET);
});

// DC-4 (remote-tracking only): a /b/ branch that resolves only to a
// remote-tracking ref renders SEALED — read-only, remoteness disclosed in
// the board chrome, its content the ref's — with no worktree cut and no
// local branch minted (witnessed by the sealed render itself: a worktree
// would have made it an authoring wall).
test("a remote-only branch renders sealed with its remoteness disclosed", async ({ page }) => {
  await page.goto(branchBoardPath(`design/${SHOWCASE.DB_SEALED_REMOTE}`, SHOWCASE.DB_SEALED_REMOTE));
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
  await expect(page.locator(".board-mode-tag")).toHaveText("read-only · sealed record");
  await expect(page.getByTestId("board-notice")).toContainText(
    `remote-tracking ref origin/design/${SHOWCASE.DB_SEALED_REMOTE}`,
  );
  await expect(page.getByTestId("placard-problem")).toContainText("sealed remote problem");
  await expect(page.getByRole("button", { name: "Add sticky" })).toHaveCount(0);
});

// DC-4 (no ref at all): a /b/ branch that resolves nowhere renders the
// disclosed notice page — HTTP 404, a body naming the branch, a working
// way back — never a dead link, never a bare failure.
test("a branch with no ref at all is a disclosed 404 with a way back", async ({ page }) => {
  const resp = await page.goto(branchBoardPath("design/never-provisioned", "whatever"));
  expect(resp!.status()).toBe(404);

  const notice = page.getByTestId("stale-entry-notice");
  await expect(notice).toBeVisible();
  await expect(notice).toContainText("design/never-provisioned");

  await page.getByTestId("back-to-directory").click();
  await expect(page).toHaveURL(/\/$/);
});
