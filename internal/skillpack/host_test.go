package skillpack

import (
	"strings"
	"testing"
)

func TestParseHosts(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Host
		err  bool
	}{
		{"", []Host{HostClaude, HostCodex}, false},
		{"all", []Host{HostClaude, HostCodex}, false},
		{"claude", []Host{HostClaude}, false},
		{"codex", []Host{HostCodex}, false},
		{"cursor", nil, true},
		{"Claude", nil, true},
	} {
		got, err := ParseHosts(tc.in)
		if (err != nil) != tc.err {
			t.Fatalf("ParseHosts(%q) err = %v, want err %v", tc.in, err, tc.err)
		}
		if !tc.err && strings.Join(hostStrings(got), ",") != strings.Join(hostStrings(tc.want), ",") {
			t.Fatalf("ParseHosts(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func hostStrings(hs []Host) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, string(h))
	}
	return out
}

func TestPath(t *testing.T) {
	for _, tc := range []struct {
		h           Host
		skill, want string
	}{
		{HostClaude, "specify", ".claude/skills/verdi-specify/SKILL.md"},
		{HostCodex, "tasks", ".agents/skills/verdi-tasks/SKILL.md"},
	} {
		if got := Path(tc.h, tc.skill); got != tc.want {
			t.Fatalf("Path(%s,%s) = %q, want %q", tc.h, tc.skill, got, tc.want)
		}
	}
}
