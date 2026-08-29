package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/sealedexec"
	"github.com/jyang234/verdi/internal/sealedreview"
)

const contextReceiptUsage = "context receipt: usage: verdi context receipt verify --request <path|-> [--out <path>]"

func cmdContextReceipt(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		fmt.Fprintln(stderr, contextReceiptUsage)
		return 2
	}
	return cmdContextReceiptVerify(args[1:], stdin, stdout, stderr)
}

func cmdContextReceiptVerify(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	requestPath, hasRequest, outPath, hasOut, rest, err := extractContextCompileFlags(args)
	if err != nil {
		printContextReceiptDiagnostic(stderr, err)
		return 2
	}
	if len(rest) != 0 {
		printContextReceiptDiagnostic(stderr, fmt.Errorf("unexpected positional argument(s): %s", strings.Join(rest, " ")))
		return 2
	}
	if !hasRequest || requestPath == "" {
		printContextReceiptDiagnostic(stderr, errors.New("--request is required"))
		return 2
	}
	if hasOut && outPath == "" {
		printContextReceiptDiagnostic(stderr, errors.New("--out requires a value"))
		return 2
	}
	if hasOut && requestPath != "-" && sameFileArg(requestPath, outPath) {
		printContextReceiptDiagnostic(stderr, errors.New("--request and --out must not name the same path"))
		return 2
	}

	var raw []byte
	if requestPath == "-" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(requestPath)
	}
	if err != nil {
		printContextReceiptDiagnostic(stderr, errors.New("reading request failed"))
		return 2
	}
	request, err := contextreceipt.DecodeVerifyRequest(bytes.NewReader(raw))
	if err != nil {
		printContextReceiptDiagnostic(stderr, err)
		return 2
	}

	var resolver contextreceipt.AuthorityResolver
	var controllerConn io.Closer
	present, err := inheritedControllerFD3()
	if err != nil {
		printContextReceiptDiagnostic(stderr, errors.New("controller FD 3 is unavailable"))
		return 2
	}
	if present {
		controller, conn, openErr := openSealedController()
		if openErr != nil {
			printContextReceiptDiagnostic(stderr, openErr)
			return 2
		}
		resolver, controllerConn = controller, conn
	}
	if controllerConn != nil {
		defer controllerConn.Close()
	}

	verifier := contextreceipt.NewVerifierWithExecutionProof(resolver, sealedExecutionProofDecoder{})
	verdict, err := verifier.Verify(context.Background(), request)
	if err != nil {
		printContextReceiptDiagnostic(stderr, err)
		return 2
	}
	encoded, err := contextreceipt.EncodeVerdict(verdict)
	if err != nil {
		printContextReceiptDiagnostic(stderr, err)
		return 2
	}
	if hasOut {
		if err := atomicfile.Write(outPath, encoded, 0o644); err != nil {
			printContextReceiptDiagnostic(stderr, errors.New("writing verdict failed"))
			return 2
		}
	} else if _, err := stdout.Write(encoded); err != nil {
		printContextReceiptDiagnostic(stderr, errors.New("writing verdict to stdout failed"))
		return 2
	}
	if verdict.State == contextreceipt.StateProven {
		return 0
	}
	return 1
}

func inheritedControllerFD3() (bool, error) {
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, 3, syscall.F_GETFD, 0)
	if errno == syscall.EBADF {
		return false, nil
	}
	if errno != 0 {
		return false, errno
	}
	// Runtime-private descriptors are opened close-on-exec. An inherited
	// capability intentionally installed as child FD 3 has this bit cleared.
	return flags&syscall.FD_CLOEXEC == 0, nil
}

type sealedExecutionProofDecoder struct{}

func (sealedExecutionProofDecoder) DecodeExecutionProof(raw []byte) (contextreceipt.ExecutionProjection, error) {
	request, err := sealedexec.DecodeExecutionRequest(bytes.NewReader(raw))
	if err != nil {
		return contextreceipt.ExecutionProjection{}, err
	}
	canonical, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		return contextreceipt.ExecutionProjection{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return contextreceipt.ExecutionProjection{}, errors.New("execution request is not byte-canonical")
	}
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return contextreceipt.ExecutionProjection{}, err
	}
	return contextreceipt.ExecutionProjection{
		Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch, Session: request.Session,
		ATCRunway: request.ATCRunway, ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
		InputCommit: request.InputCommit, InputTree: request.InputTree, ExecutionWorkspaceRequestDigest: workspaceDigest,
		Adapter: request.Adapter, AdapterVersion: request.AdapterVersion,
		ProfileRef: contextreceipt.ProfileRef{Schema: request.Profile.Schema, ID: request.Profile.ID, Digest: request.Profile.Digest},
	}, nil
}

func (sealedExecutionProofDecoder) VerifyExpansionProof(raw []byte, expansion contextreceipt.Expansion) (contextreceipt.ExpansionProofProjection, error) {
	return sealedexec.VerifyExpansionDataProof(raw, expansion)
}

func (sealedExecutionProofDecoder) VerifyReviewProof(raw []byte, receipt contextreceipt.Receipt, candidate contextreceipt.Candidate, launch contextreceipt.ReviewLaunchProof) (contextreceipt.ReviewProofProjection, error) {
	return sealedreview.VerifyReviewProof(raw, receipt, candidate, launch)
}

func printContextReceiptDiagnostic(stderr io.Writer, err error) {
	fmt.Fprintf(stderr, "context receipt: %s\n", err)
}
