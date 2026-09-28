package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/objsupersede"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

// objSupersedeRootCommit is every scenario store's root commit, pinned
// literally (lane L3d review I-2): internal/objsupersede/scenario's own
// wantRootCommit, which every committed frozen stamp names.
const objSupersedeRootCommit = "d49dd630388ff05fe4cd7d4084c785045ba15689"

// objSupersedeGoldenNoMain is the not-a-surface reason the golden expects
// wherever main has no board for a closed object, written out
// independently of the production constant.
const objSupersedeGoldenNoMain = `"no board on main shows a closed object here: spec/closed-feature and spec/closed-story are archived and archived specs have no board (ADJ-39), and a closed object renders on a board only as a reference card on a board whose spec links it (SI-278), which no spec on main does; assert the default branch's absence on the docs pages"`

// objSupersedeGolden is the endpoint's JSON contract per store, written out
// independently of the production table (lane L3d review I-4, M-5;
// re-review I-A, M-C): every key, in order, and every value, with the three
// per-run values replaced by placeholders — {url} (the store's verdi
// serve), {docs} (its docs site), and {docs_commit} (main's commit, checked
// against scenario.Build separately). A renamed json tag, a dropped key, or
// a wrong fact fails here, where the TypeScript consumer would otherwise
// break silently.
var objSupersedeGolden = map[string]string{
	"accepted": `{
  "scenario": "accepted", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "main", "main_branch": "main", "design_branch": "design/successor",
  "successor": "spec/successor", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor#dc-1", "decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": "{docs}a/conflict/successor-closed-feature/"},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor#dc-2", "decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": "{docs}a/conflict/successor-closed-story/"}
  ],
  "boards": {
    "checkout": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "design": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}b/design%2Fsuccessor/board/spec/successor", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/",
    "spec/successor": "{docs}a/spec/successor/document/"
  }
}`,
	"feature-criterion": `{
  "scenario": "feature-criterion", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "main", "main_branch": "main", "design_branch": "design/successor",
  "successor": "spec/successor", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#ac-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor#dc-3", "decision_docs_url": "{docs}a/spec/successor/document/#dc-3",
     "establishing_decision": "spec/successor#dc-3", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-3",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": "{docs}a/conflict/successor-closed-feature/"},
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor#dc-1", "decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": "{docs}a/conflict/successor-closed-feature/"},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor#dc-2", "decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": "{docs}a/conflict/successor-closed-story/"}
  ],
  "boards": {
    "checkout": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "design": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}b/design%2Fsuccessor/board/spec/successor", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/",
    "spec/successor": "{docs}a/spec/successor/document/"
  }
}`,
	"chain": `{
  "scenario": "chain", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "main", "main_branch": "main", "design_branch": "design/successor-v3",
  "successor": "spec/successor-v3", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "spec/successor-v3", "url": "{url}board/spec/successor-v3", "not_a_surface": ""},
     "decision": "spec/successor-v3#dc-1", "decision_docs_url": "{docs}a/spec/successor-v3/document/#dc-1",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": "{docs}a/conflict/successor-closed-feature/"},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor-v3", "url": "{url}board/spec/successor-v3", "not_a_surface": ""},
     "decision": "spec/successor-v3#dc-2", "decision_docs_url": "{docs}a/spec/successor-v3/document/#dc-2",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": "{docs}a/conflict/successor-closed-story/"}
  ],
  "boards": {
    "checkout": {"branch": "main", "spec": "spec/successor-v3", "url": "{url}board/spec/successor-v3", "not_a_surface": ""},
    "design": {"branch": "design/successor-v3", "spec": "spec/successor-v3", "url": "{url}b/design%2Fsuccessor-v3/board/spec/successor-v3", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "spec/successor-v3", "url": "{url}board/spec/successor-v3", "not_a_surface": ""}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/",
    "spec/successor": "{docs}a/spec/successor/document/",
    "spec/successor-v2": "{docs}a/spec/successor-v2/document/",
    "spec/successor-v3": "{docs}a/spec/successor-v3/document/"
  }
}`,
	"chain-drop": `{
  "scenario": "chain-drop", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "main", "main_branch": "main", "design_branch": "design/successor-v2",
  "successor": "spec/successor-v2", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
     "decision": "", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": "{docs}a/conflict/successor-closed-feature/"},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor-v2", "url": "{url}board/spec/successor-v2", "not_a_surface": ""},
     "decision": "spec/successor-v2#dc-2", "decision_docs_url": "{docs}a/spec/successor-v2/document/#dc-2",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": "{docs}a/conflict/successor-closed-story/"}
  ],
  "boards": {
    "checkout": {"branch": "main", "spec": "spec/successor-v2", "url": "{url}board/spec/successor-v2", "not_a_surface": ""},
    "design": {"branch": "design/successor-v2", "spec": "spec/successor-v2", "url": "{url}b/design%2Fsuccessor-v2/board/spec/successor-v2", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "spec/successor-v2", "url": "{url}board/spec/successor-v2", "not_a_surface": ""}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/",
    "spec/successor": "{docs}a/spec/successor/document/",
    "spec/successor-v2": "{docs}a/spec/successor-v2/document/"
  }
}`,
	"proposed": `{
  "scenario": "proposed", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "design/successor", "main_branch": "main", "design_branch": "design/successor",
  "successor": "spec/successor", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `},
     "decision": "spec/successor#dc-1", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": ""},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `},
     "decision": "spec/successor#dc-2", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": ""}
  ],
  "boards": {
    "checkout": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "design": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/"
  }
}`,
	"constraint-target": `{
  "scenario": "constraint-target", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "design/successor", "main_branch": "main", "design_branch": "design/successor",
  "successor": "spec/successor", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#co-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#co-1",
     "object_board": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `},
     "decision": "spec/successor#dc-1", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": ""},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `},
     "decision": "spec/successor#dc-2", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": ""}
  ],
  "boards": {
    "checkout": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "design": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/"
  }
}`,
	"no-conflict": `{
  "scenario": "no-conflict", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "design/successor", "main_branch": "main", "design_branch": "design/successor",
  "successor": "spec/successor", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `},
     "decision": "spec/successor#dc-1", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "",
     "conflict": "", "conflict_docs_url": ""},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `},
     "decision": "spec/successor#dc-2", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": ""}
  ],
  "boards": {
    "checkout": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "design": {"branch": "design/successor", "spec": "spec/successor", "url": "{url}board/spec/successor", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "", "url": "", "not_a_surface": ` + objSupersedeGoldenNoMain + `}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/"
  }
}`,
	"chain-not-in-force": `{
  "scenario": "chain-not-in-force", "url": "{url}", "docs_url": "{docs}", "docs_commit": "{docs_commit}",
  "checkout": "design/successor-v2", "main_branch": "main", "design_branch": "design/successor-v2",
  "successor": "spec/successor-v2", "establishing_successor": "spec/successor",
  "conflicts": ["conflict/successor-closed-feature", "conflict/successor-closed-story"],
  "supersessions": [
    {"object": "spec/closed-feature#ac-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}b/main/board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor-v2#dc-3", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-3", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-3",
     "conflict": "", "conflict_docs_url": ""},
    {"object": "spec/closed-feature#dc-1", "object_docs_url": "{docs}a/spec/closed-feature/document/#dc-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}b/main/board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor-v2#dc-1", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-1", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-1",
     "conflict": "conflict/successor-closed-feature", "conflict_docs_url": "{docs}a/conflict/successor-closed-feature/"},
    {"object": "spec/closed-story#ac-1", "object_docs_url": "{docs}a/spec/closed-story/document/#ac-1",
     "object_board": {"branch": "main", "spec": "spec/successor", "url": "{url}b/main/board/spec/successor", "not_a_surface": ""},
     "decision": "spec/successor-v2#dc-2", "decision_docs_url": "",
     "establishing_decision": "spec/successor#dc-2", "establishing_decision_docs_url": "{docs}a/spec/successor/document/#dc-2",
     "conflict": "conflict/successor-closed-story", "conflict_docs_url": "{docs}a/conflict/successor-closed-story/"}
  ],
  "boards": {
    "checkout": {"branch": "design/successor-v2", "spec": "spec/successor-v2", "url": "{url}board/spec/successor-v2", "not_a_surface": ""},
    "design": {"branch": "design/successor-v2", "spec": "spec/successor-v2", "url": "{url}board/spec/successor-v2", "not_a_surface": ""},
    "main": {"branch": "main", "spec": "spec/successor", "url": "{url}b/main/board/spec/successor", "not_a_surface": ""}
  },
  "docs": {
    "spec/closed-feature": "{docs}a/spec/closed-feature/document/",
    "spec/closed-story": "{docs}a/spec/closed-story/document/",
    "spec/successor": "{docs}a/spec/successor/document/"
  }
}`,
}

// objSupersedeObjectText is each closed object's own text as the committed
// records declare it (testdata/objsupersede/records/specs/closed-*.md) —
// what the docs site must render on the object's document page.
var objSupersedeObjectText = map[string]string{
	"spec/closed-feature#ac-1": "an operator can read the governed records",
	"spec/closed-feature#dc-1": "the governed records are listed newest first",
	"spec/closed-story#ac-1":   "the record list renders every governed record",
}

// objSupersedeBoardTitle is each successor spec's title as its record
// declares it: the board's exact <h1>.
var objSupersedeBoardTitle = map[string]string{
	"spec/successor":    "Successor",
	"spec/successor-v2": "Successor v2",
	"spec/successor-v3": "Successor v3",
}

// compactJSON compacts raw so two renderings of the same JSON compare
// byte for byte.
func compactJSON(t *testing.T, raw []byte) string {
	t.Helper()
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		t.Fatalf("compacting %s: %v", raw, err)
	}
	return b.String()
}

// fakeObjSupersedeStart is a substitutable starter: it counts calls and
// answers with a canned run or an error, the same shape as the other
// subprocess fixtures' fake starters.
type fakeObjSupersedeStart struct {
	calls int
	run   *objSupersedeRun
	err   error
}

func (s *fakeObjSupersedeStart) start(context.Context) (*objSupersedeRun, error) {
	s.calls++
	return s.run, s.err
}

func getObjSupersedeFixture(t *testing.T, f *objSupersedeFixture, method string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/objsupersede-fixture", nil)
	rec := httptest.NewRecorder()
	f.handler(rec, req)
	return rec
}

func canonicalObjSupersedeRun() *objSupersedeRun {
	return &objSupersedeRun{stores: map[string]*objSupersedeStore{
		"accepted": {info: objSupersedeStoreInfo{Scenario: "accepted", URL: "http://127.0.0.1:41001/", Checkout: "main"}},
	}}
}

// TestObjSupersedeFixture_Handler_LazyStartOnce pins the handler's
// contract against a fake starter: nothing starts until the first GET,
// the body is the started stores' JSON, and every later GET returns the
// same info without starting again.
func TestObjSupersedeFixture_Handler_LazyStartOnce(t *testing.T) {
	fake := &fakeObjSupersedeStart{run: canonicalObjSupersedeRun()}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start
	if fake.calls != 0 {
		t.Fatalf("constructing the fixture started it (%d calls)", fake.calls)
	}

	first := getObjSupersedeFixture(t, f, http.MethodGet)
	if first.Code != http.StatusOK {
		t.Fatalf("first GET status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	if ct := first.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var info objSupersedeFixtureInfo
	if err := json.Unmarshal(first.Body.Bytes(), &info); err != nil {
		t.Fatalf("decoding body: %v: %s", err, first.Body.String())
	}
	if len(info.Stores) != 1 || info.Stores["accepted"].URL != "http://127.0.0.1:41001/" {
		t.Fatalf("info = %+v, want one store named accepted with the canned URL", info)
	}

	second := getObjSupersedeFixture(t, f, http.MethodGet)
	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("second GET = %d %q, want 200 and the same body %q", second.Code, second.Body.String(), first.Body.String())
	}
	if fake.calls != 1 {
		t.Fatalf("starter ran %d times across two GETs, want exactly once", fake.calls)
	}
}

// TestObjSupersedeFixture_Handler_Negative_StartFails: a failing start is
// a disclosed 500 naming the cause, caches nothing (the next GET
// retries), and a starter that returns no stores is refused the same way.
func TestObjSupersedeFixture_Handler_Negative_StartFails(t *testing.T) {
	fake := &fakeObjSupersedeStart{err: errors.New("materializing objsupersede scenario \"accepted\": boom")}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start

	rec := getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "boom") {
		t.Fatalf("body = %q, want the start error disclosed", rec.Body.String())
	}
	rec = getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError || fake.calls != 2 {
		t.Fatalf("retry: status = %d, starter calls = %d; want 500 and a second attempt", rec.Code, fake.calls)
	}

	for _, run := range []*objSupersedeRun{nil, {}} {
		fake.err, fake.run = nil, run
		rec = getObjSupersedeFixture(t, f, http.MethodGet)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "no stores") {
			t.Fatalf("run %+v: status = %d body = %q, want 500 naming the missing stores", run, rec.Code, rec.Body.String())
		}
	}
	f.stop() // never started: safe
}

// TestObjSupersedeFixture_ZeroValue_DefaultsToRealStart: a struct literal
// with no starter never nil-panics — ensureStarted defaults to the real
// sequence, which here discloses its own failure as a 500 (the module
// root carries no scenario fixture) and leaves no scratch behind, and
// stop stays safe.
func TestObjSupersedeFixture_ZeroValue_DefaultsToRealStart(t *testing.T) {
	tmp := t.TempDir()
	f := &objSupersedeFixture{moduleRoot: t.TempDir(), tmpRoot: tmp}
	t.Cleanup(f.stop)
	rec := getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "loading the objsupersede scenario manifest") {
		t.Fatalf("status = %d body = %q, want 500 naming the manifest", rec.Code, rec.Body.String())
	}
	if f.start == nil {
		t.Fatal("ensureStarted left start unset")
	}
	if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
		t.Fatalf("a failed start left scratch behind: %v", entries)
	}
	var zero objSupersedeFixture
	zero.stop() // never started, no starter: safe
}

// TestObjSupersedeFixture_Handler_Negative_WrongMethod: a non-GET request
// is refused before any start.
func TestObjSupersedeFixture_Handler_Negative_WrongMethod(t *testing.T) {
	fake := &fakeObjSupersedeStart{run: canonicalObjSupersedeRun()}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start
	rec := getObjSupersedeFixture(t, f, http.MethodPost)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if fake.calls != 0 {
		t.Fatalf("a refused POST started the fixture (%d calls)", fake.calls)
	}
}

// fakeObjSupersedeProc is a stand-in serve whose cancel counts its calls
// and "exits" at once.
func fakeObjSupersedeProc(calls *int) *objSupersedeProc {
	p := &objSupersedeProc{exited: make(chan struct{})}
	p.cancel = func() { *calls++; close(p.exited) }
	return p
}

// TestObjSupersedeFixture_Stop pins stop's contract: it cancels every
// store's serve and waits for its exit, closes every docs site, removes
// the run's scratch directory, and is idempotent.
func TestObjSupersedeFixture_Stop(t *testing.T) {
	var callsA, callsB int
	scratch := t.TempDir()
	site, err := serveObjSupersedeSite(objSupersedeLoopback, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := &objSupersedeRun{scratch: scratch, stores: map[string]*objSupersedeStore{
		"accepted": {info: objSupersedeStoreInfo{Scenario: "accepted"}, serve: fakeObjSupersedeProc(&callsA), site: site},
		"chain":    {info: objSupersedeStoreInfo{Scenario: "chain"}, serve: fakeObjSupersedeProc(&callsB)},
	}}
	siteURL := site.url
	fake := &fakeObjSupersedeStart{run: run}
	f := newObjSupersedeFixture(testModuleRoot)
	f.start = fake.start
	if rec := getObjSupersedeFixture(t, f, http.MethodGet); rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200", rec.Code)
	}

	f.stop()
	if callsA != 1 || callsB != 1 {
		t.Fatalf("cancel calls = %d, %d; want each store's serve cancelled exactly once", callsA, callsB)
	}
	if _, err := http.Get(siteURL); err == nil {
		t.Fatalf("docs site %s still answers after stop", siteURL)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("scratch %s survived stop: %v", scratch, err)
	}
	f.stop()
	if callsA != 1 || callsB != 1 {
		t.Fatalf("second stop cancelled again: %d, %d; want idempotent", callsA, callsB)
	}
}

// TestControlServer_WiresObjSupersedeFixture proves the endpoint is
// mounted on the control server's own mux (no subprocess: the method
// guard answers first).
func TestControlServer_WiresObjSupersedeFixture(t *testing.T) {
	c := newControlServer(t.TempDir(), testModuleRoot, "")
	t.Cleanup(c.objSupersede.stop)
	req := httptest.NewRequest(http.MethodPost, "/objsupersede-fixture", nil)
	rec := httptest.NewRecorder()
	c.handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /objsupersede-fixture = %d, want 405 (route mounted)", rec.Code)
	}
}

// TestObjSupersedeStoreInfo_MatchesGolden pins newObjSupersedeStoreInfo,
// the pure assembly of every store's JSON, against the independent golden
// — each store's checkout read from the committed manifest, the URLs and
// commit passed as the golden's own placeholders — and pins that the
// provisioned store list, the facts table, and the golden name the same
// eight stores. Negative: an unknown store has no facts.
func TestObjSupersedeStoreInfo_MatchesGolden(t *testing.T) {
	m, err := scenario.Load(filepath.Join(testModuleRoot, "testdata", "objsupersede"))
	if err != nil {
		t.Fatal(err)
	}
	var golden, facts []string
	for name := range objSupersedeGolden {
		golden = append(golden, name)
	}
	for name := range objSupersedeFactsByStore {
		facts = append(facts, name)
	}
	stores := slices.Clone(objSupersedeStores)
	sort.Strings(golden)
	sort.Strings(facts)
	sort.Strings(stores)
	if !reflect.DeepEqual(stores, golden) || !reflect.DeepEqual(facts, golden) {
		t.Fatalf("stores %v, facts %v, golden %v: want the same eight", stores, facts, golden)
	}
	for _, name := range objSupersedeStores {
		info, err := newObjSupersedeStoreInfo(name, m.Scenarios[name].Checkout, m.Commit.InitialBranch, "{docs_commit}", "{url}", "{docs}")
		if err != nil {
			t.Fatalf("store %q: %v", name, err)
		}
		got, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		if g, w := compactJSON(t, got), compactJSON(t, []byte(objSupersedeGolden[name])); g != w {
			t.Errorf("store %q:\n got %s\nwant %s", name, g, w)
		}
	}
	if _, err := newObjSupersedeStoreInfo("no-such-store", "main", "main", "c", "u", "d"); err == nil {
		t.Fatal("an unknown store assembled info")
	}
}

// TestExcludeObjSupersedeDataZone: the data zone is excluded through the
// repository's own .git/info/exclude — nothing committed, so no SHA moves
// — so a serve's lock files read as a clean checkout (lane L3d review
// I-3). Negative: a directory that is no git repository is an error.
func TestExcludeObjSupersedeDataZone(t *testing.T) {
	ctx := context.Background()
	repo := scenario.Build(t, "accepted").Dir
	head, _ := gitOutput(ctx, repo, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(repo, ".verdi", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".verdi", "data", "writer.lock"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if porcelain, _ := gitOutput(ctx, repo, "status", "--porcelain", "--untracked-files=all"); porcelain == "" {
		t.Fatal("control: the data file should read as untracked before the exclusion")
	}
	if err := excludeObjSupersedeDataZone(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if porcelain, _ := gitOutput(ctx, repo, "status", "--porcelain", "--untracked-files=all"); porcelain != "" {
		t.Fatalf("status after excluding the data zone = %q, want clean", porcelain)
	}
	if after, _ := gitOutput(ctx, repo, "rev-parse", "HEAD"); after != head {
		t.Fatalf("HEAD moved from %s to %s: the exclusion must not commit", head, after)
	}
	if err := excludeObjSupersedeDataZone(ctx, t.TempDir()); err == nil {
		t.Fatal("excluding the data zone of a non-repository succeeded")
	}
}

// TestServeObjSupersedeSite serves a built directory on loopback until
// stop, which is idempotent and nil-safe. Negative: a page the site does
// not have is a 404, and an address already taken is a listen error with
// nothing served.
func TestServeObjSupersedeSite(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("site home"), 0o644); err != nil {
		t.Fatal(err)
	}
	site, err := serveObjSupersedeSite(objSupersedeLoopback, dir)
	if err != nil {
		t.Fatal(err)
	}
	taken := strings.TrimSuffix(strings.TrimPrefix(site.url, "http://"), "/")
	if again, err := serveObjSupersedeSite(taken, dir); err == nil {
		again.stop()
		t.Fatalf("listening on the taken address %s succeeded", taken)
	}
	t.Cleanup(site.stop)
	if !strings.HasPrefix(site.url, "http://127.0.0.1:") || !strings.HasSuffix(site.url, "/") {
		t.Fatalf("url = %q, want a loopback base URL", site.url)
	}
	if status, body := httpGetBody(t, site.url); status != http.StatusOK || body != "site home" {
		t.Fatalf("GET / = %d %q", status, body)
	}
	if status, _ := httpGetBody(t, site.url+"a/spec/nowhere/"); status != http.StatusNotFound {
		t.Fatalf("GET a missing page = %d, want 404", status)
	}
	site.stop()
	site.stop()
	if _, err := http.Get(site.url); err == nil {
		t.Fatal("the site still answers after stop")
	}
	var none *objSupersedeSite
	none.stop()
}

// TestStartObjSupersedeServe_Negative: a binary that cannot start is an
// error, and one that exits before answering healthz fails at once —
// naming the exit — rather than after the whole readiness wait.
func TestStartObjSupersedeServe_Negative(t *testing.T) {
	ctx := context.Background()
	if _, err := startObjSupersedeServe(ctx, filepath.Join(t.TempDir(), "no-such-verdi"), t.TempDir()); err == nil {
		t.Fatal("starting a missing binary succeeded")
	}
	falseBin, err := exec.LookPath("false")
	if err != nil {
		t.Fatalf("no false binary: %v", err)
	}
	began := time.Now()
	_, err = startObjSupersedeServe(ctx, falseBin, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "exited before answering healthz") {
		t.Fatalf("err = %v, want the early exit named", err)
	}
	if took := time.Since(began); took > 10*time.Second {
		t.Fatalf("an exited serve took %s to fail, want well under the 20s readiness wait", took)
	}
	var none *objSupersedeProc
	none.stop()
}

// copyObjSupersedeBinary returns a buildBinary step that installs the
// already-built bin at out instead of running go build again.
func copyObjSupersedeBinary(bin string) func(context.Context, string, string) error {
	return func(_ context.Context, _, out string) error {
		if err := os.Link(bin, out); err == nil {
			return nil
		}
		data, err := os.ReadFile(bin)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0o755)
	}
}

// buildObjSupersedeTestBinary builds the verdi binary once for a test,
// into its own temporary directory.
func buildObjSupersedeTestBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "verdi")
	if err := buildBinary(context.Background(), absModuleRoot(t), bin); err != nil {
		t.Fatal(err)
	}
	return bin
}

// TestObjSupersedeFixture_StartAll_Negative drives the start's failure paths
// through ensureStarted — the handler's own call, with the request's
// context — using the real steps (lane L3d review I-5, re-review M-B): each
// row fails at a chosen store AFTER the earlier stores are fully up (real
// `verdi serve` subprocesses and real docs sites), and proves the partial
// failure reaps every one of them — the failing store's own included —
// removes the run's scratch, and caches nothing. It also proves the
// fixture reads its scenario data from moduleRoot, not from the source
// file's location (review M-2: a -trimpath build would lose the latter).
func TestObjSupersedeFixture_StartAll_Negative(t *testing.T) {
	neutralizeCIEnvForTest(t)
	moduleRoot := absModuleRoot(t)
	bin := buildObjSupersedeTestBinary(t)
	falseBin, err := exec.LookPath("false")
	if err != nil {
		t.Fatalf("no false binary: %v", err)
	}
	// A serve that stays alive but never answers: it records its pid, then
	// becomes a sleep.
	pidFile := filepath.Join(t.TempDir(), "sleeper.pid")
	sleeper := filepath.Join(t.TempDir(), "sleeper")
	if err := os.WriteFile(sleeper, []byte("#!/bin/sh\necho $$ > '"+pidFile+"'\nexec sleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// An address a docs site cannot take: this test holds it.
	held, err := net.Listen("tcp", objSupersedeLoopback)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = held.Close() })
	// storeOf names the store a step's directory belongs to:
	// <scratch>/<store>/{repo,main,site}.
	storeOf := func(dir string) string { return filepath.Base(filepath.Dir(dir)) }

	tests := []struct {
		name       string
		moduleRoot string
		stores     []string
		bad        func(s *objSupersedeSteps, cancel context.CancelFunc)
		wants      []string
		wantServes int // serves and sites that came up: reaping must not be vacuous
		wantSites  int
		after      func(t *testing.T)
	}{
		{name: "the manifest is read from moduleRoot", moduleRoot: t.TempDir(), wants: []string{"loading the objsupersede scenario manifest"}},
		{name: "the binary build fails", bad: func(s *objSupersedeSteps, _ context.CancelFunc) {
			s.buildBinary = func(context.Context, string, string) error { return errors.New("boom") }
		}, wants: []string{"building verdi binary for the objsupersede fixture: boom"}},
		{name: "a store is missing from the manifest", stores: []string{"accepted", "no-such-scenario"},
			wants: []string{`scenario "no-such-scenario" is not defined in the manifest`}, wantServes: 1, wantSites: 1},
		{name: "materialize fails", stores: []string{"accepted", "chain"}, bad: func(s *objSupersedeSteps, _ context.CancelFunc) {
			s.materialize = func(ctx context.Context, fixtureDir, repoDir, name string) (*scenario.Repo, error) {
				if name == "chain" {
					repoDir = filepath.Join(repoDir, "missing") // the real replay, into a directory that does not exist
				}
				return scenario.Materialize(ctx, fixtureDir, repoDir, name)
			}
		}, wants: []string{`materializing objsupersede scenario "chain"`}, wantServes: 1, wantSites: 1},
		{name: "the docs build fails", stores: []string{"accepted", "chain"}, bad: func(s *objSupersedeSteps, _ context.CancelFunc) {
			s.buildSite = func(ctx context.Context, opts dex.Options) error {
				if storeOf(opts.Root) == "chain" {
					opts.Commit = "refs/heads/no-such-branch" // the real build, at a commit that does not resolve
				}
				return dex.Build(ctx, opts)
			}
		}, wants: []string{`building the docs site for objsupersede scenario "chain"`}, wantServes: 1, wantSites: 1},
		{name: "the docs site cannot listen", stores: []string{"accepted", "chain"}, bad: func(s *objSupersedeSteps, _ context.CancelFunc) {
			serve := s.serveSite
			s.serveSite = func(dir string) (*objSupersedeSite, error) {
				if storeOf(dir) == "chain" {
					return serveObjSupersedeSite(held.Addr().String(), dir) // the real listen, on a taken address
				}
				return serve(dir)
			}
		}, wants: []string{`serving the docs site for objsupersede scenario "chain"`, "address already in use"}, wantServes: 1, wantSites: 1},
		{name: "the serve exits before healthz", stores: []string{"accepted", "chain"}, bad: func(s *objSupersedeSteps, _ context.CancelFunc) {
			start := s.startServe
			s.startServe = func(ctx context.Context, binPath, root string) (*objSupersedeProc, error) {
				if storeOf(root) == "chain" {
					binPath = falseBin // the real start, of a binary that exits at once
				}
				return start(ctx, binPath, root)
			}
		}, wants: []string{`starting verdi serve for objsupersede scenario "chain": verdi serve exited before answering healthz`}, wantServes: 1, wantSites: 2},
		{name: "the serve stays alive but never answers", stores: []string{"accepted", "chain"}, bad: func(s *objSupersedeSteps, _ context.CancelFunc) {
			start := s.startServe
			s.startServe = func(ctx context.Context, binPath, root string) (*objSupersedeProc, error) {
				if storeOf(root) != "chain" {
					return start(ctx, binPath, root)
				}
				short, cancel := context.WithTimeout(ctx, time.Second)
				defer cancel()
				return startObjSupersedeServe(short, sleeper, root) // the real start, of a child that never answers
			}
		}, wants: []string{`starting verdi serve for objsupersede scenario "chain": waiting for healthz`}, wantServes: 1, wantSites: 2,
			after: func(t *testing.T) {
				raw, err := os.ReadFile(pidFile)
				if err != nil {
					t.Fatalf("the sleeper never ran: %v", err)
				}
				pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
				if err != nil {
					t.Fatal(err)
				}
				if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
					t.Errorf("the never-healthy serve (pid %d) was not reaped: kill -0 = %v", pid, err)
				}
			}},
		{name: "a store fails after its own serve is up", stores: []string{"accepted", "conflict-open"},
			wants: []string{`store "conflict-open" has no recorded facts`}, wantServes: 2, wantSites: 2},
		{name: "the request is cancelled mid-start", stores: []string{"accepted", "chain"}, bad: func(s *objSupersedeSteps, cancel context.CancelFunc) {
			start := s.startServe
			s.startServe = func(ctx context.Context, binPath, root string) (*objSupersedeProc, error) {
				p, err := start(ctx, binPath, root)
				cancel() // the request goes away once the first store is up
				return p, err
			}
		}, wants: []string{`objsupersede scenario "chain"`, "context canceled"}, wantServes: 1, wantSites: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tmp := t.TempDir()
			var serves []*objSupersedeProc
			var sites []*objSupersedeSite
			steps := objSupersedeSteps{
				buildBinary: copyObjSupersedeBinary(bin),
				serveSite: func(dir string) (*objSupersedeSite, error) {
					s, err := serveObjSupersedeSite(objSupersedeLoopback, dir)
					if err == nil {
						sites = append(sites, s)
					}
					return s, err
				},
				startServe: func(ctx context.Context, binPath, root string) (*objSupersedeProc, error) {
					p, err := startObjSupersedeServe(ctx, binPath, root)
					if err == nil {
						serves = append(serves, p)
					}
					return p, err
				},
			}
			if tc.bad != nil {
				tc.bad(&steps, cancel)
			}
			root := moduleRoot
			if tc.moduleRoot != "" {
				root = tc.moduleRoot
			}
			f := newObjSupersedeFixture(root)
			f.names, f.tmpRoot, f.steps = tc.stores, tmp, steps
			t.Cleanup(f.stop) // reaps a start that wrongly succeeds (ensureStarted caches it)

			info, err := f.ensureStarted(ctx)
			if err == nil {
				t.Fatalf("ensureStarted = %+v; want an error containing %q", info, tc.wants)
			}
			for _, want := range tc.wants {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("err = %v; want it to contain %q", err, want)
				}
			}
			if f.run != nil {
				t.Errorf("a failed start was cached: %+v", f.run)
			}
			if len(serves) != tc.wantServes || len(sites) != tc.wantSites {
				t.Fatalf("%d serves and %d docs sites came up before the failure, want %d and %d", len(serves), len(sites), tc.wantServes, tc.wantSites)
			}
			for _, p := range serves {
				select {
				case <-p.exited:
				default:
					t.Errorf("serve pid %d (%s) is still running after the failed start", p.pid, p.url)
				}
				if _, err := http.Get(p.url + "healthz"); err == nil {
					t.Errorf("serve %s still answers after the failed start", p.url)
				}
			}
			for _, s := range sites {
				if _, err := http.Get(s.url); err == nil {
					t.Errorf("docs site %s still answers after the failed start", s.url)
				}
			}
			if entries, _ := os.ReadDir(tmp); len(entries) != 0 {
				t.Errorf("the failed start left scratch behind: %v", entries)
			}
			if tc.after != nil {
				tc.after(t)
			}
		})
	}
}

// TestObjSupersedeFixture_HermeticServeEnv: the serves run under
// hermeticServeEnv, as the sibling fixtures' do (re-review M-A) — an ambient
// VERDI_REVIEW_FEED naming a missing file does not break the start, and a
// valid foreign feed is never loaded: the board it names stays out of
// review mode and never shows the feed's comment.
func TestObjSupersedeFixture_HermeticServeEnv(t *testing.T) {
	neutralizeCIEnvForTest(t)
	t.Setenv("GITHUB_TOKEN", "") // no live forge may outrank the canned feed
	bin := buildObjSupersedeTestBinary(t)
	feed := filepath.Join(t.TempDir(), "review-feed.json")
	const marker = "FOREIGN-REVIEW-FEED-COMMENT"
	if err := os.WriteFile(feed, []byte(`{"successor": [{"id": "c-1", "author": "foreign", "body": "`+marker+`", "resolved": false}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, feed string }{
		{"a feed naming a missing file", filepath.Join(t.TempDir(), "missing.json")},
		{"a valid foreign feed", feed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VERDI_REVIEW_FEED", tc.feed)
			f := newObjSupersedeFixture(absModuleRoot(t))
			f.names, f.tmpRoot = []string{"proposed"}, t.TempDir()
			f.steps.buildBinary = copyObjSupersedeBinary(bin)
			t.Cleanup(f.stop)
			rec := getObjSupersedeFixture(t, f, http.MethodGet)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: the ambient feed reached the serve; body=%s", rec.Code, rec.Body.String())
			}
			var info objSupersedeFixtureInfo
			if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
				t.Fatal(err)
			}
			board := info.Stores["proposed"].Boards.Checkout.URL
			status, page := httpGetBody(t, board)
			if status != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200", board, status)
			}
			if strings.Contains(page, marker) || strings.Contains(page, "mode-review") {
				t.Errorf("%s loaded the ambient review feed", board)
			}
		})
	}
}

// TestObjSupersedeFixture_Handler_Happy is the real witness through the
// SHIPPED binary: the handler materializes all eight scenario stores, builds
// each one's docs site from main, and serves each board, and then —
//
//   - the body matches the golden contract exactly (keys and values);
//   - every fact in it re-derives from the SERVED repository's own records;
//   - every board URL renders its spec on a CLEAN working tree, and every
//     not-a-surface view says why;
//   - every docs page renders, every closed object's text renders on its
//     document page (so an absence asserted there can fail), and each
//     docs site is main's, not the checkout's;
//   - after all that serving, the served repositories are still
//     scenario.Build's, SHA for SHA, from the pinned root, each docs site
//     was built at main's own commit in a detached checkout, and no served
//     checkout is dirty.
func TestObjSupersedeFixture_Handler_Happy(t *testing.T) {
	neutralizeCIEnvForTest(t)
	ctx := context.Background()
	f := newObjSupersedeFixture(absModuleRoot(t))
	f.tmpRoot = t.TempDir()
	t.Cleanup(f.stop)

	rec := getObjSupersedeFixture(t, f, http.MethodGet)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var raw struct {
		Stores map[string]json.RawMessage `json:"stores"`
	}
	dec := json.NewDecoder(bytes.NewReader(rec.Body.Bytes()))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		t.Fatalf("decoding body: %v: %s", err, rec.Body.String())
	}
	if len(raw.Stores) != len(objSupersedeStores) {
		t.Fatalf("got %d stores, want %d", len(raw.Stores), len(objSupersedeStores))
	}
	for _, name := range objSupersedeStores {
		t.Run(name, func(t *testing.T) {
			var got objSupersedeStoreInfo
			if err := json.Unmarshal(raw.Stores[name], &got); err != nil {
				t.Fatalf("decoding store %q: %v", name, err)
			}
			for _, base := range []string{got.URL, got.DocsURL} {
				if !strings.HasPrefix(base, "http://127.0.0.1:") || !strings.HasSuffix(base, "/") {
					t.Fatalf("base URL %q is not a loopback base", base)
				}
			}
			if len(got.DocsCommit) != 40 {
				t.Fatalf("docs_commit = %q, want a full commit", got.DocsCommit)
			}
			masked := strings.NewReplacer(got.URL, "{url}", got.DocsURL, "{docs}", got.DocsCommit, "{docs_commit}").Replace(string(raw.Stores[name]))
			if g, w := compactJSON(t, []byte(masked)), compactJSON(t, []byte(objSupersedeGolden[name])); g != w {
				t.Errorf("served JSON:\n got %s\nwant %s", g, w)
			}
			store := f.run.stores[name]
			checkObjSupersedeRecords(t, ctx, store.root, got)
			checkObjSupersedeBoards(t, got)
			checkObjSupersedeDocs(t, got)
			checkObjSupersedeServedSHAs(t, ctx, name, store, got)
			for _, dir := range []string{store.root, store.docsRoot} {
				if porcelain, err := gitOutput(ctx, dir, "status", "--porcelain", "--untracked-files=all"); err != nil || porcelain != "" {
					t.Errorf("%s is dirty after serving: %q, %v", dir, porcelain, err)
				}
			}
		})
	}
}

// checkObjSupersedeServedSHAs is the SHA proof over the store startAll
// actually serves (review I-2): its branches and remote-tracking refs are
// scenario.Build's exactly, every Base and Step commit of Build is there,
// its root is the pinned d49dd630, its checkout is the manifest's, and the
// docs site was built at main's commit from a detached checkout.
func checkObjSupersedeServedSHAs(t *testing.T, ctx context.Context, name string, store *objSupersedeStore, got objSupersedeStoreInfo) {
	t.Helper()
	want := scenario.Build(t, name)
	if len(want.Base) == 0 || want.Base[0] != objSupersedeRootCommit {
		t.Fatalf("scenario.Build(%q) root = %v, want %s", name, want.Base, objSupersedeRootCommit)
	}
	refs := func(dir string) string {
		out, err := gitOutput(ctx, dir, "for-each-ref", "--format=%(refname) %(objectname)", "refs/heads", "refs/remotes")
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if served, built := refs(store.root), refs(want.Dir); served != built {
		t.Errorf("served refs:\n%s\nwant scenario.Build's:\n%s", served, built)
	}
	for _, sha := range append(slices.Clone(want.Base), want.Steps...) {
		if kind, err := gitOutput(ctx, store.root, "cat-file", "-t", sha); err != nil || kind != "commit" {
			t.Errorf("served store lacks Build's commit %s: %q, %v", sha, kind, err)
		}
	}
	if root, _ := gitOutput(ctx, store.root, "rev-list", "--max-parents=0", "refs/heads/main"); root != objSupersedeRootCommit {
		t.Errorf("served root commit = %q, want %s", root, objSupersedeRootCommit)
	}
	if head, _ := gitOutput(ctx, store.root, "rev-parse", "--abbrev-ref", "HEAD"); head != got.Checkout {
		t.Errorf("served checkout = %q, want %q", head, got.Checkout)
	}
	if main, _ := gitOutput(ctx, want.Dir, "rev-parse", "refs/heads/main"); got.DocsCommit != main {
		t.Errorf("docs_commit = %q, want Build's main %s", got.DocsCommit, main)
	}
	if head, _ := gitOutput(ctx, store.docsRoot, "rev-parse", "HEAD"); head != got.DocsCommit {
		t.Errorf("docs checkout HEAD = %q, want %s", head, got.DocsCommit)
	}
	if branch, err := gitOutput(ctx, store.docsRoot, "symbolic-ref", "-q", "HEAD"); err == nil {
		t.Errorf("docs checkout is on %q, want a detached HEAD (never a named-branch worktree of main)", branch)
	}
}

// supersedesEdges maps each object a spec's decisions supersede to the
// deciding decision's ref.
func supersedesEdges(spec *objsupersede.Spec) map[string]string {
	edges := map[string]string{}
	if spec == nil {
		return edges
	}
	for _, d := range spec.FM.Decisions {
		for _, l := range d.Links {
			if string(l.Type) == "supersedes" && strings.Contains(l.Ref, "#") {
				edges[l.Ref] = "spec/" + spec.Name + "#" + d.ID
			}
		}
	}
	return edges
}

// checkObjSupersedeRecords re-derives the store's facts from the served
// repository's own committed records — never from the production table
// (review M-5): the conflicts and the successor every one resolves to,
// every decision-to-object edge of the successor under test and of the
// establishing successor, each object's challenging conflict, the design
// branch that proposed the successor, and main's own successor specs,
// which set the docs pages and main's board.
func checkObjSupersedeRecords(t *testing.T, ctx context.Context, root string, got objSupersedeStoreInfo) {
	t.Helper()
	recs, err := objsupersede.ReadRecords(ctx, objsupersede.WorkTree{Root: root})
	if err != nil || len(recs.Failures) != 0 {
		t.Fatalf("reading the served records: %v %+v", err, recs)
	}
	var conflicts []string
	challenged := map[string]string{}
	for _, c := range recs.Conflicts {
		conflicts = append(conflicts, c.FM.ID)
		if c.FM.ResolvedBy != got.EstablishingSuccessor {
			t.Errorf("%s resolved_by = %q, want establishing_successor %q", c.FM.ID, c.FM.ResolvedBy, got.EstablishingSuccessor)
		}
		for _, l := range c.FM.Links {
			if string(l.Type) == "challenges" {
				challenged[l.Ref] = c.FM.ID
			}
		}
	}
	if !reflect.DeepEqual(conflicts, got.Conflicts) {
		t.Errorf("conflicts = %v, want the records' %v", got.Conflicts, conflicts)
	}
	slug := func(ref string) string { return strings.TrimPrefix(ref, "spec/") }
	successor := recs.Specs[slug(got.Successor)]
	if successor == nil || successor.Archived {
		t.Fatalf("successor %q is not an active spec at the checkout", got.Successor)
	}
	head, establishing := supersedesEdges(successor), supersedesEdges(recs.Specs[slug(got.EstablishingSuccessor)])
	objects := map[string]bool{}
	for o := range head {
		objects[o] = true
	}
	for o := range establishing {
		objects[o] = true
	}
	var want []objSupersedeSupersession
	for o := range objects {
		want = append(want, objSupersedeSupersession{Object: o, Decision: head[o], EstablishingDecision: establishing[o], Conflict: challenged[o]})
	}
	sort.Slice(want, func(i, j int) bool { return want[i].Object < want[j].Object })
	var gotPairs []objSupersedeSupersession
	for _, p := range got.Supersessions {
		gotPairs = append(gotPairs, objSupersedeSupersession{Object: p.Object, Decision: p.Decision, EstablishingDecision: p.EstablishingDecision, Conflict: p.Conflict})
	}
	if !reflect.DeepEqual(gotPairs, want) {
		t.Errorf("supersessions = %+v, want the records' %+v", gotPairs, want)
	}

	// The design branch is the one whose own tip commit proposed the
	// successor under test.
	touched, err := gitOutput(ctx, root, "diff-tree", "--no-commit-id", "--name-only", "-r", "refs/heads/"+got.DesignBranch)
	if err != nil || !slices.Contains(strings.Split(touched, "\n"), successor.Path) {
		t.Errorf("design branch %q's tip does not propose %s: %q, %v", got.DesignBranch, successor.Path, touched, err)
	}

	// main's own successor specs: active specs whose decisions supersede a
	// closed object. They, with the closed specs, are the docs pages; main's
	// board is one of them, or not a surface when there is none.
	mainRecs, err := objsupersede.ReadRecords(ctx, objsupersede.CommitTree{Root: root, Commit: got.MainBranch})
	if err != nil || len(mainRecs.Failures) != 0 {
		t.Fatalf("reading main's records: %v %+v", err, mainRecs)
	}
	docs := map[string]bool{}
	for o := range objects {
		docs[strings.SplitN(o, "#", 2)[0]] = true
	}
	var mainSuccessors []string
	for name, spec := range mainRecs.Specs {
		if !spec.Archived && len(supersedesEdges(spec)) > 0 {
			mainSuccessors = append(mainSuccessors, "spec/"+name)
			docs["spec/"+name] = true
		}
	}
	var gotDocs, wantDocs []string
	for ref := range got.Docs {
		gotDocs = append(gotDocs, ref)
	}
	for ref := range docs {
		wantDocs = append(wantDocs, ref)
	}
	sort.Strings(gotDocs)
	sort.Strings(wantDocs)
	if !reflect.DeepEqual(gotDocs, wantDocs) {
		t.Errorf("docs pages = %v, want the closed specs and main's successors %v", gotDocs, wantDocs)
	}
	checkObjSupersedeMainLinks(t, mainRecs, got)
	switch mainBoard := got.Boards.Main; {
	case len(mainSuccessors) == 0 && mainBoard.NotASurface == "":
		t.Errorf("main carries no successor, yet its board %+v is handed out as a surface", mainBoard)
	case len(mainSuccessors) > 0 && !slices.Contains(mainSuccessors, mainBoard.Spec):
		t.Errorf("main board spec %q is none of main's successors %v", mainBoard.Spec, mainSuccessors)
	case got.Checkout == got.MainBranch && mainBoard.Spec != got.Successor:
		t.Errorf("checkout is main, yet main's board shows %q, not the successor %q", mainBoard.Spec, got.Successor)
	}
}

// checkObjSupersedeMainLinks re-derives, from main's own records, what each
// supersession's main-side links may name (re-review I-A, M-C): its
// object_board is the board of a spec on main whose decision links the
// object — not a surface exactly when no spec on main does — and each docs
// link is present exactly when main carries its target (the decision's
// spec, the establishing decision's spec, the conflict), pointing at that
// target's page or anchor on the docs site.
func checkObjSupersedeMainLinks(t *testing.T, mainRecs *objsupersede.Records, got objSupersedeStoreInfo) {
	t.Helper()
	mainConflicts := map[string]bool{}
	for _, c := range mainRecs.Conflicts {
		mainConflicts[c.FM.ID] = true
	}
	anchor := func(decision string) string {
		spec, id, _ := strings.Cut(decision, "#")
		if decision == "" || mainRecs.Specs[strings.TrimPrefix(spec, "spec/")] == nil {
			return ""
		}
		return got.Docs[spec] + "#" + id
	}
	for _, p := range got.Supersessions {
		var linkers []string
		for name, spec := range mainRecs.Specs {
			if _, links := supersedesEdges(spec)[p.Object]; links && !spec.Archived {
				linkers = append(linkers, "spec/"+name)
			}
		}
		switch b := p.ObjectBoard; {
		case b.Branch != got.MainBranch:
			t.Errorf("%s: object_board is on %q, want main's", p.Object, b.Branch)
		case len(linkers) == 0 && (b.NotASurface == "" || b.URL != ""):
			t.Errorf("%s: no spec on main links it, yet object_board %+v is a surface", p.Object, b)
		case len(linkers) > 0 && !slices.Contains(linkers, b.Spec):
			t.Errorf("%s: object_board shows %q, which does not link it; main's linkers are %v", p.Object, b.Spec, linkers)
		}
		if w := anchor(p.Decision); p.DecisionDocsURL != w {
			t.Errorf("%s: decision_docs_url = %q, want %q", p.Object, p.DecisionDocsURL, w)
		}
		if w := anchor(p.EstablishingDecision); p.EstablishingDecisionDocsURL != w {
			t.Errorf("%s: establishing_decision_docs_url = %q, want %q", p.Object, p.EstablishingDecisionDocsURL, w)
		}
		wantConflict := ""
		if p.Conflict != "" && mainConflicts[p.Conflict] {
			wantConflict = got.DocsURL + "a/" + p.Conflict + "/"
		}
		if p.ConflictDocsURL != wantConflict {
			t.Errorf("%s: conflict_docs_url = %q, want %q", p.Object, p.ConflictDocsURL, wantConflict)
		}
	}
}

// checkObjSupersedeBoards GETs every board view: a surface renders its
// spec's exact title on a clean working tree (review I-3: the serve's own
// data zone must not read as uncommitted changes); a not-a-surface view
// carries no URL and says why. Where main's board is reached through
// /b/main while the checkout is elsewhere, a spec only the checkout has
// must 404 there — proof /b/main is main, not the serving checkout.
func checkObjSupersedeBoards(t *testing.T, got objSupersedeStoreInfo) {
	t.Helper()
	views := []struct {
		role string
		v    objSupersedeBoard
	}{{"checkout", got.Boards.Checkout}, {"design", got.Boards.Design}, {"main", got.Boards.Main}}
	for _, p := range got.Supersessions {
		views = append(views, struct {
			role string
			v    objSupersedeBoard
		}{"object " + p.Object, p.ObjectBoard})
	}
	pages := map[string]string{}
	for _, view := range views {
		v := view.v
		if v.NotASurface != "" {
			if v.URL != "" || v.Spec != "" {
				t.Errorf("%s board %+v: a view that is not a surface must carry no URL or spec", view.role, v)
			}
			continue
		}
		status, page := httpGetBody(t, v.URL)
		if status != http.StatusOK {
			t.Errorf("%s board %s = %d, want 200", view.role, v.URL, status)
			continue
		}
		pages[v.URL] = page
		if title := "<h1>" + objSupersedeBoardTitle[v.Spec] + "</h1>"; !strings.Contains(page, title) {
			t.Errorf("%s board %s lacks %s", view.role, v.URL, title)
		}
		if !strings.Contains(page, `data-testid="asd-posture-tree" data-dirty="clean"`) {
			t.Errorf("%s board %s does not show a clean working tree", view.role, v.URL)
		}
		if strings.Contains(page, `data-testid="uncommitted-indicator">`) {
			t.Errorf("%s board %s shows the uncommitted-changes indicator", view.role, v.URL)
		}
	}
	// Each object's card renders on its object_board (re-review I-A), and on
	// the checkout's board whenever the successor there links the object.
	card := func(object string) string { return `data-ref="` + object + `"` }
	for _, p := range got.Supersessions {
		if b := p.ObjectBoard; b.URL != "" && !strings.Contains(pages[b.URL], card(p.Object)) {
			t.Errorf("object_board %s renders no card for %s", b.URL, p.Object)
		}
		if p.Decision != "" && !strings.Contains(pages[got.Boards.Checkout.URL], card(p.Object)) {
			t.Errorf("checkout board %s renders no card for %s, which %s links", got.Boards.Checkout.URL, p.Object, p.Decision)
		}
	}
	if main := got.Boards.Main; main.URL != "" && got.Checkout != got.MainBranch {
		prefix := strings.TrimSuffix(main.URL, strings.TrimPrefix(main.Spec, "spec/"))
		onlyCheckout := strings.TrimPrefix(got.Successor, "spec/")
		if status, _ := httpGetBody(t, prefix+onlyCheckout); status != http.StatusNotFound {
			t.Errorf("%s%s = %d, want 404: /b/%s must render main, not the checkout", prefix, onlyCheckout, status, got.MainBranch)
		}
		if status, _ := httpGetBody(t, got.Boards.Checkout.URL); status != http.StatusOK {
			t.Errorf("control: the checkout's own board %s = %d, want 200", got.Boards.Checkout.URL, status)
		}
	}
}

// checkObjSupersedeDocs GETs every docs page and every closed object's page
// (review C-1, I-1): each is served, stamped with main's commit (the build
// used dex.Options.Commit, never the wall clock), and renders the object's
// own text at its anchor — the positive control that makes an absence
// asserted on the same page able to fail. A successor the checkout has but
// main does not is absent from the docs site: it is main's.
func checkObjSupersedeDocs(t *testing.T, got objSupersedeStoreInfo) {
	t.Helper()
	stamp := "main @ " + got.DocsCommit[:7]
	for ref, u := range got.Docs {
		status, page := httpGetBody(t, u)
		if status != http.StatusOK {
			t.Errorf("docs page for %s (%s) = %d, want 200", ref, u, status)
			continue
		}
		if !strings.Contains(page, stamp) {
			t.Errorf("docs page for %s lacks the build stamp %q", ref, stamp)
		}
	}
	for _, p := range got.Supersessions {
		page, id, _ := strings.Cut(p.ObjectDocsURL, "#")
		status, body := httpGetBody(t, page)
		if status != http.StatusOK {
			t.Errorf("object page %s = %d, want 200", p.ObjectDocsURL, status)
			continue
		}
		if !strings.Contains(body, `<a id="`+id+`"></a>`) || !strings.Contains(body, objSupersedeObjectText[p.Object]) {
			t.Errorf("object page %s does not render %s's text %q at its anchor", p.ObjectDocsURL, p.Object, objSupersedeObjectText[p.Object])
		}
	}
	// §6's link targets (re-review M-C): the deciding decisions' anchors and
	// the conflict's page, wherever main carries them.
	for _, p := range got.Supersessions {
		for _, u := range []string{p.DecisionDocsURL, p.EstablishingDecisionDocsURL} {
			if u == "" {
				continue
			}
			page, id, _ := strings.Cut(u, "#")
			if status, body := httpGetBody(t, page); status != http.StatusOK || !strings.Contains(body, `<a id="`+id+`"></a>`) {
				t.Errorf("decision anchor %s = %d, want 200 with the anchor", u, status)
			}
		}
		if u := p.ConflictDocsURL; u != "" {
			if status, body := httpGetBody(t, u); status != http.StatusOK || !strings.Contains(body, p.Conflict) {
				t.Errorf("conflict page %s = %d, want 200 naming %s", u, status, p.Conflict)
			}
		}
	}
	if _, onMain := got.Docs[got.Successor]; !onMain {
		if status, _ := httpGetBody(t, got.DocsURL+"a/"+got.Successor+"/"); status != http.StatusNotFound {
			t.Errorf("docs site has %s (%d), which only the checkout carries: want 404, the site is main's", got.Successor, status)
		}
	}
}

// TestObjSupersedeStores_LintClean: every branch of every provisioned
// scenario lints clean in-process — the same call `verdi lint` makes
// (cmd/verdi/lint.go) — and, as the negative control that proves the call
// sees the store at all (review M-1), a refusal scenario lints dirty with
// VL-026. The other stores are align non-resolutions at most (no-conflict,
// chain-not-in-force), never lint refusals: VL-026 checks shape only;
// constraint-target alone is a served lint refusal (below).
func TestObjSupersedeStores_LintClean(t *testing.T) {
	ctx := context.Background()
	rows := map[string][]string{"top-level-supersedes": {"VL-026"}}
	for _, name := range objSupersedeStores {
		rows[name] = nil
	}
	// constraint-target is a served refusal scenario (L5 fix pass 3): its
	// successor's edge targets a constraint, which VL-026 refuses on the
	// design branch; the serve renders it regardless.
	rows["constraint-target"] = []string{"VL-026"}
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		wantRules := rows[name]
		t.Run(name, func(t *testing.T) {
			dir := scenario.Build(t, name).Dir
			branches, err := gitOutput(ctx, dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
			if err != nil || branches == "" {
				t.Fatalf("listing branches: %q, %v", branches, err)
			}
			var rules []string
			for _, branch := range strings.Split(branches, "\n") {
				if err := runGit(ctx, dir, nil, "checkout", "-q", branch); err != nil {
					t.Fatalf("checking out %s: %v", branch, err)
				}
				findings, err := lint.NewEngine().Run(ctx, dir, lint.BuildContext(ctx, dir), lint.Options{})
				if err != nil {
					t.Fatalf("branch %s: lint.Run: %v", branch, err)
				}
				for _, f := range findings {
					if f.Severity == lint.SeverityViolation && !slices.Contains(rules, f.Rule) {
						rules = append(rules, f.Rule)
					}
				}
			}
			sort.Strings(rules)
			if !reflect.DeepEqual(rules, wantRules) {
				t.Fatalf("violations %v, want %v", rules, wantRules)
			}
		})
	}
}
