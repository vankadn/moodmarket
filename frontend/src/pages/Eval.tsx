import { useEffect, useState } from "react";
import {
  getEvalSummary, getEvalDecisions, getActivity,
  EvalSummary, EvalDecision, ActivityDecision, AutoInvestConfig,
} from "../services/api";

interface Props {
  onBack: () => void;
  autoInvestConfigs?: AutoInvestConfig[];
}

function fmtPct(n: number, forceSign = false): string {
  const sign = forceSign && n > 0 ? "+" : "";
  return `${sign}${n.toFixed(2)}%`;
}

function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

function fmtDollars(n: number): string {
  return n.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
}

function configName(configId: string | undefined, configs: AutoInvestConfig[]): string {
  if (configId === undefined || configId === null || configId === "manual") return "Manual";
  if (configId === "") return "Manual";
  const cfg = configs.find(c => c.id === configId);
  return cfg?.name ?? "Deleted strategy";
}

export function Eval({ onBack, autoInvestConfigs = [] }: Props) {
  const [summary, setSummary] = useState<EvalSummary | null>(null);
  const [verdictMap, setVerdictMap] = useState<Map<string, EvalDecision>>(new Map());
  const [allDecisions, setAllDecisions] = useState<ActivityDecision[]>([]);
  const [totalInvested, setTotalInvested] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      getEvalSummary().catch((e: unknown) => { throw new Error(`eval/summary: ${e instanceof Error ? e.message : e}`); }),
      getEvalDecisions(1, 500).catch((e: unknown) => { throw new Error(`eval/decisions: ${e instanceof Error ? e.message : e}`); }),
      getActivity(null).catch((e: unknown) => { throw new Error(`activity: ${e instanceof Error ? e.message : e}`); }),
    ])
      .then(([s, evalDecisions, activity]) => {
        setSummary(s);
        setTotalInvested(activity.total_invested);
        const map = new Map<string, EvalDecision>();
        for (const d of evalDecisions) map.set(d.id, d);
        setVerdictMap(map);
        setAllDecisions(activity.decisions);
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Failed to load data"))
      .finally(() => setLoading(false));
  }, []);

  const winPct = summary ? Math.round(summary.win_rate * 100) : 0;
  const hasVerdicts = (summary?.verdicted_decisions ?? 0) > 0;

  // Merge by_strategy rows that map to the same display name (e.g. multiple configs named "Long Term")
  const mergedStrategies = (() => {
    if (!summary?.by_strategy) return [];
    const map = new Map<string, typeof summary.by_strategy[0]>();
    for (const s of summary.by_strategy) {
      const name = configName(s.config_id, autoInvestConfigs);
      const existing = map.get(name);
      if (existing) {
        const total = existing.decision_count + s.decision_count;
        map.set(name, {
          ...existing,
          decision_count: total,
          win_rate: (existing.win_rate * existing.decision_count + s.win_rate * s.decision_count) / total,
          avg_return_pct: (existing.avg_return_pct * existing.decision_count + s.avg_return_pct * s.decision_count) / total,
        });
      } else {
        map.set(name, s);
      }
    }
    return Array.from(map.values());
  })();
  const showByStrategy = mergedStrategies.length > 1;

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.75rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#666", fontSize: "14px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h2 style={{ margin: 0, fontSize: "18px", fontWeight: 600 }}>Activity</h2>
      </div>

      {loading && <p style={{ color: "#999", fontSize: "14px" }}>Loading…</p>}
      {error && <p style={{ color: "#c0392b", fontSize: "14px" }}>{error}</p>}

      {!loading && !error && summary && (
        <>
          {/* Summary card */}
          <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1.25rem", marginBottom: "1.25rem" }}>
            {/* Total invested row — always shown */}
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem", marginBottom: hasVerdicts ? "1rem" : 0 }}>
              <div>
                <div style={{ fontSize: "11px", color: "#aaa", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "4px" }}>Total invested</div>
                <div style={{ fontSize: "24px", fontWeight: 700, color: "#111" }}>{fmtDollars(totalInvested)}</div>
                <div style={{ fontSize: "12px", color: "#aaa", marginTop: "2px" }}>
                  {summary.total_decisions} decision{summary.total_decisions !== 1 ? "s" : ""}
                </div>
              </div>
              {hasVerdicts ? (
                <div>
                  <div style={{ fontSize: "11px", color: "#aaa", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "4px" }}>Avg return</div>
                  <div style={{ fontSize: "24px", fontWeight: 700, color: summary.avg_return_pct >= 0 ? "#27ae60" : "#c0392b" }}>
                    {fmtPct(summary.avg_return_pct, true)}
                  </div>
                  <div style={{ fontSize: "12px", color: "#aaa", marginTop: "2px" }}>vs SPY {fmtPct(summary.avg_spy_return_pct, true)}</div>
                </div>
              ) : (
                <div style={{ display: "flex", alignItems: "center" }}>
                  <p style={{ color: "#aaa", fontSize: "13px", margin: 0 }}>Verdicts appear after trades settle.</p>
                </div>
              )}
            </div>

            {hasVerdicts && (
              <>
                {/* Win rate bar */}
                <div style={{ marginBottom: "10px" }}>
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "4px" }}>
                    <span style={{ fontSize: "11px", color: "#aaa", textTransform: "uppercase", letterSpacing: "0.06em" }}>Win rate vs SPY</span>
                    <span style={{ fontSize: "13px", fontWeight: 700, color: winPct >= 50 ? "#27ae60" : "#c0392b" }}>{winPct}%</span>
                  </div>
                  <div style={{ height: "6px", background: "#e8e8e8", borderRadius: "3px", overflow: "hidden" }}>
                    <div style={{ height: "100%", width: `${winPct}%`, background: winPct >= 50 ? "#27ae60" : "#c0392b", borderRadius: "3px", transition: "width 0.4s ease" }} />
                  </div>
                </div>

                <div style={{ fontSize: "12px", color: "#999", borderTop: "1px solid #eee", paddingTop: "10px" }}>
                  {summary.verdicted_decisions} of {summary.total_decisions} evaluated
                  {summary.total_decisions > summary.verdicted_decisions && (
                    <span style={{ color: "#bbb" }}> · {summary.total_decisions - summary.verdicted_decisions} pending</span>
                  )}
                </div>
              </>
            )}
          </div>

          {/* Best / Worst */}
          {hasVerdicts && (summary.best_decision || summary.worst_decision) && (
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "10px", marginBottom: "1.25rem" }}>
              {summary.best_decision && (
                <div style={{ background: "#f0faf4", borderRadius: "10px", padding: "0.875rem" }}>
                  <div style={{ fontSize: "10px", color: "#27ae60", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "4px" }}>Best</div>
                  <div style={{ fontSize: "18px", fontWeight: 700, color: "#27ae60" }}>{fmtPct(summary.best_decision.return_pct, true)}</div>
                  <div style={{ fontSize: "11px", color: "#888", marginTop: "2px" }}>{fmtDate(summary.best_decision.date)}</div>
                </div>
              )}
              {summary.worst_decision && (
                <div style={{ background: "#fdf4f4", borderRadius: "10px", padding: "0.875rem" }}>
                  <div style={{ fontSize: "10px", color: "#c0392b", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "4px" }}>Worst</div>
                  <div style={{ fontSize: "18px", fontWeight: 700, color: "#c0392b" }}>{fmtPct(summary.worst_decision.return_pct, true)}</div>
                  <div style={{ fontSize: "11px", color: "#888", marginTop: "2px" }}>{fmtDate(summary.worst_decision.date)}</div>
                </div>
              )}
            </div>
          )}

          {/* Per-strategy breakdown */}
          {showByStrategy && (
            <div style={{ marginBottom: "1.25rem" }}>
              <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", textTransform: "uppercase", letterSpacing: "0.07em", marginBottom: "8px" }}>
                By strategy
              </div>
              {mergedStrategies.map(s => (
                <div
                  key={configName(s.config_id, autoInvestConfigs)}
                  style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 12px", background: "#f8f8f8", borderRadius: "8px", marginBottom: "6px" }}
                >
                  <div>
                    <div style={{ fontSize: "14px", fontWeight: 500 }}>{configName(s.config_id, autoInvestConfigs)}</div>
                    <div style={{ fontSize: "12px", color: "#999" }}>{s.decision_count} decision{s.decision_count !== 1 ? "s" : ""}</div>
                  </div>
                  <div style={{ textAlign: "right" }}>
                    <div style={{ fontSize: "14px", fontWeight: 600, color: s.win_rate >= 0.5 ? "#27ae60" : "#c0392b" }}>
                      {Math.round(s.win_rate * 100)}% win
                    </div>
                    <div style={{ fontSize: "12px", color: s.avg_return_pct >= 0 ? "#27ae60" : "#c0392b" }}>
                      {fmtPct(s.avg_return_pct, true)} avg
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* Decision history — all decisions, verdict overlaid where available */}
          {allDecisions.length > 0 && (
            <div>
              <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", textTransform: "uppercase", letterSpacing: "0.07em", marginBottom: "8px" }}>
                Decision history
              </div>
              {allDecisions.map(d => {
                const evalD = verdictMap.get(d.id);
                const v = evalD?.verdict ?? null;
                return (
                  <div key={d.id} style={{ borderBottom: "1px solid #f0f0f0", padding: "12px 0" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                      <div>
                        <div style={{ fontSize: "14px", fontWeight: 500 }}>{fmtDate(d.timestamp)}</div>
                        <div style={{ fontSize: "12px", color: "#aaa", marginTop: "1px" }}>
                          {fmtDollars(d.total_amount)} · {d.risk_level}
                          {evalD?.config_id && evalD.config_id !== "manual" && (
                            <span> · {configName(evalD.config_id, autoInvestConfigs)}</span>
                          )}
                        </div>
                      </div>
                      <div style={{ textAlign: "right" }}>
                        {v ? (
                          <>
                            <div style={{ fontSize: "15px", fontWeight: 600, color: v.overall_return_pct >= 0 ? "#27ae60" : "#c0392b" }}>
                              {fmtPct(v.overall_return_pct, true)}
                            </div>
                            <div style={{ fontSize: "11px", marginTop: "2px", color: v.beat_market ? "#27ae60" : "#c0392b" }}>
                              {v.beat_market ? "beat SPY" : "lost to SPY"}
                              {v.spy_return_pct !== 0 && (
                                <span style={{ color: "#bbb" }}> ({fmtPct(v.spy_return_pct, true)})</span>
                              )}
                            </div>
                          </>
                        ) : (
                          <span style={{ fontSize: "12px", color: "#bbb", fontStyle: "italic" }}>Pending</span>
                        )}
                      </div>
                    </div>

                    {/* Ticker pills — only for verdicted decisions */}
                    {v && v.ticker_verdicts?.length > 0 && (
                      <div style={{ marginTop: "8px", display: "flex", flexWrap: "wrap", gap: "6px" }}>
                        {v.ticker_verdicts.map(tv => (
                          <span
                            key={tv.ticker}
                            style={{
                              fontSize: "11px", padding: "2px 8px", borderRadius: "99px",
                              background: tv.return_pct >= 0 ? "#f0faf4" : "#fdf4f4",
                              color: tv.return_pct >= 0 ? "#27ae60" : "#c0392b",
                            }}
                          >
                            {tv.ticker} {fmtPct(tv.return_pct, true)}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </>
      )}
    </div>
  );
}
