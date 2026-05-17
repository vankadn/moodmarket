// frontend/src/AppShell.tsx
// Single source of truth for all post-auth routing. Both DevApp and ClerkApp
// render this after their respective auth gates pass. Adding a new page means
// editing only this file.
import { useEffect, useState } from "react";
import { Onboarding } from "./pages/Onboarding";
import { Home } from "./pages/Home";
import { Profile } from "./pages/Profile";
import { AutoInvestList } from "./pages/AutoInvestList";
import { AutoInvestSettings } from "./pages/AutoInvestSettings";
import { Activity } from "./pages/Activity";
import { Portfolio } from "./pages/Portfolio";
import { BrokerageConnect } from "./components/BrokerageConnect";
import { Documents } from "./pages/Documents";
import { NotificationSettingsPage } from "./pages/NotificationSettingsPage";
import { AutoInvestConfig, getAutoInvestConfigs, getProfile, UserProfile } from "./services/api";

export type AppState =
  | "loading"
  | "onboarding"
  | "home"
  | "profile"
  | "auto-invest-list"
  | "auto-invest-settings"
  | "notifications"
  | "activity"
  | "portfolio"
  | "brokerage"
  | "documents";

interface AppShellProps {
  signOut?: () => void;
  // keepPageOnRefresh=true (Clerk): after Plaid/brokerage change, reload profile
  // and stay on the current page. keepPageOnRefresh=false (Dev): just go to "loading"
  // which naturally navigates home via the useEffect.
  keepPageOnRefresh?: boolean;
}

export function AppShell({ signOut, keepPageOnRefresh = false }: AppShellProps) {
  const [state, setState] = useState<AppState>("loading");
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [selectedAutoInvestConfig, setSelectedAutoInvestConfig] = useState<AutoInvestConfig | undefined>(undefined);
  const [autoInvestConfigs, setAutoInvestConfigs] = useState<AutoInvestConfig[]>([]);

  useEffect(() => {
    if (state !== "loading") return;
    Promise.all([
      getProfile(),
      getAutoInvestConfigs().catch(() => [] as AutoInvestConfig[]),
    ]).then(([p, configs]) => {
      setProfile(p);
      setAutoInvestConfigs(configs);
      setState("home");
    }).catch(() => setState("onboarding"));
  }, [state]);

  function refreshAndReturn(returnTo: AppState) {
    if (!keepPageOnRefresh) {
      setState("loading");
      return;
    }
    getProfile()
      .then((p) => { setProfile(p); setState(returnTo); })
      .catch(() => setState("home"));
  }

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

  if (state === "profile") {
    return (
      <Profile
        profile={profile!}
        onBack={() => setState("home")}
        onAccountsChanged={() => refreshAndReturn("profile")}
      />
    );
  }

  if (state === "auto-invest-list") {
    return (
      <AutoInvestList
        initialConfigs={autoInvestConfigs}
        onConfigsChange={setAutoInvestConfigs}
        onBack={() => setState("home")}
        onSelectConfig={(config) => { setSelectedAutoInvestConfig(config); setState("auto-invest-settings"); }}
        onAddConfig={() => { setSelectedAutoInvestConfig(undefined); setState("auto-invest-settings"); }}
      />
    );
  }

  if (state === "auto-invest-settings") {
    return (
      <AutoInvestSettings
        initialConfig={selectedAutoInvestConfig}
        onBack={() => {
          getAutoInvestConfigs().then(setAutoInvestConfigs).catch(() => {});
          setState("auto-invest-list");
        }}
      />
    );
  }

  if (state === "notifications") {
    return (
      <NotificationSettingsPage
        onBack={() => setState("home")}
        onSaved={() => refreshAndReturn("home")}
      />
    );
  }

  if (state === "activity") {
    return <Activity onBack={() => setState("home")} />;
  }

  if (state === "portfolio") {
    return <Portfolio onBack={() => setState("home")} />;
  }

  if (state === "brokerage") {
    return (
      <BrokerageConnect
        connections={profile?.brokerages ?? []}
        onBack={() => setState("home")}
        onChanged={() => refreshAndReturn("brokerage")}
      />
    );
  }

  if (state === "documents") {
    return <Documents onBack={() => setState("home")} />;
  }

  return (
    <Home
      profile={profile!}
      autoInvestConfigs={autoInvestConfigs}
      onSignOut={signOut}
      onManageAccounts={() => setState("profile")}
      onAutoInvestSettings={() => setState("auto-invest-list")}
      onNotificationSettings={() => setState("notifications")}
      onActivity={() => setState("activity")}
      onPortfolio={() => setState("portfolio")}
      onBrokerage={() => setState("brokerage")}
      onDocuments={() => setState("documents")}
    />
  );
}
