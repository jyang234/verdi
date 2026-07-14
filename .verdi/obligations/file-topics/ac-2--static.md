---
id: obligation/file-topics--ac-2--static
kind: obligation
title: "accept.go holds one topic and the test names a real file"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/file-topics" }
frozen: { at: 2026-07-14, commit: 15d60efbe02636c1112907ded017f80eb4c46e94 }
---
# accept.go holds one topic and the test names a real file

The static evidence must show stub-match in stubmatch.go (the production
twin stubmatch_test.go always named), the supersession flow in its own
file, and accept.go retaining only runAccept — bodies verbatim.
