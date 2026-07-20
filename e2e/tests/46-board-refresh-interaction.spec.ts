import { test, expect, type Locator, type Page } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";
import { addSticky, expectAutosaved } from "./helpers";

// Owner bug report (2026-07-19): "UI a little jerky — when interacting and
// moving stickies around the refresh would delay and then the screen would
// reset. Not a good experience. Also some error messages popped up that
// certain annotations were missing, unclear where."
//
// Root cause (driven repro, this file): every mutation swaps the whole
// board region (`region.innerHTML = fragment`) with no regard for what the
// hand is doing when the response lands. Four distinct failures fell out
// of that one seam, each pinned by one test below:
//
//   1. a fragment landing MID-GESTURE detached the dragged element — the
//      sticky snapped out from under the hand back to its server position
//      ("the screen would reset");
//   2. two quick mutations could apply their fragments OUT OF ORDER — a
//      stale response rolled the wall back to the older state;
//   3. the swap replaced the scrollable canvas element, so its scroll
//      position reset to the origin on every save;
//   4. between a scratch delete's POST and its refresh the dead sticky
//      stayed visible and clickable — a second delete then hit the server
//      after the record was gone, popping "no annotation …" errors about
//      annotations the STORE never lost (a UI stale-window race, not data
//      corruption — witnessed against the unfixed build).
//
// The fix keeps one renderer and one projection: fragment responses carry
// a sequence (stale ones are discarded), application is DEFERRED while a
// gesture/editor/draft is live (resumed from a fresh fetch at interaction
// end), canvas scroll is carried across the swap, scratch deletes hide
// their element the moment the delete is posted, and a refused mutation
// re-fetches the projection so the wall never keeps showing a state the
// server rejected.

// Hold the NEXT board-fragment response: the body is fetched from the
// server at request time (so it is genuinely that moment's projection),
// but delivery to the page waits until the returned release() is called.
// Later fragment requests flow through untouched.
async function holdNextFragment(page: Page): Promise<() => Promise<void>> {
  let release!: () => void;
  const gate = new Promise<void>((resolve) => (release = resolve));
  let delivered!: () => void;
  const done = new Promise<void>((resolve) => (delivered = resolve));
  let armed = true;
  await page.route("**/board/spec/**/fragment", async (route) => {
    if (!armed) return route.continue();
    armed = false;
    const resp = await route.fetch();
    const body = await resp.text();
    await gate;
    await route.fulfill({ response: resp, body });
    delivered();
  });
  return async () => {
    release();
    await done;
  };
}

// One raw pointer drag (the board listens to pointer events; locator
// actions would auto-scroll and mask geometry). noUp leaves the gesture
// live mid-drag for the refresh-boundary tests.
async function dragBy(
  page: Page,
  target: Locator,
  dx: number,
  dy: number,
  opts?: { noUp?: boolean },
): Promise<void> {
  const box = await target.boundingBox();
  expect(box, "drag target has no layout box").not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    box!.x + box!.width / 2 + dx,
    box!.y + box!.height / 2 + dy,
    { steps: 8 },
  );
  if (!opts?.noUp) await page.mouse.up();
}

// NOTE: every position/drag-state read below goes through page.evaluate
// against the id, never a captured node — a buggy swap REPLACES nodes,
// and the assertion must see whatever element now claims the id.

const settleFrame = (page: Page) =>
  page.evaluate(
    () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))),
  );

test.describe("board refresh vs. live interaction (owner jank report)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.DESIGN_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );
  });

  test("a sticky drag across an in-flight refresh keeps following the hand (no mid-drag reset)", async ({
    page,
  }) => {
    const sticky = await addSticky(
      page,
      "hold my position across the refresh",
      "question",
    );
    const id = (await sticky.getAttribute("data-id"))!;
    await sticky.scrollIntoViewIfNeeded();

    const releaseFragment = await holdNextFragment(page);

    // Drag 1 drops the sticky; its post-mutation refresh is now in
    // flight (held). The next gesture starts before it lands — exactly
    // the owner's "moving stickies around while the refresh delays".
    await dragBy(page, sticky, 120, 60);

    // Drag 2: press and move, and STAY mid-gesture.
    await dragBy(page, sticky, 90, 45, { noUp: true });

    const before = await page.evaluate((sid) => {
      const el = document.querySelector(`.sticky[data-id="${sid}"]`) as HTMLElement;
      return {
        left: el.style.left,
        top: el.style.top,
        dragging: el.classList.contains("dragging"),
      };
    }, id);
    expect(before.dragging).toBe(true);

    // The delayed refresh lands NOW, mid-drag.
    await releaseFragment();
    await settleFrame(page);

    // The hand must not be robbed: same element still dragging, still at
    // the dragged position — never snapped back to the server state.
    const after = await page.evaluate((sid) => {
      const el = document.querySelector(`.sticky[data-id="${sid}"]`) as HTMLElement | null;
      return el
        ? {
            left: el.style.left,
            top: el.style.top,
            dragging: el.classList.contains("dragging"),
          }
        : null;
    }, id);
    expect(after).not.toBeNull();
    expect(after!.dragging).toBe(true);
    expect(after!.left).toBe(before.left);
    expect(after!.top).toBe(before.top);

    // Finish the drag; the drop commits and persists.
    await page.mouse.move(300, 400, { steps: 4 });
    await page.mouse.up();
    await expectAutosaved(page);
    const final = await page.evaluate((sid) => {
      const el = document.querySelector(`.sticky[data-id="${sid}"]`) as HTMLElement;
      return { left: parseFloat(el.style.left), top: parseFloat(el.style.top) };
    }, id);

    await page.reload();
    const persisted = await page.evaluate((sid) => {
      const el = document.querySelector(`.sticky[data-id="${sid}"]`) as HTMLElement;
      return { left: parseFloat(el.style.left), top: parseFloat(el.style.top) };
    }, id);
    expect(Math.abs(persisted.left - final.left)).toBeLessThanOrEqual(1);
    expect(Math.abs(persisted.top - final.top)).toBeLessThanOrEqual(1);
  });

  test("a stale fragment response never rolls back a newer projection", async ({
    page,
  }) => {
    const sticky = await addSticky(
      page,
      "newest projection wins the wall",
      "question",
    );
    const id = (await sticky.getAttribute("data-id"))!;
    await sticky.scrollIntoViewIfNeeded();

    const releaseFragment = await holdNextFragment(page);

    // Mutation 1: drop at A — its fragment (rendering A) is held.
    await dragBy(page, sticky, 140, 40);
    // Mutation 2: drop at B — its fragment flows through and applies.
    await dragBy(page, sticky, 60, 80);
    await expectAutosaved(page);

    const atB = await page.evaluate((sid) => {
      const el = document.querySelector(`.sticky[data-id="${sid}"]`) as HTMLElement;
      return { left: el.style.left, top: el.style.top };
    }, id);

    // The STALE response (state A) arrives last. It must be discarded,
    // never rolled over the newer projection.
    await releaseFragment();
    await settleFrame(page);

    const afterStale = await page.evaluate((sid) => {
      const el = document.querySelector(`.sticky[data-id="${sid}"]`) as HTMLElement;
      return { left: el.style.left, top: el.style.top };
    }, id);
    expect(afterStale).toEqual(atB);
  });

  test("deleting a sticky acknowledges immediately — no stale ghost inviting a double delete", async ({
    page,
  }) => {
    const sticky = await addSticky(page, "dies once, quietly", "question");
    await sticky.scrollIntoViewIfNeeded();

    const releaseFragment = await holdNextFragment(page);

    // The × posts the delete; until the refresh lands the old DOM is all
    // the user can see. The dead sticky must not keep standing there
    // clickable — that stale window is where the owner's "no annotation …"
    // popups came from (a second delete racing the first, witnessed
    // against the unfixed build; the store itself never lost the record).
    await sticky.locator('.delete-btn[data-delete="sticky"]').click();
    await expect(sticky).toBeHidden({ timeout: 1_500 });

    await releaseFragment();
    await expectAutosaved(page);
    await expect(sticky).toHaveCount(0);
    await expect(page.getByTestId("autosave-status")).not.toContainText("error");
  });

  test("a refused mutation reconciles the wall back to server truth", async ({
    page,
  }) => {
    const card = page.getByTestId(`card-${SHOWCASE.AC_IDS[0]}`);
    await expect(card).toBeVisible();
    const before = await card.evaluate((el) => ({
      left: (el as HTMLElement).style.left,
      top: (el as HTMLElement).style.top,
    }));

    // One synthetic refusal from the position endpoint: the client-side
    // contract under test is what the board DOES with a refusal — it must
    // say so AND re-fetch the projection instead of keeping the refused
    // position on screen (a wall that lies is the failure mode).
    let refused = false;
    await page.route("**/api/position", async (route) => {
      if (refused) return route.continue();
      refused = true;
      await route.fulfill({
        status: 400,
        contentType: "application/json",
        body: JSON.stringify({ error: "synthetic refusal (e2e reconcile probe)" }),
      });
    });

    await dragBy(page, card, 90, 50);

    await expect(page.getByTestId("autosave-status")).toContainText(
      "error: synthetic refusal (e2e reconcile probe)",
    );
    // The wall re-syncs: the card returns to the server's position.
    await expect
      .poll(async () =>
        card.evaluate((el) => ({
          left: (el as HTMLElement).style.left,
          top: (el as HTMLElement).style.top,
        })),
      )
      .toEqual(before);
  });

  // Runs LAST in this file: the setup write parks ac-1 far right to make
  // the canvas genuinely scrollable, and that stored position persists
  // for the rest of the run (later specs only add stickies).
  test("canvas scroll survives the post-mutation fragment swap", async ({
    page,
  }) => {
    // Park ac-1 at x=2100 through the board's own position API (setup,
    // not the behavior under test), then reload so the projection
    // renders it there and the canvas overflows.
    await page.evaluate(async (spec) => {
      const resp = await fetch(`/board/spec/${spec}/api/position`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: "ac-1", x: 2100, y: 60 }),
      });
      if (!resp.ok) throw new Error("setup position write failed: " + resp.status);
    }, SHOWCASE.DESIGN_SPEC);
    await page.reload();

    const canvas = page.getByTestId("board");
    const scrollable = await canvas.evaluate(
      (el) => el.scrollWidth > el.clientWidth,
    );
    expect(scrollable, "setup must leave the canvas horizontally scrollable").toBe(
      true,
    );

    await canvas.evaluate((el) => (el.scrollLeft = 250));
    await expect
      .poll(() => canvas.evaluate((el) => Math.round(el.scrollLeft)))
      .toBe(250);

    // Drag whichever card is fully visible in the scrolled viewport (the
    // scroll shifted every column left by 250px).
    const visible = await page.evaluate(() => {
      const cards = Array.from(
        document.querySelectorAll(".objcard"),
      ) as HTMLElement[];
      for (const el of cards) {
        const r = el.getBoundingClientRect();
        if (r.left >= 0 && r.top >= 0 && r.right <= window.innerWidth && r.bottom <= window.innerHeight) {
          return el.getAttribute("data-id");
        }
      }
      return null;
    });
    expect(visible, "no fully visible card to drag in the scrolled viewport").not.toBeNull();

    await dragBy(page, page.locator(`.objcard[data-id="${visible}"]`), -40, 30);
    await expectAutosaved(page);

    // The swap must carry the scroll across — never snap back to origin.
    await expect
      .poll(() => canvas.evaluate((el) => Math.round(el.scrollLeft)))
      .toBe(250);
  });
});
