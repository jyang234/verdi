package humanartifact

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ExtensionType is the closed set of value types a model may declare for
// a human-artifact template extension field (AC-1: "The model may
// declare typed extension fields... but a template cannot remove,
// rename, retype, or synthesize kernel fields"). There is deliberately
// no untyped/any escape hatch — unknown types fail closed, mirroring
// internal/policyartifact's own no-untyped-payload-fallback posture
// (payload.go: "There is deliberately NO untyped map fallback").
type ExtensionType string

// The closed ExtensionType enum. Any other value fails Contract.Validate
// closed.
const (
	ExtensionString     ExtensionType = "string"
	ExtensionStringList ExtensionType = "string-list"
	ExtensionBool       ExtensionType = "bool"
	ExtensionInt        ExtensionType = "int"
)

func (t ExtensionType) valid() bool {
	switch t {
	case ExtensionString, ExtensionStringList, ExtensionBool, ExtensionInt:
		return true
	default:
		return false
	}
}

// ExtensionField is one model-declared extension slot: a name outside
// the kind's immutable kernel (KernelFields), and its closed value type.
type ExtensionField struct {
	Name string
	Type ExtensionType
}

// extensionNameRe is the snake_case/kebab-case identifier grammar an
// extension field name must match: alphanumeric segments joined by
// single underscore or hyphen separators, no leading/trailing or
// doubled separator. Widened from policyartifact's kebab-only,
// lowercase-only identifier grammar (kebabRe) in two ways: underscores
// are accepted too (a template extension field is closer kin to a typed
// payload's own field naming — design_assistance's "mode"/"layout" —
// than to a kebab-only artifact id), and case is not itself restricted,
// since Contract.Validate folds case before comparing against a kind's
// kernel field names — an author who spells an extension "Title" is
// caught by that fold as a collision with kernel field "title", not
// admitted as a differently-cased grammar violation that would mask the
// real conflict.
var extensionNameRe = regexp.MustCompile(`^[A-Za-z0-9]+([_-][A-Za-z0-9]+)*$`)

// Contract is one artifact kind's extension surface: the kind name and
// its model-declared extension fields, layered on top of that kind's
// immutable kernel.
type Contract struct {
	Kind       string
	Extensions []ExtensionField
}

// Validate checks the contract's own grammar and — the AC-1 anti-
// synthesis proof this type exists for — that no extension name equals
// or case-folds to any kernel field name of Kind: shadowing, renaming,
// retyping, or synthesizing a kernel field is structurally rejected
// here, before any template or registration ever runs. Kind must name a
// recognized artifact family (KernelFields); an unrecognized kind fails
// closed rather than being treated as carrying no kernel at all.
func (c Contract) Validate() error {
	if c.Kind == "" {
		return fmt.Errorf("humanartifact: contract kind is required")
	}
	kernel, ok := KernelFields(c.Kind)
	if !ok {
		return fmt.Errorf("humanartifact: contract kind %q names no recognized artifact family (unrecognized artifact family fails closed)", c.Kind)
	}
	kernelFold := make(map[string]bool, len(kernel))
	for _, k := range kernel {
		kernelFold[strings.ToLower(k)] = true
	}

	seen := make(map[string]bool, len(c.Extensions))
	for _, ext := range c.Extensions {
		if ext.Name == "" {
			return fmt.Errorf("humanartifact: contract %q: extension field name must not be empty", c.Kind)
		}
		if !extensionNameRe.MatchString(ext.Name) {
			return fmt.Errorf("humanartifact: contract %q: extension field name %q must be a snake_case or kebab-case identifier", c.Kind, ext.Name)
		}
		if !ext.Type.valid() {
			return fmt.Errorf("humanartifact: contract %q: extension field %q has unknown type %q (known: string, string-list, bool, int)", c.Kind, ext.Name, ext.Type)
		}
		fold := strings.ToLower(ext.Name)
		if seen[fold] {
			return fmt.Errorf("humanartifact: contract %q: duplicate extension field %q", c.Kind, ext.Name)
		}
		seen[fold] = true
		if kernelFold[fold] {
			return fmt.Errorf("humanartifact: contract %q: extension field %q shadows kernel field %q — a template cannot remove, rename, retype, or synthesize kernel fields (AC-1)", c.Kind, ext.Name, ext.Name)
		}
	}
	return nil
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Contract{}
)

// RegisterContract registers kind's extension contract, at init time
// only (mirroring internal/policyartifact.RegisterPayloadKind's posture,
// payload.go). An invalid contract (Contract.Validate fails) or a kind
// registered twice is a programming error and panics — a bad contract
// must never reach runtime silently.
func RegisterContract(c Contract) {
	if err := c.Validate(); err != nil {
		panic("humanartifact: " + err.Error())
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[c.Kind]; dup {
		panic(fmt.Sprintf("humanartifact: contract kind %q registered twice", c.Kind))
	}
	registry[c.Kind] = c
}

// ContractFor returns kind's registered contract, if any.
func ContractFor(kind string) (Contract, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	c, ok := registry[kind]
	return c, ok
}
