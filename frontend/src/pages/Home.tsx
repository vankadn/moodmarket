import { useState } from "react";
import { RecommendationCard } from "../components/RecommendationCard";
import { getRecommendation, Recommendation, UserProfile } from "../services/api";

interface Props {
  profile: UserProfile;
}

const goalLabel: Record<string, string> = {
  wealth_building: "Wealth building",
  retirement: "Retirement",
  emergency_fund: "Emergency fund",
  short_term_savings: "Short-term savings",
};

const horizonLabel: Record<string, string> = {
  under_1_year: "Under 1 year",
  one_to_five: "1–5 years",
  five_to_ten: "5–10 years",
  ten_plus: "10+ years",
};

export function Home({ profile }: Props) {
  const [extra, setExtra] = useState<number>(0);
  const [rec, setRec] = useState<Recommendation | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const total = 100 + extra;

  async function handleInvest() {
    setLoading(true);
    setError(null);
    try {
      const result = await getRecommendation({ base_budget: 100, extra_money: extra });
      setRec(result);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  const profileSummary = [
    { label: "Name",    value: profile.full_name },
    { label: "Goal",    value: goalLabel[profile.investment_goal] ?? profile.investment_goal },
    { label: "Risk",    value: profile.risk_tolerance.charAt(0).toUpperCase() + profile.risk_tolerance.slice(1) },
    { label: "Horizon", value: horizonLabel[profile.time_horizon] ?? profile.time_horizon },
  ];

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ marginBottom: "1.5rem" }}>
        <h1 style={{ fontSize: "22px", fontWeight: 600, margin: "0 0 4px" }}>InvestIQ</h1>
        <p style={{ fontSize: "14px", color: "#666", margin: 0 }}>Daily investment advisor</p>
      </div>

      {/* Profile summary card */}
      <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
          Your profile
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px 16px" }}>
          {profileSummary.map(({ label, value }) => (
            <div key={label}>
              <div style={{ fontSize: "11px", color: "#999", marginBottom: "2px" }}>{label}</div>
              <div style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>{value}</div>
            </div>
          ))}
        </div>
      </div>

      {/* Extra money input */}
      <div style={{ marginBottom: "1.5rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Extra money today (optional)
        </label>
        <div style={{ display: "flex", alignItems: "center", gap: "10px", marginTop: "8px" }}>
          <div style={{ display: "flex", alignItems: "center", border: "1px solid #e0e0e0", borderRadius: "8px", overflow: "hidden" }}>
            <span style={{ padding: "8px 10px", fontSize: "14px", color: "#888", background: "#f8f8f8", borderRight: "1px solid #e0e0e0" }}>$</span>
            <input
              type="number" min={0} step={10} value={extra || ""}
              onChange={(e) => setExtra(Math.max(0, Number(e.target.value)))}
              placeholder="0"
              style={{ width: "100px", padding: "8px 10px", border: "none", outline: "none", fontSize: "14px" }}
            />
          </div>
          <span style={{ fontSize: "13px", color: "#888" }}>
            = <strong style={{ color: "#333" }}>${total} total</strong>
          </span>
        </div>
      </div>

      <button
        onClick={handleInvest}
        disabled={loading}
        style={{
          width: "100%", padding: "13px",
          background: loading ? "#ccc" : "#1a1a1a",
          color: "white", border: "none", borderRadius: "10px",
          fontSize: "15px", fontWeight: 500,
          cursor: loading ? "not-allowed" : "pointer",
          marginBottom: "1.5rem",
        }}
      >
        {loading ? "Generating recommendation…" : `Invest $${total} today`}
      </button>

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}
      {rec && <RecommendationCard rec={rec} />}
    </div>
  );
}
