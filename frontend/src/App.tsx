import { useEffect, useState } from "react";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { getProfile, UserProfile } from "./services/api";

type AppState = "loading" | "onboarding" | "home";

function App() {
  const [state, setState] = useState<AppState>("loading");
  const [profile, setProfile] = useState<UserProfile | null>(null);

  useEffect(() => {
    getProfile()
      .then((p: UserProfile) => { setProfile(p); setState("home"); })
      .catch(() => setState("onboarding"));
  }, []);

  if (state === "loading") {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh" }}>
        <span style={{ color: "#888", fontSize: "14px" }}>Loading…</span>
      </div>
    );
  }

  if (state === "onboarding") {
    return <Onboarding onComplete={(p) => { setProfile(p); setState("home"); }} />;
  }

  return <Home profile={profile!} />;
}

export default App;
