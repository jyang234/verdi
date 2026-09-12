package sealedexec

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type consolidationWitness struct {
	Schema         string               `json:"schema"`
	Baseline       string               `json:"baseline"`
	BaselineTree   string               `json:"baseline_tree"`
	BaselineSource map[string]string    `json:"baseline_source_sha256"`
	Implementation map[string]string    `json:"implementation_sha256"`
	Corpus         map[string]string    `json:"corpus_sha256"`
	Method         string               `json:"method"`
	Checks         []consolidationCheck `json:"checks"`
	Functions      []json.RawMessage    `json:"functions"`
	Types          []json.RawMessage    `json:"types"`
	Adapters       []json.RawMessage    `json:"retained_adapters"`
	Totals         struct {
		Checks      int `json:"checks"`
		Functions   int `json:"functions"`
		Types       int `json:"types"`
		ASTNodes    int `json:"source_ast_nodes"`
		CorpusCases int `json:"corpus_cases"`
		Direct      int `json:"conversion_direct_returns"`
		Delegated   int `json:"delegated_validator_calls"`
		Relations   int `json:"same_side_relations"`
		Schemas     int `json:"typed_schema_guards"`
		Unions      int `json:"typed_union_guards"`
	} `json:"totals"`
	TraceEvidence json.RawMessage   `json:"trace_evidence"`
	NamedTests    []string          `json:"named_test_roots"`
	TestSource    map[string]string `json:"test_source_sha256"`
	ATC           json.RawMessage   `json:"atc_endpoint"`
}
type consolidationCheck struct {
	ID              string          `json:"id"`
	Source          json.RawMessage `json:"source"`
	PublicFunctions []string        `json:"public_functions"`
	Status          string          `json:"status"`
	Mutations       []struct {
		Case       string            `json:"case"`
		NewReturns []json.RawMessage `json:"new_returns"`
	} `json:"mutations"`
	BaselineCases []string        `json:"baseline_rejection_cases"`
	Premises      string          `json:"premises,omitempty"`
	Normalization string          `json:"preserved_normalization,omitempty"`
	ATC           json.RawMessage `json:"atc,omitempty"`
}

func consolidationReadStrict(t *testing.T, path string, value any) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		t.Fatal(err)
	}
	if err = d.Decode(new(any)); err != io.EOF {
		t.Fatalf("%s: trailing JSON: %v", path, err)
	}
	return b
}

// This is the release-runner entrypoint for source-bound deletion preservation.
// It replays referenced immutable operands and the named negative caller tests;
// the separately sealed observation packet establishes the old/new call sites.
// Opaque nested metadata is authenticated by the reviewed document's exact
// digest, not by a general RawMessage schema validator. Source and test hashes
// bind its named assertion-bearing roots; this checker is excluded to avoid a
// self-reference. The immutable metadata binding does not replace fresh replay.
func TestPublicControllerConsolidationWitness(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "public-consolidation", "checks.json"))
	if err != nil {
		t.Fatal(err)
	}
	consolidationValidateWitness(t, b)
}

func consolidationValidateWitness(t *testing.T, b []byte) {
	t.Helper()
	if consolidationDigest(b) != "d293818aefb2950418e1f84d13240b83dac539f5a385b548b2ce73e99eea14a4" {
		t.Fatal("witness metadata does not match the reviewed immutable document")
	}
	var w consolidationWitness
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&w); err != nil {
		t.Fatal(err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatalf("witness trailing JSON: %v", err)
	}
	if w.Schema != "verdi.public-consolidation-witness/v1" || w.Baseline != "e671a326418fdfb672a92398228bafcac184c99f" || w.BaselineTree != "0cc835c72aa5ef6f6299026632edf816003d3b32" {
		t.Fatal("wrong accepted source binding")
	}
	if w.Totals.Checks != 222 || len(w.Checks) != 222 || w.Totals.Functions != 165 || len(w.Functions) != 165 || w.Totals.Types != 96 || len(w.Types) != 96 || w.Totals.ASTNodes != 1277 || w.Totals.CorpusCases != 8043 || w.Totals.Direct != 101 || w.Totals.Delegated != 59 || w.Totals.Relations != 16 || w.Totals.Schemas != 44 || w.Totals.Unions != 2 {
		t.Fatal("incomplete witness inventory")
	}
	if len(w.Implementation) == 0 || len(w.BaselineSource) != 5 || len(w.TestSource) == 0 {
		t.Fatal("missing source hashes")
	}
	for _, hashes := range []map[string]string{w.Implementation, w.TestSource} {
		for path, want := range hashes {
			if filepath.IsAbs(path) || strings.Contains(path, "..") || !strings.HasSuffix(path, ".go") {
				t.Fatalf("invalid source path %q", path)
			}
			b, err := os.ReadFile(filepath.Join("..", "..", path))
			if err != nil {
				t.Fatal(err)
			}
			if consolidationDigest(b) != want {
				t.Fatalf("stale witness source %s", path)
			}
		}
	}
	var peer struct {
		Head    string            `json:"head"`
		Tree    string            `json:"tree"`
		Source  map[string]string `json:"source_sha256"`
		Tests   map[string]string `json:"test_source_sha256"`
		Routes  json.RawMessage   `json:"invocation_routes"`
		Fresh   json.RawMessage   `json:"fresh_evidence"`
		Carried []string          `json:"carried_evidence"`
		Scope   string            `json:"scope"`
	}
	peerDecoder := json.NewDecoder(bytes.NewReader(w.ATC))
	peerDecoder.DisallowUnknownFields()
	if err := peerDecoder.Decode(&peer); err != nil {
		t.Fatal(err)
	}
	if peer.Head != "c849c0dbc1862e21034d7cfe43ab17bbb249e225" || len(peer.Source) != 23 || len(peer.Routes) == 0 {
		t.Fatal("missing ATC endpoint binding")
	}
	peerRoot := os.Getenv("VERDI_CONSOLIDATION_ATC_SOURCE")
	if peerRoot == "" && os.Getenv("VERDI_CONSOLIDATION_REQUIRE_ATC") == "1" {
		t.Fatal("paired witness requires VERDI_CONSOLIDATION_ATC_SOURCE")
	}
	if peerRoot != "" {
		for _, hashes := range []map[string]string{peer.Source, peer.Tests} {
			for path, want := range hashes {
				if filepath.IsAbs(path) || strings.Contains(path, "..") {
					t.Fatalf("invalid peer path %q", path)
				}
				b, err := os.ReadFile(filepath.Join(peerRoot, path))
				if err != nil {
					t.Fatal(err)
				}
				if !consolidationATCSourceMatches(path, want, b) {
					t.Fatalf("stale ATC witness source %s", path)
				}
			}
		}
	} else {
		t.Log("ATC source binding carried from exact-source evidence; set VERDI_CONSOLIDATION_ATC_SOURCE for paired replay")
	}
	cases := map[string]consolidationCase{}
	if len(w.Corpus) != 2 {
		t.Fatal("missing immutable corpora")
	}
	for name, want := range w.Corpus {
		if name != "corpus.json" && name != "supplement.json" {
			t.Fatalf("unknown corpus %s", name)
		}
		var corpus consolidationCorpus
		b := consolidationReadStrict(t, filepath.Join("testdata", "public-consolidation", name), &corpus)
		if consolidationDigest(b) != want || corpus.Baseline != w.Baseline {
			t.Fatalf("stale corpus %s", name)
		}
		for fixture, digest := range corpus.Fixtures {
			b, err := consolidationFixture(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if consolidationDigest(b) != digest {
				t.Fatalf("stale fixture %s", fixture)
			}
		}
		for _, row := range corpus.Cases {
			if _, ok := cases[row.ID]; ok || row.ID == "" {
				t.Fatalf("duplicate/empty mutation %q", row.ID)
			}
			cases[row.ID] = row
		}
	}
	if len(cases) != 8043 {
		t.Fatal("incomplete corpus cases")
	}
	runners := map[string]func(*testing.T){
		"TestContextControllerWireContract_Static":          TestContextControllerWireContract_Static,
		"TestContextOwnerBridgeCrossMatch":                  TestContextOwnerBridgeCrossMatch,
		"TestPublicControllerConsolidationActiveChecks":     TestPublicControllerConsolidationActiveChecks,
		"TestPublicControllerConsolidationReceiptEventKind": TestPublicControllerConsolidationReceiptEventKind,
		"TestPublicControllerConsolidationPayloadOwner":     TestPublicControllerConsolidationPayloadOwner,
		"TestPublicControllerConsolidationDomainUnions":     TestPublicControllerConsolidationDomainUnions,
	}
	requiredTests := map[string]bool{}
	seen := map[string]bool{}
	replayed := map[string]bool{}
	for _, row := range w.Checks {
		if row.ID == "" || seen[row.ID] || len(row.Source) == 0 || len(row.PublicFunctions) == 0 || len(row.Mutations) == 0 || len(row.ATC) == 0 {
			t.Fatalf("incomplete or duplicate check %s", row.ID)
		}
		seen[row.ID] = true
		switch row.Status {
		case "observed_rejection", "observed_delegated_rejection", "observed_domain_schema_rejection":
		case "dominated_duplicate", "retained_domain_predicate", "delegated_json_encoding_error", "delegated_validation_obligation", "dominated_same_side_relation":
			if row.Premises == "" {
				t.Fatalf("missing premises %s", row.ID)
			}
		default:
			t.Fatalf("unproved check %s: %s", row.ID, row.Status)
		}
		for _, mutation := range row.Mutations {
			if len(mutation.NewReturns) == 0 {
				t.Fatalf("missing actual invocation %s", row.ID)
			}
			if strings.HasPrefix(mutation.Case, "Test") {
				root := strings.Split(mutation.Case, "/")[0]
				if runners[root] == nil {
					t.Fatalf("missing named mutation runner %s", mutation.Case)
				}
				requiredTests[root] = true
				continue
			}
			operand, ok := cases[mutation.Case]
			if !ok || operand.Accepted {
				t.Fatalf("missing/nonrejecting mutation %s", mutation.Case)
			}
			if replayed[operand.ID] {
				continue
			}
			replayed[operand.ID] = true
			t.Run(operand.ID, func(t *testing.T) {
				digest, accepted, output, _, err := consolidationExecute(operand)
				if err != nil {
					t.Fatal(err)
				}
				if accepted || output != "" || digest != operand.OperandSHA256 {
					t.Fatalf("rejection witness changed: %s", operand.ID)
				}
			})
		}
	}
	if len(w.NamedTests) != len(requiredTests) {
		t.Fatal("named test inventory mismatch")
	}
	for _, name := range w.NamedTests {
		if !requiredTests[name] {
			t.Fatalf("duplicate/unused named test %s", name)
		}
		delete(requiredTests, name)
		t.Run(name, runners[name])
	}
	for label, rows := range map[string][]json.RawMessage{"function": w.Functions, "type": w.Types, "adapter": w.Adapters} {
		ids := map[string]bool{}
		for _, raw := range rows {
			var row struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			if row.ID == "" || ids[row.ID] {
				t.Fatalf("duplicate/missing %s identity %s", label, row.ID)
			}
			ids[row.ID] = true
		}
	}
}

// The immutable Task4 witness remains historical. This sole later correction
// binds exact reviewed bytes and proves the inverse edit recovers those bytes.
func consolidationATCSourceMatches(path, want string, source []byte) bool {
	if consolidationDigest(source) == want {
		return true
	}
	if path != consolidationATCBoundPath || want != consolidationATCBeforeBound || consolidationDigest(source) != consolidationATCAfterBound {
		return false
	}
	guard := []byte(consolidationATCBoundGuard)
	return bytes.Count(source, guard) == 1 && consolidationDigest(bytes.Replace(source, guard, nil, 1)) == want
}
