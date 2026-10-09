package workbench

import (
	"context"
	"errors"
	stdhtml "html"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specname"
)

// The New story dialog's name grammar (spec/new-story-dialog-v2 ac-1,
// dc-1; SI-369 (5), (6)): the server ships its own name pattern on the
// dialog's name input, and the browser's verdict on that pattern equals
// the create action's own grammar check, ValidateSuccessorName's
// ReasonInvalidName — case by case, with nothing lowercased.

// createNamePattern is the data-pattern the create dialog renders on its
// name input, unescaped as the browser reads it.
func createNamePattern(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	writeCreateDialog(&b, &BoardProjection{})
	input := regexp.MustCompile(`<input id="create-name"[^>]*>`).FindString(b.String())
	if input == "" {
		t.Fatalf("the create dialog renders no #create-name input:\n%s", b.String())
	}
	m := regexp.MustCompile(` data-pattern="([^"]*)"`).FindStringSubmatch(input)
	if m == nil {
		t.Fatalf("#create-name carries no data-pattern: %s", input)
	}
	return stdhtml.UnescapeString(m[1])
}

// TestCreateDialog_ShipsTheServersNamePattern: the name input carries
// specNameRe's source verbatim — the server's pattern, never a copy.
func TestCreateDialog_ShipsTheServersNamePattern(t *testing.T) {
	if got := createNamePattern(t); got != specNameRe.String() {
		t.Fatalf("#create-name data-pattern = %q, want specNameRe %q", got, specNameRe.String())
	}
}

// TestCreateNamePattern_AgreesWithTheServersGrammar: the shipped pattern,
// compiled as the browser compiles it (RE2 and ECMAScript agree on its
// syntax: anchors, one character class, a non-capturing group), accepts a
// name exactly when ValidateSuccessorName does not refuse it as an
// invalid name. Each name is tested as typed: an uppercase letter is a
// grammar break, never lowercased into a valid name (SI-369 (6)).
func TestCreateNamePattern_AgreesWithTheServersGrammar(t *testing.T) {
	browser, err := regexp.Compile(createNamePattern(t))
	if err != nil {
		t.Fatalf("the shipped pattern does not compile: %v", err)
	}
	root := t.TempDir() // an empty store: no collision can refuse a valid name
	for _, tc := range []struct {
		name  string
		typed string
		valid bool
	}{
		{"kebab-case", "quote-pricing", true},
		{"one letter", "a", true},
		{"digits in segments", "v2-quote-3", true},
		{"uppercase", "Quote-pricing", false},
		{"all uppercase", "QUOTE", false},
		{"a fragment #", "quote#ac-1", false},
		{"a pin @", "quote@abc1234", false},
		{"a slash", "quote/pricing", false},
		{"a double hyphen", "quote--pricing", false},
		{"a leading hyphen", "-quote", false},
		{"a trailing hyphen", "quote-", false},
		{"empty", "", false},
		{"unicode", "café-quote", false},
		{"unicode letters only", "ünïcode", false},
		{"a space", "quote pricing", false},
		{"an underscore", "quote_pricing", false},
		{"a trailing newline", "quote\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inBrowser := browser.MatchString(tc.typed)
			_, verr := specname.ValidateSuccessorName(context.Background(), root, tc.typed, "")
			var nerr *specname.NameError
			invalid := errors.As(verr, &nerr) && nerr.Reason == specname.ReasonInvalidName
			if verr != nil && !invalid {
				t.Fatalf("ValidateSuccessorName(%q) refused for another reason: %v", tc.typed, verr)
			}
			if inBrowser != !invalid {
				t.Errorf("%q: the browser pattern says valid=%v, the server's grammar says valid=%v", tc.typed, inBrowser, !invalid)
			}
			if inBrowser != tc.valid {
				t.Errorf("%q: the browser pattern says valid=%v, want %v", tc.typed, inBrowser, tc.valid)
			}
		})
	}
}
