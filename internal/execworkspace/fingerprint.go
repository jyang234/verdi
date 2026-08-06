package execworkspace

// Environment fingerprint collection for spec/execution-workspace
// §Environment fingerprint collection (ledger SI-13, controller decision
// AD-8). OD-6 ratifies that this component owns fingerprint COLLECTION;
// SI-13 ratifies the collected field set — the consumer-demand
// intersection: OS/architecture, tool/adapter versions, declared
// environment variables, input digests.
//
// AD-8: fingerprint inputs are CALLER-DECLARED (this package never probes
// the real world for tool versions or picks which env vars matter — the
// caller supplies both), and the output is canonical sorted JSON carrying
// NO schema field. That omission is deliberate, not an oversight: spec
// §Environment fingerprint collection states "COLLECTION IS SHARED;
// SCHEMAS ARE NOT" — CSE's own fingerprint schema and CI's manifest fields
// each embed this output wholesale as a feature-owned SUPERSET. A schema
// tag on the shared collection payload itself would either collide with,
// or have to be stripped ahead of, each feature's own outer schema tag; so
// this package defines only the shared field set and leaves schema framing
// entirely to the two consumers.
//
// Network policy and resource allocation are NOT collected here a second
// time (spec, same section): CSE's own fingerprint enumeration lists them,
// but they reach CSE as this package's EnforcementReport facts
// (isolation.go's could-and-could-not-apply report), embedded by CSE's own
// superset schema — never a second collection path.

import (
	"encoding/hex"
	"fmt"
	"runtime"

	"github.com/jyang234/verdi/internal/canonjson"
)

// FingerprintInputs is CollectFingerprint's caller-declared input surface
// (AD-8). ToolVersions and InputDigests are caller-supplied name->value
// maps; EnvVarNames is the list of environment-variable NAMES the caller
// wants recorded in the output.
type FingerprintInputs struct {
	// ToolVersions maps a tool/adapter name to its version string. Every
	// name and every version must be non-empty.
	ToolVersions map[string]string
	// EnvVarNames is the set of environment-variable names to resolve and
	// record. Each is resolved against the constructed Profile's OWN
	// environment — never the real process environment (see
	// CollectFingerprint's doc comment for why) — and recorded explicitly
	// as present or absent.
	EnvVarNames []string
	// InputDigests maps an input name to its content digest, a non-empty
	// hex string (e.g. a workload or fixture digest).
	InputDigests map[string]string
}

// fingerprintDoc is CollectFingerprint's canonical-JSON output shape.
// Deliberately schema-less (see this file's package doc comment). Every
// map field is always a non-nil, possibly-empty map so an empty
// FingerprintInputs still yields "{}" objects, never "null" — CollectFing-
// erprint's output shape never varies with whether an input collection was
// nil or merely empty.
type fingerprintDoc struct {
	OS           string             `json:"os"`
	Arch         string             `json:"arch"`
	ToolVersions map[string]string  `json:"tool_versions"`
	Env          map[string]*string `json:"env"`
	InputDigests map[string]string  `json:"input_digests"`
}

// CollectFingerprint collects an environment fingerprint for profile and
// declared, and renders it as canonical sorted JSON (internal/canonjson):
// sorted keys, no HTML escaping, a trailing newline, and identical bytes
// for identical inputs regardless of Go map iteration order.
//
// Output fields: os (runtime.GOOS), arch (runtime.GOARCH), tool_versions,
// env, input_digests.
//
// ENV VARS ARE RESOLVED AGAINST THE PROFILE, NEVER THE REAL PROCESS
// ENVIRONMENT: this is the same clean-environment discipline isolation.go's
// BuildProfile enforces for a launched process, applied here to the
// fingerprint that describes it — a fingerprint drawn from the real
// process environment could record ambient state the isolated process
// itself never saw, silently widening what "environment" means for this
// run. A requested name absent from profile's environment is recorded
// EXPLICITLY as JSON null (never omitted, and never conflated with an
// empty-string value), so the shape of what was asked for is always
// visible in the output.
//
// Fails closed on: an empty tool-version name or value; an empty
// input-digest name; an input digest that is not non-empty valid hex; an
// empty env-var name.
func CollectFingerprint(profile Profile, declared FingerprintInputs) ([]byte, error) {
	toolVersions := make(map[string]string, len(declared.ToolVersions))
	for name, version := range declared.ToolVersions {
		if name == "" {
			return nil, fmt.Errorf("execworkspace: collect fingerprint: tool version name is empty")
		}
		if version == "" {
			return nil, fmt.Errorf("execworkspace: collect fingerprint: tool %q: version is empty", name)
		}
		toolVersions[name] = version
	}

	inputDigests := make(map[string]string, len(declared.InputDigests))
	for name, digest := range declared.InputDigests {
		if name == "" {
			return nil, fmt.Errorf("execworkspace: collect fingerprint: input digest name is empty")
		}
		if err := validateHexDigest(digest); err != nil {
			return nil, fmt.Errorf("execworkspace: collect fingerprint: input %q: %w", name, err)
		}
		inputDigests[name] = digest
	}

	env := make(map[string]*string, len(declared.EnvVarNames))
	for _, name := range declared.EnvVarNames {
		if name == "" {
			return nil, fmt.Errorf("execworkspace: collect fingerprint: env var name is empty")
		}
		if value, ok := profile.env[name]; ok {
			v := value
			env[name] = &v
		} else {
			env[name] = nil
		}
	}

	doc := fingerprintDoc{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		ToolVersions: toolVersions,
		Env:          env,
		InputDigests: inputDigests,
	}
	data, err := canonjson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("execworkspace: collect fingerprint: %w", err)
	}
	return data, nil
}

// validateHexDigest reports whether digest is non-empty valid hex.
func validateHexDigest(digest string) error {
	if digest == "" {
		return fmt.Errorf("digest is empty")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("digest %q is not valid hex: %w", digest, err)
	}
	return nil
}
