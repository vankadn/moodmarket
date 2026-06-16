// pages/AutoInvestSettings.test.tsx
//
// AutoInvestSettings is the create/edit/delete flow for an auto-invest strategy.
// These tests pin which API call each action makes and that navigation only
// happens on success:
//  - no initialConfig → "Create strategy" calls createAutoInvestConfig
//  - initialConfig with id → "Save changes" calls updateAutoInvestConfig
//  - delete is two-step (confirm) and calls deleteAutoInvestConfig
//  - a save failure shows the error and stays on the page
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { AutoInvestSettings } from "./AutoInvestSettings";
import type { AutoInvestConfig } from "../services/api";

vi.mock("../services/api", () => ({
  getProfile: vi.fn(() => Promise.resolve({ include_cash_context: false })),
  saveProfile: vi.fn(() => Promise.resolve(null)),
  createAutoInvestConfig: vi.fn(() => Promise.resolve({})),
  updateAutoInvestConfig: vi.fn(() => Promise.resolve({})),
  deleteAutoInvestConfig: vi.fn(() => Promise.resolve()),
}));

import {
  createAutoInvestConfig,
  updateAutoInvestConfig,
  deleteAutoInvestConfig,
} from "../services/api";

const mockedCreate = vi.mocked(createAutoInvestConfig);
const mockedUpdate = vi.mocked(updateAutoInvestConfig);
const mockedDelete = vi.mocked(deleteAutoInvestConfig);

beforeEach(() => {
  vi.clearAllMocks();
});

describe("AutoInvestSettings", () => {
  it("creates a new strategy and navigates back on success", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();

    render(<AutoInvestSettings onBack={onBack} />);
    await user.click(screen.getByRole("button", { name: /create strategy/i }));

    await waitFor(() => expect(mockedCreate).toHaveBeenCalledTimes(1));
    expect(mockedUpdate).not.toHaveBeenCalled();
    // Defaults are forwarded; interval normalized on save.
    const sent = mockedCreate.mock.calls[0][0];
    expect(sent.amount).toBe(100);
    expect(sent.interval_seconds).toBe(0);
    await waitFor(() => expect(onBack).toHaveBeenCalled());
  });

  it("updates an existing strategy via updateAutoInvestConfig when editing", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    const initial: AutoInvestConfig = {
      id: "cfg-7",
      enabled: true,
      mode: "fixed",
      amount: 250,
      risk: "aggressive",
      interval_hours: 24,
      name: "My Strategy",
    };

    render(<AutoInvestSettings initialConfig={initial} onBack={onBack} />);
    await user.click(screen.getByRole("button", { name: /save changes/i }));

    await waitFor(() => expect(mockedUpdate).toHaveBeenCalledTimes(1));
    expect(mockedUpdate.mock.calls[0][0]).toBe("cfg-7");
    expect(mockedCreate).not.toHaveBeenCalled();
    await waitFor(() => expect(onBack).toHaveBeenCalled());
  });

  it("requires a confirmation step before deleting", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    const initial: AutoInvestConfig = {
      id: "cfg-7",
      enabled: true,
      mode: "fixed",
      amount: 100,
      risk: "moderate",
      interval_hours: 24,
    };

    render(<AutoInvestSettings initialConfig={initial} onBack={onBack} />);

    // First click only reveals the confirm prompt — no delete call yet.
    await user.click(screen.getByRole("button", { name: /delete strategy/i }));
    expect(mockedDelete).not.toHaveBeenCalled();
    expect(screen.getByText(/cannot be undone/i)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /yes, delete/i }));
    await waitFor(() => expect(mockedDelete).toHaveBeenCalledWith("cfg-7"));
    await waitFor(() => expect(onBack).toHaveBeenCalled());
  });

  it("shows an error and stays on the page when saving fails", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();
    mockedCreate.mockRejectedValueOnce(new Error("boom"));

    render(<AutoInvestSettings onBack={onBack} />);
    await user.click(screen.getByRole("button", { name: /create strategy/i }));

    expect(await screen.findByText(/failed to save settings/i)).toBeInTheDocument();
    expect(onBack).not.toHaveBeenCalled();
  });
});
