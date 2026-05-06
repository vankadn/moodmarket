// frontend/src/ClerkApp.tsx
// Clerk-specific app shell. Only rendered when VITE_DEV_MODE=false.
// Manages auth state via Clerk hooks; dev mode App.tsx is untouched.
import { useEffect, useState } from "react";
import { useAuth, useClerk, SignIn } from "@clerk/clerk-react";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { getProfile, UserProfile, setTokenFetcher } from "./services/api";

type AppState = "loading" | "onboarding" | "home";

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

  // Not signed in — show Clerk's embedded sign-in component.
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

  return (
    <Home
      profile={profile!}
      onSignOut={() => signOut()}
    />
  );
}
