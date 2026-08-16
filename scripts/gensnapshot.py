"""Generate docs/sample-report.json using the same formulas as the Go analyzer."""
from __future__ import annotations

import json
import math
from datetime import date, datetime, timedelta, timezone
from pathlib import Path


def human_bytes(b: int) -> str:
    unit = 1024
    if b < unit:
        return f"{b} bytes"
    div, exp = unit, 0
    n = b // unit
    while n >= unit:
        div *= unit
        exp += 1
        n //= unit
    return f"{b / div:.1f} {['kB','MB','GB','TB','PB'][exp]}"


def forecast(history: list[dict], threshold: int) -> dict:
    n = len(history)
    if n < 2:
        return {"daily_growth_bytes": 0, "daily_growth_human": "0 bytes",
                "threshold_bytes": threshold, "days_to_threshold": float("inf"),
                "projected_full": ""}
    sx = sy = sxy = sxx = 0.0
    for i, p in enumerate(history):
        x, y = float(i), float(p["bytes"])
        sx += x; sy += y; sxy += x * y; sxx += x * x
    denom = n * sxx - sx * sx
    slope = (n * sxy - sx * sy) / denom if denom else 0.0
    slope = max(0.0, slope)
    current = history[-1]["bytes"]
    out = {
        "daily_growth_bytes": int(slope),
        "daily_growth_human": human_bytes(int(slope)),
        "threshold_bytes": threshold,
        "days_to_threshold": float("inf"),
        "projected_full": "",
    }
    if slope > 0 and threshold > current:
        days = (threshold - current) / slope
        out["days_to_threshold"] = round(days, 1)
        out["projected_full"] = (date.today() + timedelta(days=int(days))).isoformat()
    return out


def score_health(h: dict, worst_dead: float) -> None:
    score = 100
    checks = []

    def chk(name, status, detail):
        checks.append({"name": name, "status": status, "detail": detail})

    c = h["cache_hit_ratio"]
    if c >= 0.99:
        chk("Cache hit ratio", "ok", f"{c*100:.1f}%")
    elif c >= 0.95:
        score -= 8; chk("Cache hit ratio", "warn", f"{c*100:.1f}%")
    else:
        score -= 18; chk("Cache hit ratio", "critical", f"{c*100:.1f}%")

    frac = h["connections_used"] / h["connections_max"]
    if frac < 0.7:
        chk("Connections", "ok", f"{h['connections_used']} / {h['connections_max']}")
    elif frac < 0.9:
        score -= 8; chk("Connections", "warn", f"{h['connections_used']} / {h['connections_max']}")
    else:
        score -= 16; chk("Connections", "critical", f"{h['connections_used']} / {h['connections_max']}")

    w = h["wraparound_pct"]
    if w < 0.5:
        chk("XID wraparound", "ok", f"{w*100:.1f}%")
    elif w < 0.8:
        score -= 12; chk("XID wraparound", "warn", f"{w*100:.1f}%")
    else:
        score -= 30; chk("XID wraparound", "critical", f"{w*100:.1f}%")

    if worst_dead < 0.1:
        chk("Dead tuples", "ok", f"{worst_dead*100:.1f}%")
    elif worst_dead < 0.25:
        score -= 8; chk("Dead tuples", "warn", f"{worst_dead*100:.1f}%")
    else:
        score -= 16; chk("Dead tuples", "critical", f"{worst_dead*100:.1f}%")

    if h["longest_query_sec"] >= 300:
        score -= 10; chk("Long-running query", "warn", f"{h['longest_query_sec']:.0f}s")
    else:
        chk("Long-running query", "ok", f"{h['longest_query_sec']:.0f}s" if h["longest_query_sec"] else "none")

    score = max(0, score)
    grade = "A" if score >= 90 else "B" if score >= 80 else "C" if score >= 70 else "D" if score >= 60 else "F"
    h["score"] = score
    h["grade"] = grade
    h["checks"] = checks


def recommend(r: dict) -> list[dict]:
    recs = []
    f = r["capacity"]["forecast"]
    if 0 < f["days_to_threshold"] < 60:
        sev = "critical" if f["days_to_threshold"] < 21 else "warning"
        recs.append({
            "id": "capacity-forecast", "severity": sev, "category": "Capacity",
            "title": f"Storage projected to hit threshold in {f['days_to_threshold']:.0f} days",
            "rationale": f"Growing ~{f['daily_growth_human']}/day; at this rate the {human_bytes(f['threshold_bytes'])} threshold is reached on {f['projected_full']}.",
            "action_sql": "-- Plan storage/partitioning; archive cold data or expand the volume.",
        })
    if r["health"]["wraparound_pct"] >= 0.5:
        sev = "critical" if r["health"]["wraparound_pct"] >= 0.8 else "warning"
        recs.append({
            "id": "wraparound", "severity": sev, "category": "Reliability",
            "title": "Transaction-ID wraparound risk is elevated",
            "rationale": f"Oldest unfrozen XID is {r['health']['wraparound_pct']*100:.1f}% of the wraparound budget. Autovacuum may not be keeping up.",
            "action_sql": "VACUUM (FREEZE, VERBOSE);  -- and review autovacuum_freeze_max_age",
        })
    for u in r["indexes"]["unused"]:
        if u["size_bytes"] < 1 << 20:
            continue
        recs.append({
            "id": f"unused-index-{u['index']}", "severity": "warning", "category": "Indexes",
            "title": f"Unused index \"{u['index']}\" ({u['size_human']})",
            "rationale": f"{u['scans']} scans since stats reset on {u['schema']}.{u['table']}. It still costs write amplification and disk.",
            "action_sql": f"DROP INDEX CONCURRENTLY {u['schema']}.{u['index']};",
        })
    for m in r["indexes"]["missing"]:
        recs.append({
            "id": f"missing-index-{m['table']}", "severity": "warning", "category": "Indexes",
            "title": f"Table \"{m['table']}\" is heavily sequentially scanned",
            "rationale": m["reason"],
            "action_sql": f"-- Review predicates on {m['schema']}.{m['table']} and add a targeted index (e.g. on filter/join columns).",
        })
    for d in r["indexes"]["duplicate"]:
        recs.append({
            "id": f"duplicate-index-{d['table']}", "severity": "info", "category": "Indexes",
            "title": f"Duplicate indexes on \"{d['table']}\" ({d['columns']})",
            "rationale": f"Indexes {d['indexes']} cover the same columns; keep one.",
            "action_sql": f"DROP INDEX CONCURRENTLY {d['schema']}.{d['indexes'][-1]};",
        })
    for b in r["bloat"]:
        if b["bloat_ratio"] < 0.3 or b["bloat_bytes"] < 50 << 20:
            continue
        sev = "warning" if b["bloat_ratio"] >= 0.5 else "info"
        recs.append({
            "id": f"bloat-{b['name']}", "severity": sev, "category": "Bloat",
            "title": f"Table \"{b['name']}\" is ~{b['bloat_ratio']*100:.0f}% bloat ({b['bloat_human']} reclaimable)",
            "rationale": "Dead space inflates scans and backups. A rewrite reclaims it (locks briefly; prefer pg_repack online).",
            "action_sql": f"VACUUM (FULL, ANALYZE) {b['schema']}.{b['name']};  -- or: pg_repack -t {b['name']}",
        })
    for v in r["vacuum"]:
        if v["dead_ratio"] < 0.2:
            continue
        sev = "warning" if v["dead_ratio"] >= 0.4 else "info"
        last = v["last_autovacuum"] or "never"
        recs.append({
            "id": f"vacuum-{v['name']}", "severity": sev, "category": "Vacuum",
            "title": f"Table \"{v['name']}\" has {v['dead_ratio']*100:.0f}% dead tuples",
            "rationale": f"{v['dead_tuples']} dead vs {v['live_tuples']} live tuples; last autovacuum {last}. Consider a tighter autovacuum scale factor.",
            "action_sql": f"VACUUM (ANALYZE) {v['schema']}.{v['name']};",
        })
    if r["health"]["cache_hit_ratio"] < 0.95:
        recs.append({
            "id": "cache-hit", "severity": "warning", "category": "Performance",
            "title": f"Buffer cache hit ratio is low ({r['health']['cache_hit_ratio']*100:.1f}%)",
            "rationale": "A hit ratio under 95% often means shared_buffers is undersized for the working set.",
            "action_sql": "-- Consider increasing shared_buffers / effective_cache_size, or add RAM.",
        })
    order = {"critical": 0, "warning": 1, "info": 2}
    recs.sort(key=lambda x: order[x["severity"]])
    return recs


def main() -> None:
    gb, mb = 1 << 30, 1 << 20
    history = []
    base = 42 * gb
    day0 = date.today() - timedelta(days=29)
    for i in range(30):
        # ~1.1 GB/day growth — interesting capacity forecast under 60 days.
        bytes_ = base + i * 1100 * mb + (i % 5) * 20 * mb
        history.append({"day": (day0 + timedelta(days=i)).isoformat(), "bytes": bytes_})
    current = history[-1]["bytes"]

    tables = [
        {"schema": "public", "name": "orders", "total_bytes": 18*gb, "heap_bytes": 14*gb, "index_bytes": 3*gb, "toast_bytes": 1*gb, "rows": 48_200_000},
        {"schema": "public", "name": "order_items", "total_bytes": 12*gb, "heap_bytes": 10*gb, "index_bytes": 2*gb, "toast_bytes": 0, "rows": 162_000_000},
        {"schema": "public", "name": "events", "total_bytes": 9*gb, "heap_bytes": 8*gb, "index_bytes": 900*mb, "toast_bytes": 100*mb, "rows": 410_000_000},
        {"schema": "public", "name": "customers", "total_bytes": 2*gb, "heap_bytes": 1400*mb, "index_bytes": 500*mb, "toast_bytes": 100*mb, "rows": 4_800_000},
        {"schema": "public", "name": "products", "total_bytes": 420*mb, "heap_bytes": 280*mb, "index_bytes": 120*mb, "toast_bytes": 20*mb, "rows": 180_000},
        {"schema": "analytics", "name": "daily_revenue", "total_bytes": 180*mb, "heap_bytes": 150*mb, "index_bytes": 30*mb, "toast_bytes": 0, "rows": 2_200_000},
    ]
    for t in tables:
        t["total_human"] = human_bytes(t["total_bytes"])

    unused = [
        {"schema": "public", "table": "orders", "index": "idx_orders_legacy_status", "size_bytes": 820*mb, "scans": 0},
        {"schema": "public", "table": "customers", "index": "idx_customers_old_email", "size_bytes": 210*mb, "scans": 0},
        {"schema": "public", "table": "products", "index": "idx_products_sku_lower", "size_bytes": 18*mb, "scans": 0},
    ]
    for u in unused:
        u["size_human"] = human_bytes(u["size_bytes"])

    missing = [{
        "schema": "public", "table": "order_items", "seq_scan": 18420,
        "seq_tup_read": 2_900_000_000, "live_rows": 162_000_000, "avg_rows_read": 157400.0,
        "reason": "18420 sequential scans read 2900000000 rows (~157400 rows/scan) on a 162000000-row table; an index on the filtered/joined columns would avoid full scans.",
    }]

    bloat = [
        {"schema": "public", "name": "orders", "bloat_bytes": 3*gb, "bloat_ratio": 0.42},
        {"schema": "public", "name": "events", "bloat_bytes": 1800*mb, "bloat_ratio": 0.36},
    ]
    for b in bloat:
        b["bloat_human"] = human_bytes(b["bloat_bytes"])

    vacuum = [
        {"schema": "public", "name": "orders", "dead_tuples": 6_400_000, "live_tuples": 48_200_000,
         "last_autovacuum": "2026-08-14 03:12", "last_autoanalyze": "2026-08-14 03:40"},
        {"schema": "public", "name": "events", "dead_tuples": 38_000_000, "live_tuples": 410_000_000,
         "last_autovacuum": "2026-08-13 21:05", "last_autoanalyze": "2026-08-13 21:40"},
        {"schema": "public", "name": "customers", "dead_tuples": 90_000, "live_tuples": 4_800_000,
         "last_autovacuum": "2026-08-15 01:10", "last_autoanalyze": "2026-08-15 01:12"},
    ]
    worst = 0.0
    for v in vacuum:
        total = v["dead_tuples"] + v["live_tuples"]
        v["dead_ratio"] = v["dead_tuples"] / total if total else 0.0
        worst = max(worst, v["dead_ratio"])

    wrap_age = 1_180_000_000
    health = {
        "cache_hit_ratio": 0.972,
        "connections_used": 62,
        "connections_max": 100,
        "wraparound_age": wrap_age,
        "wraparound_pct": wrap_age / 2146483647.0,
        "longest_query_sec": 42.0,
        "uptime_days": 47.3,
        "score": 0, "grade": "", "checks": [],
    }
    score_health(health, worst)

    report = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "database": "commerce",
        "version": "PostgreSQL 16.4 on x86_64-pc-linux-gnu",
        "capacity": {
            "total_bytes": current,
            "total_human": human_bytes(current),
            "forecast": forecast(history, 100 * gb),
            "top_tables": tables,
            "history": history,
        },
        "health": health,
        "bloat": bloat,
        "indexes": {
            "unused": unused,
            "duplicate": [{"schema": "public", "table": "customers",
                           "indexes": ["customers_email_key", "idx_customers_email"], "columns": "email"}],
            "missing": missing,
        },
        "vacuum": vacuum,
        "recommendations": [],
    }
    report["recommendations"] = recommend(report)

    out = Path(__file__).resolve().parents[1] / "docs" / "sample-report.json"
    out.write_text(json.dumps(report, indent=2))
    print(f"wrote {out} (score {health['score']}/{health['grade']}, {len(report['recommendations'])} recommendations, days-to-threshold {report['capacity']['forecast']['days_to_threshold']})")


if __name__ == "__main__":
    main()
