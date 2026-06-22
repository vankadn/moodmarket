const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:8080";

export type TimeHorizon = "under_1_year" | "one_to_five" | "five_to_ten" | "ten_plus";
export type ImmigrationStatus = "us_citizen" | "permanent_resident" | "work_visa" | "other";
export type RiskTolerance = "conservative" | "moderate" | "aggressive";
export type InvestmentGoal = "wealth_building" | "retirement" | "emergency_fund" | "short_term_savings";

export interface PlaidConnectionSummary {
  institution: string;
  item_id: string;
}

export type AssetCategory = "equity" | "bond" | "default";

export interface BrokerageStatus {
  id: string;
  name?: string;
  asset_categories?: AssetCategory[];
  connected: boolean;
  base_url?: string;
  connected_at?: string;
}

export interface PortfolioConnectionStatus {
  provider: string;
  connected: boolean;
  connected_at?: string;
}

export interface LinkedAccount {
  id: string;
  name: string;
}

export interface AssetClassLimit {
  asset_class: string;
  min_pct?: number;
  max_pct?: number;
}

export interface AllocationPreferences {
  asset_class_limits?: AssetClassLimit[];
  max_single_ticker_pct?: number;
}

export interface UserProfile {
  user_id?: string;
  full_name: string;
  salary: number;
  monthly_savings: number;
  retirement_contribution_percent: number;
  existing_portfolio_value: number;
  time_horizon: TimeHorizon;
  immigration_status: ImmigrationStatus;
  risk_tolerance: RiskTolerance;
  investment_goal: InvestmentGoal;
  has_emergency_fund: boolean;
  include_cash_context: boolean;
  notification_email?: string;
  phone?: string;
  allocation_preferences?: AllocationPreferences;
  brokerages?: BrokerageStatus[];
  connected_accounts?: PlaidConnectionSummary[];
  portfolio_aggregator?: PortfolioConnectionStatus;
}

export type StrategyType = "long_term" | "short_term";

export interface AutoInvestConfig {
  id?: string;
  user_id?: string;
  name?: string;
  enabled: boolean;
  mode?: "fixed" | "agentic"; // default "fixed" when absent
  amount: number;              // fixed mode: amount per run
  daily_budget?: number;       // agentic mode: max per calendar day
  risk: RiskTolerance;
  strategy?: StrategyType;
  interval_days?: number;      // deprecated — migrated to interval_hours on read
  interval_hours?: number;     // 1 | 2 | 4 | 24; agentic hardcodes to 1
  interval_seconds?: number;   // dev/sub-hour testing only
  enabled_at?: string;
  updated_at?: string;
  last_run_at?: string;
}

export interface InvestmentRequest {
  base_budget: number;
  extra_money: number;
}

export interface Allocation {
  ticker: string;
  name: string;
  type: string;
  amount: number;
  percentage: number;
  rationale: string;
  reasoning?: string;
}

export interface Recommendation {
  total_budget: number;
  allocations: Allocation[] | null;
  summary: string;
  risk_level: "low" | "medium" | "high";
  overall_reasoning?: string;
  from_cache?: boolean;
}

// tokenFetcher is the pluggable source of the bearer token.
// Dev mode: reads from localStorage. Clerk mode: calls Clerk's getToken().
// Swap by calling setTokenFetcher() before the first API call.
type TokenFetcher = () => Promise<string | null>;
let tokenFetcher: TokenFetcher = async () => localStorage.getItem("auth_token");

export function setTokenFetcher(fn: TokenFetcher): void {
  tokenFetcher = fn;
}

async function authHeaders(): Promise<HeadersInit> {
  const token = await tokenFetcher();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function getRecommendation(req: InvestmentRequest): Promise<Recommendation> {
  const res = await fetch(`${API_BASE}/recommend`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getProfile(): Promise<UserProfile> {
  const res = await fetch(`${API_BASE}/users/profile`, {
    headers: await authHeaders(),
  });
  if (res.status === 404) throw new Error("not_found");
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export interface TradeReceipt {
  order_id: string;
  ticker: string;
  filled_amount: number;
  filled_price: number;
  status: string;
  timestamp: string;
  brokerage_id?: string;
  brokerage_name?: string;
}

export interface InvestRequest {
  allocations: Allocation[];
  total_amount: number;
  risk_level: string;
  summary: string;
  overall_reasoning?: string;
  per_allocation_brokerage?: Record<string, string>;
}

export interface InvestResponse {
  receipts: TradeReceipt[];
  decision_id: string;
}

export async function invest(req: InvestRequest): Promise<InvestResponse> {
  const res = await fetch(`${API_BASE}/invest`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(req),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function createLinkToken(): Promise<string> {
  const res = await fetch(`${API_BASE}/plaid/link-token`, {
    method: "POST",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  const data = await res.json();
  return data.link_token;
}

export async function exchangePublicToken(publicToken: string): Promise<{ institution: string; connected_accounts: number }> {
  const res = await fetch(`${API_BASE}/plaid/exchange`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify({ public_token: publicToken }),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function deletePlaidAccount(itemId: string): Promise<void> {
  const res = await fetch(`${API_BASE}/plaid/accounts/${itemId}`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

export async function getAutoInvestConfig(): Promise<AutoInvestConfig> {
  const res = await fetch(`${API_BASE}/users/auto-invest/config`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function saveAutoInvestConfig(config: AutoInvestConfig): Promise<AutoInvestConfig> {
  const res = await fetch(`${API_BASE}/users/auto-invest/config`, {
    method: "PUT",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(config),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getAutoInvestConfigs(): Promise<AutoInvestConfig[]> {
  const res = await fetch(`${API_BASE}/users/auto-invest/configs`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  const data = await res.json();
  return data || [];
}

export async function createAutoInvestConfig(config: AutoInvestConfig): Promise<AutoInvestConfig> {
  const res = await fetch(`${API_BASE}/users/auto-invest/configs`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(config),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function updateAutoInvestConfig(id: string, config: AutoInvestConfig): Promise<AutoInvestConfig> {
  const res = await fetch(`${API_BASE}/users/auto-invest/configs/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(config),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function deleteAutoInvestConfig(id: string): Promise<void> {
  const res = await fetch(`${API_BASE}/users/auto-invest/configs/${id}`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

export interface CriticReview {
  verdict: string;
  concerns: string[];
  risk_level: string;
  reasoning: string;
}

export interface ActivityDecision {
  id: string;
  timestamp: string;
  total_amount: number;
  risk_level: string;
  decision_type?: string;
  blocked_reason?: string;
  critic_review?: CriticReview;
}

export interface ActivitySummary {
  total_decisions: number;
  total_invested: number;
  decisions: ActivityDecision[];
}

export interface StrategyActivity {
  config_id: string;
  total_invested: number;
  decision_count: number;
  first_run_at: string;
  last_run_at: string;
}

export interface StrategyPnL {
  config_id: string;
  total_invested: number;
  current_value: number;
  unrealized_pl: number;
  unrealized_pl_pct: number;
  brokerage_connected: boolean;
  tickers: string[];
}

export async function getStrategyPnL(): Promise<StrategyPnL[]> {
  const res = await fetch(`${API_BASE}/users/activity/by-strategy/pnl`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getActivityByStrategy(): Promise<StrategyActivity[]> {
  const res = await fetch(`${API_BASE}/users/activity/by-strategy`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getActivity(since: Date | null): Promise<ActivitySummary> {
  const url = since
    ? `${API_BASE}/users/activity?since=${encodeURIComponent(since.toISOString())}`
    : `${API_BASE}/users/activity`;
  const res = await fetch(url, { headers: await authHeaders() });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getOrderStatus(orderID: string): Promise<TradeReceipt> {
  const res = await fetch(`${API_BASE}/orders/${orderID}`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export interface CashContext {
  has_data: boolean;
  runway_days: number;
  runway_label: "healthy" | "moderate" | "tight" | "";
  spend_last_7d: number;
  spend_last_30d: number;
  largest_pending_amount: number;
  largest_pending_name: string;
  message: string;
  user_override: boolean;
}

export async function getCashContext(): Promise<CashContext> {
  const res = await fetch(`${API_BASE}/users/cash-context`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export interface NotificationSettings {
  notification_email: string;
  phone: string;
}

export async function getNotificationSettings(): Promise<NotificationSettings> {
  const res = await fetch(`${API_BASE}/users/notifications`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function updateNotificationSettings(settings: NotificationSettings): Promise<NotificationSettings> {
  const res = await fetch(`${API_BASE}/users/notifications`, {
    method: "PATCH",
    headers: { ...(await authHeaders()), "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export interface AddBrokerageConnectionRequest {
  id?: string;
  name: string;
  asset_categories: AssetCategory[];
  api_key: string;
  secret_key: string;
  base_url: string;
}

export async function addBrokerageConnection(req: AddBrokerageConnectionRequest): Promise<BrokerageStatus> {
  const res = await fetch(`${API_BASE}/brokerage/connections`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(req),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `API error: ${res.status}`);
  }
  return res.json();
}

export async function connectPortfolioAggregator(): Promise<{ redirect_url: string }> {
  const res = await fetch(`${API_BASE}/portfolio/connect`, {
    method: "POST",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function disconnectPortfolioAggregator(): Promise<void> {
  const res = await fetch(`${API_BASE}/portfolio/connect`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

export async function getLinkedAccounts(): Promise<{ accounts: LinkedAccount[] }> {
  const res = await fetch(`${API_BASE}/portfolio/accounts`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function removeBrokerageConnection(connectionID: string): Promise<void> {
  const res = await fetch(`${API_BASE}/brokerage/connections/${connectionID}`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

// --- Eval (Phase 23) ---

export interface EvalDecisionRef {
  id: string;
  date: string;
  return_pct: number;
  amount: number;
}

export interface StrategyEvalItem {
  config_id: string;
  win_rate: number;
  avg_return_pct: number;
  decision_count: number;
}

export interface EvalSummary {
  total_decisions: number;
  verdicted_decisions: number;
  win_rate: number;         // 0.0–1.0
  avg_return_pct: number;
  avg_spy_return_pct: number;
  best_decision?: EvalDecisionRef;
  worst_decision?: EvalDecisionRef;
  by_strategy: StrategyEvalItem[];
}

export interface TickerVerdictItem {
  ticker: string;
  entry_price: number;
  prev_day_price: number;
  prev_day_timestamp: string;
  current_price: number;
  current_timestamp: string;
  return_pct: number;
  today_change_pct: number;
}

export interface VerdictItem {
  stamped_at: string;
  overall_return_pct: number;
  spy_return_pct: number;
  beat_market: boolean;
  ticker_verdicts: TickerVerdictItem[];
}

export interface EvalDecision {
  id: string;
  timestamp: string;
  total_amount: number;
  risk_level: string;
  config_id?: string;
  verdict: VerdictItem | null;
  decision_type?: string;
  blocked_reason?: string;
  overall_reasoning?: string;
  summary?: string;
  ticker_reasoning?: Record<string, string>;
  critic_review?: CriticReview;
  allocations?: Allocation[];
}

export async function getEvalSummary(): Promise<EvalSummary> {
  const res = await fetch(`${API_BASE}/users/eval/summary`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getEvalDecisions(page = 1, limit = 20): Promise<EvalDecision[]> {
  const res = await fetch(`${API_BASE}/users/eval/decisions?page=${page}&limit=${limit}`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function connectBrokerage(apiKey: string, secretKey: string, baseURL: string): Promise<BrokerageStatus> {
  const res = await fetch(`${API_BASE}/brokerage/connect`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify({ api_key: apiKey, secret_key: secretKey, base_url: baseURL }),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function disconnectBrokerage(): Promise<void> {
  const res = await fetch(`${API_BASE}/brokerage/connect`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

export type DocumentType = "w2" | "1099" | "1098";

export interface TaxDocument {
  ID: string;
  UserID: string;
  DocumentType: DocumentType;
  TaxYear: number;
  Fields: Record<string, string>;
  Verified: boolean;
  UploadedAt: string;
  VerifiedAt: string | null;
}

export async function listDocuments(): Promise<TaxDocument[]> {
  const res = await fetch(`${API_BASE}/documents`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  const data = await res.json();
  return data || [];
}

export async function uploadDocument(file: File, docType: DocumentType): Promise<TaxDocument> {
  const form = new FormData();
  form.append("type", docType);
  form.append("document", file);
  const headers = await authHeaders();
  const res = await fetch(`${API_BASE}/documents/upload`, {
    method: "POST",
    headers,
    body: form,
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `API error: ${res.status}`);
  }
  return res.json();
}

export async function deleteDocument(docID: string): Promise<void> {
  const res = await fetch(`${API_BASE}/documents/${docID}`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
}

export interface PortfolioPosition {
  ticker: string;
  name: string;
  quantity: number;
  market_value: number;
  cost_basis: number;
  avg_entry_price: number;
  unrealized_pl: number;
  unrealized_pl_percent: number;
}

export interface PortfolioAccount {
  brokerage_id: string;
  brokerage_name: string;
  positions: PortfolioPosition[];
  total_value: number;
  total_cost: number;
  total_unrealized_pl: number;
}

export interface Portfolio {
  accounts: PortfolioAccount[];
  total_value: number;
  total_cost: number;
  total_unrealized_pl: number;
  total_unrealized_pl_percent: number;
}

export type HistoryPeriod = "1D" | "5D" | "1M" | "1Y" | "5Y";

export interface HistoryPoint {
  timestamp: number;
  equity: number;
  profit_loss: number;
  profit_loss_pct: number;
}

export interface PortfolioHistory {
  period: HistoryPeriod;
  points: HistoryPoint[];
  base_value: number;
}

export async function getPortfolioHistory(period: HistoryPeriod): Promise<PortfolioHistory> {
  const res = await fetch(`${API_BASE}/portfolio/history?period=${period}`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getPortfolio(): Promise<Portfolio> {
  const res = await fetch(`${API_BASE}/portfolio`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function saveProfile(profile: UserProfile): Promise<UserProfile> {
  const res = await fetch(`${API_BASE}/users/profile`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(profile),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

// --- Performance (Phase 25) ---

export interface WinRateTrendPoint {
  week: string;     // e.g. "2025-W18"
  total: number;
  wins: number;
  win_rate: number; // 0.0–100.0
}

export interface AssetClassBreakdownItem {
  asset_class: string;
  total: number;
  wins: number;
  win_rate: number; // 0.0–100.0
}

export async function getWinRateTrend(): Promise<WinRateTrendPoint[]> {
  const res = await fetch(`${API_BASE}/performance/win-rate-trend`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

export async function getAssetClassBreakdown(): Promise<AssetClassBreakdownItem[]> {
  const res = await fetch(`${API_BASE}/performance/asset-class-breakdown`, {
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}

// --- Rebalance Analysis ---

export type SuggestedAction = "hold" | "add" | "trim" | "reconsider";
export type TaxFlag = "short_term" | "long_term" | "unknown";

export interface PositionInsight {
  ticker: string;
  name: string;
  source: "alpaca" | "snaptrade";
  account_name: string;
  current_value: number;
  unrealized_pl: number;
  unrealized_pl_pct: number;
  original_buy_thesis?: string;
  claude_assessment: string;
  suggested_action: SuggestedAction;
  tax_flag: TaxFlag;
}

export interface RebalanceAnalysis {
  insights: PositionInsight[];
  portfolio_health_summary: string;
  generated_at: string;
}

export async function analyzePortfolio(force = false): Promise<RebalanceAnalysis> {
  const res = await fetch(`${API_BASE}/rebalance/analyze`, {
    method: "POST",
    headers: { ...(await authHeaders()), "Content-Type": "application/json" },
    body: JSON.stringify({ force }),
  });
  if (res.status === 503) throw new Error("advisor_overloaded");
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}
