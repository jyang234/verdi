package policyauthority

import (
	"strings"
	"testing"
)

// TestResolve_MapKeyIdentityMismatch_FailsClosed pins the store's
// key-to-value binding. Store's exported maps are ordinary Go maps of
// pointers to INDIVIDUALLY sealed artifacts: rekeying an entry, or
// pointing two keys at one value, disturbs no artifact's own seal, so
// the mutation is invisible to every per-artifact check. Resolve selects
// artifacts BY KEY (the selected profile, an overlay's refines target,
// an exemption's witness policy), so an unbound key means Resolve emits
// the wrong artifact under the selected name — a wrong effective policy
// with every seal intact. crossValidate must therefore prove key ==
// value identity for all four maps (DC-19/DC-20, CO-1, SI-21's
// post-load-mutation posture: a mutated store is an operational error,
// never a silently different resolution).
func TestResolve_MapKeyIdentityMismatch_FailsClosed(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*Store)
		wantSubs []string
	}{
		{
			"policy under a foreign key",
			func(s *Store) { s.Policies["policy/impostor"] = s.Policies["policy/go-toolchain"] },
			[]string{"Policies", "policy/impostor", "policy/go-toolchain"},
		},
		{
			"overlay under a foreign key",
			func(s *Store) {
				s.Overlays["policy-overlay/impostor"] = s.Overlays["policy-overlay/frontend-go-version"]
			},
			[]string{"Overlays", "policy-overlay/impostor", "policy-overlay/frontend-go-version"},
		},
		{
			"exemption under a foreign key",
			func(s *Store) {
				s.Exemptions["policy-exemption/impostor"] = s.Exemptions["policy-exemption/legacy-service-go"]
			},
			[]string{"Exemptions", "policy-exemption/impostor", "policy-exemption/legacy-service-go"},
		},
		{
			"profile under a foreign key",
			func(s *Store) { s.Profiles["impostor"] = s.Profiles["solo-default"] },
			[]string{"Profiles", "impostor", "solo-default"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeTree(t, root, minimalStoreFiles())
			s, err := Load(root)
			if err != nil {
				t.Fatalf("Load(): %v", err)
			}
			if _, err := Resolve(s); err != nil {
				t.Fatalf("baseline Resolve(): %v", err)
			}

			tt.mutate(s)

			_, err = Resolve(s)
			if err == nil {
				t.Fatal("Resolve() = nil error after a map key was unbound from its artifact's identity; want a fail-closed error")
			}
			for _, sub := range tt.wantSubs {
				if !strings.Contains(err.Error(), sub) {
					t.Fatalf("Resolve() error = %v, want it to name %q", err, sub)
				}
			}
		})
	}
}

// TestResolve_SwappedMapValues_FailsClosed is the sharper arm: two keys
// that both exist are SWAPPED, so every key still names a real artifact
// and every artifact still carries a valid seal — only the association
// between name and content is a lie. Without the key-identity assertion
// Resolve would happily emit each policy's content under the other
// policy's id.
func TestResolve_SwappedMapValues_FailsClosed(t *testing.T) {
	root := t.TempDir()
	files := minimalStoreFiles()
	files[".verdi/policy/policies/build-gate.md"] = strings.NewReplacer(
		"id: policy/go-toolchain", "id: policy/build-gate",
		"title: \"Go toolchain policy\"", "title: \"Build gate policy\"",
	).Replace(files[".verdi/policy/policies/go-toolchain.md"])
	// The second policy must not re-register the first's claim subjects
	// under a duplicate id; strip its claims entirely.
	files[".verdi/policy/policies/build-gate.md"] = strings.Replace(
		files[".verdi/policy/policies/build-gate.md"],
		`claims:
  - id: go-version
    family: configuration
    operator: allowed-values
    subject: go-version
    values: ["1.25", "1.24"]
    scope: {phases: [], environments: [], paths: [], refs: []}
    overridable: true
  - id: verify-required
    family: action
    operator: required-values
    subject: make-verify
    values: [clean-exit]
    scope: {phases: [build], environments: [], paths: [], refs: []}
    overridable: false
`, "claims: []\n", 1)
	writeTree(t, root, files)

	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if _, err := Resolve(s); err != nil {
		t.Fatalf("baseline Resolve(): %v", err)
	}

	s.Policies["policy/go-toolchain"], s.Policies["policy/build-gate"] =
		s.Policies["policy/build-gate"], s.Policies["policy/go-toolchain"]

	if _, err := Resolve(s); err == nil {
		t.Fatal("Resolve() = nil error after two policy map entries were swapped; want a fail-closed error naming the mismatch")
	} else if !strings.Contains(err.Error(), "Policies") {
		t.Fatalf("Resolve() error = %v, want it to name the Policies map", err)
	}
}
