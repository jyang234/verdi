package repositoryfacts

import (
	"strings"
	"testing"
)

// =========================================================================
// SI-251 / SI-257: the CI-ref fact's shape, its known/value invariants, the
// branch-name rules it is validated against, and "the branch being closed".
// =========================================================================

func TestCIRefFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       CIRefFact
		wantErr bool
	}{
		{"outside CI: zero value", CIRefFact{}, false},
		{"known github branch", CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "close/spec-x"}, false},
		{"known gitlab branch", CIRefFact{Known: true, Provider: CIProviderGitLab, Name: "main"}, false},
		{"unknown with a closed reason", CIRefFact{Reason: CIRefReasonRemoteTrackingNotHead}, false},
		{"known with empty name", CIRefFact{Known: true, Provider: CIProviderGitHub}, true},
		{"known with invalid branch name", CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "main~1"}, true},
		{"known with empty provider", CIRefFact{Known: true, Name: "main"}, true},
		{"known with unknown provider", CIRefFact{Known: true, Provider: "jenkins", Name: "main"}, true},
		{"known with a reason", CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "main", Reason: CIRefReasonNameInvalid}, true},
		{"unknown with a name", CIRefFact{Name: "main", Reason: CIRefReasonRemoteTrackingNotHead}, true},
		{"unknown with a provider", CIRefFact{Provider: CIProviderGitHub, Reason: CIRefReasonRemoteTrackingNotHead}, true},
		{"unknown with an unclosed reason", CIRefFact{Reason: "flaky-runner"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate(%+v): want error", tt.f)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(%+v): %v", tt.f, err)
			}
		})
	}
}

// TestCIRefReasons_ClosedAndDistinct proves every exported reason code is
// in the closed set Validate admits, and that no two failure causes share
// a code (SI-257: "every failure is Known: false with its own closed Reason
// code").
func TestCIRefReasons_ClosedAndDistinct(t *testing.T) {
	codes := []string{
		CIRefReasonProvidersAmbiguous,
		CIRefReasonRefTypeNotBranch,
		CIRefReasonTagPipeline,
		CIRefReasonNameMissing,
		CIRefReasonBranchMismatch,
		CIRefReasonNameInvalid,
		CIRefReasonHeadUnresolved,
		CIRefReasonRemoteTrackingUnresolved,
		CIRefReasonRemoteTrackingNotHead,
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if code == "" || seen[code] {
			t.Fatalf("reason code %q is empty or duplicated", code)
		}
		seen[code] = true
		if err := (CIRefFact{Reason: code}).Validate(); err != nil {
			t.Fatalf("closed reason %q rejected: %v", code, err)
		}
	}
	if len(seen) != len(validCIRefReason) {
		t.Fatalf("closed reason set has %d members, this test names %d", len(validCIRefReason), len(seen))
	}
}

// invalidBranchNameCases holds one name per git check-ref-format rule a
// branch name can break (git-check-ref-format(1), with --branch's own
// additions), each violating exactly that rule. TestGather_CIRef reuses it
// to prove none of these names ever reaches rev-parse.
var invalidBranchNameCases = []struct{ rule, name string }{
	{"empty", ""},
	{"tilde", "main~1"},
	{"caret", "main^"},
	{"colon", "a:b"},
	{"question mark", "a?b"},
	{"asterisk", "a*b"},
	{"open bracket", "a[b"},
	{"backslash", `a\b`},
	{"space", "a b"},
	{"control character", "a\tb"},
	{"DEL", "a\x7fb"},
	{"double dot", "a..b"},
	{"at-brace", "a@{b"},
	{"leading dash", "-main"},
	{"leading slash", "/main"},
	{"trailing slash", "main/"},
	{"trailing dot", "main."},
	{"trailing .lock", "main.lock"},
	{"empty component", "a//b"},
	{"single at", "@"},
	{"component beginning with a dot", "a/.b"},
	{"leading dot", ".main"},
	{"component ending with .lock", "a.lock/b"},
	{"HEAD", "HEAD"},
}

// TestValidBranchName pins git's check-ref-format rules for a branch name
// (git-check-ref-format(1), with --branch's own additions): one invalid row
// per rule, each violating exactly that rule, plus the ordinary shapes a
// CI provider names.
func TestValidBranchName(t *testing.T) {
	valid := []string{
		"main",
		"close/spec-x",
		"feature/a/b",
		"release-1.2",
		"a.b",
		"a@b",
		"a{b}",
		"ü-branch",
		"123/merge",
	}
	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if !validBranchName(name) {
				t.Fatalf("validBranchName(%q) = false, want true", name)
			}
		})
	}
	for _, tt := range invalidBranchNameCases {
		t.Run("invalid/"+tt.rule, func(t *testing.T) {
			if validBranchName(tt.name) {
				t.Fatalf("validBranchName(%q) = true, want false (%s)", tt.name, tt.rule)
			}
		})
	}
}

// validFactsDetached is validFacts with the current branch unknown because
// HEAD is detached — the CI close checkout's physical state.
func validFactsDetached() Facts {
	f := validFacts()
	f.Branch = StringFact{}
	return f
}

func TestSnapshot_BranchBeingClosed(t *testing.T) {
	knownCIRef := CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "close/spec-x"}
	unprovenCIRef := CIRefFact{Reason: CIRefReasonRemoteTrackingNotHead}
	tests := []struct {
		name string
		snap Snapshot
		want StringFact
	}{
		{
			name: "checked-out branch wins over a known CI ref",
			snap: Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{}, CIRef: knownCIRef},
			want: StringFact{Known: true, Value: "main"},
		},
		{
			name: "checked-out branch, outside CI",
			snap: Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{}},
			want: StringFact{Known: true, Value: "main"},
		},
		{
			name: "detached with a known CI ref",
			snap: Snapshot{Facts: validFactsDetached(), Disclosures: []DisclosureCode{DisclosureBranchDetached}, CIRef: knownCIRef},
			want: StringFact{Known: true, Value: "close/spec-x"},
		},
		{
			name: "detached with an unknown CI ref",
			snap: Snapshot{Facts: validFactsDetached(), Disclosures: []DisclosureCode{DisclosureBranchDetached}, CIRef: unprovenCIRef},
			want: StringFact{},
		},
		{
			name: "detached outside CI",
			snap: Snapshot{Facts: validFactsDetached(), Disclosures: []DisclosureCode{DisclosureBranchDetached}},
			want: StringFact{},
		},
		{
			name: "branch unresolved (not detached) with a known CI ref",
			snap: Snapshot{Facts: validFactsDetached(), Disclosures: []DisclosureCode{DisclosureBranchUnresolved}, CIRef: knownCIRef},
			want: StringFact{},
		},
		{
			name: "detached with a malformed known CI ref",
			snap: Snapshot{Facts: validFactsDetached(), Disclosures: []DisclosureCode{DisclosureBranchDetached}, CIRef: CIRefFact{Known: true, Provider: CIProviderGitHub, Name: "main~1"}},
			want: StringFact{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.snap.BranchBeingClosed()
			if got != tt.want {
				t.Fatalf("BranchBeingClosed() = %+v, want %+v", got, tt.want)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("BranchBeingClosed() returned an inconsistent fact: %v", err)
			}
		})
	}
}

func TestSnapshot_ValidateCoversCIRef(t *testing.T) {
	good := Snapshot{Facts: validFactsDetached(), Disclosures: []DisclosureCode{DisclosureBranchDetached}, CIRef: CIRefFact{Known: true, Provider: CIProviderGitLab, Name: "main"}}
	if err := good.Validate(); err != nil {
		t.Fatalf("Validate(): %v", err)
	}
	bad := good
	bad.CIRef = CIRefFact{Known: true, Provider: CIProviderGitLab}
	err := bad.Validate()
	if err == nil {
		t.Fatal("Validate(): want error for an inconsistent CI ref")
	}
	if !strings.Contains(err.Error(), "ci_ref") {
		t.Fatalf("Validate() error = %q, want it to name ci_ref", err)
	}
}
