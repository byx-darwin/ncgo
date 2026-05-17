# Kitex Reviewer Checklist

## Transport Layer

- [ ] handlers are thin and delegate to usecase layer
- [ ] no interceptor logic embedded in handlers
- [ ] interceptors are wired via kitex server options in `internal/base/server/server.go`
- [ ] Recovery interceptor is outermost in the chain

## Error Handling

- [ ] all handler errors pass through `rpcerror.ToBizError(err)`
- [ ] business error codes are consistent with proto/service contracts
- [ ] no plain `return errors.New(...)` from handlers

## Context Propagation

- [ ] `context.Context` is propagated through the full call chain
- [ ] no `context.Background()` created mid-request
- [ ] timeouts are set at the call site, not inside the handler

## Client Usage

- [ ] generated clients are consumed via adapters, not directly from handlers
- [ ] client instances are shared (not recreated per call)
- [ ] retry/circuit-breaker config is in the client package

## IDL & Generated Code

- [ ] proto changes are the source of truth for service contracts
- [ ] no hand-edits in generated code under `kitex_gen/`
- [ ] `kitex` has been re-run after proto changes
