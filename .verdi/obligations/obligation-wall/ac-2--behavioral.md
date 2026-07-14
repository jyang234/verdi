---
id: obligation/obligation-wall--ac-2--behavioral
kind: obligation
title: "A browser e2e shows an AC card reading out its obligation and disclosing a gap"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-wall" }
frozen: { at: 2026-07-14, commit: c7c49bbba154da36cee5ded16fb16bd262962591 }
---
# A browser e2e shows an AC card reading out its obligation and disclosing a gap

The behavioral evidence must show a Playwright e2e (36-board-obligation-wall) proving a story-AC card renders an authored obligation's demand (its title, the prose a hover away in the tooltip) and a disclosed 'no obligation' badge for a declared-but-un-obligated kind — legible on the wall itself (feature co-3).
