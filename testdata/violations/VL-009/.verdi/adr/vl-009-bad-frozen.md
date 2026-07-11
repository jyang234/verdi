---
id: adr/vl-009-bad-frozen
kind: adr
title: "VL-009 overlay: frozen stamp names a commit outside history"
status: accepted
owners: [platform-team]
decided: 2026-04-01
frozen: { at: 2026-04-01, commit: 0000000000000000000000000000000000000000 }
---
# VL-009 overlay: frozen stamp names a commit outside history

`frozen.commit` is well-formed (40 lowercase hex characters) but is not a
real commit anywhere in the corpus's git history. VL-009 requires frozen
artifacts to carry a *valid* frozen stamp — "valid" includes naming a real
commit, not just matching the field's shape (which internal/artifact,
lacking git access, cannot check at decode time).
