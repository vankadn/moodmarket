import { useEffect, useState } from "react";
import { AutoInvestConfig, getAutoInvestConfigs } from "../services/api";

interface Props {
  initialConfigs?: AutoInvestConfig[];
  onConfigsChange?: (configs: AutoInvestConfig[]) => void;
  onBack: () => void;
  onSelectConfig: (config: AutoInvestConfig) => void;
  onAddConfig: () => void;
}

const strategyLabel: Record<string, string> = {
  long_term: "Long Term",
  short_term: "Short Term",
};

function amountLabel(config: AutoInvestConfig): string {
  const days = config.interval_days ?? 1;
  if (days === 1) return `$${config.amount}/day`;
  if (days === 7) return `$${config.amount}/week`;
  return `$${config.amount} every ${days} days`;
}

export function AutoInvestList({ initialConfigs, onConfigsChange, onBack, onSelectConfig, onAddConfig }: Props) {
  const [configs, setConfigs] = useState<AutoInvestConfig[]>(initialConfigs ?? []);
  const [loading, setLoading] = useState(!initialConfigs || initialConfigs.length === 0);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getAutoInvestConfigs()
      .then((fresh) => { setConfigs(fresh); onConfigsChange?.(fresh); })
      .catch(() => setError("Failed to load strategies"))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "2rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Investment strategies</h1>
      </div>

      {loading && (
        <p style={{ color: "#888", fontSize: "14px" }}>Loading…</p>
      )}

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      {!loading && configs.length === 0 && !error && (
        <div style={{ textAlign: "center", padding: "2rem 0", color: "#999", fontSize: "14px" }}>
          No strategies yet. Add one below.
        </div>
      )}

      {configs.map((config) => (
        <button
          key={config.id}
          onClick={() => onSelectConfig(config)}
          style={{
            width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between",
            padding: "14px 16px", background: "#f8f8f8", borderRadius: "12px",
            border: "none", cursor: "pointer", marginBottom: "10px", textAlign: "left",
          }}
        >
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "4px" }}>
              <span style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>
                {config.name || "Unnamed strategy"}
              </span>
              <span style={{
                fontSize: "11px", fontWeight: 600, padding: "2px 8px", borderRadius: "10px",
                background: config.enabled ? "#27ae60" : "#e0e0e0",
                color: config.enabled ? "white" : "#888",
                flexShrink: 0,
              }}>
                {config.enabled ? "Active" : "Off"}
              </span>
            </div>
            <div style={{ fontSize: "12px", color: "#999" }}>
              {amountLabel(config)}
              {config.strategy ? ` · ${strategyLabel[config.strategy]}` : ""}
              {` · ${config.risk.charAt(0).toUpperCase() + config.risk.slice(1)}`}
            </div>
          </div>
          <span style={{ color: "#bbb", fontSize: "16px", marginLeft: "12px" }}>›</span>
        </button>
      ))}

      <button
        onClick={onAddConfig}
        style={{
          width: "100%", padding: "13px",
          background: "#1a1a1a", color: "white",
          border: "none", borderRadius: "10px",
          fontSize: "15px", fontWeight: 500,
          cursor: "pointer", marginTop: "8px",
        }}
      >
        + Add strategy
      </button>
    </div>
  );
}
