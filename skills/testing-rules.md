# Testing Rules

## What to test

Pure functions and domain logic — any function with conditional business logic, input-dependent
output, or rules that could silently break.

Examples in this codebase:
- `buildUserMessage` — prompt sections appear/absent based on input; rules like 40% concentration
- Budget calculations — BaseBudget + ExtraMoney, allocation sums
- Any future allocation validation, rounding, or capping logic

## What not to test (yet)

- Repository implementations (MongoDB) — require a real database; integration tests, Phase 9+
- HTTP handlers — require a real server; integration tests, Phase 9+
- Provider infrastructure (Plaid, Alpaca, Polygon, Claude) — require real APIs or heavy mocking

## How to write tests here

Table-driven, parallel, in the same package as the function under test:

```go
func TestFunctionName(t *testing.T) {
    t.Parallel()
    cases := []struct {
        name           string
        // inputs...
        mustContain    []string
        mustNotContain []string
    }{
        // one row per rule or boundary
    }
    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            // assert...
        })
    }
}
```

## When tests already exist

Run them before and after your change. Keep them passing. If a change intentionally breaks
an existing assertion, update the test and note why in the same commit.

## Tech debt marker

When adding basic tests as a placeholder for deeper testing later, add a comment at the top
of the file:

```go
// TECH DEBT: these are basic smoke tests. Future work:
// - LLM-level assertion tests (does the model actually respect the rules?)
// - Fuzz / property tests on numeric invariants
// - Regression snapshots
```
