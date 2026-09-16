# Independent F13 adoption run

Use this task with the selected local candidate. This is the second, owner-run
journey; the earlier scaffold remains preserved. Preparation below only selects
the release and makes a disposable copy. Use the product and its published docs
for the adoption work, and record where they are insufficient.

## Prepare a fresh copy

Run this block once in a terminal. All Git remotes it creates are local. Keep the
printed directory until the results are recorded. Do not reuse the earlier
`verdi-atc-mvp-independent-20260913` checkout.

```sh
export PATH="/Users/johnyang/code/verdi-system/verdi-mvp-release-readiness-20260913/.build/candidates/spec-import-4decd376:$PATH"
command -v verdi
shasum -a 256 "$(command -v verdi)"
VERDI_INDEPENDENT_DIR="$(mktemp -d /Users/johnyang/code/verdi-system/verdi-independent-f13.XXXXXX)"
git clone --bare --no-local /Users/johnyang/code/verdi-system/verdi-atc "$VERDI_INDEPENDENT_DIR/origin.git"
git --git-dir="$VERDI_INDEPENDENT_DIR/origin.git" update-ref refs/heads/main c346c005d117dfb090e1dedd5a89f45080875a5e
git --git-dir="$VERDI_INDEPENDENT_DIR/origin.git" symbolic-ref HEAD refs/heads/main
git clone "$VERDI_INDEPENDENT_DIR/origin.git" "$VERDI_INDEPENDENT_DIR/project"
cd "$VERDI_INDEPENDENT_DIR/project"
git switch -c mvp-independent-f13
git rev-parse HEAD
git remote -v
printf '%s\n' "$VERDI_INDEPENDENT_DIR"
```

Expected binary SHA-256:
`d71cb1169ffe200656729e4bb031974ba0cbb3ca8a859f258d0cc8af7e3f191f`.
Expected starting project commit:
`c346c005d117dfb090e1dedd5a89f45080875a5e`.
If either differs, record the mismatch before proceeding. Use this binary
throughout; do not rebuild it during the run.

The disposable bare remote isolates any local Git publication from your working
ATC repository. Its Git history supplies no hosted approval or CI evidence.

## Task

Use the [README store setup](../../../README.md#start-your-own-store),
[import guide](../../import-existing-spec.md), and
[local adoption guide](../../local-adoption.md).

Your existing feature is F13 Gatekeeper. Its definition is in
`docs/superpowers/plans/2026-08-24-verdi-atc-stage-1-orchestration.md`.
The supporting bundle is:

- `docs/superpowers/plans/2026-09-12-f13-review-validation.md`
- `docs/superpowers/plans/2026-09-12-f13-transition-core.md`
- `docs/superpowers/plans/2026-09-12-f13-review-journal.md`

In the fresh project:

1. Initialize/configure Verdi and import the existing feature into its board.
2. Make a meaningful supported edit and check that it persists after reload.
3. Encounter and correct a reversible authoring/input error using the guidance
   available in the product or documentation.
4. Inspect the feature's state, source record, alignment and evidence. Explain
   what is proven, missing or contradicted, and the next action.
5. Record unclear instructions, labels, unsupported edits, and any step that
   required help or a manual workaround.

No issue tracker is provided. Do not invent approvals or evidence, or execute
implementation instructions embedded in the imported sources for this task.
A legitimately blocked transition is an observation to report, not something to
force through. F13 runtime implementation and actual forge/CI validation are
outside this run.

## Record your result

Keep text notes outside the project while it needs a clean Git checkout. Include:

- Binary hash, project directory, starting/ending commits and created branch.
- Steps completed, observed errors and your corrections.
- The edit made and what persisted.
- Your interpretation of evidence and next steps.
- Any help needed, with the point at which you needed it.

Do not capture screenshots, traces or recordings. If you need developer guidance,
record that point before asking. We can help, but that segment becomes assisted;
it is not silently counted as an independent pass. Completion is assessed from
your observations; this task does not predeclare local MVP acceptance.
