# Hertz Implementation Guide

## Adding a New Endpoint

1. Define request/response messages in `idl/<service>.proto`
2. Add the RPC method with `(api.get)` / `(api.post)` / etc. annotations
3. Run `hz` to regenerate handler stubs and pb code
4. Implement handler logic: parse request, call usecase, return response via `response.OK(c, resp)`
5. The handler is registered automatically via `router.GeneratedRegister`

## Adding Middleware

1. Create a new function in `internal/pkg/middleware/` matching the middleware signature
2. Add it to the chain in `internal/base/server/server.go`
3. Middleware runs before/after the handler depending on where you call `c.Next()`

## Context Propagation

```go
func (h *MyHandler) Handle(c *app.RequestContext) {
    ctx := c.Request.Context()  // derive context from request
    result, err := h.usecase.Do(ctx, req)  // propagate through usecase
    if err != nil {
        response.Err(c, err)
        return
    }
    response.OK(c, result)
}
```

## Common Patterns

- Use `c.Query()`, `c.Param()`, `c.PostForm()` only for non-IDL routes; prefer proto-generated binding for IDL routes
- File uploads use `multipart/form-data` with `api.file_name` annotation on proto fields
- Pagination: standardize on `page`/`page_size` query params or cursor-based pagination
