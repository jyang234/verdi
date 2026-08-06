package experiment

import "testing"

func TestValidateDigest(t *testing.T) {
	valid := []string{
		"sha256:" + repeat("a", 64),
		"sha256:" + repeat("0", 64),
	}
	for _, v := range valid {
		if err := ValidateDigest(v); err != nil {
			t.Errorf("ValidateDigest(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{
		"",
		"sha256:" + repeat("a", 63),
		"sha256:" + repeat("a", 65),
		"sha256:" + repeat("A", 64),
		"sha1:" + repeat("a", 40),
		"sha256:" + repeat("g", 64),
	}
	for _, v := range invalid {
		if err := ValidateDigest(v); err == nil {
			t.Errorf("ValidateDigest(%q) = nil, want error", v)
		}
	}
}

func TestValidateID(t *testing.T) {
	valid := []string{"a", "a1", "a-b", "cache-placement-v1", "facts-cache"}
	for _, v := range valid {
		if err := ValidateID(v); err != nil {
			t.Errorf("ValidateID(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{"", "-a", "a-", "a--b", "A", "a_b", "a b", "a.b"}
	for _, v := range invalid {
		if err := ValidateID(v); err == nil {
			t.Errorf("ValidateID(%q) = nil, want error", v)
		}
	}
}

func TestValidateCommit(t *testing.T) {
	valid := []string{repeat("a", 40), repeat("0123456789abcdef", 2) + repeat("a", 8)}
	for _, v := range valid {
		if err := ValidateCommit(v); err != nil {
			t.Errorf("ValidateCommit(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{"", repeat("a", 39), repeat("a", 41), repeat("A", 40), repeat("g", 40)}
	for _, v := range invalid {
		if err := ValidateCommit(v); err == nil {
			t.Errorf("ValidateCommit(%q) = nil, want error", v)
		}
	}
}

func TestValidateUnit(t *testing.T) {
	valid := []string{"ms", "MiB", "requests/s", "%"}
	for _, v := range valid {
		if err := ValidateUnit(v); err != nil {
			t.Errorf("ValidateUnit(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{"", "m s", "ms\t", "ms\n", "m\x00s"}
	for _, v := range invalid {
		if err := ValidateUnit(v); err == nil {
			t.Errorf("ValidateUnit(%q) = nil, want error", v)
		}
	}
}

func TestValidateRepoRelativePath(t *testing.T) {
	valid := []string{"a", "a/b", "internal/experiment/foo.go", "candidates/baseline.patch"}
	for _, v := range valid {
		if err := ValidateRepoRelativePath(v); err != nil {
			t.Errorf("ValidateRepoRelativePath(%q) = %v, want nil", v, err)
		}
	}
	invalid := []string{"", "/a", "a/", "a//b", "../a", "a/../b", ".."}
	for _, v := range invalid {
		if err := ValidateRepoRelativePath(v); err == nil {
			t.Errorf("ValidateRepoRelativePath(%q) = nil, want error", v)
		}
	}
}

func repeat(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
