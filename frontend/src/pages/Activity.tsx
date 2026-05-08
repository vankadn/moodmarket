import { useEffect, useState } from "react";
import { getActivity, ActivitySummary } from "../services/api";

interface Props {
  onBack: () => void;
}

interface RangeInputs {
  hours: string;
  days: string;
  months: string;
}

const DEFAULT: RangeInputs = { hours: "", days: "30", months: "" };

function sinceFromInputs(inputs: RangeInputs): Date | null {
  const h = parseFloat(inputs.hours) || 0;
  const d = parseInt(inputs.days) || 0;
  const m = parseInt(inputs.months) || 0;
  if (h === 0 && d === 0 && m === 0) return null;
  const ms = (h * 60 + d * 24 * 60 + m * 30 * 24 * 60) * 60 * 1000;
  return new Date(Date.now() - ms);
}

function formatDate(ts: string) {
  return new Date(ts).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" });
}

function formatDollars(n: number) {
  return n.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });
}

const inputStyle: React.CSSProperties = {
  width: "64px", padding: "7px 10px", border: "1px solid #e0e0e0",
  borderRadius: "8px", fontSize: "14px", outline: "none", textAlign: "center",
};

const unitLabel: React.CSSProperties = {
  fontSize: "12px", color: "#888", marginTop: "4px", textAlign: "center",
};

export function Activity({ onBack }: Props) {
  const [inputs, setInputs] = useState<RangeInputs>(DEFAULT);
  const [summary, setSummary] = useState<ActivitySummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  function set(key: keyof RangeInputs, value: string) {
    const sanitized = key === "hours"
      ? value.replace(/[^\d.]/g, "").replace(/^(\d*\.?\d*).*$/, "$1") // allow one decimal point
      : value.replace(/\D/g, "");
    setInputs((prev) => ({ ...prev, [key]: sanitized }));
  }

  function reset() {
    setInputs(DEFAULT);
  }

  const isDefault =
    inputs.hours === DEFAULT.hours &&
    inputs.days === DEFAULT.days &&
    inputs.months === DEFAULT.months;

  useEffect(() => {
    const since = sinceFromInputs(inputs);
    setLoading(true);
    setError(null);
    getActivity(since)
      .then(setSummary)
      .catch(() => setError("Failed to load activity"))
      .finally(() => setLoading(false));
  }, [inputs.hours, inputs.days, inputs.months]);

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "2rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Activity</h1>
      </div>

      {/* Range inputs */}
      <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "12px" }}>
          Time range
        </div>
        <div style={{ display: "flex", alignItems: "flex-start", gap: "12px" }}>
          <div>
            <input
              style={inputStyle}
              type="text"
              inputMode="numeric"
              placeholder="0"
              value={inputs.months}
              onChange={(e) => set("months", e.target.value)}
            />
            <div style={unitLabel}>months</div>
          </div>
          <div style={{ paddingTop: "9px", color: "#ccc", fontSize: "16px" }}>+</div>
          <div>
            <input
              style={inputStyle}
              type="text"
              inputMode="numeric"
              placeholder="30"
              value={inputs.days}
              onChange={(e) => set("days", e.target.value)}
            />
            <div style={unitLabel}>days</div>
          </div>
          <div style={{ paddingTop: "9px", color: "#ccc", fontSize: "16px" }}>+</div>
          <div>
            <input
              style={inputStyle}
              type="text"
              inputMode="numeric"
              placeholder="0.5"
              value={inputs.hours}
              onChange={(e) => set("hours", e.target.value)}
            />
            <div style={unitLabel}>hours</div>
          </div>
          {!isDefault && (
            <button
              onClick={reset}
              style={{
                marginTop: "4px", padding: "7px 12px", background: "white",
                border: "1px solid #e0e0e0", borderRadius: "8px",
                fontSize: "12px", color: "#888", cursor: "pointer",
              }}
            >
              Reset
            </button>
          )}
        </div>
      </div>

      {loading && <p style={{ color: "#888", fontSize: "14px" }}>Loading…</p>}
      {error && <p style={{ color: "#c0392b", fontSize: "14px" }}>{error}</p>}

      {summary && !loading && (
        <>
          {/* Stats */}
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px", marginBottom: "1.5rem" }}>
            <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem" }}>
              <div style={{ fontSize: "11px", color: "#aaa", fontWeight: 600, letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "6px" }}>
                Total invested
              </div>
              <div style={{ fontSize: "22px", fontWeight: 600, color: "#111" }}>
                {formatDollars(summary.total_invested)}
              </div>
            </div>
            <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem" }}>
              <div style={{ fontSize: "11px", color: "#aaa", fontWeight: 600, letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "6px" }}>
                Decisions
              </div>
              <div style={{ fontSize: "22px", fontWeight: 600, color: "#111" }}>
                {summary.total_decisions}
              </div>
            </div>
          </div>

          {/* Timeline */}
          {summary.decisions.length === 0 ? (
            <p style={{ color: "#999", fontSize: "14px", textAlign: "center", padding: "2rem 0" }}>
              No investments in this period.
            </p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
              {summary.decisions.map((d) => (
                <div
                  key={d.id}
                  style={{
                    display: "flex", justifyContent: "space-between", alignItems: "center",
                    padding: "12px 14px", background: "#f8f8f8", borderRadius: "10px",
                  }}
                >
                  <div>
                    <div style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>
                      {formatDate(d.timestamp)}
                    </div>
                    <div style={{ fontSize: "11px", color: "#999", marginTop: "2px", textTransform: "capitalize" }}>
                      {d.risk_level} risk
                    </div>
                  </div>
                  <div style={{ fontSize: "15px", fontWeight: 600, color: "#111" }}>
                    {formatDollars(d.total_amount)}
                  </div>
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
