import { useEffect, useState } from "react";
import {
  AutoInvestConfig, RiskTolerance, StrategyType, UserProfile,
  createAutoInvestConfig, updateAutoInvestConfig, deleteAutoInvestConfig,
  getProfile, saveProfile,
} from "../services/api";

interface Props {
  initialConfig?: AutoInvestConfig;
  onBack: () => void;
}

const riskOptions: { value: RiskTolerance; label: string }[] = [
  { value: "conservative", label: "Conservative" },
  { value: "moderate",     label: "Moderate" },
  { value: "aggressive",   label: "Aggressive" },
];

const frequencyOptions: { days: number; label: string }[] = [
  { days: 1, label: "Daily" },
  { days: 2, label: "Every 2 days" },
  { days: 7, label: "Weekly" },
];

const strategyOptions: { value: StrategyType; label: string; description: string }[] = [
  { value: "long_term",  label: "Long Term",  description: "ETFs only, 10+ year horizon" },
  { value: "short_term", label: "Short Term", description: "ETFs + large-cap stocks, 1-year horizon" },
];

function autoName(strategy: string | undefined, risk: RiskTolerance): string {
  const s = strategy === "long_term" ? "Long Term" : strategy === "short_term" ? "Short Term" : "";
  const r = risk.charAt(0).toUpperCase() + risk.slice(1);
  return s ? `${s} — ${r}` : r;
}

function defaultConfig(): AutoInvestConfig {
  return { enabled: true, amount: 100, risk: "moderate", interval_days: 1, name: autoName("long_term", "moderate"), strategy: "long_term" };
}

export function AutoInvestSettings({ initialConfig, onBack }: Props) {
  const isEdit = !!initialConfig?.id;
  const [config, setConfig] = useState<AutoInvestConfig>(() => initialConfig ?? defaultConfig());
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [includeCashCtx, setIncludeCashCtx] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  useEffect(() => {
    getProfile()
      .then(p => { setProfile(p); setIncludeCashCtx(p.include_cash_context); })
      .catch(() => {});
  }, []);

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      const configSave = isEdit && initialConfig?.id
        ? updateAutoInvestConfig(initialConfig.id, config)
        : createAutoInvestConfig(config);
      const profileSave = profile
        ? saveProfile({ ...profile, include_cash_context: includeCashCtx })
        : Promise.resolve(null);
      await Promise.all([configSave, profileSave]);
      onBack();
    } catch {
      setError("Failed to save settings");
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!initialConfig?.id) return;
    setSaving(true);
    try {
      await deleteAutoInvestConfig(initialConfig.id);
      onBack();
    } catch {
      setError("Failed to delete strategy");
      setSaving(false);
    }
  }

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "2rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>
          {isEdit ? "Edit strategy" : "New strategy"}
        </h1>
      </div>

      {/* Name */}
      <div style={{ marginBottom: "1.25rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Strategy name
        </label>
        <input
          type="text"
          placeholder="e.g. Long Term — Aggressive"
          value={config.name ?? ""}
          onChange={(e) => setConfig({ ...config, name: e.target.value })}
          style={{
            display: "block", width: "100%", marginTop: "8px",
            padding: "10px 12px", border: "1px solid #e0e0e0", borderRadius: "8px",
            fontSize: "14px", outline: "none", boxSizing: "border-box",
          }}
        />
      </div>

      {/* Strategy */}
      <div style={{ marginBottom: "1.25rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Strategy
        </label>
        <div style={{ display: "flex", gap: "8px", marginTop: "8px" }}>
          {strategyOptions.map(({ value, label, description }) => {
            const active = (config.strategy ?? "long_term") === value;
            return (
              <button
                key={value}
                onClick={() => setConfig(prev => ({ ...prev, strategy: value, name: autoName(value, prev.risk) }))}
                style={{
                  flex: 1, padding: "10px 12px", borderRadius: "10px", border: "1.5px solid",
                  borderColor: active ? "#1a1a1a" : "#e0e0e0",
                  background: active ? "#1a1a1a" : "white",
                  color: active ? "white" : "#555",
                  fontSize: "13px", fontWeight: 500, cursor: "pointer", textAlign: "left",
                }}
              >
                <div>{label}</div>
                <div style={{ fontSize: "11px", marginTop: "3px", opacity: 0.7, fontWeight: 400 }}>
                  {description}
                </div>
              </button>
            );
          })}
        </div>
      </div>

      {/* Enable toggle */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "14px 16px", background: "#f8f8f8", borderRadius: "12px", marginBottom: "1.25rem" }}>
        <div>
          <div style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>Enable auto-invest</div>
          <div style={{ fontSize: "12px", color: "#999", marginTop: "2px" }}>Runs automatically on your set schedule</div>
        </div>
        <button
          onClick={() => setConfig({ ...config, enabled: !config.enabled })}
          style={{
            width: "44px", height: "24px", borderRadius: "12px", border: "none",
            background: config.enabled ? "#1a1a1a" : "#d0d0d0",
            cursor: "pointer", position: "relative", transition: "background 0.2s", flexShrink: 0,
          }}
        >
          <span style={{
            position: "absolute", top: "3px",
            left: config.enabled ? "23px" : "3px",
            width: "18px", height: "18px", borderRadius: "50%",
            background: "white", transition: "left 0.2s",
          }} />
        </button>
      </div>

      {/* Cash context opt-in */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "14px 16px", background: "#f8f8f8", borderRadius: "12px", marginBottom: "1.25rem" }}>
        <div>
          <div style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>Include cash balance context</div>
          <div style={{ fontSize: "12px", color: "#999", marginTop: "2px" }}>Shares spending patterns with the advisor</div>
        </div>
        <button
          onClick={() => setIncludeCashCtx(v => !v)}
          style={{
            width: "44px", height: "24px", borderRadius: "12px", border: "none",
            background: includeCashCtx ? "#1a1a1a" : "#d0d0d0",
            cursor: "pointer", position: "relative", transition: "background 0.2s", flexShrink: 0,
          }}
        >
          <span style={{
            position: "absolute", top: "3px",
            left: includeCashCtx ? "23px" : "3px",
            width: "18px", height: "18px", borderRadius: "50%",
            background: "white", transition: "left 0.2s",
          }} />
        </button>
      </div>

      {/* Frequency */}
      <div style={{ marginBottom: "1.25rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Frequency
        </label>
        <div style={{ display: "flex", gap: "8px", marginTop: "8px" }}>
          {frequencyOptions.map(({ days, label }) => {
            const active = (config.interval_days ?? 1) === days;
            return (
              <button
                key={days}
                onClick={() => setConfig({ ...config, interval_days: days })}
                style={{
                  padding: "8px 16px", borderRadius: "20px", border: "1.5px solid",
                  borderColor: active ? "#1a1a1a" : "#e0e0e0",
                  background: active ? "#1a1a1a" : "white",
                  color: active ? "white" : "#555",
                  fontSize: "13px", fontWeight: 500, cursor: "pointer",
                }}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Amount */}
      <div style={{ marginBottom: "1.25rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Investment amount
        </label>
        <div style={{ display: "flex", alignItems: "center", border: "1px solid #e0e0e0", borderRadius: "8px", overflow: "hidden", marginTop: "8px", width: "160px" }}>
          <span style={{ padding: "8px 10px", fontSize: "14px", color: "#888", background: "#f8f8f8", borderRight: "1px solid #e0e0e0" }}>$</span>
          <input
            type="number" min={1} step={10} value={config.amount}
            onChange={(e) => setConfig({ ...config, amount: Math.max(1, Number(e.target.value)) })}
            style={{ width: "100px", padding: "8px 10px", border: "none", outline: "none", fontSize: "14px" }}
          />
        </div>
      </div>

      {/* Risk */}
      <div style={{ marginBottom: "2rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Risk level
        </label>
        <div style={{ display: "flex", gap: "8px", marginTop: "8px" }}>
          {riskOptions.map(({ value, label }) => (
            <button
              key={value}
              onClick={() => setConfig(prev => ({ ...prev, risk: value, name: autoName(prev.strategy, value) }))}
              style={{
                padding: "8px 16px", borderRadius: "20px", border: "1.5px solid",
                borderColor: config.risk === value ? "#1a1a1a" : "#e0e0e0",
                background: config.risk === value ? "#1a1a1a" : "white",
                color: config.risk === value ? "white" : "#555",
                fontSize: "13px", fontWeight: 500, cursor: "pointer",
              }}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      <button
        onClick={handleSave}
        disabled={saving}
        style={{
          width: "100%", padding: "13px",
          background: saving ? "#ccc" : "#1a1a1a",
          color: "white", border: "none", borderRadius: "10px",
          fontSize: "15px", fontWeight: 500,
          cursor: saving ? "not-allowed" : "pointer",
          marginBottom: "1rem",
        }}
      >
        {saving ? "Saving…" : isEdit ? "Save changes" : "Create strategy"}
      </button>

      {isEdit && !confirmDelete && (
        <button
          onClick={() => setConfirmDelete(true)}
          style={{
            width: "100%", padding: "13px",
            background: "white", color: "#c0392b",
            border: "1.5px solid #f5c6cb", borderRadius: "10px",
            fontSize: "15px", fontWeight: 500, cursor: "pointer",
          }}
        >
          Delete strategy
        </button>
      )}

      {confirmDelete && (
        <div style={{ padding: "14px 16px", background: "#fdf0ee", borderRadius: "10px", border: "1px solid #f5c6cb" }}>
          <p style={{ margin: "0 0 12px", fontSize: "14px", color: "#c0392b", fontWeight: 500 }}>
            Delete this strategy? This cannot be undone.
          </p>
          <div style={{ display: "flex", gap: "8px" }}>
            <button
              onClick={handleDelete}
              disabled={saving}
              style={{
                flex: 1, padding: "10px",
                background: "#c0392b", color: "white",
                border: "none", borderRadius: "8px",
                fontSize: "14px", fontWeight: 500, cursor: saving ? "not-allowed" : "pointer",
              }}
            >
              Yes, delete
            </button>
            <button
              onClick={() => setConfirmDelete(false)}
              style={{
                flex: 1, padding: "10px",
                background: "white", color: "#555",
                border: "1.5px solid #e0e0e0", borderRadius: "8px",
                fontSize: "14px", fontWeight: 500, cursor: "pointer",
              }}
            >
              Cancel
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
