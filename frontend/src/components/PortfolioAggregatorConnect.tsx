import { useState } from "react";
import { PortfolioConnectionStatus, connectPortfolioAggregator, disconnectPortfolioAggregator } from "../services/api";

interface Props {
  status: PortfolioConnectionStatus | undefined;
  onBack: () => void;
  onChanged: () => void;
}

const SUPPORTED_BROKERS = [
  "Robinhood",
  "Fidelity",
  "Charles Schwab",
  "TD Ameritrade",
  "Interactive Brokers",
];

export function PortfolioAggregatorConnect({ status, onBack, onChanged }: Props) {
  const [connecting, setConnecting] = useState(false);
  const [disconnecting, setDisconnecting] = useState(false);
  const [confirmDisconnect, setConfirmDisconnect] = useState(false);
  const [portalOpened, setPortalOpened] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isConnected = status?.connected === true;

  async function handleConnect() {
    setConnecting(true);
    setError(null);
    setPortalOpened(false);
    try {
      const { redirect_url } = await connectPortfolioAggregator();
      window.open(redirect_url, "_blank", "noopener,noreferrer");
      setPortalOpened(true);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to start connection");
    } finally {
      setConnecting(false);
    }
  }

  async function handleDisconnect() {
    setDisconnecting(true);
    setError(null);
    try {
      await disconnectPortfolioAggregator();
      setConfirmDisconnect(false);
      onChanged();
    } catch {
      setError("Failed to disconnect");
    } finally {
      setDisconnecting(false);
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
        <h1 style={{ fontSize: "20px", fontWeight: 600, margin: 0 }}>External accounts</h1>
      </div>

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      {isConnected ? (
        <>
          {/* Connected card */}
          <div style={{ background: "#f0faf4", border: "1px solid #c3e6cb", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "2rem" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
              <div>
                <div style={{ fontSize: "14px", fontWeight: 600, color: "#1a1a1a", marginBottom: "4px" }}>
                  SnapTrade connected
                </div>
                {status?.connected_at && (
                  <div style={{ fontSize: "12px", color: "#555" }}>
                    Since {new Date(status.connected_at).toLocaleDateString()}
                  </div>
                )}
                <div style={{ fontSize: "12px", color: "#555", marginTop: "4px" }}>
                  Holdings from linked brokers are merged into Claude's context
                </div>
              </div>
              <span style={{ fontSize: "18px" }}>✓</span>
            </div>
          </div>

          {/* Manage brokers */}
          <div style={{ marginBottom: "2rem" }}>
            <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
              Manage linked brokers
            </div>
            <p style={{ fontSize: "13px", color: "#666", marginBottom: "1rem", lineHeight: 1.5 }}>
              Add or remove brokerages (Robinhood, Fidelity, Schwab, etc.) via the SnapTrade portal.
              The portal opens in a new tab — return here when done.
            </p>
            {portalOpened && (
              <div style={{ fontSize: "13px", color: "#27ae60", padding: "10px", background: "#f0faf4", borderRadius: "8px", marginBottom: "1rem" }}>
                SnapTrade portal opened in a new tab — pick your broker there, then return here.
              </div>
            )}
            <button
              onClick={handleConnect}
              disabled={connecting}
              style={{
                width: "100%", padding: "12px",
                background: "white", color: "#1a1a1a",
                border: "1.5px solid #1a1a1a", borderRadius: "10px",
                fontSize: "14px", fontWeight: 500,
                cursor: connecting ? "not-allowed" : "pointer",
                opacity: connecting ? 0.6 : 1,
              }}
            >
              {connecting ? "Opening portal…" : "Add or manage brokers ↗"}
            </button>
          </div>

          {/* Disconnect */}
          <div>
            <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
              Disconnect
            </div>
            {confirmDisconnect ? (
              <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
                <span style={{ fontSize: "13px", color: "#555" }}>Remove all linked brokers and disconnect?</span>
                <button
                  onClick={handleDisconnect}
                  disabled={disconnecting}
                  style={{
                    padding: "6px 14px", borderRadius: "8px", border: "1.5px solid #c0392b",
                    background: "white", color: "#c0392b", fontSize: "13px", fontWeight: 500,
                    cursor: disconnecting ? "not-allowed" : "pointer",
                    opacity: disconnecting ? 0.6 : 1,
                  }}
                >
                  {disconnecting ? "…" : "Yes, disconnect"}
                </button>
                <button
                  onClick={() => setConfirmDisconnect(false)}
                  style={{ padding: "6px 14px", borderRadius: "8px", border: "1.5px solid #e0e0e0", background: "white", color: "#555", fontSize: "13px", cursor: "pointer" }}
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                onClick={() => setConfirmDisconnect(true)}
                style={{
                  padding: "8px 16px", borderRadius: "8px",
                  border: "1.5px solid #e0e0e0", background: "white",
                  color: "#c0392b", fontSize: "13px", fontWeight: 500,
                  cursor: "pointer",
                }}
              >
                Disconnect SnapTrade
              </button>
            )}
          </div>
        </>
      ) : (
        <>
          {/* Explainer */}
          <p style={{ fontSize: "14px", color: "#444", lineHeight: 1.6, marginBottom: "1.5rem" }}>
            Connect Robinhood, Fidelity, Schwab, and other brokerages for <strong>read-only</strong> portfolio
            view. Holdings are merged into Claude's recommendation context alongside your Alpaca positions.
          </p>

          {/* Supported brokers */}
          <div style={{ marginBottom: "2rem" }}>
            <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
              Supported brokers (via SnapTrade)
            </div>
            <div style={{ display: "flex", gap: "8px", flexWrap: "wrap" }}>
              {SUPPORTED_BROKERS.map((b) => (
                <span
                  key={b}
                  style={{
                    fontSize: "12px", padding: "5px 12px", borderRadius: "20px",
                    border: "1px solid #e0e0e0", background: "#f8f8f8", color: "#555",
                  }}
                >
                  {b}
                </span>
              ))}
              <span style={{ fontSize: "12px", padding: "5px 12px", borderRadius: "20px", border: "1px solid #e0e0e0", background: "#f8f8f8", color: "#999" }}>
                + many more
              </span>
            </div>
          </div>

          {/* Portal opened confirmation */}
          {portalOpened && (
            <div style={{ fontSize: "13px", color: "#27ae60", padding: "10px", background: "#f0faf4", borderRadius: "8px", marginBottom: "1rem" }}>
              SnapTrade portal opened in a new tab — pick your broker there, then return here.
            </div>
          )}

          {/* Connect button */}
          <button
            onClick={handleConnect}
            disabled={connecting}
            style={{
              width: "100%", padding: "13px",
              background: connecting ? "#ccc" : "#1a1a1a",
              color: "white", border: "none", borderRadius: "10px",
              fontSize: "15px", fontWeight: 500,
              cursor: connecting ? "not-allowed" : "pointer",
            }}
          >
            {connecting ? "Opening portal…" : "Connect external accounts"}
          </button>

          <p style={{ fontSize: "12px", color: "#999", marginTop: "10px", lineHeight: 1.5 }}>
            Credentials are never stored by InvestIQ — SnapTrade handles broker authentication.
            InvestIQ only reads holdings.
          </p>
        </>
      )}
    </div>
  );
}
