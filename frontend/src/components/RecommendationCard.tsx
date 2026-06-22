import { Recommendation } from "../services/api";

interface Props { rec: Recommendation }

const riskColor: Record<string, string> = {
  low: "#E6F1FB", medium: "#FAEEDA", high: "#FAECE7",
};

export function RecommendationCard({ rec }: Props) {
  return (
    <div style={{ background: "white", border: "1px solid #e0e0e0", borderRadius: "12px", padding: "1.25rem 1.5rem" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "1.25rem" }}>
        <div>
          <div style={{ fontWeight: 500, fontSize: "20px" }}>${rec.total_budget.toFixed(2)}</div>
          <div style={{ fontSize: "13px", color: "#666", marginTop: "4px" }}>{rec.summary}</div>
        </div>
        <span style={{ background: riskColor[rec.risk_level], padding: "4px 12px", borderRadius: "20px", fontSize: "12px", fontWeight: 500 }}>
          {rec.risk_level} risk
        </span>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
        {(rec.allocations ?? []).map((a) => (
          <div key={a.ticker}>
            <div style={{ display: "flex", alignItems: "center", gap: "10px", marginBottom: "4px" }}>
              <span style={{ background: "#f0f0f0", padding: "2px 8px", borderRadius: "6px", fontSize: "12px", fontWeight: 500, minWidth: "44px", textAlign: "center" }}>
                {a.ticker}
              </span>
              <span style={{ fontSize: "13px", color: "#333", flex: 1 }}>{a.name}</span>
              <span style={{ fontSize: "14px", fontWeight: 500 }}>${a.amount.toFixed(2)}</span>
              <span style={{ fontSize: "12px", color: "#888", minWidth: "36px", textAlign: "right" }}>{a.percentage.toFixed(0)}%</span>
            </div>
            <div style={{ height: "4px", background: "#f0f0f0", borderRadius: "2px", marginLeft: "54px" }}>
              <div style={{ width: `${Math.min(a.percentage, 100)}%`, height: "100%", background: "#1D9E75", borderRadius: "2px" }} />
            </div>
            <div style={{ fontSize: "12px", color: "#999", marginTop: "4px", marginLeft: "54px" }}>{a.rationale}</div>
          </div>
        ))}
      </div>
    </div>
  );
}
