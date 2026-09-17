import { test, expect } from "@playwright/test";
import { SHOWCASE } from "./fixtures";

// The policy setup guide quotes the refusal's code with its detail (wave-1
// ledger R-5, after the ac-5 single-prefix fix): "The workbench reported:"
// carries code + ": " + detail exactly once — never the bare detail the
// L4 fix left behind, and never a doubled "policy-forbidden:
// policy-forbidden:" prefix. The showcase draft's branch tree carries no
// .verdi/policy at all, so its board resolves draftmutation's genuine
// not-adopted refusal (50-design-workbench.spec.ts pins the guide variant).
// State assertions ride testids and text, never screenshots (recording
// stays off).

const DRAFT_B = () =>
  "/b/" + encodeURIComponent(SHOWCASE.SHOWCASE_DRAFT_BRANCH) + "/board/spec/" + SHOWCASE.SHOWCASE_DRAFT_SPEC;

test("the policy setup guide quotes the refusal code and its detail exactly once", async ({ page }) => {
  await page.goto(DRAFT_B());
  await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");

  const guide = page.getByTestId("asd-policy-guide");
  await expect(guide).toHaveAttribute("data-policy-guide", "not-adopted");
  const report = guide.getByTestId("asd-policy-guide-report");
  await expect(report).toHaveCount(1);
  await expect(report).toBeVisible();
  await expect(report).toHaveText("policy-forbidden: project has not adopted policy authority");
  await expect(guide).not.toContainText("policy-forbidden: policy-forbidden");

  // The quote sits inside the guide's opening plain-words summary, and
  // agrees with the context/policy row's own witness (Code + ": " + Detail).
  const summary = guide.locator("p.readiness-summary").first();
  await expect(summary).toContainText("The workbench reported: policy-forbidden: project has not adopted policy authority.");
  const more = page.locator('[data-testid="asd-more"] > summary');
  if (await more.count()) await more.click();
  const policyRow = page.locator('[data-concern-id="context/policy"]').first();
  await expect(policyRow).toBeVisible();
  await expect(policyRow).toContainText("policy-forbidden: project has not adopted policy authority");
});
