export interface Report {
  generated_at: string;
  database: string;
  version: string;
  capacity: Capacity;
  health: Health;
  bloat: BloatEntry[];
  indexes: IndexReport;
  vacuum: VacuumEntry[];
  recommendations: Recommendation[];
}

export interface Capacity {
  total_bytes: number;
  total_human: string;
  forecast: Forecast;
  top_tables: Table[];
  history: SizePoint[];
}

export interface Forecast {
  daily_growth_bytes: number;
  daily_growth_human: string;
  threshold_bytes: number;
  days_to_threshold: number;
  projected_full: string;
}

export interface SizePoint {
  day: string;
  bytes: number;
}

export interface Table {
  schema: string;
  name: string;
  total_bytes: number;
  heap_bytes: number;
  index_bytes: number;
  toast_bytes: number;
  rows: number;
  total_human: string;
}

export interface Health {
  score: number;
  grade: string;
  cache_hit_ratio: number;
  connections_used: number;
  connections_max: number;
  wraparound_age: number;
  wraparound_pct: number;
  longest_query_sec: number;
  uptime_days: number;
  checks: HealthCheck[];
}

export interface HealthCheck {
  name: string;
  status: "ok" | "warn" | "critical";
  detail: string;
}

export interface BloatEntry {
  schema: string;
  name: string;
  bloat_bytes: number;
  bloat_ratio: number;
  bloat_human: string;
}

export interface IndexReport {
  unused: UnusedIndex[];
  duplicate: DuplicateIndex[];
  missing: MissingIndex[];
}

export interface UnusedIndex {
  schema: string;
  table: string;
  index: string;
  size_bytes: number;
  size_human: string;
  scans: number;
}

export interface DuplicateIndex {
  schema: string;
  table: string;
  indexes: string[];
  columns: string;
}

export interface MissingIndex {
  schema: string;
  table: string;
  seq_scan: number;
  seq_tup_read: number;
  live_rows: number;
  avg_rows_read: number;
  reason: string;
}

export interface VacuumEntry {
  schema: string;
  name: string;
  dead_tuples: number;
  live_tuples: number;
  dead_ratio: number;
  last_autovacuum: string;
  last_autoanalyze: string;
}

export interface Recommendation {
  id: string;
  severity: "info" | "warning" | "critical";
  category: string;
  title: string;
  rationale: string;
  action_sql: string;
}

export async function fetchReport(): Promise<Report> {
  const res = await fetch("/api/report");
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<Report>;
}
