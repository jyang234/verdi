package specimport

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalize_PhysicalLineOffsetsCountsUnterminatedFinalLine(t *testing.T) {
	cases := []struct {
		name  string
		data  string
		lines int
	}{
		{"empty", "", 0},
		{"single unterminated", "a", 1},
		{"single terminated", "a\n", 1},
		{"three terminated", "a\nb\nc\n", 3},
		{"three, last unterminated", "a\nb\nc", 3},
		{"one blank line", "\n", 1},
		{"crlf preserved as content", "a\r\nb\r\n", 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			offsets := physicalLineOffsets([]byte(c.data))
			if got := len(offsets) - 1; got != c.lines {
				t.Fatalf("physicalLineOffsets(%q) = %v, total lines %d, want %d", c.data, offsets, got, c.lines)
			}
		})
	}
}

func TestNormalize_SelectLineRangeExtractsExactCRLFSlice(t *testing.T) {
	data := []byte("one\r\ntwo\r\nthree\r\nfour\r\n")
	got, err := selectLineRange(data, 2, 3)
	if err != nil {
		t.Fatalf("selectLineRange: unexpected error: %v", err)
	}
	want := []byte("two\r\nthree\r\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("selectLineRange(2,3) = %q, want %q", got, want)
	}
}

func TestNormalize_SelectLineRangeZeroZeroReturnsWholeSource(t *testing.T) {
	data := []byte("one\ntwo\nthree")
	got, err := selectLineRange(data, 0, 0)
	if err != nil {
		t.Fatalf("selectLineRange: unexpected error: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("selectLineRange(0,0) = %q, want the whole source %q", got, data)
	}
}

func TestNormalize_SelectLineRangeRejectsOutOfRange(t *testing.T) {
	data := []byte("one\ntwo\n")
	if _, err := selectLineRange(data, 1, 5); err == nil {
		t.Fatal("selectLineRange(1,5) over a 2-line source: want an error")
	}
}

// --- ReadSource ---

func writeHermeticFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	full := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return full
}

func TestReadSource_ReadsHermeticRegularFile(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "sample.md", "one\ntwo\nthree\n")

	src, err := ReadSource(context.Background(), dir, "sample.md", 0, 0)
	if err != nil {
		t.Fatalf("ReadSource: unexpected error: %v", err)
	}
	if !bytes.Equal(src.Data, []byte("one\ntwo\nthree\n")) {
		t.Fatalf("ReadSource data = %q", src.Data)
	}
	if src.StartLine != 0 || src.EndLine != 0 {
		t.Fatalf("ReadSource line range = (%d,%d), want (0,0)", src.StartLine, src.EndLine)
	}
	if src.Label != "sample.md" {
		t.Fatalf("ReadSource label = %q, want %q", src.Label, "sample.md")
	}
	if !sourceIDRe.MatchString(src.ID) {
		t.Fatalf("ReadSource id %q does not match the source id grammar", src.ID)
	}
}

func TestReadSource_RecordsRequestedRangeWithoutPreSlicing(t *testing.T) {
	dir := t.TempDir()
	content := "one\ntwo\nthree\nfour\n"
	writeHermeticFile(t, dir, "multi.md", content)

	src, err := ReadSource(context.Background(), dir, "multi.md", 2, 3)
	if err != nil {
		t.Fatalf("ReadSource: unexpected error: %v", err)
	}
	// Data carries the FULL file, not a pre-sliced sub-range: Source has no
	// digest field of its own, so Normalize needs the whole original bytes
	// to compute both an original-file digest and a selected-slice digest
	// (spec-import-contract.md, "Record original file digest, selected
	// digest and original line coordinates").
	if !bytes.Equal(src.Data, []byte(content)) {
		t.Fatalf("ReadSource data = %q, want the full file %q", src.Data, content)
	}
	if src.StartLine != 2 || src.EndLine != 3 {
		t.Fatalf("ReadSource line range = (%d,%d), want (2,3)", src.StartLine, src.EndLine)
	}
}

// TestReadSource_RejectsEmptyFile and TestReadSource_RejectsInvalidUTF8 pin
// the shared source-content seam: the contract's source constraints ("each
// nonempty valid UTF-8 and no more than 2 MiB") apply to what ReadSource
// hands back, not only to what Request.Validate later sees, so a CLI
// operator is refused immediately rather than at preview time.
func TestReadSource_RejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "empty.md", "")

	if _, err := ReadSource(context.Background(), dir, "empty.md", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource on an empty file: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsInvalidUTF8(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "invalid.md", "\xff\xfe not utf-8\n")

	if _, err := ReadSource(context.Background(), dir, "invalid.md", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource on invalid UTF-8 content: got err %v, want ErrInvalidSource", err)
	}
}

// TestReadSource_FallsBackToDeterministicIDForNonASCIIFilename pins main's
// adjudicated bounded choice (task1 adjudication M6/I-127): a valid UTF-8
// file whose basename has no representable [a-z0-9-] form is read, with the
// deterministic fallback id "source" and the ORIGINAL relative path kept as
// Label. There is no ASCII-filename rule anywhere in the contract.
func TestReadSource_FallsBackToDeterministicIDForNonASCIIFilename(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "需求.md", "# 标题\n\ncontent\n")

	src, err := ReadSource(context.Background(), dir, "需求.md", 0, 0)
	if err != nil {
		t.Fatalf("ReadSource on a valid UTF-8 file with a non-ASCII basename: unexpected error: %v", err)
	}
	if src.ID != "source" {
		t.Errorf("ReadSource id = %q, want the deterministic fallback %q", src.ID, "source")
	}
	if !sourceIDRe.MatchString(src.ID) {
		t.Errorf("fallback id %q does not match the source id grammar", src.ID)
	}
	if src.Label != "需求.md" {
		t.Errorf("ReadSource label = %q, want the original relative path preserved", src.Label)
	}
}

// TestNormalize_ValidateStillRejectsTwoFallbackIDSources proves the fallback
// id does not weaken request identity: two such sources collide and fail
// Request.Validate's duplicate-id check exactly as any other duplicate does.
func TestNormalize_ValidateStillRejectsTwoFallbackIDSources(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "需求.md", "first\n")
	writeHermeticFile(t, dir, "仕様.md", "second\n")

	first, err := ReadSource(context.Background(), dir, "需求.md", 0, 0)
	if err != nil {
		t.Fatalf("ReadSource(需求.md): %v", err)
	}
	second, err := ReadSource(context.Background(), dir, "仕様.md", 0, 0)
	if err != nil {
		t.Fatalf("ReadSource(仕様.md): %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("fallback ids = %q and %q, want both to be the same deterministic value", first.ID, second.ID)
	}

	req := minimalRequest()
	req.Primary = first.ID
	req.Sources = []Source{first, second}
	if err := req.Validate(); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Validate with two fallback-id sources: got err %v, want ErrInvalidRequest", err)
	}
}

func TestReadSource_RejectsOutOfRangeLineNumbers(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "short.md", "one\ntwo\n")

	if _, err := ReadSource(context.Background(), dir, "short.md", 1, 99); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource with out-of-range end line: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadSource(context.Background(), dir, "/etc/passwd", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource with absolute path: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsTraversalComponent(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "inside.md", "content\n")
	cases := []string{"../inside.md", "sub/../../inside.md", "./inside.md", "sub/./inside.md"}
	for _, rel := range cases {
		if _, err := ReadSource(context.Background(), dir, rel, 0, 0); !errors.Is(err, ErrInvalidSource) {
			t.Errorf("ReadSource(%q): got err %v, want ErrInvalidSource", rel, err)
		}
	}
}

func TestReadSource_RejectsSymlinkComponent(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	writeHermeticFile(t, outside, "secret.md", "secret\n")

	linkedDir := filepath.Join(dir, "linked")
	if err := os.Symlink(outside, linkedDir); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}

	if _, err := ReadSource(context.Background(), dir, "linked/secret.md", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource through a symlinked directory component: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsSymlinkLeafFile(t *testing.T) {
	dir := t.TempDir()
	real := writeHermeticFile(t, dir, "real.md", "content\n")
	link := filepath.Join(dir, "link.md")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}

	if _, err := ReadSource(context.Background(), dir, "link.md", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource on a symlinked leaf file: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsSymlinkImportRoot(t *testing.T) {
	realRoot := t.TempDir()
	writeHermeticFile(t, realRoot, "sample.md", "content\n")

	parent := t.TempDir()
	linkedRoot := filepath.Join(parent, "linked-root")
	if err := os.Symlink(realRoot, linkedRoot); err != nil {
		t.Skipf("cannot create symlink in this environment: %v", err)
	}

	if _, err := ReadSource(context.Background(), linkedRoot, "sample.md", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource with a symlinked import root: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "adir"), 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	if _, err := ReadSource(context.Background(), dir, "adir", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource naming a directory: got err %v, want ErrInvalidSource", err)
	}
}

func TestReadSource_RejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadSource(context.Background(), dir, "missing.md", 0, 0); !errors.Is(err, ErrIOFailure) {
		t.Fatalf("ReadSource on a missing file: got err %v, want ErrIOFailure", err)
	}
}

func TestReadSource_RejectsCancellation(t *testing.T) {
	dir := t.TempDir()
	writeHermeticFile(t, dir, "sample.md", "content\n")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ReadSource(ctx, dir, "sample.md", 0, 0); !errors.Is(err, ErrIOFailure) {
		t.Fatalf("ReadSource with a canceled context: got err %v, want ErrIOFailure", err)
	}
}

func TestReadSource_RejectsOversizedFileBeforeLineSelection(t *testing.T) {
	dir := t.TempDir()
	big := bytes.Repeat([]byte("a\n"), (MaxSourceBytes/2)+1)
	writeHermeticFile(t, dir, "big.md", string(big))

	// start=end=0 selects the whole file, so a rejection here can only be
	// the size limit, not an out-of-range line request — proving the size
	// check runs (and fails closed) independent of line-range handling.
	if _, err := ReadSource(context.Background(), dir, "big.md", 0, 0); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("ReadSource on an oversized file: got err %v, want ErrInvalidSource", err)
	}
}
