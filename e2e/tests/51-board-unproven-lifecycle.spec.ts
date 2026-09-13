import { test, expect, type Locator, type Page } from "@playwright/test";
import { SHOWCASE, EDGE, CONTROL_URL, boardPath } from "./fixtures";

// MVP release amendment R2 (docs/superpowers/plans/2026-08-29-wave-6-
// workbench-presentation.md, 2026-09-13) under merge-signaled acceptance
// AC-7 ("Missing default-branch or ancestry evidence produces an explicit
// unproven result, never an assumed acceptance"): a read-only wall speaks
// WHY it is read-only from the spec's effective lifecycle, never from the
// CSS mode. The unproven wall — no resolvable default branch, the baseline
// report's B-06 — stays read-only, discloses the missing witness and its
// remedy, and never asserts acceptance or sealing; the proven accepted and
// closed records keep their sealed labels.
//
// The unproven fixture is the REAL workbench handler over a REAL no-remote
// store (cmd/e2eharness/unprovenboard.go), discovered through the control
// server — never a canned page, and never a mutation of the shared store,
// whose default branch every other suite needs to stay provable. State
// assertions ride data attributes and the stamp's own text.

const position = (el: Locator) =>
  el.evaluate((node) => ({
    left: (node as HTMLElement).style.left,
    top: (node as HTMLElement).style.top,
  }));

async function unprovenBoardURL(page: Page): Promise<string> {
  const res = await page.request.get(`${CONTROL_URL}/unproven-board-fixture`);
  expect(res.ok()).toBe(true);
  const isolatedURL = (await res.text()).trim();
  expect(isolatedURL).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/$/);
  return `${isolatedURL}board/spec/${EDGE.UNPROVEN_BOARD_SPEC}`;
}

test.describe("board lifecycle labels: unproven is never the sealed record", () => {
  test("an unproven lifecycle renders read-only, discloses witness and remedy, and asserts neither acceptance nor sealing", async ({
    page,
  }) => {
    const url = await unprovenBoardURL(page);
    await page.goto(url);
    const board = page.getByTestId("board");
    await expect(board).toHaveAttribute("data-board-mode", "readonly");
    await expect(board).toHaveAttribute("data-readonly-reason", "unproven");
    await expect(page.locator("body")).toHaveClass(/mode-readonly/);

    // The stamp and the rail name the unproven lifecycle — never sealed.
    await expect(page.locator(".board-mode-tag")).toHaveText("read-only · lifecycle unproven");
    await expect(page.locator(".sealed-panel")).toHaveCount(0);
    const panel = page.getByTestId("readonly-panel");
    await expect(panel).toBeVisible();
    await expect(panel).toHaveAttribute("data-readonly-reason", "unproven");
    // Three-valued: the view cannot claim acceptance or sealing — and it
    // does not assert the opposite either (missing proof proves no negative).
    await expect(panel).toContainText("cannot claim acceptance or sealing");
    await expect(panel).not.toContainText("neither accepted nor sealed");
    await expect(page.locator("body")).not.toContainText("This spec is accepted");

    // The missing witness in the board chrome; the panel's own remedy is
    // the local one (fetch the configured default branch, point
    // origin/HEAD at it, reload) — never a CI-environment workaround; the
    // posture header's formal state agrees.
    const notices = page.getByTestId("board-notice");
    await expect(notices.filter({ hasText: "default branch could not be resolved" })).toHaveCount(1);
    await expect(notices.filter({ hasText: "point origin/HEAD at it" })).toHaveCount(1);
    await expect(notices.filter({ hasText: "set CI_DEFAULT_BRANCH" })).toHaveCount(0);
    await expect(panel).toContainText("git remote set-head origin");
    await expect(panel).not.toContainText("CI_DEFAULT_BRANCH");
    const bytes = page.getByTestId("asd-posture-bytes");
    await expect(bytes).toHaveAttribute("data-state", "unproven");
    await expect(bytes).toContainText("displayed bytes: unproven");

    // No editing affordance of any tier.
    await expect(page.getByRole("button", { name: "Add sticky" })).toHaveCount(0);
    await expect(page.getByRole("button", { name: "Commit & push" })).toHaveCount(0);
    await expect(page.getByTestId("asd-forms")).toHaveCount(0);
    await expect(page.getByTestId("create-panel")).toHaveCount(0);
    await expect(page.locator(".delete-btn")).toHaveCount(0);

    // A drag is refused visibly, in the unproven wall's own words — never
    // "the accepted spec" — and nothing moves.
    const card = page.getByTestId("card-ac-1");
    const before = await position(card);
    const box = await card.boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
    await page.mouse.down();
    await page.mouse.move(box!.x + 220, box!.y + 180, { steps: 6 });
    await page.mouse.up();
    const refusal = page.getByTestId("drag-refusal");
    await expect(refusal).toBeVisible();
    await expect(refusal).toHaveText(/acceptance is unproven/);
    await expect(refusal).toHaveText(/cannot claim acceptance or sealing/);
    await expect(refusal).not.toHaveText(/accepted spec/);
    expect(await position(card)).toEqual(before);

    // The mutation boundary holds on both tiers, through the shipped
    // binary's wired design bridge (the fixture is a real `verdi serve`):
    // the domain surface refuses a typed write with the read-only gate's
    // own 403 diagnostic (never the unwired 500) — the gate answers before
    // any identity check, so the expected identity is a syntactically
    // valid placeholder — and the scratch tier refuses a sticky the same way.
    const snap = await (await page.request.get(url + "/snapshot")).json();
    const mutate = await page.request.post(url + "/api/mutate_draft", {
      data: {
        request: {
          schema: "verdi.draftmutation/v1",
          spec: "spec/" + EDGE.UNPROVEN_BOARD_SPEC,
          base_digest: snap.base_digest,
          base_spec_b64: snap.base_spec_b64,
          expected: {
            checkout: "/never-reached",
            branch: "main",
            head: "0000000000000000000000000000000000000000",
          },
          operations: [{ op: "set-problem", text: "x", anchor: "#problem" }],
        },
      },
    });
    expect(mutate.status(), await mutate.text()).toBe(403);
    expect(await mutate.text()).toContain("readonly mode");
    const sticky = await page.request.post(url + "/api/sticky", {
      data: { text: "unproven wall", type: "comment" },
    });
    expect(sticky.status(), await sticky.text()).toBe(403);
    expect(await sticky.text()).toContain("readonly mode");
    // No-mutation witness: the re-fetched projection is byte-identical to
    // the snapshot taken before the refused writes.
    const after = await (await page.request.get(url + "/snapshot")).json();
    expect(after.base_digest).toBe(snap.base_digest);
    expect(after.base_spec_b64).toBe(snap.base_spec_b64);
  });

  test("the proven accepted and closed records keep the sealed label", async ({ page }) => {
    // The accepted-pending-build feature: the sealed record, unchanged.
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    await expect(page.getByTestId("board")).toHaveAttribute("data-readonly-reason", "sealed");
    await expect(page.locator(".board-mode-tag")).toHaveText("read-only · sealed record");
    await expect(page.locator(".sealed-panel")).toContainText("accepted");
    await expect(page.getByTestId("readonly-panel")).toHaveCount(0);

    // A closed feature still in specs/active/ (its legacy terminal status
    // landed on main): proven closed, so the sealed record too.
    await page.goto(boardPath(EDGE.DIR_CLOSED_AWAITING_ARCHIVE));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    await expect(page.getByTestId("board")).toHaveAttribute("data-readonly-reason", "sealed");
    await expect(page.locator(".board-mode-tag")).toHaveText("read-only · sealed record");
    await expect(page.getByTestId("readonly-panel")).toHaveCount(0);
  });
});
