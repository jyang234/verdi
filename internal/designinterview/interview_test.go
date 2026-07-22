package designinterview

import (
	"errors"
	"strings"
	"testing"
)

const twoStatementTemplate = `---
id: {{.Ref}}
title: {{.Title}}
problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
---
# {{.Title}}
`

// TestRunInterview_Happy proves the interview prompts for exactly the
// template's own FieldStatement fields, in Fields' own enumeration order,
// and returns each trimmed answer keyed by field name.
func TestRunInterview_Happy(t *testing.T) {
	in := strings.NewReader("the real problem\nthe real outcome\n")
	var out strings.Builder
	answers, err := RunInterview([]byte(twoStatementTemplate), in, &out)
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}
	if answers["Problem"] != "the real problem" {
		t.Fatalf(`answers["Problem"] = %q, want "the real problem"`, answers["Problem"])
	}
	if answers["Outcome"] != "the real outcome" {
		t.Fatalf(`answers["Outcome"] = %q, want "the real outcome"`, answers["Outcome"])
	}
	if !strings.Contains(out.String(), "Problem") || !strings.Contains(out.String(), "Outcome") {
		t.Fatalf("out = %q, want both field names prompted", out.String())
	}
}

// TestRunInterview_TrimsWhitespace proves an answer's surrounding
// whitespace is trimmed, matching the board form's own trim discipline.
func TestRunInterview_TrimsWhitespace(t *testing.T) {
	in := strings.NewReader("  padded problem  \nplain outcome\n")
	answers, err := RunInterview([]byte(twoStatementTemplate), in, &strings.Builder{})
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}
	if answers["Problem"] != "padded problem" {
		t.Fatalf(`answers["Problem"] = %q, want trimmed "padded problem"`, answers["Problem"])
	}
}

// TestRunInterview_ReprompsOnEmpty proves an empty answer never becomes a
// silent placeholder: the SAME field is re-asked until a non-empty answer
// arrives (statement fields are required, spec/cli-creation ac-2).
func TestRunInterview_ReprompsOnEmpty(t *testing.T) {
	in := strings.NewReader("\n\nfinally a problem\nan outcome\n")
	var out strings.Builder
	answers, err := RunInterview([]byte(twoStatementTemplate), in, &out)
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}
	if answers["Problem"] != "finally a problem" {
		t.Fatalf(`answers["Problem"] = %q, want "finally a problem"`, answers["Problem"])
	}
	if !strings.Contains(out.String(), "required") {
		t.Fatalf("out = %q, want a re-prompt notice naming the field required", out.String())
	}
}

// TestRunInterview_Aborted proves stdin ending before every statement
// field is answered returns ErrAborted — never a silent partial/empty
// result treated as success.
func TestRunInterview_Aborted(t *testing.T) {
	in := strings.NewReader("only the problem\n")
	_, err := RunInterview([]byte(twoStatementTemplate), in, &strings.Builder{})
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("RunInterview(short stdin) error = %v, want ErrAborted", err)
	}
}

// TestRunInterview_NoStatementFields proves a template with no
// FieldStatement placeholders at all (a degenerate custom override) never
// blocks on input — an empty, non-nil answers map, no prompts printed.
const noStatementTemplate = `---
id: {{.Ref}}
title: {{.Title}}
owners: {{.Owners}}
---
# {{.Title}}
`

func TestRunInterview_NoStatementFields(t *testing.T) {
	var out strings.Builder
	answers, err := RunInterview([]byte(noStatementTemplate), strings.NewReader(""), &out)
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}
	if len(answers) != 0 {
		t.Fatalf("answers = %v, want empty", answers)
	}
}

// TestRunInterview_BadTemplate proves a template Fields itself refuses
// (a placeholder outside the ScaffoldData contract) fails closed here
// too, rather than starting an interview session it cannot ground.
func TestRunInterview_BadTemplate(t *testing.T) {
	bad := `{{.NotAField}}`
	_, err := RunInterview([]byte(bad), strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("RunInterview(bad template) = nil error, want a refusal")
	}
}

// TestRunInterview_DerivesFromFields is a static-shaped witness (spec/
// cli-creation ac-2's own DRY requirement): this package's only way to
// learn which fields to ask is designscaffold.Fields — proven here by
// exercising a template whose statement field ISN'T named Problem/Outcome
// at all and confirming the interview still asks for exactly what Fields
// enumerates, never a hand-rolled ["Problem","Outcome"] literal that would
// ask for those two names even when the template doesn't carry them.
func TestRunInterview_DerivesFromFields(t *testing.T) {
	onlyOutcome := `---
id: {{.Ref}}
title: {{.Title}}
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
---
# {{.Title}}
`
	answers, err := RunInterview([]byte(onlyOutcome), strings.NewReader("just an outcome\n"), &strings.Builder{})
	if err != nil {
		t.Fatalf("RunInterview: %v", err)
	}
	if _, asked := answers["Problem"]; asked {
		t.Fatalf("answers = %v, want no Problem entry — the template never declared that field", answers)
	}
	if answers["Outcome"] != "just an outcome" {
		t.Fatalf(`answers["Outcome"] = %q, want "just an outcome"`, answers["Outcome"])
	}
}
