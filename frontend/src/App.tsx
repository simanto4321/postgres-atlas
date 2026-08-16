import { useEffect, useMemo, useState } from "react";
import { fetchReport, Report, Recommendation } from "./api";

function pct(x: number) {
  return `${(x * 100).toFixed(1)}%`;
}

function Status({ s }: { s: string }) {
  return <span className={`status ${s}`}>{s}</span>;
}

function Severity({ s }: { s: string }) {
  return <span className={`sev ${s}`}>{s}</span>;
}

function ScoreRing({ score, grade }: { score: number; grade: string }) {
  const r = 52;
  const c = 2 * Math.PI * r;
  const offset = c * (1 - score / 100);
  const color = score >= 80 ? "#34d399" : score >= 70 ? "#fbbf24" : "#f87171";
  return (
    <div className="ring">
      <svg viewBox="0 0 120 120">
        <circle cx="60" cy="60" r={r} fill="none" stroke="#1a2740" strokeWidth="10" />
        <circle
          cx="60" cy="60" r={r} fill="none" stroke={color} strokeWidth="10"
          strokeDasharray={c} strokeDashoffset={offset} strokeLinecap="round"
          transform="rotate(-90 60 60)"
        />
      </svg>
      <div className="ring-label">
        <div className="grade">{grade}</div>
        <div className="score">{score}</div>
      </div>
    </div>
  );
}

function CapacityChart({ history, threshold }: { history: { day: string; bytes: number }[]; threshold: number }) {
  if (!history.length) return null;
  const max = Math.max(threshold, ...history.map((h) => h.bytes));
  const min = Math.min(...history.map((h) => h.bytes));
  const pad = (max - min) * 0.1 || max * 0.05;
  const lo = Math.max(0, min - pad);
  const hi = max + pad;
  const w = 520;
  const h = 140;
  const pts = history.map((p, i) => {
    const x = (i / (history.length - 1 || 1)) * (w - 20) + 10;
    const y = h - 10 - ((p.bytes - lo) / (hi - lo)) * (h - 20);
    return `${x},${y}`;
  });
  const thrY = h - 10 - ((threshold - lo) / (hi - lo)) * (h - 20);
  return (
    <svg className="cap-chart" viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none">
      <line x1="10" y1={thrY} x2={w - 10} y2={thrY} stroke="#f87171" strokeDasharray="4 4" strokeWidth="1.5" />
      <polyline fill="none" stroke="#60a5fa" strokeWidth="2.5" points={pts.join(" ")} />
      <polygon
        fill="url(#fade)"
        points={`10,${h - 10} ${pts.join(" ")} ${w - 10},${h - 10}`}
      />
      <defs>
        <linearGradient id="fade" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#60a5fa" stopOpacity="0.35" />
          <stop offset="1" stopColor="#60a5fa" stopOpacity="0" />
        </linearGradient>
      </defs>
    </svg>
  );
}

function RecCard({ r }: { r: Recommendation }) {
  return (
    <div className={`rec ${r.severity}`}>
      <div className="rec-head">
        <Severity s={r.severity} />
        <span className="cat">{r.category}</span>
      </div>
      <h3>{r.title}</h3>
      <p>{r.rationale}</p>
      <pre><code>{r.action_sql}</code></pre>
    </div>
  );
}

export default function App() {
  const [report, setReport] = useState<Report | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchReport()
      .then(setReport)
      .catch((e) => setError(String(e)));
  }, []);

  const daysLabel = useMemo(() => {
    if (!report) return "";
    const d = report.capacity.forecast.days_to_threshold;
    if (!Number.isFinite(d)) return "no fill projected";
    return `${d.toFixed(0)} days to threshold`;
  }, [report]);

  if (error) {
    return <div className="app"><div className="error">Failed to load report: {error}</div></div>;
  }
  if (!report) {
    return <div className="app"><div className="loading">Loading atlas report…</div></div>;
  }

  const h = report.health;
  const c = report.capacity;

  return (
    <div className="app">
      <header className="topbar">
        <div className="brand">
          <span className="logo">▣</span>
          <div>
            <h1>Postgres Atlas</h1>
            <p>{report.database} · {report.version.split(" ").slice(0, 2).join(" ")}</p>
          </div>
        </div>
        <a className="author" href="https://github.com/simanto4321" target="_blank" rel="noreferrer">
          Mehedi Ashraf Simanto
        </a>
      </header>

      <section className="hero">
        <div className="hero-left">
          <ScoreRing score={h.score} grade={h.grade} />
          <div>
            <h2>Health score</h2>
            <div className="checks">
              {h.checks.map((ch) => (
                <div key={ch.name} className="check">
                  <Status s={ch.status} />
                  <span className="check-name">{ch.name}</span>
                  <span className="check-detail">{ch.detail}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
        <div className="hero-right">
          <div className="kpi-grid">
            <div className="kpi"><div className="k">Database size</div><div className="v">{c.total_human}</div></div>
            <div className="kpi"><div className="k">Daily growth</div><div className="v">{c.forecast.daily_growth_human}</div></div>
            <div className="kpi"><div className="k">Capacity</div><div className="v">{daysLabel}</div></div>
            <div className="kpi"><div className="k">Cache hit</div><div className="v">{pct(h.cache_hit_ratio)}</div></div>
            <div className="kpi"><div className="k">Connections</div><div className="v">{h.connections_used}/{h.connections_max}</div></div>
            <div className="kpi"><div className="k">Uptime</div><div className="v">{h.uptime_days.toFixed(1)}d</div></div>
          </div>
        </div>
      </section>

      <div className="grid">
        <section className="panel">
          <h2>Capacity forecast</h2>
          <p className="muted">
            Least-squares fit over {c.history.length} days · threshold{" "}
            {(c.forecast.threshold_bytes / (1 << 30)).toFixed(0)} GB · projected full{" "}
            {c.forecast.projected_full || "—"}
          </p>
          <CapacityChart history={c.history} threshold={c.forecast.threshold_bytes} />
          <div className="axis">
            <span>{c.history[0]?.day}</span>
            <span className="thr">threshold</span>
            <span>{c.history[c.history.length - 1]?.day}</span>
          </div>
        </section>

        <section className="panel">
          <h2>Top tables by size</h2>
          <table>
            <thead>
              <tr><th>Table</th><th>Rows</th><th>Size</th></tr>
            </thead>
            <tbody>
              {c.top_tables.map((t) => (
                <tr key={`${t.schema}.${t.name}`}>
                  <td className="mono">{t.schema}.{t.name}</td>
                  <td>{t.rows.toLocaleString()}</td>
                  <td>{t.total_human}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </div>

      <section className="panel">
        <h2>Recommendations — {report.recommendations.length}</h2>
        <p className="muted">Prioritized by severity. Each one includes ready-to-run SQL.</p>
        <div className="recs">
          {report.recommendations.map((r) => (
            <RecCard key={r.id} r={r} />
          ))}
        </div>
      </section>

      <div className="grid">
        <section className="panel">
          <h2>Unused indexes</h2>
          <table>
            <thead>
              <tr><th>Index</th><th>Table</th><th>Size</th><th>Scans</th></tr>
            </thead>
            <tbody>
              {report.indexes.unused.map((u) => (
                <tr key={u.index}>
                  <td className="mono">{u.index}</td>
                  <td>{u.table}</td>
                  <td>{u.size_human}</td>
                  <td>{u.scans}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>

        <section className="panel">
          <h2>Vacuum pressure</h2>
          <table>
            <thead>
              <tr><th>Table</th><th>Dead ratio</th><th>Last autovacuum</th></tr>
            </thead>
            <tbody>
              {report.vacuum.map((v) => (
                <tr key={v.name}>
                  <td className="mono">{v.name}</td>
                  <td>
                    <div className="bar">
                      <div className="bar-fill" style={{ width: `${Math.min(100, v.dead_ratio * 100)}%` }} />
                      <span>{pct(v.dead_ratio)}</span>
                    </div>
                  </td>
                  <td>{v.last_autovacuum || "never"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      </div>

      <footer className="foot">
        Postgres Atlas · Go + PostgreSQL ops control plane · built by{" "}
        <a href="https://github.com/simanto4321" target="_blank" rel="noreferrer">Mehedi Ashraf Simanto</a>
      </footer>
    </div>
  );
}
