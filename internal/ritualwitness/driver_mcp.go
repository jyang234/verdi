package ritualwitness

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

// MCP is the MCP-server Driver (spec/ritual-effect-witness dc-1, ledger
// SI-341 (5)): Serve runs a server in process for the fixture directory,
// over one end of an in-memory pipe, and Run sends it one JSON-RPC
// tools/call request over the other end.
//
// A Serve of mcpserve.ServeConn over mcpserve.NewServer(root) runs the
// server core `verdi mcp` serves, not all of `verdi mcp` (R3ab review
// R3-B6). Standalone, `verdi mcp` also holds .verdi/data/writer.lock while
// it serves, and wires the readiness loader (get_document's readiness),
// the recovery loader (get_recovery), and a best-effort forge (with the
// review-unavailable disclosure when a configured forge is unreachable)
// into the server's Backend; with a workbench serving, it pipes to that
// server's socket instead. A case whose tool reads one of these wires the
// same into the server its Serve builds, or discloses the difference.
//
// The server runs in the test process, so the code under test reads the
// test process's own environment. Binary's CI field never reaches it: the
// CI-context variables, CI_DEFAULT_BRANCH among them (specstate's default
// branch, which the experiment and constitution tools read, among others),
// are whatever the test process holds. A case driving MCP pins the same
// set itself with PinCIEnv before Run (ledger SI-344 (2); R3ab review
// R3-B4).
//
// The MCP server has no exit codes of its own. A tool result whose isError
// is false is exit 0. A result whose isError is true, or a JSON-RPC error,
// is 2, the operational class, with the server's words in the error. A
// transport failure is -1, no verb's exit, so RunOn refuses the run instead
// of judging it: the server ending or failing before its connection closes
// cleanly, the caller's context ending first, or a response that is
// neither a tool result nor a JSON-RPC error (the response is decoded
// strictly). The server's tools root their own git calls, so no observer
// reaches them: Run reports the log unavailable, as Binary and Workbench
// do, until spec/gitx-recorder-seam threads one.
type MCP struct {
	// Serve answers the newline-delimited JSON-RPC requests it reads from
	// r with responses written to w, for the store rooted at root, until r
	// ends.
	Serve func(ctx context.Context, root string, r io.Reader, w io.Writer) error
	// Tool and Arguments make up the one tools/call request. Arguments is
	// sent as given, and omitted when nil.
	Tool      string
	Arguments json.RawMessage
}

// mcpCallID is the id of the one request Run sends.
const mcpCallID = "1"

// Run implements Driver. The exchange is bounded by ctx: when ctx ends
// first, both ends of the pipe are closed, so a blocked server read or
// write returns, and Run returns -1 wrapping ctx's error.
func (d MCP) Run(ctx context.Context, dir string) (int, CommandLog, error) {
	if d.Serve == nil {
		return -1, CommandLog{}, errors.New("ritualwitness: MCP: no server")
	}
	if d.Tool == "" {
		return -1, CommandLog{}, errors.New("ritualwitness: MCP: no tool")
	}
	request, err := mcpToolCall(d.Tool, d.Arguments)
	if err != nil {
		return -1, CommandLog{}, err
	}

	client, server := net.Pipe()
	served := make(chan error, 1)
	go func() {
		err := d.Serve(ctx, dir, server, server)
		// The client's read then ends, rather than waiting on a server
		// that will never answer.
		_ = server.Close()
		served <- err
	}()
	type answer struct {
		line []byte
		err  error
	}
	answered := make(chan answer, 1)
	go func() {
		line, err := mcpExchange(client, request)
		answered <- answer{line, err}
	}()

	var a answer
	select {
	case a = <-answered:
	case <-ctx.Done():
		_ = client.Close()
		_ = server.Close()
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: MCP: %s: %w", d.Tool, ctx.Err())
	}
	// The server reads the end of its input and returns.
	_ = client.Close()
	var serveErr error
	select {
	case serveErr = <-served:
	case <-ctx.Done():
		_ = server.Close()
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: MCP: %s: the server did not return: %w", d.Tool, ctx.Err())
	}
	if a.err != nil {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: MCP: %s: transport: %w", d.Tool, errors.Join(a.err, serveErr))
	}
	if serveErr != nil && !errors.Is(serveErr, io.EOF) && !errors.Is(serveErr, io.ErrClosedPipe) {
		return -1, CommandLog{}, fmt.Errorf("ritualwitness: MCP: %s: transport: the server failed: %w", d.Tool, serveErr)
	}
	exit, err := mcpExit(d.Tool, a.line)
	return exit, CommandLog{}, err
}

// mcpToolCall is the one tools/call request line Run sends, without its
// newline.
func mcpToolCall(tool string, args json.RawMessage) ([]byte, error) {
	type params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	line, err := json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  params          `json:"params"`
	}{"2.0", json.RawMessage(mcpCallID), "tools/call", params{tool, args}})
	if err != nil {
		return nil, fmt.Errorf("ritualwitness: MCP: %s: encoding the request's arguments: %w", tool, err)
	}
	return line, nil
}

// mcpExchange writes request as one line to conn and reads the first line
// conn answers.
func mcpExchange(conn net.Conn, request []byte) ([]byte, error) {
	if _, err := conn.Write(append(request, '\n')); err != nil {
		return nil, fmt.Errorf("sending the request: %w", err)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("reading the response: %w", err)
	}
	return line, nil
}

// mcpResponse is a JSON-RPC 2.0 response: exactly one of Result and Error.
type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *mcpRPCError    `json:"error"`
}

type mcpRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// mcpToolResult is an MCP tools/call result of protocol 2024-11-05, the
// version the repository's server speaks: text content, and isError.
type mcpToolResult struct {
	Content *[]mcpTextContent `json:"content"`
	IsError bool              `json:"isError"`
}

type mcpTextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// mcpExit maps the response line to the tool call's exit class.
func mcpExit(tool string, line []byte) (int, error) {
	malformed := func(why string, args ...any) (int, error) {
		return -1, fmt.Errorf("ritualwitness: MCP: %s: malformed response (%s): %s", tool, fmt.Sprintf(why, args...), bytes.TrimSpace(line))
	}
	var resp mcpResponse
	if err := decodeStrict(line, &resp); err != nil {
		return malformed("%v", err)
	}
	if resp.JSONRPC != "2.0" {
		return malformed("jsonrpc is %q, not 2.0", resp.JSONRPC)
	}
	hasResult := resp.Result != nil && string(resp.Result) != "null"
	switch {
	case hasResult == (resp.Error != nil):
		return malformed("it must carry exactly one of a result and an error")
	case resp.Error != nil:
		if id := string(resp.ID); id != mcpCallID && id != "null" {
			return malformed("id %s answers no request sent", id)
		}
		return 2, fmt.Errorf("ritualwitness: MCP: %s answered JSON-RPC error %d: %s", tool, resp.Error.Code, resp.Error.Message)
	}
	if id := string(resp.ID); id != mcpCallID {
		return malformed("id %q answers no request sent", id)
	}
	var result mcpToolResult
	if err := decodeStrict(resp.Result, &result); err != nil {
		return malformed("result: %v", err)
	}
	if result.Content == nil {
		return malformed("the result has no content")
	}
	var texts []string
	for _, c := range *result.Content {
		if c.Type != "text" {
			return malformed("content of type %q, not text", c.Type)
		}
		texts = append(texts, c.Text)
	}
	if result.IsError {
		return 2, fmt.Errorf("ritualwitness: MCP: %s answered isError: %s", tool, strings.Join(texts, "\n"))
	}
	return 0, nil
}

// decodeStrict decodes one JSON value into v, refusing unknown fields and
// any data after the value.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after the JSON value")
	}
	return nil
}
