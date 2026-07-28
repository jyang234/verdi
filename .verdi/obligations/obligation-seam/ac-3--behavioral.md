---
id: obligation/obligation-seam--ac-3--behavioral
kind: obligation
title: "a refusal after scaffolding unlinks exactly the newly-created stubs, leaving a pristine tree"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-seam" }
frozen: { at: 2026-07-21, commit: af0edd77237b6c52cffda3bc344c020ff5fad58e }
---
# a refusal after scaffolding unlinks exactly the newly-created stubs, leaving a pristine tree

The behavioral evidence must show a built-binary test that induces an
UNRELATED quartet lint refusal (e.g. a dangling `layout.json` positions
key, mirroring `TestRunAccept_RefusesDanglingLayoutKey`'s existing fixture
shape, or any other violation already within the D6-23 quartet scope) on
a story spec that ALSO declares one or more `(ac, kind)` pairs with no
pre-existing obligation. Running `verdi accept` against this fixture must:
exit 1 (a verdict refusal, not the operational path); leave the spec.md
frontmatter's `status:` still `draft` (no flip committed); leave HEAD
unchanged (no commit created at all — not even an uncommitted-but-staged
state); and leave NO file at any of the `(ac, kind)` pairs' convention
paths — the `.verdi/obligations/<spec>/` directory itself must not exist
afterward if it did not exist before, proving the backstop's own
mid-flight scaffolds were unlinked, not merely left uncommitted.

A second case must prove pre-existing obligations are never touched by
this cleanup: repeat with one pair already covered by a real obligation
and a second pair missing, and confirm that after the induced refusal the
pre-existing file is still present, byte-identical, while the file the
backstop created for the missing pair is gone.

A third case must prove the surface this cleanup protects: after an
induced refusal leaves the tree pristine, `verdi obligation author` for
the same `(story, ac, kind)` the backstop had scaffolded and then
unlinked must succeed as an ordinary create (never refused for
"already exists") — proving the backstop's own scaffold-then-unlink
never pre-empts the authoring surface it defers to (the sentence O-1b
exists to make true).
