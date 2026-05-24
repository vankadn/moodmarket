// src/pages/AllocationPreferencesPage.tsx
import { useEffect, useState } from "react";
import { getProfile, saveProfile, AssetClassLimit } from "../services/api";

interface Props {
  onBack: () => void;
  onSaved: () => void;
}

const ASSET_CLASSES = ["US Equity", "Crypto", "Bonds", "International", "Real Estate"];

const labelStyle = {
  fontSize: "11px", fontWeight: 600, color: "#aaa",
  letterSpacing: "0.07em", textTransform: "uppercase" as const,
  display: "block", marginBottom: "6px",
};

const numInput = {
  width: "64px", padding: "7px 10px", border: "1px solid #e0e0e0",
  borderRadius: "7px", fontSize: "14px", outline: "none",
  textAlign: "center" as const, background: "white",
};

export function AllocationPreferencesPage({ onBack, onSaved }: Props) {
  // limits keyed by asset class name
  const [limits, setLimits] = useState<Record<string, { min: string; max: string }>>(() =>
    Object.fromEntries(ASSET_CLASSES.map((c) => [c, { min: "", max: "" }]))
  );
  const [maxTicker, setMaxTicker] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getProfile()
      .then((p) => {
        if (p.allocation_preferences) {
          const next = { ...limits };
          for (const l of p.allocation_preferences.asset_class_limits ?? []) {
            if (ASSET_CLASSES.includes(l.asset_class)) {
              next[l.asset_class] = {
                min: l.min_pct ? String(l.min_pct) : "",
                max: l.max_pct ? String(l.max_pct) : "",
              };
            }
          }
          setLimits(next);
          if (p.allocation_preferences.max_single_ticker_pct) {
            setMaxTicker(String(p.allocation_preferences.max_single_ticker_pct));
          }
        }
      })
      .catch(() => setError("Failed to load preferences"))
      .finally(() => setLoading(false));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      const profile = await getProfile();

      const assetClassLimits: AssetClassLimit[] = [];
      for (const cls of ASSET_CLASSES) {
        const { min, max } = limits[cls];
        const minPct = parseFloat(min);
        const maxPct = parseFloat(max);
        if ((min !== "" && !isNaN(minPct) && minPct > 0) || (max !== "" && !isNaN(maxPct) && maxPct > 0)) {
          assetClassLimits.push({
            asset_class: cls,
            ...(min !== "" && !isNaN(minPct) && minPct > 0 ? { min_pct: minPct } : {}),
            ...(max !== "" && !isNaN(maxPct) && maxPct > 0 ? { max_pct: maxPct } : {}),
          });
        }
      }

      const maxTickerPct = parseFloat(maxTicker);
      await saveProfile({
        ...profile,
        allocation_preferences: {
          asset_class_limits: assetClassLimits,
          ...(maxTicker !== "" && !isNaN(maxTickerPct) && maxTickerPct > 0
            ? { max_single_ticker_pct: maxTickerPct }
            : {}),
        },
      });
      onSaved();
    } catch {
      setError("Failed to save — please try again");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.5rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#888", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "18px", fontWeight: 600, margin: 0, color: "#111" }}>Allocation limits</h1>
      </div>

      {loading ? (
        <div style={{ fontSize: "13px", color: "#888" }}>Loading…</div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
          <p style={{ fontSize: "13px", color: "#666", margin: "0 0 4px" }}>
            Set hard limits on how much Claude can allocate to each asset class.
            Leave a field blank to apply no constraint. Claude will never exceed your maximums
            or drop below your minimums regardless of market conditions.
          </p>

          {/* Asset class rows */}
          <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1.25rem", display: "flex", flexDirection: "column", gap: "1rem" }}>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 80px 80px", gap: "8px", alignItems: "center" }}>
              <span style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase" }}>Asset class</span>
              <span style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", textAlign: "center" }}>Min %</span>
              <span style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", textAlign: "center" }}>Max %</span>
            </div>

            {ASSET_CLASSES.map((cls) => (
              <div key={cls} style={{ display: "grid", gridTemplateColumns: "1fr 80px 80px", gap: "8px", alignItems: "center" }}>
                <span style={{ fontSize: "14px", color: "#222", fontWeight: 500 }}>{cls}</span>
                <input
                  type="number"
                  min={0}
                  max={100}
                  step={5}
                  placeholder="—"
                  value={limits[cls].min}
                  onChange={(e: { target: { value: string } }) => setLimits((prev: Record<string, { min: string; max: string }>) => ({ ...prev, [cls]: { ...prev[cls], min: e.target.value } }))}
                  style={numInput}
                />
                <input
                  type="number"
                  min={0}
                  max={100}
                  step={5}
                  placeholder="—"
                  value={limits[cls].max}
                  onChange={(e: { target: { value: string } }) => setLimits((prev: Record<string, { min: string; max: string }>) => ({ ...prev, [cls]: { ...prev[cls], max: e.target.value } }))}
                  style={numInput}
                />
              </div>
            ))}
          </div>

          {/* Single ticker cap */}
          <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1.25rem" }}>
            <label style={labelStyle}>Max single ticker (%)</label>
            <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
              <input
                type="number"
                min={0}
                max={100}
                step={5}
                placeholder="—"
                value={maxTicker}
                onChange={(e: { target: { value: string } }) => setMaxTicker(e.target.value)}
                style={{ ...numInput, width: "80px" }}
              />
              <span style={{ fontSize: "13px", color: "#888" }}>
                Claude won't put more than this in any one ticker
              </span>
            </div>
          </div>

          {error && (
            <div style={{ fontSize: "13px", color: "#c0392b", background: "#fdf0ee", padding: "8px 12px", borderRadius: "6px" }}>
              {error}
            </div>
          )}

          <button
            onClick={handleSave}
            disabled={saving}
            style={{
              padding: "10px 16px", background: saving ? "#ccc" : "#1a1a1a",
              color: "white", border: "none", borderRadius: "8px",
              fontSize: "14px", fontWeight: 500, cursor: saving ? "not-allowed" : "pointer",
            }}
          >
            {saving ? "Saving…" : "Save limits"}
          </button>
        </div>
      )}
    </div>
  );
}
