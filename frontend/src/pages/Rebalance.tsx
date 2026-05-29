import { useState } from "react";
import { analyzePortfolio, PositionInsight, RebalanceAnalysis, SuggestedAction, TaxFlag } from "../services/api";

interface Props {
  onBack: () => void;
}

const ACTION_LABEL: Record<SuggestedAction, string> = {
  hold: "Hold",
  add: "Add more",
  trim: "Trim",
  reconsider: "Reconsider",
};

const ACTION_COLOR: Record<SuggestedAction, string> = {
  hold: "#888",
  add: "#27ae60",
  trim: "#e67e22",
  reconsider: "#c0392b",
};

const ACTION_BG: Record<SuggestedAction, string> = {
  hold: "#f4f4f4",
  add: "#eafaf1",
  trim: "#fef5ea",
  reconsider: "#fdf0ee",
};

const TAX_LABEL: Record<TaxFlag, string> = {
  short_term: "Short-term gains if sold",
  long_term: "Long-term gains if sold",
  unknown: "Tax treatment unknown",
};

const TAX_COLOR: Record<TaxFlag, string> = {
  short_term: "#e67e22",
  long_term: "#27ae60",
  unknown: "#aaa",
};

function fmtDollars(n: number): string {
  return n.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
}

function fmtPct(n: number, forceSign = false): string {
  const sign = forceSign && n > 0 ? "+" : "";
  return `${sign}${n.toFixed(2)}%`;
}

function SourceLabel({ source, accountName }: { source: string; accountName: string }) {
  if (source === "alpaca") {
    return <span style={{ fontSize: "11px", color: "#888" }}>Alpaca (InvestIQ-managed)</span>;
  }
  const label = accountName || "External account";
  return (
    <span style={{ fontSize: "11px", color: "#c0841a" }}>
      {label} (SnapTrade) — act in your brokerage app
    </span>
  );
}

function InsightCard({ insight }: { insight: PositionInsight }) {
  const [expanded, setExpanded] = useState(false);
  const action = insight.suggested_action;
  const plPositive = insight.unrealized_pl >= 0;

  return (
    <div
      style={{
        border: "1px solid #eee",
        borderRadius: "12px",
        padding: "1rem 1.25rem",
        marginBottom: "10px",
        background: "white",
      }}
    >
      {/* Top row: ticker + action badge */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "4px" }}>
        <div>
          <span style={{ fontSize: "16px", fontWeight: 600, color: "#1a1a1a" }}>{insight.ticker}</span>
          {insight.name && (
            <span style={{ fontSize: "13px", color: "#888", marginLeft: "8px" }}>{insight.name}</span>
          )}
        </div>
        <span
          style={{
            fontSize: "12px",
            fontWeight: 600,
            color: ACTION_COLOR[action],
            background: ACTION_BG[action],
            borderRadius: "6px",
            padding: "3px 10px",
            whiteSpace: "nowrap",
          }}
        >
          {ACTION_LABEL[action]}
        </span>
      </div>

      {/* Source */}
      <div style={{ marginBottom: "8px" }}>
        <SourceLabel source={insight.source} accountName={insight.account_name} />
      </div>

      {/* Value + P&L row */}
      <div style={{ display: "flex", gap: "20px", marginBottom: "8px" }}>
        <div>
          <div style={{ fontSize: "11px", color: "#aaa", marginBottom: "1px" }}>Value</div>
          <div style={{ fontSize: "14px", fontWeight: 500 }}>{fmtDollars(insight.current_value)}</div>
        </div>
        {insight.unrealized_pl !== 0 && (
          <div>
            <div style={{ fontSize: "11px", color: "#aaa", marginBottom: "1px" }}>Unrealized P&amp;L</div>
            <div style={{ fontSize: "14px", fontWeight: 500, color: plPositive ? "#27ae60" : "#c0392b" }}>
              {plPositive ? "+" : ""}{fmtDollars(insight.unrealized_pl)} ({fmtPct(insight.unrealized_pl_pct, true)})
            </div>
          </div>
        )}
      </div>

      {/* Tax flag */}
      <div style={{ fontSize: "11px", color: TAX_COLOR[insight.tax_flag], marginBottom: "10px" }}>
        ⚑ {TAX_LABEL[insight.tax_flag]}
      </div>

      {/* Claude assessment */}
      <div style={{ fontSize: "13px", color: "#333", lineHeight: "1.5", marginBottom: insight.original_buy_thesis ? "8px" : 0 }}>
        {insight.claude_assessment}
      </div>

      {/* Original buy thesis — collapsible */}
      {insight.original_buy_thesis && (
        <>
          <button
            onClick={() => setExpanded(e => !e)}
            style={{
              background: "none", border: "none", padding: 0, cursor: "pointer",
              fontSize: "11px", color: "#aaa", marginTop: "4px",
            }}
          >
            {expanded ? "Hide original thesis ▲" : "Show original thesis ▼"}
          </button>
          {expanded && (
            <div style={{ fontSize: "12px", color: "#888", marginTop: "6px", fontStyle: "italic", lineHeight: "1.5" }}>
              "{insight.original_buy_thesis}"
            </div>
          )}
        </>
      )}
    </div>
  );
}

function SummaryBar({ analysis }: { analysis: RebalanceAnalysis }) {
  const counts: Record<SuggestedAction, number> = { hold: 0, add: 0, trim: 0, reconsider: 0 };
  for (const i of analysis.insights) counts[i.suggested_action]++;

  return (
    <div style={{ display: "flex", gap: "10px", flexWrap: "wrap", marginBottom: "12px" }}>
      {(["reconsider", "trim", "add", "hold"] as SuggestedAction[]).filter(a => counts[a] > 0).map(a => (
        <span key={a} style={{ fontSize: "12px", fontWeight: 600, color: ACTION_COLOR[a], background: ACTION_BG[a], borderRadius: "6px", padding: "3px 10px" }}>
          {counts[a]} {ACTION_LABEL[a]}
        </span>
      ))}
    </div>
  );
}

export function Rebalance({ onBack }: Props) {
  const [status, setStatus] = useState<"idle" | "loading" | "done" | "error">("idle");
  const [analysis, setAnalysis] = useState<RebalanceAnalysis | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function handleAnalyze() {
    setStatus("loading");
    setError(null);
    try {
      const result = await analyzePortfolio();
      setAnalysis(result);
      setStatus("done");
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "Something went wrong";
      setError(
        msg === "advisor_overloaded"
          ? "Claude is temporarily busy. Please try again in a moment."
          : "Failed to analyze portfolio. Please try again."
      );
      setStatus("error");
    }
  }

  const generatedAt = analysis
    ? new Date(analysis.generated_at).toLocaleString("en-US", { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" })
    : null;

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.5rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#888", fontSize: "20px", cursor: "pointer", padding: "0 4px", lineHeight: 1 }}
        >
          ←
        </button>
        <div>
          <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Rebalance</h1>
          <p style={{ fontSize: "13px", color: "#888", margin: "2px 0 0" }}>Portfolio review by Claude</p>
        </div>
      </div>

      {/* Permanent disclaimer */}
      <div style={{
        background: "#fffbea",
        border: "1px solid #f0d060",
        borderRadius: "10px",
        padding: "10px 14px",
        fontSize: "12px",
        color: "#7a6000",
        marginBottom: "1.25rem",
        lineHeight: "1.5",
      }}>
        <strong>Suggestions only.</strong> InvestIQ never executes sells. Execute any changes yourself in your brokerage app.
      </div>

      {/* Analyze button */}
      {status !== "done" && (
        <button
          onClick={handleAnalyze}
          disabled={status === "loading"}
          style={{
            width: "100%", padding: "12px",
            background: status === "loading" ? "#ccc" : "#1a1a1a",
            color: "white", border: "none", borderRadius: "10px",
            fontSize: "14px", fontWeight: 500,
            cursor: status === "loading" ? "not-allowed" : "pointer",
            marginBottom: "1.25rem",
          }}
        >
          {status === "loading" ? "Analyzing… (this may take up to 30s)" : "Analyze portfolio"}
        </button>
      )}

      {/* Error */}
      {status === "error" && error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px 12px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      {/* Results */}
      {status === "done" && analysis && (
        <>
          {/* Portfolio health summary */}
          <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1rem" }}>
            <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "6px" }}>
              Portfolio summary
            </div>
            <p style={{ margin: 0, fontSize: "14px", color: "#222", lineHeight: "1.5" }}>
              {analysis.portfolio_health_summary}
            </p>
          </div>

          {/* Action count badges */}
          <SummaryBar analysis={analysis} />

          {/* Re-analyze button */}
          <button
            onClick={handleAnalyze}
            style={{
              width: "100%", padding: "10px",
              background: "white",
              color: "#555", border: "1px solid #ddd", borderRadius: "10px",
              fontSize: "13px", cursor: "pointer",
              marginBottom: "1.25rem",
            }}
          >
            Re-analyze
          </button>

          {/* Generated at */}
          {generatedAt && (
            <div style={{ fontSize: "11px", color: "#bbb", marginBottom: "1rem" }}>
              Generated {generatedAt}
            </div>
          )}

          {/* Insight cards */}
          {analysis.insights.length === 0 ? (
            <p style={{ fontSize: "14px", color: "#888", textAlign: "center", padding: "2rem 0" }}>
              No positions found. Connect a brokerage account to get started.
            </p>
          ) : (
            analysis.insights.map((ins) => (
              <InsightCard key={ins.ticker} insight={ins} />
            ))
          )}
        </>
      )}
    </div>
  );
}
