---
id: adr/vl-003-dangling-link
kind: adr
title: "VL-003 overlay: dangling link"
status: proposed
owners: [platform-team]
links:
  - { type: depends-on, ref: adr/does-not-exist-anywhere }
---
# VL-003 overlay: dangling link

`links[0].ref` is well-formed (parses as a valid unpinned ref) but names
no artifact anywhere in the committed zone. VL-003 requires every link ref
to resolve.
