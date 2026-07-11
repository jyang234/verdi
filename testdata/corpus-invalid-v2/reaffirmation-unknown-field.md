---
id: reaffirmation/jira-loan-1483--ac-1
kind: reaffirmation
title: "re-affirm ac-1 as amended for jira:LOAN-1483"
schema: verdi.reaffirmation/v1
owners: [loansvc-team]
bogus_extra_field: surprise
frozen: { at: 2026-07-14, commit: 06a3f4cabb226fe9344e1645e27c344493b6b62b }
object: spec/loan-workflow-v2@06a3f4cabb226fe9344e1645e27c344493b6b62b#ac-1
hash: { old: sha256:20bb0d914cc85a12dbb4c5e85f099b69cae126b0a395780d10b98327da844bfc, new: sha256:ca80c24cd423a030096c07d690b96bfd7dcc801219a5815e0679269a6d699c97 }
---
# Re-affirmation: ac-1 amended for jira:LOAN-1483

**Re-affirmation fixture** (03 §The amendment ladder rung 4, 02 §Record
schemas: "Re-affirmation"): `spec/borrower-update-mobile`'s `story:` ref is
`jira:LOAN-1483`, so `<story-slug>` = `RefSlug("jira:LOAN-1483")` =
`jira-loan-1483`. The story's `implements` edge into
`spec/loan-workflow#ac-1` touches an object `spec/loan-workflow-v2`'s
`supersession:` block marks `amended`, so this record is required before
the story's merge gate proceeds. `hash.old`/`hash.new` are the
`(kind, id, text)` content hash (`ObjectContentHash`) of `ac-1`'s text
before ("workflow status changes are visible within one minute") and after
("workflow status changes are visible within thirty seconds") the
supersession.

The mobile update flow already meets the tightened thirty-second
visibility threshold; no implementation change is required, but the diff is
attested and audit-countable.
