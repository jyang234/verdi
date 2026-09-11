package sealedexec

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const consolidationATCBoundPath = "internal/verdiproto/controller.go"
const consolidationATCBeforeBound = "9e54ff966badbde0492aed1c2b111b69e3a8ae9e7d7781dda856cdc6cb879b56"
const consolidationATCAfterBound = "a4ce965b65bee131cfc010e9cccfc2c10b9efaa44a5974f7cbc36820d1ad2ecb"
const consolidationATCBoundGuard = "\tif len(frame) >= MaxControllerFrameBytes {\n\t\treturn c.refuse(sequence, operation, ErrorInternal, \"the reply reaches the frame bound\")\n\t}\n"

func TestPublicControllerConsolidationBoundCorrection(t *testing.T) {
	// Ordinary exact bindings and unknown changes need no private peer source.
	t.Run("pure", func(t *testing.T) {
		for _, tc := range []struct {
			name, path, want string
			source           []byte
			valid            bool
		}{
			{"unchanged-source", "other.go", consolidationDigest([]byte("unchanged")), []byte("unchanged"), true},
			{"unknown-source-change", "other.go", consolidationDigest([]byte("unchanged")), []byte("changed"), false},
			{"unknown-controller-bytes", consolidationATCBoundPath, consolidationATCBeforeBound, []byte(consolidationATCBoundGuard), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := consolidationATCSourceMatches(tc.path, tc.want, tc.source); got != tc.valid {
					t.Fatalf("source admission=%v want=%v", got, tc.valid)
				}
			})
		}
	})
	t.Run("peer", func(t *testing.T) {
		root := os.Getenv("VERDI_CONSOLIDATION_ATC_SOURCE")
		if root == "" {
			if os.Getenv("VERDI_CONSOLIDATION_REQUIRE_ATC") == "1" {
				t.Fatal("required correction sensitivity needs actual ATC source")
			}
			t.Skip("UNPROVEN: correction sensitivity requires VERDI_CONSOLIDATION_ATC_SOURCE; private ATC source is not a committed fixture")
		}
		current, err := os.ReadFile(filepath.Join(root, consolidationATCBoundPath))
		if err != nil {
			t.Fatal(err)
		}
		guard := []byte(consolidationATCBoundGuard)
		if consolidationDigest(current) != consolidationATCAfterBound || bytes.Count(current, guard) != 1 {
			t.Fatal("peer is not the exact bounded controller source")
		}
		historical := bytes.Replace(current, guard, nil, 1)
		if consolidationDigest(historical) != consolidationATCBeforeBound {
			t.Fatal("removing the sole bound guard did not reproduce historical source")
		}
		for _, tc := range []struct {
			name, path, want string
			source           []byte
			valid            bool
		}{
			{"historical", consolidationATCBoundPath, consolidationATCBeforeBound, historical, true},
			{"corrected", consolidationATCBoundPath, consolidationATCBeforeBound, current, true},
			{"wrong-path", "internal/verdiproto/other.go", consolidationATCBeforeBound, current, false},
			{"wrong-historical-hash", consolidationATCBoundPath, strings.Repeat("0", 64), current, false},
			{"changed-bound", consolidationATCBoundPath, consolidationATCBeforeBound, bytes.Replace(current, []byte("len(frame) >= MaxControllerFrameBytes"), []byte("len(frame) > MaxControllerFrameBytes"), 1), false},
			{"repeated-guard", consolidationATCBoundPath, consolidationATCBeforeBound, bytes.Replace(current, guard, append(append([]byte{}, guard...), guard...), 1), false},
			{"removed-guard-and-mutated-source", consolidationATCBoundPath, consolidationATCBeforeBound, append(append([]byte{}, historical...), '\n'), false},
			{"unknown-mutation", consolidationATCBoundPath, consolidationATCBeforeBound, append(append([]byte{}, current...), '\n'), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := consolidationATCSourceMatches(tc.path, tc.want, tc.source); got != tc.valid {
					t.Fatalf("source admission=%v want=%v", got, tc.valid)
				}
			})
		}
		for _, tc := range []struct {
			name   string
			source []byte
			reject bool
		}{
			{"paired-current", current, false},
			{"paired-historical", historical, false},
			{"paired-unknown-mutation", append(append([]byte{}, current...), '\n'), true},
		} {
			t.Run(tc.name, func(t *testing.T) { consolidationPairedBoundWitness(t, root, tc.source, tc.reject) })
		}

	})
}

// Run the real witness checker against private copies of its declared source
// operands. No private ATC source is added to Verdi's committed fixtures.
func consolidationPairedBoundWitness(t *testing.T, root string, controller []byte, reject bool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "public-consolidation", "checks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var w consolidationWitness
	if err = json.Unmarshal(raw, &w); err != nil {
		t.Fatal(err)
	}
	var peer struct {
		Source map[string]string `json:"source_sha256"`
		Tests  map[string]string `json:"test_source_sha256"`
	}
	if err = json.Unmarshal(w.ATC, &peer); err != nil {
		t.Fatal(err)
	}
	copyRoot := t.TempDir()
	for _, files := range []map[string]string{peer.Source, peer.Tests} {
		for name := range files {
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			if name == consolidationATCBoundPath {
				data = controller
			}
			path := filepath.Join(copyRoot, name)
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPublicControllerConsolidationWitness$", "-test.timeout=1m")
	cmd.Env = append(os.Environ(), "VERDI_CONSOLIDATION_REQUIRE_ATC=1", "VERDI_CONSOLIDATION_ATC_SOURCE="+copyRoot)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("paired sensitivity timed out: %v", ctx.Err())
	}
	if !reject && err != nil {
		t.Fatalf("exact paired witness refused: %v\n%s", err, output)
	}
	if reject && (err == nil || !bytes.Contains(output, []byte("stale ATC witness source "+consolidationATCBoundPath))) {
		t.Fatalf("unknown mutation was not refused at source binding: %v\n%s", err, output)
	}
}
