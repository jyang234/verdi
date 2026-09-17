import { test, expect } from "@playwright/test";
import { SHOWCASE, INSPECT_URL, boardPath, branchBoardPath, refCardTestId } from "./fixtures";

// The board's Revise action (spec/uat-round-1 ac-11, board half; PLAN.md
// I-129 option (a), ratified 2026-09-17): the sealed accepted feature
// wall offers a Revise affordance that invokes the SAME supersede
// operation `verdi design start --supersedes` runs — the successor
// carries every object and stub verbatim as `carried`, links back with a
// `supersedes` edge, and starts as a draft on its own design branch —
// minus the CLI's checkout switch: the serving checkout never moves.
//
// SHOWCASE.FEATURE_SPEC is escrow-autopay on main (the sealed record; the
// same wall 31-board-stub-instantiate drives). Its successor lands on
// design/escrow-autopay-v2, which nothing else in the harness store
// carries (the pending-supersession MR of 16-dex-v2 lives only in the
// forge double, never in the store's own refs). The successor's board is
// reached through the per-branch address grammar (38-draft-boards), where
// the `supersedes` relation renders on the surface every whole-spec link
// already uses: a reference card for the predecessor plus a document-level
// yarn chip typed `supersedes`. State assertions ride testids, roles, and
// values; screenshot, trace, video, and recording stay off (co-4).

const successor = `${SHOWCASE.FEATURE_SPEC}-v2`;
const successorBranch = `design/${successor}`;

async function porcelain(request: import("@playwright/test").APIRequestContext) {
  const resp = await request.get(`${INSPECT_URL}/porcelain`);
  expect(resp.ok()).toBe(true);
  return (await resp.json()) as { branch: string; porcelain: string };
}

test.describe("board revise: the sealed wall starts a superseding revision", () => {
  test.describe.configure({ mode: "serial" });

  test("only the sealed accepted feature wall offers Revise", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    const button = page.getByTestId("revise-spec-btn");
    await expect(button).toBeVisible();
    await expect(button).toHaveText(/Revise this feature/);
    // The dialog is rendered (hidden) with its chrome.
    await expect(page.locator("#revise-dialog")).toBeAttached();
    await expect(page.locator("#revise-dialog")).toBeHidden();

    // A draft on its design branch (the authoring wall) offers nothing:
    // supersession is the forward path AFTER acceptance only.
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    await expect(page.getByTestId("revise-spec-btn")).toHaveCount(0);
    await expect(page.locator("#revise-dialog")).toHaveCount(0);

    // A story wall offers nothing either: supersession is feature-only
    // (02 §Kind registry).
    await page.goto(boardPath(SHOWCASE.STORY_STUB_MATCHED));
    await expect(page.getByTestId("board")).toBeVisible();
    await expect(page.getByTestId("revise-spec-btn")).toHaveCount(0);
    await expect(page.locator("#revise-dialog")).toHaveCount(0);
  });

  test("revise cuts the successor on its own branch, links its board, and leaves this wall unchanged", async ({
    page,
  }) => {
    const before = await porcelain(page.request);

    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await page.getByTestId("revise-spec-btn").click();
    const dialog = page.locator("#revise-dialog");
    await expect(dialog).toBeVisible();

    // The prefilled default is the next revision; the branch tab names the
    // identity submit will cut; the explanatory line says what lands.
    const name = page.getByTestId("revise-name");
    await expect(name).toHaveValue(successor);
    await expect(dialog.locator("#revise-branch-tab")).toHaveText(successorBranch);
    const explainer = page.getByTestId("revise-explainer");
    await expect(explainer).toContainText("carried");
    await expect(explainer).toContainText("supersedes");
    await expect(explainer).toContainText(successorBranch);
    await expect(explainer).toContainText("never moves");
    await expect(page.getByTestId("revise-error")).toBeHidden();

    await page.getByTestId("revise-ok").click();

    // The receipt: the success link points at the successor's own
    // per-branch board; the OK affordance retires (nothing to resubmit).
    const link = page.getByTestId("revise-success-link");
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute("href", branchBoardPath(successorBranch, successor));
    await expect(page.getByTestId("revise-success")).toContainText(successorBranch);
    await expect(page.getByTestId("revise-ok")).toBeHidden();
    await expect(page.getByTestId("revise-error")).toBeHidden();

    // The serving checkout did not move: same branch, byte-identical
    // porcelain (the successor was cut through plumbing, never checked
    // out here).
    const after = await porcelain(page.request);
    expect(after.branch).toBe(before.branch);
    expect(after.porcelain).toBe(before.porcelain);
    expect(after.porcelain).not.toContain(successor);

    // Following the link lands on the successor's board: a draft on its
    // own design branch (the authoring wall, no terminal badge), carrying
    // the predecessor's objects and the supersedes relation on the
    // board's own link surface — the predecessor's reference card and the
    // document-level yarn chip typed supersedes.
    await link.click();
    await expect(page).toHaveURL(new RegExp(`/b/design%2F${successor}/board/spec/${successor}$`));
    const board = page.getByTestId("board");
    await expect(board).toHaveAttribute("data-board-mode", "authoring");
    await expect(board).toHaveAttribute("data-spec", successor);
    await expect(page.getByTestId("board-status-badge")).toHaveCount(0);
    await expect(page.getByTestId(refCardTestId(`spec/${SHOWCASE.FEATURE_SPEC}`))).toBeVisible();
    await expect(
      page.locator(
        `.yarn-chip[data-edge-type="supersedes"][data-from="spec"][data-to="spec/${SHOWCASE.FEATURE_SPEC}"]`,
      ),
    ).toBeAttached();
    for (const ac of SHOWCASE.AC_IDS) {
      await expect(page.getByTestId(`card-${ac}`)).toBeVisible();
    }
    // The successor is not on the serving checkout's tree: its unprefixed
    // address has nothing to serve.
    const unprefixed = await page.request.get(boardPath(successor));
    expect(unprefixed.status()).toBe(404);

    // The ORIGINAL wall, reloaded, is unchanged: still the sealed record,
    // still offering Revise.
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    await expect(page.getByTestId("board-status-badge")).toHaveCount(0);
    await expect(page.getByTestId("revise-spec-btn")).toBeVisible();
  });

  test("an invalid successor name refuses in the dialog and creates nothing", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await page.getByTestId("revise-spec-btn").click();
    await page.getByTestId("revise-name").fill("Not Kebab");
    await page.getByTestId("revise-ok").click();

    // The server's own refusal, in the dialog's alert slot; the dialog
    // stays open with what was typed; no success link.
    const error = page.getByTestId("revise-error");
    await expect(error).toBeVisible();
    await expect(error).toHaveAttribute("role", "alert");
    await expect(error).toContainText("Not Kebab");
    await expect(error).toContainText("kebab-case");
    await expect(page.locator("#revise-dialog")).toBeVisible();
    await expect(page.getByTestId("revise-name")).toHaveValue("Not Kebab");
    await expect(page.getByTestId("revise-success-link")).toBeHidden();

    // Nothing was cut: no branch of that name resolves in the store.
    const gone = await page.request.get(branchBoardPath("design/Not Kebab", "Not Kebab"));
    expect(gone.status()).toBe(404);
  });

  test("a second revise under the same name is refused plainly: the branch already exists", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.FEATURE_SPEC));
    await page.getByTestId("revise-spec-btn").click();
    // Every open starts from the prefilled default again.
    await expect(page.getByTestId("revise-name")).toHaveValue(successor);
    await page.getByTestId("revise-ok").click();

    const error = page.getByTestId("revise-error");
    await expect(error).toBeVisible();
    await expect(error).toContainText(`branch ${successorBranch} already exists`);
    await expect(page.getByTestId("revise-success-link")).toBeHidden();
    await expect(page.getByTestId("revise-ok")).toBeVisible();

    // Cancel closes; the wall is untouched.
    await page.getByTestId("revise-cancel").click();
    await expect(page.locator("#revise-dialog")).toBeHidden();
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
  });
});
