import { test, expect, type Page } from "@playwright/test";
import {
  DESIGN_SPEC,
  READONLY_SPEC,
  ADR_REF,
  DOC_EDGE_TARGET,
  boardPath,
  refCardTestId,
} from "./fixtures";

// Owner UAT (round 6, item 4): "the reference object is an ADR, which is
// unclickable… The user shouldn't be forced to exit the focused view to
// look up the external reference content." Clicking a reference card
// opens an in-board peek — title, kind, status, rendered body, and an
// "open full page" link to /a/<kind>/<name> — in EVERY board mode (the
// peek is read-only information; read-only boards especially need it).
// An unresolvable ref gets a disclosed explanation, never a dead click.

const peek = (page: Page) => page.getByTestId("ref-peek");

test.describe("board: reference cards peek their artifact", () => {
  test("read-only board: click opens the peek; ×, Escape, and outside-click close it", async ({
    page,
  }) => {
    await page.goto(boardPath(READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "readonly",
    );

    await page.getByTestId(refCardTestId(DOC_EDGE_TARGET)).click();
    await expect(peek(page)).toBeVisible();
    await expect(peek(page)).toContainText(
      "Outbox pattern for domain events, v2 (fixture)",
    );
    await expect(peek(page).locator(".peek-kind")).toHaveText("adr");
    await expect(peek(page).locator(".peek-status")).toHaveText("accepted");
    // The body is rendered content, not raw markdown.
    await expect(peek(page).locator(".peek-body")).not.toContainText("##");
    const open = peek(page).getByRole("link", { name: /open full page/i });
    await expect(open).toHaveAttribute("href", `/a/${DOC_EDGE_TARGET}`);

    // Three ways out, same as every board surface.
    await peek(page).getByRole("button", { name: "Close peek" }).click();
    await expect(peek(page)).toHaveCount(0);

    await page.getByTestId(refCardTestId(DOC_EDGE_TARGET)).click();
    await expect(peek(page)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(peek(page)).toHaveCount(0);

    await page.getByTestId(refCardTestId(DOC_EDGE_TARGET)).click();
    await expect(peek(page)).toBeVisible();
    // Any click outside the peek dismisses it — empty corkboard here.
    await page.getByTestId("board").click({ position: { x: 600, y: 350 } });
    await expect(peek(page)).toHaveCount(0);
  });

  test("authoring board: the same peek works alongside the editing surface", async ({
    page,
  }) => {
    await page.goto(boardPath(DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    await page.getByTestId(refCardTestId(ADR_REF)).click();
    await expect(peek(page)).toBeVisible();
    await expect(peek(page).locator(".peek-kind")).toHaveText("adr");
    await expect(
      peek(page).getByRole("link", { name: /open full page/i }),
    ).toHaveAttribute("href", `/a/${ADR_REF}`);
    await page.keyboard.press("Escape");
    await expect(peek(page)).toHaveCount(0);
  });

  test("an unresolvable ref discloses itself instead of failing silently", async ({
    page,
    request,
  }) => {
    // No fixture board carries an unresolvable ref card (the projection
    // resolves every closed-edge endpoint), so the disclosed state is
    // asserted at the fragment seam the click renders.
    const resp = await request.get(
      `/board/spec/${READONLY_SPEC}/peek?ref=${encodeURIComponent("jira:LOAN-1482")}`,
    );
    expect(resp.status()).toBe(200);
    const body = await resp.text();
    expect(body).toContain('data-testid="ref-peek-error"');
    expect(body).toContain("jira:LOAN-1482");
  });
});
