<div align="center">

# ▣ Postgres Atlas

**A PostgreSQL operations &amp; capacity control plane.**

Introspects a live database (or serves a demo snapshot), scores its health, forecasts when storage will fill, finds unused/duplicate/missing indexes and vacuum pressure, and turns every finding into a prioritized recommendation with ready-to-run SQL.

[![CI](https://github.com/simanto4321/postgres-atlas/actions/workflows/ci.yml/badge.svg)](https://github.com/simanto4321/postgres-atlas/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.22-00add8?logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169e1?logo=postgresql&logoColor=white)
![React](https://img.shields.io/badge/React-18-61dafb?logo=react&logoColor=white)
![TypeScript](https://img.shields.io/badge/TypeScript-5-3178c6?logo=typescript&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

</div>

![Postgres Atlas dashboard](docs/assets/dashboard.png)

---

## Why this exists

Most Postgres "monitoring" stops at charts. Atlas goes one step further: it
**diagnoses** and **prescribes**. A health score tells you how the cluster is
doing; a capacity forecast tells you when you'll run out of disk; and a
recommendation engine turns every finding into an actionable SQL snippet a DBA
can review and run.

It is deliberately **read-only** for the database under observation —
remediation SQL is suggested, never executed.

## Features

- **Health score (0–100 + letter grade)** — cache hit ratio, connection
  saturation, XID wraparound risk, dead-tuple pressure, long-running queries.
- **Capacity forecast** — least-squares fit over daily size history → growth
  rate, days-to-threshold, projected fill date.
- **Index hygiene** — unused indexes (0 scans), duplicate indexes, and
  missing-index candidates (heavy sequential scans).
- **Vacuum / bloat pressure** — dead-tuple ratios and reclaimable space.
- **Prioritized recommendations** — each with severity, rationale, and
  ready-to-run `ActionSQL` (`DROP INDEX CONCURRENTLY`, `VACUUM (FREEZE)`, …).
- **Demo mode** — ships a realistic sample report so the dashboard runs with
  zero infrastructure; flip to live mode with a DSN.

## Architecture

```mermaid
flowchart LR
    PG[(PostgreSQL)] -->|read-only catalogs| COL[Go collector]
    COL --> AN[analyze: forecast · score · recommend]
    AN --> API[/api/report]
    API --> WEB[React + TypeScript dashboard]
    SNAP[sample-report.json] -. demo .-> API
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for scoring rules and SQL.

## Quick start

### Option A — one command (Docker)

```bash
docker compose up --build
# open http://localhost:8080  (demo snapshot by default)
```

To introspect the compose Postgres instead of the snapshot, set
`ATLAS_DSN=postgres://atlas:atlas@db:5432/commerce?sslmode=disable` on the
`atlas` service (and unset `ATLAS_SNAPSHOT`).

### Option B — local demo (no Docker, no Postgres)

```bash
# Backend (Go 1.22+)
cd backend
go mod tidy
go run ./cmd/atlas --snapshot ../docs/sample-report.json --addr :8000

# Frontend
cd frontend
npm install && npm run dev     # http://localhost:5173
```

### Option C — live introspection

```bash
export ATLAS_DSN='postgres://user:pass@host:5432/dbname?sslmode=require'
go run ./cmd/atlas --dsn "$ATLAS_DSN" --threshold-gb 100
```

Atlas will create an `atlas.size_history` table (its only write) so the
forecast has real observations to fit.

## What the demo report shows

The bundled snapshot models a mid-size `commerce` database:

| Signal | Finding |
|--------|---------|
| Health | Grade **C** (72) — elevated wraparound + dead tuples + cache warn |
| Capacity | ~1.1 GB/day growth → threshold in **~25 days** |
| Indexes | Unused legacy indexes totaling ~1 GB; heavy seq scans on `order_items` |
| Recommendations | 9 prioritized actions with `DROP INDEX CONCURRENTLY` / `VACUUM` SQL |

## Testing

```bash
cd backend
go test -race ./...          # pure analyze + API cache tests (no DB required)
```

```bash
cd frontend
npm run build                # tsc type-check + production build
```

CI also boots the binary against the sample snapshot and curls `/api/report`.

## Project structure

```
postgres-atlas/
├── backend/
│   ├── cmd/atlas/           # server (live DSN or --snapshot)
│   ├── cmd/gensnapshot/     # rebuild the demo report from the analyze engine
│   └── internal/
│       ├── analyze/         # forecast, health score, recommendations (pure)
│       ├── collector/       # read-only catalog queries via pgx
│       ├── api/             # HTTP layer
│       └── model/           # shared report shape
├── frontend/                # React + TypeScript dashboard (Vite)
├── sql/catalog.sql          # auditable mirror of the collector queries
├── docs/                    # architecture + sample-report.json + assets
└── docker-compose.yml
```

## Roadmap

- Continuous size history export to Prometheus
- Autovacuum tuning suggestions from observed dead-tuple rates
- Per-table IO heatmaps from `pg_stat_io` (PG16+)
- Slack / email digest of critical recommendations

## Author

**Mehedi Ashraf Simanto** — [@simanto4321](https://github.com/simanto4321) · msimanto46@gmail.com

Licensed under the [MIT License](LICENSE).
