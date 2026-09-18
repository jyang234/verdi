package specdoc

import (
	"fmt"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// Input is everything Build needs. Spec and Body are the decoded halves of
// one spec.md; Status is the effective lifecycle status id the caller
// resolved (empty when unresolved); Facts may be the zero value.
type Input struct {
	Spec   *artifact.SpecFrontmatter
	Body   []byte
	Status string
	Stamp  Stamp
	Facts  Facts
	Model  *model.Model
	Kind   Kind
}

var fullShaRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// Build resolves an Input into a Document. It validates the stamp, fills
// Stamp.Engine, resolves display words through the model, and copies
// facts onto each object in declaration order.
func Build(in Input) (Document, error) {
	if in.Spec == nil {
		return Document{}, fmt.Errorf("specdoc: nil spec")
	}
	if _, err := ParseKind(string(in.Kind)); err != nil {
		return Document{}, err
	}
	if in.Stamp.Ref == "" {
		return Document{}, fmt.Errorf("specdoc: stamp has no ref")
	}
	if !fullShaRe.MatchString(in.Stamp.Commit) {
		return Document{}, fmt.Errorf("specdoc: stamp commit %q is not a full sha", in.Stamp.Commit)
	}

	sections := bodySections(in.Body)
	doc := Document{
		Kind:     in.Kind,
		Stamp:    in.Stamp,
		Sections: in.Kind.Sections(),
		Words: Words{
			Feature: in.Model.DisplayClass(string(artifact.ClassFeature)),
			Story:   in.Model.DisplayClass(string(artifact.ClassStory)),
			Spike:   in.Model.DisplayClass("spike"),
		},
		Title: in.Spec.Title,
	}
	doc.Stamp.Engine = EngineDigest()

	doc.Identity = identityRows(in)

	if in.Spec.Problem != nil {
		doc.ProblemKnown = true
		doc.ProblemText = in.Spec.Problem.Text
	}
	if in.Spec.Outcome != nil {
		doc.OutcomeKnown = true
		doc.OutcomeText = in.Spec.Outcome.Text
	}
	for _, d := range in.Spec.Decisions {
		doc.Decisions = append(doc.Decisions, Item{ID: d.ID, Text: d.Text, Detail: sections[d.ID]})
	}
	for _, c := range in.Spec.Constraints {
		doc.Constraints = append(doc.Constraints, Item{ID: c.ID, Text: c.Text, Detail: sections[c.ID]})
	}
	for _, ac := range in.Spec.AcceptanceCriteria {
		cr := Criterion{ID: ac.ID, Text: ac.Text, Detail: sections[ac.ID]}
		for _, e := range ac.Evidence {
			cr.Evidence = append(cr.Evidence, string(e))
		}
		if in.Facts.Coverage != nil {
			cr.CoverageKnown = true
			cr.Coverage = append([]string{}, in.Facts.Coverage[ac.ID]...)
		}
		doc.Criteria = append(doc.Criteria, cr)
	}
	for _, oq := range in.Spec.OpenQuestions {
		q := Question{ID: oq.ID, Text: oq.Text, Detail: sections[oq.ID]}
		if in.Facts.Claims != nil {
			q.ClaimsKnown = true
			q.Claims = append([]string{}, in.Facts.Claims[oq.ID]...)
		}
		doc.Questions = append(doc.Questions, q)
	}
	for _, st := range in.Spec.Stubs {
		doc.Plan = append(doc.Plan, PlanItem{
			Slug:     st.Slug,
			Spike:    st.Spike,
			Criteria: append([]string(nil), st.AcceptanceCriteria...),
			Resolves: append([]string(nil), st.Resolves...),
		})
	}
	if in.Facts.Evidence != nil {
		doc.EvidenceKnown = true
		doc.EvidenceSource = in.Facts.EvidenceSource
		for _, ac := range in.Spec.AcceptanceCriteria {
			ev := in.Facts.Evidence[ac.ID]
			doc.Evidence = append(doc.Evidence, EvidenceRow{
				ID:      ac.ID,
				Status:  ev.Status,
				Summary: ev.Summary,
				Stories: append([]string(nil), ev.Stories...),
				Kinds:   append([]KindEvidence(nil), ev.Kinds...),
			})
		}
	}
	return doc, nil
}

// identityRows builds the identity table. Labels are fixed English; the
// class and status values go through the display chain.
func identityRows(in Input) []KV {
	rows := []KV{
		{Label: "Ref", Value: in.Stamp.Ref},
		{Label: "Class", Value: in.Model.DisplayClass(string(in.Spec.Class))},
	}
	if in.Status != "" {
		rows = append(rows, KV{Label: "Status", Value: in.Model.DisplayState(string(in.Spec.Class), in.Status)})
	} else {
		rows = append(rows, KV{Label: "Status", Value: "not resolved for this render"})
	}
	rows = append(rows, KV{Label: "Commit", Value: in.Stamp.Commit})
	for _, ref := range artifact.WholeSpecSupersedesRefs(in.Spec.Links) {
		rows = append(rows, KV{Label: "Supersedes", Value: ref.String()})
	}
	if in.Spec.Supersession != nil {
		s := in.Spec.Supersession
		rows = append(rows, KV{Label: "Revision", Value: fmt.Sprintf("%d carried, %d amended, %d amended (advisory), %d removed, %d added", len(s.Carried), len(s.Amended), len(s.AmendedAdvisory), len(s.Removed), len(s.Added))})
	}
	return rows
}
