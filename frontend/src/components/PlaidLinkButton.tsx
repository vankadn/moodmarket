// src/components/PlaidLinkButton.tsx
import { useState } from "react";
import { usePlaidLink } from "react-plaid-link";
import { exchangePublicToken } from "../services/api";

interface Props {
  linkToken: string;
  onConnected: () => void;
}

export function PlaidLinkButton({ linkToken, onConnected }: Props) {
  const [exchanging, setExchanging] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { open, ready } = usePlaidLink({
    token: linkToken,
    onSuccess: async (publicToken: string) => {
      setExchanging(true);
      setError(null);
      try {
        await exchangePublicToken(publicToken);
        onConnected();
      } catch (e: unknown) {
        setError(e instanceof Error ? e.message : "Failed to connect account");
      } finally {
        setExchanging(false);
      }
    },
  });

  const disabled = !ready || exchanging;

  return (
    <div>
      <button
        onClick={() => open()}
        disabled={disabled}
        style={{
          width: "100%",
          padding: "12px",
          background: disabled ? "#ccc" : "#1a1a1a",
          color: "white",
          border: "none",
          borderRadius: "10px",
          fontSize: "14px",
          fontWeight: 500,
          cursor: disabled ? "not-allowed" : "pointer",
        }}
      >
        {exchanging ? "Connecting…" : "Connect Bank Account"}
      </button>
      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", marginTop: "8px" }}>{error}</div>
      )}
    </div>
  );
}
