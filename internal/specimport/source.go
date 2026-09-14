package specimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// physicalLineOffsets returns the byte offset at which each 1-based
// physical line of data begins, plus a trailing sentinel equal to
// len(data): line i (1-based) spans [offsets[i-1], offsets[i]). Lines are
// LF-terminated; an unterminated final line (no trailing '\n') is counted
// as one more line (spec-import-contract.md: "physical LF-terminated
// lines with the unterminated final line counted"). CRLF line endings are
// preserved byte-for-byte: a trailing '\r' immediately before the '\n' is
// simply part of that line's own content, never stripped or normalized
// here.
//
// len(offsets)-1 is the file's total physical line count. Empty data has
// zero lines (offsets == []int{0}).
func physicalLineOffsets(data []byte) []int {
	if len(data) == 0 {
		return []int{0}
	}
	offsets := []int{0}
	for i, b := range data {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	if data[len(data)-1] != '\n' {
		offsets = append(offsets, len(data))
	}
	return offsets
}

// selectLineRange resolves a Source's StartLine/EndLine request against its
// full Data, returning the exact selected byte slice
// (spec-import-contract.md: "Line ranges are inclusive, 1-based, on
// physical LF-terminated lines... Both zero selects the entire source;
// otherwise both must be positive, ordered and within the file. Preserve
// CRLF and selected bytes exactly."). It is shared, unexported machinery:
// Request.Validate calls it to reject an out-of-range or malformed line
// range at decode/construction time, ReadSource calls it to validate the
// range it was asked to record, and Normalize calls it again (never
// trusting prior validation) to compute the actual selected bytes a
// Snapshot's "selected digest" and every downstream structural/mapping
// offset are based on.
func selectLineRange(data []byte, start, end int) ([]byte, error) {
	if start == 0 && end == 0 {
		return data, nil
	}
	if start <= 0 || end <= 0 {
		return nil, fmt.Errorf("start_line and end_line must both be zero or both be positive (got %d, %d)", start, end)
	}
	if start > end {
		return nil, fmt.Errorf("start_line %d must not be after end_line %d", start, end)
	}
	offsets := physicalLineOffsets(data)
	total := len(offsets) - 1
	if end > total {
		return nil, fmt.Errorf("end_line %d exceeds the source's %d physical line(s)", end, total)
	}
	return data[offsets[start-1]:offsets[end]], nil
}

// sha256Hex returns the plain lowercase-hex SHA-256 digest of data — no
// "sha256:" prefix, unlike canonjson.Digest's content-address form, since
// Snapshot.OriginalDigest/Digest are compared directly against the F13
// prototype's pinned source-inventory.json/mechanical-field-map.json
// values, which are bare hex.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// idFromFilename derives a Source.ID candidate from a relative path's base
// filename: lowercase, non [a-z0-9] runs become a single hyphen, leading/
// trailing hyphens trimmed, truncated to the id grammar's 64-byte limit.
// ReadSource's signature (spec-import-contract.md, "Shared internal
// interfaces") takes no id/label parameter, so this package must derive
// something reversible and deterministic; the lane report flags this as a
// semantic decision for main, since the CLI (Task 4) may reasonably want
// to let an operator override it before composing the final Request.
var nonSlugRunRe = regexp.MustCompile(`[^a-z0-9]+`)

func idFromFilename(relativePath string) (string, error) {
	base := filepath.Base(relativePath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	slug := nonSlugRunRe.ReplaceAllString(strings.ToLower(base), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-")
	}
	if !sourceIDRe.MatchString(slug) {
		return "", fmt.Errorf("cannot derive a valid source id from filename %q (got %q)", relativePath, slug)
	}
	return slug, nil
}

// splitRelativePath validates relativePath's shape and returns its
// components: non-empty, not absolute, no backslash (a foreign path
// separator this contract's wire grammar never uses — rejected outright
// rather than silently reinterpreted), and no "", ".", or ".." component
// (spec-import-contract.md: "The source reader rejects absolute paths,
// traversal components... It does not walk directories").
func splitRelativePath(relativePath string) ([]string, error) {
	if relativePath == "" {
		return nil, fmt.Errorf("relative path must not be empty")
	}
	if strings.ContainsRune(relativePath, '\\') {
		return nil, fmt.Errorf("relative path %q must not contain a backslash", relativePath)
	}
	if filepath.IsAbs(relativePath) || strings.HasPrefix(relativePath, "/") {
		return nil, fmt.Errorf("relative path %q must not be absolute", relativePath)
	}
	segments := strings.Split(relativePath, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return nil, fmt.Errorf("relative path component %q in %q is not a plain name", seg, relativePath)
		}
	}
	return segments, nil
}

// lstatSafe Lstats path, classifying a missing path as ErrIOFailure (there
// is nothing unsafe about an absent file; there is simply nothing to
// read) and any other stat failure the same way, so only a component that
// genuinely EXISTS and has the wrong type is ever reported as
// ErrInvalidSource.
func lstatSafe(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s does not exist", ErrIOFailure, path)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: inspecting %s: %v", ErrIOFailure, path, err)
	}
	return info, nil
}

// checkSafeImportRoot proves importRoot itself exists, is a directory and
// is not a symlink (spec-import-contract.md: "symlink components
// (including a symlink import root)"). This deliberately differs from
// internal/constitutionapp's safepath discipline, which leaves its root
// unjudged because its root is a fixed repository checkout the caller
// already trusts; here importRoot is exactly the boundary the operator
// selected and the one thing "stay within the user-selected import root"
// means, so it is judged like every other component.
func checkSafeImportRoot(importRoot string) error {
	info, err := lstatSafe(importRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: import root %q is a symlink", ErrInvalidSource, importRoot)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: import root %q is not a directory", ErrInvalidSource, importRoot)
	}
	return nil
}

// checkSafeRegularFile component-by-component Lstats importRoot joined
// with segments, refusing any symlink component (including the final one)
// and requiring the final component to be an existing regular file, never
// walking into or through a directory it does not need to
// (spec-import-contract.md: "rejects... non-regular files and symlink
// components... It does not walk directories"). It returns the final
// FileInfo on success, so the caller can read the size before deciding to
// read the content.
func checkSafeRegularFile(importRoot string, segments []string) (string, os.FileInfo, error) {
	current := importRoot
	for i, seg := range segments {
		current = filepath.Join(current, seg)
		info, err := lstatSafe(current)
		if err != nil {
			return "", nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("%w: path component %q is a symlink; a source is never read through a link", ErrInvalidSource, current)
		}
		if i == len(segments)-1 {
			if !info.Mode().IsRegular() {
				return "", nil, fmt.Errorf("%w: %q is not a regular file", ErrInvalidSource, current)
			}
			return current, info, nil
		}
		if !info.IsDir() {
			return "", nil, fmt.Errorf("%w: path component %q is not a directory", ErrInvalidSource, current)
		}
	}
	return "", nil, fmt.Errorf("%w: empty relative path", ErrInvalidSource)
}

// ReadSource is the safe, read-only CLI file helper
// (spec-import-contract.md, "Shared internal interfaces"). It reads
// exactly one regular file located by joining importRoot with
// relativePath's validated components, recording startLine/endLine as the
// caller's requested selection.
//
// Data always carries the file's FULL bytes, never a pre-sliced
// sub-range: Source has no digest field of its own
// (spec-import-contract.md's Go type), so Normalize needs the complete
// original bytes to compute both an original-file digest and a
// selected-slice digest from the same Source value — see
// TestReadSource_RecordsRequestedRangeWithoutPreSlicing and the paired
// TestNormalize_SelectLineRange* parity tests. ReadSource still validates
// startLine/endLine against the file it just read (reusing
// selectLineRange, the same helper Normalize uses) so a CLI operator gets
// an immediate, specific refusal rather than a deferred one at preview
// time — Normalize repeats this check regardless, since it never trusts
// a prior validation.
//
// Path safety: importRoot must exist, be a directory and not be a
// symlink; relativePath must be a non-empty, non-absolute, traversal-free
// sequence of plain components, none of which — including the final one
// — may be a symlink; the final component must be an existing regular
// file. No directory is ever listed or walked. The file's size is checked
// against the 2 MiB per-source cap from its Lstat info BEFORE its content
// is read, so an oversized file is refused without first reading it fully
// into memory ("limits apply before line selection").
func ReadSource(ctx context.Context, importRoot, relativePath string, startLine, endLine int) (Source, error) {
	if err := ctx.Err(); err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrIOFailure, err)
	}
	if err := checkSafeImportRoot(importRoot); err != nil {
		return Source{}, err
	}
	segments, err := splitRelativePath(relativePath)
	if err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrInvalidSource, err)
	}
	path, info, err := checkSafeRegularFile(importRoot, segments)
	if err != nil {
		return Source{}, err
	}
	if info.Size() > MaxSourceBytes {
		return Source{}, fmt.Errorf("%w: %s is %d bytes, over the %d byte per-source limit", ErrInvalidSource, path, info.Size(), MaxSourceBytes)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Source{}, fmt.Errorf("%w: reading %s: %v", ErrIOFailure, path, err)
	}
	if err := ctx.Err(); err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrIOFailure, err)
	}
	if _, err := selectLineRange(data, startLine, endLine); err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrInvalidSource, err)
	}

	id, err := idFromFilename(relativePath)
	if err != nil {
		return Source{}, fmt.Errorf("%w: %v", ErrInvalidSource, err)
	}
	return Source{
		ID:        id,
		Label:     relativePath,
		Data:      data,
		StartLine: startLine,
		EndLine:   endLine,
	}, nil
}
