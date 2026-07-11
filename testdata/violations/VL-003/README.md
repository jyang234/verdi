# testdata/violations/VL-003

- `dangling-link/` — an ADR's `links[0].ref` is well-formed but names no
  artifact anywhere in the committed zone.
- `dangling-pin/` — a feature spec's `context[0]` is a well-formed pinned
  ref, but the pinned commit is not real git history.
- `dangling-fragment/` — a story spec's `implements` edge names a real
  target spec (`spec/stale-decline`) but an object id (`ac-99`) that spec
  does not declare — VL-003 (rescoped, R4-I-3) resolves object-id fragments
  against the target's parsed frontmatter objects.
