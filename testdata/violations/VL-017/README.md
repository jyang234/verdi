# testdata/violations/VL-017

VL-017 (open-question stickies resolved-or-carried — 02 §Lint rules),
implemented at V1-P2 (which also fills the `open_questions:` frontmatter
block gap this skeleton's V1-P1 README flagged — R4-I-16, 02 §Object
model).

VL-017's fixtures live inline in `internal/lint/vl017_test.go` rather than
as overlay directories under this one: the rule's mutable-zone-present
twin needs an annotation JSONL file written *untracked* directly to the
built repo's working tree (`.verdi/data/mutable/annotations/`, 01 §Zones —
the mutable zone is never git-tracked, VL-013), which the shared
`overlayLayer`/`buildLintRepo` harness commits everything it's given
through fixturegit — the wrong mechanism for content that must never be
committed. The mutable-zone-absent twin removes the directory
`buildLintRepo` provisions by default (see harness_test.go's
`provisionMutableZone`) rather than needing its own overlay at all.
