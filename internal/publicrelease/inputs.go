package publicrelease

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
)

const baselineVerdi = "8ab423fefa14f6cee3070ca96754928daf28c062"
const baselineATC = "b1f9e995a62aaa2f2ea78248f9b784fd3720817d"
const corpusSHA = "b9074029e486a69bc89e427cd21d99a820dcc74ba5d2a8066f6967de13a19c13"

// Config names actual local inputs. No option accepts a supplied success report.
type Config struct {
	VerdiDir, ATCDir                           string
	VerdiCommit, ATCCommit                     string
	VerdiBinary, ATCBinary                     string
	BaselineSource, BaselineVerdi, BaselineATC string
	OutputDir                                  string
	WorkflowRun                                bool
	BundleSHA                                  string
}

type sourceIdentity struct {
	Commit string            `json:"commit"`
	Tree   string            `json:"tree"`
	Files  map[string]string `json:"files"`
}
type inputs struct {
	CheckerSHA      string            `json:"checker_sha256"`
	BundleSHA       string            `json:"private_atc_bundle_sha256,omitempty"`
	Verdi           sourceIdentity    `json:"verdi"`
	ATC             sourceIdentity    `json:"atc"`
	Binaries        map[string]string `json:"binaries"`
	BaselineSource  map[string]string `json:"baseline_source"`
	Contract        string            `json:"contract_bytes"`
	ContractSHA     string            `json:"contract_sha256"`
	Story           map[string]string `json:"story"`
	CorpusSHA       string            `json:"corpus_sha256"`
	DeclarationsSHA string            `json:"checker_declarations_sha256"`
}

func digest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func fileDigest(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return digest(b), nil
}
func commandBytes(ctx context.Context, dir string, args ...string) ([]byte, error) {
	c := exec.CommandContext(ctx, args[0], args[1:]...)
	c.Dir = dir
	c.Env = childEnvironment()
	b, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", args[0], err)
	}
	return b, nil
}
func source(ctx context.Context, dir, want string) (sourceIdentity, error) {
	out := sourceIdentity{Files: map[string]string{}}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(want) {
		return out, fmt.Errorf("exact approved commit required")
	}
	state, err := commandBytes(ctx, dir, "git", "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return out, err
	}
	if len(state) != 0 {
		return out, fmt.Errorf("source must be clean: %s", dir)
	}
	for _, field := range []struct {
		ref string
		dst *string
	}{{"HEAD", &out.Commit}, {"HEAD^{tree}", &out.Tree}} {
		b, e := commandBytes(ctx, dir, "git", "rev-parse", field.ref)
		if e != nil {
			return out, e
		}
		*field.dst = strings.TrimSpace(string(b))
	}
	if out.Commit != want {
		return out, fmt.Errorf("stale source commit: got %s want %s", out.Commit, want)
	}
	names, err := commandBytes(ctx, dir, "git", "ls-files", "-z")
	if err != nil {
		return out, err
	}
	for _, name := range strings.Split(string(names), "\x00") {
		if name == "" {
			continue
		}
		h, e := fileDigest(filepath.Join(dir, name))
		if e != nil {
			return out, e
		}
		out.Files[name] = h
	}
	return out, nil
}

func inventory(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("nonregular evidence/source entry %s", path)
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		h, e := fileDigest(path)
		if e != nil {
			return e
		}
		out[filepath.ToSlash(rel)] = h
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty inventory %s", root)
	}
	return out, nil
}

func measure(ctx context.Context, c Config) (inputs, error) {
	out := inputs{Binaries: map[string]string{}, Story: map[string]string{}, DeclarationsSHA: digest(checkBytes), CorpusSHA: corpusSHA}
	var err error
	out.BundleSHA = c.BundleSHA
	checker, e := os.Executable()
	if e != nil {
		return out, e
	}
	if out.CheckerSHA, e = fileDigest(checker); e != nil {
		return out, e
	}
	if out.Verdi, err = source(ctx, c.VerdiDir, c.VerdiCommit); err != nil {
		return out, err
	}
	if out.ATC, err = source(ctx, c.ATCDir, c.ATCCommit); err != nil {
		return out, err
	}
	for name, path := range map[string]string{"verdi": c.VerdiBinary, "atc": c.ATCBinary, "baseline_verdi": c.BaselineVerdi, "baseline_atc": c.BaselineATC} {
		h, e := fileDigest(path)
		if e != nil {
			return out, e
		}
		out.Binaries[name] = h
	}
	if out.BaselineSource, err = inventory(c.BaselineSource); err != nil {
		return out, err
	}
	b, err := commandBytes(ctx, c.VerdiDir, c.VerdiBinary, "context", "contract")
	if err != nil {
		return out, err
	}
	out.Contract = string(b)
	out.ContractSHA = digest(b)
	if out.ContractSHA != "21a150a9ca67a77978ff65a6cd7c69dea0d9f3ef8782be60cf5de2792b2a32d8" {
		return out, fmt.Errorf("actual contract differs from ratified v2 bytes")
	}
	paths := []string{".verdi/specs/active/public-execution-contract/spec.md"}
	for n := 1; n <= 5; n++ {
		for _, kind := range []string{"static", "behavioral"} {
			paths = append(paths, fmt.Sprintf(".verdi/obligations/public-execution-contract/ac-%d--%s.md", n, kind))
		}
	}
	for _, path := range paths {
		b, e := os.ReadFile(filepath.Join(c.VerdiDir, path))
		if e != nil {
			return out, e
		}
		accepted, e := commandBytes(ctx, c.VerdiDir, "git", "show", "4502917a8ec5cb4f34f125d9ba71ed493065b8aa:"+path)
		if e != nil {
			return out, e
		}
		if !sameBytes(b, accepted) {
			return out, fmt.Errorf("governing authority changed: %s", path)
		}
		out.Story[path] = digest(b)
	}
	for _, root := range []string{filepath.Join(c.VerdiDir, "internal/contextowner/testdata/public-contract"), filepath.Join(c.ATCDir, "internal/verdiproto/testdata/public-contract")} {
		if err = verifyCorpus(root); err != nil {
			return out, err
		}
	}
	return out, nil
}
func verifyCorpus(root string) error {
	b, err := os.ReadFile(filepath.Join(root, "SHA256SUMS"))
	if err != nil {
		return err
	}
	if digest(b) != corpusSHA {
		return fmt.Errorf("frozen manifest mismatch")
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || filepath.IsAbs(fields[1]) || strings.Contains(fields[1], "..") {
			return fmt.Errorf("bad frozen manifest row")
		}
		h, e := fileDigest(filepath.Join(root, fields[1]))
		if e != nil {
			return e
		}
		if h != fields[0] {
			return fmt.Errorf("frozen fixture changed: %s", fields[1])
		}
		count++
	}
	if count != 241 {
		return fmt.Errorf("frozen fixture inventory count %d", count)
	}
	return nil
}
func unchanged(before, after inputs) error {
	a, e := canonjson.Marshal(before)
	if e != nil {
		return e
	}
	b, e := canonjson.Marshal(after)
	if e != nil {
		return e
	}
	if !sameBytes(a, b) {
		return fmt.Errorf("release inputs changed during execution")
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
