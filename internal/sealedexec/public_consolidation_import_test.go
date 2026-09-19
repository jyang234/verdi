package sealedexec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// consolidationVerdiSuccessors pins the reviewed successors of the sources the
// immutable Task4 witness still binds by their historical digests. Two waves
// of accepted work are pinned here:
//
//   - SI-200 (spec-import-contract.md's import-sidecar clause, adopted and
//     Task3 reviewed): contextcompile's classify.go, schema.go, validate.go.
//   - SI-204 / spec/spec-documents ac-10 (ruling R-W4-11): governanceprincipal's
//     decode.go and profile.go, whose Profile.Template seam the wave-4 Task 1
//     review read and approved for this binding.
//
// The witness document itself is unchanged in both waves: its digest, corpora,
// totals and replay operands stay exactly as reviewed, and every other bound
// source is still admitted only on an exact historical match.
var consolidationVerdiSuccessors = map[string]consolidationVerdiSuccessor{
	"internal/contextcompile/classify.go": {
		Historical: "451ca5f22e4e2ef14408d312686b871009418e2b8edde216627e45581ecf5c76",
		Successor:  "31131fa517997478e716cd0cd6823a8dc6d935b47d8d6a93675e36899c5ddae4",
		Inverse: []consolidationVerdiEdit{
			{From: "\tif candidate.Source == SourceHeadTree && specImportSidecarPath(candidate.Path) {\n" +
				"\t\treturn ExclusionSpecImportSidecar, true, nil\n" +
				"\t}\n"},
			{From: "// specImportSidecarBoundary is the fixed committed import-provenance zone\n" +
				"// (store.ImportDirRelPath's own .verdi/imports/<slug>/<preview-digest>/\n" +
				"// root). It is a boundary prefix, not a per-file identity: the whole\n" +
				"// subtree is non-authoritative import provenance.\n" +
				"const specImportSidecarBoundary = \".verdi/imports/\"\n" +
				"\n" +
				"// specImportSidecarPath reports whether repoPath lies beneath the committed\n" +
				"// import-provenance boundary (spec-import-contract.md: \"The context\n" +
				"// compiler's separate HEAD-tree repository-file channel must also exclude\n" +
				"// paths beneath .verdi/imports/ before reading their bytes\"). Ordinary\n" +
				"// neighbours whose names merely begin with the same letters\n" +
				"// (.verdi/imports.md, .verdi/importsomething/...) are not beneath it.\n" +
				"func specImportSidecarPath(repoPath string) bool {\n" +
				"\treturn strings.HasPrefix(repoPath, specImportSidecarBoundary)\n" +
				"}\n" +
				"\n"},
		},
	},
	"internal/contextcompile/schema.go": {
		Historical: "090019ac0e8fa222421b32cf6481a29e200e86b7e1462f82531e999403797a15",
		Successor:  "ccab9c42c74f89541b3c6a4b6bd967055c108b6ad67b3bda4634bec3881b392c",
		Inverse: []consolidationVerdiEdit{
			{From: "// ExclusionReason is one of the eleven closed exclusion reasons: the ten\n" +
				"// of authority design §5.2, whose closed v1 enum SI-200 supplements with\n" +
				"// the single member spec-import-sidecar (spec-import-contract.md: \"This\n" +
				"// SI-200 clause supplements the context-compiler authority design, §5.2:\n" +
				"// its closed v1 enum gains this one member; all existing members and\n" +
				"// fail-closed decoding remain\"). Unknown values fail closed.\n",
				To: "// ExclusionReason is one of the ten closed exclusion reasons (authority\n" +
					"// design §5.2). Unknown values fail closed.\n"},
			{From: "\tExclusionSpecImportSidecar         ExclusionReason = \"spec-import-sidecar\"\n"},
		},
	},
	"internal/contextcompile/validate.go": {
		Historical: "51eed09d21e9ad0da04237f1f1860b37fc71686ca38ad152c3482f2a859e0487",
		Successor:  "31cf1c6733aca29b9a03b10bb585f2f872f672d2d3489a2b73fafa1425f3126d",
		Inverse: []consolidationVerdiEdit{
			{From: "\t\tExclusionArchivedRecord, ExclusionGeneratedProjectionOutput, ExclusionNonTextData, ExclusionNonRegularFile,\n" +
				"\t\tExclusionSpecImportSidecar:\n",
				To: "\t\tExclusionArchivedRecord, ExclusionGeneratedProjectionOutput, ExclusionNonTextData, ExclusionNonRegularFile:\n"},
		},
	},
	"internal/governanceprincipal/decode.go": {
		Historical: "d69540dd109971b7bf36f40f44366345a32f1501c3b08e2d9eda7eec19c65043",
		Successor:  "d0b9e48a966885089c658ff4e0d0240e55b519b7a9237cbe9f50e2e68551a3bb",
		Inverse: []consolidationVerdiEdit{
			{From: "\tTemplate                   *TemplateRecord           `yaml:\"template\"`\n"},
			{From: "\tif doc.Template != nil {\n" +
				"\t\tif err := doc.Template.Validate(); err != nil {\n" +
				"\t\t\treturn Profile{}, fmt.Errorf(\"governanceprincipal: profile %w\", err)\n" +
				"\t\t}\n" +
				"\t}\n" +
				"\n"},
			{From: "\t\tTemplate:                   doc.Template,\n"},
		},
	},
	"internal/governanceprincipal/profile.go": {
		Historical: "91960126159351d7def440918c7d9141095939609ebdb949799d43607662f89d",
		Successor:  "6ee7192e0fce13271089bdf54bda538846fc0d4d16cf9d82dbb411b19ee093c2",
		Inverse: []consolidationVerdiEdit{
			{From: "\t// Template is the resolved scaffold record a starter-written profile\n" +
				"\t// carries (spec/spec-documents ac-10, SI-204); optional \u2014 a\n" +
				"\t// hand-authored profile carries none; digest-bound like every other\n" +
				"\t// exported field.\n" +
				"\tTemplate *TemplateRecord `json:\"template,omitempty\"`\n" +
				"\n"},
		},
	},
}

func TestPublicControllerConsolidationImportSuccessor(t *testing.T) {
	// The inverse machinery itself, on bytes no repository file supplies: an
	// edit that is absent, repeated or out of order recovers nothing.
	t.Run("inverse", func(t *testing.T) {
		remove := consolidationVerdiEdit{From: "b\n"}
		rename := consolidationVerdiEdit{From: "a\n", To: "A\n"}
		for _, tc := range []struct {
			name     string
			source   string
			inverse  []consolidationVerdiEdit
			restored string
			ok       bool
		}{
			{"single-removal", "a\nb\nc\n", []consolidationVerdiEdit{remove}, "a\nc\n", true},
			{"ordered-pair", "a\nb\nc\n", []consolidationVerdiEdit{remove, rename}, "A\nc\n", true},
			{"absent-edit", "a\nc\n", []consolidationVerdiEdit{remove}, "", false},
			{"repeated-edit", "a\nb\nb\nc\n", []consolidationVerdiEdit{remove}, "", false},
			{"repeated-after-earlier-edit", "a\nb\na\n", []consolidationVerdiEdit{remove, rename}, "", false},
			{"no-edits-leaves-source", "a\nb\n", nil, "a\nb\n", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := consolidationApplyInverse([]byte(tc.source), tc.inverse)
				if ok != tc.ok {
					t.Fatalf("applied=%v want=%v", ok, tc.ok)
				}
				if ok && string(got) != tc.restored {
					t.Fatalf("restored %q want %q", got, tc.restored)
				}
			})
		}
	})

	// Exact historical bytes are admitted on any path, and unknown bytes are
	// refused whether or not the path carries a pinned successor.
	t.Run("pure", func(t *testing.T) {
		unbound := "internal/contextcompile/absent.go"
		mapped := "internal/contextcompile/schema.go"
		historical := consolidationVerdiSuccessors[mapped].Historical
		for _, tc := range []struct {
			name, path, want string
			source           []byte
			valid            bool
		}{
			{"unchanged-source", unbound, consolidationDigest([]byte("unchanged")), []byte("unchanged"), true},
			{"unknown-source-change", unbound, consolidationDigest([]byte("unchanged")), []byte("changed"), false},
			{"unknown-bytes-on-mapped-path", mapped, historical, []byte("changed"), false},
			{"successor-hash-is-not-an-admission", mapped, consolidationVerdiSuccessors[mapped].Successor, []byte("changed"), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := consolidationVerdiSourceMatches(tc.path, tc.want, tc.source); got != tc.valid {
					t.Fatalf("source admission=%v want=%v", got, tc.valid)
				}
			})
		}
	})

	// Each mapped successor, against the actual reviewed bytes in this tree.
	t.Run("bound", func(t *testing.T) {
		for path, bound := range consolidationVerdiSuccessors {
			t.Run(path, func(t *testing.T) {
				current, err := os.ReadFile(filepath.Join("..", "..", path))
				if err != nil {
					t.Fatal(err)
				}
				if consolidationDigest(current) != bound.Successor {
					t.Fatalf("%s is not the pinned reviewed successor", path)
				}
				historical, ok := consolidationApplyInverse(current, bound.Inverse)
				if !ok || consolidationDigest(historical) != bound.Historical {
					t.Fatalf("the pinned inverse edit did not reproduce %s's historical bytes", path)
				}
				other := ""
				for candidate := range consolidationVerdiSuccessors {
					if candidate != path {
						other = candidate
					}
				}
				for _, tc := range []struct {
					name, path, want string
					source           []byte
					valid            bool
				}{
					{"historical", path, bound.Historical, historical, true},
					{"successor", path, bound.Historical, current, true},
					{"wrong-path", path + ".bak", bound.Historical, current, false},
					{"other-mapped-path", other, bound.Historical, current, false},
					{"wrong-historical-hash", path, strings.Repeat("0", 64), current, false},
					{"crossed-historical-hash", path, consolidationVerdiSuccessors[other].Historical, current, false},
					{"appended-to-successor", path, bound.Historical, append(append([]byte{}, current...), '\n'), false},
					{"appended-to-historical", path, bound.Historical, append(append([]byte{}, historical...), '\n'), false},
					{"changed-inside-the-reviewed-delta", path, bound.Historical, consolidationMutateFirstEdit(current, bound.Inverse[0]), false},
					{"repeated-reviewed-delta", path, bound.Historical, consolidationRepeatFirstEdit(current, bound.Inverse[0]), false},
				} {
					t.Run(tc.name, func(t *testing.T) {
						if got := consolidationVerdiSourceMatches(tc.path, tc.want, tc.source); got != tc.valid {
							t.Fatalf("source admission=%v want=%v", got, tc.valid)
						}
					})
				}
			})
		}
	})

	// The table describes real stale witness entries and nothing else: every
	// checked source that differs from its bound digest is a pinned successor,
	// and every pinned successor is bound by the witness at its historical hash.
	t.Run("witness-coverage", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("testdata", "public-consolidation", "checks.json"))
		if err != nil {
			t.Fatal(err)
		}
		var w consolidationWitness
		if err = json.Unmarshal(raw, &w); err != nil {
			t.Fatal(err)
		}
		bound := map[string]string{}
		drifted := map[string]bool{}
		for _, hashes := range []map[string]string{w.Implementation, w.TestSource} {
			for path, want := range hashes {
				bound[path] = want
				source, err := os.ReadFile(filepath.Join("..", "..", path))
				if err != nil {
					t.Fatal(err)
				}
				if consolidationDigest(source) == want {
					continue
				}
				drifted[path] = true
				if _, pinned := consolidationVerdiSuccessors[path]; !pinned {
					t.Fatalf("unmapped drift in bound source %s", path)
				}
				if !consolidationVerdiSourceMatches(path, want, source) {
					t.Fatalf("pinned successor %s is not admitted", path)
				}
			}
		}
		for path, successor := range consolidationVerdiSuccessors {
			if bound[path] != successor.Historical {
				t.Fatalf("%s is not bound by the witness at its pinned historical digest", path)
			}
			if !drifted[path] {
				t.Logf("%s currently holds its historical bytes; its successor pin is unexercised", path)
			}
		}
	})
}

// consolidationMutateFirstEdit changes one byte inside the reviewed delta, so
// the result is neither the historical nor the reviewed successor source. The
// byte is the delta's first ASCII letter, case-flipped: a mutation every pinned
// delta admits whatever its text, since the waves pinned above share no
// vocabulary to key on.
func consolidationMutateFirstEdit(source []byte, edit consolidationVerdiEdit) []byte {
	mutated := []byte(edit.From)
	for i, b := range mutated {
		if b >= 'a' && b <= 'z' {
			mutated[i] = b - ('a' - 'A')
			break
		}
		if b >= 'A' && b <= 'Z' {
			mutated[i] = b + ('a' - 'A')
			break
		}
	}
	if bytes.Equal(mutated, []byte(edit.From)) {
		panic("consolidationMutateFirstEdit: delta carries no ASCII letter to change")
	}
	return bytes.Replace(source, []byte(edit.From), mutated, 1)
}

// consolidationRepeatFirstEdit duplicates the reviewed delta, so no single
// inverse edit can be unambiguous.
func consolidationRepeatFirstEdit(source []byte, edit consolidationVerdiEdit) []byte {
	from := []byte(edit.From)
	return bytes.Replace(source, from, append(append([]byte{}, from...), from...), 1)
}
