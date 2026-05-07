# Fix a failing test

Use the `debugger` agent for this workflow.

When a test is failing:

1. reproduce the failure
2. explicitly decide whether the test, implementation, or golden output is wrong
3. use `go test -race` early if the failure is flaky, timing-sensitive, or concurrency-related
4. apply the smallest fix
5. rerun the failing test first
6. expand validation only after the target test passes

Finish with a short report:

- root cause
- files changed
- target test rerun result
- whether broader validation is needed
