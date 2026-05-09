import { useState } from "react";
import { BrokerageStatus, connectBrokerage, disconnectBrokerage } from "../services/api";

interface Props {
  status: BrokerageStatus | undefined;
  onBack: () => void;
  onChanged: () => void; // tells parent to reload profile
}

const BASE_URL_OPTIONS = [
  { label: "Paper trading", value: "https://paper-api.alpaca.markets" },
  { label: "Live trading", value: "https://api.alpaca.markets" },
];

export function BrokerageConnect({ status, onBack, onChanged }: Props) {
  const [apiKey, setApiKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [baseURL, setBaseURL] = useState(BASE_URL_OPTIONS[0].value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const connected = status?.connected === true;
  const accountType = status?.base_url?.includes("paper") ? "Paper" : "Live";

  async function handleConnect() {
    if (!apiKey.trim() || !secretKey.trim()) {
      setError("API key and secret key are required");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await connectBrokerage(apiKey.trim(), secretKey.trim(), baseURL);
      onChanged();
    } catch {
      setError("Failed to connect — check your credentials and try again");
    } finally {
      setSaving(false);
    }
  }

  async function handleDisconnect() {
    setSaving(true);
    setError(null);
    try {
      await disconnectBrokerage();
      onChanged();
    } catch {
      setError("Failed to disconnect");
    } finally {
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
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>Brokerage account</h1>
      </div>

      {connected ? (
        <div>
          <div style={{ background: "#f0faf4", border: "1px solid #c3e6cb", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
            <div style={{ fontSize: "13px", fontWeight: 500, color: "#276749" }}>
              Alpaca connected — {accountType} trading
            </div>
            <div style={{ fontSize: "12px", color: "#555", marginTop: "4px" }}>
              {status?.base_url}
            </div>
          </div>
          {error && (
            <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
              {error}
            </div>
          )}
          <button
            onClick={handleDisconnect}
            disabled={saving}
            style={{
              width: "100%", padding: "13px",
              background: "white", color: "#c0392b",
              border: "1.5px solid #c0392b", borderRadius: "10px",
              fontSize: "15px", fontWeight: 500,
              cursor: saving ? "not-allowed" : "pointer",
              opacity: saving ? 0.6 : 1,
            }}
          >
            {saving ? "Disconnecting…" : "Disconnect Alpaca"}
          </button>
        </div>
      ) : (
        <div>
          <p style={{ fontSize: "13px", color: "#666", marginBottom: "1.5rem", lineHeight: 1.5 }}>
            Connect your Alpaca account to execute real trades. Your API key and secret are encrypted before storage.
          </p>

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

          {error && (
            <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
              {error}
            </div>
          )}

          <button
            onClick={handleConnect}
            disabled={saving}
            style={{
              width: "100%", padding: "13px",
              background: saving ? "#ccc" : "#1a1a1a",
              color: "white", border: "none", borderRadius: "10px",
              fontSize: "15px", fontWeight: 500,
              cursor: saving ? "not-allowed" : "pointer",
            }}
          >
            {saving ? "Connecting…" : "Connect Alpaca"}
          </button>
        </div>
      )}
    </div>
  );
}
