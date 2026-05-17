# Hertz Debugger Playbook

## Common Issues

### Handler Not Called / 404

- Check if route is registered in `router.GeneratedRegister`
- Verify `hz` was run after proto changes
- Check middleware chain — an early-returning middleware may block the handler

### Context Lost in Middleware

- `*app.RequestContext` is the carrier; derive context via `c.Request.Context()`
- do not create `context.Background()` in the middle of a request path
- if spawning a goroutine, explicitly pass the request context or a derived one with timeout

### Response Format Mismatch

- All handlers should use `response.OK()` / `response.Err()` — raw `c.JSON()` may bypass the standard envelope
- Check that error codes are registered via `response.MustRegister` at startup

### Middleware Chain Order

- The order in `internal/base/server/server.go` matters: earlier middleware wraps later ones
- Recovery middleware should be outermost (registered first)
- Auth middleware should run after CORS and before handler

## Debugging Steps

1. Enable Hertz debug logging via `server.WithDebug(true)`
2. Add `log.Printf` at middleware entry/exit to trace chain execution
3. Use `c.Request.URL` and `c.Request.Method` to verify the request matches the proto definition
4. Check generated handler stubs for correct binding of query/body/path parameters

## Reproduce First

Before fixing: write a failing test with `httptest` against the handler, then fix. Run with `go test -race`.
