import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, beforeEach, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router";
import { EvidenceModeProvider, useEvidenceMode } from "../../src/hooks/useEvidenceMode";
import { BenchLeaderboard } from "../../src/pages/bench/BenchLeaderboard";
import { BenchDashboard } from "../../src/pages/bench/BenchDashboard";
import { BenchRuns } from "../../src/pages/bench/BenchRuns";
import { BenchRunDetail } from "../../src/pages/bench/BenchRunDetail";

const { requestMock } = vi.hoisted(() => ({
  requestMock: vi.fn(),
}));

vi.mock("../../src/hooks/useApi", () => ({
  useApi: () => ({
    request: requestMock,
  }),
}));

vi.mock("../../src/context/AuthContext", () => ({
  useAuth: () => ({
    apiKey: null,
    setApiKey: vi.fn(),
    clearApiKey: vi.fn(),
  }),
}));

function EvidenceModeProbe() {
  const { mode } = useEvidenceMode();
  return <div data-testid="mode">{mode}</div>;
}

function renderWithEvidenceMode(ui: ReactNode) {
  return render(<EvidenceModeProvider>{ui}</EvidenceModeProvider>);
}

function renderWithRouter(ui: ReactNode, initialEntries = ["/bench"]) {
  return render(
    <EvidenceModeProvider>
      <MemoryRouter initialEntries={initialEntries}>{ui}</MemoryRouter>
    </EvidenceModeProvider>,
  );
}

describe("Bench evidence mode UI", () => {
  beforeEach(() => {
    localStorage.clear();
    requestMock.mockReset();
  });

  it.each([
    ["all", "all"],
    ["none", "none"],
    ["evidra", "evidra"],
  ])("loads persisted mode %s as %s", (stored, expected) => {
    localStorage.setItem("evidra-bench-evidence-mode", stored);

    renderWithEvidenceMode(<EvidenceModeProbe />);

    expect(screen.getByTestId("mode")).toHaveTextContent(expected);
  });

  it.each(["proxy", "smart", "direct"])(
    "normalizes legacy persisted mode %s to evidra",
    (stored) => {
      localStorage.setItem("evidra-bench-evidence-mode", stored);

      renderWithEvidenceMode(<EvidenceModeProbe />);

      expect(screen.getByTestId("mode")).toHaveTextContent("evidra");
    },
  );

  it("shows demo labels on the leaderboard filter buttons and switches requests", async () => {
    requestMock.mockImplementation(async (path: string) => {
      if (path.startsWith("/v1/bench/leaderboard")) {
        return {
          evidence_mode: "",
          models: [],
        };
      }
      throw new Error(`unexpected request: ${path}`);
    });

    renderWithRouter(<BenchLeaderboard />);

    expect(await screen.findByRole("button", { name: "All" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Baseline" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Evidra" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "proxy" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "smart" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "direct" })).not.toBeInTheDocument();

    requestMock.mockClear();

    await userEvent.click(screen.getByRole("button", { name: "Baseline" }));
    await waitFor(() => {
      expect(
        requestMock.mock.calls.some(([path]) => path === "/v1/bench/leaderboard?evidence_mode=none"),
      ).toBe(true);
    });

    requestMock.mockClear();

    await userEvent.click(screen.getByRole("button", { name: "Evidra" }));
    await waitFor(() => {
      expect(
        requestMock.mock.calls.some(([path]) => path === "/v1/bench/leaderboard?evidence_mode=evidra"),
      ).toBe(true);
    });
  });

  it.each([
    ["Baseline", "none"],
    ["Evidra", "smart"],
  ])("submits %s as canonical evidence_mode %s", async (label, expectedMode) => {
    const user = userEvent.setup();

    requestMock.mockImplementation(async (path: string, options?: RequestInit) => {
      if (path.startsWith("/v1/bench/stats")) {
        return {
          total_runs: 0,
          pass_count: 0,
          fail_count: 0,
          by_scenario: [],
        };
      }
      if (path.startsWith("/v1/bench/runs?limit=8") || path.startsWith("/v1/bench/runs?limit=500")) {
        return {
          items: [],
          total: 0,
        };
      }
      if (path === "/v1/bench/scenarios") {
        return {
          scenarios: [
            { id: "scenario-1", title: "Scenario 1", category: "demo" },
          ],
        };
      }
      if (path === "/v1/bench/trigger") {
        return {
          id: "trigger-1",
          status: "completed",
          total: 1,
          completed: 1,
          passed: 1,
          failed: 0,
          current_scenario: "",
          progress: [],
        };
      }
      if (path.startsWith("/v1/bench/trigger/")) {
        return {
          id: "trigger-1",
          status: "completed",
          total: 1,
          completed: 1,
          passed: 1,
          failed: 0,
          current_scenario: "",
          progress: [],
        };
      }
      throw new Error(`unexpected request: ${path}`);
    });

    renderWithRouter(<BenchDashboard />);

    await user.click(screen.getByRole("button", { name: "Run Benchmark" }));
    await screen.findByRole("checkbox", { name: "Scenario 1" });
    expect(screen.getByRole("radio", { name: "Baseline" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Evidra" })).toBeInTheDocument();
    expect(screen.getAllByRole("radio")).toHaveLength(2);

    await user.click(screen.getByRole("radio", { name: label }));
    await user.click(screen.getByRole("button", { name: "Start" }));

    await waitFor(() => {
      expect(requestMock).toHaveBeenCalledWith(
        "/v1/bench/trigger",
        expect.objectContaining({ method: "POST" }),
      );
    });

    const triggerCall = requestMock.mock.calls.find(([path]) => path === "/v1/bench/trigger");
    expect(triggerCall).toBeDefined();

    const [, options] = triggerCall ?? [];
    expect(options).toMatchObject({ method: "POST" });
    expect(JSON.parse((options as RequestInit).body as string)).toEqual(
      expect.objectContaining({
        evidence_mode: expectedMode,
      }),
    );
  });

  it("renders exact subtype labels in the runs table", async () => {
    requestMock.mockImplementation(async (path: string) => {
      if (path === "/v1/bench/catalog") {
        return {
          models: ["gpt-4o"],
          providers: ["openai"],
        };
      }
      if (path.startsWith("/v1/bench/runs?")) {
        return {
          items: [
            {
              id: "baseline-1",
              scenario_id: "scenario-baseline",
              model: "gpt-4o",
              provider: "openai",
              adapter: "default",
              evidence_mode: "none",
              passed: true,
              duration_seconds: 12.3,
              exit_code: 0,
              turns: 3,
              memory_window: 2,
              prompt_tokens: 100,
              completion_tokens: 50,
              estimated_cost_usd: 0.01,
              checks_passed: 1,
              checks_total: 1,
              created_at: "2026-03-26T00:00:00.000Z",
            },
            {
              id: "smart-1",
              scenario_id: "scenario-smart",
              model: "gpt-4o",
              provider: "openai",
              adapter: "default",
              evidence_mode: "smart",
              passed: true,
              duration_seconds: 13.3,
              exit_code: 0,
              turns: 4,
              memory_window: 2,
              prompt_tokens: 100,
              completion_tokens: 50,
              estimated_cost_usd: 0.02,
              checks_passed: 1,
              checks_total: 1,
              created_at: "2026-03-26T00:00:00.000Z",
            },
            {
              id: "proxy-1",
              scenario_id: "scenario-proxy",
              model: "gpt-4o",
              provider: "openai",
              adapter: "default",
              evidence_mode: "proxy",
              passed: false,
              duration_seconds: 14.3,
              exit_code: 1,
              turns: 5,
              memory_window: 2,
              prompt_tokens: 100,
              completion_tokens: 50,
              estimated_cost_usd: 0.03,
              checks_passed: 0,
              checks_total: 1,
              created_at: "2026-03-26T00:00:00.000Z",
            },
            {
              id: "direct-1",
              scenario_id: "scenario-direct",
              model: "gpt-4o",
              provider: "openai",
              adapter: "default",
              evidence_mode: "direct",
              passed: false,
              duration_seconds: 15.3,
              exit_code: 1,
              turns: 6,
              memory_window: 2,
              prompt_tokens: 100,
              completion_tokens: 50,
              estimated_cost_usd: 0.04,
              checks_passed: 0,
              checks_total: 1,
              created_at: "2026-03-26T00:00:00.000Z",
            },
          ],
          total: 4,
          limit: 25,
          offset: 0,
        };
      }
      throw new Error(`unexpected request: ${path}`);
    });

    renderWithRouter(<BenchRuns />);

    expect(await screen.findByText("Baseline")).toBeInTheDocument();
    expect(screen.getByText("Evidra Smart")).toBeInTheDocument();
    expect(screen.getByText("Evidra Proxy")).toBeInTheDocument();
    expect(screen.getByText("Evidra Direct")).toBeInTheDocument();
  });

  it("renders the run detail subtype badge with the friendly label", async () => {
    requestMock.mockImplementation(async (path: string) => {
      if (path.startsWith("/v1/bench/runs/run-smart")) {
        return {
          id: "run-smart",
          scenario_id: "scenario-smart",
          model: "gpt-4o",
          provider: "openai",
          passed: true,
          duration_seconds: 9.4,
          turns: 4,
          prompt_tokens: 100,
          completion_tokens: 50,
          estimated_cost_usd: 0.02,
          exit_code: 0,
          checks_passed: 1,
          checks_total: 1,
          checks_json: "",
          created_at: "2026-03-26T00:00:00.000Z",
          evidence_mode: "smart",
        };
      }
      throw new Error(`unexpected request: ${path}`);
    });

    render(
      <EvidenceModeProvider>
        <MemoryRouter initialEntries={["/bench/runs/run-smart"]}>
          <Routes>
            <Route path="/bench/runs/:id" element={<BenchRunDetail />} />
          </Routes>
        </MemoryRouter>
      </EvidenceModeProvider>,
    );

    expect(await screen.findByText("Evidra Smart")).toBeInTheDocument();
  });
});
