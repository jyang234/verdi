package ritualwitness

import (
	"slices"
	"strings"
)

// config judges every changed local configuration key (SI-325 (7)): outside,
// except a branch's upstream configuration (branch.<b>.remote,
// branch.<b>.merge) for a branch that a push the ritual may make created
// or moved on the remote, which belongs to may_push (parent dc-7; SI-314
// (4a)) and is attributed to a logged push.
func (e *evaluation) config() []Verdict {
	var out []Verdict
	for _, k := range unionKeys(e.b.Config, e.a.Config) {
		bv, had := e.b.Config[k]
		av, has := e.a.Config[k]
		if had && has && slices.Equal(bv, av) {
			continue
		}
		verb := "changed"
		switch {
		case !had:
			verb = "set"
		case !has:
			verb = "unset"
		}
		out = append(out, Verdict{Field: "config", Detail: "config " + k + " " + verb,
			Status: classify(e.upstreamOfPush(k), e.at.has("", primPush))})
	}
	return out
}

// upstreamOfPush reports whether key is branch.<b>.remote or
// branch.<b>.merge for a branch a may-push push created or moved.
func (e *evaluation) upstreamOfPush(key string) bool {
	rest, ok := strings.CutPrefix(key, "branch.")
	if !ok || !e.decl.MayPush {
		return false
	}
	for _, suffix := range []string{".remote", ".merge"} {
		if branch, ok := strings.CutSuffix(rest, suffix); ok && branch != "" {
			return e.remotePushedTo("refs/heads/" + branch)
		}
	}
	return false
}

// gitDir judges every changed file under the common directory's hooks/
// and info/ (SI-325 (7)): no field admits one, so each is outside.
func (e *evaluation) gitDir() []Verdict {
	var out []Verdict
	for _, p := range unionKeys(e.b.GitFiles, e.a.GitFiles) {
		bv, had := e.b.GitFiles[p]
		av, has := e.a.GitFiles[p]
		if had == has && bv == av {
			continue
		}
		out = append(out, Verdict{Field: "git_dir", Status: Outside, Detail: p + " " + changeVerb(had, has)})
	}
	return out
}
