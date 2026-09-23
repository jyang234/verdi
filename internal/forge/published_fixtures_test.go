package forge_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Published provider response examples (ruling R-W1-8: provider responses are
// an open contract, so fixtures are the providers' own published examples
// with every field, never a trimmed subset).
//
// testdata/published/github/*.json are the `value` members of
// components/examples in GitHub's official OpenAPI description
// (github/rest-api-description, descriptions/api.github.com/api.github.com.json
// at commit 7026ef612efe011b970e81e96023771886b67b96, retrieved 2026-09-22),
// which is the source docs.github.com renders as each endpoint's example
// response. They are written as two-space-indented JSON with every member and
// value unchanged:
//
//	workflow-run.json                  example "workflow-run": Get a workflow run
//	                                   and Get a workflow run attempt
//	                                   (docs.github.com/rest/actions/workflow-runs)
//	job-paginated.json                 example "job-paginated": List jobs for a
//	                                   workflow run attempt
//	                                   (docs.github.com/rest/actions/workflow-jobs)
//	environment.json                   example "environment": Get an environment
//	                                   (docs.github.com/rest/deployments/environments)
//	environment-approvals-items.json   example "environment-approvals-items": Get
//	                                   the review history for a workflow run
//	                                   (docs.github.com/rest/actions/workflow-runs)
//	pull-request.json                  example "pull-request": Get a pull request
//	                                   (docs.github.com/rest/pulls/pulls)
//	pull-request-review-items.json     example "pull-request-review-items": List
//	                                   reviews for a pull request
//	                                   (docs.github.com/rest/pulls/reviews)
//	pull-request-simple-items.json     example "pull-request-simple-items": List
//	                                   pull requests associated with a commit
//	                                   (docs.github.com/rest/commits/commits)
//	full-repository-default-response.json
//	                                   example "full-repository-default-response":
//	                                   Get a repository
//	                                   (docs.github.com/rest/repos/repos). This
//	                                   is Get a repository's own example; the
//	                                   "full-repository" example is the one
//	                                   GitHub publishes for Update a repository
//	                                   and the create and fork endpoints.
//
// The last two were retrieved from the same commit on 2026-09-23 (plan
// R-PB-2, the merge-record read).
//
// testdata/published/gitlab/*.json are the example responses in GitLab's
// official API documentation (gitlab-org/gitlab doc/api/*.md at commit
// f29843b2c4b8cae1f01e560495320963042713ae, retrieved 2026-09-22):
//
//	merge-request-approvals.json   merge_request_approvals.md, "Retrieve
//	                               approval state for a merge request"
//	                               (GET /projects/:id/merge_requests/:iid/approvals),
//	                               byte for byte
//	merge-request.json             merge_requests.md, "Retrieve a merge
//	                               request" (GET /projects/:id/merge_requests/:iid).
//	                               The published block is not JSON; the only
//	                               repairs are removing its five `// ...`
//	                               annotations and the trailing comma after the
//	                               last member. Every member and value is kept,
//	                               including the duplicated
//	                               `approvals_before_merge` key.
//
// Every fixture is pinned by digest, so an edit to a published example fails
// every test that loads it.
var publishedFixtureDigests = map[string]string{
	"github/environment-approvals-items.json":      "a915c7dbb9936a0263addd83fe3ae3a4629a2c8c0cbbe887f6f24bb8c23aafb4",
	"github/environment.json":                      "b15abc3bc1b3ff59d8a23ed7df119f9e70dcd05cf5f51e26042b2c210cdc7592",
	"github/full-repository-default-response.json": "737f337dbad75f57aac779a8a252943cae2c9d147ca873f426f8cf4299e28020",
	"github/job-paginated.json":                    "173dad604c1c1476130178fc8243a55efca541ad6987743ebb1d26e1d50df6a0",
	"github/pull-request-review-items.json":        "91d4608c277b2113c647922c51c95ee1e111fde867cad98344bc2051b727f31a",
	"github/pull-request-simple-items.json":        "fdb544829605fc3cdd93afb401343ce1916ec443d1d966b6e27ed99fb508a723",
	"github/pull-request.json":                     "9ddeaeacb4261ac334d3122d62287231a565f30cadadc93c325669a6707dc192",
	"github/workflow-run.json":                     "8621daf2a19b1ae8161c9cdef790bf80dd919459f03944540f46d4ed8d71527b",
	"gitlab/merge-request-approvals.json":          "9d5ba2817c13f69d33a01144697cb074293d7373ddd8138dd3e1c402d8605a79",
	"gitlab/merge-request.json":                    "c4ed439df0521e266b8d916fe6e0c52d52c5e25a13105a465b4d53f619b53556",
}

// publishedFixture returns one pinned published example's exact bytes.
func publishedFixture(t *testing.T, name string) []byte {
	t.Helper()
	want, ok := publishedFixtureDigests[name]
	if !ok {
		t.Fatalf("published fixture %q has no pinned digest", name)
	}
	data, err := os.ReadFile(filepath.Join("testdata", "published", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("reading published fixture %s: %v", name, err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("published fixture %s digest = %s, want pinned %s (published examples are never edited)", name, got, want)
	}
	return data
}

// publishedValue decodes one pinned published example into generic JSON,
// keeping numbers exact.
func publishedValue(t *testing.T, name string) any {
	t.Helper()
	return decodeGeneric(t, name, publishedFixture(t, name))
}

func decodeGeneric(t *testing.T, name string, data []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("decoding %s: %v", name, err)
	}
	return value
}

// encodeGeneric renders a generic JSON value without HTML escaping.
func encodeGeneric(t *testing.T, value any) string {
	t.Helper()
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		t.Fatalf("encoding fixture: %v", err)
	}
	return buf.String()
}

// setMember replaces the value of a member the published object already
// carries; a fixture may bind scenario values, never invent members.
func setMember(t *testing.T, object map[string]any, key string, value any) {
	t.Helper()
	if _, ok := object[key]; !ok {
		t.Fatalf("fixture edit sets member %q the published example does not carry", key)
	}
	object[key] = value
}

// asObject asserts a generic JSON value is an object.
func asObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("fixture value is %T, want a JSON object", value)
	}
	return object
}

// asArray asserts a generic JSON value is an array.
func asArray(t *testing.T, value any) []any {
	t.Helper()
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("fixture value is %T, want a JSON array", value)
	}
	return array
}

// jsonKeyPaths lists every member path in a generic JSON value, with array
// elements collapsed to "[]" (so one path covers every element).
func jsonKeyPaths(value any, prefix string, into map[string]struct{}) {
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			path := prefix + "." + k
			into[path] = struct{}{}
			jsonKeyPaths(item, path, into)
		}
	case []any:
		for _, item := range v {
			jsonKeyPaths(item, prefix+"[]", into)
		}
	}
}

// shapeDiff reports the member paths a variant adds to or drops from a
// published example.
func shapeDiff(published, variant any) (added, removed []string) {
	want := map[string]struct{}{}
	got := map[string]struct{}{}
	jsonKeyPaths(published, "", want)
	jsonKeyPaths(variant, "", got)
	for path := range got {
		if _, ok := want[path]; !ok {
			added = append(added, path)
		}
	}
	for path := range want {
		if _, ok := got[path]; !ok {
			removed = append(removed, path)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// publishedShapeViolation reports how a served fixture departs from its
// published example: any member it adds or drops beyond the declared
// exceptions. "" means the fixture keeps every published member and adds
// none.
func publishedShapeViolation(published, variant any, allowedAdded, allowedRemoved []string) string {
	added, removed := shapeDiff(published, variant)
	var problems []string
	if extra := notIn(added, allowedAdded); len(extra) > 0 {
		problems = append(problems, "adds "+strings.Join(extra, ", "))
	}
	if missing := notIn(removed, allowedRemoved); len(missing) > 0 {
		problems = append(problems, "drops "+strings.Join(missing, ", "))
	}
	return strings.Join(problems, "; ")
}

func notIn(values, allowed []string) []string {
	permitted := make(map[string]bool, len(allowed))
	for _, value := range allowed {
		permitted[value] = true
	}
	var out []string
	for _, value := range values {
		if !permitted[value] {
			out = append(out, value)
		}
	}
	return out
}
