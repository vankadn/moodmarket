import { useEffect, useState } from "react";
import { AutoInvestConfig, RiskTolerance, UserProfile, getAutoInvestConfig, saveAutoInvestConfig, getProfile, saveProfile } from "../services/api";

interface Props {
  onBack: () => void;
}

const riskOptions: { value: RiskTolerance; label: string }[] = [
  { value: "conservative", label: "Conservative" },
  { value: "moderate",     label: "Moderate" },
  { value: "aggressive",   label: "Aggressive" },
];

export function AutoInvestSettings({ onBack }: Props) {
  const [config, setConfig] = useState<AutoInvestConfig | null>(null);
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [includeCashCtx, setIncludeCashCtx] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    getAutoInvestConfig().then(setConfig).catch(() => setError("Failed to load settings"));
    getProfile().then(p => { setProfile(p); setIncludeCashCtx(p.include_cash_context); }).catch(() => {});
  }, []);

  async function handleSave() {
    if (!config) return;
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const [updated] = await Promise.all([
        saveAutoInvestConfig(config),
        profile ? saveProfile({ ...profile, include_cash_context: includeCashCtx }) : Promise.resolve(null),
      ]);
      setConfig(updated);
      setSaved(true);
    } catch {
      setError("Failed to save settings");
    } finally {
      setSaving(false);
    }
  }

  if (!config) {
    return (
      <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
        <p style={{ color: "#888", fontSize: "14px" }}>{error ?? "Loading…"}</p>
      </div>
    );
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
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Auto-invest settings</h1>
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

      {/* Amount */}
      <div style={{ marginBottom: "1.25rem" }}>
        <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Daily investment amount
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
              onClick={() => setConfig({ ...config, risk: value })}
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
      {saved && (
        <div style={{ color: "#27ae60", fontSize: "13px", padding: "10px", background: "#f0fdf4", borderRadius: "8px", marginBottom: "1rem" }}>
          Settings saved
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
        }}
      >
        {saving ? "Saving…" : "Save settings"}
      </button>
    </div>
  );
}
