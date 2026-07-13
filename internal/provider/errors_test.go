package provider_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jyang234/verdi/internal/provider"
)

func TestSentinelErrors_DistinctAndWrappable(t *testing.T) {
	sentinels := map[string]error{
		"NotFound":     provider.ErrNotFound,
		"Unauthorized": provider.ErrUnauthorized,
		"Unavailable":  provider.ErrUnavailable,
	}

	for name, want := range sentinels {
		t.Run(name+" wraps and unwraps", func(t *testing.T) {
			wrapped := fmt.Errorf("adapter: issue X-1: %w", want)
			if !errors.Is(wrapped, want) {
				t.Fatalf("errors.Is(wrapped, %v) = false, want true", want)
			}
		})

		for otherName, other := range sentinels {
			if name == otherName {
				continue
			}
			t.Run(name+" is not "+otherName, func(t *testing.T) {
				if errors.Is(want, other) {
					t.Fatalf("errors.Is(%v, %v) = true, want false (sentinels must be distinct)", want, other)
				}
			})
		}
	}
}

func TestSentinelErrors_UnrelatedErrorNegative(t *testing.T) {
	other := errors.New("some other failure")
	if errors.Is(other, provider.ErrNotFound) {
		t.Fatalf("errors.Is(unrelated, ErrNotFound) = true, want false")
	}
}
