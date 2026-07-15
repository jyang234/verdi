import { test, expect, type Locator } from "@playwright/test";
import { addSticky } from "./helpers";
import { SHOWCASE, slotChipTestId, boardPath } from "./fixtures";

// spec/evidence-slot: a story AC card renders, per DECLARED evidence
// kind, what that kind HOLDS — the fold's own record state — on the SAME
// row that already reads what the kind demands (the obligation column,
// ac-3/dc-2: one line per kind, never a second per-kind list). An empty
// slot badges through the badge compute layer with a full fold:empty-slot
// derivation record (ac-2/dc-3) and blocks nothing (co-2).
//
// Two fixture walls:
//  - SHOWCASE.OBLIGATION_WALL_SPEC (refi-decline-replay) has NO derived tree at
//    all — dc-1's ordinary design-branch authoring state. Every declared
//    kind wears a CALM empty chip, the receipt discloses the location
//    probed as absent, and a real write path still succeeds.
//  - SHOWCASE.SLOT_WALL_SPEC (decline-slot-wall) has REAL fold-visible state: a
//    derived-tree CI static record (held), an attestation file on disk
//    (held), and nothing behavioral (empty) — ac-1's filled-versus-empty
//    proof on one card.

// assertOneRowPerKind is ac-3's coherence claim on one card: each
// declared kind reads as exactly ONE row carrying both its obligation
// half and its record-state chip, inside the card's single per-kind list.
async function assertOneRowPerKind(
  card: Locator,
  acId: string,
  kinds: string[],
): Promise<void> {
  // One per-kind list on the card, ever.
  await expect(card.locator(".card-obligations")).toHaveCount(1);
  // Exactly one row per declared kind, and no row for anything else.
  await expect(card.locator("[data-obligation-kind]")).toHaveCount(
    kinds.length,
  );
  for (const kind of kinds) {
    const row = card.locator(`.obligation[data-obligation-kind="${kind}"]`);
    await expect(row).toHaveCount(1);
    // Both halves live INSIDE the one row: the demand (an obligation
    // title or the disclosed "no obligation" badge) and the holdings (the
    // record-state chip).
    await expect(
      row.locator(".obligation-title, .obligation-badge"),
    ).toHaveCount(1);
    await expect(row.getByTestId(slotChipTestId(acId, kind))).toHaveCount(1);
  }
}

test.describe("evidence slot: a story AC card reads out what each kind holds", () => {
  test("a never-synced wall wears calm empty slots, its badge discloses the probe, and writing still works", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.OBLIGATION_WALL_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const card = page.getByTestId(`card-${SHOWCASE.OBLIGATION_WALL_AC}`);
    await expect(card).toBeVisible();

    // ac-3: one row per declared kind, each carrying demand AND holdings.
    await assertOneRowPerKind(card, SHOWCASE.OBLIGATION_WALL_AC, [
      SHOWCASE.OBLIGATION_WALL_PRESENT_KIND,
      SHOWCASE.OBLIGATION_WALL_MISSING_KIND,
    ]);

    // The obligated kind's row: the authored demand on the left half,
    // the empty-slot chip on the right — one line, both truths.
    const presentRow = card.locator(
      `.obligation[data-obligation-kind="${SHOWCASE.OBLIGATION_WALL_PRESENT_KIND}"]`,
    );
    await expect(presentRow.locator(".obligation-title")).toContainText(
      SHOWCASE.OBLIGATION_WALL_DEMAND,
    );
    const presentChip = presentRow.getByTestId(
      slotChipTestId(SHOWCASE.OBLIGATION_WALL_AC, SHOWCASE.OBLIGATION_WALL_PRESENT_KIND),
    );
    await expect(presentChip).toHaveAttribute("data-slot-state", "empty");
    await expect(presentChip).toHaveText("no record");

    // The un-obligated kind's row: BOTH disclosures share the dashed
    // pending register — "no obligation" (demand) and "no record"
    // (holdings) — a calm authoring fact, not an alarm (dc-1).
    const missingRow = card.locator(
      `.obligation[data-obligation-kind="${SHOWCASE.OBLIGATION_WALL_MISSING_KIND}"]`,
    );
    await expect(missingRow.locator(".obligation-badge")).toHaveText(
      "no obligation",
    );
    await expect(
      missingRow.getByTestId(
        slotChipTestId(SHOWCASE.OBLIGATION_WALL_AC, SHOWCASE.OBLIGATION_WALL_MISSING_KIND),
      ),
    ).toHaveText("no record");

    // ac-2: the empty slots badge through the badge compute layer — a
    // real button on the card's badge surface whose derivation record
    // names the derived-tree location probed (revision "absent": nothing
    // was ever synced), pins the spec by content digest, and states
    // per-kind that nothing was found. No timestamp anywhere (co-1).
    const badge = card.locator(
      '.badge-chip[data-badge-source="fold:empty-slot"]',
    );
    await expect(badge).toBeVisible();
    expect(await badge.evaluate((el) => el.tagName)).toBe("BUTTON");
    const raw = await badge.getAttribute("data-badge-record");
    expect(raw).toBeTruthy();
    const record = JSON.parse(raw!);
    expect(record.source).toBe("fold:empty-slot");
    const inputNames = record.inputs.map((i: { name: string }) => i.name);
    expect(inputNames).toContain("spec");
    expect(inputNames).toContain("derived-tree");
    for (const input of record.inputs) {
      if (input.name === "spec") {
        expect(input.revision).toMatch(/^sha256:[0-9a-f]{64}$/);
      }
      if (input.name === "derived-tree") {
        expect(input.path).toContain(
          `derived/spec--${SHOWCASE.OBLIGATION_WALL_SPEC}`,
        );
        expect(input.revision).toBe("absent");
      }
    }
    expect(record.records).toContain(
      `${SHOWCASE.OBLIGATION_WALL_PRESENT_KIND}: no current record`,
    );
    expect(record.records).toContain(
      `${SHOWCASE.OBLIGATION_WALL_MISSING_KIND}: no current record`,
    );
    // co-1: digests, never wall-clock — nothing date-shaped in the record.
    expect(raw!).not.toMatch(/\d{4}-\d{2}-\d{2}/);

    // co-2: the badged, all-empty wall refuses nothing — a real write
    // path succeeds (addSticky asserts the autosave receipt itself)...
    await addSticky(page, "an empty slot is a fact, not a gate");

    // ...and after the post-mutation fragment swap (one renderer for page
    // and fragment) the slots and badge are still worn.
    await assertOneRowPerKind(card, SHOWCASE.OBLIGATION_WALL_AC, [
      SHOWCASE.OBLIGATION_WALL_PRESENT_KIND,
      SHOWCASE.OBLIGATION_WALL_MISSING_KIND,
    ]);
    await expect(
      card.locator('.badge-chip[data-badge-source="fold:empty-slot"]'),
    ).toBeVisible();
  });

  test("a folded record fills exactly its kind's slot; an attestation fills its own; siblings stay empty", async ({
    page,
  }) => {
    await page.goto(boardPath(SHOWCASE.SLOT_WALL_SPEC));
    await expect(page.getByTestId("board")).toHaveAttribute(
      "data-board-mode",
      "authoring",
    );

    const card = page.getByTestId(`card-${SHOWCASE.SLOT_WALL_AC}`);
    await expect(card).toBeVisible();

    // ac-1: every declared kind appears as a slot entry — and only the
    // declared kinds (assertOneRowPerKind counts exactly three rows).
    await assertOneRowPerKind(card, SHOWCASE.SLOT_WALL_AC, [
      SHOWCASE.SLOT_HELD_KIND,
      SHOWCASE.SLOT_EMPTY_KIND,
      SHOWCASE.SLOT_ATTESTED_KIND,
    ]);

    // The derived-tree static record fills the static slot...
    const heldChip = card.getByTestId(
      slotChipTestId(SHOWCASE.SLOT_WALL_AC, SHOWCASE.SLOT_HELD_KIND),
    );
    await expect(heldChip).toHaveAttribute("data-slot-state", "held");
    await expect(heldChip).toHaveText("1 record");

    // ...the attestation file fills the attestation slot...
    const attestedChip = card.getByTestId(
      slotChipTestId(SHOWCASE.SLOT_WALL_AC, SHOWCASE.SLOT_ATTESTED_KIND),
    );
    await expect(attestedChip).toHaveAttribute("data-slot-state", "held");
    await expect(attestedChip).toHaveText("attested");

    // ...and the record-less behavioral kind stays empty: the record
    // flipped exactly ITS kind, never a sibling.
    const emptyChip = card.getByTestId(
      slotChipTestId(SHOWCASE.SLOT_WALL_AC, SHOWCASE.SLOT_EMPTY_KIND),
    );
    await expect(emptyChip).toHaveAttribute("data-slot-state", "empty");
    await expect(emptyChip).toHaveText("no record");

    // dc-4: presence only — the fixture record's verdict is "pass", and
    // that word must reach no slot chip (verdicts stay with matrix/gate).
    for (const chip of [heldChip, attestedChip, emptyChip]) {
      await expect(chip).not.toContainText("pass");
      await expect(chip).not.toContainText("fail");
    }

    // ac-2/dc-3: the one empty kind badges, and the receipt cites the
    // record file actually read (a real sha256 of its exact bytes) plus
    // the derived-tree probe pinned to a commit sha — and discloses
    // per-kind what was found: the full three-way story of this card.
    const badge = card.locator(
      '.badge-chip[data-badge-source="fold:empty-slot"]',
    );
    await expect(badge).toBeVisible();
    const record = JSON.parse((await badge.getAttribute("data-badge-record"))!);
    expect(record.target).toBe(SHOWCASE.SLOT_WALL_AC);
    expect(record.label).toBe("empty slot");
    const recordFileInputs = record.inputs.filter((i: { name: string }) =>
      i.name.startsWith("record:"),
    );
    expect(recordFileInputs.length).toBe(1);
    expect(recordFileInputs[0].revision).toMatch(/^sha256:[0-9a-f]{64}$/);
    expect(recordFileInputs[0].path).toContain(
      `derived/spec--${SHOWCASE.SLOT_WALL_SPEC}`,
    );
    const tree = record.inputs.find(
      (i: { name: string }) => i.name === "derived-tree",
    );
    expect(tree.revision).toMatch(/^[0-9a-f]{7,40}$/);
    expect(record.records).toContain(`${SHOWCASE.SLOT_HELD_KIND}: 1 current record`);
    expect(record.records).toContain(
      `${SHOWCASE.SLOT_EMPTY_KIND}: no current record`,
    );
    expect(record.records).toContain(
      `${SHOWCASE.SLOT_ATTESTED_KIND}: attestation file present`,
    );
  });
});
