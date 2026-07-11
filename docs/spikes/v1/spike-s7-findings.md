# Spike S7 — board-as-projection bidirectional edit round-trip

PLAN-V1.md §5 Phase V1-P0. Prototype code:
`/Users/johnyang/.claude/jobs/f8ad4a26/tmp/spike-s7/` (throwaway Go module,
own `go.mod`, `gopkg.in/yaml.v3`). `verdi/` was read-only reference
throughout (`internal/artifact/decode.go`'s `SplitFrontmatter`/
`DecodeStrict` idioms, `internal/lint/headings.go`'s anchor-slug
algorithm) — nothing in `verdi/` was modified.

Sample artifact: `spike-s7/testdata/spec.md`, a round-four feature spec
(`class: feature`) with `problem`/`outcome` attributes, 3 ACs (evidence +
anchor each), 1 constraint, 2 decisions (`dc-2` carries an `exempts` link),
and `stubs:` — plus a matching markdown body with anchor headings and free
prose. Deliberately YAML-hostile values are present: `problem.text` and
`co-1.text` each contain a colon+space inside the string
(`"...rejected: has no way..."`, `"...capped at: 3 attempts..."`), and
`outcome.text` / `ac-3.text` each start with a literal `"` character
(`"\"resubmitted\" documents..."`).

## 1. Naive remarshal churns almost the entire frontmatter

`protoA/main.go`: `shared.SplitFrontmatter` → `yaml.Unmarshal` into a typed
`Frontmatter` struct → change **one field** (`ac-2.text`, a card-text edit)
→ `yaml.Marshal` the whole struct back → reassemble → diff against the
original. Output: `protoA/spec.remarshaled.md`.

Result: **60 of the frontmatter's 26 lines changed** (every flow-style
`{ ... }`/`[ ... ]` entry got reformatted to expanded block style, plain
scalars that didn't need quoting got single-quoted, `owners: [platform-team]`
became a 2-line block sequence) for a single semantic edit. Verbatim excerpt
(full diff is in the terminal record; this is representative):

```diff
-title: "Borrower can resubmit a rejected document"
+title: Borrower can resubmit a rejected document
 schema: verdi.artifact/v1
 status: draft
-owners: [platform-team]
-problem: { text: "a borrower whose document was rejected: has no way to resubmit without restarting the whole application", anchor: "#problem" }
-outcome: { text: "\"resubmitted\" documents route straight back to review", anchor: "#outcome" }
+owners:
+    - platform-team
+problem:
+    text: 'a borrower whose document was rejected: has no way to resubmit without restarting the whole application'
+    anchor: '#problem'
+outcome:
+    text: '"resubmitted" documents route straight back to review'
+    anchor: '#outcome'
 story: okr:LOAN-Q3
 acceptance_criteria:
-  - { id: ac-1, text: "a borrower can resubmit a rejected document", evidence: [attestation], anchor: "#ac-1" }
-  - { id: ac-2, text: "the reviewer sees: resubmission history for the document", evidence: [behavioral, attestation], anchor: "#ac-2" }
+    - id: ac-1
+      text: a borrower can resubmit a rejected document
+      evidence:
+        - attestation
+      anchor: '#ac-1'
+    - id: ac-2
+      text: 'the reviewer sees: resubmission history for the document, oldest first'
+      evidence:
+        - behavioral
+        - attestation
+      anchor: '#ac-2'
```

Every untouched object (`ac-1`, `ac-3`, `co-1`, `dc-1`, `dc-2`, `problem`,
`outcome`, `stubs`) shows spurious churn: flow→block reformatting, quote
style flips (`"..."` → `'...'` for the colon-hostile and quote-hostile
values — Marshal's own quoting heuristic, not our choice), and indentation
width changes (2-space source → yaml.v3's 4-space block default). The body
(everything after the closing `---`) is untouched only because Prototype A
never touches it — but a git diff of this file is unreviewable: a
one-field edit produces a 60-line change, and a human reviewer (or a
`supersession:` carried/amended classifier keyed on content hash — 02
§Object model's `(kind, id, text)` identity — could tolerate this since
text is unchanged) sees no signal distinguishing the real edit from
reformatting noise.

## 2. Surgical Node-position splicing is REQUIRED, and it works

**Verdict: naive decode-marshal-remarshal is disqualified for the board
editor; surgical byte-range splicing is required and proven.**

Technique (`shared/surgical.go`, exercised by `protoB/main.go`, output
`protoB/spec.surgical.md`):

1. Parse frontmatter with `yaml.Unmarshal(fm, &node)` into a `yaml.Node`
   tree (never a typed struct) to get **positions**, not just values.
2. `yaml.Node.Line`/`.Column` (1-indexed) point at a node's **first
   character** — for a quoted scalar this is the opening quote itself, for
   a flow container (`{`/`[`) the opening delimiter. Confirmed empirically
   by dumping the sample's full node tree and cross-checking columns
   against the raw source by hand (`spike-s7/probe/main.go`).
3. Because yaml.v3 exposes no end position, re-scan the source from that
   start: `ScanQuotedSpan` (quote-aware, handles `\"` escapes and doubled
   `''`) for scalars, `FindMatchingClose` (depth-counted, quote-aware so a
   brace inside a string doesn't confuse it) for flow containers.
4. Convert `Node.Line` (relative to the **extracted frontmatter
   substring**) back to a **whole-file-relative** line by adding 1 (fm's
   line 1 = the file's line 2, immediately after the opening `---\n`), and
   splice directly against the **pristine whole-file byte buffer** — never
   against a reassembled `"---\n"+fm+"---\n"+body` string.
5. Multiple edits are collected as `(start, end, replacement)` triples
   computed against the *original* buffer, then applied **tail-to-head**
   (descending by start offset) so an edit doesn't invalidate the recorded
   offsets of edits to its left.
6. Re-extract and strict-decode the frontmatter from the result before
   treating it as written (see §4, validate-before-write).

Result on the sample: **exactly 2 changed lines** in the diff (one for the
`ac-2` text edit, one for the yarn-draw), byte count +66 (precisely the
inserted characters, nothing else). Programmatic check
(`verifyUntouchedBytesIdentical` in `protoB/main.go`), not just an
eyeballed diff: it re-locates `problem`, `outcome`, `ac-1`, `ac-3`, `co-1`,
`dc-1` by the same Node-position technique in both the original and edited
buffers and asserts `bytes.Equal` on each extracted span —
**`ALL UNTOUCHED OBJECTS BYTE-IDENTICAL: true`**. The body (everything
after the closing `---`) was never read into the edit path at all, so it
is preserved by construction, not by hope.

**Gotcha this spike caught (worth flagging for V1-P6):** verdi's own
`internal/artifact.SplitFrontmatter` idiom (`bytes.Join(lines[1:end], "\n")`)
is safe for **decoding** (its only current production use) but is **lossy
by exactly one trailing newline** if naively re-joined for **writing** —
`Join` never re-adds the `\n` that terminated the frontmatter's last line,
because that newline was consumed as the split separator. A first draft of
Prototype B reassembled `"---\n"+fm+"---\n"+body` and glued the last
frontmatter line directly onto the closing `---` with no newline between
them (`...ac-1, ac-2] }---`). Fixed by converting Node positions to
whole-file-relative offsets and splicing against the pristine full-file
buffer directly, never through split/join. **This is the surprise of the
spike**: the bug is invisible if you only inspect the frontmatter as a
Go value (still decodes and re-encodes "fine" in isolation) — it only
shows up as a byte-level file corruption on write. Any V1-P6 code that
reuses `SplitFrontmatter` for anything write-adjacent must not reassemble
through it.

## 3. Yarn-draw (typed-link append) result

Appended `{ type: implements, ref: spec/other-feature#ac-1 }` to `dc-2`'s
existing `links:` flow sequence (which already held one `exempts` entry) by
locating the **last existing element's own end offset** (via
`FindMatchingClose` on that element, not the seq's own closing bracket) and
inserting `, { ... }` immediately after it — a zero-length `Edit` at that
offset. Same tail-to-head multi-edit machinery as the text edit; both
applied in one `ApplySurgicalEdits` call. Result line:

```yaml
      links: [ { type: exempts, ref: adr/0012-outbox-loansvc-events, note: "..." }, { type: implements, ref: spec/other-feature#ac-1 } ] }
```

Re-decodes cleanly, `dc-2.links` now has 2 entries, every other object
untouched. **Disclosed scope limit**: this append technique is proven only
for a **non-empty flow sequence** (append after the last element). An
empty `links: []` (first yarn ever drawn on an object with no prior links)
needs a different insertion point (right after `[`) — not exercised here,
flagged as a follow-up for the real implementation, not re-derived from
first principles by this spike.

## 4. Anchor exact-match vs. adjacent body-prose edits

**Yes — survives.** `internal/lint/headings.go`'s `headingAnchors`/
`slugify` (copied verbatim into `shared/headings.go` as a read-only
reference implementation, not reinvented) only scans lines that start with
`#` after left-trim; free prose between headings is never inspected.
Demonstrated (`demoAdjacentProseEdit` in `protoB/main.go`): inserted a new
sentence immediately after the `## AC-2` heading line, in the body
paragraph. `#ac-2` resolves before and after (`true`/`true`), and the
heading-anchor set is unchanged. Only the heading line's own text matters;
touching prose immediately adjacent to (but not on) a heading line is
always safe for anchor resolution — confirmed, not merely asserted from
reading the spec.

## 5. Invalid-mid-keystroke failure mode → validate-before-write

Simulated (`demoInvalidMidEdit`): truncate the buffer right after a
newly-typed, still-unterminated opening quote inside `ac-2.text` (the
shape of what a naive per-keystroke autosave could produce). Result:
`yaml.Unmarshal` fails cleanly (`yaml: line 13: found unexpected end of
stream`) — it does **not** silently accept partial content or write
garbage as if it were valid.

**Recommendation for V1-P6's autosave path**: gate every write behind
**validate-before-write** — apply the surgical edit to an in-memory copy,
re-extract and `artifact.DecodeStrict` the resulting frontmatter (the exact
seam `internal/artifact/decode.go` already provides), and only replace the
on-disk file if that decode succeeds. On failure, hold the edit in the
board's in-memory/browser state (debounce further autosave attempts until
the buffer is valid again — e.g. wait for the matching closing quote) and
**never write the invalid intermediate state to the working tree**. This
matches 05 §Workbench's own framing ("an hour of board work evaporating in
someone else's working tree is exactly the silent loss this system exists
to forbid") — the failure mode to avoid is a half-written file that then
fails `verdi lint` or blocks the next successful autosave from parsing its
own prior state, not a rejected keystroke.

## 6. Binding constraints for V1-P6's implementer

- **Never decode→struct→`yaml.Marshal`→reassemble as the write path.**
  Proven lossy: quote style, flow/block style, and indentation all churn
  even for a single-field edit (§1). This would make every board edit an
  unreviewable diff and would defeat git-blame/history on spec objects.
- **Splice against the pristine whole-file byte buffer**, using
  `yaml.Node.Line`/`.Column` (converted to file-relative coordinates, not
  left relative to an extracted frontmatter substring) plus a quote-aware
  scan for the node's own end (`ScanQuotedSpan`/`FindMatchingClose` in this
  spike, or equivalent). Never reconstruct through
  `SplitFrontmatter`'s join for a write — that path drops a newline (§2's
  gotcha).
- **Batch same-write edits and apply tail-to-head** (descending start
  offset) so multiple edits computed against one parse of the original
  buffer stay valid without a re-parse between them.
- **This spike's technique covers flow-style scalars and flow containers
  only** (`{ ... }` / `[ ... ]`), which is what every §Object model example
  in 02 uses. Block-style YAML (indentation-delimited) was not proven and
  would need a different, indentation-based end-detection strategy —
  disclosed as unproven, not silently assumed to work.
- **Validate-before-write, always** (§5): re-decode the spliced result
  through `artifact.DecodeStrict` before it touches the working tree;
  hold invalid in-progress edits in memory rather than writing them.
- **Anchor resolution is robust to adjacent prose edits** (§4) — the board
  never needs to re-anchor an object just because the author edited nearby
  prose; it only needs to react to a heading-line rename (which VL-014's
  `where`-style resolution would then fail on, correctly, until the object's
  `anchor:` field is updated to match).
- **Empty-list yarn-draw (first link on an object) is a distinct insertion
  case** from append-after-last-element and needs its own splice point;
  not proven here (§3's disclosed scope limit) — V1-P6 should prove it
  explicitly rather than assume the append-case logic generalizes.
