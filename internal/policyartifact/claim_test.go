package policyartifact

import (
	"strings"
	"testing"
)

// validClaim returns a minimal valid configuration claim for mutation in
// table tests.
func validClaim() Claim {
	return Claim{
		ID:          "require-go-version",
		Family:      FamilyConfiguration,
		Operator:    OpAllowedValues,
		Subject:     "go-version",
		Values:      []string{"1.24", "1.25"},
		Scope:       universalScope(),
		Overridable: false,
	}
}

func universalScope() Scope {
	return Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
}

func TestClaimValidate_Happy(t *testing.T) {
	tests := []struct {
		name string
		c    Claim
	}{
		{"allowed-values", validClaim()},
		{"equals single value", func() Claim {
			c := validClaim()
			c.Operator = OpEquals
			c.Values = []string{"1.25"}
			return c
		}()},
		{"minimum with bound", func() Claim {
			c := validClaim()
			c.Operator = OpMinimum
			c.Values = nil
			b := 2
			c.Bound = &b
			return c
		}()},
		{"identity family same-principal", func() Claim {
			c := validClaim()
			c.Family = FamilyIdentity
			c.Operator = OpSamePrincipal
			c.Values = nil
			return c
		}()},
		{"resource family path-read", func() Claim {
			c := validClaim()
			c.Family = FamilyResource
			c.Operator = OpPathRead
			c.Values = []string{"docs/**"}
			return c
		}()},
		{"scoped claim", func() Claim {
			c := validClaim()
			c.Scope = Scope{Phases: []string{"build"}, Environments: []string{"production"}, Paths: []string{"cmd/"}, Refs: []string{"spec/example"}}
			return c
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestClaimValidate_Negative(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Claim)
		wantSub string
	}{
		{"empty id", func(c *Claim) { c.ID = "" }, "id"},
		{"non-kebab id", func(c *Claim) { c.ID = "Bad_ID" }, "kebab"},
		{"unknown family", func(c *Claim) { c.Family = "posture" }, "family"},
		{"unknown operator", func(c *Claim) { c.Operator = "matches-regex" }, "operator"},
		{"empty subject", func(c *Claim) { c.Subject = "" }, "subject"},
		{"equals needs exactly one value", func(c *Claim) { c.Operator = OpEquals }, "exactly one value"},
		{"allowed-values needs values", func(c *Claim) { c.Values = nil }, "at least one value"},
		{"allowed-values duplicate values", func(c *Claim) { c.Values = []string{"a", "a"} }, "duplicate"},
		{"allowed-values empty value", func(c *Claim) { c.Values = []string{""} }, "empty"},
		{"minimum without bound", func(c *Claim) {
			c.Operator = OpMinimum
			c.Values = nil
			c.Bound = nil
		}, "bound"},
		{"minimum with values", func(c *Claim) {
			c.Operator = OpMinimum
			b := 1
			c.Bound = &b
		}, "values"},
		{"bound outside minimum/maximum", func(c *Claim) { b := 1; c.Bound = &b }, "bound"},
		{"same-principal outside identity family", func(c *Claim) {
			c.Operator = OpSamePrincipal
			c.Values = nil
		}, "identity"},
		{"identity family with non-principal operator", func(c *Claim) {
			c.Family = FamilyIdentity
		}, "identity"},
		{"path-read outside resource family", func(c *Claim) {
			c.Operator = OpPathRead
			c.Values = []string{"docs/**"}
		}, "resource"},
		{"same-principal with values", func(c *Claim) {
			c.Family = FamilyIdentity
			c.Operator = OpSamePrincipal
			c.Values = []string{"x"}
		}, "values"},
		{"path-write absolute path", func(c *Claim) {
			c.Family = FamilyResource
			c.Operator = OpPathWrite
			c.Values = []string{"/etc/passwd"}
		}, "absolute"},
		{"path-write escaping path", func(c *Claim) {
			c.Family = FamilyResource
			c.Operator = OpPathWrite
			c.Values = []string{"../outside"}
		}, "escape"},
		{"bad scope phase", func(c *Claim) { c.Scope.Phases = []string{"deploy"} }, "phase"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validClaim()
			tt.mutate(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tt.wantSub)
			}
		})
	}
}

func TestScopeValidate_Negative(t *testing.T) {
	tests := []struct {
		name    string
		s       Scope
		wantSub string
	}{
		{"nil phases", Scope{Environments: []string{}, Paths: []string{}, Refs: []string{}}, "phases"},
		{"nil environments", Scope{Phases: []string{}, Paths: []string{}, Refs: []string{}}, "environments"},
		{"nil paths", Scope{Phases: []string{}, Environments: []string{}, Refs: []string{}}, "paths"},
		{"nil refs", Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}}, "refs"},
		{"unknown phase", func() Scope { s := universalScope(); s.Phases = []string{"ship"}; return s }(), "phase"},
		{"duplicate phase", func() Scope { s := universalScope(); s.Phases = []string{"build", "build"}; return s }(), "duplicate"},
		{"empty environment", func() Scope { s := universalScope(); s.Environments = []string{""}; return s }(), "empty"},
		{"non-kebab environment", func() Scope { s := universalScope(); s.Environments = []string{"Prod"}; return s }(), "kebab"},
		{"absolute path", func() Scope { s := universalScope(); s.Paths = []string{"/abs"}; return s }(), "absolute"},
		{"escaping path", func() Scope { s := universalScope(); s.Paths = []string{"a/../b"}; return s }(), "escape"},
		{"backslash path", func() Scope { s := universalScope(); s.Paths = []string{`a\b`}; return s }(), "backslash"},
		{"dot-segment path", func() Scope { s := universalScope(); s.Paths = []string{"./cmd/"}; return s }(), "canonical"},
		{"inner dot path", func() Scope { s := universalScope(); s.Paths = []string{"a/./b"}; return s }(), "canonical"},
		{"doubled-slash path", func() Scope { s := universalScope(); s.Paths = []string{"cmd//x"}; return s }(), "canonical"},
		{"control-char path", func() Scope { s := universalScope(); s.Paths = []string{"\x00bad"}; return s }(), "control"},
		{"invalid ref", func() Scope { s := universalScope(); s.Refs = []string{"not a ref"}; return s }(), "ref"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("Validate() = %v, want error containing %q", err, tt.wantSub)
			}
		})
	}
}

func TestScopeValidate_Happy(t *testing.T) {
	if err := universalScope().Validate(); err != nil {
		t.Fatalf("universal scope: %v", err)
	}
	s := Scope{
		Phases:       []string{"design", "build", "review"},
		Environments: []string{"local", "production"},
		Paths:        []string{"cmd/", "internal/**"},
		Refs:         []string{"spec/example", "adr/logging"},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("full scope: %v", err)
	}
}

func TestClaimDigest_DeterministicAndOrderInsensitive(t *testing.T) {
	a := validClaim()
	b := validClaim()
	// Same semantic content, different source ordering of the value set.
	b.Values = []string{"1.25", "1.24"}
	normalizeClaim(&a)
	normalizeClaim(&b)

	da, err := ClaimDigest(a)
	if err != nil {
		t.Fatalf("ClaimDigest(a): %v", err)
	}
	db, err := ClaimDigest(b)
	if err != nil {
		t.Fatalf("ClaimDigest(b): %v", err)
	}
	if da != db {
		t.Fatalf("normalized digests differ: %s vs %s", da, db)
	}
	if !strings.HasPrefix(da, "sha256:") {
		t.Fatalf("digest %q does not carry sha256: prefix", da)
	}

	c := validClaim()
	c.Values = []string{"1.26"}
	normalizeClaim(&c)
	dc, err := ClaimDigest(c)
	if err != nil {
		t.Fatalf("ClaimDigest(c): %v", err)
	}
	if dc == da {
		t.Fatalf("different claims share digest %s", dc)
	}
}
