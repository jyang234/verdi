package execworkspace

import (
	"path/filepath"
	"testing"
)

func TestExecutionRoot(t *testing.T) {
	got := ExecutionRoot("/store")
	want := filepath.Join("/store", ".verdi", "data", "execution")
	if got != want {
		t.Fatalf("ExecutionRoot(/store) = %q, want %q", got, want)
	}
}

func TestUnitPath(t *testing.T) {
	got := UnitPath("/store", "run--abc123456789")
	want := filepath.Join(ExecutionRoot("/store"), "run--abc123456789")
	if got != want {
		t.Fatalf("UnitPath = %q, want %q", got, want)
	}
}

func TestRequestPath(t *testing.T) {
	got := RequestPath("/store", "wid")
	want := filepath.Join(ExecutionRoot("/store"), "wid.request")
	if got != want {
		t.Fatalf("RequestPath = %q, want %q", got, want)
	}
}

func TestRequestStagingPath(t *testing.T) {
	got := RequestStagingPath("/store", "wid")
	want := filepath.Join(ExecutionRoot("/store"), "wid.request.staging")
	if got != want {
		t.Fatalf("RequestStagingPath = %q, want %q", got, want)
	}
}

func TestReleasedPath(t *testing.T) {
	got := ReleasedPath("/store", "wid")
	want := filepath.Join(ExecutionRoot("/store"), "wid.released")
	if got != want {
		t.Fatalf("ReleasedPath = %q, want %q", got, want)
	}
}

func TestLockPath(t *testing.T) {
	got := LockPath("/store", "wid")
	want := filepath.Join(ExecutionRoot("/store"), "wid.lock")
	if got != want {
		t.Fatalf("LockPath = %q, want %q", got, want)
	}
}

func TestClassifyEntry_UnitDir(t *testing.T) {
	ce, ok := ClassifyEntry("feature--my-run--abcdef012345")
	if !ok {
		t.Fatalf("ClassifyEntry: want classified, got unclassified")
	}
	if ce.Form != FormUnit {
		t.Fatalf("Form = %v, want FormUnit", ce.Form)
	}
	if ce.WorkspaceID != "feature--my-run--abcdef012345" {
		t.Fatalf("WorkspaceID = %q, want full name", ce.WorkspaceID)
	}
}

func TestClassifyEntry_Siblings(t *testing.T) {
	cases := []struct {
		name string
		want EntryForm
	}{
		{"wid.request", FormRequest},
		{"wid.request.staging", FormRequestStaging},
		{"wid.released", FormReleased},
		{"wid.lock", FormLock},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ce, ok := ClassifyEntry(tc.name)
			if !ok {
				t.Fatalf("ClassifyEntry(%q): want classified, got unclassified", tc.name)
			}
			if ce.Form != tc.want {
				t.Fatalf("ClassifyEntry(%q).Form = %v, want %v", tc.name, ce.Form, tc.want)
			}
			if ce.WorkspaceID != "wid" {
				t.Fatalf("ClassifyEntry(%q).WorkspaceID = %q, want %q", tc.name, ce.WorkspaceID, "wid")
			}
		})
	}
}

func TestClassifyEntry_BareRequestIsGrammarExternal(t *testing.T) {
	// A bare ".request" has an empty base id — not a plausible unit id.
	if _, ok := ClassifyEntry(".request"); ok {
		t.Fatalf("ClassifyEntry(.request): want grammar-external, got classified")
	}
}

func TestClassifyEntry_BareSiblingSuffixesAreGrammarExternal(t *testing.T) {
	for _, name := range []string{".request.staging", ".released", ".lock"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := ClassifyEntry(name); ok {
				t.Fatalf("ClassifyEntry(%q): want grammar-external, got classified", name)
			}
		})
	}
}

func TestClassifyEntry_EmptyNameIsGrammarExternal(t *testing.T) {
	if _, ok := ClassifyEntry(""); ok {
		t.Fatalf("ClassifyEntry(\"\"): want grammar-external, got classified")
	}
}

func TestClassifyEntry_NestedPathIsGrammarExternal(t *testing.T) {
	for _, name := range []string{"nested/path", "wid/nested.request", "/absolute", "a\\b"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := ClassifyEntry(name); ok {
				t.Fatalf("ClassifyEntry(%q): want grammar-external (path separator present), got classified", name)
			}
		})
	}
}

func TestClassifyEntry_NestedPathWithSiblingSuffixIsGrammarExternal(t *testing.T) {
	if _, ok := ClassifyEntry("nested/wid.request"); ok {
		t.Fatalf("ClassifyEntry(nested/wid.request): want grammar-external, got classified")
	}
}

func TestClassifyEntry_NeverPanics(t *testing.T) {
	inputs := []string{"", ".", "..", ".request", ".released", ".lock", ".request.staging", "/", "\\", "a.request.staging.request"}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ClassifyEntry(%q) panicked: %v", in, r)
				}
			}()
			ClassifyEntry(in)
		}()
	}
}

func TestClassifyEntry_SuffixLookingUnitNameStillClassifiesAsUnit(t *testing.T) {
	// A unit id that itself embeds ".request" as a substring but not as its
	// trailing suffix form must still classify as a plain unit dir name,
	// since it doesn't end in one of the four sibling suffixes.
	ce, ok := ClassifyEntry("run.requestish--abc123456789")
	if !ok {
		t.Fatalf("ClassifyEntry: want classified, got unclassified")
	}
	if ce.Form != FormUnit {
		t.Fatalf("Form = %v, want FormUnit", ce.Form)
	}
}
