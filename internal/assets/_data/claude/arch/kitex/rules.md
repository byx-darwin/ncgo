# Kitex RPC Service Rules

This service uses Kitex as the RPC transport layer.

## Interceptors vs Handler

- interceptors (`internal/pkg/interceptor/`) handle cross-cutting concerns: RequestID, AccessLog, Recovery, RequestTimeout, CallerAllowlist
- handlers (`internal/handler/`) delegate to usecase; do NOT embed interceptor logic in handlers
- interceptors are wired in `internal/base/server/server.go` via kitex server options

## RPC Error Handling

- all errors crossing the RPC boundary go through `rpcerror.ToBizError(err)`
- callers receive a `kitex.BizStatusError` carrying a 5-digit business code
- do NOT return plain Go errors from handlers; always use the rpcerror mapping

## Request Context

- Kitex uses `context.Context` carried through the RPC invocation chain
- propagate the caller's context; do not create `context.Background()` in the middle of a request path
- timeouts and deadlines should be set at the call site, not inside the handler

## Client-Side Usage

- generated clients live in `pkg/client/<service>/`
- consumed by adapters, never directly from handlers or usecases
- retry and circuit-breaker config is in the client package, not per-call-site
- client initialization is done once at startup; share the client instance

## IDL-Driven Contracts

- service definitions in `idl/<service>.proto` drive all generated code
- change the proto first, then run `kitex` to regenerate
- do not hand-edit generated code under `kitex_gen/`
