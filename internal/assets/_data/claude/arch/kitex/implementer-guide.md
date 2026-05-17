# Kitex Implementation Guide

## Adding a New RPC Method

1. Define the method in `idl/<service>.proto` with request/response messages
2. Run `kitex -template-dir template/kitex-template -type protobuf idl/<service>.proto`
3. Implement the handler in `internal/handler/`: receive request, call usecase, return response
4. Use `rpcerror.ToBizError(err)` for all error mappings
5. The handler is registered automatically by the kitex generator

## Adding an Interceptor

1. Create a new function in `internal/pkg/interceptor/` matching the interceptor signature
2. Wire it as a kitex server option in `internal/base/server/server.go`
3. Interceptors run in registration order; Recovery should be outermost

## Context Propagation

```go
func (h *MyHandler) Handle(ctx context.Context, req *pb.MyRequest) (*pb.MyResponse, error) {
    result, err := h.usecase.Do(ctx, req)  // propagate context through usecase
    if err != nil {
        return nil, rpcerror.ToBizError(err)
    }
    return result, nil
}
```

## Client Consumption

- Generated clients in `pkg/client/<service>/` should be wrapped by an adapter interface
- Adapters live in `internal/adapter/` and implement the usecase-facing interface
- This keeps usecase code decoupled from the specific RPC client implementation

## Common Patterns

- Streaming: use kitex streaming support for large payloads; configure max message size in server options
- Timeout: set via `client.WithRPCTimeout()` at client init, not per-call
- Caller validation: use the CallerAllowlist interceptor to restrict which services can call your endpoints
