# testdata/violations/VL-015

VL-015 (supersession manifest completeness and fidelity — 02 §Lint rules),
implemented at V1-P2.

VL-015's fixtures live inline in `internal/lint/vl015_test.go`
(`TestVL015_TableDriven`) rather than as overlay directories here: the
rule needs the predecessor spec's own object manifest at its
*frozen.commit*, read from real git history (`gitx.Show`) — the shared
`examples/showcase` fixturegit history (`layers.txt`) and
`internal/artifact/v2fixture_test.go`'s own dedicated loan-workflow /
loan-workflow-v2 history each bake in *their own* golden SHAs, neither of
which is reproducible inside `internal/lint`'s separate fixturegit-built
test repos. `vl015_test.go` builds its own small, dedicated two-layer
history per test case (loan-workflow v1 draft, then v1 frozen at its own
freshly-computed SHA plus loan-workflow-v2 with the supersession: block
under test) — the same "one commit, no invented mechanism" pattern
`v2fixture_test.go` established, just with a fresh SHA each run rather
than a baked-in golden one, since nothing else in this package's corpus
cites it.

Table cases: happy path (every predecessor object classified exactly
once, `carried` byte-identical); `carried-byte-drift` (a `carried` object
whose text differs from its predecessor); `unclassified-object` (a real
predecessor object named in no bucket); `double-classified` (a predecessor
object named in more than one bucket).
