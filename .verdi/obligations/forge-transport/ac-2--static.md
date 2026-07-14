---
id: obligation/forge-transport--ac-2--static
kind: obligation
title: "Every list read is a drain walker"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# Every list read is a drain walker

The static evidence must show every REST list endpoint riding a page walker
(per_page=100 + Link rel=next on GitHub, per_page=100 + X-Next-Page on
GitLab), the GraphQL thread query carrying outer pageInfo/endCursor AND
inner comments pageInfo, and a same-signal loop guard on every walker that
fails loud instead of spinning.
