import { test, expect, type Page, type Locator } from "@playwright/test";
import { SHOWCASE, boardPath } from "./fixtures";

// spec/document-page-v2 (jira:VERDI-WR-6), ac-1 and ac-4: the Document
// page opens with a temporal stamp (proposed or accepted, the commit
// shown, and a refreshed time computed in the browser), an identity card
// (the ref, class, branch, owners, and the files behind the spec), and a
// contents rail listing the body's sections with their counts, around
// the shared body it never writes into (ac-3); the manual Refresh control
// and the body's own not-authority footer stay. The page works at 320 px
// and at 200 % zoom with no horizontal scroll, and before JavaScript runs
// the stamp, the rail, and the body read while no id chip is drawn.
//
// The three tests are the producers their obligations name
// (.verdi/obligations/document-page-v2/ac-1--behavioral.md,
// ac-2--behavioral.md, and ac-4--behavioral.md), titled exactly as each
// claim spells it; the ac-2 producer (the chips' arrival on the wall)
// landed once the wall's selection lane had (SI-340 (11), SI-350's
// carry-in). The file passes when run alone (BL-98): every count
// and fact it asserts is read from the page's own /snapshot at run time,
// because other suites mutate the shared store, and each is then checked
// against the body the reader sees. State rides test ids, data
// attributes, and text — never a screenshot (recording stays off).
//
// DESIGN_SPEC lives only on its design branch, which the harness serves
// checked out, so its document renders proposed; READONLY_SPEC's
// working-tree bytes are the accepted bytes on main, so its document is
// the accepted reading (79-board-document's fixtures, unchanged).

const docPath = (spec: string) => `${boardPath(spec)}/document`;

interface Fact {
  text: string;
  unproven?: string;
}
interface Facts {
  stamp: { state: string; words: string; commit: string };
  identity: {
    ref: string;
    class?: string;
    classLabel?: string;
    branch: Fact;
    detached?: boolean;
    owners: string[];
    files: string[];
  };
  rail: { id: string; text: string; count?: number }[];
  chips: { id: string; kind: string }[];
}
interface Snapshot {
  revision: string;
  html: string;
  markdown: string;
  kind: string;
  ref: string;
  proposed: boolean;
  disclosures: string[];
  facts: Facts;
}

async function snapshotOf(page: Page, path: string): Promise<Snapshot> {
  const res = await page.request.get(`${path}/snapshot`);
  expect(res.status(), `${path}/snapshot`).toBe(200);
  return (await res.json()) as Snapshot;
}

// pageOverflow is the page's horizontal overflow in CSS px
// (87-workbench-topbar's own measure).
async function pageOverflow(page: Page): Promise<number> {
  return page.evaluate(() => {
    const el = document.scrollingElement!;
    return el.scrollWidth - el.clientWidth;
  });
}

// activeKey names the focused element by test id, id, or tag; "BODY"
// when focus is nowhere.
async function activeKey(page: Page): Promise<string> {
  return page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    return el && el !== document.body ? el.getAttribute("data-testid") || el.id || el.tagName : "BODY";
  });
}

// The object sections the design spec declares, each rail id with the
// body's own anchor prefix for its items (02 §Object model id
// conventions), so a count is checked against the body, not only
// against the snapshot that produced it.
const OBJECT_SECTIONS: { id: string; prefix: string }[] = [
  { id: "decisions", prefix: "dc-" },
  { id: "constraints", prefix: "co-" },
  { id: "acceptance-criteria", prefix: "ac-" },
  { id: "open-questions", prefix: "oq-" },
];

// expectChrome asserts the stamp, the identity card, and the rail of the
// page at `path` against the facts its own snapshot states, and the
// rail's entries and counts against the body: every entry links a
// heading the body carries, in the body's order, and an object section's
// count is the number of object anchors the body renders for it.
async function expectChrome(page: Page, path: string, facts: Facts, state: "proposed" | "accepted"): Promise<void> {
  const what = `${path} (${state})`;
  const stamp = page.getByTestId("document-stamp");
  await expect(stamp, `${what}: stamp`).toBeVisible();
  await expect(stamp, `${what}: stamp state`).toHaveAttribute("data-state", state);
  await expect(stamp, `${what}: stamp token`).toHaveClass(new RegExp(`\\bdocument-stamp--${state}\\b`));
  expect(facts.stamp.state, `${what}: the snapshot's state`).toBe(state);
  expect(facts.stamp.commit, `${what}: a full commit`).toMatch(/^[0-9a-f]{40}$/);
  await expect(stamp, `${what}: stamp commit`).toHaveAttribute("data-commit", facts.stamp.commit);
  await expect(page.getByTestId("document-stamp-words"), `${what}: stamp words`).toHaveText(
    state === "proposed" ? "proposed, not accepted" : "accepted",
  );
  await expect(page.getByTestId("document-stamp-commit"), `${what}: shown commit`).toHaveText(facts.stamp.commit.slice(0, 8));
  await expect(page.getByTestId("document-stamp-commit"), `${what}: full commit on hover`).toHaveAttribute("title", facts.stamp.commit);

  const id = facts.identity;
  await expect(page.getByTestId("document-identity-ref"), `${what}: ref`).toHaveText(id.ref);
  expect(id.ref, `${what}: the ref is the spec's`).toMatch(/^spec\//);
  await expect(page.getByTestId("document-identity-class"), `${what}: class`).toHaveText(id.classLabel ?? "not declared");
  const branch = page.getByTestId("document-identity-branch");
  expect(id.branch.unproven ?? "", `${what}: the checkout's branch is proven`).toBe("");
  await expect(branch, `${what}: branch`).toHaveAttribute("data-state", "proven");
  await expect(branch, `${what}: branch text`).toHaveText(id.detached ? "detached HEAD" : id.branch.text);
  // One branch on the page: the card states the one the bar states
  // (SI-343 (3)).
  await expect(page.getByTestId("topbar-branch"), `${what}: the bar's branch`).toHaveText(id.detached ? "detached HEAD" : id.branch.text);
  expect(id.owners.length, `${what}: owners declared`).toBeGreaterThan(0);
  await expect(page.getByTestId("document-identity-owners"), `${what}: owners`).toHaveText(id.owners.join(", "));
  expect(id.files, `${what}: the file behind the spec`).toEqual([`.verdi/specs/active/${id.ref.slice("spec/".length)}/spec.md`]);
  const files = page.getByTestId("document-identity-files").locator("code");
  await expect(files, `${what}: files`).toHaveText(id.files);

  // The rail, against the snapshot and against the body.
  const region = page.getByTestId("document-region");
  const entries = page.getByTestId("document-contents").locator("li");
  await expect(entries, `${what}: rail entries`).toHaveCount(facts.rail.length);
  expect(facts.rail.length, `${what}: the rail lists sections`).toBeGreaterThan(0);
  const bodyHeadings = await region.locator("h2[id]").evaluateAll((els) => els.map((el) => el.id));
  expect(
    facts.rail.map((e) => e.id),
    `${what}: the rail is the body's h2 sections in order`,
  ).toEqual(bodyHeadings);
  for (let i = 0; i < facts.rail.length; i++) {
    const e = facts.rail[i];
    const li = entries.nth(i);
    await expect(li, `${what}: rail entry ${i}`).toHaveAttribute("data-testid", `document-contents-${e.id}`);
    await expect(li.locator("a"), `${what}: rail link ${e.id}`).toHaveAttribute("href", `#${e.id}`);
    await expect(li.locator(".document-contents-text"), `${what}: rail text ${e.id}`).toHaveText(e.text);
    await expect(region.locator(`h2[id="${e.id}"]`), `${what}: the body carries #${e.id}`).toHaveText(e.text);
    const count = li.getByTestId(`document-contents-${e.id}-count`);
    if (e.count === undefined) {
      await expect(count, `${what}: ${e.id} shows no count`).toHaveCount(0);
    } else {
      await expect(count, `${what}: ${e.id} count`).toHaveText(String(e.count));
    }
  }
  for (const s of OBJECT_SECTIONS) {
    const entry = facts.rail.find((e) => e.id === s.id);
    if (!entry) continue;
    const anchors = await region.locator(`a[id^="${s.prefix}"]`).count();
    expect(entry.count, `${what}: ${s.id} counts the body's ${s.prefix}* anchors`).toBe(anchors);
    await expect(page.getByTestId(`document-contents-${s.id}-count`), `${what}: ${s.id} count shown`).toHaveText(String(anchors));
  }

  // The Refresh control and the body's own not-authority footer stay.
  await expect(page.getByTestId("document-refresh"), `${what}: Refresh`).toBeVisible();
  await expect(region, `${what}: not-authority footer`).toContainText("not authority");
}

// expectChips asserts the id chips the script drew: exactly the facts'
// anchors, each a link to the wall's card in the kind's colour word,
// drawn right at the body's own anchor.
async function expectChips(page: Page, boardHref: string, facts: Facts, what: string): Promise<Locator> {
  const region = page.getByTestId("document-region");
  const chips = region.locator(".document-chip");
  await expect(chips, `${what}: one chip per listed anchor`).toHaveCount(facts.chips.length);
  for (const c of facts.chips) {
    const chip = page.getByTestId(`document-chip-${c.id}`);
    await expect(chip, `${what}: chip ${c.id}`).toHaveText(c.id);
    await expect(chip, `${what}: chip ${c.id} href`).toHaveAttribute("href", `${boardHref}#obj-${c.id}`);
    await expect(chip, `${what}: chip ${c.id} kind`).toHaveAttribute("data-object-kind", c.kind);
    await expect(chip, `${what}: chip ${c.id} colour class`).toHaveClass(new RegExp(`\\bdocument-chip--${c.kind}\\b`));
    expect(await chip.evaluate((el) => el.nextElementSibling?.id ?? ""), `${what}: chip ${c.id} sits at its anchor`).toBe(c.id);
  }
  return chips;
}

// The four object kinds ac-2's producer clicks a chip for, in the kind
// words the chips and the wall's cards share (boardlayout.ZoneKind): the
// test checks the chip it clicked against the card the wall selected.
const ARRIVAL_KINDS = ["acceptance-criterion", "constraint", "decision", "open-question"] as const;

// A viewport the wall outgrows on both axes (90-wall-keyboard's SMALL):
// the layout stacks, so the wall frame starts far below the fold, and the
// canvas is bounded to the viewport and scrolls inside itself, so a
// column's lowest card starts outside the canvas's visible box as well.
// Arrival's reveal is then a real move of the page and of the canvas,
// never a card that was in view before the chip was clicked.
const SMALL = { width: 1000, height: 640 };

// cardView reports "in view" when the wall's card for `id` lies whole
// inside the canvas's visible box (its scrollport, not its content) and
// inside the viewport — 90-wall-keyboard's expectRevealed, read in one
// evaluate so it can be polled — or says where the card is instead.
async function cardView(page: Page, id: string): Promise<string> {
  return page.evaluate((cardID) => {
    const c = document.getElementById("board-canvas");
    const el = c && c.querySelector(`.objcard[data-id="${CSS.escape(cardID)}"]`);
    if (!c || !el) return "no card";
    const r = c.getBoundingClientRect();
    const port = {
      left: r.left + c.clientLeft,
      top: r.top + c.clientTop,
      right: r.left + c.clientLeft + c.clientWidth,
      bottom: r.top + c.clientTop + c.clientHeight,
    };
    const vp = { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight };
    const b = el.getBoundingClientRect();
    const inside = (box: { left: number; top: number; right: number; bottom: number }) =>
      b.left >= box.left - 0.5 && b.right <= box.right + 0.5 && b.top >= box.top - 0.5 && b.bottom <= box.bottom + 0.5;
    if (inside(port) && inside(vp)) return "in view";
    const show = (box: { left: number; top: number; right: number; bottom: number }) =>
      `${Math.round(box.left)},${Math.round(box.top)}–${Math.round(box.right)},${Math.round(box.bottom)}`;
    return `card at ${show(b)}; canvas box ${show(port)}; viewport ${show(vp)}`;
  }, id);
}

// postTypedEdit posts one typed edit-ac on the design wall from outside
// the page, against the wall's current base (87-workbench-topbar's
// idiom): a real change to the working tree, so the next poll answers
// 200 with a new revision.
async function postTypedEdit(page: Page, acID: string, text: string): Promise<void> {
  const design = boardPath(SHOWCASE.DESIGN_SPEC);
  const snap = await (await page.request.get(design + "/snapshot")).json();
  const resp = await page.request.post(design + "/api/mutate_draft", {
    data: {
      request: {
        schema: "verdi.draftmutation/v1",
        spec: "spec/" + SHOWCASE.DESIGN_SPEC,
        base_digest: snap.base_digest,
        base_spec_b64: snap.base_spec_b64,
        expected: snap.expected,
        operations: [{ op: "edit-ac", id: acID, text, evidence: ["attestation"], anchor: "#" + acID }],
      },
    },
  });
  expect(resp.status(), await resp.text()).toBe(200);
  expect((await resp.json()).result, "the typed mutation landed").toBeTruthy();
}

test.describe("document-page", () => {
  test("The temporal stamp, identity card, and contents rail", async ({ page }) => {
    test.setTimeout(150_000);
    const design = boardPath(SHOWCASE.DESIGN_SPEC);
    const proposed = docPath(SHOWCASE.DESIGN_SPEC);
    await page.goto(proposed);
    let facts = (await snapshotOf(page, proposed)).facts;
    await expectChrome(page, proposed, facts, "proposed");
    // The body's own proposal notice stays, unhidden, beside the stamp
    // (SI-340 (2); ac-3).
    await expect(page.getByTestId("document-region")).toContainText("Proposed, not accepted");
    await expectChips(page, design, facts, "proposed");

    // The refreshed time (dc-2; SI-340 (5)): computed in the browser from
    // the page's load, counted in seconds, and moved by the manual
    // Refresh — a completed check that answers 304 — which the document
    // script reports through its one event, with the status.
    const refreshed = page.getByTestId("document-refreshed");
    await expect(refreshed).toHaveText(/^refreshed \d+ s ago$/);
    const loaded = await refreshed.getAttribute("data-refreshed-at");
    expect(loaded).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    // Outside the live region: the status line, not the stamp, is the
    // page's one role="status".
    expect(await refreshed.evaluate((el) => !!el.closest("[aria-live],[role=status]"))).toBe(false);
    await page.evaluate(() => {
      const w = window as unknown as { __checks: number[] };
      w.__checks = [];
      document.getElementById("document-region")!.addEventListener("verdi:document-refresh", (e) => {
        w.__checks.push((e as CustomEvent<{ status: number }>).detail.status);
      });
    });
    await page.waitForTimeout(1_100);
    await page.getByTestId("document-refresh").focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("status")).toHaveText("Up to date");
    await expect.poll(() => refreshed.getAttribute("data-refreshed-at")).not.toBe(loaded);
    const afterRefresh = (await refreshed.getAttribute("data-refreshed-at")) as string;
    expect(Date.parse(afterRefresh)).toBeGreaterThan(Date.parse(loaded as string));
    await expect(refreshed).toHaveText(/^refreshed \d+ s ago$/);
    expect(await page.evaluate(() => (window as unknown as { __checks: number[] }).__checks)).toContain(304);

    // A poll that brings a new revision (a real edit to the working tree)
    // answers 200: the event carries it, the body swaps, the chips are
    // drawn again at the new body's anchors, and the chip the reader had
    // focused is focused again (SI-340 (8), (12)).
    const region = page.getByTestId("document-region");
    const before = await region.getAttribute("data-revision");
    const focusedChip = `document-chip-${SHOWCASE.AC_IDS[1]}`;
    await page.getByTestId(focusedChip).focus();
    expect(await activeKey(page)).toBe(focusedChip);
    await postTypedEdit(page, SHOWCASE.AC_IDS[0], `edited by 94-document-page at ${Date.now()}`);
    await expect.poll(() => region.getAttribute("data-revision"), { timeout: 10_000 }).not.toBe(before);
    expect(await page.evaluate(() => (window as unknown as { __checks: number[] }).__checks)).toContain(200);
    facts = (await snapshotOf(page, proposed)).facts;
    await expectChips(page, design, facts, "after a 200");
    await expect.poll(() => activeKey(page), { timeout: 3_000 }).toBe(focusedChip);
    await expectChrome(page, proposed, facts, "proposed");

    // The chrome is redrawn from the snapshot a 200 carries, never left
    // as loaded (SI-340 (8)): a snapshot whose facts differ — another
    // commit, another owner, one more criterion counted, one chip fewer —
    // is what the stamp, the card, the rail, and the chips then show, and
    // the real snapshot takes them back on the next poll.
    const real = await snapshotOf(page, proposed);
    const probe: Snapshot = JSON.parse(JSON.stringify(real));
    probe.revision = `probe-${Date.now()}`;
    probe.facts.stamp.commit = "f".repeat(40);
    probe.facts.identity.owners = [...probe.facts.identity.owners, "probe-team"];
    const criteria = probe.facts.rail.find((e) => e.id === "acceptance-criteria")!;
    expect(criteria.count).toBeGreaterThan(0);
    criteria.count = (criteria.count as number) + 1;
    const dropped = probe.facts.chips[0].id;
    probe.facts.chips = probe.facts.chips.slice(1);
    let probed = 0;
    // One route for the whole block: swapping routes would leave a gap a
    // poll could slip through to the real server, moving the refreshed
    // time by chance; the non-JSON body below is served by this same
    // handler once the flag is set.
    let badBody = false;
    await page.route(`**${proposed}/snapshot*`, (route) => {
      probed++;
      if (badBody) return route.fulfill({ status: 200, contentType: "application/json", body: "<html>not json</html>" });
      return route.fulfill({ status: 200, contentType: "application/json", headers: { etag: `"${probe.revision}"` }, body: JSON.stringify(probe) });
    });
    try {
      await expect.poll(() => region.getAttribute("data-revision"), { timeout: 10_000 }).toBe(probe.revision);
      await expect(page.getByTestId("document-stamp")).toHaveAttribute("data-commit", "f".repeat(40));
      await expect(page.getByTestId("document-stamp-commit")).toHaveText("ffffffff");
      await expect(page.getByTestId("document-identity-owners")).toHaveText(probe.facts.identity.owners.join(", "));
      await expect(page.getByTestId("document-contents-acceptance-criteria-count")).toHaveText(String(criteria.count));
      await expect(region.locator(".document-chip")).toHaveCount(probe.facts.chips.length);
      await expect(page.getByTestId(`document-chip-${dropped}`)).toHaveCount(0);

      // A 200 is a completed check too (SI-340 (5)): with every poll
      // answered 200 here and no 304 in between, the refreshed time moves
      // from one 200 to the next.
      const atFirst200 = (await refreshed.getAttribute("data-refreshed-at")) as string;
      const seen = probed;
      await expect.poll(() => probed, { timeout: 8_000 }).toBeGreaterThan(seen);
      await expect.poll(() => refreshed.getAttribute("data-refreshed-at"), { timeout: 4_000 }).not.toBe(atFirst200);
      expect(Date.parse((await refreshed.getAttribute("data-refreshed-at")) as string)).toBeGreaterThan(Date.parse(atFirst200));

      // A 200 whose body is not JSON is a failed refresh, not a completed
      // check: the status line says so and the refreshed time stays.
      badBody = true;
      await page.waitForTimeout(300); // a probe 200 answered just before the flag settles first
      const beforeBad = (await refreshed.getAttribute("data-refreshed-at")) as string;
      await page.waitForTimeout(1_100);
      await page.getByTestId("document-refresh").click();
      await expect(page.getByRole("status")).toHaveText(/^Refresh failed/);
      await page.waitForTimeout(300);
      expect(await refreshed.getAttribute("data-refreshed-at"), "a non-JSON 200 is not a completed check").toBe(beforeBad);
      await expect(page.getByTestId("document-stamp")).toHaveAttribute("data-commit", "f".repeat(40));
    } finally {
      await page.unroute(`**${proposed}/snapshot*`);
    }
    await expect.poll(() => region.getAttribute("data-revision"), { timeout: 10_000 }).not.toBe(probe.revision);
    await expect(page.getByTestId("document-stamp")).toHaveAttribute("data-commit", real.facts.stamp.commit);
    await expect(page.getByTestId("document-identity-owners")).toHaveText(real.facts.identity.owners.join(", "));
    await expectChips(page, design, real.facts, "after the real snapshot returns");

    // A stale 200 — a poll delayed past a newer Refresh — is dropped by
    // the document script's sequence guard and never drives the chrome:
    // the facts are applied only for the revision the body carries, so
    // the chrome never shows another revision than the body (SI-340
    // (8); the F5b review's stale-drop probe).
    const stale: Snapshot = JSON.parse(JSON.stringify(real));
    stale.revision = "probe-stale";
    stale.facts.stamp.commit = "1".repeat(40);
    stale.facts.identity.owners = ["stale-team"];
    const fresh: Snapshot = JSON.parse(JSON.stringify(real));
    fresh.revision = "probe-fresh";
    fresh.facts.stamp.commit = "2".repeat(40);
    fresh.facts.identity.owners = ["fresh-team"];
    let raced = 0;
    await page.route(`**${proposed}/snapshot*`, async (route) => {
      raced++;
      const inm = route.request().headers()["if-none-match"];
      if (raced === 1) {
        await new Promise((r) => setTimeout(r, 2_500));
        return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(stale) });
      }
      if (inm === `"${fresh.revision}"`) return route.fulfill({ status: 304 });
      return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(fresh) });
    });
    try {
      await expect.poll(() => raced, { timeout: 10_000 }).toBeGreaterThanOrEqual(1);
      await page.getByTestId("document-refresh").click();
      await expect.poll(() => region.getAttribute("data-revision"), { timeout: 10_000 }).toBe(fresh.revision);
      await page.waitForTimeout(4_000); // the stale response lands and is dropped
      await expect(region).toHaveAttribute("data-revision", fresh.revision);
      await expect(page.getByTestId("document-stamp"), "the chrome shows the body's revision, never the dropped one").toHaveAttribute("data-commit", "2".repeat(40));
      await expect(page.getByTestId("document-identity-owners")).toHaveText("fresh-team");
    } finally {
      await page.unroute(`**${proposed}/snapshot*`);
    }
    await expect.poll(() => region.getAttribute("data-revision"), { timeout: 10_000 }).not.toBe(fresh.revision);
    await expect(page.getByTestId("document-stamp")).toHaveAttribute("data-commit", real.facts.stamp.commit);
    await expect(page.getByTestId("document-identity-owners")).toHaveText(real.facts.identity.owners.join(", "));

    // The accepted reading, in the accepted token.
    const accepted = docPath(SHOWCASE.READONLY_SPEC);
    await page.goto(accepted);
    const acceptedFacts = (await snapshotOf(page, accepted)).facts;
    await expectChrome(page, accepted, acceptedFacts, "accepted");
    await expect(page.getByTestId("document-region")).not.toContainText("Proposed, not accepted");
    await expectChips(page, boardPath(SHOWCASE.READONLY_SPEC), acceptedFacts, "accepted");
    // Its refreshed time moves after Refresh too (ac-1: "for a proposed
    // and an accepted spec").
    const acceptedRefreshed = page.getByTestId("document-refreshed");
    await expect(acceptedRefreshed).toHaveText(/^refreshed \d+ s ago$/);
    const acceptedLoaded = (await acceptedRefreshed.getAttribute("data-refreshed-at")) as string;
    expect(acceptedLoaded).toMatch(/^\d{4}-\d{2}-\d{2}T/);
    await page.waitForTimeout(1_100);
    await page.getByTestId("document-refresh").focus();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("status")).toHaveText("Up to date");
    await expect.poll(() => acceptedRefreshed.getAttribute("data-refreshed-at")).not.toBe(acceptedLoaded);
    expect(Date.parse((await acceptedRefreshed.getAttribute("data-refreshed-at")) as string)).toBeGreaterThan(Date.parse(acceptedLoaded));
    await expect(acceptedRefreshed).toHaveText(/^refreshed \d+ s ago$/);
  });

  test("The Document page at 320 px, 200 % zoom, and without JavaScript", async ({ page, browser }) => {
    test.setTimeout(150_000);
    const proposed = docPath(SHOWCASE.DESIGN_SPEC);
    const facts = (await snapshotOf(page, proposed)).facts;
    const chrome = ["document-stamp", "document-identity", "document-contents", "document-region", "document-refresh", "document-kind-plan"];

    // 320 px: no horizontal scroll, the chrome and the body on screen, the
    // rail folded above the body.
    await page.setViewportSize({ width: 320, height: 800 });
    await page.goto(proposed);
    expect(await pageOverflow(page), "@320: horizontal overflow").toBeLessThanOrEqual(1);
    for (const id of chrome) {
      await expect(page.getByTestId(id), `@320: ${id}`).toBeVisible();
    }
    const railBox320 = (await page.getByTestId("document-contents").boundingBox())!;
    const regionBox320 = (await page.getByTestId("document-region").boundingBox())!;
    expect(railBox320.y + railBox320.height, "@320: the rail folds above the body").toBeLessThanOrEqual(regionBox320.y + 1);
    // The body's wide table scrolls inside its own box (Wave 6 §5.2).
    const table = page.getByTestId("document-region").locator("table").first();
    expect(await table.evaluate((el) => el.scrollWidth <= el.clientWidth + 1 || getComputedStyle(el).overflowX === "auto"), "@320: a wide table is contained").toBe(true);

    // 200 % zoom at a laptop width: the same, folded by the layout's own
    // width rather than the viewport's.
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto(proposed);
    await page.evaluate(() => {
      (document.body.style as unknown as { zoom: string }).zoom = "200%";
    });
    expect(await pageOverflow(page), "@200%: horizontal overflow").toBeLessThanOrEqual(1);
    for (const id of chrome) {
      await expect(page.getByTestId(id), `@200%: ${id}`).toBeVisible();
    }
    expect(
      await page.evaluate(() => {
        const rail = document.querySelector('[data-testid="document-contents"]')!.getBoundingClientRect();
        const body = document.querySelector('[data-testid="document-region"]')!.getBoundingClientRect();
        return rail.bottom <= body.top + 1;
      }),
      "@200%: the rail folds above the body",
    ).toBe(true);

    // At a desktop width the rail sits beside the body.
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(proposed);
    expect(await pageOverflow(page), "@1440: horizontal overflow").toBeLessThanOrEqual(1);
    const railBox = (await page.getByTestId("document-contents").boundingBox())!;
    const regionBox = (await page.getByTestId("document-region").boundingBox())!;
    expect(railBox.x, "@1440: the rail sits beside the body").toBeGreaterThanOrEqual(regionBox.x + regionBox.width);

    // Without JavaScript: the stamp states the proposed or accepted state
    // and the commit, the rail and the body read, no clock is written,
    // and no id chip exists (ac-4; dc-1; dc-2).
    const noJS = await browser.newContext({ javaScriptEnabled: false });
    try {
      const quiet = await noJS.newPage();
      for (const [path, state, f] of [
        [proposed, "proposed", facts],
        [docPath(SHOWCASE.READONLY_SPEC), "accepted", (await snapshotOf(page, docPath(SHOWCASE.READONLY_SPEC))).facts],
      ] as [string, "proposed" | "accepted", Facts][]) {
        await quiet.goto(path);
        const what = `no JS ${path}`;
        const stamp = quiet.getByTestId("document-stamp");
        await expect(stamp, `${what}: stamp`).toBeVisible();
        await expect(stamp, `${what}: stamp state`).toHaveAttribute("data-state", state);
        await expect(stamp, `${what}: stamp commit`).toHaveAttribute("data-commit", f.stamp.commit);
        await expect(quiet.getByTestId("document-stamp-words"), `${what}: stamp words`).toHaveText(
          state === "proposed" ? "proposed, not accepted" : "accepted",
        );
        await expect(quiet.getByTestId("document-stamp-commit"), `${what}: shown commit`).toHaveText(f.stamp.commit.slice(0, 8));
        const refreshed = quiet.getByTestId("document-refreshed");
        await expect(refreshed, `${what}: no refreshed time without a browser clock`).toHaveText("");
        expect(await refreshed.getAttribute("data-refreshed-at"), `${what}: no refreshed stamp`).toBeNull();
        await expect(quiet.getByTestId("document-contents").locator("li"), `${what}: rail`).toHaveCount(f.rail.length);
        for (const e of f.rail) {
          await expect(quiet.getByTestId(`document-contents-${e.id}`).locator("a"), `${what}: rail link ${e.id}`).toHaveAttribute("href", `#${e.id}`);
        }
        const region = quiet.getByTestId("document-region");
        await expect(region, `${what}: body`).toContainText("Identity");
        await expect(region, `${what}: not-authority footer`).toContainText("not authority");
        await expect(quiet.locator(".document-chip"), `${what}: no chip`).toHaveCount(0);
        expect(f.chips.length, `${what}: the facts list chips the script would draw`).toBeGreaterThan(0);
        await expect(quiet.getByTestId("document-refresh"), `${what}: Refresh`).toBeVisible();
      }
      await quiet.setViewportSize({ width: 320, height: 800 });
      await quiet.goto(proposed);
      expect(await pageOverflow(quiet), "no JS @320: horizontal overflow").toBeLessThanOrEqual(1);
    } finally {
      await noJS.close();
    }
  });

  // ac-2: each object's id chip opens the wall with that card selected,
  // and the wall selects and reveals the card on arrival. For an
  // acceptance criterion, a constraint, a decision, and an open question
  // the test clicks the chip the Document page drew — a real click on the
  // link, never a goto — and asserts the wall loads at <wall>#obj-<id>,
  // that this card alone is selected (data-selected="true", the mark
  // wallselect.js sets on .objcard[data-id]; SI-340 (11), SI-350's
  // carry-in), and that the card lies in view. The chips are read from
  // the page's own snapshot (the file's idiom); for each kind the last
  // chip is taken, the lowest card of its column, which at SMALL starts
  // out of view on the bare wall — proven before the click, so the
  // reveal the test then sees is a real move.
  test("Id chips open the wall with the card selected", async ({ page }) => {
    test.setTimeout(150_000);
    const proposed = docPath(SHOWCASE.DESIGN_SPEC);
    const wall = boardPath(SHOWCASE.DESIGN_SPEC);
    const facts = (await snapshotOf(page, proposed)).facts;
    await page.setViewportSize(SMALL);
    const canvas = page.getByTestId("board");
    const selected = page.locator("#board-canvas [data-selected]");
    for (const kind of ARRIVAL_KINDS) {
      const chips = facts.chips.filter((c) => c.kind === kind);
      expect(chips.length, `${kind}: the document lists a chip of this kind`).toBeGreaterThan(0);
      const id = chips[chips.length - 1].id;
      const what = `${kind} ${id}`;

      // The bare wall, page at its top: nothing selected, the card out of
      // view, so what follows is arrival's doing.
      await page.goto(wall);
      await expect(canvas, `${what}: the wall`).toHaveAttribute("data-board-mode", "authoring");
      await expect(page.getByTestId(`card-${id}`), `${what}: the wall's card is of the chip's kind`).toHaveAttribute("data-object-kind", kind);
      await expect(selected, `${what}: nothing is selected before arrival`).toHaveCount(0);
      expect(await cardView(page, id), `${what}: starts out of view`).not.toBe("in view");

      // The real click on the chip.
      await page.goto(proposed);
      const chip = page.getByTestId(`document-chip-${id}`);
      await expect(chip, `${what}: chip href`).toHaveAttribute("href", `${wall}#obj-${id}`);
      const origin = new URL(page.url()).origin;
      await chip.click();
      await expect(page, `${what}: the wall loads at its card's hash`).toHaveURL(`${origin}${wall}#obj-${id}`);
      await page.waitForLoadState("load");
      await expect(canvas, `${what}: the wall`).toHaveAttribute("data-board-mode", "authoring");

      // Selected, alone, and in view.
      const card = page.getByTestId(`card-${id}`);
      await expect(card, `${what}: the card is selected`).toHaveAttribute("data-selected", "true");
      await expect(selected, `${what}: no other card or thread is selected`).toHaveCount(1);
      await expect(selected, `${what}: the one selection is this card`).toHaveAttribute("data-id", id);
      await expect(canvas, `${what}: the canvas carries a card selection`).toHaveAttribute("data-selection", "card");
      await expect.poll(() => cardView(page, id), { message: `${what}: the card is revealed` }).toBe("in view");
    }
  });
});
