import { useEffect, useState } from "react";
import {
  getEvalSummary, getEvalDecisions, getActivity,
  getWinRateTrend, getAssetClassBreakdown,
  EvalSummary, EvalDecision, ActivityDecision, AutoInvestConfig,
  WinRateTrendPoint, AssetClassBreakdownItem,
} from "../services/api";

interface Props {
  onBack: () => void;
  autoInvestConfigs?: AutoInvestConfig[];
}

export function fmtPct(n: number, forceSign = false): string {
  const sign = forceSign && n > 0 ? "+" : "";
  return `${sign}${n.toFixed(2)}%`;
}

export function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

export function fmtDollars(n: number): string {
  return n.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
}

export function configName(configId: string | undefined, configs: AutoInvestConfig[]): string {
  if (configId === undefined || configId === null || configId === "manual") return "Manual";
  if (configId === "") return "Manual";
  const cfg = configs.find(c => c.id === configId);
  return cfg?.name ?? "Deleted strategy";
}

// --- Win Rate Trend chart ---

function WinRateTrendChart({ points }: { points: WinRateTrendPoint[] }) {
  const W = 520, H = 120;
  const PAD = { top: 10, bottom: 28, left: 36, right: 8 };
  const chartW = W - PAD.left - PAD.right;
  const chartH = H - PAD.top - PAD.bottom;

  const toX = (i: number) => PAD.left + (i / (points.length - 1)) * chartW;
  const toY = (rate: number) => PAD.top + chartH - (rate / 100) * chartH;

  const polyPoints = points.map((p, i) => `${toX(i).toFixed(1)},${toY(p.win_rate).toFixed(1)}`).join(" ");

  const weekLabel = (w: string) => {
    const parts = w.split("-W");
    return parts.length === 2 ? `W${parts[1]}` : w;
  };

  return (
    <svg width="100%" viewBox={`0 0 ${W} ${H}`} style={{ display: "block", overflow: "visible" }}>
      {/* Reference lines at 0, 50, 100 */}
      {([0, 50, 100] as const).map(pct => {
        const y = toY(pct);
        return (
          <g key={pct}>
            <line
              x1={PAD.left} y1={y} x2={W - PAD.right} y2={y}
              stroke={pct === 50 ? "#e0e0e0" : "#f4f4f4"}
              strokeWidth="1"
              strokeDasharray={pct === 50 ? "4 3" : undefined}
            />
            <text x={PAD.left - 4} y={y + 4} fontSize="9" fill="#ccc" textAnchor="end">{pct}%</text>
          </g>
        );
      })}

      {/* Data line */}
      <polyline points={polyPoints} fill="none" stroke="#27ae60" strokeWidth="2" strokeLinejoin="round" />

      {/* Data points — colored by win rate */}
      {points.map((p, i) => (
        <circle
          key={p.week}
          cx={toX(i).toFixed(1)}
          cy={toY(p.win_rate).toFixed(1)}
          r={3}
          fill={p.win_rate >= 50 ? "#27ae60" : "#c0392b"}
        />
      ))}

      {/* X axis week labels — skip every other when many points */}
      {points.map((p, i) => {
        if (points.length > 8 && i % 2 !== 0) return null;
        return (
          <text key={p.week} x={toX(i).toFixed(1)} y={H - 4} fontSize="9" fill="#bbb" textAnchor="middle">
            {weekLabel(p.week)}
          </text>
        );
      })}
    </svg>
  );
}

// --- Asset Class Breakdown section ---

function AssetBreakdown({ items }: { items: AssetClassBreakdownItem[] }) {
  return (
    <div style={{ marginBottom: "1.25rem" }}>
      <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", textTransform: "uppercase", letterSpacing: "0.07em", marginBottom: "10px" }}>
        By asset class
      </div>
      {items.map(item => {
        const pct = Math.round(item.win_rate);
        const isGood = pct >= 50;
        return (
          <div key={item.asset_class} style={{ background: "#f8f8f8", borderRadius: "10px", padding: "12px 14px", marginBottom: "8px" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "8px" }}>
              <div>
                <div style={{ fontSize: "14px", fontWeight: 500, textTransform: "capitalize" }}>{item.asset_class}</div>
                <div style={{ fontSize: "12px", color: "#999" }}>
                  {item.total} decision{item.total !== 1 ? "s" : ""} · {item.wins} win{item.wins !== 1 ? "s" : ""}
                </div>
              </div>
              <div style={{ fontSize: "16px", fontWeight: 700, color: isGood ? "#27ae60" : "#c0392b" }}>{pct}%</div>
            </div>
            <div style={{ height: "5px", background: "#e8e8e8", borderRadius: "3px", overflow: "hidden" }}>
              <div style={{ height: "100%", width: `${pct}%`, background: isGood ? "#27ae60" : "#c0392b", borderRadius: "3px" }} />
            </div>
          </div>
        );
      })}
    </div>
  );
}

// --- Skeleton placeholder ---

function Skeleton({ height }: { height: number }) {
  return <div style={{ background: "#f4f4f4", borderRadius: "12px", height: `${height}px`, marginBottom: "1.25rem" }} />;
}

// --- Main page ---

export function Eval({ onBack, autoInvestConfigs = [] }: Props) {
  const [summary, setSummary] = useState<EvalSummary | null>(null);
  const [verdictMap, setVerdictMap] = useState<Map<string, EvalDecision>>(new Map());
  const [allDecisions, setAllDecisions] = useState<ActivityDecision[]>([]);
  const [totalInvested, setTotalInvested] = useState(0);
  const [winRateTrend, setWinRateTrend] = useState<WinRateTrendPoint[]>([]);
  const [assetClassBreakdown, setAssetClassBreakdown] = useState<AssetClassBreakdownItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    Promise.all([
      getEvalSummary().catch((e: unknown) => { throw new Error(`eval/summary: ${e instanceof Error ? e.message : e}`); }),
      getEvalDecisions(1, 500).catch((e: unknown) => { throw new Error(`eval/decisions: ${e instanceof Error ? e.message : e}`); }),
      getActivity(null).catch((e: unknown) => { throw new Error(`activity: ${e instanceof Error ? e.message : e}`); }),
      getWinRateTrend().catch(() => [] as WinRateTrendPoint[]),
      getAssetClassBreakdown().catch(() => [] as AssetClassBreakdownItem[]),
    ])
      .then(([s, evalDecisions, activity, trend, breakdown]) => {
        setSummary(s);
        setTotalInvested(activity.total_invested);
        const map = new Map<string, EvalDecision>();
        for (const d of evalDecisions) map.set(d.id, d);
        setVerdictMap(map);
        setAllDecisions(activity.decisions);
        setWinRateTrend(trend);
        setAssetClassBreakdown(breakdown);
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : "Failed to load data"))
      .finally(() => setLoading(false));
  }, []);

  const winPct = summary ? Math.round(summary.win_rate * 100) : 0;
  const hasVerdicts = (summary?.verdicted_decisions ?? 0) > 0;

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
        <h2 style={{ margin: 0, fontSize: "18px", fontWeight: 600 }}>Performance</h2>
      </div>

      {loading && <p style={{ color: "#999", fontSize: "14px" }}>Loading…</p>}
      {error && <p style={{ color: "#c0392b", fontSize: "14px" }}>{error}</p>}

      {/* Win Rate Trend — always shown after load (placeholder when < 3 weeks) */}
      {loading && <Skeleton height={148} />}
      {!loading && !error && (
        <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1.25rem", marginBottom: "1.25rem" }}>
          <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", textTransform: "uppercase", letterSpacing: "0.07em", marginBottom: "12px" }}>
            Win rate trend · last 12 weeks
          </div>
          {winRateTrend.length >= 3 ? (
            <WinRateTrendChart points={winRateTrend} />
          ) : (
            <div style={{ textAlign: "center", padding: "1.5rem 0", color: "#bbb", fontSize: "13px" }}>
              Not enough data yet
            </div>
          )}
        </div>
      )}

      {/* Asset Class Breakdown — hidden when no data */}
      {loading && <Skeleton height={80} />}
      {!loading && !error && assetClassBreakdown.length > 0 && (
        <AssetBreakdown items={assetClassBreakdown} />
      )}

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
                const isBlocked = d.decision_type === "blocked";
                const isSkip = d.decision_type === "skip";
                return (
                  <div
                    key={d.id}
                    style={{
                      borderBottom: "1px solid #f0f0f0",
                      padding: "12px 0",
                      ...(isBlocked ? { borderLeft: "3px solid #e67e22", paddingLeft: "10px" } : {}),
                      ...(isSkip ? { borderLeft: "3px solid #ddd", paddingLeft: "10px" } : {}),
                    }}
                  >
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                      <div>
                        <div style={{ fontSize: "14px", fontWeight: 500, display: "flex", alignItems: "center", gap: "6px" }}>
                          {fmtDate(d.timestamp)}
                          {isBlocked && (
                            <span style={{ fontSize: "10px", background: "#fef3e8", color: "#e67e22", borderRadius: "4px", padding: "1px 6px", fontWeight: 600, letterSpacing: "0.03em" }}>
                              Blocked
                            </span>
                          )}
                          {isSkip && (
                            <span style={{ fontSize: "10px", background: "#f5f5f5", color: "#999", borderRadius: "4px", padding: "1px 6px", fontWeight: 600, letterSpacing: "0.03em" }}>
                              Skipped
                            </span>
                          )}
                        </div>
                        <div style={{ fontSize: "12px", color: "#aaa", marginTop: "1px" }}>
                          {fmtDollars(d.total_amount)} · {d.risk_level}
                          {evalD?.config_id && evalD.config_id !== "manual" && (
                            <span> · {configName(evalD.config_id, autoInvestConfigs)}</span>
                          )}
                        </div>
                        {!isBlocked && !isSkip && evalD?.overall_reasoning && (
                          <div style={{ fontSize: "12px", color: "#bbb", fontStyle: "italic", marginTop: "3px" }}>
                            {evalD.overall_reasoning}
                          </div>
                        )}
                      </div>
                      <div style={{ textAlign: "right" }}>
                        {isBlocked || isSkip ? null : v ? (
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
                          <span style={{ fontSize: "12px", color: "#bbb", fontStyle: "italic" }}>Too young to rank</span>
                        )}
                      </div>
                    </div>

                    {/* Blocked reason + critic concerns */}
                    {isBlocked && d.blocked_reason && (
                      <div style={{ marginTop: "6px", fontSize: "12px", color: "#e67e22" }}>
                        {d.blocked_reason}
                        {d.critic_review?.verdict === "block" && d.critic_review.concerns.length > 0 && (
                          <ul style={{ margin: "4px 0 0 0", paddingLeft: "16px", color: "#999" }}>
                            {d.critic_review.concerns.map((c, i) => (
                              <li key={i} style={{ fontSize: "11px" }}>{c}</li>
                            ))}
                          </ul>
                        )}
                      </div>
                    )}

                    {/* Ticker pills — only for verdicted decisions */}
                    {v && v.ticker_verdicts?.length > 0 && (
                      <div style={{ marginTop: "8px", display: "flex", flexWrap: "wrap", gap: "6px" }}>
                        {v.ticker_verdicts.map(tv => {
                          const reasoning = evalD?.ticker_reasoning?.[tv.ticker];
                          return (
                            <span
                              key={tv.ticker}
                              title={reasoning || undefined}
                              style={{
                                fontSize: "11px", padding: "2px 8px", borderRadius: "99px",
                                background: tv.return_pct >= 0 ? "#f0faf4" : "#fdf4f4",
                                color: tv.return_pct >= 0 ? "#27ae60" : "#c0392b",
                              }}
                            >
                              {tv.ticker} {fmtPct(tv.return_pct, true)}
                            </span>
                          );
                        })}
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
