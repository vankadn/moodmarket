// frontend/src/App.tsx
import { useState } from "react";
import { Login } from "./pages/Login";
import { AppShell } from "./AppShell";
import { ClerkApp } from "./ClerkApp";

const DEV_MODE = import.meta.env.VITE_DEV_MODE === "true";

function DevApp() {
  const [authed, setAuthed] = useState(() => Boolean(localStorage.getItem("auth_token")));

  if (!authed) return <Login onAuthenticated={() => setAuthed(true)} />;
  return <AppShell />;
}

// App: single branch point. DEV_MODE is a build-time constant — no runtime check elsewhere.
export default function App() {
  return DEV_MODE ? <DevApp /> : <ClerkApp />;
}
