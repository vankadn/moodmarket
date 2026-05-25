import { useEffect, useState } from "react";
import { ConfirmScreen } from "../components/ConfirmScreen";
import { ReceiptScreen } from "../components/ReceiptScreen";
import { CashContextCard } from "../components/CashContextCard";
import { getRecommendation, invest, getCashContext, AutoInvestConfig, BrokerageStatus, CashContext, Recommendation, TradeReceipt, UserProfile } from "../services/api";

interface Props {
  profile: UserProfile;
  autoInvestConfigs: AutoInvestConfig[];
  onSignOut?: () => void;
  onManageAccounts?: () => void;
  onAutoInvestSettings?: () => void;
  onNotificationSettings?: () => void;
  onBrokerage?: () => void;
  onPortfolioConnect?: () => void;
  onDocuments?: () => void;
  onPortfolio?: () => void;
  onEval?: () => void;
  onAllocationPreferences?: () => void;
}

const goalLabel: Record<string, string> = {
  wealth_building: "Wealth building",
  retirement: "Retirement",
  emergency_fund: "Emergency fund",
  short_term_savings: "Short-term savings",
};

const horizonLabel: Record<string, string> = {
  under_1_year: "Under 1 year",
  one_to_five: "1–5 years",
  five_to_ten: "5–10 years",
  ten_plus: "10+ years",
};

type HomeState = "idle" | "confirming" | "investing" | "receipt";

const brokerageIsConnected = (brokerages: BrokerageStatus[] | undefined) =>
  (brokerages?.length ?? 0) > 0;

export function Home({ profile, autoInvestConfigs, onSignOut, onManageAccounts, onAutoInvestSettings, onNotificationSettings, onBrokerage, onPortfolioConnect, onDocuments, onPortfolio, onEval, onAllocationPreferences }: Props) {
  const [amount, setAmount] = useState<number>(100);
  const [perAllocBrokerage, setPerAllocBrokerage] = useState<Record<string, string>>({});
  const [cashCtx, setCashCtx] = useState<CashContext | null>(null);
  const [homeState, setHomeState] = useState<HomeState>("idle");
  const [rec, setRec] = useState<Recommendation | null>(null);
  const [receipts, setReceipts] = useState<TradeReceipt[]>([]);
  const [decisionId, setDecisionId] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const connected = brokerageIsConnected(profile.brokerages);
  const brokerages = profile.brokerages ?? [];

  useEffect(() => {
    getCashContext().then(ctx => {
      if (ctx.has_data && ctx.runway_label === "tight") {
        const today = new Date().toISOString().slice(0, 10);
        if (localStorage.getItem("cash_card_shown_date") !== today) {
          setCashCtx(ctx);
        }
      }
    }).catch(() => {});
  }, []);

  async function handleGetRecommendation() {
    setLoading(true);
    setError(null);
    try {
      const result = await getRecommendation({ base_budget: amount, extra_money: 0 });
      setRec(result);
      setPerAllocBrokerage({});
      setHomeState("confirming");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  async function handleConfirmInvest() {
    if (!rec) return;
    setHomeState("investing");
    setError(null);
    try {
      const overrides = Object.keys(perAllocBrokerage).length > 0 ? perAllocBrokerage : undefined;
      const response = await invest({
        allocations: rec.allocations,
        total_amount: rec.total_budget,
        risk_level: rec.risk_level,
        summary: rec.summary,
        per_allocation_brokerage: overrides,
      });
      setReceipts(response.receipts);
      setDecisionId(response.decision_id);
      setHomeState("receipt");
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "Investment failed");
      setHomeState("confirming");
    }
  }

  function handleCancel() {
    setRec(null);
    setPerAllocBrokerage({});
    setHomeState("idle");
    setError(null);
  }

  function handleDone() {
    setRec(null);
    setReceipts([]);
    setDecisionId("");
    setPerAllocBrokerage({});
    setHomeState("idle");
    setError(null);
  }

  function handleCashCardDismiss() {
    const today = new Date().toISOString().slice(0, 10);
    localStorage.setItem("cash_card_shown_date", today);
    setCashCtx(null);
  }

  function handlePerAllocChange(ticker: string, connID: string) {
    setPerAllocBrokerage(prev => ({ ...prev, [ticker]: connID }));
  }

  const profileSummary = [
    { label: "Name",    value: profile.full_name },
    { label: "Goal",    value: goalLabel[profile.investment_goal] ?? profile.investment_goal },
    { label: "Risk",    value: profile.risk_tolerance.charAt(0).toUpperCase() + profile.risk_tolerance.slice(1) },
    { label: "Horizon", value: horizonLabel[profile.time_horizon] ?? profile.time_horizon },
  ];

  const enabledConfigs = autoInvestConfigs.filter(c => c.enabled);
  const autoInvestLabel =
    enabledConfigs.length === 0 ? "Off" :
    enabledConfigs.length === 1 ? `Enabled — $${enabledConfigs[0].amount}/day` :
    `${enabledConfigs.length} active`;

  return (
    <div style={{ maxWidth: "560px", margin: "0 auto", padding: "2rem 1rem" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "1.5rem" }}>
        <div>
          <h1 style={{ fontSize: "22px", fontWeight: 600, margin: "0 0 4px" }}>InvestIQ</h1>
          <p style={{ fontSize: "14px", color: "#666", margin: 0 }}>Daily investment advisor</p>
        </div>
        <div style={{ display: "flex", gap: "12px", alignItems: "center" }}>
          {onPortfolio && (
            <button
              onClick={onPortfolio}
              style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Portfolio
            </button>
          )}
          {onEval && (
            <button
              onClick={onEval}
              style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Performance
            </button>
          )}
          {onBrokerage && (
            <button
              onClick={onBrokerage}
              style={{ background: "none", border: "none", color: connected ? "#999" : "#c0392b", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Brokerage
            </button>
          )}
          {onPortfolioConnect && (
            <button
              onClick={onPortfolioConnect}
              style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Ext. accounts
            </button>
          )}
          {onDocuments && (
            <button
              onClick={onDocuments}
              style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Tax docs
            </button>
          )}
          {onManageAccounts && (
            <button
              onClick={onManageAccounts}
              style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Bank accounts
            </button>
          )}
          {onSignOut && (
            <button
              onClick={onSignOut}
              style={{ background: "none", border: "none", color: "#999", fontSize: "13px", cursor: "pointer", padding: "4px 0" }}
            >
              Sign out
            </button>
          )}
        </div>
      </div>

      {/* Profile summary card */}
      <div style={{ background: "#f8f8f8", borderRadius: "12px", padding: "1rem 1.25rem", marginBottom: "1.5rem" }}>
        <div style={{ fontSize: "11px", fontWeight: 600, color: "#aaa", letterSpacing: "0.07em", textTransform: "uppercase", marginBottom: "10px" }}>
          Your profile
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px 16px" }}>
          {profileSummary.map(({ label, value }) => (
            <div key={label}>
              <div style={{ fontSize: "11px", color: "#999", marginBottom: "2px" }}>{label}</div>
              <div style={{ fontSize: "14px", fontWeight: 500, color: "#222" }}>{value}</div>
            </div>
          ))}
        </div>
      </div>

      {homeState === "idle" && (
        <>
          {/* Auto-invest settings row */}
          <button
            onClick={onAutoInvestSettings}
            style={{
              width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between",
              padding: "12px 14px", background: "#f8f8f8", borderRadius: "10px",
              border: "none", cursor: "pointer", marginBottom: "8px", textAlign: "left",
            }}
          >
            <div>
              <div style={{ fontSize: "13px", fontWeight: 500, color: "#222" }}>Auto-invest</div>
              <div style={{ fontSize: "11px", color: enabledConfigs.length > 0 ? "#27ae60" : "#999", marginTop: "2px" }}>
                {autoInvestLabel}
              </div>
            </div>
            <span style={{ color: "#bbb", fontSize: "16px" }}>›</span>
          </button>

          {/* Notifications settings row */}
          <button
            onClick={onNotificationSettings}
            style={{
              width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between",
              padding: "12px 14px", background: "#f8f8f8", borderRadius: "10px",
              border: "none", cursor: "pointer", marginBottom: "8px", textAlign: "left",
            }}
          >
            <div>
              <div style={{ fontSize: "13px", fontWeight: 500, color: "#222" }}>Notifications</div>
              <div style={{ fontSize: "11px", color: "#999", marginTop: "2px" }}>
                {profile.notification_email ? profile.notification_email : "Not configured"}
              </div>
            </div>
            <span style={{ color: "#bbb", fontSize: "16px" }}>›</span>
          </button>

          {/* Allocation preferences row */}
          <button
            onClick={onAllocationPreferences}
            style={{
              width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between",
              padding: "12px 14px", background: "#f8f8f8", borderRadius: "10px",
              border: "none", cursor: "pointer", marginBottom: "1.5rem", textAlign: "left",
            }}
          >
            <div>
              <div style={{ fontSize: "13px", fontWeight: 500, color: "#222" }}>Allocation limits</div>
              <div style={{ fontSize: "11px", color: "#999", marginTop: "2px" }}>
                {profile.allocation_preferences?.asset_class_limits?.length
                  ? `${profile.allocation_preferences.asset_class_limits.length} constraint${profile.allocation_preferences.asset_class_limits.length > 1 ? "s" : ""} set`
                  : "No limits set — Claude decides freely"}
              </div>
            </div>
            <span style={{ color: "#bbb", fontSize: "16px" }}>›</span>
          </button>

          {/* Cash context nudge — tight runway only, shown once per day */}
          {cashCtx && (
            <CashContextCard ctx={cashCtx} onDismiss={handleCashCardDismiss} />
          )}

          {/* Investment amount + action row */}
          <div style={{ marginBottom: "1.5rem" }}>
            <label style={{ fontSize: "12px", fontWeight: 500, color: "#888", letterSpacing: "0.05em", textTransform: "uppercase" }}>
              Today's investment
            </label>
            {!connected && (
              <div style={{ marginTop: "8px", padding: "10px 12px", background: "#fdf0ee", border: "1px solid #f5c6cb", borderRadius: "8px", fontSize: "13px", color: "#c0392b" }}>
                Connect a brokerage account to invest.{" "}
                {onBrokerage && (
                  <button onClick={onBrokerage} style={{ background: "none", border: "none", color: "#c0392b", textDecoration: "underline", cursor: "pointer", fontSize: "13px", padding: 0 }}>
                    Set up now →
                  </button>
                )}
              </div>
            )}
            <div style={{ display: "flex", alignItems: "center", gap: "10px", marginTop: "8px" }}>
              <div style={{ display: "flex", alignItems: "center", border: "1px solid #e0e0e0", borderRadius: "8px", overflow: "hidden", flexShrink: 0 }}>
                <span style={{ padding: "8px 10px", fontSize: "14px", color: "#888", background: "#f8f8f8", borderRight: "1px solid #e0e0e0" }}>$</span>
                <input
                  type="number" min={1} step={10} value={amount}
                  disabled={loading}
                  onChange={(e) => setAmount(Math.max(1, Number(e.target.value)))}
                  style={{ width: "80px", padding: "8px 10px", border: "none", outline: "none", fontSize: "14px", background: loading ? "#f8f8f8" : "white" }}
                />
              </div>
              <button
                onClick={handleGetRecommendation}
                disabled={loading}
                style={{
                  flex: 1, padding: "8px 16px",
                  background: loading ? "#ccc" : "#1a1a1a",
                  color: "white", border: "none", borderRadius: "8px",
                  fontSize: "14px", fontWeight: 500,
                  cursor: loading ? "not-allowed" : "pointer",
                }}
              >
                {loading ? "Generating…" : "Get recommendation"}
              </button>
            </div>
          </div>
        </>
      )}

      {error && (
        <div style={{ color: "#c0392b", fontSize: "13px", padding: "10px", background: "#fdf0ee", borderRadius: "8px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      {homeState === "confirming" && rec && (
        <ConfirmScreen
          rec={rec}
          brokerages={brokerages}
          perAllocBrokerage={perAllocBrokerage}
          onPerAllocChange={handlePerAllocChange}
          onConfirm={handleConfirmInvest}
          onCancel={handleCancel}
          loading={false}
        />
      )}

      {homeState === "investing" && rec && (
        <ConfirmScreen
          rec={rec}
          brokerages={brokerages}
          perAllocBrokerage={perAllocBrokerage}
          onPerAllocChange={handlePerAllocChange}
          onConfirm={handleConfirmInvest}
          onCancel={handleCancel}
          loading={true}
        />
      )}

      {homeState === "receipt" && (
        <ReceiptScreen
          receipts={receipts}
          decisionId={decisionId}
          onDone={handleDone}
        />
      )}
    </div>
  );
}
