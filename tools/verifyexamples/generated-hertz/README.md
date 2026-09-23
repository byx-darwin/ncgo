# Generated Hertz dependency lock

This fixture is the reviewed dependency graph for the database-backed Hertz
profile generated with the code-generation versions pinned in CI. Integration
tests overlay its `go.mod`/`go.sum` onto fresh projects, prefetch the graph, and
then run `go mod tidy`, build, and tests with the module proxy disabled.

Refresh this lock only when intentionally reviewing a generator, template, Go
version, or generated runtime dependency update.
