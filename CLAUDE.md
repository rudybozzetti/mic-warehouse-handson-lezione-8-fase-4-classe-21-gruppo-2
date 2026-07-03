# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A hands-on lab for extracting the **Warehouse Bounded Context** out of **MIC**, a legacy PHP invoicing monolith, into a Go microservice. The migration follows the **Strangler Fig** pattern (ADR-001) with a **Dual-Write** transition (ADR-013). The active phase is **Phase 4** (`phase-04-db/`).

## Commands (run from `phase-04-db/`)

```bash
# Run the full test suite (no local Go required)
docker compose run --rm test

# Start the dual-stack (two MySQL containers + Go app + Adminer)
docker compose up --build

# Health check
curl -s http://localhost:8081/health

# Demo CLI: exercise dual-write without HTTP
docker compose --profile demo run --build --rm seed write demo-1 ABC-001 "Widget" 2999 EUR
docker compose --profile demo run --build --rm seed read demo-1
docker compose --profile demo run --build --rm seed compare demo-1

# Stop and remove volumes
docker compose down -v
```

If Go 1.22+ is installed locally:
```bash
cd phase-04-db
go test ./...
go test ./repositories/...   # single package
go test -run TestLegacyConversions ./repositories/...   # single test
```

**Port conflicts:** Phase 04 binds 8081, 8082, 3306, 3307. Stop prior phases before starting:
```bash
cd ../phase-01-monolith && docker compose down
cd ../phase-03-skeleton && docker compose down
```

## Architecture (`phase-04-db/`)

Clean Architecture layers, innermost to outermost:

```
entities/          — Article aggregate root, SKU/Money value objects, InventoryLevel
events/            — domain events (ArticleCreated, ArticlePriceChanged, …)
interfaces/        — ArticleRepository port (the only contract; never change this)
repositories/      — four adapters that implement the port
main.go            — composition root (wires databases, creates DualWrite, starts Echo)
cmd/seed/          — demo CLI for exercising dual-write without HTTP
```

### The three repository implementations

| File | Type | Purpose |
|---|---|---|
| `article_repository.go` | `MySQLArticleRepository` | BC schema (`warehouse_db`): `price_cents BIGINT + currency CHAR(3)` |
| `legacy_article_repository.go` | `LegacyMySQLArticleRepository` | ACL for legacy schema (`legacy_db`): `price DECIMAL(10,2)`, no currency column |
| `dual_write_article_repository.go` | `DualWriteArticleRepository` | Decorator: writes to both (legacy first), reads from one store per `ReadMode` |
| `in_memory_article_repository.go` | `InMemoryArticleRepository` | In-memory fake used only in tests |

### The two databases

| | `legacy_db` (port 3306) | `warehouse_db` (port 3307) |
|---|---|---|
| price | `price DECIMAL(10,2)` e.g. `29.99` | `price_cents BIGINT` e.g. `2999` |
| currency | none (always EUR) | `currency CHAR(3)` explicit |

The **ACL** (`LegacyMySQLArticleRepository`) is the only file that knows about this schema difference. `centsToDecimal` / `decimalToCents` are the conversion helpers; never use `float64` for money.

### Dual-write rules (ADR-013)

- **Writes**: always both stores; **legacy first**. If legacy fails → abort; if BC fails → return error (no rollback of legacy in this exercise).
- **Reads**: single store, chosen by `ReadMode`. `ReadFromLegacy` (default) → legacy DB; `ReadFromBC` (post-cutover) → warehouse DB. Controlled by env var `DUAL_WRITE_READ_MODE=legacy|bc`.

### Domain rules enforced by value-object factories

Always construct `SKU` via `entities.NewSKU(code)` and `Money` via `entities.NewMoney(cents, currency)`. Bypassing the factories produces invalid aggregates. Currency must be three uppercase letters (ISO 4217); `AmountCents` must be ≥ 0.

## What is in scope for Phase 4

Only `repositories/legacy_article_repository.go` and `repositories/dual_write_article_repository.go`. Do **not** edit `entities/`, `events/`, `interfaces/repository.go`, the BC adapter, or the in-memory fake. Use cases (CP5), HTTP handlers (CP6), and auth/events (CP7–CP10) are future phases.

## Reference docs

- `docs/adr/ADR-001` — Strangler Fig
- `docs/adr/ADR-002` — Clean Architecture layers
- `docs/adr/ADR-011` — DDD building blocks (aggregate, value object, repository port)
- `docs/adr/ADR-013` — Dual-Write pattern and read modes
- `phase-02-analysis/context-mapping.md` — Anti-Corruption Layer explanation