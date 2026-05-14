// frontend/src/ClerkApp.tsx
// Clerk-specific app shell. Only rendered when VITE_DEV_MODE=false.
import { useEffect, useState } from "react";
import { useAuth, useClerk, SignIn } from "@clerk/clerk-react";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { Profile } from "./pages/Profile";
import { AutoInvestSettings } from "./pages/AutoInvestSettings";
import { Activity } from "./pages/Activity";
import { getProfile, UserProfile, setTokenFetcher } from "./services/api";
import { BrokerageConnect } from "./components/BrokerageConnect";
import { Documents } from "./pages/Documents";

type AppState = "loading" | "onboarding" | "home" | "profile" | "auto-invest-settings" | "activity" | "brokerage" | "documents";

function Spinner() {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh" }}>
      <span style={{ color: "#888", fontSize: "14px" }}>Loading…</span>
    </div>
  );
}

export function ClerkApp() {
  const { isLoaded, isSignedIn, getToken } = useAuth();
  const { signOut } = useClerk();
  const [state, setState] = useState<AppState>("loading");
  const [profile, setProfile] = useState<UserProfile | null>(null);

  // Wire Clerk's getToken into the api.ts layer so all fetch calls use the Clerk session JWT.
  useEffect(() => {
    setTokenFetcher(() => getToken());
  }, [getToken]);

  // Once Clerk confirms the user is signed in, attempt to load their profile.
  useEffect(() => {
    if (!isLoaded || !isSignedIn) return;
    setState("loading");
    getProfile()
      .then((p) => { setProfile(p); setState("home"); })
      .catch(() => setState("onboarding"));
  }, [isLoaded, isSignedIn]);

  if (!isLoaded) return <Spinner />;

  if (!isSignedIn) {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh", background: "#0a0a0a" }}>
        <SignIn routing="hash" />
      </div>
    );
  }

  if (state === "loading") return <Spinner />;

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
        onAccountsChanged={() => {
          setState("loading");
          getProfile()
            .then((p) => { setProfile(p); setState("profile"); }) // stay on profile page after refresh
            .catch(() => setState("home"));
        }}
      />
    );
  }

  if (state === "auto-invest-settings") {
    return <AutoInvestSettings onBack={() => setState("home")} />;
  }

  if (state === "activity") {
    return <Activity onBack={() => setState("home")} />;
  }

  if (state === "brokerage") {
    return (
      <BrokerageConnect
        status={profile?.brokerage}
        onBack={() => setState("home")}
        onChanged={() => {
          setState("loading");
          getProfile()
            .then((p) => { setProfile(p); setState("brokerage"); })
            .catch(() => setState("home"));
        }}
      />
    );
  }

  if (state === "documents") {
    return <Documents onBack={() => setState("home")} />;
  }

  return (
    <Home
      profile={profile!}
      onSignOut={() => signOut()}
      onManageAccounts={() => setState("profile")}
      onAutoInvestSettings={() => setState("auto-invest-settings")}
      onActivity={() => setState("activity")}
      onBrokerage={() => setState("brokerage")}
      onDocuments={() => setState("documents")}
    />
  );
}
