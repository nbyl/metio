# Architecture Decision Records

This directory records significant architecture decisions for Metio using a lightweight
[MADR](https://adr.github.io/madr/)-style format.

## Conventions

- One file per decision, named `NNNN-short-title.md` (zero-padded, incrementing).
- Each record captures: Status, Context, Decision Drivers, Considered Options, Decision
  Outcome, and Consequences.
- Status values: `Proposed`, `Accepted`, `Superseded by ADR-XXXX`, `Deprecated`.
- ADRs are immutable once accepted; supersede them with a new ADR rather than rewriting.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [0001](0001-controller-agent-api.md) | Machine-agent writes state through a Controller API | Accepted |
| [0003](0003-postgresql-state-backend.md) | PostgreSQL state backend (Cloud SQL or BYO) | Accepted |
