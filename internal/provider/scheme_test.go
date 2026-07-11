package provider_test

import (
	"errors"
	"testing"

	"github.com/OWNER/verdi/internal/provider"
)

func TestParseStoryRef(t *testing.T) {
	cases := []struct {
		name       string
		ref        provider.StoryRef
		wantScheme string
		wantKey    string
		wantErr    error
	}{
		{
			name:       "jira",
			ref:        "jira:LOAN-1482",
			wantScheme: "jira",
			wantKey:    "LOAN-1482",
		},
		{
			name:       "gitlab with hash-suffixed key",
			ref:        "gitlab:platform#482",
			wantScheme: "gitlab",
			wantKey:    "platform#482",
		},
		{
			name:       "scheme with digits",
			ref:        "jira2:X-1",
			wantScheme: "jira2",
			wantKey:    "X-1",
		},
		{
			name:    "missing separator",
			ref:     "jiraLOAN-1482",
			wantErr: provider.ErrInvalidRef,
		},
		{
			name:    "empty scheme",
			ref:     ":LOAN-1482",
			wantErr: provider.ErrInvalidRef,
		},
		{
			name:    "empty key",
			ref:     "jira:",
			wantErr: provider.ErrInvalidRef,
		},
		{
			name:    "uppercase scheme",
			ref:     "JIRA:LOAN-1482",
			wantErr: provider.ErrInvalidRef,
		},
		{
			name:    "scheme starting with digit",
			ref:     "1jira:LOAN-1482",
			wantErr: provider.ErrInvalidRef,
		},
		{
			name:    "scheme with punctuation",
			ref:     "ji-ra:LOAN-1482",
			wantErr: provider.ErrInvalidRef,
		},
		{
			name:    "empty ref",
			ref:     "",
			wantErr: provider.ErrInvalidRef,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheme, key, err := provider.ParseStoryRef(tc.ref)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("ParseStoryRef(%q) error = nil, want %v", tc.ref, tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ParseStoryRef(%q) error = %v, want errors.Is(err, %v)", tc.ref, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseStoryRef(%q) error = %v, want nil", tc.ref, err)
			}
			if scheme != tc.wantScheme || key != tc.wantKey {
				t.Fatalf("ParseStoryRef(%q) = (%q, %q), want (%q, %q)", tc.ref, scheme, key, tc.wantScheme, tc.wantKey)
			}
		})
	}
}
