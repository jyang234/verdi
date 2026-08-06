package policyauthority

import (
	"strings"
	"testing"
)

// loadAndResolve writes files to a fresh temp dir and runs Load then
// Resolve, returning the first error either stage produces (or nil if
// both succeed). Tests in this file assert on that single combined
// outcome because several of DC-3's boundary checks are legitimately
// split between Load's structural cross-validation and Resolve's
// narrow-only semantic rules (see store.go's checkOperandKind doc).
func loadAndResolve(t *testing.T, files map[string]string) (*EffectivePolicy, error) {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, files)
	s, err := Load(root)
	if err != nil {
		return nil, err
	}
	return Resolve(s)
}

func withRulesOverlay(overlay string) map[string]string {
	files := rulesStoreFiles()
	files[".verdi/policy/overlays/o.md"] = overlay
	return files
}

func TestResolve_NarrowOnlyRefinement_Matrix(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		wantErr   bool
		errSubstr string
	}{
		{
			name:  "allowed-values valid subset narrows",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "allowed-region", `values: ["us-east"]`)),
		},
		{
			name:      "allowed-values widening rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "allowed-region", `values: ["us-east", "ap-south"]`)),
			wantErr:   true,
			errSubstr: "not a subset",
		},
		{
			name: "allowed-values empty intersection across two overlays",
			files: func() map[string]string {
				files := rulesStoreFiles()
				files[".verdi/policy/overlays/o1.md"] = overlayFile("o1", `"web/"`, "allowed-region", `values: ["us-east"]`)
				files[".verdi/policy/overlays/o2.md"] = overlayFile("o2", `"web/"`, "allowed-region", `values: ["us-west"]`)
				return files
			}(),
			wantErr:   true,
			errSubstr: "empty intersection",
		},
		{
			name:  "required-values valid superset unions",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "env-required", `values: ["x", "y"]`)),
		},
		{
			name:      "required-values dropping a base member rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "env-required", `values: ["y"]`)),
			wantErr:   true,
			errSubstr: "drops a base value",
		},
		{
			name:  "forbidden-values valid superset unions",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "env-forbidden", `values: ["z", "y"]`)),
		},
		{
			name:      "forbidden-values dropping a base member rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "env-forbidden", `values: ["y"]`)),
			wantErr:   true,
			errSubstr: "drops a base value",
		},
		{
			name:  "minimum raising the bound accepted",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "coverage-min", `bound: 80`)),
		},
		{
			name:      "minimum with a lower bound rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "coverage-min", `bound: 50`)),
			wantErr:   true,
			errSubstr: "must be >= the base bound",
		},
		{
			name:  "maximum lowering the bound accepted",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "coverage-max", `bound: 85`)),
		},
		{
			name:      "maximum with a higher bound rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "coverage-max", `bound: 95`)),
			wantErr:   true,
			errSubstr: "must be <= the base bound",
		},
		{
			name:      "equals is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "exact-version", `values: ["1.24"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "not-equals is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "not-legacy", `values: ["ancient"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "same-principal accepts no refinement operand",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "same-owner", `values: ["x"]`)),
			wantErr:   true,
			errSubstr: "accepts no refinement operand",
		},
		{
			name:      "different-principal accepts no refinement operand",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "diff-owner", `values: ["x"]`)),
			wantErr:   true,
			errSubstr: "accepts no refinement operand",
		},
		{
			name:      "path-read is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "readable-paths", `values: ["docs/sub/"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "path-write is not refinable",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "writable-paths", `values: ["scripts/sub/"]`)),
			wantErr:   true,
			errSubstr: "not refinable",
		},
		{
			name:      "refinement of a non-overridable claim rejected",
			files:     withRulesOverlay(overlayFile("o", `"web/"`, "locked-region", `values: ["us-east"]`)),
			wantErr:   true,
			errSubstr: "not overridable",
		},
		{
			name:  "overlay scope subset of nonempty claim scope accepted",
			files: withRulesOverlay(overlayFile("o", `"web/"`, "scoped-region", `values: ["us-east"]`)),
		},
		{
			name:      "overlay universal scope against nonempty claim scope rejected",
			files:     withRulesOverlay(overlayFile("o", ``, "scoped-region", `values: ["us-east"]`)),
			wantErr:   true,
			errSubstr: "universal",
		},
		{
			name:      "overlay scope spelling mismatch is not a provable subset",
			files:     withRulesOverlay(overlayFile("o", `"Web/"`, "scoped-region", `values: ["us-east"]`)),
			wantErr:   true,
			errSubstr: "not a provable subset",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadAndResolve(t, tc.files)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
