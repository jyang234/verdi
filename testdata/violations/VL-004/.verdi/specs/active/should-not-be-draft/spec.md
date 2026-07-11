---
id: spec/should-not-be-draft
kind: spec
class: feature
title: "VL-004 overlay: draft on default branch"
status: draft
owners: [platform-team]
story: jira:LOAN-0002
acceptance_criteria:
  - { id: ac-1, text: "placeholder", evidence: [static] }
---
# VL-004 overlay: draft on default branch

This file decodes fine on its own — the violation is contextual: VL-004
fires when a `status: draft` artifact exists on the default branch (or an
MR targeting it), per I-14's git-aware baseline. Layering this file onto
the corpus's default branch and linting there is the violation; linting a
design branch is not.
