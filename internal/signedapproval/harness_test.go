package signedapproval

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// testRepo is a hermetic git repository: fixed identity and dates, no
// signing, and no global or system configuration. Real signatures are
// never needed — the verifier is a fake keyed by commit.
type testRepo struct {
	t   *testing.T
	dir string
}

func newTestRepo(t *testing.T) *testRepo {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	r := &testRepo{t: t, dir: t.TempDir()}
	r.git("init", "-q", "--initial-branch=main")
	r.git("config", "user.name", "Verdi Fixture")
	r.git("config", "user.email", "fixture@verdi.invalid")
	r.git("config", "commit.gpgsign", "false")
	r.git("config", "gc.autoDetach", "false")
	r.git("config", "maintenance.auto", "false")
	return r
}

func (r *testRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"TZ=UTC",
		"GIT_AUTHOR_DATE=1704067200 +0000",
		"GIT_COMMITTER_DATE=1704067200 +0000",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r *testRepo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// commit stages everything and commits it, returning the new commit.
func (r *testRepo) commit(msg string) string {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "--no-verify", "--allow-empty", "-m", msg)
	return r.git("rev-parse", "HEAD")
}

// shallowClone clones the repository at depth into a fresh directory.
func (r *testRepo) shallowClone(depth int) string {
	r.t.Helper()
	return fixturegit.ShallowClone(r.t, &fixturegit.Repo{Dir: r.dir}, depth)
}

// row is one approval row as written into a test artifact.
type row struct{ role, principal string }

// exemptionDoc renders a test artifact: fixed frontmatter keys around the
// approvals sequence, then a body. extra is inserted before approvals.
func exemptionDoc(extra, body string, rows ...row) string {
	var b strings.Builder
	b.WriteString("---\nschema: test/v1\nid: exemption/x\n")
	b.WriteString(extra)
	b.WriteString("approvals:\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "  - role: %s\n    principal: %s\n", r.role, r.principal)
	}
	b.WriteString("expiry: \"2026-12-31\"\n---\n")
	b.WriteString(body)
	return b.String()
}

const (
	artifactPath    = ".verdi/policy/exemptions/x.md"
	defaultBody     = "The departure is bounded.\n"
	signedSource    = "github-signed"
	forgeSource     = "github-forge"
	ownerRole       = "policy-owner"
	escalationRole  = "unsealed-exemption-escalation"
	exemptionTarget = "policy-exemption-approval"
	usesMetric      = "unsealed-provenance-exemption-uses-90d"
)

func principalFor(t *testing.T, source, subject string) string {
	t.Helper()
	id, err := gp.CanonicalPrincipalID(source, subject)
	if err != nil {
		t.Fatal(err)
	}
	return string(id)
}

// fakeVerifier answers from facts seeded per commit and records calls.
type fakeVerifier struct {
	facts map[string]CommitVerification
	err   error
	calls []string
}

func (f *fakeVerifier) VerifyCommit(_ context.Context, commit string) (CommitVerification, error) {
	f.calls = append(f.calls, commit)
	if f.err != nil {
		return CommitVerification{}, f.err
	}
	v, ok := f.facts[commit]
	if !ok {
		return CommitVerification{}, fmt.Errorf("fake verifier: no fact seeded for %s", commit)
	}
	return v, nil
}

func verifiedBy(commit, account string) CommitVerification {
	return CommitVerification{Commit: commit, Available: true, Verified: true, SignerAccountID: account, ProviderSnapshotID: snapshotFor(commit)}
}

func unverified(commit string) CommitVerification {
	return CommitVerification{Commit: commit, Available: true, ProviderSnapshotID: snapshotFor(commit)}
}

func unavailable(commit, reason string) CommitVerification {
	return CommitVerification{Commit: commit, UnavailableReason: reason}
}

func snapshotFor(commit string) string {
	return "sha256:" + strings.Repeat("0", 64-len(commit[:40])) + commit[:40]
}

// testCatalog is the closed universe the profile fixtures decode against.
func testCatalog() gp.Catalog {
	return gp.Catalog{
		Roles:             []string{ownerRole, escalationRole},
		Transitions:       []string{exemptionTarget},
		EscalationMetrics: []string{usesMetric},
	}
}

// soloProfileYAML maps accounts 1001 and 2002 of the signed-commit source
// to the owner role and 1001 to the escalation role; a forge source maps
// 1001 too, so a forge principal is a real, mapped principal that is still
// not a signed-commit principal.
const soloProfileYAML = `schema: verdi.governance-profile/v1
id: solo-signed
class: solo
applicable_transitions: [policy-exemption-approval]
identity_trust_sources:
  - {id: github-forge, kind: forge}
  - {id: github-signed, kind: signed-commit}
role_mappings:
  - {role: policy-owner, trust_source: github-signed, subjects: ["1001", "2002"]}
  - {role: unsealed-exemption-escalation, trust_source: github-signed, subjects: ["1001"]}
  - {role: policy-owner, trust_source: github-forge, subjects: ["1001"]}
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [policy-exemption-approval], roles: [policy-owner], minimum: 1}
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds:
  - {transitions: [policy-exemption-approval], metric: unsealed-provenance-exemption-uses-90d, at_least: 2, required_roles: [unsealed-exemption-escalation]}
`

func decodeProfile(t *testing.T, raw string) gp.Profile {
	t.Helper()
	p, err := gp.DecodeProfile([]byte(raw), testCatalog())
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return p
}

func soloProfile(t *testing.T) gp.Profile { return decodeProfile(t, soloProfileYAML) }
