package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/upstream"
)

// phase7ManifestYAML is a self-contained verdi.yaml for Phase 7's own
// tests: providers.jira configured (so a jira: story scheme passes the
// "is this scheme configured" check), a toolchain: block (so baseline
// regeneration has something to key an injected FakeRunner off of), and
// flowmap service discovery. Built fresh per test via fixturegit — never
// testdata/corpus, per this phase's own instructions.
const phase7ManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
toolchain:
  module: github.com/jyang234/golang-code-graph
  commit: cd38b1a56bb782177a207d741a39807821cf2c1c
`

// loansvcFlowmapYAML is the minimal flowmap-shaped service root Phase 7's
// baseline-regeneration tests scope an spec's impacts: to — no
// obligations, no boundary contract, no policy.json, no bindings sidecar,
// so regenerateService's only upstream call is `flowmap graph`.
const loansvcFlowmapYAML = `version: 1
service: loansvc
`

// phase7GitAttributes satisfies VL-012 (02 §Repository plumbing: every
// committed-generated path pattern needs a `gitlab-generated` attribute
// line, since phase7ManifestYAML's forge: is gitlab) so that a spec
// scaffolded and accepted against this fixture is lint-clean on that rule
// too, not just the ones design/accept/feature directly touch.
const phase7GitAttributes = `.verdi/specs/*/*/board.json gitlab-generated
.verdi/specs/*/*/rollup.json gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
`

// buildPhase7Repo builds a one-layer fixturegit repo carrying just
// verdi.yaml and the loansvc service fixture — the common starting point
// for design/accept/feature start tests.
func buildPhase7Repo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{
		{
			Files: map[string]string{
				".verdi/verdi.yaml":     phase7ManifestYAML,
				"loansvc/.flowmap.yaml": loansvcFlowmapYAML,
				".gitattributes":        phase7GitAttributes,
			},
			Message: "init store",
		},
	})
}

// phase7Manifest decodes phase7ManifestYAML, the manifest every Phase 7
// test's runXxxStart core function needs as an argument (mirroring
// cmdDesignStart/cmdFeatureStart's own loadManifest call).
func phase7Manifest(t *testing.T) *store.Manifest {
	t.Helper()
	m, err := store.DecodeManifest([]byte(phase7ManifestYAML))
	if err != nil {
		t.Fatalf("decoding phase7ManifestYAML: %v", err)
	}
	return m
}

// readSpec strict-decodes the spec.md at .verdi/specs/active/<name>/spec.md
// under root, failing the test on any read/decode error.
func readSpec(t *testing.T, root, name string) (*artifact.SpecFrontmatter, []byte) {
	t.Helper()
	path := filepath.Join(root, ".verdi", "specs", "active", name, "spec.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		t.Fatalf("splitting frontmatter of %s: %v", path, err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("decoding spec %s: %v", path, err)
	}
	return spec, raw
}

// fakeGraphRunner returns a FakeRunner primed to answer exactly one
// `flowmap graph` call with an empty-but-valid graph — enough for
// regenerateService's minimal path (no boundary contract, no policy.json,
// no bindings sidecar on the fixture service).
func fakeGraphRunner() upstream.Runner {
	fr := upstream.NewFakeRunner()
	fr.Enqueue("flowmap", "graph", upstream.Result{Stdout: []byte("{}"), ExitCode: 0})
	return fr
}
