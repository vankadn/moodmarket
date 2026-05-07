# Claude Code Instructions — InvestIQ

> For project phases, tech stack, and product decisions — read `InvestIQ_Project_Context.md`

---

## Onion Architecture — strictly enforced

Dependency direction: outer layers depend on inner. Never the reverse.

```
domain/          ← innermost: entities, value objects, interfaces (domain/ports/)
application/     ← use cases, orchestration; imports domain only
infrastructure/  ← implements domain/ports/; imports domain only
api/             ← HTTP handlers; imports application + domain only
```

Any import that violates this direction is a hard stop — redesign before proceeding.

## Interface-first

Every external dependency (AI, database, auth, market data, brokerage, banking) must be hidden behind an interface in `domain/ports/`. Application layer talks to interfaces only. Swapping any provider requires one new file + one env var change. Nothing else.

## DEV_MODE pattern

`DEV_MODE=true` bypasses auth. That branch lives in exactly one place: `infrastructure/auth/factory.go`. No DEV_MODE checks anywhere else in the codebase.

## Go rules

- Standard library only — no Gin, Echo, GORM, or any third-party framework
- `fmt.Errorf("context: %w", err)` everywhere — no silent failures, no bare `return err`
- Names read like English: `GetDailyRecommendation`, not `getData`. Abbreviations: `ctx err db req res` only
- One primary responsibility per file — no god files
- userId comes from context only — always `identityProvider.GetCurrentUser(ctx)`, never a global or hardcoded value
- Business logic lives in the domain layer — never in handlers or infrastructure

## React rules

- TypeScript + Vite only — no UI component libraries (MUI, Chakra, Shadcn, etc.)
- No Redux — component and hook state only

## When writing code

1. State the layer you are working in
2. Name the interface before writing the implementation
3. Show `// path/to/file.go` at the top of every code block
4. One sentence explaining *why* per significant decision
5. Flag architecture violations instead of complying — propose the correct approach
6. End with the exact commands to verify the change works
