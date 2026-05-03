import { useState } from "react";
import {
  saveProfile,
  UserProfile,
  TimeHorizon,
  ImmigrationStatus,
  RiskTolerance,
  InvestmentGoal,
} from "../services/api";

interface Props {
  onComplete: (profile: UserProfile) => void;
}

const input: React.CSSProperties = {
  width: "100%",
  padding: "10px 12px",
  border: "1px solid #e0e0e0",
  borderRadius: "8px",
  fontSize: "14px",
  outline: "none",
  boxSizing: "border-box",
  background: "white",
};

const label: React.CSSProperties = {
  display: "block",
  fontSize: "13px",
  fontWeight: 500,
  color: "#555",
  marginBottom: "6px",
};

const field: React.CSSProperties = { marginBottom: "1.25rem" };

const defaultProfile: UserProfile = {
  full_name: "",
  salary: 0,
  monthly_savings: 0,
  retirement_contribution_percent: 0,
  existing_portfolio_value: 0,
  time_horizon: "five_to_ten",
  immigration_status: "us_citizen",
  risk_tolerance: "moderate",
  investment_goal: "wealth_building",
  has_emergency_fund: false,
};

export function Onboarding({ onComplete }: Props) {
  const [form, setForm] = useState<UserProfile>(defaultProfile);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function set<K extends keyof UserProfile>(key: K, value: UserProfile[K]) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const saved = await saveProfile(form);
      onComplete(saved);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to save profile");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ maxWidth: "480px", margin: "0 auto", padding: "2.5rem 1.5rem" }}>
      <div style={{ marginBottom: "2rem" }}>
        <h1 style={{ fontSize: "22px", fontWeight: 600, margin: "0 0 6px" }}>InvestIQ</h1>
        <p style={{ fontSize: "14px", color: "#666", margin: 0 }}>
          Set up your financial profile once. We'll handle the rest.
        </p>
      </div>

      <form onSubmit={handleSubmit}>
        <div style={field}>
          <label style={label}>Full name</label>
          <input
            style={input} type="text" required placeholder="Jane Smith"
            value={form.full_name}
            onChange={(e) => set("full_name", e.target.value)}
          />
        </div>

        <div style={field}>
          <label style={label}>Annual salary ($)</label>
          <input
            style={input} type="number" min={0} step={1000} required placeholder="85000"
            value={form.salary || ""}
            onChange={(e) => set("salary", Number(e.target.value))}
          />
        </div>

        <div style={field}>
          <label style={label}>Monthly savings ($)</label>
          <input
            style={input} type="number" min={0} step={100} required placeholder="1500"
            value={form.monthly_savings || ""}
            onChange={(e) => set("monthly_savings", Number(e.target.value))}
          />
        </div>

        <div style={field}>
          <label style={label}>Retirement contribution (%)</label>
          <input
            style={input} type="number" min={0} max={100} step={1} required placeholder="6"
            value={form.retirement_contribution_percent || ""}
            onChange={(e) => set("retirement_contribution_percent", Number(e.target.value))}
          />
        </div>

        <div style={field}>
          <label style={label}>Existing portfolio value ($)</label>
          <input
            style={input} type="number" min={0} step={1000} required placeholder="25000"
            value={form.existing_portfolio_value || ""}
            onChange={(e) => set("existing_portfolio_value", Number(e.target.value))}
          />
        </div>

        <div style={field}>
          <label style={label}>Time horizon</label>
          <select style={input} value={form.time_horizon} onChange={(e) => set("time_horizon", e.target.value as TimeHorizon)}>
            <option value="under_1_year">Under 1 year</option>
            <option value="one_to_five">1–5 years</option>
            <option value="five_to_ten">5–10 years</option>
            <option value="ten_plus">10+ years</option>
          </select>
        </div>

        <div style={field}>
          <label style={label}>Immigration status</label>
          <select style={input} value={form.immigration_status} onChange={(e) => set("immigration_status", e.target.value as ImmigrationStatus)}>
            <option value="us_citizen">US Citizen</option>
            <option value="permanent_resident">Permanent Resident</option>
            <option value="work_visa">Work Visa</option>
            <option value="other">Other</option>
          </select>
        </div>

        <div style={field}>
          <label style={label}>Risk tolerance</label>
          <select style={input} value={form.risk_tolerance} onChange={(e) => set("risk_tolerance", e.target.value as RiskTolerance)}>
            <option value="conservative">Conservative</option>
            <option value="moderate">Moderate</option>
            <option value="aggressive">Aggressive</option>
          </select>
        </div>

        <div style={field}>
          <label style={label}>Primary investment goal</label>
          <select style={input} value={form.investment_goal} onChange={(e) => set("investment_goal", e.target.value as InvestmentGoal)}>
            <option value="wealth_building">Wealth building</option>
            <option value="retirement">Retirement</option>
            <option value="emergency_fund">Emergency fund</option>
            <option value="short_term_savings">Short-term savings</option>
          </select>
        </div>

        <div style={{ ...field, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <label style={{ ...label, marginBottom: 0 }}>I have an emergency fund</label>
          <div
            role="switch"
            aria-checked={form.has_emergency_fund}
            onClick={() => set("has_emergency_fund", !form.has_emergency_fund)}
            style={{
              width: "44px", height: "24px",
              background: form.has_emergency_fund ? "#1a1a1a" : "#d0d0d0",
              borderRadius: "12px", cursor: "pointer", position: "relative",
              transition: "background 0.2s", flexShrink: 0,
            }}
          >
            <div style={{
              width: "18px", height: "18px", background: "white", borderRadius: "50%",
              position: "absolute", top: "3px",
              left: form.has_emergency_fund ? "23px" : "3px",
              transition: "left 0.2s",
            }} />
          </div>
        </div>

        {error && (
          <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={loading}
          style={{
            width: "100%", padding: "13px",
            background: loading ? "#ccc" : "#1a1a1a",
            color: "white", border: "none", borderRadius: "10px",
            fontSize: "15px", fontWeight: 500,
            cursor: loading ? "not-allowed" : "pointer",
          }}
        >
          {loading ? "Saving…" : "Continue"}
        </button>
      </form>
    </div>
  );
}
