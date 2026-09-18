package skillpack

import (
	"fmt"
	"path"
)

// Host is a coding-agent harness that reads skills from a repository.
type Host string

const (
	HostClaude Host = "claude"
	HostCodex  Host = "codex"
)

// Hosts lists every supported host in render order.
func Hosts() []Host { return []Host{HostClaude, HostCodex} }

// ParseHosts maps the --host flag value to hosts: "" and "all" select
// every host; a single host name selects that host; anything else is a
// usage error.
func ParseHosts(s string) ([]Host, error) {
	switch s {
	case "", "all":
		return Hosts(), nil
	case string(HostClaude):
		return []Host{HostClaude}, nil
	case string(HostCodex):
		return []Host{HostCodex}, nil
	}
	return nil, fmt.Errorf("skillpack: unknown host %q (want claude, codex, or all)", s)
}

// dir is the repository-relative skills directory each host scans
// (Claude Code: .claude/skills; Codex: .agents/skills — SI-201).
func (h Host) dir() string {
	switch h {
	case HostClaude:
		return ".claude/skills"
	case HostCodex:
		return ".agents/skills"
	}
	return ""
}

// Path is the slash-separated repository-relative path of one rendered
// skill: <host dir>/verdi-<skill>/SKILL.md.
func Path(h Host, skill string) string {
	return path.Join(h.dir(), "verdi-"+skill, "SKILL.md")
}
