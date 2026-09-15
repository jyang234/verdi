package main

// Tests for the `--request <path>` half of the import grammar and for the
// bounded request reader's error classification (Task 4 CLI review F1/F2,
// spec-import-contract.md "Errors and browser behavior": `invalid-request`
// and `io-failure` are DISTINCT operational exit-2 codes, mapped by a later
// browser adapter to 400/413 and 500 respectively).
//
// designimport_test.go and designimportapply_test.go drive every preview and
// apply case through `--request -`, so the os.Open branch — the one carrying
// the classification split — never executed. Every built-binary case here
// runs the REAL compiled verdi binary with an EMPTY stdin, so a passing
// positive case proves the bytes actually came from the named file.
//
// Request files are always written under t.TempDir(), never inside a fixture
// repository: the read-only and refusal assertions compare
// designImportRepoStateNow projections that include `status
// --untracked-files=all`, which a request file in the repo would dirty.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specimport"
)

// designImportRequestFile writes data as name inside a fresh temp directory
// outside any fixture repository, and returns its path.
func designImportRequestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// designImportPreviewDigestFromFile runs `preview --request <path>` with an
// empty stdin and returns its ready digest — the file-path counterpart of
// designImportPreviewDigest's stdin chain.
func designImportPreviewDigestFromFile(t *testing.T, bin, root, path string) string {
	t.Helper()
	run := runDesignImportBinary(t, bin, root, nil, nil, "preview", "--request", path)
	if run.code != 0 || run.stderr != "" {
		t.Fatalf("file-path preview for digest = %+v", run)
	}
	var result specimport.PreviewResult
	if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
		t.Fatalf("decoding file-path preview result: %v\n%s", err, run.stdout)
	}
	if !result.Ready {
		t.Fatalf("file-path preview not ready, cannot proceed to apply: %+v", result.Findings)
	}
	return result.Digest
}

// TestDesignImportRequestFileBuiltBinary proves `--request <path>` feeds the
// same bytes into the same algorithms `--request -` does: a file-path preview
// produces the digest the stdin preview produced for identical request bytes
// and leaves the repository untouched, and that digest carries through a
// file-path apply to a created branch and an already-created retry at the
// same commit. Stdin is empty throughout.
func TestDesignImportRequestFileBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)

	t.Run("file-path preview matches the stdin digest and is read-only", func(t *testing.T) {
		root := designImportRepo(t)
		reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, "sample-feature-file-preview"))
		stdinDigest := designImportPreviewDigest(t, bin, root, reqBytes)
		path := designImportRequestFile(t, "request.json", reqBytes)

		before := designImportRepoStateNow(t, root)
		run := runDesignImportBinary(t, bin, root, nil, nil, "preview", "--request", path)
		if run.code != 0 || run.stderr != "" {
			t.Fatalf("file-path preview = %+v", run)
		}
		var result specimport.PreviewResult
		if err := json.Unmarshal([]byte(run.stdout), &result); err != nil {
			t.Fatalf("decoding file-path preview result: %v\n%s", err, run.stdout)
		}
		if result.Schema != specimport.PreviewResultSchema || !result.Ready {
			t.Fatalf("file-path preview result = %+v", result)
		}
		if result.Digest != stdinDigest {
			t.Fatalf("file-path preview digest = %q, want the stdin digest %q for identical request bytes", result.Digest, stdinDigest)
		}
		if after := designImportRepoStateNow(t, root); before != after {
			t.Fatalf("file-path preview changed the repository:\nbefore: %+v\nafter:  %+v", before, after)
		}
	})

	t.Run("file-path apply creates, then retries to already-created", func(t *testing.T) {
		root := designImportPolicyRepo(t, "draft-write")
		slug := "sample-feature-file-apply"
		reqBytes := designImportRequestJSON(t, designImportReadyRequest(t, slug))
		path := designImportRequestFile(t, "request.json", reqBytes)
		digest := designImportPreviewDigestFromFile(t, bin, root, path)

		created := runDesignImportBinary(t, bin, root, nil, nil,
			"apply", "--request", path, "--preview", digest, "--harness", "codex", "--session", "session-1")
		if created.code != 0 || created.stderr != "" {
			t.Fatalf("file-path apply = %+v", created)
		}
		var createdResult specimport.Result
		if err := json.Unmarshal([]byte(created.stdout), &createdResult); err != nil {
			t.Fatalf("decoding file-path apply result: %v\n%s", err, created.stdout)
		}
		if createdResult.Status != specimport.StatusCreated || createdResult.Branch != "design/"+slug {
			t.Fatalf("file-path apply result = %+v", createdResult)
		}
		if !designImportBranchExists(t, root, "design/"+slug) {
			t.Fatalf("design/%s branch was not created by a file-path apply", slug)
		}

		retry := runDesignImportBinary(t, bin, root, nil, nil,
			"apply", "--request", path, "--preview", digest, "--harness", "codex", "--session", "session-1")
		if retry.code != 0 || retry.stderr != "" {
			t.Fatalf("file-path retry = %+v", retry)
		}
		var retryResult specimport.Result
		if err := json.Unmarshal([]byte(retry.stdout), &retryResult); err != nil {
			t.Fatalf("decoding file-path retry result: %v\n%s", err, retry.stdout)
		}
		if retryResult.Status != specimport.StatusAlreadyCreated || retryResult.Commit != createdResult.Commit {
			t.Fatalf("file-path retry result = %+v, want already-created at %s", retryResult, createdResult.Commit)
		}

		// The committed record must read back through the same file-path
		// journey's own slug, proving the created branch is the real one.
		recordRun := runDesignImportBinary(t, bin, root, nil, nil, "record", "--branch", "design/"+slug, "--spec", slug)
		if recordRun.code != 0 || recordRun.stderr != "" {
			t.Fatalf("record after file-path apply = %+v", recordRun)
		}
		var view specimport.RecordView
		if err := json.Unmarshal([]byte(recordRun.stdout), &view); err != nil {
			t.Fatalf("decoding record view: %v\n%s", err, recordRun.stdout)
		}
		if view.ImportCommit != createdResult.Commit || !view.CurrentSpecMatches {
			t.Fatalf("record after file-path apply = %+v, want import_commit %s and a matching current spec", view, createdResult.Commit)
		}
	})
}

// TestDesignImportRequestIOFailureBuiltBinary pins F1: a request file the CLI
// cannot open, read or close is an operational io-failure (exit 2), not
// invalid-request — the same answer `design import source` already gives a
// missing file and `store.FindRoot` already gives an unresolvable root. The
// underlying filesystem detail must survive into the operator message.
func TestDesignImportRequestIOFailureBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	// A syntactically valid digest: apply must fail while READING the
	// request, long before any preview digest is compared.
	digest := strings.Repeat("a", 64)

	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{
			name: "missing file",
			path: func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent.json") },
		},
		{
			name: "directory",
			path: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name: "unreadable file",
			path: func(t *testing.T) string {
				path := designImportRequestFile(t, "unreadable.json", []byte("{}"))
				if err := os.Chmod(path, 0o000); err != nil {
					t.Fatal(err)
				}
				if _, err := os.ReadFile(path); err == nil {
					t.Skip("file modes are not enforced for this user; skipping the unreadable-file case")
				}
				return path
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, op := range []struct {
				name string
				args func(path string) []string
			}{
				{"preview", func(path string) []string { return []string{"preview", "--request", path} }},
				{"apply", func(path string) []string {
					return []string{"apply", "--request", path, "--preview", digest, "--harness", "codex"}
				}},
			} {
				t.Run(op.name, func(t *testing.T) {
					root := designImportRepo(t)
					before := designImportRepoStateNow(t, root)
					path := tc.path(t)

					run := runDesignImportBinary(t, bin, root, nil, nil, op.args(path)...)
					if run.code != 2 || run.stdout != "" {
						t.Fatalf("%s %s = %+v, want exit 2 with empty stdout", op.name, tc.name, run)
					}
					wantPrefix := "design import " + op.name + ": io-failure: "
					if !strings.HasPrefix(run.stderr, wantPrefix) {
						t.Fatalf("%s %s stderr = %q, want prefix %q", op.name, tc.name, run.stderr, wantPrefix)
					}
					if !strings.Contains(run.stderr, path) {
						t.Fatalf("%s %s stderr = %q, want the underlying detail to name %q", op.name, tc.name, run.stderr, path)
					}
					if after := designImportRepoStateNow(t, root); before != after {
						t.Fatalf("%s %s changed the repository:\nbefore: %+v\nafter:  %+v", op.name, tc.name, before, after)
					}
				})
			}
		})
	}
}

// TestDesignImportRequestMalformedFileBuiltBinary pins the other half of F1's
// split: reaching the file successfully and finding a malformed or oversized
// envelope stays invalid-request (exit 2), exactly as the same bytes on stdin
// already do. DecodeRequest wraps specimport.ErrInvalidRequest for both the
// size cap and the strict decode, so the CLI must not relabel either.
func TestDesignImportRequestMalformedFileBuiltBinary(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := designImportRepo(t)

	cases := []struct {
		name string
		data []byte
	}{
		{"null request", []byte("null")},
		{"not json at all", []byte("this is not json")},
		{"unknown field", designImportRequestWithUnknownField(t)},
		{"trailing bytes", designImportRequestWithTrailingBytes(t)},
		{"oversize envelope", bytes.Repeat([]byte("a"), specimport.MaxEnvelopeBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := designImportRequestFile(t, "request.json", tc.data)
			run := runDesignImportBinary(t, bin, root, nil, nil, "preview", "--request", path)
			if run.code != 2 || run.stdout != "" {
				t.Fatalf("%s = %+v, want exit 2 with empty stdout", tc.name, run)
			}
			if !strings.HasPrefix(run.stderr, "design import preview: invalid-request: ") {
				t.Fatalf("%s stderr = %q, want prefix design import preview: invalid-request: ", tc.name, run.stderr)
			}
		})
	}
}

// errDesignImportReadFault stands in for a mid-read stdin fault.
var errDesignImportReadFault = errors.New("simulated read fault")

// failingDesignImportReader fails every Read. A real stdin fault (a closed
// pipe, a vanished device) cannot be forced deterministically through the
// built binary, so this reader covers that seam at the unit level; the
// binary tests above cover the file-path wiring end to end.
type failingDesignImportReader struct{}

func (failingDesignImportReader) Read([]byte) (int, error) { return 0, errDesignImportReadFault }

// TestDesignImportRequestReaderFaultIsIOFailure pins the reader seam's own
// classification and the exact operator line it renders: a reader fault (and
// an unavailable stdin) is io-failure with its detail intact, while an
// oversized envelope from a perfectly healthy reader stays invalid-request.
func TestDesignImportRequestReaderFaultIsIOFailure(t *testing.T) {
	cases := []struct {
		name   string
		reader io.Reader
		want   string
	}{
		{
			name:   "mid-read fault",
			reader: failingDesignImportReader{},
			want:   "design import preview: io-failure: reading request: " + errDesignImportReadFault.Error() + "\n",
		},
		{
			name:   "unavailable stdin",
			reader: nil,
			want:   "design import preview: io-failure: reading request from stdin: stdin is unavailable\n",
		},
		{
			name:   "oversize envelope",
			reader: bytes.NewReader(bytes.Repeat([]byte("a"), specimport.MaxEnvelopeBytes+1)),
			want:   fmt.Sprintf("design import preview: invalid-request: request exceeds %d bytes\n", specimport.MaxEnvelopeBytes),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := readDesignImportRequest("-", tc.reader)
			if err == nil {
				t.Fatalf("readDesignImportRequest returned %d bytes and no error", len(raw))
			}
			var stderr bytes.Buffer
			if exit := renderDesignImportServiceError(&stderr, "preview", err); exit != 2 {
				t.Fatalf("exit = %d, want 2", exit)
			}
			if stderr.String() != tc.want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
			}
		})
	}
}
