# Contributing

Thanks for your interest! Keep the analysis layer pure and the collector thin.

## Development setup

```bash
# Backend (Go 1.22+)
cd backend
go mod tidy
go test ./...
go run ./cmd/atlas --snapshot ../docs/sample-report.json

# Frontend
cd frontend && npm install && npm run build
```

## Guidelines

- Pure analysis (forecast, health score, recommendations) lives in
  `internal/analyze` and must be fully unit-tested with no database.
- Catalog SQL lives in `internal/collector/queries.go` and is mirrored under
  `sql/` for auditability. New queries must be read-only.
- Recommendation `ActionSQL` is advisory — never execute it from the server.
- Run `go test ./...` and `npm run build` before opening a PR.

## Commit style

Conventional-ish, imperative mood: `add wraparound check`, `fix forecast slope`.
