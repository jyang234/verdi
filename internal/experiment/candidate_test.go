package experiment

import "testing"

func TestCandidateValidate(t *testing.T) {
	base := repeat("a", 40)
	otherBase := repeat("b", 40)
	validDigest := "sha256:" + repeat("c", 64)

	tests := []struct {
		name    string
		c       Candidate
		base    string
		wantErr bool
	}{
		{
			name:    "happy path",
			c:       Candidate{ID: "facts-cache", Patch: "candidates/facts-cache.patch", Digest: validDigest, Base: base},
			base:    base,
			wantErr: false,
		},
		{
			name:    "bad id",
			c:       Candidate{ID: "Facts_Cache", Patch: "candidates/Facts_Cache.patch", Digest: validDigest, Base: base},
			base:    base,
			wantErr: true,
		},
		{
			name:    "patch path mismatch",
			c:       Candidate{ID: "facts-cache", Patch: "candidates/other.patch", Digest: validDigest, Base: base},
			base:    base,
			wantErr: true,
		},
		{
			name:    "bad digest",
			c:       Candidate{ID: "facts-cache", Patch: "candidates/facts-cache.patch", Digest: "sha256:not-hex", Base: base},
			base:    base,
			wantErr: true,
		},
		{
			name:    "bad base grammar",
			c:       Candidate{ID: "facts-cache", Patch: "candidates/facts-cache.patch", Digest: validDigest, Base: "not-hex"},
			base:    base,
			wantErr: true,
		},
		{
			name:    "differing base",
			c:       Candidate{ID: "facts-cache", Patch: "candidates/facts-cache.patch", Digest: validDigest, Base: otherBase},
			base:    base,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate(tt.base)
			if (err != nil) != tt.wantErr {
				t.Errorf("Candidate.Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
