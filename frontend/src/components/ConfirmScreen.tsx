import { BrokerageStatus, Recommendation } from "../services/api";

const riskColor: Record<string, string> = {
  low: "#E6F1FB", medium: "#FAEEDA", high: "#FAECE7",
};

interface Props {
  rec: Recommendation;
  brokerages: BrokerageStatus[];
  perAllocBrokerage: Record<string, string>;
  onPerAllocChange: (ticker: string, connID: string) => void;
  onConfirm: () => void;
  onCancel: () => void;
  loading: boolean;
}

export function ConfirmScreen({ rec, brokerages, perAllocBrokerage, onPerAllocChange, onConfirm, onCancel, loading }: Props) {
  const showVia = brokerages.length > 0;
  const multiConn = brokerages.length > 1;
  const colSpanTotal = showVia ? 3 : 2;
  const hasInvalidAllocation = (rec.allocations ?? []).some((a) => a.amount < 1.00);

  return (
    <div style={{ background: "white", border: "1px solid #e0e0e0", borderRadius: "12px", padding: "1.25rem 1.5rem" }}>
      {rec.from_cache && (
        <div style={{ marginBottom: "1rem", padding: "8px 12px", background: "#fffbea", border: "1px solid #f0d060", borderRadius: "8px", fontSize: "12px", color: "#7a6000" }}>
          AI advisor is temporarily unavailable. Showing your last recommendation — amounts scaled to today's budget.
        </div>
      )}

      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.25rem" }}>
        <div>
          <div style={{ fontWeight: 600, fontSize: "15px", color: "#111" }}>Confirm investment</div>
          <div style={{ fontSize: "13px", color: "#666", marginTop: "2px" }}>{rec.summary}</div>
        </div>
        <span style={{ background: riskColor[rec.risk_level], padding: "4px 12px", borderRadius: "20px", fontSize: "12px", fontWeight: 500 }}>
          {rec.risk_level} risk
        </span>
      </div>

      <table style={{ width: "100%", borderCollapse: "collapse", marginBottom: "1.25rem", fontSize: "13px" }}>
        <thead>
          <tr style={{ borderBottom: "1px solid #f0f0f0", color: "#999", textAlign: "left" }}>
            <th style={{ padding: "6px 0", fontWeight: 500 }}>Ticker</th>
            <th style={{ padding: "6px 0", fontWeight: 500 }}>Name</th>
            {showVia && <th style={{ padding: "6px 8px", fontWeight: 500 }}>Via</th>}
            <th style={{ padding: "6px 0", fontWeight: 500, textAlign: "right" }}>Amount</th>
            <th style={{ padding: "6px 0", fontWeight: 500, textAlign: "right" }}>%</th>
          </tr>
        </thead>
        <tbody>
          {(rec.allocations ?? []).map((a) => (
            <tr key={a.ticker} style={{ borderBottom: "1px solid #f8f8f8" }}>
              <td style={{ padding: "10px 0" }}>
                <span style={{ background: "#f0f0f0", padding: "2px 8px", borderRadius: "6px", fontSize: "12px", fontWeight: 500 }}>
                  {a.ticker}
                </span>
              </td>
              <td style={{ padding: "10px 8px", color: "#444" }}>{a.name}</td>
              {showVia && (
                <td style={{ padding: "10px 8px" }}>
                  <select
                    value={perAllocBrokerage[a.ticker] ?? (multiConn ? "" : brokerages[0].id)}
                    onChange={(e) => onPerAllocChange(a.ticker, e.target.value)}
                    disabled={loading || !multiConn}
                    style={{
                      fontSize: "12px", border: "1px solid #e0e0e0", borderRadius: "6px",
                      padding: "3px 6px", background: "white", outline: "none",
                      cursor: (loading || !multiConn) ? "default" : "pointer", maxWidth: "130px",
                    }}
                  >
                    {multiConn && <option value="">Auto</option>}
                    {brokerages.map((b) => (
                      <option key={b.id} value={b.id}>
                        {b.name || "Alpaca"}
                      </option>
                    ))}
                  </select>
                </td>
              )}
              <td style={{ padding: "10px 0", textAlign: "right", fontWeight: 500 }}>${a.amount.toFixed(2)}</td>
              <td style={{ padding: "10px 0", textAlign: "right", color: "#888" }}>{a.percentage.toFixed(0)}%</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          <tr>
            <td colSpan={colSpanTotal} style={{ padding: "10px 0", fontWeight: 600 }}>Total</td>
            <td style={{ padding: "10px 0", textAlign: "right", fontWeight: 600 }}>${rec.total_budget.toFixed(2)}</td>
            <td />
          </tr>
        </tfoot>
      </table>

      {hasInvalidAllocation && (
        <div style={{ fontSize: "13px", color: "#d97706", marginBottom: "12px" }}>
          One or more allocations is under $1.00 — reduce tickers or increase your investment amount
        </div>
      )}

      <div style={{ display: "flex", gap: "10px" }}>
        <button
          onClick={onConfirm}
          disabled={loading || brokerages.length === 0 || hasInvalidAllocation}
          style={{
            flex: 1, padding: "12px",
            background: (loading || brokerages.length === 0 || hasInvalidAllocation) ? "#ccc" : "#1a1a1a",
            color: "white", border: "none", borderRadius: "8px",
            fontSize: "14px", fontWeight: 500, cursor: (loading || brokerages.length === 0 || hasInvalidAllocation) ? "not-allowed" : "pointer",
          }}
        >
          {loading ? "Placing orders…" : brokerages.length === 0 ? "Connect a brokerage to invest" : "Confirm & Invest"}
        </button>
        <button
          onClick={onCancel}
          disabled={loading}
          style={{
            padding: "12px 20px",
            background: "transparent", color: "#666",
            border: "1px solid #ddd", borderRadius: "8px",
            fontSize: "14px", cursor: loading ? "not-allowed" : "pointer",
          }}
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
