// pages/Eval.test.tsx
//
// Reading this file explains what the Activity (Eval) page shows users:
// - Total invested comes from the Activity API, not the eval API
// - Verdict stats (win rate, avg return) only appear once verdicts exist
// - Decision history shows all decisions; verdicts are overlaid where available
// - By-strategy section only appears when there are 2+ distinct strategy names
// - Same-display-name configs (different IDs) are merged with weighted averages
// - A config ID with no matching config shows as "Deleted strategy"
//
// Each test name is a sentence stating a requirement.
import { render, screen, waitFor } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { Eval, fmtPct, configName } from "./Eval";
import type { AutoInvestConfig } from "../services/api";

// Mock the entire api module so tests control what each fetch returns.
// getWinRateTrend / getAssetClassBreakdown are called with .catch(() => []) in
// the component, so they must return a promise — default them to resolve [].
vi.mock("../services/api", () => ({
  getEvalSummary: vi.fn(),
  getEvalDecisions: vi.fn(),
  getActivity: vi.fn(),
  getWinRateTrend: vi.fn(() => Promise.resolve([])),
  getAssetClassBreakdown: vi.fn(() => Promise.resolve([])),
}));

import { getEvalSummary, getEvalDecisions, getActivity } from "../services/api";

// ---------- shared test data factories ----------

function makeSummary(overrides = {}) {
  return {
    total_decisions: 5,
    verdicted_decisions: 3,
    win_rate: 0.67,
    avg_return_pct: 2.1,
    avg_spy_return_pct: 1.0,
    by_strategy: [],
    ...overrides,
  };
}

function makeActivity(overrides = {}) {
  return {
    total_decisions: 5,
    total_invested: 500,
    decisions: [],
    ...overrides,
  };
}

function makeVerdict(overrides = {}) {
  return {
    stamped_at: "2026-05-15T10:00:00Z",
    overall_return_pct: 2.1,
    spy_return_pct: 1.0,
    beat_market: true,
    ticker_verdicts: [],
    ...overrides,
  };
}

function makeEvalDecision(id: string, overrides = {}) {
  return {
    id,
    timestamp: "2026-05-15T10:00:00Z",
    total_amount: 100,
    risk_level: "moderate",
    verdict: makeVerdict(),
    ...overrides,
  };
}

function makeActivityDecision(id: string, overrides = {}) {
  return {
    id,
    timestamp: "2026-05-15T10:00:00Z",
    total_amount: 100,
    risk_level: "moderate",
    ...overrides,
  };
}

const noConfigs: AutoInvestConfig[] = [];

// ---------- describe fmtPct ----------

describe("fmtPct", () => {
  it("formats_positive_return_with_plus_sign_when_forceSign_is_true", () => {
    expect(fmtPct(1.23, true)).toBe("+1.23%");
  });

  it("formats_negative_return_with_minus_sign", () => {
    expect(fmtPct(-4.56)).toBe("-4.56%");
    expect(fmtPct(-4.56, true)).toBe("-4.56%");
  });

  it("formats_zero_with_no_plus_sign_even_when_forceSign_is_true", () => {
    expect(fmtPct(0, true)).toBe("0.00%");
  });

  it("rounds_to_two_decimal_places", () => {
    expect(fmtPct(1.2345)).toBe("1.23%");
    expect(fmtPct(1.2355)).toBe("1.24%");
  });
});

// ---------- describe configName ----------

describe("configName", () => {
  const configs: AutoInvestConfig[] = [
    {
      id: "abc123",
      name: "Long Term",
      enabled: true,
      amount: 100,
      risk: "moderate",
      strategy: "long_term",
      interval_days: 1,
    },
  ];

  it("returns_Manual_when_config_id_is_undefined", () => {
    expect(configName(undefined, configs)).toBe("Manual");
  });

  it("returns_Manual_when_config_id_is_null", () => {
    // @ts-expect-error testing null explicitly (legacy data from MongoDB)
    expect(configName(null, configs)).toBe("Manual");
  });

  it("returns_Manual_when_config_id_is_the_string_manual", () => {
    expect(configName("manual", configs)).toBe("Manual");
  });

  it("returns_Manual_when_config_id_is_empty_string_legacy_decisions", () => {
    // Pre-Phase-19 decisions have config_id="" in MongoDB.
    // These should show as "Manual", not "" or "Legacy".
    expect(configName("", configs)).toBe("Manual");
  });

  it("returns_config_name_when_id_matches_a_loaded_config", () => {
    expect(configName("abc123", configs)).toBe("Long Term");
  });

  it("returns_Deleted_strategy_when_id_does_not_match_any_config", () => {
    // Config was deleted after decisions were made.
    // Must not show raw hex ID like "6a07700c".
    expect(configName("6a07700cdeadbeef", noConfigs)).toBe("Deleted strategy");
  });
});

// ---------- describe Eval component ----------

describe("Eval component", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows_loading_state_while_api_calls_are_in_flight", () => {
    // Never-resolving promises = perpetual loading state.
    vi.mocked(getEvalSummary).mockReturnValue(new Promise(() => {}));
    vi.mocked(getEvalDecisions).mockReturnValue(new Promise(() => {}));
    vi.mocked(getActivity).mockReturnValue(new Promise(() => {}));

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });

  it("shows_error_message_when_any_api_call_rejects", async () => {
    vi.mocked(getEvalSummary).mockRejectedValue(new Error("network failure"));
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText(/eval\/summary: network failure/)).toBeInTheDocument();
    });
  });

  it("shows_total_invested_from_activity_api_even_when_no_verdicts_exist", async () => {
    // Total invested comes from the Activity API (not eval),
    // so it shows even before any verdicts have been stamped.
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary({ verdicted_decisions: 0 }));
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity({ total_invested: 1500 }));

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText("$1,500")).toBeInTheDocument();
    });
  });

  it("shows_verdicts_appear_message_when_verdicted_decisions_is_zero", async () => {
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary({ verdicted_decisions: 0 }));
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText("Verdicts appear after trades settle.")).toBeInTheDocument();
    });
  });

  it("shows_avg_return_and_win_rate_bar_when_verdicts_exist", async () => {
    vi.mocked(getEvalSummary).mockResolvedValue(
      makeSummary({ verdicted_decisions: 3, avg_return_pct: 2.1, win_rate: 0.67 })
    );
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText("+2.10%")).toBeInTheDocument();
      expect(screen.getByText("67%")).toBeInTheDocument();
    });
  });

  it("colors_avg_return_green_for_positive_return", async () => {
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary({ avg_return_pct: 3.5 }));
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      const el = screen.getByText("+3.50%");
      expect(el).toHaveStyle({ color: "#27ae60" });
    });
  });

  it("colors_avg_return_red_for_negative_return", async () => {
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary({ avg_return_pct: -1.2 }));
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      const el = screen.getByText("-1.20%");
      expect(el).toHaveStyle({ color: "#c0392b" });
    });
  });

  it("hides_by_strategy_section_when_only_one_distinct_strategy_name_exists", async () => {
    // Single strategy → no useful comparison → section hidden.
    vi.mocked(getEvalSummary).mockResolvedValue(
      makeSummary({
        by_strategy: [{ config_id: "abc123", win_rate: 0.7, avg_return_pct: 2.0, decision_count: 5 }],
      })
    );
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.queryByText("By strategy")).not.toBeInTheDocument();
    });
  });

  it("shows_by_strategy_section_when_multiple_distinct_strategy_names_exist", async () => {
    const configs: AutoInvestConfig[] = [
      { id: "cfg1", name: "Long Term", enabled: true, amount: 100, risk: "moderate", strategy: "long_term", interval_days: 1 },
      { id: "cfg2", name: "Short Term", enabled: true, amount: 50, risk: "aggressive", strategy: "short_term", interval_days: 1 },
    ];
    vi.mocked(getEvalSummary).mockResolvedValue(
      makeSummary({
        by_strategy: [
          { config_id: "cfg1", win_rate: 0.8, avg_return_pct: 3.0, decision_count: 5 },
          { config_id: "cfg2", win_rate: 0.4, avg_return_pct: -0.5, decision_count: 3 },
        ],
      })
    );
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={configs} />);
    await waitFor(() => {
      expect(screen.getByText("By strategy")).toBeInTheDocument();
      expect(screen.getByText("Long Term")).toBeInTheDocument();
      expect(screen.getByText("Short Term")).toBeInTheDocument();
    });
  });

  it("merges_two_configs_with_same_display_name_into_one_row_with_weighted_average", async () => {
    // Two configs named "Long Term" (different IDs, e.g. user recreated the strategy).
    // They should appear as ONE row with weighted-averaged win_rate and avg_return_pct.
    // cfg-a: 4 decisions, win_rate=1.0, avg_return=4%
    // cfg-b: 6 decisions, win_rate=0.5, avg_return=2%
    // Merged: 10 decisions, win_rate=(1.0*4 + 0.5*6)/10=0.7, avg_return=(4*4 + 2*6)/10=2.8%
    const configs: AutoInvestConfig[] = [
      { id: "cfg-a", name: "Long Term", enabled: true, amount: 100, risk: "moderate", strategy: "long_term", interval_days: 1 },
      { id: "cfg-b", name: "Long Term", enabled: true, amount: 100, risk: "moderate", strategy: "long_term", interval_days: 1 },
      { id: "cfg-c", name: "Short Term", enabled: true, amount: 50, risk: "aggressive", strategy: "short_term", interval_days: 1 },
    ];
    vi.mocked(getEvalSummary).mockResolvedValue(
      makeSummary({
        by_strategy: [
          { config_id: "cfg-a", win_rate: 1.0, avg_return_pct: 4.0, decision_count: 4 },
          { config_id: "cfg-b", win_rate: 0.5, avg_return_pct: 2.0, decision_count: 6 },
          { config_id: "cfg-c", win_rate: 0.3, avg_return_pct: -1.0, decision_count: 4 },
        ],
      })
    );
    vi.mocked(getEvalDecisions).mockResolvedValue([]);
    vi.mocked(getActivity).mockResolvedValue(makeActivity());

    render(<Eval onBack={() => {}} autoInvestConfigs={configs} />);
    await waitFor(() => {
      // Only two rows: merged "Long Term" + "Short Term"
      const rows = screen.getAllByText("Long Term");
      expect(rows).toHaveLength(1);
      // Merged win rate: 70%
      expect(screen.getByText("70% win")).toBeInTheDocument();
    });
  });

  it("shows_too_young_label_for_decisions_without_a_verdict", async () => {
    // Activity API returns a decision that has no matching entry in the eval API.
    // The decision was made too recently for a verdict to be stamped yet.
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary());
    vi.mocked(getEvalDecisions).mockResolvedValue([]); // no verdicts
    vi.mocked(getActivity).mockResolvedValue(
      makeActivity({ decisions: [makeActivityDecision("dec-no-verdict")] })
    );

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText("Too young to rank")).toBeInTheDocument();
    });
  });

  it("shows_return_pct_and_beat_SPY_label_for_verdicted_decisions", async () => {
    const verdict = makeVerdict({ overall_return_pct: 3.5, spy_return_pct: 1.2, beat_market: true });
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary());
    vi.mocked(getEvalDecisions).mockResolvedValue([makeEvalDecision("dec-1", { verdict })]);
    vi.mocked(getActivity).mockResolvedValue(
      makeActivity({ decisions: [makeActivityDecision("dec-1")] })
    );

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText("+3.50%")).toBeInTheDocument();
      expect(screen.getByText("beat SPY")).toBeInTheDocument();
    });
  });

  it("shows_lost_to_SPY_label_when_portfolio_underperformed_benchmark", async () => {
    const verdict = makeVerdict({ overall_return_pct: 0.5, spy_return_pct: 2.0, beat_market: false });
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary());
    vi.mocked(getEvalDecisions).mockResolvedValue([makeEvalDecision("dec-1", { verdict })]);
    vi.mocked(getActivity).mockResolvedValue(
      makeActivity({ decisions: [makeActivityDecision("dec-1")] })
    );

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      expect(screen.getByText("lost to SPY")).toBeInTheDocument();
    });
  });

  it("shows_ticker_pills_with_return_pct_for_verdicted_decisions", async () => {
    const verdict = makeVerdict({
      ticker_verdicts: [
        { ticker: "VTI", return_pct: 2.5, entry_price: 100, prev_day_price: 99, current_price: 102.5, prev_day_timestamp: "", current_timestamp: "", today_change_pct: 0 },
        { ticker: "QQQ", return_pct: -1.2, entry_price: 200, prev_day_price: 198, current_price: 197.6, prev_day_timestamp: "", current_timestamp: "", today_change_pct: 0 },
      ],
    });
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary());
    vi.mocked(getEvalDecisions).mockResolvedValue([makeEvalDecision("dec-1", { verdict })]);
    vi.mocked(getActivity).mockResolvedValue(
      makeActivity({ decisions: [makeActivityDecision("dec-1")] })
    );

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      // Each pill renders "{ticker} {return_pct}" as two text nodes inside one span.
      // Use regex to match the combined text content.
      expect(screen.getByText(/VTI.*\+2\.50%/)).toBeInTheDocument();
      expect(screen.getByText(/QQQ.*-1\.20%/)).toBeInTheDocument();
    });
  });

  it("shows_Deleted_strategy_for_decisions_referencing_a_config_not_in_autoInvestConfigs", async () => {
    // User deleted a strategy after decisions were made with it.
    // Must show "Deleted strategy", not the raw hex config_id.
    vi.mocked(getEvalSummary).mockResolvedValue(makeSummary());
    vi.mocked(getEvalDecisions).mockResolvedValue([
      makeEvalDecision("dec-1", { config_id: "6a07700cdeadbeef" }),
    ]);
    vi.mocked(getActivity).mockResolvedValue(
      makeActivity({ decisions: [makeActivityDecision("dec-1")] })
    );

    render(<Eval onBack={() => {}} autoInvestConfigs={noConfigs} />);
    await waitFor(() => {
      // Text is in " · Deleted strategy" span — use regex to match substring.
      expect(screen.getByText(/Deleted strategy/)).toBeInTheDocument();
      expect(screen.queryByText(/6a07700c/)).not.toBeInTheDocument();
    });
  });
});
