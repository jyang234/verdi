import { test, expect } from "@playwright/test";
import { SHOWCASE, EDGE, boardPath, stubCardTestId, refCardTestId } from "./fixtures";

// spec/family-board-links: family navigation rendered in both directions
// from the `implements` edge alone (dc-1) — a story board's parent-
// feature affordance (ac-1), a feature board's stub card linking to
// every matching story anywhere in the store, active or archived alike
// (ac-2, dc-1's ADJ-28 completion reading) — the live
// refs/heads/design/<slug> in-between disclosure short of any match at
// all (ac-3, dc-3/dc-5), and a disclosed notice in place of a dead link
// wherever an implements target cannot resolve (ac-4, co-3).
//
// Per ADJ-39 (2026-07-16, constraint-over-mandate): the board route serves
// the active zone only, so an ARCHIVED match/target links to the servable
// corpus page (/a/spec/<name>) — the one surface that empirically serves an
// archived spec — with its archived state disclosed, never the board href
// that 404s (co-3/ac-4). Both the AC-2 archived-match and the AC-1
// archived-parent tests below FOLLOW their link to prove it lands on a live
// 200 corpus page, never a dead link.
//
// AC-1's ACTIVE story-to-feature direction and AC-2's ACTIVE-match direction
// drive the real, already-committed showcase pair
// (SHOWCASE.READONLY_SPEC "stale-decline" / SHOWCASE.STORY_STUB_MATCHED
// "borrower-update-api", dc-5) — no new fixture data. The archived-match,
// archived-parent, in-between, and dangling-target branches drive
// cmd/e2eharness/provision_familyboardlinks.go's EDGE fixtures.

test.describe("family board links: story board -> parent feature board (ac-1)", () => {
  test("the document-level implements edge resolves to the feature's own board, not only the corpus page, and follows there", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.STORY_STUB_MATCHED));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      SHOWCASE.STORY_STUB_MATCHED,
    );

    const link = page.getByTestId("refcard-board-link");
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute(
      "href",
      `/board/spec/${SHOWCASE.READONLY_SPEC}`,
    );

    await link.click();
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      SHOWCASE.READONLY_SPEC,
    );
    expect(page.url()).toContain(boardPath(SHOWCASE.READONLY_SPEC));
  });
});

test.describe("family board links: feature stub -> story board, active vs archived (ac-2)", () => {
  test("a matching ACTIVE story renders the plain board link and follows to that story's board", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    const stub = page.getByTestId(stubCardTestId(SHOWCASE.STORY_STUB_MATCHED));
    const link = stub.locator('[data-testid^="stub-story-link-"]');
    await expect(link).toHaveCount(1);
    await expect(link).toHaveAttribute(
      "href",
      `/board/spec/${SHOWCASE.STORY_STUB_MATCHED}`,
    );
    await expect(link).toHaveAttribute("data-archived", "false");
    // No archived disclosure and no in-between notice on a plain match.
    await expect(stub.locator(".badge-archived")).toHaveCount(0);
    await expect(
      stub.getByTestId(`stub-instantiated-notice-${SHOWCASE.STORY_STUB_MATCHED}`),
    ).toHaveCount(0);

    await link.click();
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      SHOWCASE.STORY_STUB_MATCHED,
    );
  });

  test("a matching ARCHIVED story links to its SERVABLE corpus page (never the 404 board route), archived disclosed, and follows there — never the in-between notice", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.FL_PARENT));
    const stub = page.getByTestId(stubCardTestId(EDGE.FL_ARCHIVED_CHILD));
    const link = stub.locator('[data-testid^="stub-story-link-"]');
    await expect(link).toHaveCount(1);
    // ADJ-39 (2026-07-16): the board route serves the active zone only, so
    // the archived spec's /board/spec/<name> 404s (co-3/ac-4 forbid a dead
    // href). The card links to the zone-agnostic corpus page instead — the
    // one surface that empirically serves an archived spec — with its
    // archived state disclosed.
    await expect(link).toHaveAttribute("href", `/a/spec/${EDGE.FL_ARCHIVED_CHILD}`);
    await expect(link).toHaveAttribute("data-archived", "true");
    await expect(link.locator(".badge-archived")).toHaveText("archived");
    // Never the dead board href the finding caught.
    await expect(
      stub.locator(`a[href="/board/spec/${EDGE.FL_ARCHIVED_CHILD}"]`),
    ).toHaveCount(0);

    // Per ADJ-28: the in-between notice must NOT render for an archived
    // match, even though this fixture's design/<slug> branch genuinely
    // exists (cmd/e2eharness/provision_familyboardlinks.go) — proving the
    // ref-check path never runs once a match resolves, not merely that
    // its output is absent.
    await expect(
      stub.getByTestId(`stub-instantiated-notice-${EDGE.FL_ARCHIVED_CHILD}`),
    ).toHaveCount(0);
    await expect(stub).not.toContainText("not yet in this checkout");

    // The link is LIVE: following it lands on the archived spec's corpus
    // page (HTTP 200), never a 404 — the whole point of the servability
    // fix (ADJ-39 constraint-over-mandate).
    await link.click();
    await expect(page).toHaveURL(new RegExp(`/a/spec/${EDGE.FL_ARCHIVED_CHILD}$`));
    await expect(
      page.getByRole("heading", {
        name: "Family links archived child (e2e fixture)",
      }),
    ).toBeVisible();
  });
});

test.describe("family board links: story board -> ARCHIVED parent feature (ac-1, ADJ-39 direction d)", () => {
  test("a story whose parent feature resolves only in the archive zone links to the servable corpus page, archived disclosed, never a 404 board href", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.FL_ORPHAN_STORY));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-spec",
      EDGE.FL_ORPHAN_STORY,
    );

    const card = page.getByTestId(refCardTestId(EDGE.FL_ARCHIVED_PARENT_TARGET));
    await expect(card).toBeVisible();

    const link = card.getByTestId("refcard-board-link");
    await expect(link).toHaveAttribute("href", `/a/spec/${EDGE.FL_ARCHIVED_PARENT}`);
    await expect(link).toHaveAttribute("data-archived", "true");
    await expect(link.getByTestId("refcard-feature-archived")).toHaveText("archived");
    // Never the dead board href, and no unresolved notice (the parent DOES
    // resolve — just in the archive zone).
    await expect(
      card.locator(`a[href="/board/spec/${EDGE.FL_ARCHIVED_PARENT}"]`),
    ).toHaveCount(0);
    await expect(card.getByTestId("refcard-unresolved-notice")).toHaveCount(0);

    // Live: following it lands on the archived feature's corpus page (200).
    await link.click();
    await expect(page).toHaveURL(
      new RegExp(`/a/spec/${EDGE.FL_ARCHIVED_PARENT}$`),
    );
    await expect(
      page.getByRole("heading", {
        name: "Family links archived parent (e2e fixture)",
      }),
    ).toBeVisible();
  });
});

test.describe("family board links: the live in-between disclosure (ac-3)", () => {
  test("a design branch with no matching story anywhere discloses the branch name", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.FL_PARENT));
    const stub = page.getByTestId(stubCardTestId(EDGE.FL_INSTANTIATED_CHILD));

    const notice = stub.getByTestId(
      `stub-instantiated-notice-${EDGE.FL_INSTANTIATED_CHILD}`,
    );
    await expect(notice).toHaveText(
      `instantiated on design/${EDGE.FL_INSTANTIATED_CHILD}, not yet in this checkout's active store`,
    );

    // No story link renders (nothing matched anywhere) — but the notice
    // is ADDITIVE, never a replacement: dc-4 takes no position on
    // whether a stub's coverage is complete, so the sealed wall's own
    // Instantiate affordance stays exactly as available as it always
    // was (clicking it would fail informatively — "branch already
    // exists" — a refusal 31-board-stub-instantiate.spec.ts already
    // proves works).
    await expect(stub.locator('[data-testid^="stub-story-link-"]')).toHaveCount(
      0,
    );
    await expect(
      stub.getByTestId(`instantiate-${EDGE.FL_INSTANTIATED_CHILD}`),
    ).toBeVisible();
  });

  test("no match and no design branch renders the plain un-instantiated state, unchanged", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.FL_PARENT));
    const stub = page.getByTestId(stubCardTestId(EDGE.FL_UNSTARTED_CHILD));

    // Today's plain state: the Instantiate affordance, and nothing else.
    await expect(
      stub.getByTestId(`instantiate-${EDGE.FL_UNSTARTED_CHILD}`),
    ).toBeVisible();
    await expect(
      stub.getByTestId(`stub-instantiated-notice-${EDGE.FL_UNSTARTED_CHILD}`),
    ).toHaveCount(0);
    await expect(stub.locator('[data-testid^="stub-story-link-"]')).toHaveCount(
      0,
    );
  });
});

test.describe("family board links: unresolvable implements target (ac-4)", () => {
  test("a story whose implements edge targets a feature ref absent from the store discloses it, with no dead link", async ({
    page,
  }) => {
    await page.goto(boardPath(EDGE.FL_DANGLING_STORY));

    const card = page.getByTestId(refCardTestId(EDGE.FL_DANGLING_TARGET));
    await expect(card).toBeVisible();

    const notice = card.getByTestId("refcard-unresolved-notice");
    await expect(notice).toHaveText(
      `${EDGE.FL_DANGLING_TARGET} does not resolve in this checkout's store — no board to link to`,
    );

    // Never a dead link: no board-link affordance, and no <a> at all on
    // this card.
    await expect(card.getByTestId("refcard-board-link")).toHaveCount(0);
    await expect(card.locator("a")).toHaveCount(0);
  });
});
