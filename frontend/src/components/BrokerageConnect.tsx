import { useState } from "react";
import {
  AssetCategory,
  BrokerageStatus,
  addBrokerageConnection,
  removeBrokerageConnection,
} from "../services/api";

interface Props {
  connections: BrokerageStatus[];
  onBack: () => void;
  onChanged: () => void;
}

const BASE_URL_OPTIONS = [
  { label: "Paper trading", value: "https://paper-api.alpaca.markets" },
  { label: "Live trading", value: "https://api.alpaca.markets" },
];

const CATEGORY_OPTIONS: { label: string; value: AssetCategory }[] = [
  { label: "Stocks & ETFs", value: "equity" },
  { label: "Bonds & Fixed Income", value: "bond" },
  { label: "Default fallback", value: "default" },
];

const BROKERS = [
  { id: "alpaca", label: "Alpaca Markets", available: true },
  { id: "fidelity", label: "Fidelity Investments", available: false },
  { id: "robinhood", label: "Robinhood", available: false },
  { id: "schwab", label: "Charles Schwab", available: false },
  { id: "etrade", label: "E*TRADE", available: false },
];

function categoryLabel(cat: AssetCategory): string {
  return CATEGORY_OPTIONS.find((o) => o.value === cat)?.label ?? cat;
}

export function BrokerageConnect({ connections, onBack, onChanged }: Props) {
  const [selectedBroker, setSelectedBroker] = useState<string>("");
  const [name, setName] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [baseURL, setBaseURL] = useState(BASE_URL_OPTIONS[0].value);
  const [selectedCats, setSelectedCats] = useState<AssetCategory[]>([]);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);
  const [confirmRemove, setConfirmRemove] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const brokerIsReady = selectedBroker === "alpaca";

  function toggleCat(cat: AssetCategory) {
    setSelectedCats((prev) =>
      prev.includes(cat) ? prev.filter((c) => c !== cat) : [...prev, cat]
    );
  }

  function resetForm() {
    setSelectedBroker("");
    setName("");
    setApiKey("");
    setSecretKey("");
    setSelectedCats([]);
    setBaseURL(BASE_URL_OPTIONS[0].value);
  }

  async function handleAdd() {
    if (!apiKey.trim() || !secretKey.trim()) {
      setError("API key and secret key are required");
      return;
    }
    if (selectedCats.length === 0) {
      setError("Select at least one asset category");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await addBrokerageConnection({
        name: name.trim() || "Alpaca account",
        asset_categories: selectedCats,
        api_key: apiKey.trim(),
        secret_key: secretKey.trim(),
        base_url: baseURL,
      });
      resetForm();
      onChanged();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to add connection");
    } finally {
      setSaving(false);
    }
  }

  async function handleRemove(id: string) {
    setRemoving(id);
    setError(null);
    try {
      await removeBrokerageConnection(id);
      setConfirmRemove(null);
      onChanged();
    } catch {
      setError("Failed to remove connection");
    } finally {
      setRemoving(null);
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
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Brokerage accounts</h1>
      </div>

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      {/* Connected accounts list */}
      {connections.length > 0 && (
        <div style={{ marginBottom: "2rem" }}>
          <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
            Connected accounts
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
            {connections.map((conn) => (
              <div
                key={conn.id}
                style={{ background: "#f0faf4", border: "1px solid #c3e6cb", borderRadius: "12px", padding: "1rem 1.25rem" }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                  <div>
                    <div style={{ fontSize: "14px", fontWeight: 600, color: "#1a1a1a", marginBottom: "4px" }}>
                      {conn.name || "Alpaca"}
                    </div>
                    <div style={{ fontSize: "12px", color: "#555", marginBottom: "6px" }}>
                      {conn.base_url?.includes("paper") ? "Paper trading" : "Live trading"}
                    </div>
                    <div style={{ display: "flex", gap: "6px", flexWrap: "wrap" }}>
                      {(conn.asset_categories ?? []).map((cat) => (
                        <span
                          key={cat}
                          style={{
                            fontSize: "11px", fontWeight: 500,
                            padding: "3px 8px", borderRadius: "12px",
                            background: cat === "bond" ? "#e8f4fd" : cat === "default" ? "#f3f0ff" : "#fff3cd",
                            color: cat === "bond" ? "#1a6baa" : cat === "default" ? "#5f3dc4" : "#856404",
                            border: "1px solid",
                            borderColor: cat === "bond" ? "#bee3f8" : cat === "default" ? "#c5b4e3" : "#ffeaa7",
                          }}
                        >
                          {categoryLabel(cat)}
                        </span>
                      ))}
                    </div>
                  </div>
                  <div>
                    {confirmRemove === conn.id ? (
                      <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
                        <span style={{ fontSize: "12px", color: "#888" }}>Remove?</span>
                        <button
                          onClick={() => handleRemove(conn.id)}
                          disabled={removing === conn.id}
                          style={{
                            padding: "4px 10px", borderRadius: "6px", border: "1.5px solid #c0392b",
                            background: "white", color: "#c0392b", fontSize: "12px", fontWeight: 500,
                            cursor: removing === conn.id ? "not-allowed" : "pointer",
                            opacity: removing === conn.id ? 0.6 : 1,
                          }}
                        >
                          {removing === conn.id ? "…" : "Yes"}
                        </button>
                        <button
                          onClick={() => setConfirmRemove(null)}
                          style={{ padding: "4px 10px", borderRadius: "6px", border: "1.5px solid #e0e0e0", background: "white", color: "#555", fontSize: "12px", cursor: "pointer" }}
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setConfirmRemove(conn.id)}
                        style={{
                          padding: "5px 12px", borderRadius: "8px",
                          border: "1.5px solid #e0e0e0", background: "white",
                          color: "#c0392b", fontSize: "12px", fontWeight: 500,
                          cursor: "pointer",
                        }}
                      >
                        Remove
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Add new connection */}
      <div>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "14px" }}>
          Add account
        </div>

        {/* Broker selector */}
        <div style={{ marginBottom: "1.5rem" }}>
          <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
            Broker
          </label>
          <select
            value={selectedBroker}
            onChange={(e) => { setSelectedBroker(e.target.value); setError(null); }}
            style={{
              display: "block", width: "100%", marginTop: "6px",
              padding: "10px 12px", border: "1px solid #e0e0e0", borderRadius: "8px",
              fontSize: "14px", background: "white", outline: "none", color: selectedBroker ? "#1a1a1a" : "#aaa",
            }}
          >
            <option value="" disabled>Select a broker…</option>
            {BROKERS.map((b) => (
              <option key={b.id} value={b.id} disabled={!b.available}>
                {b.label}{!b.available ? " (not ready)" : ""}
              </option>
            ))}
          </select>
          {selectedBroker && !brokerIsReady && (
            <p style={{ fontSize: "12px", color: "#c0392b", marginTop: "6px" }}>
              {BROKERS.find(b => b.id === selectedBroker)?.label} integration is coming soon — only Alpaca Markets is available today.
            </p>
          )}
        </div>

        {/* Credential form — only shown when an available broker is selected */}
        {brokerIsReady && (
          <>
            <p style={{ fontSize: "13px", color: "#666", marginBottom: "1.5rem", lineHeight: 1.5 }}>
              Enter your <strong>Alpaca Markets</strong> API credentials. Keys are encrypted before storage and never logged.
              Get them at <span style={{ color: "#1a6baa" }}>alpaca.markets → Your account → API keys</span>.
            </p>

            <div style={{ marginBottom: "1.25rem" }}>
              <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
                Account name
              </label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Main account, Bonds account"
                style={{ display: "block", width: "100%", padding: "10px 12px", marginTop: "6px", border: "1px solid #e0e0e0", borderRadius: "8px", fontSize: "14px", outline: "none", boxSizing: "border-box" }}
              />
            </div>

            <div style={{ marginBottom: "1.25rem" }}>
              <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
                Route these asset types here
              </label>
              <div style={{ display: "flex", gap: "8px", flexWrap: "wrap", marginTop: "8px" }}>
                {CATEGORY_OPTIONS.map((opt) => {
                  const selected = selectedCats.includes(opt.value);
                  return (
                    <button
                      key={opt.value}
                      onClick={() => toggleCat(opt.value)}
                      style={{
                        padding: "7px 14px", borderRadius: "20px", border: "1.5px solid",
                        borderColor: selected ? "#1a1a1a" : "#e0e0e0",
                        background: selected ? "#1a1a1a" : "white",
                        color: selected ? "white" : "#555",
                        fontSize: "13px", fontWeight: 500, cursor: "pointer",
                      }}
                    >
                      {opt.label}
                    </button>
                  );
                })}
              </div>
            </div>

            <div style={{ marginBottom: "1.25rem" }}>
              <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
                API Key
              </label>
              <input
                type="text"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                placeholder="PKxxxxxxxxxxxxxxxxxxxxxxxx"
                style={{ display: "block", width: "100%", padding: "10px 12px", marginTop: "6px", border: "1px solid #e0e0e0", borderRadius: "8px", fontSize: "14px", outline: "none", boxSizing: "border-box" }}
              />
            </div>

            <div style={{ marginBottom: "1.25rem" }}>
              <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
                Secret Key
              </label>
              <input
                type="password"
                value={secretKey}
                onChange={(e) => setSecretKey(e.target.value)}
                placeholder="••••••••••••••••••••••••"
                style={{ display: "block", width: "100%", padding: "10px 12px", marginTop: "6px", border: "1px solid #e0e0e0", borderRadius: "8px", fontSize: "14px", outline: "none", boxSizing: "border-box" }}
              />
            </div>

            <div style={{ marginBottom: "2rem" }}>
              <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
                Account type
              </label>
              <div style={{ display: "flex", gap: "8px", marginTop: "8px" }}>
                {BASE_URL_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    onClick={() => setBaseURL(opt.value)}
                    style={{
                      padding: "8px 16px", borderRadius: "20px", border: "1.5px solid",
                      borderColor: baseURL === opt.value ? "#1a1a1a" : "#e0e0e0",
                      background: baseURL === opt.value ? "#1a1a1a" : "white",
                      color: baseURL === opt.value ? "white" : "#555",
                      fontSize: "13px", fontWeight: 500, cursor: "pointer",
                    }}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>

            <button
              onClick={handleAdd}
              disabled={saving}
              style={{
                width: "100%", padding: "13px",
                background: saving ? "#ccc" : "#1a1a1a",
                color: "white", border: "none", borderRadius: "10px",
                fontSize: "15px", fontWeight: 500,
                cursor: saving ? "not-allowed" : "pointer",
              }}
            >
              {saving ? "Adding…" : "Add Alpaca account"}
            </button>
          </>
        )}
      </div>
    </div>
  );
}
