import { test, expect, Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { SHOWCASE, boardPath } from "./fixtures";
import { addSticky, dragToTrash, uncommittedIndicator } from "./helpers";

// Wave 6 Task 2 — the ASD synchronized workbench (design §§3-6, SI-163/
// SI-165/SI-167/SI-168; ASD AC-2/AC-4..AC-8, CO-9 §Browser behavior).
//
// Every case here drives the REAL `verdi serve` subprocess: the six
// application operations ride POST <page>/api/<exact-operation>, every
// domain mutation is one typed mutate_draft transaction through the shared
// core (with the kernel's explicit unauthenticated-human attribution), and
// the page's freshness contract is the conditional /snapshot projection.
//
// State assertions ride chip classes and data attributes, never innerText
// (lane rule).

const DESIGN = () => boardPath(SHOWCASE.DESIGN_SPEC);
const DRAFT_B = () =>
  "/b/" +
  encodeURIComponent(SHOWCASE.SHOWCASE_DRAFT_BRANCH) +
  "/board/spec/" +
  SHOWCASE.SHOWCASE_DRAFT_SPEC;

// snapshotOf fetches the conditional projection's machine facts.
async function snapshotOf(page: Page, path: string) {
  const resp = await page.request.get(path + "/snapshot");
  expect(resp.status()).toBe(200);
  return resp.json();
}

// envelopeFrom builds one typed mutation envelope against a snapshot's
// exact base — precisely what the page's own transport posts.
function envelopeFrom(
  snap: { base_digest: string; base_spec_b64: string; expected: object },
  spec: string,
  operations: object[],
  extras?: { graduate?: string[]; del?: string[] },
) {
  const envelope: Record<string, unknown> = {
    request: {
      schema: "verdi.draftmutation/v1",
      spec: "spec/" + spec,
      base_digest: snap.base_digest,
      base_spec_b64: snap.base_spec_b64,
      expected: snap.expected,
      operations,
    },
  };
  if (extras?.graduate?.length) envelope.graduate_annotations = extras.graduate;
  if (extras?.del?.length) envelope.delete_annotations = extras.del;
  return envelope;
}

async function postMutate(
  page: Page,
  path: string,
  spec: string,
  operations: object[],
  extras?: { graduate?: string[]; del?: string[] },
) {
  const snap = await snapshotOf(page, path);
  return page.request.post(path + "/api/mutate_draft", {
    data: envelopeFrom(snap, spec, operations, extras),
    headers: { "Content-Type": "application/json" },
  });
}

// expectClean asserts a mutation landed, carrying the full response body
// into the failure message so a refusal names itself.
async function expectClean(resp: import("@playwright/test").APIResponse) {
  const body = await resp.json();
  expect(body.result, JSON.stringify(body)).toBeTruthy();
  return body;
}

// addFreshQuestion declares a NEW open question through one typed
// transaction (id from the server's own next-id data), returning its id —
// an order-independent mutation target for state-heavy suite runs.
async function addFreshQuestion(page: Page, marker: string): Promise<string> {
  const id = (await page.getByTestId("board").getAttribute("data-next-id-oq"))!;
  const resp = await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
    { op: "add-question", id, text: marker, anchor: "#" + id },
  ]);
  await expectClean(resp);
  return id;
}

// ---------------------------------------------------------------------------
// Shell, posture, and the closed action surface
// ---------------------------------------------------------------------------
test.describe("shell and posture", () => {
  test("the board page presents the four-area shell and posture header", async ({ page }) => {
    await page.goto(DESIGN());
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    await expect(page.getByTestId("asd-posture")).toBeVisible();
    await expect(page.getByTestId("asd-shell")).toBeVisible();
    const stations = page.locator('[data-testid="asd-shell"] .readiness-station');
    await expect(stations).toHaveCount(4);
    // One deterministic focus, spoken as Step N of 4 (SI-125).
    await expect(page.locator(".readiness-station--focus")).toHaveCount(1);
    await expect(page.getByTestId("asd-step")).toContainText("of 4");
    // Exactly-three preview: never more than three ranked cards outside
    // the inline remainder.
    const preview = page.locator(".asd-focus > ol.readiness-queue-list > li");
    expect(await preview.count()).toBeLessThanOrEqual(3);
    // Explicit sequencing note (F-04) and downstream count (F-02).
    await expect(page.getByTestId("asd-sequence-note")).toBeVisible();
    await expect(page.getByTestId("asd-downstream")).toBeVisible();
    // States speak through chip classes; the formal state rides data-state.
    expect(await page.locator(".readiness-station[data-state]").count()).toBe(4);
    // Repository posture facts (design §4.2).
    await page.locator(".asd-posture-tech > summary").click();
    const tech = page.locator(".asd-posture-tech");
    await expect(tech).toContainText(SHOWCASE.DESIGN_BRANCH);
    await expect(page.getByTestId("asd-posture-bytes")).toHaveAttribute("data-state", "proposed");
  });

  test("no browser control claims governance authority", async ({ page }) => {
    await page.goto(DESIGN());
    // The human-review row is a plain label with formal secondary
    // evidence — never an accept/approve/merge control (DC-4, §4.2: "no
    // action button changes accepted state by itself"). On a busy wall it
    // may rank below the three-item preview: expand the exact-count
    // remainder inline first (SI-125 keeps it lossless).
    const more = page.locator('[data-testid="asd-more"] > summary');
    if (await more.count()) await more.click();
    await expect(page.getByTestId("asd-human-review").first()).toBeVisible();
    await expect(page.getByRole("button", { name: /accept|approve|merge/i })).toHaveCount(0);
    const guidance = page.locator('[data-testid="asd-guidance-review/acceptance"]');
    await expect(guidance).toContainText("owner's merge");
  });

  test("unknown actions and malformed bodies fail before any application call", async ({ page }) => {
    await page.goto(DESIGN());
    // Unknown operation → 404 out of the closed inventory (SI-167).
    const unknown = await page.request.post(DESIGN() + "/api/not-an-operation", { data: {} });
    expect(unknown.status()).toBe(404);
    // A deleted legacy DOMAIN action is genuinely out of the union.
    const legacy = await page.request.post(DESIGN() + "/api/edit-text", {
      data: { id: "ac-1", text: "x" },
    });
    expect(legacy.status()).toBe(404);
    // Strict grammar: unknown fields, duplicate keys, nulls, trailing data.
    for (const body of [
      `{"bogus":1}`,
      `{"request":null}`,
      `{"graduate_annotations":[],"graduate_annotations":[]}`,
      `{}{}`,
    ]) {
      const resp = await page.request.post(DESIGN() + "/api/mutate_draft", {
        headers: { "Content-Type": "application/json" },
        data: body,
      });
      expect(resp.status(), body).toBe(400);
    }
  });
});

// ---------------------------------------------------------------------------
// The six application operations through the browser surface
// ---------------------------------------------------------------------------
test.describe("the six application operations", () => {
  test("get_board returns the canonical board envelope", async ({ page }) => {
    await page.goto(DESIGN());
    const resp = await page.request.post(DESIGN() + "/api/get_board", { data: {} });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.schema).toBe("verdi.design-board/v1");
    expect(body.spec).toBe(SHOWCASE.DESIGN_SPEC);
    expect(body.identity.branch).toBe(SHOWCASE.DESIGN_BRANCH);
  });

  test("get_design_capabilities returns the agent posture on this adopted-policy wall", async ({ page }) => {
    await page.goto(DESIGN());
    const resp = await page.request.post(DESIGN() + "/api/get_design_capabilities", { data: {} });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.schema).toBe("verdi.design-capabilities/v1");
    // The harness store adopts design_assistance mode proposal-only: the
    // delegated-agent posture is honestly not mutable, while THIS page's
    // own browser-human mutations proceed (SI-176).
    expect(body.policy_mode).toBe("proposal-only");
    expect(body.mutable).toBe(false);
    expect(body.mutability_refusal.precondition).toBe("policy-mode");
  });

  test("get_design_context is served with its typed classification", async ({ page }) => {
    await page.goto(DESIGN());
    const resp = await page.request.post(DESIGN() + "/api/get_design_context", { data: {} });
    const body = await resp.json();
    // The classification lives IN the body (§4.3): either a clean context
    // envelope or the typed failure envelope — never an untyped error.
    if (body.schema === "verdi.design-context/v1") {
      expect(resp.status()).toBe(200);
    } else {
      expect(body.schema).toBe("verdi.design-failure/v1");
      expect(["verdict", "operational"]).toContain(body.classification);
    }
  });

  test("mutate_draft lands one typed transaction and a fresh projection", async ({ page }) => {
    await page.goto(DESIGN());
    const marker = "typed transaction through the shared core [50-e2e]";
    const resp = await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
      {
        op: "edit-ac",
        id: SHOWCASE.AC_IDS[0],
        text: marker,
        evidence: ["attestation"],
        anchor: "#" + SHOWCASE.AC_IDS[0],
      },
    ]);
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.result.schema).toBe("verdi.draftmutation-result/v1");
    expect(body.projection.revision).toBeTruthy();
    expect(body.projection.dirty).toBe(true);
    // The open page's visible polling picks the change up (SI-165).
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[0])).toContainText("[50-e2e]", {
      timeout: 8_000,
    });
  });

  test("get_design_provenance carries the v2 closed policy union on the wire (F5)", async ({ page }) => {
    await page.goto(DESIGN());
    const resp = await page.request.post(DESIGN() + "/api/get_design_provenance", { data: {} });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.schema).toBe("verdi.design-provenance-result/v1");
    expect(body.entries.length).toBeGreaterThan(0);
    const entry = body.entries[body.entries.length - 1];
    // SI-163: the browser mutation's attribution is the kernel's explicit
    // unauthenticated-human marker — no principal, no harness.
    expect(entry.attribution.unauthenticated).toBe(true);
    expect(entry.attribution.principal_id).toBeUndefined();
    expect(entry.harness).toBeUndefined();
    // F5: the v2 policy union, wire-level: the resolved arm carries the
    // effective digest; the v1 policy_digest field is gone.
    expect(entry.schema).toBe("verdi.design-provenance/v2");
    expect(entry.policy.state).toBe("resolved");
    expect(entry.policy.digest).toMatch(/^sha256:/);
    expect(entry.policy_digest).toBeUndefined();
  });

  test("prepare_design_review derives the semantic packet", async ({ page }) => {
    await page.goto(DESIGN());
    const resp = await page.request.post(DESIGN() + "/api/prepare_design_review", { data: {} });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.schema).toBe("verdi.design-review/v1");
    expect(Array.isArray(body.changes)).toBe(true);
    expect(body).toHaveProperty("unclassified_edits");
  });

  test("provenance and semantic review are on-demand panels, collapsed by default", async ({ page }) => {
    await page.goto(DESIGN());
    const prov = page.getByTestId("asd-provenance");
    const review = page.getByTestId("asd-review");
    await expect(prov).toBeVisible();
    await expect(review).toBeVisible();
    // Collapsed by default: no derived content until opened (AC-4/DC-7 —
    // provenance stays off the main board).
    await expect(prov.locator(".asd-panel-json")).toHaveCount(0);
    await prov.locator("summary").click();
    await expect(prov.locator(".asd-panel-json")).toBeVisible();
    await expect(prov).toContainText("never evidence");
    await review.locator("summary").click();
    await expect(review.locator(".asd-panel-json")).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Failure classes: verdict, stale (zero mutation), operational
// ---------------------------------------------------------------------------
test.describe("failure classes", () => {
  test("a kernel verdict renders its exact typed classification", async ({ page }) => {
    await page.goto(DESIGN());
    const resp = await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
      { op: "edit-ac", id: "ac-99", text: "x", evidence: ["attestation"], anchor: "#ac-99" },
    ]);
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.failure.schema).toBe("verdi.design-failure/v1");
    expect(body.failure.classification).toBe("verdict");
    expect(body.failure.code).toBeTruthy();
    // A verdict still returns a usable fresh projection (§4.3).
    expect(body.projection.revision).toBeTruthy();
  });

  test("a stale action is refused with zero mutation and a fresh projection", async ({ page }) => {
    await page.goto(DESIGN());
    const snap = await snapshotOf(page, DESIGN());
    const first = await page.request.post(DESIGN() + "/api/mutate_draft", {
      data: envelopeFrom(snap, SHOWCASE.DESIGN_SPEC, [
        {
          op: "edit-ac",
          id: SHOWCASE.AC_IDS[1],
          text: "winner of the race [50-stale]",
          evidence: ["behavioral", "attestation"],
          anchor: "#" + SHOWCASE.AC_IDS[1],
        },
      ]),
    });
    await expectClean(first);
    // The SAME base again: the application's expected-identity/base
    // precondition refuses; nothing merges, nothing lands (DC-5).
    const second = await page.request.post(DESIGN() + "/api/mutate_draft", {
      data: envelopeFrom(snap, SHOWCASE.DESIGN_SPEC, [
        {
          op: "edit-ac",
          id: SHOWCASE.AC_IDS[1],
          text: "loser of the race [50-stale-lost]",
          evidence: ["behavioral", "attestation"],
          anchor: "#" + SHOWCASE.AC_IDS[1],
        },
      ]),
    });
    expect(second.status()).toBe(200);
    const refusal = await second.json();
    expect(refusal.stale).toBeTruthy();
    expect(refusal.stale.code).toBe("stale-base");
    expect(refusal.stale.current_digest).toMatch(/^sha256:/);
    expect(refusal.projection.revision).toBeTruthy();
    // Zero mutation: the board carries the winner, never the loser.
    const after = await snapshotOf(page, DESIGN());
    expect(after.html).toContain("[50-stale]");
    expect(after.html).not.toContain("[50-stale-lost]");
  });

  test("an operational failure renders a stable non-favorable panel", async ({ page }) => {
    // stale-decline's committed context pin does not resolve in the
    // hermetic history — the application core's own operational
    // classification, surfaced verbatim (§4.3: never a favorable state).
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    const resp = await page.request.post(
      boardPath(SHOWCASE.READONLY_SPEC) + "/api/get_design_context",
      { data: {} },
    );
    const body = await resp.json();
    expect(body.schema).toBe("verdi.design-failure/v1");
    expect(body.classification).toBe("operational");
    expect(resp.status()).toBe(500);
    expect(body.code).toBeTruthy();
    expect(body.detail).toBeTruthy();
  });

  test("a projection failure after a landed mutation renders applied-but-unrefreshed", async ({ page }) => {
    // Codex correction round 1, finding 2 (client half): the server's
    // disclosure shape for a post-commit projection failure is pinned by
    // the Go adapter test; here the page's OWN transport receives exactly
    // that shape (the real response with the fresh projection withheld
    // and the typed operational disclosure added) and must render
    // applied-but-unrefreshed — never failure-to-apply (§4.3: no partial
    // action effect may be hidden).
    await page.goto(DESIGN());
    await page.route("**/api/mutate_draft", async (route) => {
      const resp = await route.fetch();
      const body = await resp.json();
      delete body.projection;
      body.projection_failure = {
        schema: "verdi.design-failure/v1",
        classification: "operational",
        code: "projection-unavailable",
        detail: "rendering fresh projection: injected client probe",
      };
      await route.fulfill({
        status: 500,
        contentType: "application/json; charset=utf-8",
        body: JSON.stringify(body),
      });
    });
    await page.locator("#asd-add-object").click();
    await page.locator("#asd-op-kind").selectOption("add-question");
    const preview = await page.getByTestId("asd-op-id-preview").textContent();
    const newID = preview!.replace("will be declared as ", "").trim();
    await page.getByTestId("asd-op-text").fill("applied but unrefreshed probe [f2-client]");
    await page.getByTestId("asd-op-ok").click();
    const result = page.getByTestId("asd-last-result");
    // Applied-but-unrefreshed, never a failed mutation.
    await expect(result).toHaveAttribute("data-result-kind", "applied-unrefreshed", {
      timeout: 10_000,
    });
    await expect(result).toContainText("landed");
    await expect(result).toContainText("could not be rendered");
    // The wall recovers through the ordinary conditional refresh once the
    // projection is derivable again — the landed write appears.
    await page.unroute("**/api/mutate_draft");
    await expect(page.getByTestId("card-" + newID)).toBeVisible({ timeout: 10_000 });
    // Cleanup through the same typed surface.
    await expectClean(
      await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
        { op: "remove-question", id: newID },
      ]),
    );
  });
});

// ---------------------------------------------------------------------------
// Proposed vs accepted posture; policy postures
// ---------------------------------------------------------------------------
test.describe("posture and policy", () => {
  test("an accepted wall renders read-only with its accepted posture and refuses writes", async ({ page }) => {
    await page.goto(boardPath(SHOWCASE.READONLY_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "readonly");
    await expect(page.getByTestId("asd-posture-bytes")).toHaveAttribute(
      "data-state",
      "accepted-pending-build",
    );
    const snap = await snapshotOf(page, boardPath(SHOWCASE.READONLY_SPEC));
    const resp = await page.request.post(
      boardPath(SHOWCASE.READONLY_SPEC) + "/api/mutate_draft",
      {
        data: {
          request: {
            schema: "verdi.draftmutation/v1",
            spec: "spec/" + SHOWCASE.READONLY_SPEC,
            base_digest: snap.base_digest,
            base_spec_b64: snap.base_spec_b64,
            // Syntactically valid placeholders: the adapter's own
            // read-only-mode gate refuses BEFORE the kernel would verify
            // this expected identity.
            expected: {
              checkout: "/never-reached",
              branch: "main",
              head: "0000000000000000000000000000000000000000",
            },
            operations: [{ op: "set-problem", text: "x", anchor: "#problem" }],
          },
        },
      },
    );
    expect(resp.status()).toBe(403);
  });

  test("a policy-less draft mutates with the honest not-applicable posture", async ({ page }) => {
    // The payoff-quote-portal draft's branch tree carries no .verdi/policy
    // (it was cut before the policy fixtures were committed): SI-176's
    // browser-human path proceeds and records {"state":"not-applicable"}.
    await page.goto(DRAFT_B());
    await expect(page.getByTestId("board")).toHaveAttribute("data-board-mode", "authoring");
    // The shell's check-context area discloses the absence honestly —
    // an unproven chip, never a violation, never silence.
    // The nonblocking policy disclosure ranks below the three-item
    // preview: expand the exact-count remainder inline (SI-125), then
    // assert the honest unproven chip.
    const more = page.locator('[data-testid="asd-more"] > summary');
    if (await more.count()) await more.click();
    const policyRow = page.locator('[data-concern-id="context/policy"]').first();
    await expect(policyRow).toBeVisible();
    await expect(policyRow.locator(".readiness-state--unproven")).toBeVisible();

    const marker = "not-applicable posture probe [50-na]";
    const resp = await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
      { op: "set-outcome", text: marker, anchor: "#outcome" },
    ]);
    expect(resp.status()).toBe(200);
    await expectClean(resp);

    const prov = await page.request.post(DRAFT_B() + "/api/get_design_provenance", { data: {} });
    const provBody = await prov.json();
    const entry = provBody.entries[provBody.entries.length - 1];
    expect(entry.schema).toBe("verdi.design-provenance/v2");
    expect(entry.policy.state).toBe("not-applicable");
    expect(entry.policy.digest).toBeUndefined();
    expect(entry.attribution.unauthenticated).toBe(true);
  });

  test("direct-Markdown authoring is disclosed as unclassified in the review packet", async ({ page }) => {
    // stale-decline-notices was authored as direct Markdown (provisioning
    // bytes) and nothing ever mutated it through the typed core: AC-4's
    // honest posture is ONE open unclassified edit covering the whole
    // current content — never fabricated attribution.
    const path = boardPath(SHOWCASE.REVIEW_SPEC);
    await page.goto(path);
    const resp = await page.request.post(path + "/api/prepare_design_review", { data: {} });
    expect(resp.status()).toBe(200);
    const body = await resp.json();
    expect(body.schema).toBe("verdi.design-review/v1");
    expect(Array.isArray(body.unclassified_edits)).toBe(true);
    expect(body.unclassified_edits.length).toBeGreaterThan(0);
    const gap = body.unclassified_edits[0];
    expect(gap.to_digest).toMatch(/^sha256:/);
  });
});

// ---------------------------------------------------------------------------
// Conditional refresh: polling, hidden pause, resume, manual, preservation
// ---------------------------------------------------------------------------
test.describe("conditional refresh", () => {
  test("snapshot answers 304 for an unchanged revision token", async ({ page }) => {
    await page.goto(DESIGN());
    const first = await page.request.get(DESIGN() + "/snapshot");
    const etag = first.headers()["etag"];
    expect(etag).toBeTruthy();
    const second = await page.request.get(DESIGN() + "/snapshot", {
      headers: { "If-None-Match": etag },
    });
    expect(second.status()).toBe(304);
  });

  test("hidden tabs pause polling; visibility resumes with one immediate refresh", async ({ page }) => {
    await page.goto(DESIGN());
    // Count snapshot fetches from inside the page.
    await page.evaluate(() => {
      const w = window as unknown as { __snapCount: number; fetch: typeof fetch };
      w.__snapCount = 0;
      const real = w.fetch.bind(window);
      w.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
        if (String(input).includes("/snapshot")) w.__snapCount++;
        return real(input, init);
      }) as typeof fetch;
    });
    // Simulate a hidden tab: the poll loop reads document.hidden live.
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", { configurable: true, get: () => true });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await page.waitForTimeout(4_500);
    const whileHidden = await page.evaluate(
      () => (window as unknown as { __snapCount: number }).__snapCount,
    );
    expect(whileHidden).toBe(0);
    // Visible again: exactly the immediate conditional refresh fires.
    await page.evaluate(() => {
      Object.defineProperty(document, "hidden", { configurable: true, get: () => false });
      document.dispatchEvent(new Event("visibilitychange"));
    });
    await expect
      .poll(
        () => page.evaluate(() => (window as unknown as { __snapCount: number }).__snapCount),
        { timeout: 3_000 },
      )
      .toBeGreaterThan(0);
  });

  test("the manual Refresh control is keyboard-reachable and refreshes", async ({ page }) => {
    await page.goto(DESIGN());
    const marker = "manual refresh landed [50-manual]";
    // Change the projection from outside the page.
    const resp = await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
      {
        op: "edit-ac",
        id: SHOWCASE.AC_IDS[2],
        text: marker,
        evidence: ["attestation"],
        anchor: "#" + SHOWCASE.AC_IDS[2],
      },
    ]);
    await expectClean(resp);
    // Keyboard: focus the control and press Enter.
    await page.getByTestId("asd-refresh").focus();
    await expect(page.getByTestId("asd-refresh")).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[2])).toContainText("[50-manual]", {
      timeout: 5_000,
    });
  });

  test("background refresh preserves unsaved edits, expansion, and the last result", async ({ page }) => {
    await page.goto(DESIGN());
    // Expand a shell disclosure and record the last action result.
    await page.locator(".asd-posture-tech > summary").click();
    await expect(page.locator(".asd-posture-tech")).toHaveAttribute("open", "");
    const targetID = await addFreshQuestion(page, "preservation case seed [50-pres-1]");
    // Open the inline editor and type WITHOUT saving.
    await page.getByTestId("card-" + SHOWCASE.AC_IDS[0]).dblclick();
    const editor = page.getByRole("textbox", { name: "Card text" });
    await expect(editor).toBeVisible();
    await editor.fill("unsaved human bytes that must survive [50-unsaved]");
    // An external mutation lands while the editor is open...
    const external = await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
      {
        op: "edit-question",
        id: targetID,
        text: "external change under an open editor [50-pres-2]",
        anchor: "#" + targetID,
      },
    ]);
    await expectClean(external);
    // ...two poll ticks pass; the swap is HELD: the editor and its bytes
    // survive untouched (AC-2's protected unsaved edit).
    await page.waitForTimeout(4_500);
    await expect(editor).toBeVisible();
    await expect(editor).toHaveValue("unsaved human bytes that must survive [50-unsaved]");
    // The expanded disclosure is still expanded.
    await expect(page.locator(".asd-posture-tech")).toHaveAttribute("open", "");
    // Saving now is a STALE write: the kernel refuses it, the conflict is
    // visible, and the user's bytes are preserved in the disclosure.
    await editor.blur();
    await expect(page.getByTestId("asd-last-result")).toHaveAttribute("data-result-kind", "stale", {
      timeout: 5_000,
    });
    await expect(page.getByTestId("asd-last-result")).toContainText("[50-unsaved]");
    // The refreshed board carries the external edit — nothing was merged.
    await expect(page.getByTestId("card-" + targetID)).toContainText("[50-pres-2]", {
      timeout: 8_000,
    });
  });

  test("an external typed mutation appears on an open board via visible polling", async ({ page, context }) => {
    await page.goto(DESIGN());
    // A second client (CO-9: "an external mutation appearing on an open
    // board") — the same typed transaction surface.
    const other = await context.newPage();
    const marker = "interleaved agent-and-human edit [50-interleave]";
    const resp = await postMutate(other, DESIGN(), SHOWCASE.DESIGN_SPEC, [
      {
        op: "edit-ac",
        id: SHOWCASE.AC_IDS[1],
        text: marker,
        evidence: ["behavioral", "attestation"],
        anchor: "#" + SHOWCASE.AC_IDS[1],
      },
    ]);
    await expectClean(resp);
    await other.close();
    await expect(page.getByTestId("card-" + SHOWCASE.AC_IDS[1])).toContainText("[50-interleave]", {
      timeout: 8_000,
    });
  });
});

// ---------------------------------------------------------------------------
// Typed forms and in-place correction
// ---------------------------------------------------------------------------
test.describe("typed forms", () => {
  test("the graduation preview validates the slug grammar before authoring (F-06)", async ({ page }) => {
    await page.goto(DESIGN());
    // A story proto-sticky whose title cannot typeset to a kebab slug.
    const sticky = await addSticky(page, "Bad Slug! With? Punct.", "story");
    await sticky.getByRole("button", { name: "Graduate" }).click();
    // The refusal comes BEFORE any durable mutation, naming the rejected
    // bytes and the grammar (F-06's exact gap).
    const dialog = page.locator("#edge-confirm");
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("not kebab-case");
    await expect(dialog).toContainText("kebab-case slug");
    await page.getByRole("button", { name: "Cancel" }).click();
    // Clean up the sticky (scratch dies without ceremony).
    await sticky.getByRole("button", { name: "Delete sticky" }).click();
  });

  test("in-place stub correction rides the same typed transaction (F-06)", async ({ page }) => {
    // Review fix I-4: the fixture GUARANTEES a stub (seeded through the
    // typed API), the correction is APPLIED (edit-stub) and asserted in
    // both the projection and the persisted spec bytes, and the slug
    // rename exercises the atomic [remove-stub, add-stub] batch. No
    // skip, no Cancel-only walkthrough.
    const seedSlug = "stub-correct-seed";
    const renamedSlug = "stub-correct-renamed";
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "add-stub", slug: seedSlug, acceptance_criteria: ["ac-1"] },
      ]),
    );
    await page.goto(DRAFT_B());
    const seeded = page.locator(`.stubcard[data-stub="${seedSlug}"]`);
    await expect(seeded).toBeVisible();

    await seeded.locator("[data-asd-correct-stub]").click();
    const dialog = page.locator("#asd-stub-dialog");
    await expect(dialog).toBeVisible();

    // Grammar validation at the field, live (F-06's field-level refusal).
    const slugField = page.getByTestId("asd-stub-slug");
    await slugField.fill("Not Kebab");
    await expect(page.getByTestId("asd-stub-slug-error")).toBeVisible();
    await expect(page.getByTestId("asd-stub-slug-error")).toContainText("kebab-case");
    await slugField.fill(seedSlug);
    await expect(page.getByTestId("asd-stub-slug-error")).toBeHidden();

    // APPLY a binding correction: the stub claims ac-2 as well — ONE
    // edit-stub through the shared core.
    await dialog.locator('[data-asd-stub-ac="ac-2"]').check();
    const editPosted = page.waitForResponse(
      (r) => r.url().includes("/api/mutate_draft") && r.ok(),
      { timeout: 20_000 },
    );
    await page.getByTestId("asd-stub-ok").click();
    await editPosted;
    // Projection: the corrected binding is on the wall (data attributes,
    // lane rule).
    await expect(page.locator(`.stubcard[data-stub="${seedSlug}"]`)).toHaveAttribute(
      "data-acs",
      "ac-1,ac-2",
      { timeout: 10_000 },
    );
    // Persisted spec: the snapshot's exact base bytes carry the
    // corrected stub declaration.
    const afterEdit = await snapshotOf(page, DRAFT_B());
    const specAfterEdit = Buffer.from(afterEdit.base_spec_b64, "base64").toString("utf8");
    const editedLine = specAfterEdit
      .split("\n")
      .find((l: string) => l.includes(seedSlug));
    expect(editedLine, specAfterEdit).toBeTruthy();
    expect(editedLine!).toContain("ac-2");

    // RENAME: a slug change is ONE atomic [remove-stub, add-stub] batch.
    await page
      .locator(`.stubcard[data-stub="${seedSlug}"]`)
      .locator("[data-asd-correct-stub]")
      .click();
    await expect(dialog).toBeVisible();
    await slugField.fill(renamedSlug);
    const renamePosted = page.waitForResponse(
      (r) => r.url().includes("/api/mutate_draft") && r.ok(),
      { timeout: 20_000 },
    );
    await page.getByTestId("asd-stub-ok").click();
    await renamePosted;
    await expect(page.locator(`.stubcard[data-stub="${renamedSlug}"]`)).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.locator(`.stubcard[data-stub="${seedSlug}"]`)).toHaveCount(0);
    // The rename carried the bindings and replaced the slug atomically in
    // the persisted spec.
    const afterRename = await snapshotOf(page, DRAFT_B());
    const specAfterRename = Buffer.from(afterRename.base_spec_b64, "base64").toString("utf8");
    expect(specAfterRename).toContain(renamedSlug);
    expect(specAfterRename).not.toContain(seedSlug);
    const renamedLine = specAfterRename
      .split("\n")
      .find((l: string) => l.includes(renamedSlug));
    expect(renamedLine, specAfterRename).toBeTruthy();
    expect(renamedLine!).toContain("ac-1");
    expect(renamedLine!).toContain("ac-2");

    // Cleanup: the probe stub leaves through the same typed surface, so
    // the shared wall returns to its provisioned shape.
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-stub", slug: renamedSlug },
      ]),
    );
  });

  test("the stub-correction dialog offers the wall's current objects, not the page-load inventory", async ({ page }) => {
    // Codex correction round 1, finding 3: the dialog's AC/question
    // choices must track the fresh projection — a criterion added on this
    // very page is offered and bindable, and an object removed since page
    // load is no longer offered.
    const slug = "stub-choices-probe";
    // A probe question that EXISTS AT PAGE LOAD, so the frozen-inventory
    // defect is observable on the removal half.
    const preSnap = await snapshotOf(page, DRAFT_B());
    const oqMatch = (preSnap.html as string).match(/data-next-id-oq="([^"]+)"/);
    expect(oqMatch, "snapshot html carries data-next-id-oq").toBeTruthy();
    const probeOQ = oqMatch![1];
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "add-question", id: probeOQ, text: "stub-choice probe question [f3]", anchor: "#" + probeOQ },
      ]),
    );
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "add-stub", slug, acceptance_criteria: ["ac-1"] },
      ]),
    );
    await page.goto(DRAFT_B());
    const dialog = page.locator("#asd-stub-dialog");

    // (a) An AC added on the SAME page via the typed form is offered and
    // bindable.
    await page.locator("#asd-add-object").click();
    await page.locator("#asd-op-kind").selectOption("add-ac");
    const preview = await page.getByTestId("asd-op-id-preview").textContent();
    const newAC = preview!.replace("will be declared as ", "").trim();
    await page.getByTestId("asd-op-text").fill("a criterion added after page load [f3]");
    const posted = page.waitForResponse(
      (r) => r.url().includes("/api/mutate_draft") && r.ok(),
      { timeout: 20_000 },
    );
    await page.getByTestId("asd-op-ok").click();
    await posted;
    await expect(page.getByTestId("card-" + newAC)).toBeVisible({ timeout: 10_000 });

    await page.locator(`.stubcard[data-stub="${slug}"]`).locator("[data-asd-correct-stub]").click();
    await expect(dialog).toBeVisible();
    const newChoice = dialog.locator(`[data-asd-stub-ac="${newAC}"]`);
    await expect(newChoice, "a newly added AC must be offered").toHaveCount(1);
    await newChoice.check();
    const bindPosted = page.waitForResponse(
      (r) => r.url().includes("/api/mutate_draft") && r.ok(),
      { timeout: 20_000 },
    );
    await page.getByTestId("asd-stub-ok").click();
    await bindPosted;
    await expect(page.locator(`.stubcard[data-stub="${slug}"]`)).toHaveAttribute(
      "data-acs",
      "ac-1," + newAC,
      { timeout: 10_000 },
    );

    // (b) An object removed since page load is no longer offered.
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-question", id: probeOQ },
      ]),
    );
    await expect(page.getByTestId("card-" + probeOQ)).toHaveCount(0, { timeout: 10_000 });
    await page.locator(`.stubcard[data-stub="${slug}"]`).locator("[data-asd-correct-stub]").click();
    await expect(dialog).toBeVisible();
    await expect(
      dialog.locator(`[data-asd-stub-oq="${probeOQ}"]`),
      "a removed question must not be offered",
    ).toHaveCount(0);
    await page.locator("#asd-stub-cancel").click();

    // Cleanup through the same typed surface.
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-stub", slug },
      ]),
    );
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-ac", id: newAC },
      ]),
    );
  });

  test("an applying snapshot never clobbers the open stub-correction dialog", async ({ page }) => {
    // Codex correction round 1, finding 3 (the guard half), tightened by
    // the closure round: an OPEN dialog is an active interaction — the
    // projection swap AND the mutation base are HELD while it is open
    // (never adopted under the author's draft), the in-flight input
    // survives untouched, and the dialog's end is the resume point where
    // the held projection applies.
    const slug = "stub-hold-probe";
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "add-stub", slug, acceptance_criteria: ["ac-1"] },
      ]),
    );
    await page.goto(DRAFT_B());
    await page.locator(`.stubcard[data-stub="${slug}"]`).locator("[data-asd-correct-stub]").click();
    const dialog = page.locator("#asd-stub-dialog");
    await expect(dialog).toBeVisible();
    // In-flight input: a half-typed slug and a changed binding choice.
    await page.getByTestId("asd-stub-slug").fill("half-typed-rename");
    await dialog.locator('[data-asd-stub-ac="ac-2"]').check();
    // A server-state change lands while the dialog is open…
    const resp = await page.request.post(DRAFT_B() + "/api/sticky", {
      data: { text: "poll probe [f3-hold]", type: "comment" },
    });
    expect(resp.ok()).toBeTruthy();
    // …polls FETCH it (two 200s prove the change was seen), but the swap
    // is HELD: the wall does not change under the open dialog.
    await page.waitForResponse((r) => r.url().includes("/snapshot") && r.status() === 200, {
      timeout: 15_000,
    });
    await page.waitForResponse((r) => r.url().includes("/snapshot"), { timeout: 15_000 });
    const sticky = page.locator('[data-testid^="sticky-"]').filter({ hasText: "[f3-hold]" });
    await expect(sticky).toHaveCount(0);
    // The open dialog is untouched: typed slug, changed choice, existing
    // choice.
    await expect(page.getByTestId("asd-stub-slug")).toHaveValue("half-typed-rename");
    await expect(dialog.locator('[data-asd-stub-ac="ac-2"]')).toBeChecked();
    await expect(dialog.locator('[data-asd-stub-ac="ac-1"]')).toBeChecked();
    // The dialog's end is the resume point: the held projection applies.
    await page.locator("#asd-stub-cancel").click();
    await expect(sticky).toHaveCount(1, { timeout: 10_000 });
    // Cleanup: the sticky dies by its own affordance; the stub through
    // the typed surface.
    await sticky.first().getByRole("button", { name: "Delete sticky" }).click();
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-stub", slug },
      ]),
    );
  });

  test("applying a dialog drafted against a superseded base is refused stale, never a silent overwrite", async ({ page }) => {
    // Codex closure round 1 (the reviewer's own probe shape): while a
    // typed-operation dialog is open, the mutation base must be HELD —
    // pre-fix, polling adopted the fresh base under the open dialog and
    // the preserved input then applied CLEANLY over the concurrent
    // change. The honest outcome is the kernel's typed stale refusal
    // against the base the dialog was drafted on, with the external
    // value surviving.
    await page.goto(DESIGN());
    const placard = page.locator('[data-testid="placard-problem"] .placard-text');
    const original = (await placard.textContent())!.trim();
    const anchorBtn = page.locator("#asd-set-problem");
    const anchor = (await anchorBtn.getAttribute("data-anchor")) || "#problem";
    await anchorBtn.click();
    const dialog = page.locator("#asd-op-dialog");
    await expect(dialog).toBeVisible();
    await page.getByTestId("asd-op-text").fill("dialog-stale problem text [c3-op]");
    // An external typed mutation lands while the dialog is open…
    await expectClean(
      await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
        { op: "set-problem", text: "external problem update [c3-external]", anchor },
      ]),
    );
    // …and a poll FETCHES it (its application is held under the dialog).
    await page.waitForResponse((r) => r.url().includes("/snapshot") && r.status() === 200, {
      timeout: 15_000,
    });
    await page.getByTestId("asd-op-ok").click();
    // The typed STALE refusal, never a clean overwrite.
    await expect(page.getByTestId("asd-last-result")).toHaveAttribute("data-result-kind", "stale", {
      timeout: 10_000,
    });
    // The external value survives on the reconciled wall.
    await expect(placard).toContainText("[c3-external]", { timeout: 10_000 });
    // Cleanup: restore the provisioned problem statement.
    await expectClean(
      await postMutate(page, DESIGN(), SHOWCASE.DESIGN_SPEC, [
        { op: "set-problem", text: original, anchor },
      ]),
    );
  });

  test("applying a stub correction drafted against a superseded base is refused stale", async ({ page }) => {
    // Same closure finding, second dialog type: the stub-correction
    // dialog's preserved binding state must refuse stale against a
    // concurrent external binding change, never overwrite it.
    const slug = "stub-stale-probe";
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "add-stub", slug, acceptance_criteria: ["ac-1"] },
      ]),
    );
    await page.goto(DRAFT_B());
    await page.locator(`.stubcard[data-stub="${slug}"]`).locator("[data-asd-correct-stub]").click();
    const dialog = page.locator("#asd-stub-dialog");
    await expect(dialog).toBeVisible();
    // An external binding change lands while the dialog is open…
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "edit-stub", slug, acceptance_criteria: ["ac-2"] },
      ]),
    );
    await page.waitForResponse((r) => r.url().includes("/snapshot") && r.status() === 200, {
      timeout: 15_000,
    });
    // …then the preserved dialog state (still claiming ac-1) is applied.
    await page.getByTestId("asd-stub-ok").click();
    await expect(page.getByTestId("asd-last-result")).toHaveAttribute("data-result-kind", "stale", {
      timeout: 10_000,
    });
    // The external binding survives on the reconciled wall.
    await expect(page.locator(`.stubcard[data-stub="${slug}"]`)).toHaveAttribute("data-acs", "ac-2", {
      timeout: 10_000,
    });
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-stub", slug },
      ]),
    );
  });

  test("the trash confirmation names a scoping-layer stub claim before anything is removed (F-08)", async ({ page }) => {
    // Review fix I-3(b): the stub's claim on an AC is a SCOPING-layer
    // chip; the trash impact preview must name it — the spec-layer chip
    // filter alone never saw it, so the confirm was silent about the
    // dangling claim the removal would create.
    const slug = "trash-claim-probe";
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "add-stub", slug, acceptance_criteria: ["ac-1"] },
      ]),
    );
    await page.goto(DRAFT_B());
    // The claim hangs on the wall as a scoping chip (chip classes, lane
    // rule).
    await expect(
      page.locator(`.yarn-chip--scoping[data-from="stub:${slug}"][data-to="ac-1"]`),
    ).toHaveCount(1);

    await dragToTrash(page, page.getByTestId("card-ac-1"));
    const confirm = page.locator("#edge-confirm");
    await expect(confirm).toBeVisible();
    await expect(confirm).toContainText(slug);
    await expect(confirm).toContainText("dangle");

    // Cancel leaves everything standing.
    await page.getByRole("button", { name: "Cancel" }).click();
    await expect(page.getByTestId("card-ac-1")).toBeVisible();
    await expect(
      page.locator(`.yarn-chip--scoping[data-from="stub:${slug}"][data-to="ac-1"]`),
    ).toHaveCount(1);

    // Cleanup the probe stub through the same typed surface.
    await expectClean(
      await postMutate(page, DRAFT_B(), SHOWCASE.SHOWCASE_DRAFT_SPEC, [
        { op: "remove-stub", slug },
      ]),
    );
  });

  test("the add-object form declares through one typed operation", async ({ page }) => {
    await page.goto(DESIGN());
    await page.locator("#asd-add-object").click();
    const dialog = page.locator("#asd-op-dialog");
    await expect(dialog).toBeVisible();
    await expect(page.getByTestId("asd-op-id-preview")).toContainText("will be declared as");
    await page.locator("#asd-op-kind").selectOption("add-decision");
    const preview = await page.getByTestId("asd-op-id-preview").textContent();
    const newID = preview!.replace("will be declared as ", "").trim();
    await page.getByTestId("asd-op-text").fill("a decision declared through the typed form [50-form]");
    const posted = page.waitForResponse(
      (r) => r.url().includes("/api/mutate_draft") && r.ok(),
      { timeout: 20_000 },
    );
    await page.getByTestId("asd-op-ok").click();
    await posted;
    await expect(page.getByTestId("card-" + newID)).toContainText("[50-form]", { timeout: 10_000 });
    await expect(uncommittedIndicator(page)).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Keyboard, accessibility, responsiveness, reduced motion
// ---------------------------------------------------------------------------
test.describe("accessibility and responsiveness", () => {
  test("a keyboard-only journey reaches the shell, refresh, and card editing", async ({ page }) => {
    await page.goto(DESIGN());
    // Skip link first.
    await page.keyboard.press("Tab");
    const skip = page.locator(".skip-link");
    await expect(skip).toBeFocused();
    await page.keyboard.press("Enter");
    // Shell disclosures toggle with the keyboard.
    const summary = page.locator(".asd-posture-tech > summary");
    await summary.focus();
    await page.keyboard.press("Enter");
    await expect(page.locator(".asd-posture-tech")).toHaveAttribute("open", "");
    // The manual refresh is reachable and operable.
    await page.getByTestId("asd-refresh").focus();
    await expect(page.getByTestId("asd-refresh")).toBeFocused();
    await page.keyboard.press("Enter");
    // A card is focusable; Enter opens the inline editor; Escape-free
    // blur exit leaves no trap.
    const card = page.getByTestId("card-" + SHOWCASE.AC_IDS[0]);
    await card.focus();
    await expect(card).toBeFocused();
    await page.keyboard.press("Enter");
    const editor = page.getByRole("textbox", { name: "Card text" });
    await expect(editor).toBeVisible();
    await expect(editor).toBeFocused();
    // Leave without a change: Tab away (no focus trap), editor commits
    // nothing.
    await page.keyboard.press("Tab");
    await expect(editor).toHaveCount(0);
  });

  test("the automated accessibility scan reports no violations", async ({ page }) => {
    await page.goto(DESIGN());
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa"])
      .analyze();
    const violations = results.violations.map((v) => ({
      id: v.id,
      impact: v.impact,
      nodes: v.nodes.length,
      targets: v.nodes.slice(0, 3).map((n) => n.target.join(" ")),
    }));
    expect(violations, JSON.stringify(violations, null, 2)).toEqual([]);
  });

  test("320px layout stays usable with horizontal scroll contained to the canvas", async ({ page }) => {
    await page.setViewportSize({ width: 320, height: 800 });
    await page.goto(DESIGN());
    await expect(page.getByTestId("asd-posture")).toBeVisible();
    await expect(page.getByTestId("asd-shell")).toBeVisible();
    // The page body never scrolls horizontally; the intrinsically wide
    // canvas scrolls inside its own container.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement!;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);
    const canvasScrolls = await page.evaluate(() => {
      const c = document.getElementById("board-canvas")!;
      return c.scrollWidth >= c.clientWidth;
    });
    expect(canvasScrolls).toBe(true);
  });

  test("200% zoom keeps the shell and posture usable", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto(DESIGN());
    await page.evaluate(() => {
      (document.body.style as unknown as { zoom: string }).zoom = "200%";
    });
    await expect(page.getByTestId("asd-posture")).toBeVisible();
    await expect(page.getByTestId("asd-shell")).toBeVisible();
    await expect(page.getByTestId("asd-refresh")).toBeVisible();
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement!;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);
  });

  test("reduced motion leaves the live projection fully functional", async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(DESIGN());
    await expect(page.getByTestId("asd-shell")).toBeVisible();
    const motionID = await addFreshQuestion(page, "reduced motion refresh [50-motion]");
    await expect(page.getByTestId("card-" + motionID)).toContainText("[50-motion]", {
      timeout: 8_000,
    });
  });

  test("the initial HTML is complete before JavaScript and inside the size budget", async ({ page }) => {
    // SI-168: the server response alone carries every fact (shell,
    // posture, canvas) and stays under the 512 KiB structural ceiling.
    const resp = await page.request.get(DESIGN());
    const body = await resp.text();
    expect(body.length).toBeLessThanOrEqual(512 * 1024);
    for (const marker of [
      'data-testid="asd-posture"',
      'data-testid="asd-shell"',
      'data-testid="board"',
      'data-testid="asd-provenance"',
    ]) {
      expect(body).toContain(marker);
    }
    // And the ASD asset itself is under its 64 KiB ceiling.
    const asset = await page.request.get("/assets/boardspecasd.js");
    expect((await asset.text()).length).toBeLessThanOrEqual(64 * 1024);
  });
});
