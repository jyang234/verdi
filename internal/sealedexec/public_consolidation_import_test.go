package sealedexec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// consolidationVerdiSuccessors pins the reviewed successors of the sources the
// immutable Task4 witness still binds by their historical digests. The
// accepted work pinned here, wave by wave:
//
//   - SI-200 (spec-import-contract.md's import-sidecar clause, adopted and
//     Task3 reviewed): contextcompile's classify.go, schema.go, validate.go.
//   - SI-204 / spec/spec-documents ac-10 (ruling R-W4-11): governanceprincipal's
//     decode.go and profile.go, whose Profile.Template seam the wave-4 Task 1
//     review read and approved for this binding.
//   - spec/readiness-recovery wave 1 (ruling R-RR1-22): policyconflict's
//     service.go, whose cache-only judge b0681e61 added, and cmd/verdi's
//     context.go, context_conflict.go and context_constitution.go, which
//     410db101's reviewed round-1 fix (the hermetic conflict-provider seam and
//     the move of the file-local hasDotDotElement to store.HasDotDotElement)
//     and 1be75d01's startup judge-cache warm changed together.
//   - SI-229 / plan R-CM-3 (closing-machinery wave 1, lane L1a; the
//     controller's fix-brief ruling on review finding I-1): artifact's
//     evidence.go, whose EvidenceProvenance gained the optional job_name field
//     and its doc paragraph, and nothing else.
//
// The witness document itself is unchanged in every wave: its digest, corpora,
// totals and replay operands stay exactly as reviewed, and every other bound
// source is still admitted only on an exact historical match.
var consolidationVerdiSuccessors = map[string]consolidationVerdiSuccessor{
	"internal/contextcompile/classify.go": {
		Historical: "451ca5f22e4e2ef14408d312686b871009418e2b8edde216627e45581ecf5c76",
		Successor:  "31131fa517997478e716cd0cd6823a8dc6d935b47d8d6a93675e36899c5ddae4",
		Inverse: []consolidationVerdiEdit{
			{From: "\tif candidate.Source == SourceHeadTree && specImportSidecarPath(candidate.Path) {\n" +
				"\t\treturn ExclusionSpecImportSidecar, true, nil\n" +
				"\t}\n"},
			{From: "// specImportSidecarBoundary is the fixed committed import-provenance zone\n" +
				"// (store.ImportDirRelPath's own .verdi/imports/<slug>/<preview-digest>/\n" +
				"// root). It is a boundary prefix, not a per-file identity: the whole\n" +
				"// subtree is non-authoritative import provenance.\n" +
				"const specImportSidecarBoundary = \".verdi/imports/\"\n" +
				"\n" +
				"// specImportSidecarPath reports whether repoPath lies beneath the committed\n" +
				"// import-provenance boundary (spec-import-contract.md: \"The context\n" +
				"// compiler's separate HEAD-tree repository-file channel must also exclude\n" +
				"// paths beneath .verdi/imports/ before reading their bytes\"). Ordinary\n" +
				"// neighbours whose names merely begin with the same letters\n" +
				"// (.verdi/imports.md, .verdi/importsomething/...) are not beneath it.\n" +
				"func specImportSidecarPath(repoPath string) bool {\n" +
				"\treturn strings.HasPrefix(repoPath, specImportSidecarBoundary)\n" +
				"}\n" +
				"\n"},
		},
	},
	"internal/contextcompile/schema.go": {
		Historical: "090019ac0e8fa222421b32cf6481a29e200e86b7e1462f82531e999403797a15",
		Successor:  "ccab9c42c74f89541b3c6a4b6bd967055c108b6ad67b3bda4634bec3881b392c",
		Inverse: []consolidationVerdiEdit{
			{From: "// ExclusionReason is one of the eleven closed exclusion reasons: the ten\n" +
				"// of authority design §5.2, whose closed v1 enum SI-200 supplements with\n" +
				"// the single member spec-import-sidecar (spec-import-contract.md: \"This\n" +
				"// SI-200 clause supplements the context-compiler authority design, §5.2:\n" +
				"// its closed v1 enum gains this one member; all existing members and\n" +
				"// fail-closed decoding remain\"). Unknown values fail closed.\n",
				To: "// ExclusionReason is one of the ten closed exclusion reasons (authority\n" +
					"// design §5.2). Unknown values fail closed.\n"},
			{From: "\tExclusionSpecImportSidecar         ExclusionReason = \"spec-import-sidecar\"\n"},
		},
	},
	"internal/contextcompile/validate.go": {
		Historical: "51eed09d21e9ad0da04237f1f1860b37fc71686ca38ad152c3482f2a859e0487",
		Successor:  "31cf1c6733aca29b9a03b10bb585f2f872f672d2d3489a2b73fafa1425f3126d",
		Inverse: []consolidationVerdiEdit{
			{From: "\t\tExclusionArchivedRecord, ExclusionGeneratedProjectionOutput, ExclusionNonTextData, ExclusionNonRegularFile,\n" +
				"\t\tExclusionSpecImportSidecar:\n",
				To: "\t\tExclusionArchivedRecord, ExclusionGeneratedProjectionOutput, ExclusionNonTextData, ExclusionNonRegularFile:\n"},
		},
	},
	"internal/governanceprincipal/decode.go": {
		Historical: "d69540dd109971b7bf36f40f44366345a32f1501c3b08e2d9eda7eec19c65043",
		Successor:  "d0b9e48a966885089c658ff4e0d0240e55b519b7a9237cbe9f50e2e68551a3bb",
		Inverse: []consolidationVerdiEdit{
			{From: "\tTemplate                   *TemplateRecord           `yaml:\"template\"`\n"},
			{From: "\tif doc.Template != nil {\n" +
				"\t\tif err := doc.Template.Validate(); err != nil {\n" +
				"\t\t\treturn Profile{}, fmt.Errorf(\"governanceprincipal: profile %w\", err)\n" +
				"\t\t}\n" +
				"\t}\n" +
				"\n"},
			{From: "\t\tTemplate:                   doc.Template,\n"},
		},
	},
	"internal/governanceprincipal/profile.go": {
		Historical: "91960126159351d7def440918c7d9141095939609ebdb949799d43607662f89d",
		Successor:  "6ee7192e0fce13271089bdf54bda538846fc0d4d16cf9d82dbb411b19ee093c2",
		Inverse: []consolidationVerdiEdit{
			{From: "\t// Template is the resolved scaffold record a starter-written profile\n" +
				"\t// carries (spec/spec-documents ac-10, SI-204); optional \u2014 a\n" +
				"\t// hand-authored profile carries none; digest-bound like every other\n" +
				"\t// exported field.\n" +
				"\tTemplate *TemplateRecord `json:\"template,omitempty\"`\n" +
				"\n"},
		},
	},
	"internal/policyconflict/service.go": {
		Historical: "c9bccee4f99c1b1b22b17e1e19451d7254d70504fd5cc1029209aaa013a2bc54",
		Successor:  "1c3ae85440649b240d2cc8970224e100b00781e9181ccf44d0b54c77e5ebbb61",
		Inverse: []consolidationVerdiEdit{
			{From: "\t\tcase cacheOnlyJudge:\n" +
				"\t\t\tadapters = append(adapters, adapter.adapter)\n"},
			{From: "\tcase cacheOnlyJudge:\n" +
				"\t\t// AC-3/R-RR1-4: the cache-only judge is recognized here, alongside\n" +
				"\t\t// JudgeAdapter/*JudgeAdapter, so it gets the SAME tree-hash/profile/\n" +
				"\t\t// authority axes a real run's CachedJudge call above uses — the\n" +
				"\t\t// cache key a request computes is byte-identical to the one a real\n" +
				"\t\t// run (`verdi context conflict`, or serve's startup pre-run)\n" +
				"\t\t// published under. It never runs adapter.Argv: cacheOnlyLookup only\n" +
				"\t\t// performs CachedJudge's hit-check.\n" +
				"\t\tif !cache.enabled {\n" +
				"\t\t\treturn nil, fmt.Errorf(\"concrete judge adapter has no prepared cache context\")\n" +
				"\t\t}\n" +
				"\t\tlookupAdapter := adapter.adapter\n" +
				"\t\tlookupAdapter.Root = cache.root\n" +
				"\t\tvalidated, cacheErr := cacheOnlyLookup(lookupAdapter, input, cache.treeHash, view.Profile.ID, view.Snapshot.ProfileDigest, view.Snapshot.EffectivePolicyDigest)\n" +
				"\t\texchange, err = validated.Exchange, cacheErr\n"},
			{From: "\t// A cache miss (from the cache-only judge, in production always the\n" +
				"\t// case above — the default arm's own direct callers can, in principle,\n" +
				"\t// return the same sentinel) is \"no judgment\", exactly like a nil judge,\n" +
				"\t// never an operational failure (service.go:354, R-RR1-4).\n" +
				"\tif errors.Is(err, ErrJudgeCacheMiss) {\n" +
				"\t\treturn nil, nil\n" +
				"\t}\n"},
		},
	},
	"cmd/verdi/context_constitution.go": {
		Historical: "78137e4e88905f9dcdf2a7d111d2d6f0244e2aa27d9625cae86041141180a98b",
		Successor:  "ccf68869ba6d29bcc05383149bd379e4ac4bcddf8b454d71a8663b92a8d688d0",
		Inverse: []consolidationVerdiEdit{
			{From: "// validateContextOutputStoreZone/sameFileArg helpers and\n" +
				"// store.HasDotDotElement byte-for-byte (the exact --request/--out grammar\n" +
				"// `context compile` and\n",
				To: "// validateContextOutputStoreZone/sameFileArg/hasDotDotElement helpers\n" +
					"// byte-for-byte (the exact --request/--out grammar `context compile` and\n"},
			{From: "\tif hasOut && store.HasDotDotElement(outArg) {\n",
				To: "\tif hasOut && hasDotDotElement(outArg) {\n"},
		},
	},
	"cmd/verdi/context_conflict.go": {
		Historical: "6453b4a780c7375d5eb9ce457ab68ba28d366e1f59010d757b1cc507b7beec62",
		Successor:  "94f3c9653a0ca999437ab4c73823eeab71823fea31d33185c48c988dccfacbcd",
		Inverse: []consolidationVerdiEdit{
			{From: "\t\"os\"\n",
				To: "\t\"os\"\n" +
					"\t\"time\"\n"},
			{From: "\t\"github.com/jyang234/verdi/internal/atomicfile\"\n",
				To: "\t\"github.com/jyang234/verdi/internal/align\"\n" +
					"\t\"github.com/jyang234/verdi/internal/atomicfile\"\n"},
			{From: "\t\"github.com/jyang234/verdi/internal/governanceprincipal\"\n",
				To: "\t\"github.com/jyang234/verdi/internal/contextcompile\"\n" +
					"\t\"github.com/jyang234/verdi/internal/governanceprincipal\"\n"},
			{From: "\t\"github.com/jyang234/verdi/internal/readinessload\"\n"},
			{From: "// globals or bypassing the real request codec. Provider construction itself\n" +
				"// is internal/readinessload.NewConflictProvider (moved from this file's old\n" +
				"// newLocalContextConflictProvider, spec/readiness-recovery Task 2): `verdi\n" +
				"// context conflict` is a JudgeRun caller, exactly like it always was — it\n" +
				"// may launch the manifest's configured judge on a cache miss.\n",
				To: "// globals or bypassing the real request codec.\n"},
			{From: "\treturn cmdContextConflictWithFactory(args, stdin, stdout, stderr, func(ctx context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {\n" +
				"\t\treturn readinessload.NewConflictProvider(ctx, root, request, readinessload.JudgeRun, resolveConflictActors)\n" +
				"\t})\n",
				To: "\treturn cmdContextConflictWithFactory(args, stdin, stdout, stderr, newLocalContextConflictProvider)\n"},
			{From: "\tif hasOut && store.HasDotDotElement(outArg) {\n",
				To: "\tif hasOut && hasDotDotElement(outArg) {\n"},
			{From: "// resolveConflictActors resolves the store's local-operator actor claim\n",
				To: "func newLocalContextConflictProvider(ctx context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {\n" +
					"\tmanifest, err := loadManifest(root)\n" +
					"\tif err != nil {\n" +
					"\t\treturn nil, err\n" +
					"\t}\n" +
					"\tvar primary policyconflict.Judge\n" +
					"\tif manifest.Align != nil && len(manifest.Align.JudgeCmd) != 0 {\n" +
					"\t\ttimeout := align.DefaultJudgeTimeout\n" +
					"\t\tif manifest.Align.JudgeTimeoutSeconds != 0 {\n" +
					"\t\t\ttimeout = time.Duration(manifest.Align.JudgeTimeoutSeconds) * time.Second\n" +
					"\t\t}\n" +
					"\t\tprimary = policyconflict.JudgeAdapter{\n" +
					"\t\t\tRole:    string(policyconflict.JudgePrimary),\n" +
					"\t\t\tAdapter: contextConflictRequestAdapter(request),\n" +
					"\t\t\tModel:   \"align.judge_cmd\",\n" +
					"\t\t\tArgv:    append([]string(nil), manifest.Align.JudgeCmd...),\n" +
					"\t\t\tTimeout: timeout,\n" +
					"\t\t\tRoot:    root,\n" +
					"\t\t\tRunner:  contextConflictJudgeRunner{delegate: align.ExecJudgeRunner{}},\n" +
					"\t\t}\n" +
					"\t}\n" +
					"\tactors, err := resolveConflictActors(ctx, root)\n" +
					"\tif err != nil {\n" +
					"\t\treturn nil, err\n" +
					"\t}\n" +
					"\treturn policyconflict.NewService(root, policyconflict.ServiceDeps{\n" +
					"\t\tCompiler:   contextcompile.NewCompiler(),\n" +
					"\t\tRefs:       contextConflictRefResolver{},\n" +
					"\t\tPrimary:    primary,\n" +
					"\t\tTreeHasher: contextConflictTreeHasher{},\n" +
					"\t\tDates:      contextConflictDateSource{},\n" +
					"\t\tActors:     actors,\n" +
					"\t}), nil\n" +
					"}\n" +
					"\n" +
					"// resolveConflictActors resolves the store's local-operator actor claim\n"},
			{From: "\treturn resolveLocalActors(ctx, root, profile)\n" +
				"}\n",
				To: "\treturn resolveLocalActors(ctx, root, profile)\n" +
					"}\n" +
					"\n" +
					"func contextConflictRequestAdapter(request policyconflict.Request) contextcompile.AdapterRef {\n" +
					"\tif request.Target.AcceptedContext != nil {\n" +
					"\t\treturn request.Target.AcceptedContext.Adapter\n" +
					"\t}\n" +
					"\tif request.Target.AcceptanceCandidate != nil {\n" +
					"\t\treturn request.Target.AcceptanceCandidate.Adapter\n" +
					"\t}\n" +
					"\treturn contextcompile.AdapterRef{}\n" +
					"}\n" +
					"\n" +
					"type contextConflictJudgeRunner struct{ delegate align.JudgeRunner }\n" +
					"\n" +
					"func (r contextConflictJudgeRunner) Run(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {\n" +
					"\tif r.delegate == nil {\n" +
					"\t\treturn nil, 0, errors.New(\"context conflict judge runner is nil\")\n" +
					"\t}\n" +
					"\tresult, err := r.delegate.RunJudge(ctx, argv, stdin)\n" +
					"\treturn result.Stdout, result.ExitCode, err\n" +
					"}\n" +
					"\n" +
					"type contextConflictTreeHasher struct{}\n" +
					"\n" +
					"func (contextConflictTreeHasher) TreeHash(ctx context.Context, root string) (string, error) {\n" +
					"\tservices, err := store.DiscoverServices(root)\n" +
					"\tif err != nil {\n" +
					"\t\treturn \"\", err\n" +
					"\t}\n" +
					"\treturn store.TreeHash(ctx, root, services)\n" +
					"}\n" +
					"\n" +
					"type contextConflictDateSource struct{}\n" +
					"\n" +
					"func (contextConflictDateSource) TodayUTC(ctx context.Context) (string, error) {\n" +
					"\tif err := ctx.Err(); err != nil {\n" +
					"\t\treturn \"\", err\n" +
					"\t}\n" +
					"\treturn time.Now().UTC().Format(\"2006-01-02\"), nil\n" +
					"}\n" +
					"\n" +
					"// contextConflictRefResolver makes absent local graph proof explicit. Exact\n" +
					"// ref equality is settled before this port is called; every different pair\n" +
					"// remains unknown and is therefore sent to semantic evaluation, never treated\n" +
					"// as favorable overlap/disjointness. Managed callers may inject a stronger\n" +
					"// graph resolver directly into ServiceDeps.\n" +
					"type contextConflictRefResolver struct{}\n" +
					"\n" +
					"func (contextConflictRefResolver) Relate(context.Context, string, string) (policyconflict.ScopeState, []string, error) {\n" +
					"\treturn policyconflict.ScopeUnknown, []string{\"ref-relation-unproven\"}, nil\n" +
					"}\n" +
					"\n" +
					"func (contextConflictRefResolver) Covers(context.Context, string, string) (policyconflict.ProofState, []string, error) {\n" +
					"\treturn policyconflict.ProofUnproven, []string{\"ref-coverage-unproven\"}, nil\n" +
					"}\n"},
		},
	},
	"cmd/verdi/context.go": {
		Historical: "555cb3c6baa7c7d3e25cf12d62e29b4fd69309c548852cceaaa855247315b53a",
		Successor:  "234d4daea5d5d5aa5f76a5a4553855cd80b1b0b4a0392ff2b1d90a119b64330b",
		Inverse: []consolidationVerdiEdit{
			{From: "\tif hasOut && store.HasDotDotElement(outArg) {\n",
				To: "\tif hasOut && hasDotDotElement(outArg) {\n"},
			{From: "// canonicalOutPath returns the single absolute, alias-resolved destination\n",
				To: "// hasDotDotElement reports whether p contains a \"..\" PATH ELEMENT under\n" +
					"// either separator convention. It is element-wise, never a substring test:\n" +
					"// a file honestly named \"..notes.json\" or \"a..b\" carries no traversal and\n" +
					"// stays allowed.\n" +
					"func hasDotDotElement(p string) bool {\n" +
					"\tfor _, seg := range strings.FieldsFunc(p, func(r rune) bool {\n" +
					"\t\treturn r == '/' || r == filepath.Separator\n" +
					"\t}) {\n" +
					"\t\tif seg == \"..\" {\n" +
					"\t\t\treturn true\n" +
					"\t\t}\n" +
					"\t}\n" +
					"\treturn false\n" +
					"}\n" +
					"\n" +
					"// canonicalOutPath returns the single absolute, alias-resolved destination\n"},
			{From: "// store.HasDotDotElement rejects such spellings earlier still, so this function's\n",
				To: "// hasDotDotElement rejects such spellings earlier still, so this function's\n"},
		},
	},
	"internal/artifact/evidence.go": {
		Historical: "a7cb95eed8a7bca420dce3035e72a4fc80f5bee2f598c59f069bc8e9be318201",
		Successor:  "7b9865491b4b62dd545715a9eb20b5fce2c6cd3798595903abb9f94b76e25a2f",
		Inverse: []consolidationVerdiEdit{
			{From: "\tJobName  string           `json:\"job_name,omitempty\"`\n"},
			{From: "//\n" +
				"// JobName is an SI-229 addition (ledger SI-229, plan R-CM-3): SI-71's\n" +
				"// authoritative-source match compares an obligation's CI-job reference\n" +
				"// against a record's provenance, but Job carries I-25's ordering-id meaning\n" +
				"// (GitHub's GITHUB_RUN_ATTEMPT, GitLab's monotonic CI_JOB_ID) — one field\n" +
				"// cannot carry both a retry ordinal and a stable declared job name. JobName\n" +
				"// is the CI job's declared name (GitHub GITHUB_JOB, GitLab CI_JOB_NAME),\n" +
				"// optional and never inferred from a display label; 03 §Evidence records:\n" +
				"// \"provenance.job_name is optional: the CI job's declared name.\n" +
				"// Authoritative-source matching compares an obligation's CI-job reference\n" +
				"// with job_name; job stays the ordering id.\" JobName never joins the fold's\n" +
				"// (pipeline id, job id) ordering (internal/evidence's groupKey/\n" +
				"// laterProvenance/recordSortKey stay Job-only) — amends SI-71's matching\n" +
				"// operand only, not I-25's ordering.\n"},
		},
	},
}

func TestPublicControllerConsolidationImportSuccessor(t *testing.T) {
	// The inverse machinery itself, on bytes no repository file supplies: an
	// edit that is absent, repeated or out of order recovers nothing.
	t.Run("inverse", func(t *testing.T) {
		remove := consolidationVerdiEdit{From: "b\n"}
		rename := consolidationVerdiEdit{From: "a\n", To: "A\n"}
		for _, tc := range []struct {
			name     string
			source   string
			inverse  []consolidationVerdiEdit
			restored string
			ok       bool
		}{
			{"single-removal", "a\nb\nc\n", []consolidationVerdiEdit{remove}, "a\nc\n", true},
			{"ordered-pair", "a\nb\nc\n", []consolidationVerdiEdit{remove, rename}, "A\nc\n", true},
			{"absent-edit", "a\nc\n", []consolidationVerdiEdit{remove}, "", false},
			{"repeated-edit", "a\nb\nb\nc\n", []consolidationVerdiEdit{remove}, "", false},
			{"repeated-after-earlier-edit", "a\nb\na\n", []consolidationVerdiEdit{remove, rename}, "", false},
			{"no-edits-leaves-source", "a\nb\n", nil, "a\nb\n", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				got, ok := consolidationApplyInverse([]byte(tc.source), tc.inverse)
				if ok != tc.ok {
					t.Fatalf("applied=%v want=%v", ok, tc.ok)
				}
				if ok && string(got) != tc.restored {
					t.Fatalf("restored %q want %q", got, tc.restored)
				}
			})
		}
	})

	// Exact historical bytes are admitted on any path, and unknown bytes are
	// refused whether or not the path carries a pinned successor.
	t.Run("pure", func(t *testing.T) {
		unbound := "internal/contextcompile/absent.go"
		mapped := "internal/contextcompile/schema.go"
		historical := consolidationVerdiSuccessors[mapped].Historical
		for _, tc := range []struct {
			name, path, want string
			source           []byte
			valid            bool
		}{
			{"unchanged-source", unbound, consolidationDigest([]byte("unchanged")), []byte("unchanged"), true},
			{"unknown-source-change", unbound, consolidationDigest([]byte("unchanged")), []byte("changed"), false},
			{"unknown-bytes-on-mapped-path", mapped, historical, []byte("changed"), false},
			{"successor-hash-is-not-an-admission", mapped, consolidationVerdiSuccessors[mapped].Successor, []byte("changed"), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := consolidationVerdiSourceMatches(tc.path, tc.want, tc.source); got != tc.valid {
					t.Fatalf("source admission=%v want=%v", got, tc.valid)
				}
			})
		}
	})

	// Each mapped successor, against the actual reviewed bytes in this tree.
	t.Run("bound", func(t *testing.T) {
		for path, bound := range consolidationVerdiSuccessors {
			t.Run(path, func(t *testing.T) {
				current, err := os.ReadFile(filepath.Join("..", "..", path))
				if err != nil {
					t.Fatal(err)
				}
				if consolidationDigest(current) != bound.Successor {
					t.Fatalf("%s is not the pinned reviewed successor", path)
				}
				historical, ok := consolidationApplyInverse(current, bound.Inverse)
				if !ok || consolidationDigest(historical) != bound.Historical {
					t.Fatalf("the pinned inverse edit did not reproduce %s's historical bytes", path)
				}
				other := ""
				for candidate := range consolidationVerdiSuccessors {
					if candidate != path {
						other = candidate
					}
				}
				for _, tc := range []struct {
					name, path, want string
					source           []byte
					valid            bool
				}{
					{"historical", path, bound.Historical, historical, true},
					{"successor", path, bound.Historical, current, true},
					{"wrong-path", path + ".bak", bound.Historical, current, false},
					{"other-mapped-path", other, bound.Historical, current, false},
					{"wrong-historical-hash", path, strings.Repeat("0", 64), current, false},
					{"crossed-historical-hash", path, consolidationVerdiSuccessors[other].Historical, current, false},
					{"appended-to-successor", path, bound.Historical, append(append([]byte{}, current...), '\n'), false},
					{"appended-to-historical", path, bound.Historical, append(append([]byte{}, historical...), '\n'), false},
					{"changed-inside-the-reviewed-delta", path, bound.Historical, consolidationMutateFirstEdit(current, bound.Inverse[0]), false},
					{"repeated-reviewed-delta", path, bound.Historical, consolidationRepeatFirstEdit(current, bound.Inverse[0]), false},
				} {
					t.Run(tc.name, func(t *testing.T) {
						if got := consolidationVerdiSourceMatches(tc.path, tc.want, tc.source); got != tc.valid {
							t.Fatalf("source admission=%v want=%v", got, tc.valid)
						}
					})
				}
			})
		}
	})

	// The table describes real stale witness entries and nothing else: every
	// checked source that differs from its bound digest is a pinned successor,
	// and every pinned successor is bound by the witness at its historical hash.
	t.Run("witness-coverage", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join("testdata", "public-consolidation", "checks.json"))
		if err != nil {
			t.Fatal(err)
		}
		var w consolidationWitness
		if err = json.Unmarshal(raw, &w); err != nil {
			t.Fatal(err)
		}
		bound := map[string]string{}
		drifted := map[string]bool{}
		for _, hashes := range []map[string]string{w.Implementation, w.TestSource} {
			for path, want := range hashes {
				bound[path] = want
				source, err := os.ReadFile(filepath.Join("..", "..", path))
				if err != nil {
					t.Fatal(err)
				}
				if consolidationDigest(source) == want {
					continue
				}
				drifted[path] = true
				if _, pinned := consolidationVerdiSuccessors[path]; !pinned {
					t.Fatalf("unmapped drift in bound source %s", path)
				}
				if !consolidationVerdiSourceMatches(path, want, source) {
					t.Fatalf("pinned successor %s is not admitted", path)
				}
			}
		}
		for path, successor := range consolidationVerdiSuccessors {
			if bound[path] != successor.Historical {
				t.Fatalf("%s is not bound by the witness at its pinned historical digest", path)
			}
			if !drifted[path] {
				t.Logf("%s currently holds its historical bytes; its successor pin is unexercised", path)
			}
		}
	})
}

// consolidationMutateFirstEdit changes one byte inside the reviewed delta, so
// the result is neither the historical nor the reviewed successor source. The
// byte is the delta's first ASCII letter, case-flipped: a mutation every pinned
// delta admits whatever its text, since the waves pinned above share no
// vocabulary to key on.
func consolidationMutateFirstEdit(source []byte, edit consolidationVerdiEdit) []byte {
	mutated := []byte(edit.From)
	for i, b := range mutated {
		if b >= 'a' && b <= 'z' {
			mutated[i] = b - ('a' - 'A')
			break
		}
		if b >= 'A' && b <= 'Z' {
			mutated[i] = b + ('a' - 'A')
			break
		}
	}
	if bytes.Equal(mutated, []byte(edit.From)) {
		panic("consolidationMutateFirstEdit: delta carries no ASCII letter to change")
	}
	return bytes.Replace(source, []byte(edit.From), mutated, 1)
}

// consolidationRepeatFirstEdit duplicates the reviewed delta, so no single
// inverse edit can be unambiguous.
func consolidationRepeatFirstEdit(source []byte, edit consolidationVerdiEdit) []byte {
	from := []byte(edit.From)
	return bytes.Replace(source, from, append(append([]byte{}, from...), from...), 1)
}
