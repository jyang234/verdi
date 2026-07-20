---
name: stale-skill
description: Deliberately stale fixture skill for internal/specalign's red-direction proof (spec/instruction-conformance ac-4). Never used by a real agent; committed testdata only.
---

# stale-skill (fixture only)

This fixture combines two independent drift classes in one file, proving
AC-2 and AC-3 fire together end to end.

## Finish the ritual

Once the mechanical half has run, finish up by running
`verdi board commit <board-key> --name <spec-name>` to freeze the board.

## Worked example

```
verdi frobnicate --example
```

This file intentionally carries no disclosure note anywhere.
