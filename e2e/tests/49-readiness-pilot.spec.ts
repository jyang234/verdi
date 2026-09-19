import { test, expect, type Page } from "@playwright/test";
import { CONTROL_URL, SHOWCASE, branchBoardPath } from "./fixtures";
import { addSticky, expectAutosaved } from "./helpers";

// The Wave 3.5 readiness pilot cockpit (GET /readiness), F-01 corrected
// form (SI-125): orientation first ("where am I?"), the four-step
// process rail with plain labels, a ranked focus list showing the top
// three priorities with the exact remainder behind one inline
// disclosure, and completed checks holding every proven fact. The page
// is a GET-only view of readiness derived fresh for each request
// (spec/readiness-recovery ac-2 — the stamp names the HEAD the request
// looked at); the only interactive state is the ephemeral open state of
// native disclosures.
//
// Closed instrumentation vocabulary (unchanged): readiness-opened,
// area-inspected, concern-inspected, board-link-followed,
// cli-fallback-copied, stale-notice-inspected — page memory only.

// Pinned INDEPENDENTLY of the rendered DOM, from the committed hermetic
// harness fixtures (provision_board.go's refi-decline-flow design branch
// + provision_readiness.go's policy fixtures) and the corrected
// comparators (current-focus area first, then blocking before
// non-blocking, current before eventual, violated before unproven, then
// area order, then id). A reordered, omitted, or extra concern — in
// either inventory — fails these exact-array oracles. The sha256 semantic id is
// the digest of committed fixture bytes and is therefore deterministic —
// it moved when ac-10's oq-2 + stubs: entries changed the candidate
// content; the new value was captured from a real harness run, not typed
// by hand.
const SEMANTIC_ID =
  "context/semantic/sha256:9fe503eb5bb9fcaaf95da12b4f7b695c79abd6c8f793f4018e6c627895e8ef4d";

// The eventual closure blockers (spec/readiness-recovery ac-1): the
// journey now derives, for the draft feature refi-decline-flow, what will
// block its closure later — later-transition obligations, the unsatisfied
// outcome floor of each criterion, principal resolution, the spike-claimed
// oq-2, and the request's own semantic conflict row (present only because
// the harness serves with --context-request; a bare `verdi journey`
// discloses "no policy-conflict report was supplied" and lists the other
// seven). readinesspilot surfaces each as a non-blocking, eventual,
// violated-with-witness review/blocker/* concern, so they all sort after
// every current row and among themselves by id. Read from the harness's
// own derivation at design/refi-decline-flow (2026-09-19), not typed.
const EVENTUAL_REVIEW_BLOCKERS = [
  "review/blocker/conflict-semantic/sha256-9fe503eb5bb9fcaaf95da12b4f7b695c79abd6c8f793f4018e6c627895e8ef4d",
  "review/blocker/obligation-countersign-unproven/close/attestation/countersign",
  "review/blocker/obligation-fold-green-unproven/close/behavioral/fold-green",
  "review/blocker/outcome-floor/ac-1",
  "review/blocker/outcome-floor/ac-2",
  "review/blocker/outcome-floor/ac-3",
  "review/blocker/principal-resolution-unproven/close",
  "review/blocker/question-claimed/oq-2",
];

// The exact focus order: the current-focus area (shape-proposal) leads.
// shape/question/oq-2 (spec/uat-round-1 ac-10, PLAN.md §7 I-128) is a
// spike-claimed open question: non-blocking/eventual, so it still groups
// with the current-focus (shape-proposal) rows but sorts after
// shape/provenance (blocking ties, current before eventual) — oq-1 stays
// unclaimed, blocking/current, and first. The eventual review blockers
// close the list.
const ATTENTION_QUEUE = [
  "shape/question/oq-1",
  "shape/provenance",
  "shape/question/oq-2",
  "review/blocker/forge-facts-unavailable/merge",
  "review/blocker/obligation-author-vouch-unproven/merge/attestation/author-vouch",
  "context/verdict",
  "review/action",
  "success/contributor/attestation",
  "success/contributor/behavioral",
  "success/contributor/static",
  "context/disclosure/repository-remote-unknown",
  SEMANTIC_ID,
  "review/role/merge/attestation/author-vouch",
  ...EVENTUAL_REVIEW_BLOCKERS,
];

// Every and only the proven concerns, in existing AllConcerns order.
const COMPLETED_CHECKS = [
  "shape/board",
  "shape/mutation",
  "shape/outcome",
  "shape/problem",
  "context/mechanical/action:make-verify#complete",
  "context/mechanical/configuration:go-version#complete",
];

// The four stations in snapshot order: id, plain label, formal state.
const RAIL: Array<[id: string, label: string, formal: string]> = [
  ["shape-proposal", "Define the work", "unproven"],
  ["show-success", "Define success", "proven"],
  ["check-context", "Check constraints", "unproven"],
  ["request-review", "Get approval", "violated-with-witness"],
];

const TARGET_TITLE = "Refinancing decline flow";

// The isolated all-proven cockpit (cmd/e2eharness/readinessallproven.go):
// pinned to that fixture's own literals, independent of any render.
const ALL_PROVEN = {
  title: "Fully proven pilot flow",
  head: "e2eallproven0001",
  concerns: [
    "shape/problem",
    "success/contributor/static",
    "context/verdict",
    "review/action",
  ],
};

async function allProvenReadinessURL(page: Page): Promise<string> {
  const res = await page.request.get(
    `${CONTROL_URL}/readiness-all-proven-fixture`,
  );
  expect(res.ok()).toBe(true);
  return (await res.text()).trim() + "readiness";
}

// The first board-destination concern in focus order — the cockpit's
// first board link — pinned for exact event assertions below.
const BOARD_LINK_CONCERN = "shape/question/oq-1";
const BOARD_LINK_AREA = "shape-proposal";

const PLAIN_LABELS: Record<string, string> = {
  proven: "Ready",
  "violated-with-witness": "Needs attention",
  unproven: "Not enough evidence yet",
};

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

function focusIds(page: Page): Promise<Array<string | null>> {
  return page
    .locator(".readiness-queue [data-concern-id]")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-concern-id")));
}

async function derivedHead(page: Page): Promise<string> {
  const head = await page
    .locator('.readiness-target-tech dt:text-is("Head") + dd')
    .textContent();
  expect((head ?? "").trim()).not.toBe("");
  return (head ?? "").trim();
}

test("orientation and rail answer where-am-I with plain labels", async ({
  page,
}) => {
  await page.goto("/readiness");

  // Orientation: exact title first, current step, exact purpose copy.
  await expect(page.locator("h2.readiness-title")).toHaveText(TARGET_TITLE);
  await expect(page.locator(".readiness-step")).toHaveText(
    "Step 1 of 4 — Define the work",
  );
  await expect(page.locator(".readiness-purpose")).toHaveText(
    "This page derives readiness for the current design work on every request.",
  );
  // REAL DOM order: title → step → purpose all precede the target
  // technical metadata, and the shell's old leading metadata card is
  // gone from this page.
  const ordered = await page.evaluate(() => {
    const before = (a: Element | null, b: Element | null) =>
      !!a && !!b &&
      !!(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);
    const title = document.querySelector(".readiness-title");
    const step = document.querySelector(".readiness-step");
    const purpose = document.querySelector(".readiness-purpose");
    const target = document.querySelector(".readiness-target-tech");
    return before(title, step) && before(step, purpose) && before(purpose, target);
  });
  expect(ordered).toBe(true);
  await expect(page.locator(".metadata-card")).toHaveCount(0);
  // The target facts stay exact inside the trailing disclosure.
  const targetTech = page.locator(".readiness-target-tech");
  expect(await targetTech.textContent()).toContain(
    `spec/${SHOWCASE.DESIGN_SPEC}`,
  );
  await expect(page.locator("h2.readiness-title")).not.toContainText(
    `spec/${SHOWCASE.DESIGN_SPEC}`,
  );

  // Rail: snapshot order, plain labels, exact formal states in
  // data-state, one aria-current station on the current focus.
  const stations = page.locator(".readiness-rail .readiness-station");
  await expect(stations).toHaveCount(4);
  for (let i = 0; i < RAIL.length; i++) {
    const [id, label, formal] = RAIL[i];
    await expect(stations.nth(i)).toHaveAttribute("data-area-id", id);
    await expect(stations.nth(i)).toHaveAttribute("data-state", formal);
    await expect(stations.nth(i)).toContainText(label);
    await expect(stations.nth(i)).toContainText(PLAIN_LABELS[formal]);
    await expect(
      stations.nth(i).locator(".readiness-station-num"),
    ).toHaveText(String(i + 1));
  }
  await expect(page.locator('[aria-current="step"]')).toHaveCount(1);
  await expect(
    stations.first().locator('a[aria-current="step"]'),
  ).toHaveCount(1);
});

test("focus list shows exactly three priorities and the exact disclosed remainder", async ({
  page,
}) => {
  await page.goto("/readiness");

  // Exactly the first three, in the pinned order, ranked 1..3.
  expect(await focusIds(page)).toEqual(ATTENTION_QUEUE); // complete list is in the DOM…
  const visible = page.locator(
    ".readiness-queue [data-concern-id]:visible",
  );
  await expect(visible).toHaveCount(3);
  for (let i = 0; i < 3; i++) {
    await expect(visible.nth(i)).toHaveAttribute(
      "data-concern-id",
      ATTENTION_QUEUE[i],
    );
    await expect(visible.nth(i).locator(".readiness-rank")).toHaveText(
      String(i + 1),
    );
  }

  // Downstream disclosure: exactly the violated concerns in areas after
  // the current focus (the two current review blockers plus the eight
  // eventual ones) — nothing else counted.
  await expect(page.locator(".readiness-downstream")).toHaveText(
    "Known problems in later steps: 10",
  );

  // The inline control carries the exact remaining count; expanding
  // reveals the complete ordered remainder; the open control reads
  // "Show fewer"; collapsing hides it again. No event is recorded.
  const more = page.locator("details.readiness-more");
  const summary = more.locator(".readiness-more-summary");
  await expect(summary).toHaveText(/18 more items\s*Show fewer/); // both spans in DOM…
  await expect(more.locator(".readiness-more-closed")).toBeVisible();
  await expect(more.locator(".readiness-more-open")).toBeHidden();

  const eventsBefore = await pilotEvents(page);
  await summary.click();
  await expect(more).toHaveAttribute("open", "");
  await expect(more.locator(".readiness-more-open")).toBeVisible();
  await expect(more.locator(".readiness-more-closed")).toBeHidden();
  const revealed = more.locator("[data-concern-id]");
  await expect(revealed).toHaveCount(18);
  for (let i = 0; i < 18; i++) {
    await expect(revealed.nth(i)).toHaveAttribute(
      "data-concern-id",
      ATTENTION_QUEUE[i + 3],
    );
    await expect(revealed.nth(i).locator(".readiness-rank")).toHaveText(
      String(i + 4),
    );
  }
  await summary.click();
  await expect(more.locator("[data-concern-id]").first()).toBeHidden();
  expect(await pilotEvents(page)).toEqual(eventsBefore); // expansion records nothing
});

test("focus and completed checks are lossless, disjoint, and complete", async ({
  page,
}) => {
  await page.goto("/readiness");

  const queueIds = await focusIds(page);
  expect(queueIds).toEqual(ATTENTION_QUEUE);

  const completedIds = await page
    .locator(".readiness-completed [data-concern-id]")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-concern-id")));
  expect(completedIds).toEqual(COMPLETED_CHECKS);

  // Disjoint union covering every concern exactly once.
  const union = new Set([...queueIds, ...completedIds]);
  expect(union.size).toBe(queueIds.length + completedIds.length);
  const totalRows = await page.locator("[data-concern-id]").count();
  expect(totalRows).toBe(queueIds.length + completedIds.length);

  // Completed rows carry the plain Ready label; their technical details
  // retain the exact formal state.
  const firstDone = page
    .locator(".readiness-completed [data-concern-id]")
    .first();
  await expect(firstDone.locator(".readiness-state")).toHaveText("Ready");
  await firstDone.locator(".readiness-tech summary").click();
  await expect(firstDone.locator(".readiness-tech-facts")).toContainText(
    "proven",
  );
});

test("plain state labels pair with exact formal technical details", async ({
  page,
}) => {
  await page.goto("/readiness");

  // Every chip pairs its formal modifier class with the plain label.
  const chips = await page
    .locator(".readiness-state")
    .evaluateAll((els) =>
      els.map((el) => [
        el.className.replace(/.*readiness-state--/, ""),
        el.textContent?.trim(),
      ]),
    );
  expect(chips.length).toBeGreaterThan(0);
  for (const [formal, text] of chips) {
    expect(text).toBe(PLAIN_LABELS[formal as string]);
  }

  // Violated and unproven stay visibly distinct beyond color.
  const violated = page
    .locator(".readiness-state--violated-with-witness")
    .first();
  const unproven = page.locator(".readiness-state--unproven").first();
  await expect(violated).toHaveCSS("border-top-style", "solid");
  await expect(unproven).toHaveCSS("border-top-style", "dashed");

  // A violated concern's technical details carry the exact formal facts.
  // Index 3, not 2: shape/question/oq-2 (non-blocking, spike-claimed) now
  // occupies index 2, ahead of this still-violated review blocker.
  await page.locator("details.readiness-more > summary").click();
  const blocker = page.locator(
    `[data-concern-id="${ATTENTION_QUEUE[3]}"]`,
  );
  await blocker.locator(".readiness-tech summary").click();
  const facts = blocker.locator(".readiness-tech-facts");
  await expect(facts).toContainText("violated-with-witness");
  await expect(facts).toContainText(ATTENTION_QUEUE[3]);
  await expect(facts).toContainText("request-review");
});

test("a spike-claimed open question is non-blocking/eventual; the unclaimed one stays blocking (ac-10)", async ({
  page,
}) => {
  await page.goto("/readiness");
  await page.locator("details.readiness-more > summary").click();

  const claimed = page.locator('[data-concern-id="shape/question/oq-2"]');
  await claimed.locator(".readiness-tech summary").click();
  await expect(
    claimed.locator('dt:text-is("Blocking") + dd'),
  ).toHaveText("false");
  await expect(
    claimed.locator('dt:text-is("Timing") + dd'),
  ).toHaveText("eventual");

  const unclaimed = page.locator('[data-concern-id="shape/question/oq-1"]');
  await unclaimed.locator(".readiness-tech summary").click();
  await expect(
    unclaimed.locator('dt:text-is("Blocking") + dd'),
  ).toHaveText("true");
  await expect(
    unclaimed.locator('dt:text-is("Timing") + dd'),
  ).toHaveText("current");
});

test("every eventual review blocker is non-blocking with Timing eventual in its technical details (ac-1)", async ({
  page,
}) => {
  await page.goto("/readiness");
  await page.locator("details.readiness-more > summary").click();

  for (const id of EVENTUAL_REVIEW_BLOCKERS) {
    const row = page.locator(`[data-concern-id="${id}"]`);
    await expect(row).toHaveAttribute("data-area-id", "request-review");
    await expect(row.locator(".readiness-state")).toHaveText("Needs attention");
    await row.locator(".readiness-tech summary").click();
    const facts = row.locator(".readiness-tech-facts");
    await expect(facts.locator('dt:text-is("Blocking") + dd')).toHaveText(
      "false",
    );
    await expect(facts.locator('dt:text-is("Timing") + dd')).toHaveText(
      "eventual",
    );
    await expect(facts).toContainText("violated-with-witness");
    await expect(facts).toContainText(id);
  }
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
  await expect(popup.getByRole("button", { name: "Add sticky" })).toBeVisible();

  // The source tab is preserved and the primary click appended EXACTLY
  // ONE event — the expected board-link-followed and nothing else.
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
  // The nearest CLI-destination concern (review/blocker/forge-facts-
  // unavailable/merge) now sits past the top three (ac-10 moved the
  // spike-claimed shape/question/oq-2 in ahead of it); expand the
  // disclosure so its rendered tokens are visible, not innerText-empty.
  await page.locator("details.readiness-more > summary").click();

  const cli = page.locator(".readiness-cli").first();
  const tokens = await cli.locator(".readiness-cli-token").allInnerTexts();
  expect(tokens.length).toBeGreaterThan(0);
  expect(tokens[0]).toBe("verdi");

  await cli.evaluate((el) => {
    const selection = window.getSelection()!;
    selection.removeAllRanges();
    selection.selectAllChildren(el);
  });
  await page.keyboard.press("ControlOrMeta+c");

  const copied = await page.evaluate(() => navigator.clipboard.readText());
  expect(copied.split(/\s+/).filter((t) => t !== "")).toEqual(tokens);

  const copyEvents = (await pilotEvents(page)).filter(
    (e) => e.event === "cli-fallback-copied",
  );
  expect(copyEvents.length).toBeGreaterThanOrEqual(1);
  expect(copyEvents[0].concern_id).not.toBe("");
});

test("keyboard traversal reaches every cockpit landmark", async ({ page }) => {
  await page.goto("/readiness");
  await page.emulateMedia({ reducedMotion: "reduce" });

  // The only CLI-destination landmark in this fixture now sits past the
  // top three (ac-10 moved the spike-claimed shape/question/oq-2 in
  // ahead of it), so reaching it needs the disclosure open first — via
  // the keyboard, consistent with this test's own traversal.
  await page.locator(".readiness-more-summary").focus();
  await page.keyboard.press("Enter");
  await expect(page.locator("details.readiness-more")).toHaveAttribute(
    "open",
    "",
  );

  const reached = new Set<string>();
  await page.locator("body").press("Tab");
  for (let i = 0; i < 160; i++) {
    const kind = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el) return "";
      const cls = String(el.className || "");
      if (cls.includes("readiness-stale")) return "stale";
      if (cls.includes("readiness-station-link")) return "station";
      if (cls.includes("readiness-more-summary")) return "expansion";
      if (cls.includes("readiness-board-link")) return "board";
      if (cls.includes("readiness-cli")) return "cli";
      if (el.matches("details.readiness-tech > summary")) return "tech";
      return "";
    });
    if (kind) reached.add(kind);
    if (reached.size === 6) break;
    await page.keyboard.press("Tab");
  }
  expect([...reached].sort()).toEqual([
    "board",
    "cli",
    "expansion",
    "stale",
    "station",
    "tech",
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

  const requests: string[] = [];
  page.on("request", (r) => requests.push(r.url()));

  await page.evaluate(() => {
    const w = window as unknown as { __seen: unknown[] };
    w.__seen = [];
    document.addEventListener("verdi:readiness-pilot", (e) => {
      w.__seen.push((e as CustomEvent).detail);
    });
  });

  const htmlBefore = await page.evaluate(() => document.body.innerHTML);

  await page.locator(".readiness-stale").click();
  await page.locator(".readiness-queue [data-concern-id]").first().click();
  await page.evaluate(() => {
    const link = document.querySelector<HTMLAnchorElement>(
      ".readiness-station-link",
    )!;
    for (let i = 0; i < 210; i++) link.click();
  });

  const events = await pilotEvents(page);
  expect(events).toHaveLength(200);
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
  expect(events[events.length - 1].event).toBe("area-inspected");
  expect(events[events.length - 1].area_id).toBe("shape-proposal");

  const seen = await page.evaluate(
    () => (window as unknown as { __seen: PilotEvent[] }).__seen,
  );
  expect(seen.length).toBeGreaterThanOrEqual(212);
  expect(seen[seen.length - 1]).toEqual(events[events.length - 1]);

  expect(requests).toEqual([]);
  const persistence = await page.evaluate(() => ({
    local: window.localStorage.length,
    session: window.sessionStorage.length,
    cookie: document.cookie,
  }));
  expect(persistence).toEqual({ local: 0, session: 0, cookie: "" });
  expect(await page.evaluate(() => document.body.innerHTML)).toBe(htmlBefore);
});

test("an edit through the existing board leaves the open cockpit tab unchanged while the next request derives it afresh at the same HEAD", async ({
  page,
}) => {
  await page.goto("/readiness");
  const head = await derivedHead(page);

  // The derivation stamp (ac-2): plain label first, the exact HEAD this
  // request looked at, nothing about restarting.
  const notice = page.locator(".readiness-stale");
  await expect(notice).toHaveAttribute("aria-label", "Derivation stamp");
  await expect(notice.locator("strong")).toHaveText("Derivation stamp.");
  await expect(notice).toContainText(
    `Derived at HEAD ${head} for this request.`,
  );
  await expect(notice).not.toContainText("Startup snapshot");
  await expect(notice).not.toContainText("restart verdi serve");

  const bodyBefore = await (await page.request.get("/readiness")).text();
  expect(bodyBefore).not.toMatch(/data-concern-id="shape\/board\/question\//);
  const eventsBefore = await pilotEvents(page);
  const domBefore = await page.evaluate(
    () => document.querySelector("main.content")!.outerHTML,
  );

  const [popup] = await Promise.all([
    page.waitForEvent("popup"),
    page.locator(".readiness-board-link").first().click(),
  ]);
  await popup.waitForLoadState();
  const probe = await addSticky(
    popup,
    "readiness pilot probe: an open board question",
  );

  // The already-rendered tab never refreshes itself (no live refresh, no
  // polling, no recording): its DOM and its event log are exactly what
  // they were before the edit.
  const domAfter = await page.evaluate(
    () => document.querySelector("main.content")!.outerHTML,
  );
  expect(domAfter).toBe(domBefore);
  await expect(page.locator(".readiness-stale")).toContainText(
    `Derived at HEAD ${head} for this request.`,
  );
  const events = await pilotEvents(page);
  expect(events.slice(0, eventsBefore.length)).toEqual(eventsBefore);

  // But the NEXT request derives afresh (ac-2): the same HEAD — the
  // sticky is working-tree scratch, nothing was committed — and the new
  // open board question is now a shape concern of its own.
  const bodyAfter = await (await page.request.get("/readiness")).text();
  expect(bodyAfter).not.toBe(bodyBefore);
  expect(bodyAfter).toContain(`Derived at HEAD ${head} for this request.`);
  expect(bodyAfter).toMatch(/data-concern-id="shape\/board\/question\/[^"]+"/);

  // Deleting the probe through the same board restores the store, and the
  // same ref at the same HEAD derives identical bytes again (ac-2) — so
  // every later test inherits the pinned posture, not this probe.
  await probe.getByRole("button", { name: "Delete sticky" }).click();
  await expectAutosaved(popup);
  await popup.close();
  const bodyRestored = await (await page.request.get("/readiness")).text();
  expect(bodyRestored).toBe(bodyBefore);
});

test("420px shows exactly the first three priorities before expansion", async ({
  page,
}) => {
  await page.setViewportSize({ width: 420, height: 800 });
  await page.goto("/readiness");

  const visible = page.locator(".readiness-queue [data-concern-id]:visible");
  await expect(visible).toHaveCount(3);
  for (let i = 0; i < 3; i++) {
    await expect(visible.nth(i)).toHaveAttribute(
      "data-concern-id",
      ATTENTION_QUEUE[i],
    );
  }
});

test("the all-proven snapshot renders the honest complete posture", async ({
  page,
}) => {
  await page.goto(await allProvenReadinessURL(page));

  await expect(page.locator("h2.readiness-title")).toHaveText(
    ALL_PROVEN.title,
  );
  await expect(page.locator(".readiness-step")).toHaveText(
    "All four steps are complete.",
  );
  await expect(page.locator('[aria-current]')).toHaveCount(0);
  await expect(page.locator(".readiness-queue-empty")).toHaveText(
    "Nothing needs attention: every check in this snapshot is proven.",
  );
  await expect(page.locator(".readiness-queue [data-concern-id]")).toHaveCount(
    0,
  );
  await expect(page.locator(".readiness-downstream")).toHaveCount(0);
  await expect(page.locator(".readiness-stale")).toHaveAttribute(
    "aria-label",
    "Derivation stamp",
  );
  await expect(page.locator(".readiness-stale")).toContainText(
    `Derived at HEAD ${ALL_PROVEN.head} for this request.`,
  );

  // Every concern is present under completed checks with its exact
  // formal facts.
  const completedIds = await page
    .locator(".readiness-completed [data-concern-id]")
    .evaluateAll((els) => els.map((el) => el.getAttribute("data-concern-id")));
  expect(completedIds).toEqual(ALL_PROVEN.concerns);
  for (const id of ALL_PROVEN.concerns) {
    const row = page.locator(`[data-concern-id="${id}"]`);
    await expect(row.locator(".readiness-state")).toHaveText("Ready");
    await row.locator(".readiness-tech summary").click();
    const facts = row.locator(".readiness-tech-facts");
    await expect(facts).toContainText("proven");
    await expect(facts).toContainText(id);
  }
});

test("light and dark schemes keep the cockpit legible with identical facts", async ({
  page,
}) => {
  const palette = async () =>
    page.evaluate(() => {
      const luminance = (color: string) => {
        const m = color.match(/\d+(\.\d+)?/g)!.map(Number);
        return (0.2126 * m[0] + 0.7152 * m[1] + 0.0722 * m[2]) / 255;
      };
      const bodyBg = getComputedStyle(document.body).backgroundColor;
      const ink = getComputedStyle(
        document.querySelector(".readiness-summary")!,
      ).color;
      return { bg: luminance(bodyBg), ink: luminance(ink) };
    });

  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/readiness");
  const light = await palette();
  const lightIds = await focusIds(page);

  await page.emulateMedia({ colorScheme: "dark" });
  await page.reload();
  const dark = await palette();
  const darkIds = await focusIds(page);

  // The selected palette is actually used: a light ground in light mode,
  // a dark ground in dark mode, and legible ink contrast in both.
  expect(light.bg).toBeGreaterThan(0.5);
  expect(dark.bg).toBeLessThan(0.5);
  expect(Math.abs(light.ink - light.bg)).toBeGreaterThan(0.3);
  expect(Math.abs(dark.ink - dark.bg)).toBeGreaterThan(0.3);
  // Same facts, same order, in both schemes.
  expect(lightIds).toEqual(ATTENTION_QUEUE);
  expect(darkIds).toEqual(ATTENTION_QUEUE);
});

test("keyboard traversal reaches technical details for every priority and completed check", async ({
  page,
}) => {
  await page.goto("/readiness");

  // Expand the remainder with the keyboard so every priority's control
  // is in the tab order.
  await page.locator(".readiness-more-summary").focus();
  await page.keyboard.press("Enter");
  await expect(page.locator("details.readiness-more")).toHaveAttribute(
    "open",
    "",
  );

  const reachable = new Set<string>();
  await page.locator("body").press("Tab");
  for (let i = 0; i < 300; i++) {
    const id = await page.evaluate(() => {
      const el = document.activeElement;
      if (!el || !el.matches("details.readiness-tech > summary")) return "";
      const row = el.closest("[data-concern-id]");
      return row ? row.getAttribute("data-concern-id") || "" : "";
    });
    if (id) reachable.add(id);
    if (reachable.size === ATTENTION_QUEUE.length + COMPLETED_CHECKS.length) {
      break;
    }
    await page.keyboard.press("Tab");
  }
  expect([...reachable].sort()).toEqual(
    [...ATTENTION_QUEUE, ...COMPLETED_CHECKS].sort(),
  );
});

test("420px: pinned rail hides nothing, anchors reveal disclosed rows, long values fit", async ({
  page,
}) => {
  await page.setViewportSize({ width: 420, height: 800 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/readiness");

  const widths = await page.evaluate(() => ({
    doc: document.documentElement.scrollWidth,
    clipped: Array.from(
      document.querySelectorAll(
        ".readiness-card, .readiness-row, .readiness-cli, .readiness-tech-facts",
      ),
    )
      .filter((el) => el.scrollWidth > el.clientWidth + 1)
      .map((el) => el.className),
  }));
  expect(widths.doc).toBeLessThanOrEqual(420);
  expect(widths.clipped).toEqual([]);

  const railBottom = () =>
    page.evaluate(
      () =>
        document.querySelector(".readiness-rail")!.getBoundingClientRect()
          .bottom,
    );

  // A rail anchor whose target sits INSIDE the collapsed remainder must
  // reveal it (native details auto-expansion) and land below the rail.
  await page.locator('a[href="#area-check-context"]').click();
  await expect(page.locator("details.readiness-more")).toHaveAttribute(
    "open",
    "",
  );
  let top = await page.evaluate(
    () =>
      document.getElementById("area-check-context")!.getBoundingClientRect()
        .top,
  );
  expect(top).toBeGreaterThanOrEqual((await railBottom()) - 1);

  // Keyboard-activated anchor to a visible target likewise.
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.locator('a[href="#area-request-review"]').focus();
  await page.keyboard.press("Enter");
  top = await page.evaluate(
    () =>
      document.getElementById("area-request-review")!.getBoundingClientRect()
        .top,
  );
  expect(top).toBeGreaterThanOrEqual((await railBottom()) - 1);
});
