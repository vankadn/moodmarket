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

## Git workflow

Before any commit or push, read `skills/pre-commit-checklist.md` and verify every item.

1. Write and build the code
2. You verify it works
3. You say "commit" or "push" — then do it

## Project context

`InvestIQ_Project_Context.md` is the source of truth for phases, decisions, and known debt. Keep it current — update it as part of the same session that completes a feature, not after the fact.

Update when:
- A phase or sub-phase is marked complete
- A new architectural or product decision is made
- Known debt is resolved or added
- A domain model, collection, or interface changes

## Skills — load when relevant

- Adding or reviewing logs → read `skills/logging-rules.md`
- Touching frontend → read `skills/react-rules.md`
- Starting a new feature implementation → read `skills/new-feature-checklist.md`
- Committing or pushing → read `skills/pre-commit-checklist.md`
