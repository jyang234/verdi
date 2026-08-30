package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
	"github.com/jyang234/verdi/internal/experimenthuman"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

const experimentHumanTrustSource = "offline-human"

const experimentHumanPrompt = "openssl pkeyutl -sign -rawin -inkey <private-key.pem> -in <challenge-file> -out <signature-file>"

type experimentHumanAuthorizationResult struct {
	Outcome   experimentapp.Outcome     `json:"outcome"`
	Challenge experimenthuman.Challenge `json:"challenge"`
	Prompt    string                    `json:"manual_signing_prompt"`
}

func runExperimentHuman(ctx context.Context, operation string, service *experimentapp.Service, identity experimentapp.Identity, flags experimentFlags, stdout, stderr io.Writer) int {
	review := service.ReviewRegistration(ctx, identity)
	if review.Outcome.Classification == experimentapp.ClassificationOperational || review.Packet.AcceptedHead == "" || review.ProposedArtifactDigest == "" {
		return renderExperimentResult(operation, review.Outcome, review, flags.json, stdout, stderr)
	}
	proposalHead, err := gitx.RevParse(ctx, identity.CheckoutRoot, "HEAD")
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("resolving proposal HEAD: %w", err), stderr)
	}
	inputDigest := review.PacketDigest
	humanOperation := experimenthuman.OperationProposeRegistration
	switch operation {
	case "reconcile-draft":
		humanOperation = experimenthuman.OperationReconcileDraft
		inputDigest, err = canonjson.Digest(struct{}{})
		if err != nil {
			return experimentOperational(operation, fmt.Errorf("digesting reconciliation input: %w", err), stderr)
		}
	case "propose-ratification":
		humanOperation = experimenthuman.OperationProposeRatification
		inputDigest, err = experimentapp.RatificationInputDigest(
			flags.values["result"],
			experiment.Disposition(flags.values["disposition"]),
			flags.values["candidate"],
			flags.values["reason"],
		)
		if err != nil {
			return experimentOperational(operation, err, stderr)
		}
	}
	facts := experimenthuman.ChallengeFacts{
		Operation: humanOperation, Spike: identity.Spike, ExperimentID: identity.ExperimentID,
		AcceptedHEAD: review.Packet.AcceptedHead, ProposalHEAD: proposalHead,
		TrustSource: experimentHumanTrustSource, InputDigest: inputDigest,
		ProposalDigest: review.ProposedArtifactDigest,
	}
	challenge, err := experimenthuman.NewChallenge(facts)
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("constructing human challenge: %w", err), stderr)
	}
	challengeBytes, err := challenge.Canonical()
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("encoding human challenge: %w", err), stderr)
	}
	proofPath := flags.values["human-proof"]
	if proofPath == "" {
		result := experimentHumanAuthorizationResult{
			Outcome:   experimentapp.Outcome{Classification: experimentapp.ClassificationVerdict, Code: "human-authorization-required", Detail: "offline human authorization is required"},
			Challenge: challenge, Prompt: experimentHumanPrompt,
		}
		if flags.json {
			return renderExperimentResult(operation, result.Outcome, result, true, stdout, stderr)
		}
		// The human rendering must carry the exact canonical challenge bytes
		// and the manual signing prompt as data (Wave 5 design §8) — the
		// generic outcome line alone would strand the signer.
		if _, err := fmt.Fprintf(stdout, "experiment %s: %s (%s)\n%s\n%s%s\n", operation, result.Outcome.Classification, result.Outcome.Code, result.Outcome.Detail, challengeBytes, experimentHumanPrompt); err != nil {
			return experimentOperational(operation, fmt.Errorf("writing result: %w", err), stderr)
		}
		return result.Outcome.ExitCode()
	}
	if proofPath == "-" {
		return experimentOperational(operation, fmt.Errorf("--human-proof requires a signature file path"), stderr)
	}
	proof, err := readExperimentInput(proofPath, nil, 64)
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("loading human proof: %w", err), stderr)
	}
	acceptedSource, err := experimentAcceptedTreeFS(ctx, identity.CheckoutRoot, facts.AcceptedHEAD)
	if err != nil {
		return experimentOperational(operation, fmt.Errorf("loading accepted authority tree: %w", err), stderr)
	}
	verification, err := experimenthuman.Verify(ctx, facts, challengeBytes, proof, experimenthuman.AcceptedAuthority{Head: facts.AcceptedHEAD, Source: acceptedSource})
	if err != nil {
		return experimentOperational(operation, err, stderr)
	}
	if verification.State != governanceprincipal.ResolutionAuthenticated {
		outcome := experimentapp.Outcome{Classification: experimentapp.ClassificationVerdict, Code: verification.Code, Detail: "offline human proof was not authenticated"}
		result := struct {
			Outcome   experimentapp.Outcome     `json:"outcome"`
			Challenge experimenthuman.Challenge `json:"challenge"`
		}{Outcome: outcome, Challenge: challenge}
		return renderExperimentResult(operation, outcome, result, flags.json, stdout, stderr)
	}
	actor, err := experimentapp.NewAuthenticatedHuman(verification.Resolution)
	if err != nil {
		return experimentOperational(operation, err, stderr)
	}
	identity.Actor = actor
	if operation == "reconcile-draft" {
		result := service.ReconcileDraft(ctx, identity)
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	}
	if operation == "propose-ratification" {
		result := service.ProposeRatification(ctx, identity, experimentapp.RatificationProposalInput{
			ResultDigest: flags.values["result"], Disposition: experiment.Disposition(flags.values["disposition"]),
			Candidate: flags.values["candidate"], Reason: flags.values["reason"],
			Resolution: verification.Resolution, Proof: verification.Retained,
		})
		return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
	}
	result := service.ProposeRegistration(ctx, identity, experimentapp.RegistrationInput{ReviewPacketDigest: review.PacketDigest})
	return renderExperimentResult(operation, result.Outcome, result, flags.json, stdout, stderr)
}
