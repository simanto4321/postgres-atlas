-- Catalog queries used by Postgres Atlas (read-only).
-- Mirrored in backend/internal/collector/queries.go.

-- Top tables by total size
SELECT n.nspname,
       c.relname,
       pg_total_relation_size(c.oid) AS total,
       pg_relation_size(c.oid)       AS heap,
       pg_indexes_size(c.oid)        AS idx,
       COALESCE(pg_total_relation_size(c.reltoastrelid), 0) AS toast,
       c.reltuples::bigint           AS rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r'
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY total DESC
LIMIT 15;

-- Unused indexes (never scanned, not unique/primary)
SELECT s.schemaname, s.relname, s.indexrelname,
       pg_relation_size(s.indexrelid) AS size, s.idx_scan
FROM pg_stat_user_indexes s
JOIN pg_index i ON i.indexrelid = s.indexrelid
WHERE s.idx_scan = 0
  AND NOT i.indisunique
  AND NOT i.indisprimary
ORDER BY size DESC
LIMIT 20;

-- Heavy sequential scans (missing-index candidates)
SELECT schemaname, relname, seq_scan, seq_tup_read, n_live_tup
FROM pg_stat_user_tables
WHERE seq_scan > 0
  AND n_live_tup > 10000
  AND seq_scan > COALESCE(idx_scan, 0)
ORDER BY seq_tup_read DESC
LIMIT 15;

-- Vacuum pressure
SELECT schemaname, relname, n_dead_tup, n_live_tup,
       last_autovacuum, last_autoanalyze
FROM pg_stat_user_tables
WHERE n_dead_tup > 0
ORDER BY n_dead_tup DESC
LIMIT 20;

-- Cache hit ratio
SELECT COALESCE(sum(heap_blks_hit)::float
       / NULLIF(sum(heap_blks_hit) + sum(heap_blks_read), 0), 1.0)
FROM pg_statio_user_tables;

-- XID wraparound age
SELECT COALESCE(max(age(datfrozenxid)), 0) FROM pg_database;
