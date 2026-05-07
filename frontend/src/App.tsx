// frontend/src/App.tsx
import { useEffect, useState } from "react";
import { Login } from "./pages/Login";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { Profile } from "./pages/Profile";
import { getProfile, UserProfile } from "./services/api";
import { ClerkApp } from "./ClerkApp";

const DEV_MODE = import.meta.env.VITE_DEV_MODE === "true";

type DevAppState = "auth" | "loading" | "onboarding" | "home" | "profile";

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

  if (state === "profile") {
    return (
      <Profile
        profile={profile!}
        onBack={() => setState("home")}
        onAccountsChanged={() => setState("loading")} // reload profile to pick up updated connected_accounts
      />
    );
  }

  return (
    <Home
      profile={profile!}
      onManageAccounts={() => setState("profile")}
    />
  );
}

// App: single branch point. DEV_MODE is a build-time constant — no runtime check elsewhere.
export default function App() {
  return DEV_MODE ? <DevApp /> : <ClerkApp />;
}
