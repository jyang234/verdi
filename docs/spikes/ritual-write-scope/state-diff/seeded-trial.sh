#!/bin/sh
# seeded-trial.sh — the ac-2-shaped state-diff trial (fix round 1, finding F-2).
#
# The first trial ran on a PRISTINE clone, so its before/after `git status`
# snapshots were both empty and "the command log explains every effect" was
# true only because there was nothing to explain. ac-2 demands a fixture
# "seeded with an untracked file, a pre-staged unrelated index entry, and a
# dirty tracked file"; this script seeds exactly those three, then runs two
# rituals against the SAME seeded state:
#
#   1. `close --force-local` — reaches runClose, whose first statement is
#      requireCleanIndex (cmd/verdi/close.go:655). Its refusal is the
#      empirical witness for F-1 (close cannot carry a foreign index entry
#      because it refuses to begin at all). Snapshotted again afterwards to
#      prove the refusal precedes every mutation.
#   2. `design start` — unguarded, so the same seeded index reaches its
#      pathspec-less `git commit`. The commit's own file list settles
#      UAT-036 for design start empirically.
#
# Re-runnable (constraint 7). Usage, from anywhere:
#   sh seeded-trial.sh <source-worktree> <patched-verdi-binary> <output-dir>
# The patched binary is built from a /tmp clone with recorder.patch applied
# (never from a worktree); the ritual itself runs only in a /tmp clone
# (constraint 4). GOPROXY=off keeps the trial off the network (co-3).
set -eu

SRC=${1:?usage: seeded-trial.sh <source-worktree> <verdi-binary> <output-dir>}
BIN=${2:?usage: seeded-trial.sh <source-worktree> <verdi-binary> <output-dir>}
OUT=${3:?usage: seeded-trial.sh <source-worktree> <verdi-binary> <output-dir>}
CLONE=${CLONE:-/tmp/fix-rws-trial}

rm -rf "$CLONE"
git clone -q --no-hardlinks "$SRC" "$CLONE"
mkdir -p "$OUT"

snap() {
	git -C "$CLONE" for-each-ref >"$OUT/$1.for-each-ref.txt"
	git -C "$CLONE" ls-files -s >"$OUT/$1.ls-files-s.txt"
	git -C "$CLONE" status --porcelain=v2 >"$OUT/$1.status.txt"
}

# --- ac-2's three seeded conditions, one file each ------------------------
printf 'operator scratch the ritual never named\n' >"$CLONE/spike-untracked.txt"
printf 'unrelated work the operator staged before the ritual\n' >"$CLONE/spike-foreign-staged.txt"
git -C "$CLONE" add -- spike-foreign-staged.txt
printf '\n<!-- spike: dirty tracked edit, never staged -->\n' >>"$CLONE/README.md"

snap seeded-before

# --- ritual 1: close, which must refuse before touching anything ---------
rc=0
( cd "$CLONE" && VERDI_SPIKE_GITLOG="$OUT/seeded-close-gitlog.tsv" GOPROXY=off \
	"$BIN" close --force-local spec/ritual-write-scope ) \
	>"$OUT/seeded-close-stdout.txt" 2>"$OUT/seeded-close-stderr.txt" || rc=$?
echo "$rc" >"$OUT/seeded-close-exit.txt"
snap seeded-after-close

# --- ritual 2: design start, unguarded, against the same seeded state ----
rc=0
( cd "$CLONE" && VERDI_SPIKE_GITLOG="$OUT/seeded-designstart-gitlog.tsv" GOPROXY=off \
	"$BIN" design start --kind feature --name seeded-trial --defer-statements ) \
	>"$OUT/seeded-designstart-stdout.txt" 2>"$OUT/seeded-designstart-stderr.txt" || rc=$?
echo "$rc" >"$OUT/seeded-designstart-exit.txt"
snap seeded-after

# --- the diffs, plus the commit's own file list (the UAT-036 witness) ----
for k in for-each-ref ls-files-s status; do
	diff "$OUT/seeded-before.$k.txt" "$OUT/seeded-after.$k.txt" >"$OUT/diff.seeded.$k.txt" || true
	diff "$OUT/seeded-before.$k.txt" "$OUT/seeded-after-close.$k.txt" >"$OUT/diff.seeded-close.$k.txt" || true
done
git -C "$CLONE" show --name-only --format='%H%n%s' HEAD >"$OUT/seeded-designstart-commit-files.txt"

echo "seeded trial complete: $OUT"
