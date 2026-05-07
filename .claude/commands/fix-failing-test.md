# Fix a failing test

Use the `debugger` agent for this workflow.

When a test is failing:

1. reproduce the failure
2. explicitly decide whether the test or implementation is wrong
3. apply the smallest fix
4. rerun the failing test first
5. expand validation only after the target test passes

Finish with a short report:

- root cause
- files changed
- target test rerun result
- whether broader validation is needed
