const API_BASE = import.meta.env.VITE_API_URL || "http://localhost:8080";

export type TimeHorizon = "under_1_year" | "one_to_five" | "five_to_ten" | "ten_plus";
export type ImmigrationStatus = "us_citizen" | "permanent_resident" | "work_visa" | "other";
export type RiskTolerance = "conservative" | "moderate" | "aggressive";
export type InvestmentGoal = "wealth_building" | "retirement" | "emergency_fund" | "short_term_savings";

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

export async function saveProfile(profile: UserProfile): Promise<UserProfile> {
  const res = await fetch(`${API_BASE}/users/profile`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(await authHeaders()) },
    body: JSON.stringify(profile),
  });
  if (!res.ok) throw new Error(`API error: ${res.status}`);
  return res.json();
}
