package draftmutation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

type semanticValue struct {
	Digest       string
	ObjectDigest string
	Text         string
}

type semanticSnapshot map[string]semanticValue

// PercentComponent applies RFC 3986 component encoding over UTF-8 bytes with
// uppercase hexadecimal escapes, leaving only ASCII unreserved bytes literal.
func PercentComponent(value string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		b := value[i]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || strings.ContainsRune("-._~", rune(b)) {
			out.WriteByte(b)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hex[b>>4])
		out.WriteByte(hex[b&15])
	}
	return out.String()
}

func digestSemantic(value any) (string, error) {
	digest, err := canonjson.Digest(value)
	if err != nil {
		return "", fmt.Errorf("draftmutation: digesting semantic object: %w", err)
	}
	return digest, nil
}

func snapshot(specBytes []byte) (semanticSnapshot, error) {
	frontmatter, _, err := artifact.SplitFrontmatter(specBytes)
	if err != nil {
		return nil, err
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return nil, err
	}
	result := semanticSnapshot{}
	put := func(target string, value any, text string) error {
		digest, err := digestSemantic(value)
		if err != nil {
			return err
		}
		result[target] = semanticValue{Digest: digest, ObjectDigest: digest, Text: text}
		return nil
	}
	putOrdered := func(target string, position int, value any, text string) error {
		objectDigest, err := digestSemantic(value)
		if err != nil {
			return err
		}
		digest, err := digestSemantic(struct {
			Position int `json:"position"`
			Value    any `json:"value"`
		}{position, value})
		if err != nil {
			return err
		}
		result[target] = semanticValue{Digest: digest, ObjectDigest: objectDigest, Text: text}
		return nil
	}
	if spec.Problem != nil {
		if err := put("problem", *spec.Problem, spec.Problem.Text); err != nil {
			return nil, err
		}
	}
	if spec.Outcome != nil {
		if err := put("outcome", *spec.Outcome, spec.Outcome.Text); err != nil {
			return nil, err
		}
	}
	for index, value := range spec.AcceptanceCriteria {
		if err := putOrdered(value.ID, index, value, value.Text); err != nil {
			return nil, err
		}
	}
	for _, value := range spec.Constraints {
		if err := put(value.ID, value, value.Text); err != nil {
			return nil, err
		}
	}
	for _, value := range spec.Decisions {
		if err := put(value.ID, value, value.Text); err != nil {
			return nil, err
		}
	}
	for _, value := range spec.OpenQuestions {
		if err := put(value.ID, value, value.Text); err != nil {
			return nil, err
		}
	}
	for index, value := range spec.Stubs {
		target := "stub/" + PercentComponent(value.Slug)
		if err := putOrdered(target, index, value, ""); err != nil {
			return nil, err
		}
	}
	for _, ref := range spec.Context {
		target := "context/" + PercentComponent(ref)
		if err := put(target, ref, ""); err != nil {
			return nil, err
		}
	}
	linkGroups := map[string][]artifact.Link{}
	for _, link := range spec.Links {
		target := linkTarget("spec", link.Type, link.Ref)
		linkGroups[target] = append(linkGroups[target], link)
	}
	for _, decision := range spec.Decisions {
		for _, link := range decision.Links {
			target := linkTarget(decision.ID, link.Type, link.Ref)
			linkGroups[target] = append(linkGroups[target], link)
		}
	}
	for target, links := range linkGroups {
		sort.Slice(links, func(i, j int) bool {
			left := string(links[i].Type) + "\x00" + links[i].Ref + "\x00" + links[i].Note
			right := string(links[j].Type) + "\x00" + links[j].Ref + "\x00" + links[j].Note
			return left < right
		})
		if err := put(target, links, ""); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func linkTarget(source string, linkType artifact.LinkType, ref string) string {
	return "link/" + PercentComponent(source) + "/" + PercentComponent(string(linkType)) + "/" + PercentComponent(ref)
}

func operationTarget(operation Operation) string {
	switch operation.Op {
	case OpSetProblem:
		return "problem"
	case OpSetOutcome:
		return "outcome"
	case OpAddAC, OpEditAC, OpRemoveAC, OpReorderAC, OpSetACEvidence,
		OpAddConstraint, OpEditConstraint, OpRemoveConstraint,
		OpAddDecision, OpEditDecision, OpRemoveDecision,
		OpAddQuestion, OpEditQuestion, OpRemoveQuestion:
		return operation.ID
	case OpAddLink, OpRemoveLink:
		return linkTarget(operation.Source, operation.Type, operation.Ref)
	case OpAddStub, OpEditStub, OpRemoveStub, OpReorderStub:
		return "stub/" + PercentComponent(operation.Slug)
	case OpAddContextRef, OpRemoveContextRef:
		return "context/" + PercentComponent(operation.Ref)
	default:
		return ""
	}
}

func changedSnapshotTargets(left, right semanticSnapshot) []string {
	seen := map[string]bool{}
	for target, value := range left {
		if current, ok := right[target]; !ok || current.Digest != value.Digest {
			seen[target] = true
		}
	}
	for target, value := range right {
		if previous, ok := left[target]; !ok || previous.Digest != value.Digest {
			seen[target] = true
		}
	}
	targets := make([]string, 0, len(seen))
	for target := range seen {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

// ChangedTargets returns every semantic target whose canonical value or
// presence differs, sorted by canonical target identity for stale refusals.
func ChangedTargets(before, after []byte) ([]string, error) {
	left, err := snapshot(before)
	if err != nil {
		return nil, fmt.Errorf("draftmutation: decoding base snapshot: %w", err)
	}
	right, err := snapshot(after)
	if err != nil {
		return nil, fmt.Errorf("draftmutation: decoding current snapshot: %w", err)
	}
	return changedSnapshotTargets(left, right), nil
}
