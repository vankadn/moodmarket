// frontend/src/App.tsx
import { useEffect, useState } from "react";
import { Login } from "./pages/Login";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { getProfile, UserProfile } from "./services/api";
import { ClerkApp } from "./ClerkApp";

const DEV_MODE = import.meta.env.VITE_DEV_MODE === "true";

// DevApp: existing localStorage-based auth flow — unchanged from Phase 2.
type DevAppState = "auth" | "loading" | "onboarding" | "home";

function DevApp() {
  const [state, setState] = useState<DevAppState>(() =>
    localStorage.getItem("auth_token") ? "loading" : "auth"
  );
  const [profile, setProfile] = useState<UserProfile | null>(null);

  useEffect(() => {
    if (state !== "loading") return;
    getProfile()
      .then((p: UserProfile) => { setProfile(p); setState("home"); })
      .catch(() => setState("onboarding"));
  }, [state]);

  if (state === "auth") return <Login onAuthenticated={() => setState("loading")} />;

  if (state === "loading") {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh" }}>
        <span style={{ color: "#888", fontSize: "14px" }}>Loading…</span>
      </div>
    );
  }

  if (state === "onboarding") {
    return (
      <Onboarding
        onComplete={(p) => { setProfile(p); setState("home"); }}
      />
    );
  }

  return <Home profile={profile!} />;
}

// App: single branch point. DEV_MODE is a build-time constant — no runtime check elsewhere.
export default function App() {
  return DEV_MODE ? <DevApp /> : <ClerkApp />;
}
