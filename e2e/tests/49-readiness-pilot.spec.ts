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

// Pinned INDEPENDENTLY of the rendered DOM, from the committed hermetic
// harness fixtures (provision_board.go's refi-decline-flow design branch
// + provision_readiness.go's policy/constitution fixtures) and the fixed
// readiness comparators: the complete concern inventory in area-then-id
// order with each row's exact three-valued state, and the exact
// deterministic attention order (blocking first, violated first, then
// area/id order). A reordered, omitted, or extra concern — in either
// view — fails these exact-array oracles. The sha256 semantic id is the
// digest of committed fixture bytes and is therefore deterministic.
const SEMANTIC_ID =
  "context/semantic/sha256:a42722bcbc7bf152d376083fab35c04b462cf3f6735880306e78e8bee1815d6a";

const COMPLETE_CONCERNS: Array<[id: string, state: string]> = [
  ["shape/board", "proven"],
  ["shape/mutation", "proven"],
  ["shape/outcome", "proven"],
  ["shape/problem", "proven"],
  ["shape/provenance", "unproven"],
  ["shape/question/oq-1", "unproven"],
  ["success/contributor/attestation", "unproven"],
  ["success/contributor/behavioral", "unproven"],
  ["success/contributor/static", "unproven"],
  ["context/disclosure/repository-remote-unknown", "unproven"],
  ["context/mechanical/action:make-verify#complete", "proven"],
  ["context/mechanical/configuration:go-version#complete", "proven"],
  [SEMANTIC_ID, "unproven"],
  ["context/verdict", "unproven"],
  ["review/action", "unproven"],
  ["review/blocker/forge-facts-unavailable/merge", "violated-with-witness"],
  [
    "review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch",
    "violated-with-witness",
  ],
  ["review/role/merge/attestation/author-vouch", "unproven"],
];

const ATTENTION_QUEUE = [
  "review/blocker/forge-facts-unavailable/merge",
  "review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch",
  "shape/question/oq-1",
  "context/verdict",
  "review/action",
  "shape/provenance",
  "success/contributor/attestation",
  "success/contributor/behavioral",
  "success/contributor/static",
  "context/disclosure/repository-remote-unknown",
  SEMANTIC_ID,
  "review/role/merge/attestation/author-vouch",
];

// The first board-destination concern in queue order — the cockpit's
// first board link — pinned for exact event assertions below.
const BOARD_LINK_CONCERN = "shape/question/oq-1";
const BOARD_LINK_AREA = "shape-proposal";

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

  // EXACT ordered attention queue — pinned from the committed fixture,
  // never derived from the page. A reversed queue, a dropped concern, or
  // an extra row fails this array equality.
  const queueIds = await page
    .locator(".readiness-queue [data-concern-id]")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-concern-id")));
  expect(queueIds).toEqual(ATTENTION_QUEUE);

  // EXACT complete inventory with each row's exact state, in area-then-id
  // order. A concern omitted from BOTH views still fails here, because
  // the expected array is independent of the DOM.
  const allRows = await page
    .locator(".readiness-all [data-concern-id]")
    .evaluateAll((els) =>
      els.map((el) => [
        el.getAttribute("data-concern-id"),
        el
          .querySelector(".readiness-state")
          ?.className.replace(/.*readiness-state--/, ""),
      ]),
    );
  expect(allRows).toEqual(COMPLETE_CONCERNS);

  // Priority is positional: the queue renders above the complete section.
  const queueBox = await page.locator(".readiness-queue").boundingBox();
  const allBox = await page.locator(".readiness-all").boundingBox();
  expect(queueBox!.y).toBeLessThan(allBox!.y);
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

  // The cockpit source tab is preserved and the primary click appended
  // EXACTLY ONE event to the complete array — the expected
  // board-link-followed and nothing else (no filtering: any unintended
  // extra event fails the prefix/length equality).
  await expect(page.locator(".readiness-page")).toBeVisible();
  const after = await pilotEvents(page);
  expect(after.length).toBe(before.length + 1);
  expect(after.slice(0, before.length)).toEqual(before);
  expect(after[after.length - 1]).toEqual({
    sequence: before[before.length - 1].sequence + 1,
    event: "board-link-followed",
    area_id: BOARD_LINK_AREA,
    concern_id: BOARD_LINK_CONCERN,
  });
  await popup.close();
});

test("middle-button appends exactly one event; right-click appends none", async ({
  page,
}) => {
  await page.goto("/readiness");
  const boardLink = page.locator(".readiness-board-link").first();

  // Middle-button auxclick: the COMPLETE array grows by exactly the one
  // expected board-link-followed event.
  const beforeMiddle = await pilotEvents(page);
  await boardLink.click({ button: "middle" });
  const afterMiddle = await pilotEvents(page);
  expect(afterMiddle).toEqual([
    ...beforeMiddle,
    {
      sequence: beforeMiddle[beforeMiddle.length - 1].sequence + 1,
      event: "board-link-followed",
      area_id: BOARD_LINK_AREA,
      concern_id: BOARD_LINK_CONCERN,
    },
  ]);

  // Right-click: the complete array is identical — not one event of any
  // kind may be appended.
  await boardLink.click({ button: "right" });
  const afterRight = await pilotEvents(page);
  expect(afterRight).toEqual(afterMiddle);
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
  // The COMPLETE snapshot-bearing cockpit DOM of the source tab (metadata
  // card, stale notice, rail, queue, complete section, script include),
  // captured before the board is opened or edited.
  const domBefore = await page.evaluate(
    () => document.querySelector("main.content")!.outerHTML,
  );

  // One supported edit through the EXISTING editable board, in its own tab.
  const [popup] = await Promise.all([
    page.waitForEvent("popup"),
    page.locator(".readiness-board-link").first().click(),
  ]);
  await popup.waitForLoadState();
  await addSticky(popup, "readiness pilot probe: cockpit must not notice this");
  await popup.close();

  // The preserved source tab's cockpit DOM is byte-for-byte unchanged
  // after the real edit — instrumentation appended an event, but neither
  // it nor the edit may touch the rendered snapshot.
  const domAfter = await page.evaluate(
    () => document.querySelector("main.content")!.outerHTML,
  );
  expect(domAfter).toBe(domBefore);
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
