// Permanent, encoder-independent verification of the frozen public execution
// contract fixtures (VATC F12 controller-owner bridge correction §3, sibling
// invention-ledger row SI-181; Task 1 Lane A).
//
// This file deliberately never imports internal/canonjson, internal/artifact,
// or internal/sealedexec — the encoders whose future v2 change the frozen
// bytes must survive unmoved. It judges the fixtures with nothing but the
// bytes themselves, crypto/sha256, bytes/strings/slices/maps/encoding/json/
// fmt, and this package's own public decoders (DecodeCall/EncodeCall/
// DecodeReply/EncodeReply), so it stays a valid oracle no matter how the
// private controller wire changes underneath it.
//
// Every schema literal it judges a fixture against is spelled here, never
// read out of the code under test; the package's own RequestSchema/
// ResultSchema derivation is separately asserted to equal those literals, so
// product drift fails this test instead of silently redefining what it
// checks.
//
// It runs as package contextowner (not contextowner_test): that is what lets
// it reach the unexported digestBytes helper directly without re-importing
// the package it is testing, while still never importing sealedexec (which
// itself imports contextowner, so the reverse import would cycle).
package contextowner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// publicContractFixtureDir is the one frozen fixture directory this file and
// any later contextowner test may read.
const publicContractFixtureDir = "testdata/public-contract"

// readPublicContractFixture loads one frozen byte-exact fixture by name. It
// is the one seam later contextowner tests should reuse rather than
// re-deriving the path or re-reading the file themselves.
func readPublicContractFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(publicContractFixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// nonProvenEpochFixtureNames are the six frozen files carrying contract §2.1's
// non-proven context resolution embedded in an epoch check -- the arm §6
// requires per-arm non-proven coverage for and Task 1 requires the frozen
// fixtures to include. They are named once here so the inventory's required
// set and the sensitivity witness below cannot drift apart.
var nonProvenEpochFixtureNames = []string{
	"verify-epoch.nonproven-resolution.legacy-request.json",
	"verify-epoch.nonproven-resolution.request.json",
	"verify-epoch.nonproven-resolution.owner-call.json",
	"verify-epoch.nonproven-resolution.owner-reply.json",
	"verify-epoch.nonproven-resolution.call.v1.json",
	"verify-epoch.nonproven-resolution.call.v2.json",
}

// legacyControllerRequestSchemaLiteral and legacyControllerResultSchemaLiteral
// reproduce, by literal string derivation only, the private controller
// schema tags internal/sealedexec's controllerRequestSchema/
// controllerResultSchema compute for one operation. This package cannot
// import sealedexec to call them directly -- sealedexec already imports
// contextowner, so the reverse import would cycle -- and restating the
// derivation here, rather than trusting the frozen fixtures to be
// self-consistent, is what lets this test catch a fixture whose "legacy"
// half was not actually the private wire's literal.
func legacyControllerRequestSchemaLiteral(operation string) string {
	if operation == "install-expansion" {
		return "verdi.context-controller/" + operation + "-request/v2"
	}
	return "verdi.context-controller/" + operation + "-request/v1"
}

func legacyControllerResultSchemaLiteral(operation string) string {
	return "verdi.context-controller/" + operation + "-result/v1"
}

// publicOwnerRequestSchemaLiteral and publicOwnerResultSchemaLiteral pin, by
// literal string derivation only, the published arm schemas contract §2.1
// fixes: verdi.context-owner/<op>-request/v1 and
// verdi.context-owner/<op>-result/v1, with install-expansion's one ratified
// -request/v2 exception (SI-182).
//
// The guard must spell these itself rather than read them out of
// RequestSchema/ResultSchema. Taking the expected literal from the product
// code under test would make any drift in that code invisible here -- the
// fixtures would simply be re-judged against whatever the code now says. The
// product derivation is instead asserted equal to these literals (see
// public_arm_schemas_match_the_contract below), so drift is caught rather
// than trusted.
func publicOwnerRequestSchemaLiteral(operation string) string {
	if operation == "install-expansion" {
		return "verdi.context-owner/" + operation + "-request/v2"
	}
	return "verdi.context-owner/" + operation + "-request/v1"
}

func publicOwnerResultSchemaLiteral(operation string) string {
	return "verdi.context-owner/" + operation + "-result/v1"
}

// The four outer-envelope schema literals contract §2.2 fixes, plus the one
// error-document literal, spelled once.
const (
	controllerCallSchemaV1   = "verdi.context-controller-call/v1"
	controllerCallSchemaV2   = "verdi.context-controller-call/v2"
	controllerResultSchemaV1 = "verdi.context-controller-result/v1"
	controllerResultSchemaV2 = "verdi.context-controller-result/v2"
	controllerErrorSchema    = "verdi.context-controller-error/v1"
)

// literalControllerCall and literalControllerResult are the exact
// outer-envelope composition rule every frozen *.call.v*.json /
// *.result.v*.json fixture must equal: plain string assembly around one
// already-canonical inner payload, never a second encoder. The v1 frames obey
// the identical rule, differing only in the envelope schema literal and in
// carrying the legacy private-wire arm as their payload.
func literalControllerCall(operation, envelopeSchema string, payloadNoLF []byte) []byte {
	return []byte(`{"call_sequence":1,"operation":"` + operation + `","payload":` + string(payloadNoLF) +
		`,"schema":"` + envelopeSchema + `"}` + "\n")
}

func literalControllerResult(operation, envelopeSchema, payloadMember string, memberNoLF []byte) []byte {
	return []byte(`{"call_sequence":1,"operation":"` + operation + `","payload":{"` + payloadMember + `":` + string(memberNoLF) +
		`},"schema":"` + envelopeSchema + `"}` + "\n")
}

func literalControllerCallV1(operation string, payloadNoLF []byte) []byte {
	return literalControllerCall(operation, controllerCallSchemaV1, payloadNoLF)
}

func literalControllerCallV2(operation string, payloadNoLF []byte) []byte {
	return literalControllerCall(operation, controllerCallSchemaV2, payloadNoLF)
}

func literalControllerResultV1(operation string, resultMemberNoLF []byte) []byte {
	return literalControllerResult(operation, controllerResultSchemaV1, "result", resultMemberNoLF)
}

func literalControllerResultV2(operation string, resultMemberNoLF []byte) []byte {
	return literalControllerResult(operation, controllerResultSchemaV2, "result", resultMemberNoLF)
}

// requireOneLineJSON requires data to be exactly one canonical JSON document
// followed by exactly one LF, with no other LF anywhere in the file.
func requireOneLineJSON(t *testing.T, name string, data []byte) {
	t.Helper()
	if len(data) == 0 {
		t.Fatalf("%s is empty", name)
	}
	if data[len(data)-1] != '\n' {
		t.Fatalf("%s does not end in LF", name)
	}
	if count := bytes.Count(data, []byte("\n")); count != 1 {
		t.Fatalf("%s is not exactly one line: contains %d LF byte(s)", name, count)
	}
	var probe json.RawMessage
	if err := json.Unmarshal(bytes.TrimSuffix(data, []byte("\n")), &probe); err != nil {
		t.Fatalf("%s does not contain one valid JSON document: %v", name, err)
	}
}

// requireSchemaReplacement requires published to be exactly legacy with its
// one top-level schema literal replaced -- the publication rule applied to
// bytes, checked independently of the bridge code that is supposed to
// implement it.
func requireSchemaReplacement(t *testing.T, legacy, published []byte, legacySchema, publicSchema string) {
	t.Helper()
	from := []byte(`"schema":"` + legacySchema + `"`)
	to := []byte(`"schema":"` + publicSchema + `"`)
	if count := bytes.Count(legacy, from); count != 1 {
		t.Fatalf("legacy payload carries %d occurrence(s) of %s, want exactly 1", count, from)
	}
	want := bytes.Replace(legacy, from, to, 1)
	if !bytes.Equal(want, published) {
		t.Fatalf("published payload is not the legacy payload with only its schema literal replaced\n legacy:    %s\n published: %s\n want:      %s", legacy, published, want)
	}
}

// verifyFixtureDigests parses dir's SHA256SUMS manifest and requires it to
// list, in strict sorted order and exact two-space format, the exact SHA-256
// digest of every other file dir contains -- no file present but unlisted,
// no file listed but absent, no digest mismatch. It is factored to take an
// arbitrary directory so the mutation sub-test below can prove it actually
// detects tampering rather than merely restating the fixtures' own claim
// about themselves.
func verifyFixtureDigests(dir string) error {
	manifestPath := filepath.Join(dir, "SHA256SUMS")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	if len(manifest) == 0 || manifest[len(manifest)-1] != '\n' {
		return errorf("%s does not end in a single LF-terminated line", manifestPath)
	}
	lines := strings.Split(strings.TrimSuffix(string(manifest), "\n"), "\n")

	listed := make(map[string]string, len(lines))
	order := make([]string, 0, len(lines))
	previous := ""
	for i, line := range lines {
		if len(line) < 68 || line[64] != ' ' || line[65] != ' ' {
			return errorf("%s line %d is not <64 lowercase hex><two spaces><filename>-formatted: %q", manifestPath, i+1, line)
		}
		digest, name := line[:64], line[66:]
		if !isLowerHex64(digest) {
			return errorf("%s line %d digest %q is not 64 lowercase hex characters", manifestPath, i+1, digest)
		}
		if name == "SHA256SUMS" {
			return errorf("%s lists itself", manifestPath)
		}
		if _, dup := listed[name]; dup {
			return errorf("%s lists %s more than once", manifestPath, name)
		}
		if previous != "" && previous >= name {
			return errorf("%s is not sorted bytewise by filename: %q must precede %q", manifestPath, previous, name)
		}
		listed[name] = digest
		order = append(order, name)
		previous = name
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "SHA256SUMS" {
			continue
		}
		present[entry.Name()] = true
	}
	for name := range present {
		if _, ok := listed[name]; !ok {
			return errorf("%s is present in %s but not listed in SHA256SUMS", name, dir)
		}
	}
	for _, name := range order {
		if !present[name] {
			return errorf("SHA256SUMS lists %s but it is absent from %s", name, dir)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		got := hex.EncodeToString(sum[:])
		if got != listed[name] {
			return errorf("digest mismatch for %s: manifest says %s, computed %s", name, listed[name], got)
		}
	}
	return nil
}

func isLowerHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// errorf is a small local alias for fmt.Errorf used throughout
// verifyFixtureDigests, kept as one name so every manifest-format complaint
// is constructed the same way.
func errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// copyFixtureDir copies every regular file in src into dst (already created
// by the caller, typically t.TempDir()). It refuses a subdirectory rather
// than silently skipping it, since the frozen fixture directory is defined
// to be flat.
func copyFixtureDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("unexpected subdirectory %s in %s", entry.Name(), src)
		}
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.Name(), err)
		}
	}
}

// removeFixturesAndManifestRows deletes each of names from dir and drops the
// matching row from dir's SHA256SUMS, so the mutilated directory stays
// internally consistent: its manifest still verifies clean. That is exactly
// the shape a coverage regression would take, and it is why the manifest
// check alone cannot catch it.
func removeFixturesAndManifestRows(t *testing.T, dir string, names []string) {
	t.Helper()
	drop := make(map[string]bool, len(names))
	for _, name := range names {
		drop[name] = true
	}
	manifestPath := filepath.Join(dir, "SHA256SUMS")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read %s: %v", manifestPath, err)
	}
	kept := make([]string, 0)
	dropped := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(manifest), "\n"), "\n") {
		if len(line) > 66 && drop[line[66:]] {
			dropped[line[66:]] = true
			continue
		}
		kept = append(kept, line)
	}
	for _, name := range names {
		if !dropped[name] {
			t.Fatalf("%s has no SHA256SUMS row for %s", manifestPath, name)
		}
	}
	if err := os.WriteFile(manifestPath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("rewrite %s: %v", manifestPath, err)
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
}

// requiredPublicContractFixtureNames computes the exact set of file names the
// frozen fixture directory must contain.
//
// It deliberately takes no directory: the required set is a property of the
// contract, never of what happens to be on disk. Deriving it from the disk is
// what previously let the six non-proven epoch fixtures be deleted without
// failing anything -- a guard that asks the directory what it should contain
// cannot detect missing coverage.
func requiredPublicContractFixtureNames(t *testing.T) map[string]bool {
	t.Helper()
	ops := Operations()
	if len(ops) != 22 {
		t.Fatalf("Operations() returned %d operations, want 22", len(ops))
	}
	suffixes := []string{
		".legacy-request.json", ".request.json", ".legacy-result.json", ".result.json",
		".owner-call.json", ".owner-reply.json", ".call.v1.json", ".result.v1.json",
		".call.v2.json", ".result.v2.json",
	}
	required := map[string]bool{}
	for _, op := range ops {
		for _, suffix := range suffixes {
			required[string(op)+suffix] = true
		}
	}
	for _, name := range []string{
		"resolve-claim-mcp.legacy-request.json", "resolve-claim-mcp.legacy-result.json",
		"resolve-claim-mcp.call.v1.json", "resolve-claim-mcp.result.v1.json",
		"resolve-claim-mcp.call.v2.json", "resolve-claim-mcp.result.v2.json",
		"resolve-context.nonproven.legacy-result.json", "resolve-context.nonproven.result.json",
		"resolve-context.nonproven.owner-reply.json", "resolve-context.nonproven.result.v1.json",
		"resolve-context.nonproven.result.v2.json",
		"error.result.v1.json", "error.result.v2.json",
		"digests.json", "provenance.txt", "SHA256SUMS",
	} {
		required[name] = true
	}
	for _, name := range nonProvenEpochFixtureNames {
		required[name] = true
	}
	return required
}

// publicContractFixtureInventory reports which required names are absent from
// dir and which of dir's files are not required, sorted. It takes a directory
// so the sensitivity witness below can run the very checker the primary test
// runs against a deliberately mutilated copy.
func publicContractFixtureInventory(t *testing.T, dir string) (missing, unexpected []string) {
	t.Helper()
	required := requiredPublicContractFixtureNames(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	present := map[string]bool{}
	for _, entry := range entries {
		present[entry.Name()] = true
	}
	for name := range required {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	for name := range present {
		if !required[name] {
			unexpected = append(unexpected, name)
		}
	}
	slices.Sort(missing)
	slices.Sort(unexpected)
	return missing, unexpected
}

// TestPublicContractFixtures_Inventory is the permanent, always-on guard over
// the frozen fixtures: every claim Task 1 Lane A makes about them is checked
// here from the bytes alone, so a later change to the v2 codec (Task 2+)
// cannot silently regenerate -- and thereby quietly launder -- the oracle it
// is supposed to be judged against.
func TestPublicContractFixtures_Inventory(t *testing.T) {
	if err := verifyFixtureDigests(publicContractFixtureDir); err != nil {
		t.Fatalf("frozen fixture manifest is invalid: %v", err)
	}

	t.Run("expected_files_present", func(t *testing.T) {
		missing, unexpected := publicContractFixtureInventory(t, publicContractFixtureDir)
		for _, name := range missing {
			t.Errorf("expected fixture file missing: %s", name)
		}
		for _, name := range unexpected {
			t.Errorf("unexpected fixture file present: %s", name)
		}
	})

	t.Run("missing_nonproven_epoch_fixtures_are_detected", func(t *testing.T) {
		tempDir := t.TempDir()
		copyFixtureDir(t, publicContractFixtureDir, tempDir)
		removeFixturesAndManifestRows(t, tempDir, nonProvenEpochFixtureNames)

		// The mutilated copy is still internally consistent -- its manifest
		// verifies clean -- which is precisely why the digest check alone
		// cannot catch a dropped arm and the required-set check must.
		if err := verifyFixtureDigests(tempDir); err != nil {
			t.Fatalf("mutilated copy is not internally consistent, so this witness would prove nothing: %v", err)
		}

		missing, unexpected := publicContractFixtureInventory(t, tempDir)
		want := slices.Sorted(slices.Values(nonProvenEpochFixtureNames))
		if len(unexpected) != 0 {
			t.Errorf("deleting the non-proven epoch arm reported unexpected files %v, want none", unexpected)
		}
		if !slices.Equal(missing, want) {
			t.Errorf("deleting the non-proven epoch arm reported missing=%v, want exactly %v", missing, want)
		}
	})

	// Product drift, not fixture drift: the arm schemas this guard pins by its
	// own literal derivation must still be exactly what the package derives.
	// Asserting the equality here is what lets every other check below use the
	// pinned literal without thereby trusting the code under test.
	t.Run("public_arm_schemas_match_the_contract", func(t *testing.T) {
		for _, op := range Operations() {
			opName := string(op)
			if got, want := RequestSchema(op), publicOwnerRequestSchemaLiteral(opName); got != want {
				t.Errorf("RequestSchema(%s) = %q, want the contract §2.1 literal %q", opName, got, want)
			}
			if got, want := ResultSchema(op), publicOwnerResultSchemaLiteral(opName); got != want {
				t.Errorf("ResultSchema(%s) = %q, want the contract §2.1 literal %q", opName, got, want)
			}
		}
	})

	t.Run("digests_key_set", func(t *testing.T) {
		var digests map[string]string
		if err := json.Unmarshal(readPublicContractFixture(t, "digests.json"), &digests); err != nil {
			t.Fatalf("unmarshal digests.json: %v", err)
		}
		want := make([]string, 0, 23)
		for _, op := range Operations() {
			want = append(want, string(op))
		}
		want = append(want, "verify-epoch.nonproven-resolution")
		slices.Sort(want)
		got := slices.Sorted(maps.Keys(digests))
		if !slices.Equal(got, want) {
			t.Errorf("digests.json keys = %v, want exactly the 22 operations plus verify-epoch.nonproven-resolution %v", got, want)
		}
	})

	t.Run("per_operation_semantics", func(t *testing.T) {
		var digests map[string]string
		digestsRaw := readPublicContractFixture(t, "digests.json")
		requireOneLineJSON(t, "digests.json", digestsRaw)
		if err := json.Unmarshal(digestsRaw, &digests); err != nil {
			t.Fatalf("unmarshal digests.json: %v", err)
		}

		for _, op := range Operations() {
			opName := string(op)
			t.Run(opName, func(t *testing.T) {
				legacyRequest := readPublicContractFixture(t, opName+".legacy-request.json")
				requireOneLineJSON(t, opName+".legacy-request.json", legacyRequest)
				request := readPublicContractFixture(t, opName+".request.json")
				requireOneLineJSON(t, opName+".request.json", request)
				requireSchemaReplacement(t, legacyRequest, request, legacyControllerRequestSchemaLiteral(opName), publicOwnerRequestSchemaLiteral(opName))

				legacyResult := readPublicContractFixture(t, opName+".legacy-result.json")
				requireOneLineJSON(t, opName+".legacy-result.json", legacyResult)
				result := readPublicContractFixture(t, opName+".result.json")
				requireOneLineJSON(t, opName+".result.json", result)
				requireSchemaReplacement(t, legacyResult, result, legacyControllerResultSchemaLiteral(opName), publicOwnerResultSchemaLiteral(opName))

				ownerCall := readPublicContractFixture(t, opName+".owner-call.json")
				requireOneLineJSON(t, opName+".owner-call.json", ownerCall)

				wantDigest := digestBytes(legacyRequest)
				if got := digests[opName]; got != wantDigest {
					t.Errorf("digests.json[%s] = %q, want %q", opName, got, wantDigest)
				}
				var callMembers map[string]json.RawMessage
				if err := json.Unmarshal(ownerCall, &callMembers); err != nil {
					t.Fatalf("unmarshal %s.owner-call.json: %v", opName, err)
				}
				var controllerRequestDigest string
				if err := json.Unmarshal(callMembers["controller_request_digest"], &controllerRequestDigest); err != nil {
					t.Fatalf("unmarshal %s.owner-call.json controller_request_digest: %v", opName, err)
				}
				if controllerRequestDigest != wantDigest {
					t.Errorf("%s.owner-call.json controller_request_digest = %q, want %q", opName, controllerRequestDigest, wantDigest)
				}

				call, err := DecodeCall(bytes.NewReader(ownerCall))
				if err != nil {
					t.Fatalf("DecodeCall(%s.owner-call.json): %v", opName, err)
				}
				if reencoded, err := EncodeCall(call); err != nil || !bytes.Equal(reencoded, ownerCall) {
					t.Fatalf("%s.owner-call.json does not re-encode byte-identically (err=%v)", opName, err)
				}

				ownerReply := readPublicContractFixture(t, opName+".owner-reply.json")
				requireOneLineJSON(t, opName+".owner-reply.json", ownerReply)
				reply, err := DecodeReply(bytes.NewReader(ownerReply))
				if err != nil {
					t.Fatalf("DecodeReply(%s.owner-reply.json): %v", opName, err)
				}
				if reencoded, err := EncodeReply(reply); err != nil || !bytes.Equal(reencoded, ownerReply) {
					t.Fatalf("%s.owner-reply.json does not re-encode byte-identically (err=%v)", opName, err)
				}

				legacyRequestNoLF := bytes.TrimSuffix(legacyRequest, []byte("\n"))
				legacyResultNoLF := bytes.TrimSuffix(legacyResult, []byte("\n"))

				callV1 := readPublicContractFixture(t, opName+".call.v1.json")
				requireOneLineJSON(t, opName+".call.v1.json", callV1)
				if want := literalControllerCallV1(opName, legacyRequestNoLF); !bytes.Equal(callV1, want) {
					t.Errorf("%s.call.v1.json does not equal the literal composition of its legacy request arm\n got  %s\n want %s", opName, callV1, want)
				}
				resultV1 := readPublicContractFixture(t, opName+".result.v1.json")
				requireOneLineJSON(t, opName+".result.v1.json", resultV1)
				if want := literalControllerResultV1(opName, legacyResultNoLF); !bytes.Equal(resultV1, want) {
					t.Errorf("%s.result.v1.json does not equal the literal composition of its legacy result arm\n got  %s\n want %s", opName, resultV1, want)
				}

				callV2 := readPublicContractFixture(t, opName+".call.v2.json")
				requireOneLineJSON(t, opName+".call.v2.json", callV2)
				requestArmNoLF := bytes.TrimSuffix(request, []byte("\n"))
				if want := literalControllerCallV2(opName, requestArmNoLF); !bytes.Equal(callV2, want) {
					t.Errorf("%s.call.v2.json does not equal the literal composition of its request arm\n got  %s\n want %s", opName, callV2, want)
				}

				resultV2 := readPublicContractFixture(t, opName+".result.v2.json")
				requireOneLineJSON(t, opName+".result.v2.json", resultV2)
				resultArmNoLF := bytes.TrimSuffix(result, []byte("\n"))
				if want := literalControllerResultV2(opName, resultArmNoLF); !bytes.Equal(resultV2, want) {
					t.Errorf("%s.result.v2.json does not equal the literal composition of its result arm\n got  %s\n want %s", opName, resultV2, want)
				}
			})
		}
	})

	t.Run("resolve_claim_mcp", func(t *testing.T) {
		legacyRequest := readPublicContractFixture(t, "resolve-claim-mcp.legacy-request.json")
		legacyResult := readPublicContractFixture(t, "resolve-claim-mcp.legacy-result.json")
		callV1 := readPublicContractFixture(t, "resolve-claim-mcp.call.v1.json")
		resultV1 := readPublicContractFixture(t, "resolve-claim-mcp.result.v1.json")
		for name, data := range map[string][]byte{
			"resolve-claim-mcp.legacy-request.json": legacyRequest,
			"resolve-claim-mcp.legacy-result.json":  legacyResult,
			"resolve-claim-mcp.call.v1.json":        callV1,
			"resolve-claim-mcp.result.v1.json":      resultV1,
		} {
			requireOneLineJSON(t, name, data)
		}

		requestPayloadNoLF := bytes.TrimSuffix(legacyRequest, []byte("\n"))
		resultPayloadNoLF := bytes.TrimSuffix(legacyResult, []byte("\n"))
		if want := literalControllerCallV1("resolve-claim-mcp", requestPayloadNoLF); !bytes.Equal(callV1, want) {
			t.Errorf("resolve-claim-mcp.call.v1.json does not carry the legacy request payload under the v1 outer schema\n got  %s\n want %s", callV1, want)
		}
		if want := literalControllerResultV1("resolve-claim-mcp", resultPayloadNoLF); !bytes.Equal(resultV1, want) {
			t.Errorf("resolve-claim-mcp.result.v1.json does not carry the legacy result payload under the v1 outer schema\n got  %s\n want %s", resultV1, want)
		}

		callV2 := readPublicContractFixture(t, "resolve-claim-mcp.call.v2.json")
		requireOneLineJSON(t, "resolve-claim-mcp.call.v2.json", callV2)
		if want := literalControllerCallV2("resolve-claim-mcp", requestPayloadNoLF); !bytes.Equal(callV2, want) {
			t.Errorf("resolve-claim-mcp.call.v2.json does not carry the legacy request payload under the v2 outer schema\n got  %s\n want %s", callV2, want)
		}
		if !bytes.Contains(callV2, []byte(`"verdi.context-controller/resolve-claim-mcp-request/v1"`)) {
			t.Errorf("resolve-claim-mcp.call.v2.json does not preserve the controller-prefixed inner wrapper schema")
		}

		resultV2 := readPublicContractFixture(t, "resolve-claim-mcp.result.v2.json")
		requireOneLineJSON(t, "resolve-claim-mcp.result.v2.json", resultV2)
		if want := literalControllerResultV2("resolve-claim-mcp", resultPayloadNoLF); !bytes.Equal(resultV2, want) {
			t.Errorf("resolve-claim-mcp.result.v2.json does not carry the legacy result payload under the v2 outer schema\n got  %s\n want %s", resultV2, want)
		}
		if !bytes.Contains(resultV2, []byte(`"verdi.context-controller/resolve-claim-mcp-result/v1"`)) {
			t.Errorf("resolve-claim-mcp.result.v2.json does not preserve the controller-prefixed inner wrapper schema")
		}
	})

	t.Run("error_frames", func(t *testing.T) {
		// Both frames are the same error document under the same composition
		// rule, differing only in the outer envelope version -- so the v1
		// frame is pinned exactly as tightly as the v2 frame.
		errorMember := []byte(`{"class":"operational","code":"unavailable","schema":"` + controllerErrorSchema + `","witnesses":["fixture controller refusal"]}`)

		v1 := readPublicContractFixture(t, "error.result.v1.json")
		requireOneLineJSON(t, "error.result.v1.json", v1)
		if want := literalControllerResult("next-stamp", controllerResultSchemaV1, "error", errorMember); !bytes.Equal(v1, want) {
			t.Errorf("error.result.v1.json does not equal the fixed literal composition\n got  %s\n want %s", v1, want)
		}

		v2 := readPublicContractFixture(t, "error.result.v2.json")
		requireOneLineJSON(t, "error.result.v2.json", v2)
		if want := literalControllerResult("next-stamp", controllerResultSchemaV2, "error", errorMember); !bytes.Equal(v2, want) {
			t.Errorf("error.result.v2.json does not equal the fixed literal composition\n got  %s\n want %s", v2, want)
		}
	})

	t.Run("resolve_context_nonproven", func(t *testing.T) {
		names := []string{
			"resolve-context.nonproven.legacy-result.json", "resolve-context.nonproven.result.json",
			"resolve-context.nonproven.owner-reply.json", "resolve-context.nonproven.result.v1.json",
			"resolve-context.nonproven.result.v2.json",
		}
		for _, name := range names {
			data := readPublicContractFixture(t, name)
			requireOneLineJSON(t, name, data)
			if bytes.Contains(data, []byte(`"data"`)) {
				t.Errorf("%s unexpectedly names a data member", name)
			}
		}

		ownerReply := readPublicContractFixture(t, "resolve-context.nonproven.owner-reply.json")
		reply, err := DecodeReply(bytes.NewReader(ownerReply))
		if err != nil {
			t.Fatalf("DecodeReply(resolve-context.nonproven.owner-reply.json): %v", err)
		}
		if reencoded, err := EncodeReply(reply); err != nil || !bytes.Equal(reencoded, ownerReply) {
			t.Fatalf("resolve-context.nonproven.owner-reply.json does not re-encode byte-identically (err=%v)", err)
		}

		legacyResult := readPublicContractFixture(t, "resolve-context.nonproven.legacy-result.json")
		result := readPublicContractFixture(t, "resolve-context.nonproven.result.json")
		requireSchemaReplacement(t, legacyResult, result, legacyControllerResultSchemaLiteral("resolve-context"), publicOwnerResultSchemaLiteral("resolve-context"))

		resultV1 := readPublicContractFixture(t, "resolve-context.nonproven.result.v1.json")
		legacyResultNoLF := bytes.TrimSuffix(legacyResult, []byte("\n"))
		if want := literalControllerResultV1("resolve-context", legacyResultNoLF); !bytes.Equal(resultV1, want) {
			t.Errorf("resolve-context.nonproven.result.v1.json does not equal the literal composition of its legacy result arm\n got  %s\n want %s", resultV1, want)
		}

		resultArmNoLF := bytes.TrimSuffix(result, []byte("\n"))
		resultV2 := readPublicContractFixture(t, "resolve-context.nonproven.result.v2.json")
		if want := literalControllerResultV2("resolve-context", resultArmNoLF); !bytes.Equal(resultV2, want) {
			t.Errorf("resolve-context.nonproven.result.v2.json does not equal the literal composition of its result arm\n got  %s\n want %s", resultV2, want)
		}
	})

	// The verify-epoch non-proven-resolution arm is unconditional: contract
	// §2.1 requires a non-proven context resolution to omit data "also when
	// embedded in an epoch check", §6 requires per-arm non-proven coverage,
	// and the baseline accepted all six files. There is no skip and no
	// documented exception.
	t.Run("verify_epoch_nonproven_resolution_variant", func(t *testing.T) {
		legacyRequest := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.legacy-request.json")
		requireOneLineJSON(t, "verify-epoch.nonproven-resolution.legacy-request.json", legacyRequest)
		request := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.request.json")
		requireOneLineJSON(t, "verify-epoch.nonproven-resolution.request.json", request)
		requireSchemaReplacement(t, legacyRequest, request, legacyControllerRequestSchemaLiteral("verify-epoch"), publicOwnerRequestSchemaLiteral("verify-epoch"))

		ownerCall := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.owner-call.json")
		requireOneLineJSON(t, "verify-epoch.nonproven-resolution.owner-call.json", ownerCall)
		call, err := DecodeCall(bytes.NewReader(ownerCall))
		if err != nil {
			t.Fatalf("DecodeCall(verify-epoch.nonproven-resolution.owner-call.json): %v", err)
		}
		if reencoded, err := EncodeCall(call); err != nil || !bytes.Equal(reencoded, ownerCall) {
			t.Fatalf("verify-epoch.nonproven-resolution.owner-call.json does not re-encode byte-identically (err=%v)", err)
		}

		wantDigest := digestBytes(legacyRequest)
		var callMembers map[string]json.RawMessage
		if err := json.Unmarshal(ownerCall, &callMembers); err != nil {
			t.Fatalf("unmarshal verify-epoch.nonproven-resolution.owner-call.json: %v", err)
		}
		var controllerRequestDigest string
		if err := json.Unmarshal(callMembers["controller_request_digest"], &controllerRequestDigest); err != nil {
			t.Fatalf("unmarshal verify-epoch.nonproven-resolution.owner-call.json controller_request_digest: %v", err)
		}
		if controllerRequestDigest != wantDigest {
			t.Errorf("verify-epoch.nonproven-resolution.owner-call.json controller_request_digest = %q, want %q", controllerRequestDigest, wantDigest)
		}

		var digests map[string]string
		if err := json.Unmarshal(readPublicContractFixture(t, "digests.json"), &digests); err != nil {
			t.Fatalf("unmarshal digests.json: %v", err)
		}
		if got := digests["verify-epoch.nonproven-resolution"]; got != wantDigest {
			t.Errorf("digests.json[verify-epoch.nonproven-resolution] = %q, want %q", got, wantDigest)
		}

		ownerReply := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.owner-reply.json")
		requireOneLineJSON(t, "verify-epoch.nonproven-resolution.owner-reply.json", ownerReply)
		reply, err := DecodeReply(bytes.NewReader(ownerReply))
		if err != nil {
			t.Fatalf("DecodeReply(verify-epoch.nonproven-resolution.owner-reply.json): %v", err)
		}
		if reencoded, err := EncodeReply(reply); err != nil || !bytes.Equal(reencoded, ownerReply) {
			t.Fatalf("verify-epoch.nonproven-resolution.owner-reply.json does not re-encode byte-identically (err=%v)", err)
		}

		callV1 := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.call.v1.json")
		requireOneLineJSON(t, "verify-epoch.nonproven-resolution.call.v1.json", callV1)
		legacyRequestNoLF := bytes.TrimSuffix(legacyRequest, []byte("\n"))
		if want := literalControllerCallV1("verify-epoch", legacyRequestNoLF); !bytes.Equal(callV1, want) {
			t.Errorf("verify-epoch.nonproven-resolution.call.v1.json does not equal the literal composition of its legacy request arm\n got  %s\n want %s", callV1, want)
		}

		callV2 := readPublicContractFixture(t, "verify-epoch.nonproven-resolution.call.v2.json")
		requireOneLineJSON(t, "verify-epoch.nonproven-resolution.call.v2.json", callV2)
		requestArmNoLF := bytes.TrimSuffix(request, []byte("\n"))
		if want := literalControllerCallV2("verify-epoch", requestArmNoLF); !bytes.Equal(callV2, want) {
			t.Errorf("verify-epoch.nonproven-resolution.call.v2.json does not equal the literal composition of its request arm\n got  %s\n want %s", callV2, want)
		}
	})

	t.Run("mutation_detects_tamper", func(t *testing.T) {
		tempDir := t.TempDir()
		copyFixtureDir(t, publicContractFixtureDir, tempDir)
		const target = "error.result.v1.json"
		targetPath := filepath.Join(tempDir, target)
		data, err := os.ReadFile(targetPath)
		if err != nil {
			t.Fatalf("read copied fixture: %v", err)
		}
		if len(data) == 0 {
			t.Fatalf("copied fixture %s is empty", target)
		}
		mutated := append([]byte(nil), data...)
		mutated[0] ^= 0xFF
		if err := os.WriteFile(targetPath, mutated, 0o644); err != nil {
			t.Fatalf("write mutated fixture: %v", err)
		}

		err = verifyFixtureDigests(tempDir)
		if err == nil {
			t.Fatalf("verifyFixtureDigests did not detect the tampered byte in %s", target)
		}
		if !strings.Contains(err.Error(), target) {
			t.Fatalf("verifyFixtureDigests error does not name the tampered file %s: %v", target, err)
		}
		if !strings.Contains(err.Error(), "digest mismatch") {
			t.Fatalf("verifyFixtureDigests error is not a digest mismatch: %v", err)
		}

		// The untampered original must still verify clean, proving the
		// checker's positive case is not accidentally vacuous.
		if err := verifyFixtureDigests(publicContractFixtureDir); err != nil {
			t.Fatalf("verifyFixtureDigests reports a mismatch against the untampered fixtures: %v", err)
		}
	})
}
