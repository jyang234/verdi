package experimentapp

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

type recordingCapabilityDiscoverer struct {
	bytes    []byte
	err      error
	calls    int
	requests []CapabilityRequest
}

func (d *recordingCapabilityDiscoverer) DiscoverCapabilities(_ context.Context, request CapabilityRequest) (CapabilityDiscovery, error) {
	d.calls++
	d.requests = append(d.requests, request)
	return CapabilityDiscovery{Bytes: d.bytes}, d.err
}

func TestDiscoverCapabilitiesReturnsCanonicalTypedResultWithoutWriting(t *testing.T) {
	root := t.TempDir()
	copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
	before := snapshotWorktree(t, root)
	capabilitiesBytes, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	discoverer := &recordingCapabilityDiscoverer{bytes: capabilitiesBytes}
	policy := &fakePolicyResolver{}
	git := &fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: testHead}}
	service, err := NewService(policy, git, discoverer, &acceptingVerifier{})
	if err != nil {
		t.Fatal(err)
	}
	identity := testIdentity(t, root, "request-path-v2")

	result := service.DiscoverCapabilities(context.Background(), identity)

	if result.Outcome.Classification != ClassificationClean || result.Outcome.ExitCode() != 0 {
		t.Fatalf("DiscoverCapabilities() outcome = %+v", result.Outcome)
	}
	wantPath := ".verdi/specs/active/request-path-spike/experiments/request-path-v2"
	if result.AcceptedHead != testHead || result.ExperimentPath != wantPath {
		t.Fatalf("DiscoverCapabilities() identity = %q %q, want %q %q", result.AcceptedHead, result.ExperimentPath, testHead, wantPath)
	}
	wantCapabilities := experiment.Capabilities{
		Schema:           experiment.CapabilitiesSchemaV2,
		EvaluatorVersion: "fixture/1.0.0",
		ProtocolVersions: []string{experiment.EvaluatorProtocolSchema, experiment.ObservationSchemaV2},
		Metrics: []experiment.CapabilityMetric{{
			ID: "request-latency", Type: experiment.MetricDuration, Unit: "ms", Direction: experiment.DirectionLower,
		}},
		Environment:      []string{"GOMAXPROCS"},
		RequiresNetwork:  false,
		RequiresElevated: false,
	}
	if !reflect.DeepEqual(result.Capabilities, wantCapabilities) {
		t.Fatalf("DiscoverCapabilities() capabilities = %#v, want %#v", result.Capabilities, wantCapabilities)
	}
	if !bytes.Equal(result.CapabilitiesBytes, capabilitiesBytes) {
		t.Fatalf("DiscoverCapabilities() bytes = %q, want %q", result.CapabilitiesBytes, capabilitiesBytes)
	}
	const wantDigest = "sha256:812cd93a95c48eef49c81b2c8712e0d6b100baa585bb398aad7ad89b720afa15"
	if result.CapabilitiesDigest != wantDigest {
		t.Fatalf("DiscoverCapabilities() digest = %q, want %q", result.CapabilitiesDigest, wantDigest)
	}
	if discoverer.calls != 1 || len(discoverer.requests) != 1 || git.headCalls != 1 || len(git.treeCalls) != 0 || policy.calls != 0 {
		t.Fatalf("calls discovery=%d requests=%d head=%d tree=%v policy=%d", discoverer.calls, len(discoverer.requests), git.headCalls, git.treeCalls, policy.calls)
	}
	request := discoverer.requests[0]
	if request.CheckoutRoot != root || request.Definition.ID != identity.ExperimentID || request.Definition.Spike != identity.Spike {
		t.Fatalf("discovery request = %+v", request)
	}
	if after := snapshotWorktree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("DiscoverCapabilities() wrote worktree\nbefore=%#v\nafter=%#v", before, after)
	}

	discoverer.bytes[0] = '['
	if result.CapabilitiesBytes[0] != '{' {
		t.Fatalf("DiscoverCapabilities() returned bytes alias the discoverer response")
	}
}

func TestDiscoverCapabilitiesClassifiesHeadDefinitionAndResponseFailures(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("testdata", "capabilities.json"))
	if err != nil {
		t.Fatal(err)
	}
	noncanonical := bytes.Replace(canonical, []byte(`{"environment"`), []byte("{\n\"environment\""), 1)
	mismatch := bytes.Replace(canonical, []byte("fixture/1.0.0"), []byte("fixture/1.0.1"), 1)

	tests := []struct {
		name             string
		head             string
		gitErr           error
		fixture          bool
		definitionChange func([]byte) []byte
		response         []byte
		discoverErr      error
		want             Classification
		code             string
		exit             int
		wantCalls        int
	}{
		{name: "stale accepted HEAD is a verdict", head: oldHead, fixture: true, response: canonical, want: ClassificationVerdict, code: "accepted-head-stale", exit: 1},
		{name: "unavailable accepted HEAD is operational", gitErr: errors.New("git unavailable"), fixture: true, response: canonical, want: ClassificationOperational, code: "accepted-head-invalid", exit: 2},
		{name: "malformed accepted HEAD is operational", head: "not-a-commit", fixture: true, response: canonical, want: ClassificationOperational, code: "accepted-head-invalid", exit: 2},
		{name: "missing definition is operational", head: testHead, response: canonical, want: ClassificationOperational, code: "proposal-location-invalid", exit: 2},
		{
			name: "invalid definition is operational", head: testHead, fixture: true, response: canonical,
			definitionChange: func(data []byte) []byte { return append(data, []byte("unknown_field: true\n")...) },
			want:             ClassificationOperational, code: "definition-invalid", exit: 2,
		},
		{name: "unavailable discovery is operational", head: testHead, fixture: true, discoverErr: errors.New("describe failed"), want: ClassificationOperational, code: "capability-discovery-failed", exit: 2, wantCalls: 1},
		{name: "invalid response is operational", head: testHead, fixture: true, response: []byte(`{"schema":"unknown"}`), want: ClassificationOperational, code: "capabilities-invalid", exit: 2, wantCalls: 1},
		{
			name: "noncanonical response is operational", head: testHead, fixture: true, response: noncanonical,
			definitionChange: capabilitiesDigestReplacement(noncanonical),
			want:             ClassificationOperational, code: "capabilities-invalid", exit: 2, wantCalls: 1,
		},
		{name: "digest-mismatched response is operational", head: testHead, fixture: true, response: mismatch, want: ClassificationOperational, code: "capabilities-digest-mismatch", exit: 2, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.fixture {
				copyExperimentFixture(t, root, "experiment-v2", "request-path-v2")
			}
			if tt.definitionChange != nil {
				definitionPath := filepath.Join(root, ".verdi", "specs", "active", "request-path-spike", "experiments", "request-path-v2", "experiment.yaml")
				definition, readErr := os.ReadFile(definitionPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if writeErr := os.WriteFile(definitionPath, tt.definitionChange(definition), 0o644); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			before := snapshotWorktree(t, root)
			discoverer := &recordingCapabilityDiscoverer{bytes: tt.response, err: tt.discoverErr}
			git := &fakeGit{revision: DefaultBranch{Name: "main", Ref: "refs/remotes/origin/main", Head: tt.head}, err: tt.gitErr}
			service, newErr := NewService(&fakePolicyResolver{}, git, discoverer, &acceptingVerifier{})
			if newErr != nil {
				t.Fatal(newErr)
			}

			result := service.DiscoverCapabilities(context.Background(), testIdentity(t, root, "request-path-v2"))

			if result.Outcome.Classification != tt.want || result.Outcome.Code != tt.code || result.Outcome.ExitCode() != tt.exit {
				t.Fatalf("DiscoverCapabilities() outcome = %+v exit %d, want %s/%s exit %d", result.Outcome, result.Outcome.ExitCode(), tt.want, tt.code, tt.exit)
			}
			if discoverer.calls != tt.wantCalls {
				t.Fatalf("discovery calls = %d, want %d", discoverer.calls, tt.wantCalls)
			}
			if after := snapshotWorktree(t, root); !reflect.DeepEqual(after, before) {
				t.Fatalf("DiscoverCapabilities() failure wrote worktree\nbefore=%#v\nafter=%#v", before, after)
			}
		})
	}
}

func capabilitiesDigestReplacement(capabilities []byte) func([]byte) []byte {
	return func(definition []byte) []byte {
		const oldDigest = "sha256:812cd93a95c48eef49c81b2c8712e0d6b100baa585bb398aad7ad89b720afa15"
		return []byte(strings.Replace(string(definition), oldDigest, rawDigest(capabilities), 1))
	}
}
