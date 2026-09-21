#!/bin/sh
# reduce-operands.sh — the operand-preserving reduction (fix round 1, F-3).
#
# `gitlog-dedup-all-verbs.tsv` keeps only (who, subcommand, flags), which
# discards exactly the tokens ac-1's refs_create and stage_paths fields are
# made of: branch names, pathspecs, and the `--` separator that decides
# whether a commit is scoped. This reduction keeps the FULL argument vector
# of every mutating invocation of the five governed rituals.
#
# Reduction rule, in order:
#   1. Source is the raw recorder log, one `<who>\t<dir>\t<args>` line per
#      gitx invocation (recorder.patch).
#   2. Keep only the five governed rituals, by `who`:
#        verdi design start      -> design_start
#        verdi build start       -> build_start
#        verdi close ...         -> close        (any flag/arg form)
#        verdi policy adopt      -> policy_adopt
#        commitdesign.test ...   -> commit_to_design  (the package IS the
#                                  ritual; it has no CLI verb)
#   3. Keep only MUTATING invocations. The allowlist below is derived from
#      the raw log's own subcommand census (`cut -f3 raw | awk '{print $1}'
#      | sort | uniq -c`), so no observed subcommand is silently dropped:
#      unconditionally mutating -- add, checkout, commit, commit-tree,
#      update-ref, update-index, read-tree, write-tree, branch, merge,
#      worktree; conditionally -- hash-object only with -w, config only when
#      it is not a --get read, remote only with add/set-url/remove.
#      Everything else in the census (show, show-ref, rev-parse, ls-tree,
#      status, symbolic-ref, rev-list, merge-base, log, ls-files,
#      for-each-ref, diff, var) is read-only.
#   4. Normalize ONLY the volatile fixture root, never the operands: the
#      invocation's own `dir` (and its /private-prefixed twin) becomes
#      <repo>, and any residual per-test temp root becomes <tmp>. Branch
#      names, pathspecs, `--` separators, flags and commit messages stay
#      byte-exact.
#   5. sort -u over (ritual, args), then sort by ritual.
#
# Usage: sh reduce-operands.sh <raw-log.tsv> > inventory-operands.tsv
set -eu
RAW=${1:?usage: reduce-operands.sh <raw-recorder-log.tsv>}

printf 'ritual\targs (full vector, operands intact)\n'

awk -F'\t' '
function ritual(who) {
	if (who == "verdi design start") return "design_start"
	if (who == "verdi build start") return "build_start"
	if (who ~ /^verdi close( |$)/) return "close"
	if (who == "verdi policy adopt") return "policy_adopt"
	if (who ~ /^commitdesign\.test( |$)/) return "commit_to_design"
	return ""
}
function mutating(args,   sub1) {
	sub1 = args
	gsub(/ .*$/, "", sub1)
	if (sub1 ~ /^(add|checkout|commit|commit-tree|update-ref|update-index|read-tree|write-tree|branch|merge|worktree)$/) return 1
	if (sub1 == "hash-object") return (args ~ / -w( |$)/)
	if (sub1 == "config") return (args !~ / --get/)
	if (sub1 == "remote") return (args ~ /^remote (add|set-url|remove|rename) /)
	return 0
}
{
	r = ritual($1)
	if (r == "") next
	if (!mutating($3)) next
	dir = $2; args = $3
	if (dir != "") {
		gsub(/[][\\^$.|?*+(){}]/, "\\\\&", dir)
		gsub("/private" dir, "<repo>", args)
		gsub(dir, "<repo>", args)
	}
	gsub(/(\/private)?\/var\/folders\/[^ ]*\/T\/[A-Za-z0-9_]+[0-9]+\/[0-9]+/, "<tmp>", args)
	gsub(/\/tmp\/[A-Za-z0-9_.-]+/, "<tmp>", args)
	print r "\t" args
}
' "$RAW" | sort -u -t"$(printf '\t')" -k1,1 -k2,2
