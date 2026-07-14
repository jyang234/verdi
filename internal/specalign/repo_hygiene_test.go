// Repo hygiene self-audit (spec/fail-loud ac-1 / dc-1). The audit's
// code-health finding: a 21.8 MB compiled e2eharness binary was tracked at
// the repo root, un-ignored, while playwright.config.ts runs the harness
// via `go run`. Removing that one instance is not the fix — dc-1 chose
// specalign, the self-audit home, for a class-level gate: walk every
// git-tracked file and refuse any that starts with a compiled-binary magic
// (Mach-O, ELF, PE), so a future re-introduction of ANY compiled artifact
// fails loud with a witness, not just this one path.
package specalign

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// binaryMagicKind names the compiled-binary family a magic-byte prefix
// identifies. Empty string means "not a recognized compiled-binary magic"
// — classifyBinaryMagic's zero value, not a sentinel worth its own type.
type binaryMagicKind string

// Recognized compiled-binary magics (dc-1: "Mach-O 0xFEEDFACF/0xCAFEBABE,
// ELF 0x7F454C46, PE MZ"), expanded to the full Mach-O family this repo can
// actually produce or encounter: 32-bit and 64-bit, big-endian and the
// byte-swapped form each reads as on a little-endian host (which is what
// this repo's own tracked e2eharness — arm64 Mach-O — carries: CF FA ED
// FE, the swapped 64-bit magic). The fat/universal-binary magic
// (0xCAFEBABE) is also, confusingly, the Java .class file magic; that
// collision is accepted here per dc-1's explicit list — this gate is about
// compiled machine-code artifacts, and a stray .class file tracked in a Go
// repo would be exactly the kind of surprise this check exists to surface.
var magicPrefixes = []struct {
	kind binaryMagicKind
	name string
	b    []byte
}{
	{"macho", "Mach-O 32-bit", []byte{0xFE, 0xED, 0xFA, 0xCE}},
	{"macho", "Mach-O 32-bit (swapped)", []byte{0xCE, 0xFA, 0xED, 0xFE}},
	{"macho", "Mach-O 64-bit", []byte{0xFE, 0xED, 0xFA, 0xCF}},
	{"macho", "Mach-O 64-bit (swapped)", []byte{0xCF, 0xFA, 0xED, 0xFE}},
	{"macho", "Mach-O fat/universal", []byte{0xCA, 0xFE, 0xBA, 0xBE}},
	{"macho", "Mach-O fat/universal (swapped)", []byte{0xBE, 0xBA, 0xFE, 0xCA}},
	{"elf", "ELF", []byte{0x7F, 'E', 'L', 'F'}},
	{"pe", "PE (MZ)", []byte{'M', 'Z'}},
}

// classifyBinaryMagic reports the compiled-binary family prefix identifies,
// or "" if prefix matches none of the recognized magics. prefix may be
// shorter than 4 bytes (e.g. a zero-byte or 1-byte tracked file); such
// inputs never match and classify as "" rather than panicking or padding.
func classifyBinaryMagic(prefix []byte) (kind binaryMagicKind, name string) {
	for _, m := range magicPrefixes {
		if len(prefix) >= len(m.b) && bytes.Equal(prefix[:len(m.b)], m.b) {
			return m.kind, m.name
		}
	}
	return "", ""
}

// TestRepoHygieneClassifyBinaryMagic is the negative-path unit test for the
// magic-byte classifier itself: table-driven over every recognized magic
// (positive) plus non-binary prefixes it must NOT flag (negative) — a text
// file, a PNG (a common non-compiled binary asset this repo could
// legitimately track), a short/empty prefix, and an unrelated 4-byte
// sequence.
func TestRepoHygieneClassifyBinaryMagic(t *testing.T) {
	tests := []struct {
		name     string
		prefix   []byte
		wantKind binaryMagicKind
	}{
		{"empty", nil, ""},
		{"one byte, not MZ", []byte{'M'}, ""},
		{"text prefix", []byte("pack"), ""},
		{"PNG signature", []byte{0x89, 'P', 'N', 'G'}, ""},
		{"unrelated 4 bytes", []byte{0x01, 0x02, 0x03, 0x04}, ""},
		{"YAML frontmatter dash", []byte("---\n"), ""},
		{"Mach-O 32-bit", []byte{0xFE, 0xED, 0xFA, 0xCE}, "macho"},
		{"Mach-O 32-bit swapped", []byte{0xCE, 0xFA, 0xED, 0xFE}, "macho"},
		{"Mach-O 64-bit", []byte{0xFE, 0xED, 0xFA, 0xCF}, "macho"},
		{"Mach-O 64-bit swapped (this repo's tracked e2eharness magic)", []byte{0xCF, 0xFA, 0xED, 0xFE}, "macho"},
		{"Mach-O fat/universal", []byte{0xCA, 0xFE, 0xBA, 0xBE}, "macho"},
		{"Mach-O fat/universal swapped", []byte{0xBE, 0xBA, 0xFE, 0xCA}, "macho"},
		{"ELF", []byte{0x7F, 'E', 'L', 'F'}, "elf"},
		{"PE (MZ), 2 bytes only", []byte{'M', 'Z'}, "pe"},
		{"PE (MZ), 4 bytes", []byte{'M', 'Z', 0x90, 0x00}, "pe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, gotName := classifyBinaryMagic(tt.prefix)
			if gotKind != tt.wantKind {
				t.Errorf("classifyBinaryMagic(%x) kind = %q, want %q (name %q)", tt.prefix, gotKind, tt.wantKind, gotName)
			}
			if tt.wantKind == "" && gotName != "" {
				t.Errorf("classifyBinaryMagic(%x) unexpectedly named %q for a non-matching prefix", tt.prefix, gotName)
			}
			if tt.wantKind != "" && gotName == "" {
				t.Errorf("classifyBinaryMagic(%x) matched kind %q but returned no name", tt.prefix, tt.wantKind)
			}
		})
	}
}

// TestRepoHygieneNoTrackedCompiledBinaries is dc-1's behavioral proof: walk
// every path `git ls-files -z` names at this repo's root (local git only,
// over the checkout already on disk — co-1's "no network in any test") and
// FAIL, naming the offending path as witness, if any tracked file's first
// bytes match a compiled-binary magic. Only the first 4 bytes of each file
// are read (co-1/dc-1: "hermetic and fast") and directories/symlinks are
// skipped — git ls-files lists blobs (including symlink blobs, mode
// 120000), and a symlink's own bytes are a text path, not the linked
// file's content, so classifying them would be meaningless and the target
// they point to (if tracked itself) is independently checked.
func TestRepoHygieneNoTrackedCompiledBinaries(t *testing.T) {
	cmd := exec.Command("git", "-C", verdiRepoRoot, "ls-files", "-z")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git -C %s ls-files -z: %v", verdiRepoRoot, err)
	}
	if len(out) == 0 {
		t.Fatalf("git ls-files returned no tracked paths under %s — that itself is suspicious for this repo and would make this check vacuous", verdiRepoRoot)
	}

	paths := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")

	var offenders []string
	for _, rel := range paths {
		if rel == "" {
			continue
		}
		abs := filepath.Join(verdiRepoRoot, rel)

		info, err := os.Lstat(abs)
		if err != nil {
			t.Fatalf("stat tracked file %q: %v", rel, err)
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}

		f, err := os.Open(abs)
		if err != nil {
			t.Fatalf("open tracked file %q: %v", rel, err)
		}
		prefix := make([]byte, 4)
		n, readErr := f.Read(prefix)
		_ = f.Close()
		if readErr != nil && n == 0 {
			// Empty tracked file: nothing to classify.
			continue
		}

		if kind, name := classifyBinaryMagic(prefix[:n]); kind != "" {
			offenders = append(offenders, fmt.Sprintf("%s (%s)", rel, name))
		}
	}

	if len(offenders) > 0 {
		t.Errorf("tracked compiled-binary file(s) found — a repo check must refuse build output being tracked (spec/fail-loud ac-1): %s", strings.Join(offenders, ", "))
	}
}
