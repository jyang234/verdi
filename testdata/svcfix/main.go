// Command svcfix is the fixture service's composition root (PLAN.md §4):
// wires app.Service to handler.Server and serves the two entrypoints
// .flowmap/boundary-contract.json declares.
package main

import (
	"net/http"

	"example.com/svcfix/internal/app"
	"example.com/svcfix/internal/audit"
	"example.com/svcfix/internal/bus"
	"example.com/svcfix/internal/handler"
)

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}

func run() error {
	svc := app.New(audit.New(), bus.New())
	srv := handler.New(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /refunds/{id}", srv.GetRefund)
	mux.HandleFunc("POST /refunds/{id}/publish", srv.PublishRefund)

	return http.ListenAndServe(":8080", mux)
}
