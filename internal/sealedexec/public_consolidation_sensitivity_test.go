package sealedexec

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Run the real witness checker in a child test process so a refusal is observed
// as a failing process without weakening the checker's testing.T assertions.
func TestPublicControllerConsolidationWitnessSensitivity(t *testing.T) {
	const variantEnv = "VERDI_CONSOLIDATION_SENSITIVITY_VARIANT"
	if variant := os.Getenv(variantEnv); variant != "" {
		b, err := os.ReadFile(filepath.Join("testdata", "public-consolidation", "checks.json"))
		if err != nil {
			t.Fatal(err)
		}
		if variant != "valid-control" {
			var w consolidationWitness
			if err := json.Unmarshal(b, &w); err != nil {
				t.Fatal(err)
			}
			switch variant {
			case "nonexistent-descendant", "positive-substitution":
				found := false
				for i := range w.Checks {
					if w.Checks[i].ID != "relation:PersistAbort" {
						continue
					}
					found = true
					w.Checks[i].Mutations[0].Case = "TestContextControllerWireContract_Static/does-not-exist"
					if variant == "positive-substitution" {
						w.Checks[i].Mutations[0].Case = "TestContextControllerWireContract_Static/all_typed_wrappers_carry_literal_requests_and_return_typed_results/PersistAbort"
					}
				}
				if !found {
					t.Fatal("missing sensitivity target")
				}
			case "null-metadata":
				w.Checks[0].Source = json.RawMessage("null")
				w.Checks[0].Mutations[0].NewReturns = []json.RawMessage{json.RawMessage("null")}
				w.Checks[0].ATC = json.RawMessage("null")
			case "invented-id":
				w.Checks[0].ID = "invented-substitute-obligation"
			default:
				t.Fatalf("unknown sensitivity variant %q", variant)
			}
			b, err = json.Marshal(w)
			if err != nil {
				t.Fatal(err)
			}
		}
		consolidationValidateWitness(t, b)
		return
	}
	for _, variant := range []string{"valid-control", "nonexistent-descendant", "positive-substitution", "null-metadata", "invented-id"} {
		t.Run(variant, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPublicControllerConsolidationWitnessSensitivity$", "-test.timeout=1m")
			cmd.Env = append(os.Environ(), variantEnv+"="+variant)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("sensitivity process timed out: %v", ctx.Err())
			}
			if variant == "valid-control" {
				if err != nil {
					t.Fatalf("complete witness refused: %v\n%s", err, output)
				}
			} else {
				if err == nil {
					t.Fatalf("invalid witness accepted: %s", variant)
				}
				if !bytes.Contains(output, []byte("witness metadata does not match the reviewed immutable document")) {
					t.Fatalf("witness refused for an unrelated reason: %v\n%s", err, output)
				}
			}
		})
	}
}
