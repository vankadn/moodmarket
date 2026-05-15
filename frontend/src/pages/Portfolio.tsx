import { useEffect, useState } from "react";
import { getPortfolio, getPortfolioHistory, Portfolio as PortfolioData, PortfolioAccount, HistoryPoint, HistoryPeriod } from "../services/api";

const PERIODS: HistoryPeriod[] = ["1D", "5D", "1M", "1Y", "5Y"];

function PortfolioChart({ points, loading }: { points: HistoryPoint[]; loading: boolean }) {
  const W = 520;
  const H = 110;
  const PAD = { top: 8, bottom: 24, left: 4, right: 4 };

  if (loading) {
    return (
      <svg width="100%" viewBox={`0 0 ${W} ${H}`} style={{ display: "block" }}>
        <polyline points={`${PAD.left},${H / 2} ${W - PAD.right},${H / 2}`} fill="none" stroke="#e0e0e0" strokeWidth="2" />
      </svg>
    );
  }

  if (points.length < 2) {
    return <div style={{ height: `${H}px`, display: "flex", alignItems: "center", justifyContent: "center", color: "#ccc", fontSize: "12px" }}>No data for this period</div>;
  }

  const equities = points.map((p) => p.equity);
  const minE = Math.min(...equities);
  const maxE = Math.max(...equities);
  const range = maxE - minE || 1;

  const chartW = W - PAD.left - PAD.right;
  const chartH = H - PAD.top - PAD.bottom;

  const toX = (i: number) => PAD.left + (i / (points.length - 1)) * chartW;
  const toY = (e: number) => PAD.top + chartH - ((e - minE) / range) * chartH;

  const polyPoints = points.map((p, i) => `${toX(i).toFixed(1)},${toY(p.equity).toFixed(1)}`).join(" ");

  const isPositive = points[points.length - 1].equity >= points[0].equity;
  const lineColor = isPositive ? "#27ae60" : "#c0392b";
  const fillColor = isPositive ? "rgba(39,174,96,0.07)" : "rgba(192,57,43,0.07)";

  // Fill area under the line
  const fillPoints = `${PAD.left},${PAD.top + chartH} ${polyPoints} ${toX(points.length - 1).toFixed(1)},${PAD.top + chartH}`;

  // 5 evenly-spaced time labels
  const labelCount = 5;
  const timeLabels = Array.from({ length: labelCount }, (_, i) => {
    const idx = Math.round((i / (labelCount - 1)) * (points.length - 1));
    const ts = points[idx].timestamp * 1000;
    const d = new Date(ts);
    return { x: toX(idx), label: d.toLocaleDateString("en-US", { month: "short", day: "numeric" }) };
  });

  return (
    <svg width="100%" viewBox={`0 0 ${W} ${H}`} style={{ display: "block", overflow: "visible" }}>
      <polygon points={fillPoints} fill={fillColor} />
      <polyline points={polyPoints} fill="none" stroke={lineColor} strokeWidth="1.8" strokeLinejoin="round" />
      {timeLabels.map((l, i) => (
        <text key={i} x={l.x} y={H - 4} textAnchor="middle" fontSize="10" fill="#aaa">{l.label}</text>
      ))}
    </svg>
  );
}

interface Props {
  onBack: () => void;
}

function plColor(value: number): string {
  return value >= 0 ? "#27ae60" : "#c0392b";
}

function formatPL(value: number): string {
  return `${value >= 0 ? "+" : ""}$${Math.abs(value).toFixed(2)}`;
}

function formatPct(value: number): string {
  return `${value >= 0 ? "+" : ""}${value.toFixed(2)}%`;
}

function AccountSection({ account, showHeader }: { account: PortfolioAccount; showHeader: boolean }) {
  return (
    <div style={{ marginBottom: "1.5rem" }}>
      {showHeader && (
        <div style={{ fontSize: "12px", fontWeight: 600, color: "#aaa", letterSpacing: "0.06em", textTransform: "uppercase", marginBottom: "10px", paddingBottom: "6px", borderBottom: "1px solid #f0f0f0" }}>
          {account.brokerage_name || "Account"}
        </div>
      )}
      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "13px" }}>
        <thead>
          <tr style={{ color: "#999", textAlign: "left", borderBottom: "1px solid #f0f0f0" }}>
            <th style={{ padding: "6px 0", fontWeight: 500 }}>Ticker</th>
            <th style={{ padding: "6px 8px", fontWeight: 500 }}>Shares</th>
            <th style={{ padding: "6px 8px", fontWeight: 500 }}>Avg Entry</th>
            <th style={{ padding: "6px 0", fontWeight: 500, textAlign: "right" }}>Value</th>
            <th style={{ padding: "6px 0 6px 12px", fontWeight: 500, textAlign: "right" }}>Gain/Loss</th>
          </tr>
        </thead>
        <tbody>
          {account.positions.map((p) => (
            <tr key={p.ticker} style={{ borderBottom: "1px solid #f8f8f8" }}>
              <td style={{ padding: "10px 0" }}>
                <div>
                  <span style={{ background: "#f0f0f0", padding: "2px 8px", borderRadius: "6px", fontSize: "12px", fontWeight: 500 }}>{p.ticker}</span>
                  {p.name && <div style={{ fontSize: "11px", color: "#999", marginTop: "3px" }}>{p.name}</div>}
                </div>
              </td>
              <td style={{ padding: "10px 8px", color: "#444" }}>{p.quantity.toFixed(4)}</td>
              <td style={{ padding: "10px 8px", color: "#444" }}>${p.avg_entry_price.toFixed(2)}</td>
              <td style={{ padding: "10px 0", textAlign: "right", fontWeight: 500 }}>${p.market_value.toFixed(2)}</td>
              <td style={{ padding: "10px 0 10px 12px", textAlign: "right" }}>
                <div style={{ color: plColor(p.unrealized_pl), fontWeight: 500 }}>{formatPL(p.unrealized_pl)}</div>
                <div style={{ color: plColor(p.unrealized_pl_percent), fontSize: "11px" }}>{formatPct(p.unrealized_pl_percent)}</div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function Portfolio({ onBack }: Props) {
  const [portfolio, setPortfolio] = useState<PortfolioData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [period, setPeriod] = useState<HistoryPeriod>("1M");
  const [historyPoints, setHistoryPoints] = useState<HistoryPoint[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);

  useEffect(() => {
    getPortfolio()
      .then(setPortfolio)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load portfolio"))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    setHistoryLoading(true);
    getPortfolioHistory(period)
      .then((h) => setHistoryPoints(h.points))
      .catch(() => setHistoryPoints([]))
      .finally(() => setHistoryLoading(false));
  }, [period]);

  const multiAccount = (portfolio?.accounts.length ?? 0) > 1;
  const hasPositions = portfolio?.accounts.some((a) => a.positions.length > 0);

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.5rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Portfolio</h1>
      </div>

      {loading && (
        <div style={{ color: "#999", fontSize: "13px", textAlign: "center", padding: "2rem" }}>Loading…</div>
      )}

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px" }}>{error}</div>
      )}

      {portfolio && !loading && (
        <>
          {/* Summary header */}
          <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
            <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
              Total portfolio
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "12px" }}>
              <div>
                <div style={{ fontSize: "11px", color: "#999", marginBottom: "2px" }}>Value</div>
                <div style={{ fontSize: "16px", fontWeight: 600, color: "#111" }}>${portfolio.total_value.toFixed(2)}</div>
              </div>
              <div>
                <div style={{ fontSize: "11px", color: "#999", marginBottom: "2px" }}>Cost</div>
                <div style={{ fontSize: "16px", fontWeight: 600, color: "#111" }}>${portfolio.total_cost.toFixed(2)}</div>
              </div>
              <div>
                <div style={{ fontSize: "11px", color: "#999", marginBottom: "2px" }}>Gain/Loss</div>
                <div style={{ fontSize: "16px", fontWeight: 600, color: plColor(portfolio.total_unrealized_pl) }}>
                  {formatPL(portfolio.total_unrealized_pl)}
                </div>
                <div style={{ fontSize: "11px", color: plColor(portfolio.total_unrealized_pl_percent) }}>
                  {formatPct(portfolio.total_unrealized_pl_percent)}
                </div>
              </div>
            </div>
          </div>

          {/* Chart + period selector */}
          <div style={{ marginBottom: "1.5rem" }}>
            <PortfolioChart points={historyPoints} loading={historyLoading} />
            <div style={{ display: "flex", gap: "4px", marginTop: "10px" }}>
              {PERIODS.map((p) => (
                <button
                  key={p}
                  onClick={() => setPeriod(p)}
                  style={{
                    flex: 1, padding: "5px 0", fontSize: "12px", fontWeight: 500,
                    border: "none", borderRadius: "6px", cursor: "pointer",
                    background: p === period ? "#1a1a1a" : "transparent",
                    color: p === period ? "white" : "#888",
                  }}
                >
                  {p}
                </button>
              ))}
            </div>
          </div>

          {/* Positions */}
          {!hasPositions ? (
            <div style={{ textAlign: "center", color: "#999", fontSize: "13px", padding: "2rem" }}>
              No open positions yet. Invest to get started.
            </div>
          ) : (
            portfolio.accounts.map((account) =>
              account.positions.length > 0 ? (
                <AccountSection key={account.brokerage_id} account={account} showHeader={multiAccount} />
              ) : null
            )
          )}
        </>
      )}

      {!portfolio && !loading && !error && (
        <div style={{ textAlign: "center", color: "#999", fontSize: "13px", padding: "2rem" }}>
          Connect a brokerage account to see your holdings.
        </div>
      )}
    </div>
  );
}
