import { test, expect } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// spec/obligation-wall ac-2: the board AC card renders its obligations on the
// wall itself — for each declared evidence kind, that kind's obligation
// (title/prose) when one is authored, and a disclosed "no obligation" badge
// when none is yet (dc-2, the wall-receipts posture: disclosure, never
// refusal). So an operator reads what an AC demands from the AC's own rendered
// obligations, never recovered from the sidecar (feature co-3).
//
// The fixture wall's ac-1 declares two kinds (behavioral, static) with a
// COMMITTED obligation for behavioral only, so one card proves both halves.
// The obligation file is provisioned onto the design branch by
// cmd/e2eharness/provisionv2.go — a hermetic wall on disk, no network.

test.describe("obligation wall: a story AC card reads out its obligations", () => {
  test("the card shows an authored obligation's demand and discloses a kind with none", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.OBLIGATION_WALL_SPEC));

    const card = page.getByTestId(`card-${SHOWCASE.OBLIGATION_WALL_AC}`);
    await expect(card).toBeVisible();

    const obligations = page.getByTestId(`obligations-${SHOWCASE.OBLIGATION_WALL_AC}`);
    await expect(obligations).toBeVisible();

    // The authored (behavioral) obligation: its kind tag, and its title as the
    // visible demand read on the wall — the specific thing this AC requires,
    // legible without opening the obligation file or verdi.bindings.yaml.
    const present = obligations.locator(
      `.obligation[data-obligation-kind="${SHOWCASE.OBLIGATION_WALL_PRESENT_KIND}"]`,
    );
    await expect(present).toHaveAttribute("data-obligation-present", "true");
    await expect(present).toContainText(SHOWCASE.OBLIGATION_WALL_PRESENT_KIND);
    await expect(present.locator(".obligation-title")).toContainText(
      SHOWCASE.OBLIGATION_WALL_DEMAND,
    );

    // The declared-but-un-obligated (static) kind: the disclosed badge, never
    // an error, never silently omitted (dc-2 discloses; the activation gate is
    // what refuses at accept).
    const missing = page.getByTestId(
      `obligation-none-${SHOWCASE.OBLIGATION_WALL_AC}-${SHOWCASE.OBLIGATION_WALL_MISSING_KIND}`,
    );
    await expect(missing).toBeVisible();
    await expect(missing).toHaveText("no obligation");
    await expect(
      obligations.locator(
        `.obligation[data-obligation-kind="${SHOWCASE.OBLIGATION_WALL_MISSING_KIND}"]`,
      ),
    ).toHaveAttribute("data-obligation-present", "false");
  });
});
