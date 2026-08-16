package collector

// Catalog queries used by the collector. Kept as named constants (and mirrored
// in ../../sql/*.sql) so they are easy to audit — Atlas only ever issues
// read-only introspection queries plus writes to its own size-history table.

const qVersion = `SELECT version()`

const qDatabase = `SELECT current_database(), pg_database_size(current_database())`

const qTopTables = `
SELECT n.nspname,
       c.relname,
       pg_total_relation_size(c.oid)                             AS total,
       pg_relation_size(c.oid)                                   AS heap,
       pg_indexes_size(c.oid)                                    AS idx,
       COALESCE(pg_total_relation_size(c.reltoastrelid), 0)      AS toast,
       c.reltuples::bigint                                       AS rows
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'r'
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
ORDER BY total DESC
LIMIT 15`

const qCacheHit = `
SELECT COALESCE(sum(heap_blks_hit)::float
       / NULLIF(sum(heap_blks_hit) + sum(heap_blks_read), 0), 1.0)
FROM pg_statio_user_tables`

const qConnections = `
SELECT (SELECT count(*) FROM pg_stat_activity),
       current_setting('max_connections')::int`

const qWraparound = `SELECT COALESCE(max(age(datfrozenxid)), 0) FROM pg_database`

const qLongestQuery = `
SELECT COALESCE(max(EXTRACT(EPOCH FROM (now() - query_start))), 0)
FROM pg_stat_activity
WHERE state = 'active'
  AND query NOT ILIKE '%pg_stat_activity%'`

const qUptimeDays = `SELECT EXTRACT(EPOCH FROM (now() - pg_postmaster_start_time())) / 86400`

const qUnusedIndexes = `
SELECT s.schemaname, s.relname, s.indexrelname,
       pg_relation_size(s.indexrelid) AS size, s.idx_scan
FROM pg_stat_user_indexes s
JOIN pg_index i ON i.indexrelid = s.indexrelid
WHERE s.idx_scan = 0
  AND NOT i.indisunique
  AND NOT i.indisprimary
ORDER BY size DESC
LIMIT 20`

const qMissingIndexes = `
SELECT schemaname, relname, seq_scan, seq_tup_read, n_live_tup
FROM pg_stat_user_tables
WHERE seq_scan > 0
  AND n_live_tup > 10000
  AND seq_scan > COALESCE(idx_scan, 0)
ORDER BY seq_tup_read DESC
LIMIT 15`

const qVacuum = `
SELECT schemaname, relname, n_dead_tup, n_live_tup,
       COALESCE(to_char(last_autovacuum, 'YYYY-MM-DD HH24:MI'), ''),
       COALESCE(to_char(last_autoanalyze, 'YYYY-MM-DD HH24:MI'), '')
FROM pg_stat_user_tables
WHERE n_dead_tup > 0
ORDER BY n_dead_tup DESC
LIMIT 20`

// Atlas maintains its own daily size history in a dedicated schema so the
// forecast is based on real observations rather than a guess.
const qEnsureHistory = `
CREATE SCHEMA IF NOT EXISTS atlas;
CREATE TABLE IF NOT EXISTS atlas.size_history (
    day   date PRIMARY KEY,
    bytes bigint NOT NULL
)`

const qRecordSize = `
INSERT INTO atlas.size_history (day, bytes)
VALUES (current_date, $1)
ON CONFLICT (day) DO UPDATE SET bytes = EXCLUDED.bytes`

const qLoadHistory = `
SELECT to_char(day, 'YYYY-MM-DD'), bytes
FROM atlas.size_history
ORDER BY day
LIMIT 90`
