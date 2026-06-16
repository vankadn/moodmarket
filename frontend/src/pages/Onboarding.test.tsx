// pages/Onboarding.test.tsx
//
// Onboarding is the profile-capture flow. These tests pin the contract that
// matters to a user finishing setup:
//  - filled fields are submitted to saveProfile and the saved profile is handed
//    back via onComplete
//  - the emergency-fund toggle flips the value it submits
//  - a save failure surfaces the error and does NOT advance (onComplete unused)
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { Onboarding } from "./Onboarding";

vi.mock("../services/api", () => ({
  saveProfile: vi.fn(),
}));

import { saveProfile } from "../services/api";

const mockedSaveProfile = vi.mocked(saveProfile);

// fillRequiredFields populates every field with a `required` attribute so the
// form will actually submit under jsdom's constraint validation.
async function fillRequiredFields(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByPlaceholderText("Jane Smith"), "Jane Smith");
  await user.type(screen.getByPlaceholderText("85000"), "85000");
  await user.type(screen.getByPlaceholderText("1500"), "1500");
  await user.type(screen.getByPlaceholderText("6"), "6");
  await user.type(screen.getByPlaceholderText("25000"), "25000");
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("Onboarding", () => {
  it("submits the filled profile and calls onComplete with the saved profile", async () => {
    const user = userEvent.setup();
    const saved = { full_name: "Jane Smith", salary: 85000 };
    mockedSaveProfile.mockResolvedValue(saved as never);
    const onComplete = vi.fn();

    render(<Onboarding onComplete={onComplete} />);
    await fillRequiredFields(user);
    await user.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => expect(mockedSaveProfile).toHaveBeenCalledTimes(1));
    const submitted = mockedSaveProfile.mock.calls[0][0];
    expect(submitted.full_name).toBe("Jane Smith");
    expect(submitted.salary).toBe(85000);
    expect(submitted.monthly_savings).toBe(1500);
    // Defaults flow through untouched.
    expect(submitted.risk_tolerance).toBe("moderate");
    expect(submitted.has_emergency_fund).toBe(false);

    await waitFor(() => expect(onComplete).toHaveBeenCalledWith(saved));
  });

  it("submits has_emergency_fund=true after toggling the switch", async () => {
    const user = userEvent.setup();
    mockedSaveProfile.mockResolvedValue({} as never);

    render(<Onboarding onComplete={vi.fn()} />);
    await fillRequiredFields(user);

    const toggle = screen.getByRole("switch");
    expect(toggle).toHaveAttribute("aria-checked", "false");
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-checked", "true");

    await user.click(screen.getByRole("button", { name: /continue/i }));

    await waitFor(() => expect(mockedSaveProfile).toHaveBeenCalledTimes(1));
    expect(mockedSaveProfile.mock.calls[0][0].has_emergency_fund).toBe(true);
  });

  it("shows the error and does not advance when saving fails", async () => {
    const user = userEvent.setup();
    mockedSaveProfile.mockRejectedValue(new Error("server exploded"));
    const onComplete = vi.fn();

    render(<Onboarding onComplete={onComplete} />);
    await fillRequiredFields(user);
    await user.click(screen.getByRole("button", { name: /continue/i }));

    expect(await screen.findByText("server exploded")).toBeInTheDocument();
    expect(onComplete).not.toHaveBeenCalled();
  });
});
