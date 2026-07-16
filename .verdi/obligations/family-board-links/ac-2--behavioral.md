---
id: obligation/family-board-links--ac-2--behavioral
kind: obligation
title: "An e2e follows a feature stub card to a matching active story's board and sees an archived match disclosed"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/family-board-links" }
frozen: { at: 2026-07-16, commit: 71bf8422596fadd4c3060aa719785b91e114924f }
---
# An e2e follows a feature stub card to a matching active story's board and sees an archived match disclosed

The behavioral evidence must show a Playwright e2e in
`e2e/tests/43-family-board-links.spec.ts` driving the served showcase feature
board `stale-decline`, whose stub `{ slug: borrower-update-api,
acceptance_criteria: [ac-2] }` is genuinely realized by an ACTIVE story, and
following the stub card to `/board/spec/borrower-update-api`. Per ADJ-28's
firing semantics it must ALSO drive the NEW archived-match fixture (dc-5) — a
stub whose implementing story resolves only in `specs/archive/` with its
`design/<slug>` branch still present — and assert the card renders the
story-board link WITH its archived state disclosed and does NOT render the "not
yet in this checkout's active store" in-between notice. No network (co-2).
