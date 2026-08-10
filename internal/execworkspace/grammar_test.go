package execworkspace

import (
	"path/filepath"
	"strconv"
	"strings"
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
	const wid = "wid--abcdef012345"
	cases := []struct {
		name string
		want EntryForm
	}{
		{wid + ".request", FormRequest},
		{wid + ".request.staging", FormRequestStaging},
		{wid + ".released", FormReleased},
		{wid + ".lock", FormLock},
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
			if ce.WorkspaceID != wid {
				t.Fatalf("ClassifyEntry(%q).WorkspaceID = %q, want %q", tc.name, ce.WorkspaceID, wid)
			}
		})
	}
}

// TestClassifyEntry_Table asserts the EXACT classification of every listed
// name, including the grammar-external ones a scan must disclose and keep
// (spec §GC slice: "Any entry under data/execution/ matching NO unit grammar
// at all is DISCLOSED AND KEPT"). A name whose id part does not satisfy the
// normative <workspace-id> shape (spec §Workspace naming) is grammar-external
// in EVERY form, unit and sibling alike.
func TestClassifyEntry_Table(t *testing.T) {
	cases := []struct {
		name    string
		wantOK  bool
		wantID  string
		wantFrm EntryForm
	}{
		// Grammar-external: ordinary files a human or a tool dropped in.
		{name: "README", wantOK: false},
		{name: ".DS_Store", wantOK: false},
		{name: "notes.txt", wantOK: false},
		{name: ".", wantOK: false},
		{name: "..", wantOK: false},
		// Grammar-external: id-shaped but not a <workspace-id>.
		{name: "wid", wantOK: false},
		{name: "x--ABCDEF012345", wantOK: false},
		{name: "--abcdef012345", wantOK: false},
		{name: " wid--abcdef012345", wantOK: false},
		// Grammar-external sibling: base "a.request.staging" is not a
		// <workspace-id> (no trailing --<sha12> group).
		{name: "a.request.staging.request", wantOK: false},
		{name: "wid.request", wantOK: false},
		{name: "wid.lock", wantOK: false},
		// Valid: a run slug that itself contains "--" (RefSlug maps "/" to
		// "--"), so the sha group is the LAST "--<sha12>" occurrence.
		{name: "a--b--abcdef012345", wantOK: true, wantID: "a--b--abcdef012345", wantFrm: FormUnit},
		{name: "a--b--abcdef012345.request", wantOK: true, wantID: "a--b--abcdef012345", wantFrm: FormRequest},
		{name: "a--b--abcdef012345.request.staging", wantOK: true, wantID: "a--b--abcdef012345", wantFrm: FormRequestStaging},
		{name: "a--b--abcdef012345.released", wantOK: true, wantID: "a--b--abcdef012345", wantFrm: FormReleased},
		{name: "a--b--abcdef012345.lock", wantOK: true, wantID: "a--b--abcdef012345", wantFrm: FormLock},
		// Valid: base-plus-patch shape.
		{name: "x--abcdef012345-pdeadbeef0123", wantOK: true, wantID: "x--abcdef012345-pdeadbeef0123", wantFrm: FormUnit},
		{name: "x--abcdef012345-pdeadbeef0123.request", wantOK: true, wantID: "x--abcdef012345-pdeadbeef0123", wantFrm: FormRequest},
		{name: "x--abcdef012345-pdeadbeef0123.request.staging", wantOK: true, wantID: "x--abcdef012345-pdeadbeef0123", wantFrm: FormRequestStaging},
		{name: "x--abcdef012345-pdeadbeef0123.released", wantOK: true, wantID: "x--abcdef012345-pdeadbeef0123", wantFrm: FormReleased},
		{name: "x--abcdef012345-pdeadbeef0123.lock", wantOK: true, wantID: "x--abcdef012345-pdeadbeef0123", wantFrm: FormLock},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ce, ok := ClassifyEntry(tc.name)
			if ok != tc.wantOK {
				t.Fatalf("ClassifyEntry(%q) ok = %v, want %v (entry %+v)", tc.name, ok, tc.wantOK, ce)
			}
			if !tc.wantOK {
				if ce != (ClassifiedEntry{}) {
					t.Fatalf("ClassifyEntry(%q) returned %+v alongside ok=false, want zero value", tc.name, ce)
				}
				return
			}
			if ce.WorkspaceID != tc.wantID {
				t.Fatalf("ClassifyEntry(%q).WorkspaceID = %q, want %q", tc.name, ce.WorkspaceID, tc.wantID)
			}
			if ce.Form != tc.wantFrm {
				t.Fatalf("ClassifyEntry(%q).Form = %v, want %v", tc.name, ce.Form, tc.wantFrm)
			}
		})
	}
}

// TestValidWorkspaceID exercises the shape predicate directly, including the
// suffix-parsing rule: RefSlug maps "/" to "--", so a slug may itself contain
// "--" and the sha group is the LAST "--<sha12>" occurrence.
func TestValidWorkspaceID(t *testing.T) {
	valid := []string{
		"x--abcdef012345",
		"a--b--abcdef012345",
		"a--b--c--abcdef012345",
		"run.name--0123456789ab",
		"x--abcdef012345-pdeadbeef0123",
		"a--b--abcdef012345-p0123456789ab",
		"..--abcdef012345",
		"-.--abcdef012345",
		"x--abcdef012345--abcdef012345",
		// The normative alphabet [a-z0-9._-] CONTAINS '_', and RefSlug
		// preserves it, so an underscored run slug is a well-formed id.
		"wid_x--abcdef012345",
		"ci_run--abcdef012345-pdeadbeef0123",
	}
	for _, id := range valid {
		t.Run("valid/"+id, func(t *testing.T) {
			if !ValidWorkspaceID(id) {
				t.Fatalf("ValidWorkspaceID(%q) = false, want true", id)
			}
		})
	}

	invalid := map[string]string{
		"empty":                 "",
		"no sha group":          "wid",
		"empty slug":            "--abcdef012345",
		"uppercase hex":         "x--ABCDEF012345",
		"uppercase slug":        "X--abcdef012345",
		"non-hex group":         "x--abcdefg12345",
		"short hex group":       "x--abcdef01234",
		"long hex group":        "x--abcdef0123456",
		"single dash separator": "x-abcdef012345",
		"space byte":            " wid--abcdef012345",
		"slash byte":            "a/b--abcdef012345",
		"backslash byte":        "a\\b--abcdef012345",
		"patch group only":      "x-pabcdef012345",
		"patch group bad hex":   "x--abcdef012345-pdeadbeefzzzz",
		"patch group short":     "x--abcdef012345-pdeadbeef012",
		"dot":                   ".",
		"dotdot":                "..",
		"readme":                "README",
		"ds store":              ".DS_Store",
		"notes":                 "notes.txt",
		"sibling base":          "a.request.staging",
	}
	for name, id := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			if ValidWorkspaceID(id) {
				t.Fatalf("ValidWorkspaceID(%q) = true, want false", id)
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
			// Every input here is grammar-external, so the classification
			// is asserted too rather than discarded — the panic guard above
			// is the point, the verdict is the free extra.
			if ce, ok := ClassifyEntry(in); ok {
				t.Fatalf("ClassifyEntry(%q) = %+v, want grammar-external", in, ce)
			}
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

// TestEntryForm_String_SelfNamingFallback pins whole-wave finding F6: the
// out-of-set fallback names its own type and value, matching PathKind.String
// and GCOutcome.String, so a diagnostic can never print a bare "unknown"
// that could be mistaken for a real form label.
func TestEntryForm_String_SelfNamingFallback(t *testing.T) {
	for _, f := range []EntryForm{FormUnit, FormRequest, FormRequestStaging, FormReleased, FormLock} {
		if got := f.String(); strings.Contains(got, "EntryForm(") {
			t.Fatalf("EntryForm(%d).String() = %q, want a real label, not the fallback", int(f), got)
		}
	}
	for _, f := range []EntryForm{EntryForm(-1), EntryForm(99)} {
		got := f.String()
		if got == "unknown" {
			t.Fatalf("EntryForm(%d).String() = %q, want a self-naming fallback", int(f), got)
		}
		if !strings.Contains(got, "EntryForm") || !strings.Contains(got, strconv.Itoa(int(f))) {
			t.Fatalf("EntryForm(%d).String() = %q, want it to name both the type and the value", int(f), got)
		}
	}
}
