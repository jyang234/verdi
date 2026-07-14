---
id: obligation/forge-transport--ac-1--static
kind: obligation
title: "One transport seam, one disclosure site"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/forge-transport" }
frozen: { at: 2026-07-13, commit: b52051b1058e17bb26f1f54c79bdaa8d2dbec71d }
---
# One transport seam, one disclosure site

The static evidence must show internal/httpjson existing as the single
transport seam — request build, auth hook, deadline, classifier hook,
tolerant-subset decode — with code-health dc-1's foreign-payload decode
policy stated in ITS doc comment and nowhere else, and github/gitlab/jira
transports as thin bindings with no surviving per-adapter copy of the
plumbing.
