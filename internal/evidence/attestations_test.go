package evidence

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const testAttestation = `---
id: attestation/story-1--ac-2
kind: attestation
title: "AC-2 attested (test)"
owners: [qa-lead]
frozen: { at: 2026-05-01, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# Attestation
`

// unauthoredScaffoldFixture is a hand-written but scaffold-SHAPED fixture
// (spec/attest-helper dc-3): the marker present in the body, exactly what
// `verdi attest` itself would write before an operator authors a claim.
const unauthoredScaffoldFixture = `---
id: attestation/story-1--ac-2
kind: attestation
title: "unauthored attestation scaffold: jira:STORY-1 ac-2"
owners: [platform-team]
schema: verdi.attestation/v1
links:
  - { type: verifies, ref: "spec/story-1" }
frozen: { at: 2026-07-16, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
<!-- verdi:attestation-unauthored -->
This attestation was scaffolded by ` + "`verdi attest`" + ` for jira:STORY-1 ac-2
and has not been authored.
`

func writeAttestation(t *testing.T, root, storySlug, acID, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "attestations", storySlug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, acID+".md"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing attestation: %v", err)
	}
}

// TestAttestationExists_Happy proves existence, alone, is the record
// (02 §Kind registry: "(none — existence is the record)").
func TestAttestationExists_Happy(t *testing.T) {
	root := t.TempDir()
	writeAttestation(t, root, "story-1", "ac-2", testAttestation)

	exists, err := AttestationExists(root, "story-1", "ac-2")
	if err != nil {
		t.Fatalf("AttestationExists: %v", err)
	}
	if !exists {
		t.Fatal("AttestationExists(present file) = false, want true")
	}
}

// TestAttestationExists_Negative proves a missing file reads as false, no
// error, and a path that is a directory (not a file) is a real error.
func TestAttestationExists_Negative(t *testing.T) {
	root := t.TempDir()

	exists, err := AttestationExists(root, "story-1", "ac-999")
	if err != nil {
		t.Fatalf("AttestationExists(missing): %v", err)
	}
	if exists {
		t.Fatal("AttestationExists(missing file) = true, want false")
	}

	dirPath := filepath.Join(root, ".verdi", "attestations", "story-1", "ac-2.md")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dirPath, err)
	}
	if _, err := AttestationExists(root, "story-1", "ac-2"); err == nil {
		t.Fatal("AttestationExists(path is a directory): want error, got nil")
	}
}

// TestAttestationPath proves the one shared path-construction convention
// (spec/close-preflight dc-4/ac-1's attestation obligation: "every path a
// --preflight disclosure names ... is produced by calling the real
// path-construction helpers ... never a hand-typed string literal that
// could drift"): AttestationExists and LoadAttestationState both resolve
// the exact path AttestationPath returns, and passing an empty storeRoot
// yields the store-relative display form a disclosure prints (filepath.Join
// drops an empty leading element).
func TestAttestationPath(t *testing.T) {
	root := t.TempDir()
	got := AttestationPath(root, "story-1", "ac-2")
	want := filepath.Join(root, ".verdi", "attestations", "story-1", "ac-2.md")
	if got != want {
		t.Fatalf("AttestationPath(root, ...) = %q, want %q", got, want)
	}

	writeAttestation(t, root, "story-1", "ac-2", testAttestation)
	if _, err := os.Stat(AttestationPath(root, "story-1", "ac-2")); err != nil {
		t.Fatalf("AttestationPath does not resolve to the same file AttestationExists/LoadAttestationState check: %v", err)
	}

	rel := AttestationPath("", "story-1", "ac-2")
	wantRel := filepath.Join(".verdi", "attestations", "story-1", "ac-2.md")
	if rel != wantRel {
		t.Fatalf("AttestationPath(\"\", ...) = %q, want %q (the store-relative display form)", rel, wantRel)
	}
}

// TestUnauthoredAttestationMarker_IsFixedSentinel pins the exact byte-for-
// byte sentinel (spec/attest-helper dc-3) so the scaffold writer and every
// fold reader are provably sharing the one literal this test locks in.
func TestUnauthoredAttestationMarker_IsFixedSentinel(t *testing.T) {
	const want = "<!-- verdi:attestation-unauthored -->"
	if UnauthoredAttestationMarker != want {
		t.Fatalf("UnauthoredAttestationMarker = %q, want %q", UnauthoredAttestationMarker, want)
	}
}

// TestLoadAttestationState proves the three-way state (spec/attest-helper
// dc-3): no file is Absent, a marker-bearing scaffold is Unauthored, and a
// hand-authored file with no marker is Authored — over both a real
// marker-bearing fixture and a hand-written authored one (dc-3's own test
// obligation).
func TestLoadAttestationState(t *testing.T) {
	t.Run("absent", func(t *testing.T) {
		root := t.TempDir()
		state, err := LoadAttestationState(root, "story-1", "ac-9")
		if err != nil {
			t.Fatalf("LoadAttestationState: %v", err)
		}
		if state != AttestationAbsent {
			t.Fatalf("state = %v, want AttestationAbsent", state)
		}
	})

	t.Run("unauthored scaffold", func(t *testing.T) {
		root := t.TempDir()
		writeAttestation(t, root, "story-1", "ac-2", unauthoredScaffoldFixture)
		state, err := LoadAttestationState(root, "story-1", "ac-2")
		if err != nil {
			t.Fatalf("LoadAttestationState: %v", err)
		}
		if state != AttestationUnauthored {
			t.Fatalf("state = %v, want AttestationUnauthored", state)
		}
	})

	t.Run("authored (hand-written, no marker)", func(t *testing.T) {
		root := t.TempDir()
		writeAttestation(t, root, "story-1", "ac-2", testAttestation)
		state, err := LoadAttestationState(root, "story-1", "ac-2")
		if err != nil {
			t.Fatalf("LoadAttestationState: %v", err)
		}
		if state != AttestationAuthored {
			t.Fatalf("state = %v, want AttestationAuthored", state)
		}
	})

	t.Run("path is a directory is an operational error", func(t *testing.T) {
		root := t.TempDir()
		dirPath := filepath.Join(root, ".verdi", "attestations", "story-1", "ac-2.md")
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dirPath, err)
		}
		if _, err := LoadAttestationState(root, "story-1", "ac-2"); err == nil {
			t.Fatal("LoadAttestationState(path is a directory): want error, got nil")
		}
	})

	// A file that EXISTS but cannot be read (mode 000) must fail closed —
	// the os.ReadFile error propagates as an operational error, never a
	// swallowed AttestationAuthored. This is the exact input ADJ-67 / D6-38
	// turned on: the round's stat-only-swallow predecessor (AttestationExists)
	// returned (true, nil) here, silently counting an unreadable file as a
	// satisfied HUMAN attestation (the unproven-silent-pass three-valued
	// honesty forbids); LoadAttestationState reads content, so it fails
	// closed. This subtest must FAIL if anyone restores the swallow.
	t.Run("present but unreadable is an operational error, never a swallowed attested=true", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("DISCLOSURE: running as root — os.Chmod(0o000) does not restrict root's own reads, so this permission-based negative test cannot exercise the unreadable-attestation path under this user")
		}
		root := t.TempDir()
		writeAttestation(t, root, "story-1", "ac-2", testAttestation)
		path := AttestationPath(root, "story-1", "ac-2")
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("os.Chmod(%s, 0o000): %v", path, err)
		}
		t.Cleanup(func() {
			_ = os.Chmod(path, 0o644) // restore so t.TempDir()'s own cleanup can remove it
		})

		state, err := LoadAttestationState(root, "story-1", "ac-2")
		if err == nil {
			t.Fatalf("LoadAttestationState(present-but-unreadable) err = nil (state=%v), want a propagated read error — a present attestation that cannot be read must fail closed, never swallow to a satisfied attestation (ADJ-67/D6-38)", state)
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("err = %v, want it to wrap os.ErrPermission (the os.ReadFile EACCES, preserved through %%w)", err)
		}
		if !strings.Contains(err.Error(), "loading attestation state") {
			t.Fatalf("err = %q, want it to name LoadAttestationState's own read wrapping", err.Error())
		}
		if state != AttestationAbsent {
			t.Fatalf("state = %v, want AttestationAbsent alongside the error — crucially NOT AttestationAuthored", state)
		}
	})
}

// attestationScaffoldCases are representative (story, ac) inputs shared by
// the frontmatter-shape and self-validation tests below.
func attestationScaffoldCases() []struct {
	name string
	in   AttestationScaffold
} {
	return []struct {
		name string
		in   AttestationScaffold
	}{
		{
			name: "scheme-prefixed story-ref arg, single owner",
			in: AttestationScaffold{
				StorySlug:   "jira-loan-1482",
				ACID:        "ac-2",
				StoryRefArg: "jira:LOAN-1482",
				VerifiesRef: "spec/borrower-update-api",
				Owners:      []string{"platform-team"},
				Frozen:      artifact.Frozen{At: "2026-07-16", Commit: "e606a109dbc28ea08cc86265c4fa2dd026f8373a"},
			},
		},
		{
			name: "bare spec-ref story-ref arg, multiple owners",
			in: AttestationScaffold{
				StorySlug:   "borrower-update-api",
				ACID:        "ac-1",
				StoryRefArg: "spec/borrower-update-api",
				VerifiesRef: "spec/borrower-update-api",
				Owners:      []string{"platform-team", "qa-lead"},
				Frozen:      artifact.Frozen{At: "2026-07-16", Commit: "e606a109dbc28ea08cc86265c4fa2dd026f8373a"},
			},
		},
	}
}

// TestRenderAttestationScaffold_FrontmatterShape proves AC-1's exact
// frontmatter shape: id, kind, schema, owners copied VERBATIM (never
// invented, never an [unassigned] placeholder), a single bare verifies
// edge, a frozen stamp, and an identifier-shaped (never claim-shaped)
// title — plus a body that is exactly the marker followed by instructional
// prose naming the (story-ref, ac-id) pair. No case may show a generated,
// defaulted claim.
func TestRenderAttestationScaffold_FrontmatterShape(t *testing.T) {
	for _, tc := range attestationScaffoldCases() {
		t.Run(tc.name, func(t *testing.T) {
			content := RenderAttestationScaffold(tc.in)

			fm, bodyBytes, err := artifact.SplitFrontmatter([]byte(content))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
			}
			decoded, err := artifact.DecodeAttestation(fm)
			if err != nil {
				t.Fatalf("DecodeAttestation: %v\ncontent:\n%s", err, content)
			}
			body := string(bodyBytes)

			wantID := "attestation/" + tc.in.StorySlug + "--" + tc.in.ACID
			if decoded.ID != wantID {
				t.Errorf("id = %q, want %q", decoded.ID, wantID)
			}
			if decoded.Kind != artifact.KindAttestation {
				t.Errorf("kind = %q, want %q", decoded.Kind, artifact.KindAttestation)
			}
			if decoded.Schema != "verdi.attestation/v1" {
				t.Errorf("schema = %q, want verdi.attestation/v1", decoded.Schema)
			}
			if len(decoded.Owners) != len(tc.in.Owners) {
				t.Fatalf("owners = %v, want %v (verbatim copy)", decoded.Owners, tc.in.Owners)
			}
			for i, want := range tc.in.Owners {
				if decoded.Owners[i] != want {
					t.Errorf("owners[%d] = %q, want %q — owners must be copied verbatim, never invented, never [unassigned]", i, decoded.Owners[i], want)
				}
			}
			if len(decoded.Links) != 1 {
				t.Fatalf("links = %+v, want exactly one entry", decoded.Links)
			}
			if decoded.Links[0].Type != artifact.LinkVerifies || decoded.Links[0].Ref != tc.in.VerifiesRef {
				t.Errorf("links[0] = %+v, want a bare verifies edge to %q", decoded.Links[0], tc.in.VerifiesRef)
			}
			if decoded.Frozen == nil || decoded.Frozen.At != tc.in.Frozen.At || decoded.Frozen.Commit != tc.in.Frozen.Commit {
				t.Errorf("frozen = %+v, want %+v", decoded.Frozen, tc.in.Frozen)
			}

			// Title is mechanically derived (identifier-shaped), never
			// claim-shaped prose (parent spec/closure-ergonomics dc-2): it
			// names the story-ref arg and ac-id verbatim and must never
			// read like a first-person claim.
			if !strings.Contains(decoded.Title, tc.in.StoryRefArg) || !strings.Contains(decoded.Title, tc.in.ACID) {
				t.Errorf("title = %q, want it to name %q and %q", decoded.Title, tc.in.StoryRefArg, tc.in.ACID)
			}
			for _, claimWord := range []string{"verified", "confirmed", "observed", "I ", "satisfied"} {
				if strings.Contains(decoded.Title, claimWord) {
					t.Errorf("title = %q reads as claim-shaped prose (contains %q) — dc-2 forbids this", decoded.Title, claimWord)
				}
			}

			// Body is exactly the marker, then fixed instructional prose —
			// never a generated claim (parent dc-2).
			if !strings.HasPrefix(body, UnauthoredAttestationMarker) {
				t.Errorf("body does not start with the unauthored marker:\n%s", body)
			}
			if !strings.Contains(body, tc.in.StoryRefArg) || !strings.Contains(body, tc.in.ACID) {
				t.Errorf("body does not name the (story-ref, ac-id) it was scaffolded for:\n%s", body)
			}
			if !strings.Contains(body, "verdi attest") {
				t.Errorf("body does not name the verb that scaffolded it:\n%s", body)
			}
		})
	}
}

// storyClassGoldenBytes pins the exact story-class rendering byte-for-byte
// (R-RR2-1): captured from HEAD, before this change ever touches
// RenderAttestationScaffold, so TestRenderAttestationScaffold_StoryBytesUnchanged
// proves the story render's contract (spec/attest-helper dc-2's byte
// contract, the showcase proof, TestRenderAttestationScaffold_FrontmatterShape's
// own goldens) is untouched by the feature-class addition below.
var storyClassGoldenBytes = map[string]string{
	"scheme-prefixed story-ref arg, single owner":  "---\nid: attestation/jira-loan-1482--ac-2\nkind: attestation\ntitle: \"unauthored attestation scaffold: jira:LOAN-1482 ac-2\"\nowners: [\"platform-team\"]\nschema: verdi.attestation/v1\nlinks:\n  - { type: verifies, ref: \"spec/borrower-update-api\" }\nfrozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a }\n---\n<!-- verdi:attestation-unauthored -->\nThis attestation was scaffolded by `verdi attest` for jira:LOAN-1482 ac-2\nand has not been authored. Replace this entire paragraph, and delete the\nmarker comment above, with your own first-person account of what you\nverified, how, and why this acceptance criterion is satisfied. Until the\nmarker above is removed, this file folds as absent, with disclosure — it\nis not evidence of anything.\n\nThe `frozen.commit` stamped above is a convenience: it was pre-filled with\nthe repository HEAD when this scaffold was written. By the store's\nattestation convention that field names the tree your claim was verified\nagainst — not this file's own commit — so set it to the exact commit you\nactually reviewed when you author your claim. The stamp is yours to\ncorrect: nothing here is frozen until this file's first commit (VL-010\nbinds only committed frozen artifacts), so updating it in this same\nauthoring pass is always legitimate.\n",
	"bare spec-ref story-ref arg, multiple owners": "---\nid: attestation/borrower-update-api--ac-1\nkind: attestation\ntitle: \"unauthored attestation scaffold: spec/borrower-update-api ac-1\"\nowners: [\"platform-team\", \"qa-lead\"]\nschema: verdi.attestation/v1\nlinks:\n  - { type: verifies, ref: \"spec/borrower-update-api\" }\nfrozen: { at: 2026-07-16, commit: e606a109dbc28ea08cc86265c4fa2dd026f8373a }\n---\n<!-- verdi:attestation-unauthored -->\nThis attestation was scaffolded by `verdi attest` for spec/borrower-update-api ac-1\nand has not been authored. Replace this entire paragraph, and delete the\nmarker comment above, with your own first-person account of what you\nverified, how, and why this acceptance criterion is satisfied. Until the\nmarker above is removed, this file folds as absent, with disclosure — it\nis not evidence of anything.\n\nThe `frozen.commit` stamped above is a convenience: it was pre-filled with\nthe repository HEAD when this scaffold was written. By the store's\nattestation convention that field names the tree your claim was verified\nagainst — not this file's own commit — so set it to the exact commit you\nactually reviewed when you author your claim. The stamp is yours to\ncorrect: nothing here is frozen until this file's first commit (VL-010\nbinds only committed frozen artifacts), so updating it in this same\nauthoring pass is always legitimate.\n",
}

// TestRenderAttestationScaffold_StoryBytesUnchanged is R-RR2-1's own
// witness: the story render (Class: story, and the empty-Class zero value
// so untouched callers stay byte-stable) produces bytes IDENTICAL to
// storyClassGoldenBytes above, captured from HEAD before this feature-class
// addition. A story-render byte drift here is a merge blocker on its own.
func TestRenderAttestationScaffold_StoryBytesUnchanged(t *testing.T) {
	for _, tc := range attestationScaffoldCases() {
		t.Run(tc.name, func(t *testing.T) {
			want, ok := storyClassGoldenBytes[tc.name]
			if !ok {
				t.Fatalf("test setup: no golden captured for case %q", tc.name)
			}
			got := RenderAttestationScaffold(tc.in)
			if got != want {
				t.Fatalf("story-class render bytes changed (R-RR2-1 pins them byte-for-byte):\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

// TestRenderAttestationScaffold_FeatureQuotesTheCriterion proves ac-6/
// R-RR2-1: a Class: feature scaffold carries the exact same unauthored
// marker and fixed instructional prose as the story form, plus an appended,
// clearly-labeled quoted-context section — the criterion's own accepted
// text and its declared evidence kinds — that can never be mistaken for the
// claim itself (dc-2's never-a-claim contract extended to quoted context).
func TestRenderAttestationScaffold_FeatureQuotesTheCriterion(t *testing.T) {
	in := AttestationScaffold{
		StorySlug:     "loan-workflow",
		ACID:          "ac-1",
		StoryRefArg:   "spec/loan-workflow",
		VerifiesRef:   "spec/loan-workflow",
		Owners:        []string{"platform-team"},
		Frozen:        artifact.Frozen{At: "2026-07-16", Commit: "e606a109dbc28ea08cc86265c4fa2dd026f8373a"},
		Class:         artifact.ClassFeature,
		CriterionText: "the ledger rejects a decline older than 30 days",
		EvidenceKinds: []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceAttestation},
	}
	content := RenderAttestationScaffold(in)

	fm, bodyBytes, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
	}
	decoded, err := artifact.DecodeAttestation(fm)
	if err != nil {
		t.Fatalf("DecodeAttestation: %v\ncontent:\n%s", err, content)
	}
	body := string(bodyBytes)

	if wantID := "attestation/loan-workflow--ac-1"; decoded.ID != wantID {
		t.Errorf("id = %q, want %q", decoded.ID, wantID)
	}
	if wantTitle := "unauthored outcome attestation scaffold: spec/loan-workflow ac-1"; decoded.Title != wantTitle {
		t.Errorf("title = %q, want %q", decoded.Title, wantTitle)
	}

	markerIdx := strings.Index(body, UnauthoredAttestationMarker)
	if markerIdx != 0 {
		t.Fatalf("body does not start with the unauthored marker:\n%s", body)
	}
	if !strings.Contains(body, "This attestation was scaffolded by `verdi attest`") {
		t.Errorf("body dropped the fixed instructional prose:\n%s", body)
	}

	sectionIdx := strings.Index(body, "Criterion under attestation (accepted text, quoted for context):")
	if sectionIdx == -1 {
		t.Fatalf("body missing the quoted-criterion section header:\n%s", body)
	}
	if sectionIdx < markerIdx {
		t.Fatalf("quoted-criterion section appears before the unauthored marker:\n%s", body)
	}
	if !strings.Contains(body, "> the ledger rejects a decline older than 30 days") {
		t.Errorf("body does not quote the criterion text on its own quoted line:\n%s", body)
	}
	if !strings.Contains(body, "Declared evidence kinds: static, attestation") {
		t.Errorf("body missing the declared evidence kinds line:\n%s", body)
	}

	if strings.Contains(body, "I verified") || strings.Contains(body, "observed in staging") {
		t.Errorf("body contains claim-shaped prose — dc-2 forbids this:\n%s", body)
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "The outcome") {
			t.Errorf("body contains a claim-shaped sentence starting with %q — dc-2 forbids this:\n%s", "The outcome", body)
		}
	}
}

// TestRenderAttestationScaffold_FeatureEscapesCriterionText proves the
// quoted section never lets a criterion's own text reopen a second
// frontmatter block or otherwise get interpreted: a criterion carrying a
// bare "---" line, backticks, and a YAML-looking "key: value" line all
// render as inert, "> "-quoted body prose — SplitFrontmatter must still
// find exactly one frontmatter/body split.
func TestRenderAttestationScaffold_FeatureEscapesCriterionText(t *testing.T) {
	tricky := "line one\n---\nkey: value\n`backtick`"
	in := AttestationScaffold{
		StorySlug:     "tricky-feature",
		ACID:          "ac-1",
		StoryRefArg:   "spec/tricky-feature",
		VerifiesRef:   "spec/tricky-feature",
		Owners:        []string{"platform-team"},
		Frozen:        artifact.Frozen{At: "2026-07-16", Commit: "e606a109dbc28ea08cc86265c4fa2dd026f8373a"},
		Class:         artifact.ClassFeature,
		CriterionText: tricky,
		EvidenceKinds: []artifact.EvidenceKind{artifact.EvidenceAttestation},
	}
	content := RenderAttestationScaffold(in)

	fm, bodyBytes, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
	}
	if _, err := artifact.DecodeAttestation(fm); err != nil {
		t.Fatalf("DecodeAttestation: %v\ncontent:\n%s", err, content)
	}
	body := string(bodyBytes)

	for _, line := range strings.Split(tricky, "\n") {
		if !strings.Contains(body, "> "+line) {
			t.Errorf("body does not quote criterion line %q on its own quoted line:\n%s", line, body)
		}
	}

	// The tricky "---" line must render only as the quoted "> ---" — never
	// a second bare frontmatter delimiter line — so the whole document
	// still carries exactly one frontmatter/body split.
	if delimCount := strings.Count(content, "\n---\n"); delimCount != 1 {
		t.Fatalf("content contains %d bare \"---\" delimiter lines, want exactly 1 (the criterion's own \"---\" line must stay quoted, never a second frontmatter delimiter):\n%s", delimCount, content)
	}
}

// TestRenderAttestationScaffold_SelfValidates is spec/attest-helper AC-4's
// own static register: the rendered bytes always strict-decode and
// validate as kind: attestation frontmatter WHILE THE UNAUTHORED MARKER IS
// STILL PRESENT — i.e. before any claim is ever authored — so a malformed
// scaffold shape is caught at the rendering seam itself, not only after a
// write.
func TestRenderAttestationScaffold_SelfValidates(t *testing.T) {
	for _, tc := range attestationScaffoldCases() {
		t.Run(tc.name, func(t *testing.T) {
			content := RenderAttestationScaffold(tc.in)
			if !strings.Contains(content, UnauthoredAttestationMarker) {
				t.Fatalf("rendered content lost the unauthored marker:\n%s", content)
			}
			fm, _, err := artifact.SplitFrontmatter([]byte(content))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v\ncontent:\n%s", err, content)
			}
			if _, err := artifact.DecodeAttestation(fm); err != nil {
				t.Fatalf("DecodeAttestation: %v\ncontent:\n%s", err, content)
			}
		})
	}
}
