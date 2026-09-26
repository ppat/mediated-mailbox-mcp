package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)

// Handler returns the MCP root generated from reg, served over the streamable HTTP transport with no
// session kept between requests (ADR-0086). version is the mediator's version, which the server
// reports to a client when it initializes.
func Handler(reg service.Registry, version string) http.Handler {
	// The capabilities are declared explicitly. With none given, the SDK advertises logging and a
	// changing tool list, and an agent then holds a subscription stream open against the server.
	server := sdk.NewServer(
		&sdk.Implementation{Name: "mediated-mailbox-mediate", Version: version},
		&sdk.ServerOptions{Capabilities: &sdk.ServerCapabilities{Tools: &sdk.ToolCapabilities{}}},
	)
	for _, op := range reg.Operations() {
		server.AddTool(&sdk.Tool{
			Name:         op.Name,
			Description:  op.Description,
			InputSchema:  op.Input,
			OutputSchema: op.Output,
			Annotations:  annotations(op.Annotations),
		}, call(reg, op.Name))
	}
	server.AddReceivingMiddleware(refuseBeyondTools)
	return sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return server }, &sdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       slog.Default(),
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
func call(reg service.Registry, name string) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		input := req.Params.Arguments
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		out, err := reg.Call(ctx, name, input)
		if err != nil {
			slog.ErrorContext(ctx, "operation failed", "operation", name, "error", err)
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
// another at dispatch, a batch's elements included. The SDK carries the error in a 404 from
// 2026-07-28 on and in a 200 before it.
func refuseBeyondTools(next sdk.MethodHandler) sdk.MethodHandler {
	return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
		if refused(method) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not found: " + method}
		}
		return next(ctx, method, req)
	}
}
