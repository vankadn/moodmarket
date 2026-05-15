// frontend/src/ClerkApp.tsx
// Clerk auth gate — only rendered when VITE_DEV_MODE=false.
// Once the user is signed in, delegates all routing to AppShell.
import { useEffect } from "react";
import { useAuth, useClerk, SignIn } from "@clerk/clerk-react";
import { setTokenFetcher } from "./services/api";
import { AppShell } from "./AppShell";

export function ClerkApp() {
  const { isLoaded, isSignedIn, getToken } = useAuth();
  const { signOut } = useClerk();

  useEffect(() => {
    setTokenFetcher(() => getToken());
  }, [getToken]);

  if (!isLoaded) {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh" }}>
        <span style={{ color: "#888", fontSize: "14px" }}>Loading…</span>
      </div>
    );
  }

  if (!isSignedIn) {
    return (
      <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100vh", background: "#0a0a0a" }}>
        <SignIn routing="hash" />
      </div>
    );
  }

  return <AppShell signOut={() => signOut()} keepPageOnRefresh />;
}
