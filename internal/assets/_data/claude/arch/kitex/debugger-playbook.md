# Kitex Debugger Playbook

## Common Issues

### RPC Call Fails / Timeout

- Check if the server is registered and listening on the expected address
- Verify the caller is in the CallerAllowlist (if configured)
- Check interceptor chain — a failing interceptor may prevent the handler from running
- Verify `kitex` has been re-run after proto changes

### Context Lost

- `context.Context` is carried through the RPC invocation chain
- do not create `context.Background()` mid-request
- if spawning a goroutine, pass the incoming context or a derived one with timeout

### Error Mapping Issues

- All errors must pass through `rpcerror.ToBizError(err)` before returning
- Plain Go errors returned from handlers become generic RPC errors, losing business code info
- Check that error codes are registered and consistent with the service contract

### Interceptor Chain Order

- The order in `internal/base/server/server.go` matters
- Recovery interceptor must be outermost (registered first)
- RequestTimeout should run after Recovery but before business interceptors

### Client-Side Issues

- Generated clients must be initialized and shared; per-call client creation is slow and error-prone
- Retry config is set at client init; check that retry policy is configured
- Circuit-breaker may be blocking calls; check breaker state and fallback logic

## Debugging Steps

1. Enable Kitex debug logging via `server.WithDebugMode(true)` or environment variable `KITEX_DEBUG=true`
2. Add `log.Printf` at interceptor entry/exit to trace chain execution
3. Use `kitex` CLI to verify service registration: `kitex -check`
4. Check generated handler stubs for correct proto binding

## Reproduce First

Before fixing: write a failing integration test against the handler using the generated client, then fix. Run with `go test -race`.
