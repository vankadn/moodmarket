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
  brokerages?: BrokerageStatus[];
  connected_accounts?: PlaidConnectionSummary[];
}

export interface AutoInvestConfig {
  id?: string;
  user_id?: string;
  enabled: boolean;
  amount: number;
  risk: RiskTolerance;
  interval_days?: number;
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
}

export interface Recommendation {
  total_budget: number;
  allocations: Allocation[];
  summary: string;
  risk_level: "low" | "medium" | "high";
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

export interface ActivityDecision {
  id: string;
  timestamp: string;
  total_amount: number;
  risk_level: string;
}

export interface ActivitySummary {
  total_decisions: number;
  total_invested: number;
  decisions: ActivityDecision[];
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

export async function removeBrokerageConnection(connectionID: string): Promise<void> {
  const res = await fetch(`${API_BASE}/brokerage/connections/${connectionID}`, {
    method: "DELETE",
    headers: await authHeaders(),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
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
