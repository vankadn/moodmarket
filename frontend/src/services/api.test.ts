// services/api.test.ts
//
// api.ts is the single data-access layer for the whole app. These tests pin the
// cross-cutting contract every endpoint wrapper relies on:
//  - the bearer token from the pluggable tokenFetcher is attached (and omitted
//    when there is no token)
//  - JSON bodies are serialized and Content-Type is set on writes
//  - a non-2xx response throws "API error: <status>"
//  - getProfile maps 404 to the sentinel "not_found" error
//  - list endpoints coalesce a null body to an empty array
//  - token-unwrapping endpoints return the inner field
//
// fetch is replaced with a vi.fn() so no network is touched.
import { vi, describe, it, expect, beforeEach, afterEach } from "vitest";
import {
  setTokenFetcher,
  getRecommendation,
  getProfile,
  getAutoInvestConfigs,
  createLinkToken,
  invest,
} from "./api";

const BASE = "http://localhost:8080";

// jsonResponse builds a minimal Response-like object good enough for api.ts,
// which only reads .ok, .status, and .json().
function jsonResponse(body: unknown, init?: { ok?: boolean; status?: number }) {
  return {
    ok: init?.ok ?? true,
    status: init?.status ?? 200,
    json: async () => body,
  } as Response;
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  // Default: a token is available.
  setTokenFetcher(async () => "test-token");
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("auth headers", () => {
  it("attaches the bearer token returned by the token fetcher", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ total_budget: 0, allocations: [] }));

    await getRecommendation({ base_budget: 100, extra_money: 0 });

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers.Authorization).toBe("Bearer test-token");
  });

  it("omits the Authorization header when no token is available", async () => {
    setTokenFetcher(async () => null);
    fetchMock.mockResolvedValue(jsonResponse({}));

    await getProfile();

    const [, init] = fetchMock.mock.calls[0];
    expect(init.headers.Authorization).toBeUndefined();
  });
});

describe("getRecommendation", () => {
  it("POSTs the request body to /recommend with JSON content type", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ total_budget: 100, allocations: [] }));

    const req = { base_budget: 100, extra_money: 25 };
    await getRecommendation(req);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}/recommend`);
    expect(init.method).toBe("POST");
    expect(init.headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(init.body)).toEqual(req);
  });

  it("throws API error with the status code on a non-ok response", async () => {
    fetchMock.mockResolvedValue(jsonResponse({}, { ok: false, status: 500 }));

    await expect(getRecommendation({ base_budget: 1, extra_money: 0 })).rejects.toThrow(
      "API error: 500",
    );
  });
});

describe("getProfile", () => {
  it("maps a 404 to the not_found sentinel error", async () => {
    fetchMock.mockResolvedValue(jsonResponse(null, { ok: false, status: 404 }));

    await expect(getProfile()).rejects.toThrow("not_found");
  });

  it("throws a generic API error for other failures", async () => {
    fetchMock.mockResolvedValue(jsonResponse(null, { ok: false, status: 503 }));

    await expect(getProfile()).rejects.toThrow("API error: 503");
  });

  it("returns the parsed profile on success", async () => {
    const profile = { full_name: "Ada", salary: 1 };
    fetchMock.mockResolvedValue(jsonResponse(profile));

    await expect(getProfile()).resolves.toEqual(profile);
  });
});

describe("getAutoInvestConfigs", () => {
  it("coalesces a null body to an empty array", async () => {
    fetchMock.mockResolvedValue(jsonResponse(null));

    await expect(getAutoInvestConfigs()).resolves.toEqual([]);
  });

  it("returns the configs array when present", async () => {
    const configs = [{ id: "1", enabled: true, amount: 10, risk: "moderate" }];
    fetchMock.mockResolvedValue(jsonResponse(configs));

    await expect(getAutoInvestConfigs()).resolves.toEqual(configs);
  });
});

describe("createLinkToken", () => {
  it("unwraps the link_token field from the response", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ link_token: "link-sandbox-abc" }));

    await expect(createLinkToken()).resolves.toBe("link-sandbox-abc");
  });
});

describe("invest", () => {
  it("serializes the invest request and returns receipts", async () => {
    const response = { receipts: [{ ticker: "VTI" }], decision_id: "d1" };
    fetchMock.mockResolvedValue(jsonResponse(response));

    const req = {
      allocations: [],
      total_amount: 100,
      risk_level: "moderate",
      summary: "s",
    };
    const got = await invest(req);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe(`${BASE}/invest`);
    expect(JSON.parse(init.body)).toEqual(req);
    expect(got).toEqual(response);
  });
});
