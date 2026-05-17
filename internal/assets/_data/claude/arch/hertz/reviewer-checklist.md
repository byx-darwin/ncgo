# Hertz Reviewer Checklist

## Transport Layer

- [ ] handlers are thin and delegate to usecase layer
- [ ] no middleware logic embedded in handlers
- [ ] `*app.RequestContext` is not stored in struct state or passed past handler boundary
- [ ] middleware chain order in `internal/base/server/server.go` is correct

## Response & Error Handling

- [ ] all handlers use `response.OK(c, resp)` / `response.Err(c, err)` for consistent output
- [ ] error codes follow 5-digit convention (`1xxxx` for request/auth/rate-limit)
- [ ] no raw `c.JSON()` or `c.String()` bypassing the response helper

## IDL & Route Contracts

- [ ] routes match the proto IDL definitions (paths, methods, query params)
- [ ] generated route registration matches `router.GeneratedRegister`
- [ ] no hand-edits in generated pb code

## Security & Validation

- [ ] request validation uses proto `api.vd` annotations or explicit validation
- [ ] auth middleware is applied to protected routes
- [ ] rate-limit middleware is configured for public-facing endpoints
