package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
)

type designMutator interface {
	Mutate(context.Context, string, draftmutation.Request, draftmutation.Actor) (draftmutation.Response, *draftmutation.Error)
}

type designMutateFlags struct {
	request string
	harness string
	session string
}

func cmdDesignMutate(args []string, stdout, stderr io.Writer) int {
	return runDesignMutate(context.Background(), ".", args, os.Stdin, stdout, stderr, draftmutation.NewService())
}

// runDesignMutate is a deliberately thin adapter: it parses transport-only
// flags, strict-decodes one request, mints the fixed delegated-agent actor,
// calls the kernel, and maps its closed result/error sets to 0/1/2.
func runDesignMutate(ctx context.Context, start string, args []string, stdin io.Reader, stdout, stderr io.Writer, service designMutator) int {
	flags, err := parseDesignMutateFlags(args)
	if err != nil {
		return renderDesignMutateInputError(stderr, err)
	}
	actor, err := draftmutation.NewDelegatedAgent(flags.harness, flags.session)
	if err != nil {
		return renderDesignMutateInputError(stderr, err)
	}
	raw, err := readDesignMutationRequest(flags.request, stdin)
	if err != nil {
		return renderDesignMutateInputError(stderr, err)
	}
	request, err := draftmutation.DecodeRequest(raw)
	if err != nil {
		return renderDesignMutateInputError(stderr, err)
	}
	if service == nil {
		fmt.Fprintln(stderr, "io-failure: draft mutation service is unavailable")
		return 2
	}

	response, diagnostic := service.Mutate(ctx, start, request, actor)
	if diagnostic != nil {
		if diagnostic.Code == draftmutation.CodeStaleBase && response.Stale != nil {
			encoded, err := draftmutation.EncodeStaleRefusal(*response.Stale)
			if err != nil {
				return renderDesignMutateDiagnostic(stderr, request, draftmutation.WrapError(draftmutation.CodeResultInvalid, diagnostic.Identity, "encoding stale refusal", err))
			}
			if _, err := stdout.Write(encoded); err != nil {
				return renderDesignMutateDiagnostic(stderr, request, draftmutation.WrapError(draftmutation.CodeIOFailure, diagnostic.Identity, "writing stale refusal", err))
			}
			return 1
		}
		return renderDesignMutateDiagnostic(stderr, request, diagnostic)
	}
	if response.Result == nil || response.Stale != nil {
		fmt.Fprintln(stderr, "result-invalid: draft mutation service returned an invalid response union")
		return 2
	}
	encoded, err := draftmutation.EncodeResult(*response.Result)
	if err != nil {
		return renderDesignMutateDiagnostic(stderr, request, draftmutation.WrapError(draftmutation.CodeResultInvalid, response.Result.Identity, "encoding mutation result", err))
	}
	if _, err := stdout.Write(encoded); err != nil {
		return renderDesignMutateDiagnostic(stderr, request, draftmutation.WrapError(draftmutation.CodeIOFailure, response.Result.Identity, "writing mutation result", err))
	}
	return 0
}

func parseDesignMutateFlags(args []string) (designMutateFlags, error) {
	var flags designMutateFlags
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		switch name {
		case "--request", "--harness", "--session":
		default:
			return flags, fmt.Errorf("usage: verdi design mutate --request <path|-> --harness <id> [--session <id>]: unknown argument %q", args[i])
		}
		if seen[name] {
			return flags, fmt.Errorf("%s given more than once", name)
		}
		seen[name] = true
		if !hasValue {
			if i+1 >= len(args) {
				return flags, fmt.Errorf("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		if strings.TrimSpace(value) == "" {
			return flags, fmt.Errorf("%s requires a nonblank value", name)
		}
		switch name {
		case "--request":
			flags.request = value
		case "--harness":
			flags.harness = value
		case "--session":
			flags.session = value
		}
	}
	if flags.request == "" {
		return flags, fmt.Errorf("--request <path|-> is required")
	}
	if flags.harness == "" {
		return flags, fmt.Errorf("--harness <id> is required")
	}
	return flags, nil
}

func readDesignMutationRequest(path string, stdin io.Reader) (_ []byte, resultErr error) {
	var reader io.Reader
	var file *os.File
	if path == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("reading request from stdin: stdin is unavailable")
		}
		reader = stdin
	} else {
		var err error
		file, err = os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening request %q: %w", path, err)
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil && resultErr == nil {
				resultErr = fmt.Errorf("closing request %q: %w", path, closeErr)
			}
		}()
		reader = file
	}
	raw, err := io.ReadAll(io.LimitReader(reader, draftmutation.MaxRequestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading request: %w", err)
	}
	if len(raw) > draftmutation.MaxRequestBytes {
		return nil, fmt.Errorf("request exceeds 1 MiB")
	}
	return raw, nil
}

func renderDesignMutateInputError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: %v\n", draftmutation.CodeInputInvalid, err)
	return 2
}

func renderDesignMutateDiagnostic(stderr io.Writer, request draftmutation.Request, diagnostic *draftmutation.Error) int {
	detail := diagnostic.Detail
	if diagnostic.Cause != nil {
		if detail != "" {
			detail += ": "
		}
		detail += diagnostic.Cause.Error()
	}
	if diagnostic.IdentityAvailable() {
		fmt.Fprintf(stderr, "%s: identity=%s: %s\n", diagnostic.Code, canonicalInline(diagnostic.Identity), detail)
	} else {
		fmt.Fprintf(stderr, "%s: resolved_identity=unavailable expected=%s: %s\n", diagnostic.Code, canonicalInline(request.Expected), detail)
	}
	if diagnostic.Verdict() {
		return 1
	}
	return 2
}

func canonicalInline(value any) string {
	encoded, err := canonjson.Marshal(value)
	if err != nil {
		return `{"encoding":"unavailable"}`
	}
	return string(bytes.TrimSuffix(encoded, []byte("\n")))
}
