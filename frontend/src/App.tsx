import { useEffect, useState } from "react";
import { Login } from "./pages/Login";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { getProfile, UserProfile } from "./services/api";

type AppState = "auth" | "loading" | "onboarding" | "home";

function App() {
  const [state, setState] = useState<AppState>(() =>
    localStorage.getItem("auth_token") ? "loading" : "auth"
  );
  const [profile, setProfile] = useState<UserProfile | null>(null);

  useEffect(() => {
    if (state !== "loading") return;
    getProfile()
      .then((p: UserProfile) => {
        setProfile(p);
        setState("home");
      })
      .catch(() => setState("onboarding"));
  }, [state]);

  if (state === "auth") {
    return <Login onAuthenticated={() => setState("loading")} />;
  }

  if (state === "loading") {
    return (
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          height: "100vh",
        }}
      >
        <span style={{ color: "#888", fontSize: "14px" }}>Loading…</span>
      </div>
    );
  }

  if (state === "onboarding") {
    return (
      <Onboarding
        onComplete={(p) => {
          setProfile(p);
          setState("home");
        }}
      />
    );
  }

  return <Home profile={profile!} />;
}

export default App;
