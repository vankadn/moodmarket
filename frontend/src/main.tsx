// frontend/src/main.tsx
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ClerkProvider } from "@clerk/clerk-react";
import App from "./App";

const DEV_MODE = import.meta.env.VITE_DEV_MODE === "true";
const publishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY ?? "";

// ClerkProvider is only mounted when DEV_MODE=false — it requires a valid publishable key.
// In dev mode App renders directly, keeping the existing auth flow intact.
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {DEV_MODE ? (
      <App />
    ) : (
      <ClerkProvider publishableKey={publishableKey}>
        <App />
      </ClerkProvider>
    )}
  </StrictMode>
);
