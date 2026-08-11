package draftmutation

import "github.com/jyang234/verdi/internal/designprovenance"

// Operation and Change are shared verbatim with the committed provenance
// contract so request, result, and sidecar cannot drift into similar-but-
// different wire unions.
type Operation = designprovenance.Operation
type OperationKind = designprovenance.OperationKind
type Change = designprovenance.Change
type ChangeKind = designprovenance.ChangeKind

const (
	OpSetProblem       = designprovenance.OpSetProblem
	OpSetOutcome       = designprovenance.OpSetOutcome
	OpAddAC            = designprovenance.OpAddAC
	OpEditAC           = designprovenance.OpEditAC
	OpRemoveAC         = designprovenance.OpRemoveAC
	OpReorderAC        = designprovenance.OpReorderAC
	OpSetACEvidence    = designprovenance.OpSetACEvidence
	OpAddConstraint    = designprovenance.OpAddConstraint
	OpEditConstraint   = designprovenance.OpEditConstraint
	OpRemoveConstraint = designprovenance.OpRemoveConstraint
	OpAddDecision      = designprovenance.OpAddDecision
	OpEditDecision     = designprovenance.OpEditDecision
	OpRemoveDecision   = designprovenance.OpRemoveDecision
	OpAddQuestion      = designprovenance.OpAddQuestion
	OpEditQuestion     = designprovenance.OpEditQuestion
	OpRemoveQuestion   = designprovenance.OpRemoveQuestion
	OpAddLink          = designprovenance.OpAddLink
	OpRemoveLink       = designprovenance.OpRemoveLink
	OpAddStub          = designprovenance.OpAddStub
	OpEditStub         = designprovenance.OpEditStub
	OpRemoveStub       = designprovenance.OpRemoveStub
	OpReorderStub      = designprovenance.OpReorderStub
	OpAddContextRef    = designprovenance.OpAddContextRef
	OpRemoveContextRef = designprovenance.OpRemoveContextRef

	ChangeAdded               = designprovenance.ChangeAdded
	ChangeReplaced            = designprovenance.ChangeReplaced
	ChangeRemoved             = designprovenance.ChangeRemoved
	ChangeReordered           = designprovenance.ChangeReordered
	ChangeRelationshipAdded   = designprovenance.ChangeRelationshipAdded
	ChangeRelationshipRemoved = designprovenance.ChangeRelationshipRemoved
)
