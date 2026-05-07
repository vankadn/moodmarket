// src/hooks/usePlaidSetup.ts
import { useState, useEffect } from "react";
import { createLinkToken } from "../services/api";

interface PlaidSetup {
  linkToken: string | null;
  loading: boolean;
  error: string | null;
}

// usePlaidSetup fetches a Plaid Link initialization token on mount.
// The token is short-lived (~30 min) and is used to open the Plaid Link popup.
export function usePlaidSetup(): PlaidSetup {
  const [linkToken, setLinkToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    createLinkToken()
      .then((token) => {
        setLinkToken(token);
        setLoading(false);
      })
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : "Failed to initialize bank connection");
        setLoading(false);
      });
  }, []);

  return { linkToken, loading, error };
}
