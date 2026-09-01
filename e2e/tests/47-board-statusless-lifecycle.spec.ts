import { test, expect } from "@playwright/test";
import { EDGE, boardPath, dirEntryTestId } from "./fixtures";
import { addSticky, editCard, uncommittedIndicator } from "./helpers";

// EXECUTABLE ACCEPTANCE — merge-signaled spec acceptance, workbench half
// (2026-08-01 design, Task 6 step 3): a spec's board mode and displayed
// status derive from its git-derived EFFECTIVE lifecycle state, never
// from a persisted `status:` field. The CLI's scaffold now writes NO
// status field at all, so the pre-migration mode rule
// (`status == "draft" && branch != default`) rendered every fresh
// scaffold read-only and 403'd every board write — breaking the
// advertised design-start → serve → board-edit flow. These journeys pin
// the fixed behavior on the STATUSLESS shape the CLI actually produces:
//   - proposed (bytes only on the design branch)  → live authoring wall;
//   - accepted-pending-build (exact bytes on main) → sealed read-only.
test.describe("statusless lifecycle (merge-signaled acceptance)", () => {
  test("a statusless draft on its design branch is a live authoring wall: a sticky lands and survives reload", async ({
    page,
  }) => {
    // Wave 6 Task 2: typed draft mutations ride the shared core, whose
    // mutable-branch rule is design/<spec-name> exactly (AC-2's identical
    // semantics). The statusless draft's own namesake branch board is the
    // editable wall; the serving checkout's copy remains a projection.
    await page.goto(
      "/b/" +
        encodeURIComponent("design/" + EDGE.STATUSLESS_DRAFT_SPEC) +
        "/board/spec/" +
        EDGE.STATUSLESS_DRAFT_SPEC,
    );
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const text = "statusless wall: sticky lands and survives";
    const sticky = await addSticky(page, text, "question");
    await expect(sticky).toHaveAttribute("data-annotation-type", "question");

    // Durable (the mutable-zone annotation stream), not page state.
    await page.reload();
    await expect(
      page.locator('[data-testid^="sticky-"]').filter({ hasText: text }),
    ).toHaveCount(1);
  });

  test("a statusless draft accepts a real spec edit (the design-start → serve → board-edit flow)", async ({
    page,
  }) => {
    // Wave 6 Task 2: typed draft mutations ride the shared core, whose
    // mutable-branch rule is design/<spec-name> exactly (AC-2's identical
    // semantics). The statusless draft's own namesake branch board is the
    // editable wall; the serving checkout's copy remains a projection.
    await page.goto(
      "/b/" +
        encodeURIComponent("design/" + EDGE.STATUSLESS_DRAFT_SPEC) +
        "/board/spec/" +
        EDGE.STATUSLESS_DRAFT_SPEC,
    );
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
    // (No clean-tree precondition here: earlier suites legitimately leave
    // the shared working tree dirty — graduation edits are never committed
    // by their own tests. The indicator assertion below is the honest one:
    // visible AFTER this spec's own edit.)

    // Before the fix this write 403'd (the board read the absent status
    // field as not-a-draft and rendered read-only).
    await editCard(page, "ac-1", (current) => current + " [statusless edit]");
    await expect(
      page.getByTestId("card-ac-1").filter({ hasText: "[statusless edit]" }),
    ).toHaveCount(1);

    // A spec edit is a working-tree write: the indicator rises.
    await expect(uncommittedIndicator(page)).toBeVisible();
  });

  test("the same statusless shape exact on the default branch renders the sealed read-only record", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.STATUSLESS_SEALED_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );
    // The sealed room offers no scratch tier and no commit affordance.
    await expect(
      page.getByRole("button", { name: "Add sticky" }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Commit & push" }),
    ).toHaveCount(0);
  });

  test("the directory speaks the derived vocabulary: proposed reads draft, landed reads accepted-pending-build", async ({
    page,
  }) => {
    await page.goto("/");

    // The design-branch draft: drafts-in-progress, chipped "draft" —
    // resolved from git state, with no status field anywhere on disk.
    const draft = page.getByTestId(dirEntryTestId(EDGE.STATUSLESS_DRAFT_SPEC));
    await expect(draft).toHaveCount(1);
    await expect(draft.locator(".badge-draft")).toHaveCount(1);

    // The landed twin: accepted-pending-build, in that group.
    const sealed = page.getByTestId(
      dirEntryTestId(EDGE.STATUSLESS_SEALED_SPEC),
    );
    await expect(sealed).toHaveCount(1);
    await expect(sealed.locator(".badge-accepted-pending-build")).toHaveCount(
      1,
    );
    await expect(
      page
        .getByTestId("dir-group-accepted-pending-build")
        .getByTestId(dirEntryTestId(EDGE.STATUSLESS_SEALED_SPEC)),
    ).toHaveCount(1);
  });
});
