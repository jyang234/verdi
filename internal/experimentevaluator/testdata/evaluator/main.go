package main

import (
	"fmt"
	"io"
	"os"
)

const capabilities = "{\"evaluator_version\":\"fixture-evaluator/1\",\"metrics\":[],\"protocol_versions\":[\"verdi.experiment-evaluator/v1\",\"verdi.experiment-observation/v2\"],\"requires_elevated\":false,\"requires_network\":false,\"schema\":\"verdi.experiment-evaluator-capabilities/v2\"}\n"

const request = "{\"candidate\":\"candidate-a\",\"contract\":{\"digest\":\"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\",\"id\":\"contract-a\",\"path\":\"contracts/a.json\"},\"cycle\":{\"kind\":\"measured\",\"number\":1},\"experiment_digest\":\"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"fixtures\":[{\"digest\":\"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd\",\"id\":\"fixture-a\",\"path\":\"fixtures/a.json\"}],\"run\":\"run-1\",\"schema\":\"verdi.experiment-evaluator/v1\",\"workload\":{\"digest\":\"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\",\"id\":\"workload-a\",\"path\":\"workloads/a.json\"}}\n"

const response = "{\"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n"

func main() {
	if len(os.Args) != 3 || os.Args[1] != "--mode=ok" {
		fmt.Fprintln(os.Stderr, "unexpected argv")
		os.Exit(9)
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(8)
	}
	switch os.Args[2] {
	case "describe":
		if len(stdin) != 0 {
			fmt.Fprintln(os.Stderr, "describe stdin was not empty")
			os.Exit(7)
		}
		fmt.Print(capabilities)
	case "run":
		if string(stdin) != request {
			fmt.Fprintln(os.Stderr, "run stdin was not the exact canonical request")
			os.Exit(6)
		}
		fmt.Print(response)
	default:
		fmt.Fprintln(os.Stderr, "unknown operation")
		os.Exit(5)
	}
}
