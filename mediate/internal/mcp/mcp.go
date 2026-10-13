package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)

// Handler returns the MCP root generated from reg, served over the streamable HTTP transport with no
// session kept between requests (ADR-0086). version is the mediator's version, which the server
// reports to a client when it initializes. The transport's and the server's own logs and each failed
// call are logged through logger, a failed call at the level its origin sets. The server's info
// records are lowered to debug (ADR-0122).
func Handler(reg service.Registry, version string, logger *slog.Logger) http.Handler {
	// The capabilities are declared explicitly. With none given, the SDK advertises logging and a
	// changing tool list, and an agent then holds a subscription stream open against the server.
	server := sdk.NewServer(
		&sdk.Implementation{Name: "mediated-mailbox-mediate", Version: version},
		&sdk.ServerOptions{
			Capabilities: &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{}},
			Logger:       slog.New(routineAtDebug{logger.Handler()}),
		},
	)
	for _, op := range reg.Operations() {
		server.AddTool(&sdk.Tool{
			Name:         op.Name,
			Description:  op.Description,
			InputSchema:  op.Input,
			OutputSchema: op.Output,
			Annotations:  annotations(op.Annotations),
		}, call(reg, op.Name, logger))
	}
	server.AddReceivingMiddleware(refuseBeyondTools)
	// A client's cancellation reaches the tool handler, so an abandoned call stops waiting for its
	// lease and spends nothing on a provider. The SDK follows it from 2026-07-28 on, and the lease
	// wait's own deadline bounds a call on an earlier revision (ADR-0130).
	return refuseBatches(sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{
		Stateless:                    true,
		JSONResponse:                 true,
		Logger:                       logger,
		PropagateRequestCancellation: true,
	}))
}

// refuseBatches answers 400 to a POST whose body is a JSON-RPC batch, a body whose first byte after
// JSON's whitespace is an opening bracket, before the SDK reads it, at every protocol revision. The
// SDK runs a batch's calls at once and holds every answer until the last completes, so one request
// could make the mediator hold thousands of calls, and revision 2025-06-18 removed batches
// (ADR-0130). It decides on the body's first byte after the whitespace alone, and on no method, which
// the SDK alone parses (refuseBeyondTools). The whitespace it reads is bounded by the SDK's own body
// limit and dropped, which JSON ignores, and the body from that byte on reaches the SDK as it was
// sent.
func refuseBatches(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}
		body := bufio.NewReader(http.MaxBytesReader(w, r.Body, sdk.DefaultMaxRequestBodyBytes))
		for {
			c, err := body.ReadByte()
			if errors.Is(err, io.EOF) {
				break
			}
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				http.Error(w, fmt.Sprintf("request body exceeds %d bytes", tooLarge.Limit), http.StatusRequestEntityTooLarge)
				return
			}
			if err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
				continue
			}
			if c == '[' {
				http.Error(w, "JSON-RPC batches are not accepted", http.StatusBadRequest)
				return
			}
			if err := body.UnreadByte(); err != nil {
				http.Error(w, "failed to read body", http.StatusBadRequest)
				return
			}
			break
		}
		r.Body = struct {
			io.Reader
			io.Closer
		}{body, r.Body}
		next.ServeHTTP(w, r)
	})
}

// annotations returns the tool annotations derived from the operation's effect, with all four hints
// set. A nil destructive or open-world hint is left out of the listing, and a client then reads the
// tool as destructive and open-world (ADR-0086).
func annotations(a service.Annotations) *sdk.ToolAnnotations {
	destructive, openWorld := a.Destructive, a.OpenWorld
	return &sdk.ToolAnnotations{
		ReadOnlyHint:    a.ReadOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  a.Idempotent,
		OpenWorldHint:   &openWorld,
	}
}

// call returns the tool handler that runs the operation named name on the call's arguments.
func call(reg service.Registry, name string, logger *slog.Logger) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		input := req.Params.Arguments
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		out, err := reg.Call(ctx, name, input)
		if err != nil {
			logger.Log(ctx, service.FailureLevel(err), "operation failed", "operation", name, "error", err)
			failure := service.Failure(err)
			return &sdk.CallToolResult{
				IsError:           true,
				StructuredContent: failure,
				Content:           []sdk.Content{&sdk.TextContent{Text: string(failure)}},
			}, nil
		}
		return &sdk.CallToolResult{
			StructuredContent: out,
			Content:           []sdk.Content{&sdk.TextContent{Text: string(out)}},
		}, nil
	}
}

// refused reports whether a method belongs to the protocol surface parity forbids, prompts,
// resources and a log level. The SDK answers these with empty or accepting replies whatever its
// options, so the root refuses them (ADR-0086).
func refused(method string) bool {
	return strings.HasPrefix(method, "prompts/") || strings.HasPrefix(method, "resources/") || method == "logging/setLevel"
}

// refuseBeyondTools answers a refused method as a method the server does not have. It runs inside
// the SDK, on the method the SDK parsed from the request, so no request reads as one method here and
// another at dispatch. The SDK carries the error in a 404 from
// 2026-07-28 on and in a 200 before it.
func refuseBeyondTools(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		if refused(method) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not found: " + method}
		}
		return next(ctx, method, req)
	}
}

// routineAtDebug hands the SDK server's records to the handed logger's handler, with every record
// below warn and at info or above lowered to debug. With no session kept, the server connects one
// per request and writes "server connecting" and "server session connected" at info for each, which
// is detail. Its warnings and errors keep their levels (ADR-0122).
type routineAtDebug struct{ slog.Handler }

func (h routineAtDebug) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, lowered(level))
}

func (h routineAtDebug) Handle(ctx context.Context, r slog.Record) error {
	r.Level = lowered(r.Level)
	return h.Handler.Handle(ctx, r)
}

func (h routineAtDebug) WithAttrs(attrs []slog.Attr) slog.Handler {
	return routineAtDebug{h.Handler.WithAttrs(attrs)}
}

func (h routineAtDebug) WithGroup(name string) slog.Handler {
	return routineAtDebug{h.Handler.WithGroup(name)}
}

// lowered returns debug for a level at info or above and below warn, and level otherwise.
func lowered(level slog.Level) slog.Level {
	if level >= slog.LevelInfo && level < slog.LevelWarn {
		return slog.LevelDebug
	}
	return level
}
