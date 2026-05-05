import { useState } from "react";

const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:8080";
const DEV_MODE = import.meta.env.VITE_DEV_MODE === "true";

interface LoginProps {
  onAuthenticated: () => void;
}

export function Login({ onAuthenticated }: LoginProps) {
  const [email, setEmail] = useState("");
  const [error, setError] = useState("");

  async function handleDevLogin() {
    setError("");
    try {
      const res = await fetch(`${API_BASE}/auth/dev-login`);
      if (!res.ok) throw new Error(`server error ${res.status}`);
      const { token } = await res.json();
      localStorage.setItem("auth_token", token);
      onAuthenticated();
    } catch (e) {
      setError("Dev login failed — is the backend running?");
    }
  }

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        height: "100vh",
        background: "#0a0a0a",
      }}
    >
      <div
        style={{
          background: "#111",
          border: "1px solid #222",
          borderRadius: "12px",
          padding: "40px",
          width: "360px",
          display: "flex",
          flexDirection: "column",
          gap: "16px",
        }}
      >
        <div>
          <h1 style={{ color: "#fff", fontSize: "22px", margin: "0 0 4px" }}>
            InvestIQ
          </h1>
          <p style={{ color: "#555", fontSize: "13px", margin: 0 }}>
            Sign in to your account
          </p>
        </div>

        <input
          type="email"
          placeholder="Email address"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          style={{
            background: "#1a1a1a",
            border: "1px solid #333",
            borderRadius: "8px",
            padding: "12px",
            color: "#fff",
            fontSize: "14px",
            outline: "none",
            width: "100%",
            boxSizing: "border-box",
          }}
        />

        <button
          onClick={() => alert("Google OAuth — coming in Phase 4")}
          style={{
            background: "#fff",
            color: "#000",
            border: "none",
            borderRadius: "8px",
            padding: "12px",
            fontSize: "14px",
            fontWeight: 600,
            cursor: "pointer",
            width: "100%",
          }}
        >
          Continue with Google
        </button>

        {DEV_MODE && (
          <button
            onClick={handleDevLogin}
            style={{
              background: "transparent",
              color: "#555",
              border: "1px dashed #333",
              borderRadius: "8px",
              padding: "12px",
              fontSize: "13px",
              cursor: "pointer",
              width: "100%",
            }}
          >
            Dev login (local only)
          </button>
        )}

        {error && (
          <p style={{ color: "#f66", fontSize: "13px", margin: 0 }}>{error}</p>
        )}
      </div>
    </div>
  );
}
