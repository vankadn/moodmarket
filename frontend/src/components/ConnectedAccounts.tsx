// src/components/ConnectedAccounts.tsx
import { useState } from "react";
import { deletePlaidAccount, PlaidConnectionSummary } from "../services/api";

interface Props {
  connections: PlaidConnectionSummary[];
  onDisconnect: (itemId: string) => void;
}

export function ConnectedAccounts({ connections, onDisconnect }: Props) {
  const [disconnecting, setDisconnecting] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  if (connections.length === 0) {
    return (
      <p style={{ fontSize: "13px", color: "#999", margin: "4px 0 0" }}>
        No bank accounts connected yet.
      </p>
    );
  }

  async function handleDisconnect(itemId: string) {
    setDisconnecting(itemId);
    setError(null);
    try {
      await deletePlaidAccount(itemId);
      onDisconnect(itemId);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Failed to disconnect account");
    } finally {
      setDisconnecting(null);
    }
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
      {connections.map((c) => (
        <div
          key={c.item_id}
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            padding: "10px 14px",
            background: "white",
            border: "1px solid #e8e8e8",
            borderRadius: "8px",
          }}
        >
          <div style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>{c.institution}</div>
          <button
            onClick={() => handleDisconnect(c.item_id)}
            disabled={disconnecting === c.item_id}
            style={{
              background: "none",
              border: "1px solid #e0e0e0",
              borderRadius: "6px",
              padding: "4px 12px",
              fontSize: "12px",
              color: disconnecting === c.item_id ? "#bbb" : "#c0392b",
              cursor: disconnecting === c.item_id ? "not-allowed" : "pointer",
            }}
          >
            {disconnecting === c.item_id ? "Disconnecting…" : "Disconnect"}
          </button>
        </div>
      ))}
      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px" }}>{error}</div>
      )}
    </div>
  );
}
