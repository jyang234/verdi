import { test, expect, type Page } from "@playwright/test";
import { SHOWCASE, branchBoardPath } from "./fixtures";
import { addSticky } from "./helpers";

// The Wave 3.5 readiness pilot cockpit (GET /readiness): browser proof of
// the FABLE-owned hybrid surface over the ONE immutable startup snapshot
// the harness provisions via --context-request (target
// SHOWCASE.DESIGN_SPEC on SHOWCASE.DESIGN_BRANCH). The cockpit is
// GET-only and never recomputes readiness — every assertion here reads;
// the one supported mutation below happens through the EXISTING board
// surface in its own tab, and the cockpit must not notice.
//
// The closed instrumentation vocabulary (plan, Task 3/4): readiness-opened,
// area-inspected, concern-inspected, board-link-followed,
// cli-fallback-copied, stale-notice-inspected — page memory only.

const RAIL_ORDER = [
  "shape-proposal",
  "show-success",
  "check-context",
  "request-review",
];

const EVENT_VOCABULARY = new Set([
  "readiness-opened",
  "area-inspected",
  "concern-inspected",
  "board-link-followed",
  "cli-fallback-copied",
  "stale-notice-inspected",
]);

type PilotEvent = {
  sequence: number;
  event: string;
  area_id: string;
  concern_id: string;
};

function pilotEvents(page: Page): Promise<PilotEvent[]> {
  return page.evaluate(
    () =>
      (window as unknown as { __verdiReadinessPilotEvents: PilotEvent[] })
        .__verdiReadinessPilotEvents,
  );
}

async function startupHead(page: Page): Promise<string> {
  const head = await page
    .locator('.metadata-card dt:text-is("Head") + dd')
    .innerText();
  expect(head.trim()).not.toBe("");
  return head.trim();
}

test("rail gives linear orientation in the fixed four-area order", async ({
  page,
}) => {
  await page.goto("/readiness");

  const stations = page.locator(".readiness-rail .readiness-station");
  await expect(stations).toHaveCount(4);
  for (let i = 0; i < RAIL_ORDER.length; i++) {
    await expect(stations.nth(i)).toHaveAttribute("data-area-id", RAIL_ORDER[i]);
    // Every station speaks its exact three-valued state word (the chip
    // text is the word plus an aria-hidden shape glyph).
    await expect(
      stations.nth(i).locator(".readiness-state"),
    ).toHaveText(/(^|[✓✕◌])(proven|violated-with-witness|unproven)$/);
  }

  // The current-focus marker sits on exactly the FIRST non-proven area
  // (state read from the chip's modifier class, glyph-independent).
  const states = await stations
    .locator(".readiness-state")
    .evaluateAll((els) =>
      els.map((el) => el.className.replace(/.*readiness-state--/, "")),
    );
  const firstNonProven = states.findIndex((s) => s !== "proven");
  expect(firstNonProven, "harness snapshot must carry a non-proven area").toBeGreaterThanOrEqual(0);
  await expect(page.locator(".readiness-focus")).toHaveCount(1);
  await expect(
    stations.nth(firstNonProven).locator(".readiness-focus"),
  ).toHaveText("current focus");
  await expect(
    stations.nth(firstNonProven).locator("a[aria-current]"),
  ).toHaveCount(1);

  // Continuously visible: the rail stays on screen at the page bottom.
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  const railBox = await page.locator(".readiness-rail").boundingBox();
  expect(railBox).not.toBeNull();
  expect(railBox!.y).toBeGreaterThanOrEqual(0);
  expect(railBox!.y).toBeLessThan(page.viewportSize()!.height);
});

test("attention queue is prioritized without omitting any concern", async ({
  page,
}) => {
  await page.goto("/readiness");

  const queueIds = await page
    .locator(".readiness-queue [data-concern-id]")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-concern-id")));
  expect(queueIds.length).toBeGreaterThan(0);

  const allRows = await page
    .locator(".readiness-all [data-concern-id]")
    .evaluateAll((els) =>
      els.map((el) => ({
        id: el.getAttribute("data-concern-id"),
        state: el
          .querySelector(".readiness-state")
          ?.className.replace(/.*readiness-state--/, ""),
      })),
    );

  // Every queue entry appears in the complete grouped view…
  const allIds = new Set(allRows.map((r) => r.id));
  for (const id of queueIds) {
    expect(allIds.has(id), `queue concern ${id} missing from All concerns`).toBe(true);
  }
  // …and the queue omits NO unresolved concern (priority ≠ omission).
  const unresolved = allRows.filter((r) => r.state !== "proven").map((r) => r.id);
  expect(new Set(queueIds)).toEqual(new Set(unresolved));
  // Proven facts stay visible in the complete view.
  expect(allRows.some((r) => r.state === "proven")).toBe(true);

  // Priority is positional: the queue renders above the complete section,
  // as a numbered ordered list.
  const queueBox = await page.locator(".readiness-queue").boundingBox();
  const allBox = await page.locator(".readiness-all").boundingBox();
  expect(queueBox!.y).toBeLessThan(allBox!.y);
  await expect(page.locator(".readiness-queue ol.readiness-queue-list")).toHaveCount(1);
});

test("violated-with-witness and unproven are visibly and textually distinct", async ({
  page,
}) => {
  await page.goto("/readiness");

  const violated = page.locator(".readiness-state--violated-with-witness").first();
  const unproven = page.locator(".readiness-state--unproven").first();
  await expect(violated).toContainText("violated-with-witness");
  await expect(unproven).toContainText("unproven");

  // Beyond color: the glyphs differ and unproven carries a dashed border.
  const vGlyph = await violated.locator(".readiness-glyph").innerText();
  const uGlyph = await unproven.locator(".readiness-glyph").innerText();
  expect(vGlyph).not.toBe(uGlyph);
  await expect(unproven).toHaveCSS("border-top-style", "dashed");
  await expect(violated).toHaveCSS("border-top-style", "solid");

  // An unresolved concern carries its exact witnesses.
  const witnessed = page
    .locator(".readiness-queue .readiness-card .readiness-witnesses code")
    .first();
  await expect(witnessed).toBeVisible();
  await expect(witnessed).not.toHaveText("");
});

test("board destination opens the editable board in a new tab and both tabs keep their state", async ({
  page,
}) => {
  await page.goto("/readiness");
  const before = await pilotEvents(page);
  expect(before[0]).toMatchObject({ sequence: 1, event: "readiness-opened" });

  const boardLink = page.locator(".readiness-board-link").first();
  await expect(boardLink).toHaveAttribute("target", "_blank");
  await expect(boardLink).toHaveAttribute("rel", "noopener");
  await expect(boardLink).toHaveAttribute(
    "href",
    branchBoardPath(SHOWCASE.DESIGN_BRANCH, SHOWCASE.DESIGN_SPEC),
  );

  const [popup] = await Promise.all([
    page.waitForEvent("popup"),
    boardLink.click(),
  ]);
  await popup.waitForLoadState();
  expect(popup.url()).toContain(
    branchBoardPath(SHOWCASE.DESIGN_BRANCH, SHOWCASE.DESIGN_SPEC),
  );
  // The existing board, in authoring (editable) mode — not a copy.
  await expect(popup.getByRole("button", { name: "Add sticky" })).toBeVisible();

  // The cockpit source tab is preserved with its event array intact: the
  // prior events are still there, board-link-followed was appended with a
  // monotonic sequence, exactly once for the primary click.
  await expect(page.locator(".readiness-page")).toBeVisible();
  const after = await pilotEvents(page);
  expect(after.slice(0, before.length)).toEqual(before);
  const boardEvents = after.filter((e) => e.event === "board-link-followed");
  expect(boardEvents).toHaveLength(1);
  expect(boardEvents[0].concern_id).not.toBe("");
  for (let i = 1; i < after.length; i++) {
    expect(after[i].sequence).toBeGreaterThan(after[i - 1].sequence);
  }
  await popup.close();
});

test("middle-button navigation records exactly one event; right-click records none", async ({
  page,
}) => {
  await page.goto("/readiness");
  const boardLink = page.locator(".readiness-board-link").first();

  await boardLink.click({ button: "middle" });
  let events = (await pilotEvents(page)).filter(
    (e) => e.event === "board-link-followed",
  );
  expect(events).toHaveLength(1);

  await boardLink.click({ button: "right" });
  events = (await pilotEvents(page)).filter(
    (e) => e.event === "board-link-followed",
  );
  expect(events).toHaveLength(1);
});

test("CLI fallback tokens copy as the exact vector, never an invented shell command", async ({
  page,
  context,
}) => {
  await page.goto("/readiness");
  await context.grantPermissions(["clipboard-read", "clipboard-write"], {
    origin: new URL(page.url()).origin,
  });

  const cli = page.locator(".readiness-cli").first();
  const tokens = await cli
    .locator(".readiness-cli-token")
    .allInnerTexts();
  expect(tokens.length).toBeGreaterThan(0);
  expect(tokens[0]).toBe("verdi");

  // Select the vector and copy with the keyboard.
  await cli.evaluate((el) => {
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.selectAllChildren(el);
  });
  await page.keyboard.press("ControlOrMeta+c");

  const copied = await page.evaluate(() => navigator.clipboard.readText());
  // The copy is exactly the tokens in order, whitespace-separated — no
  // quoting, escaping, or joined shell syntax invented by the page.
  expect(copied.split(/\s+/).filter((t) => t !== "")).toEqual(tokens);

  const copyEvents = (await pilotEvents(page)).filter(
    (e) => e.event === "cli-fallback-copied",
  );
  expect(copyEvents.length).toBeGreaterThanOrEqual(1);
  expect(copyEvents[0].concern_id).not.toBe("");
});

test("keyboard traversal reaches notice, rail, queue, board link, and CLI fallback", async ({
  page,
}) => {
  await page.goto("/readiness");
  await page.emulateMedia({ reducedMotion: "reduce" });

  // Walk the tab order and record which cockpit landmarks receive focus.
  const reached = new Set<string>();
  await page.locator("body").press("Tab"); // enter the page's tab order
  for (let i = 0; i < 120; i++) {
    const cls = await page.evaluate(
      () => (document.activeElement && document.activeElement.className) || "",
    );
    for (const landmark of [
      "readiness-stale",
      "readiness-station-link",
      "readiness-concern-link",
      "readiness-board-link",
      "readiness-cli",
    ]) {
      if (String(cls).split(" ").includes(landmark)) reached.add(landmark);
    }
    if (reached.size === 5) break;
    await page.keyboard.press("Tab");
  }
  expect([...reached].sort()).toEqual([
    "readiness-board-link",
    "readiness-cli",
    "readiness-concern-link",
    "readiness-stale",
    "readiness-station-link",
  ]);

  // Focusing the stale notice records its inspection.
  await page.locator(".readiness-stale").focus();
  const staleEvents = (await pilotEvents(page)).filter(
    (e) => e.event === "stale-notice-inspected",
  );
  expect(staleEvents.length).toBeGreaterThanOrEqual(1);

  // Reduced motion: the cockpit's opt-in transitions are inert.
  await expect(page.locator(".readiness-station-link").first()).toHaveCSS(
    "transition-duration",
    "0s",
  );
});

test("instrumentation keeps the closed vocabulary, exact shape, 200-cap, and page-memory-only posture", async ({
  page,
}) => {
  await page.goto("/readiness");
  await page.waitForLoadState("networkidle");

  // From here on, the cockpit itself must issue NO network request.
  const requests: string[] = [];
  page.on("request", (r) => requests.push(r.url()));

  // Mirror the same-page CustomEvent stream.
  await page.evaluate(() => {
    const w = window as unknown as { __seen: unknown[] };
    w.__seen = [];
    document.addEventListener("verdi:readiness-pilot", (e) => {
      w.__seen.push((e as CustomEvent).detail);
    });
  });

  const htmlBefore = await page.evaluate(() => document.body.innerHTML);

  // Real interactions plus a burst that overflows the cap.
  await page.locator(".readiness-stale").click();
  await page.locator(".readiness-queue [data-concern-id]").first().click();
  await page.evaluate(() => {
    const link = document.querySelector<HTMLAnchorElement>(
      ".readiness-station-link",
    )!;
    for (let i = 0; i < 210; i++) link.click();
  });

  const events = await pilotEvents(page);
  expect(events).toHaveLength(200); // capped at the final 200
  for (const event of events) {
    expect(Object.keys(event).sort()).toEqual([
      "area_id",
      "concern_id",
      "event",
      "sequence",
    ]);
    expect(EVENT_VOCABULARY.has(event.event)).toBe(true);
  }
  for (let i = 1; i < events.length; i++) {
    expect(events[i].sequence).toBe(events[i - 1].sequence + 1);
  }
  // The cap keeps the FINAL 200: the tail is the last synthetic click.
  expect(events[events.length - 1].event).toBe("area-inspected");
  expect(events[events.length - 1].area_id).toBe("shape-proposal");

  // The same-page CustomEvent stream carried the same objects.
  const seen = await page.evaluate(
    () => (window as unknown as { __seen: PilotEvent[] }).__seen,
  );
  expect(seen.length).toBeGreaterThanOrEqual(212);
  expect(seen[seen.length - 1]).toEqual(events[events.length - 1]);

  // No network, no persistence, no rendering effect.
  expect(requests).toEqual([]);
  const persistence = await page.evaluate(() => ({
    local: window.localStorage.length,
    session: window.sessionStorage.length,
    cookie: document.cookie,
  }));
  expect(persistence).toEqual({ local: 0, session: 0, cookie: "" });
  expect(await page.evaluate(() => document.body.innerHTML)).toBe(htmlBefore);
});

test("an edit through the existing board leaves the preserved cockpit and its snapshot unchanged", async ({
  page,
}) => {
  await page.goto("/readiness");
  const head = await startupHead(page);

  // The stale notice names the exact startup HEAD and directs a restart.
  const notice = page.locator(".readiness-stale");
  await expect(notice).toContainText(`Startup snapshot at ${head}`);
  await expect(notice).toContainText("restart verdi serve");

  const bodyBefore = await (await page.request.get("/readiness")).text();
  const eventsBefore = await pilotEvents(page);

  // One supported edit through the EXISTING editable board, in its own tab.
  const [popup] = await Promise.all([
    page.waitForEvent("popup"),
    page.locator(".readiness-board-link").first().click(),
  ]);
  await popup.waitForLoadState();
  await addSticky(popup, "readiness pilot probe: cockpit must not notice this");
  await popup.close();

  // The preserved cockpit tab still shows the original immutable snapshot.
  await expect(page.locator(".readiness-stale")).toContainText(
    `Startup snapshot at ${head}`,
  );
  const events = await pilotEvents(page);
  expect(events.slice(0, eventsBefore.length)).toEqual(eventsBefore);

  // A fresh render after the edit is byte-identical: startup snapshot,
  // never recomputed.
  const bodyAfter = await (await page.request.get("/readiness")).text();
  expect(bodyAfter).toBe(bodyBefore);
});

test("420px: pinned rail hides nothing and long values do not overflow", async ({
  page,
}) => {
  await page.setViewportSize({ width: 420, height: 800 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/readiness");

  // No horizontal overflow and no cockpit element clipping its content.
  const widths = await page.evaluate(() => ({
    doc: document.documentElement.scrollWidth,
    clipped: Array.from(
      document.querySelectorAll(
        ".readiness-card, .readiness-row, .readiness-cli, .readiness-witnesses",
      ),
    )
      .filter((el) => el.scrollWidth > el.clientWidth + 1)
      .map((el) => el.className),
  }));
  expect(widths.doc).toBeLessThanOrEqual(420);
  expect(widths.clipped).toEqual([]);

  // Rail-link fragment target lands below the pinned rail.
  const railBottom = () =>
    page.evaluate(
      () => document.querySelector(".readiness-rail")!.getBoundingClientRect().bottom,
    );
  await page.locator('a[href="#area-check-context"]').click();
  let top = await page.evaluate(
    () => document.getElementById("area-check-context")!.getBoundingClientRect().top,
  );
  expect(top).toBeGreaterThanOrEqual((await railBottom()) - 1);

  // Keyboard-activated concern fragment target likewise.
  await page.evaluate(() => window.scrollTo(0, 0));
  const concernLink = page.locator("a.readiness-concern-link").first();
  const targetId = (await concernLink.getAttribute("href"))!.slice(1);
  await concernLink.focus();
  await page.keyboard.press("Enter");
  top = await page.evaluate(
    (id) => document.getElementById(id)!.getBoundingClientRect().top,
    targetId,
  );
  expect(top).toBeGreaterThanOrEqual((await railBottom()) - 1);
});
