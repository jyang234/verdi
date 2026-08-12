package policyartifact

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

var updateGolden = flag.Bool("update", false, "regenerate testdata golden digests")

// TestFixtureDigests_Ratchet proves the committed fixture store decodes
// clean and produces byte-stable canonical digests (co-6's digest
// ratchet): any change to fixture content, normalization, or canonical
// encoding shows up as an explicit golden diff, never silent drift.
func TestFixtureDigests_Ratchet(t *testing.T) {
	got := map[string]string{}

	walk := func(dir string, digest func(data []byte) (string, error)) {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join("testdata", "store", dir))
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		for _, e := range entries {
			rel := dir + "/" + e.Name()
			data, err := os.ReadFile(filepath.Join("testdata", "store", dir, e.Name()))
			if err != nil {
				t.Fatalf("reading %s: %v", rel, err)
			}
			d, err := digest(data)
			if err != nil {
				t.Fatalf("digesting %s: %v", rel, err)
			}
			got[rel] = d
		}
	}

	conData, err := os.ReadFile(filepath.Join("testdata", "store", "constitution.md"))
	if err != nil {
		t.Fatalf("reading constitution: %v", err)
	}
	con, err := DecodeConstitution(conData)
	if err != nil {
		t.Fatalf("decoding constitution: %v", err)
	}
	conDigest, err := con.Digest()
	if err != nil {
		t.Fatalf("constitution digest: %v", err)
	}
	got["constitution.md"] = conDigest

	walk(DirPolicies, func(data []byte) (string, error) {
		p, err := DecodePolicy(data)
		if err != nil {
			return "", err
		}
		return p.Digest()
	})
	walk(DirOverlays, func(data []byte) (string, error) {
		o, err := DecodeOverlay(data)
		if err != nil {
			return "", err
		}
		return o.Digest()
	})
	walk(DirExemptions, func(data []byte) (string, error) {
		e, err := DecodeExemption(data)
		if err != nil {
			return "", err
		}
		return e.Digest()
	})
	walk(DirDispositions, func(data []byte) (string, error) {
		d, err := DecodeDisposition(data)
		if err != nil {
			return "", err
		}
		return d.Digest()
	})
	govCat, err := con.GovernanceCatalog()
	if err != nil {
		t.Fatalf("GovernanceCatalog: %v", err)
	}
	walk(DirProfiles, func(data []byte) (string, error) {
		sp, err := DecodeStoredProfile(data, govCat)
		if err != nil {
			return "", err
		}
		return sp.ProfileDigest, nil
	})

	goldenPath := filepath.Join("testdata", "golden-digests.json")
	if *updateGolden {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make(map[string]string, len(got))
		for _, k := range keys {
			ordered[k] = got[k]
		}
		data, err := json.MarshalIndent(ordered, "", "  ")
		if err != nil {
			t.Fatalf("marshaling golden: %v", err)
		}
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
	}

	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run with -update to regenerate): %v", err)
	}
	var want map[string]string
	if err := json.Unmarshal(goldenData, &want); err != nil {
		t.Fatalf("decoding golden: %v", err)
	}
	if len(want) != len(got) {
		t.Fatalf("golden has %d entries, fixtures produced %d", len(want), len(got))
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("fixture %s digest = %s, golden %s", k, got[k], w)
		}
	}
}

// TestFixtureExemptionWitnessDigest proves the committed exemption
// fixture's claim_digest is the REAL canonical digest of the governing
// policy's claim — the exact-witness discipline (DC-8), pinned in the
// fixture rather than a synthetic value.
func TestFixtureExemptionWitnessDigest(t *testing.T) {
	polData, err := os.ReadFile(filepath.Join("testdata", "store", DirPolicies, "go-toolchain.md"))
	if err != nil {
		t.Fatalf("reading policy: %v", err)
	}
	pol, err := DecodePolicy(polData)
	if err != nil {
		t.Fatalf("decoding policy: %v", err)
	}
	claim, ok := pol.Claim("go-version")
	if !ok {
		t.Fatal("policy has no go-version claim")
	}
	claimDigest, err := ClaimDigest(claim)
	if err != nil {
		t.Fatalf("ClaimDigest: %v", err)
	}

	exData, err := os.ReadFile(filepath.Join("testdata", "store", DirExemptions, "legacy-service-go.md"))
	if err != nil {
		t.Fatalf("reading exemption: %v", err)
	}
	ex, err := DecodeExemption(exData)
	if err != nil {
		t.Fatalf("decoding exemption: %v", err)
	}
	if got := ex.Witnesses[0].ClaimDigest; got != claimDigest {
		t.Fatalf("fixture witness claim_digest = %s, real claim digest = %s (update the fixture)", got, claimDigest)
	}
}
