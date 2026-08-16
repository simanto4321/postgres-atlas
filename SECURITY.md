# Security Policy

## Reporting a vulnerability

Please email **msimanto46@gmail.com** with details. Do not open a public issue
for security-sensitive reports. You'll get an acknowledgement within a few days.

## Design safeguards

- **Read-only by design:** Atlas only issues catalog introspection queries
  (`pg_stat_*`, `pg_class`, `pg_database_size`). Suggested remediation SQL is
  returned as text for a human to review — never executed automatically.
- **No remote code execution surface:** recommendations are plain strings; the
  API has no `eval` / dynamic SQL execution path.
- **Optional write scope:** the only write Atlas performs is to its own
  `atlas.size_history` table (for capacity forecasting). Grant a dedicated role
  with `SELECT` on catalogs + `INSERT` on that table.
- **Secrets:** the DSN is environment-driven (`ATLAS_DSN`); `.env` is
  git-ignored and only `.env.example` is committed.
