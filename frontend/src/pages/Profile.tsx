// src/pages/Profile.tsx
import { useState } from "react";
import { UserProfile, PlaidConnectionSummary } from "../services/api";
import { usePlaidSetup } from "../hooks/usePlaidSetup";
import { PlaidLinkButton } from "../components/PlaidLinkButton";
import { ConnectedAccounts } from "../components/ConnectedAccounts";

interface Props {
  profile: UserProfile;
  onBack: () => void;
  onAccountsChanged: () => void; // tells the parent to refresh the profile
}

export function Profile({ profile, onBack, onAccountsChanged }: Props) {
  const { linkToken, loading: linkLoading, error: linkError } = usePlaidSetup();

  // Initialise from the profile prop so the list updates immediately on disconnect
  // without waiting for a full profile refetch.
  const [connections, setConnections] = useState<PlaidConnectionSummary[]>(
    profile.connected_accounts ?? []
  );

  function handleConnected() {
    onAccountsChanged(); // parent will reload profile with the new account included
  }

  function handleDisconnect(itemId: string) {
    setConnections((prev) => prev.filter((c) => c.item_id !== itemId));
    onAccountsChanged();
  }

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: "12px", marginBottom: "1.5rem" }}>
        <button
          onClick={onBack}
          style={{ background: "none", border: "none", color: "#888", fontSize: "13px", cursor: "pointer", padding: 0 }}
        >
          ← Back
        </button>
        <h1 style={{ fontSize: "18px", fontWeight: 600, margin: 0, color: "#111" }}>Bank Accounts</h1>
      </div>

      {/* Connected accounts list */}
      <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
          Connected accounts
        </div>
        <ConnectedAccounts connections={connections} onDisconnect={handleDisconnect} />
      </div>

      {/* Add new account */}
      <div>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
          Add account
        </div>
        {linkError && (
          <div style={{ color: "#c0392b", fontSize: "13px", marginBottom: "8px" }}>{linkError}</div>
        )}
        {linkLoading ? (
          <div style={{ fontSize: "13px", color: "#888" }}>Initializing bank connection…</div>
        ) : linkToken ? (
          <PlaidLinkButton linkToken={linkToken} onConnected={handleConnected} />
        ) : null}
      </div>
    </div>
  );
}
