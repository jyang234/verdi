package workbench

import (
	"strings"
	"testing"
)

// TestInReviewChip drives the in-review chip's copy (spec/workbench-redesign
// dc-4; SI-376 (2); BL-177): the open request's number in its forge's
// notation, the lowest of several with the rest counted, and a disclosed
// absence — never an invented number — when no usable id or no known forge
// kind is at hand.
func TestInReviewChip(t *testing.T) {
	const unavailable = "in review · number unavailable"
	tests := []struct {
		name string
		kind ForgeKind
		ids  []string
		want string
		// wantTitle is a fragment the chip's disclosure title must carry;
		// "" means the chip names a number and carries no title.
		wantTitle string
	}{
		{"one pull request on a GitHub store", ForgeGitHub, []string{"42"}, "PR #42 open", ""},
		{"one merge request on a GitLab store", ForgeGitLab, []string{"9"}, "MR !9 open", ""},
		{"several: the lowest id, then the rest counted", ForgeGitHub, []string{"12", "3", "7"}, "PR #3 open · +2", ""},
		{"several on GitLab", ForgeGitLab, []string{"30", "4"}, "MR !4 open · +1", ""},
		{"ids compare as numbers, never as text", ForgeGitHub, []string{"10", "9"}, "PR #9 open · +1", ""},
		{"a numbered request beside an unnumbered one counts it", ForgeGitLab, []string{"", "5"}, "MR !5 open · +1", ""},
		{"an empty id is disclosed, never a bare #", ForgeGitHub, []string{""}, unavailable, "without a usable number"},
		{"an empty id on GitLab is disclosed, never a bare !", ForgeGitLab, []string{""}, unavailable, "without a usable number"},
		{"several, none numbered", ForgeGitHub, []string{"", ""}, unavailable, "without a usable number"},
		{"a zero id is no request number", ForgeGitHub, []string{"0"}, unavailable, "without a usable number"},
		{"a non-numeric id is no number", ForgeGitLab, []string{"mr-9"}, unavailable, "without a usable number"},
		{"a leading zero is not the forge's own form", ForgeGitHub, []string{"07"}, unavailable, "without a usable number"},
		{"a signed id is not the forge's own form", ForgeGitHub, []string{"+7"}, unavailable, "without a usable number"},
		{"an id past uint64 is not readable", ForgeGitHub, []string{"99999999999999999999999"}, unavailable, "without a usable number"},
		{"no forge kind: no notation to write the number in", "", []string{"42"}, unavailable, "names no forge kind"},
		{"an unknown forge kind fails closed", ForgeKind("bitbucket"), []string{"42"}, unavailable, "names no forge kind"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inReviewChip(tt.kind, tt.ids)
			if got.text != tt.want {
				t.Fatalf("text = %q, want %q", got.text, tt.want)
			}
			if tt.wantTitle == "" {
				if got.title != "" {
					t.Fatalf("a numbered chip carries no disclosure; title = %q", got.title)
				}
				return
			}
			if !strings.Contains(got.title, tt.wantTitle) {
				t.Fatalf("title = %q, want it to disclose %q", got.title, tt.wantTitle)
			}
			if strings.ContainsAny(got.text, "#!") {
				t.Fatalf("a disclosed chip must invent no number notation; text = %q", got.text)
			}
		})
	}
}

// TestInReviewChip_NoReviewState: the chip names the request as open and
// nothing more (dc-4: no review state) — no approval, no requested change,
// no pipeline.
func TestInReviewChip_NoReviewState(t *testing.T) {
	for _, kind := range []ForgeKind{ForgeGitHub, ForgeGitLab, ""} {
		for _, ids := range [][]string{{"1"}, {"1", "2"}, {""}} {
			got := strings.ToLower(inReviewChip(kind, ids).text)
			for _, state := range []string{"approved", "changes", "draft", "pending", "passed", "failed", "merged"} {
				if strings.Contains(got, state) {
					t.Errorf("inReviewChip(%q, %v) = %q states a review state (%q)", kind, ids, got, state)
				}
			}
		}
	}
}

// TestLowestRequestNumber: the numerically lowest id in the forges' own
// form, and false when no id is one.
func TestLowestRequestNumber(t *testing.T) {
	tests := []struct {
		name   string
		ids    []string
		want   string
		wantOK bool
	}{
		{"one", []string{"42"}, "42", true},
		{"numeric order, not text order", []string{"100", "20", "3"}, "3", true},
		{"unusable ids are skipped", []string{"", "mr-1", "0", "8"}, "8", true},
		{"the uint64 ceiling is a number", []string{"18446744073709551615"}, "18446744073709551615", true},
		{"none", nil, "", false},
		{"only unusable ids", []string{"", "0", "07", "+7", " 7", "7 ", "1_000", "18446744073709551616"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := lowestRequestNumber(tt.ids)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("lowestRequestNumber(%q) = (%q, %v), want (%q, %v)", tt.ids, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
