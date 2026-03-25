import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { Link } from "react-router";
import { useApi } from "../../hooks/useApi";
import { buildRunsPath, evidenceModeParam } from "../../lib/catalogData.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";

/* ── Types ── */

type Period = "24h" | "7d" | "30d" | "90d" | "all";

type ScenarioStatus = "pending" | "running" | "passed" | "failed";

interface ScenarioSummary {
  id: string;
  title: string;
  category: string;
}

interface TriggerProgress {
  id: string;
  status: string;
  scenarios: Record<string, ScenarioStatus>;
}

interface ScenarioStat {
  scenario_id: string;
  runs: number;
  passed: number;
}

interface Stats {
  total_runs: number;
  pass_count: number;
  fail_count: number;
  by_scenario: ScenarioStat[];
}

interface Run {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  passed: boolean;
  duration_seconds: number;
  estimated_cost_usd: number;
  prompt_tokens: number;
  completion_tokens: number;
  created_at: string;
}

interface RunsResponse {
  items: Run[];
  total: number;
}

/* ── Helpers ── */

const PERIODS: { value: Period; label: string }[] = [
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "90d", label: "90d" },
  { value: "all", label: "All" },
];

const PERIOD_MS: Record<Exclude<Period, "all">, number> = {
  "24h": 86_400_000,
  "7d": 7 * 86_400_000,
  "30d": 30 * 86_400_000,
  "90d": 90 * 86_400_000,
};

function periodToSince(p: Period): string | undefined {
  if (p === "all") return undefined;
  return new Date(Date.now() - PERIOD_MS[p]).toISOString();
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const day = String(d.getDate()).padStart(2, "0");
  const mon = d.toLocaleString("en-US", { month: "short" });
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${day} ${mon} ${hh}:${mm}`;
}

function formatDateShort(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return `${d.getDate()} ${d.toLocaleString("en-US", { month: "short" })}`;
}

function formatDuration(s: number): string {
  return `${s.toFixed(1)}s`;
}

function formatCost(usd: number): string {
  if (usd < 0.001) return "$0.00";
  return `$${usd.toFixed(3)}`;
}

function formatTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

/* ── Skeleton pulse ── */

function Pulse({ className = "" }: { className?: string }) {
  return (
    <div
      className={`animate-pulse rounded bg-border-subtle ${className}`}
    />
  );
}

/* ── Component ── */

export function BenchDashboard() {
  const { request } = useApi();
  const { mode } = useEvidenceMode();
  const [period, setPeriod] = useState<Period>("7d");
  const [stats, setStats] = useState<Stats | null>(null);
  const [recentRuns, setRecentRuns] = useState<Run[]>([]);
  const [allRuns, setAllRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);

  /* ── Scenarios catalog ── */
  const [scenarios, setScenarios] = useState<ScenarioSummary[]>([]);

  /* ── Trigger state ── */
  const [showTriggerModal, setShowTriggerModal] = useState(false);
  const [triggerModel, setTriggerModel] = useState("deepseek-chat");
  const [triggerProvider, setTriggerProvider] = useState("deepseek");
  const [triggerScenarios, setTriggerScenarios] = useState<Set<string>>(
    new Set(),
  );
  const [triggerProgress, setTriggerProgress] = useState<TriggerProgress | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchData = useCallback(() => {
    const since = periodToSince(period);
    const modeFirst = evidenceModeParam("?", mode);
    const modeAmp = evidenceModeParam("&", mode);
    const sinceAmp = since ? `&since=${encodeURIComponent(since)}` : "";
    const sinceParam = since ? `&since=${encodeURIComponent(since)}` : "";

    setLoading(true);
    Promise.all([
      request<Stats>(`/v1/bench/stats${modeFirst}${sinceAmp}`),
      request<RunsResponse>(buildRunsPath(8, since, mode)),
      request<RunsResponse>(`/v1/bench/runs?limit=500${modeAmp}${sinceParam}`),
    ])
      .then(([s, recent, all]) => {
        setStats(s);
        setRecentRuns(recent.items ?? []);
        setAllRuns(all.items ?? []);
      })
      .catch(() => {
        setStats(null);
        setRecentRuns([]);
        setAllRuns([]);
      })
      .finally(() => setLoading(false));
  }, [period, request, mode]);

  useEffect(() => {
    fetchData();
    // Fetch scenario catalog once.
    request<{ scenarios: ScenarioSummary[] }>("/v1/bench/scenarios")
      .then((res) => {
        const list = res.scenarios ?? [];
        setScenarios(list);
        setTriggerScenarios(new Set(list.map((s) => s.id)));
      })
      .catch(() => {});
  }, [fetchData, request]);

  /* ── Trigger actions ── */

  function toggleScenario(s: string) {
    setTriggerScenarios((prev) => {
      const next = new Set(prev);
      if (next.has(s)) next.delete(s);
      else next.add(s);
      return next;
    });
  }

  async function startTrigger() {
    const scenarios = [...triggerScenarios];
    if (scenarios.length === 0) return;
    setShowTriggerModal(false);
    try {
      const res = await request<TriggerProgress>("/v1/bench/trigger", {
        method: "POST",
        body: JSON.stringify({
          model: triggerModel,
          provider: triggerProvider,
          scenarios,
        }),
      });
      setTriggerProgress(res);
      startPolling(res.id);
    } catch {
      // trigger failed — nothing to poll
    }
  }

  function startPolling(id: string) {
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      try {
        const snap = await request<TriggerProgress>(`/v1/bench/trigger/${id}`);
        setTriggerProgress(snap);
        if (snap.status === "completed" || snap.status === "failed") {
          if (pollRef.current) clearInterval(pollRef.current);
          pollRef.current = null;
          fetchData();
        }
      } catch {
        if (pollRef.current) clearInterval(pollRef.current);
        pollRef.current = null;
      }
    }, 2000);
  }

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, []);

  /* Derived data */
  const passRate =
    stats && stats.total_runs > 0
      ? ((stats.pass_count / stats.total_runs) * 100).toFixed(1)
      : "0.0";

  const distinctModels = useMemo(
    () => [...new Set(allRuns.map((r) => r.model).filter(Boolean))],
    [allRuns],
  );

  const totalCost = useMemo(
    () => allRuns.reduce((sum, r) => sum + (r.estimated_cost_usd || 0), 0),
    [allRuns],
  );

  const totalTokens = useMemo(
    () =>
      allRuns.reduce(
        (sum, r) => sum + (r.prompt_tokens || 0) + (r.completion_tokens || 0),
        0,
      ),
    [allRuns],
  );

  const modelPassRates = useMemo(() => {
    const map = new Map<string, { total: number; passed: number; cost: number }>();
    allRuns.forEach((r) => {
      if (!r.model) return;
      const entry = map.get(r.model) ?? { total: 0, passed: 0, cost: 0 };
      entry.total += 1;
      if (r.passed) entry.passed += 1;
      entry.cost += r.estimated_cost_usd || 0;
      map.set(r.model, entry);
    });
    return [...map.entries()]
      .map(([model, { total, passed, cost }]) => ({
        model,
        rate: total > 0 ? Math.round((passed / total) * 100) : 0,
        total,
        passed,
        cost,
      }))
      .sort((a, b) => b.rate - a.rate);
  }, [allRuns]);

  // Activity: group runs by day
  const dailyActivity = useMemo(() => {
    const map = new Map<string, { pass: number; fail: number }>();
    allRuns.forEach((r) => {
      const d = new Date(r.created_at);
      if (isNaN(d.getTime())) return;
      const key = d.toISOString().slice(0, 10);
      const entry = map.get(key) ?? { pass: 0, fail: 0 };
      if (r.passed) entry.pass += 1;
      else entry.fail += 1;
      map.set(key, entry);
    });
    return [...map.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .slice(-14)
      .map(([date, counts]) => ({ date, ...counts, total: counts.pass + counts.fail }));
  }, [allRuns]);

  const maxDailyTotal = useMemo(
    () => Math.max(1, ...dailyActivity.map((d) => d.total)),
    [dailyActivity],
  );

  // Worst scenarios (lowest pass rate with >= 2 runs)
  const worstScenarios = useMemo(() => {
    if (!stats?.by_scenario) return [];
    return [...stats.by_scenario]
      .filter((s) => s.runs >= 2)
      .map((s) => ({
        ...s,
        rate: Math.round((s.passed / s.runs) * 100),
      }))
      .sort((a, b) => a.rate - b.rate)
      .slice(0, 5);
  }, [stats]);

  // Best scenarios
  const bestScenarios = useMemo(() => {
    if (!stats?.by_scenario) return [];
    return [...stats.by_scenario]
      .filter((s) => s.runs >= 2)
      .map((s) => ({
        ...s,
        rate: Math.round((s.passed / s.runs) * 100),
      }))
      .sort((a, b) => b.rate - a.rate)
      .slice(0, 5);
  }, [stats]);

  /* ── Render ── */

  return (
    <div className="max-w-[980px] mx-auto px-8 py-10 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
            Bench Dashboard
          </h1>
          <p className="text-[0.85rem] text-fg-muted mt-0.5">
            Infrastructure agent benchmark overview
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            disabled={!!triggerProgress && triggerProgress.status !== "completed" && triggerProgress.status !== "failed"}
            onClick={() => setShowTriggerModal(true)}
            className="text-[0.78rem] font-semibold px-3 py-1.5 rounded-md bg-accent text-white hover:opacity-90 transition-opacity disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
          >
            Run Benchmark
          </button>
          <div className="flex gap-0 border border-border rounded-md overflow-hidden">
            {PERIODS.map(({ value, label }) => (
              <button
                key={value}
                onClick={() => setPeriod(value)}
                className={`font-mono text-[0.74rem] px-3 py-1.5 border-r border-border last:border-r-0 cursor-pointer transition-all ${
                  period === value
                    ? "bg-accent-tint text-accent font-semibold"
                    : "bg-bg-elevated text-fg-muted hover:text-fg"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
        {loading ? (
          Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-4"
            >
              <Pulse className="h-3 w-20 mb-3" />
              <Pulse className="h-7 w-16" />
            </div>
          ))
        ) : (
          <>
            <StatCard
              label="Total Runs"
              value={String(stats?.total_runs ?? 0)}
              detail={`${stats?.fail_count ?? 0} failed`}
              borderColor="border-l-accent"
            />
            <StatCard
              label="Pass Rate"
              value={`${passRate}%`}
              detail={`${stats?.pass_count ?? 0} / ${stats?.total_runs ?? 0}`}
              borderColor="border-l-accent"
            />
            <StatCard
              label="Models Tested"
              value={String(distinctModels.length)}
              detail={distinctModels.join(", ")}
              borderColor="border-l-info"
            />
            <StatCard
              label="Total Cost"
              value={`$${totalCost.toFixed(2)}`}
              detail={`${formatTokens(totalTokens)} tokens`}
              borderColor="border-l-warning"
            />
          </>
        )}
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4">
        {/* Recent Runs */}
        <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
          <div className="flex items-center justify-between px-5 pt-4 pb-3">
            <h2 className="text-[0.95rem] font-semibold text-fg">
              Recent Runs
            </h2>
            <Link
              to="/bench/runs"
              className="text-[0.78rem] text-accent hover:text-accent-bright"
            >
              View all &rarr;
            </Link>
          </div>
          {loading ? (
            <div className="px-5 pb-4 space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Pulse key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : recentRuns.length === 0 ? (
            <p className="text-fg-muted text-[0.85rem] py-8 text-center">
              No runs recorded yet.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[0.82rem]">
                <thead>
                  <tr className="border-b border-border bg-bg-alt">
                    {["Status", "Scenario", "Model", "Duration", "Cost", "Date"].map(
                      (h) => (
                        <th
                          key={h}
                          className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2"
                        >
                          {h}
                        </th>
                      ),
                    )}
                  </tr>
                </thead>
                <tbody>
                  {recentRuns.map((run) => (
                    <tr
                      key={run.id}
                      className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors cursor-pointer"
                      onClick={() => (window.location.href = `/bench/runs/${run.id}`)}
                    >
                      <td className="py-2.5 px-4">
                        <span
                          className={`inline-block font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded ${
                            run.passed
                              ? "bg-accent-tint text-accent"
                              : "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
                          }`}
                        >
                          {run.passed ? "PASS" : "FAIL"}
                        </span>
                      </td>
                      <td className="py-2.5 px-4 font-medium text-fg">
                        {run.scenario_id}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-fg-muted text-[0.78rem]">
                        {run.model}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-fg-muted text-[0.78rem]">
                        {formatDuration(run.duration_seconds)}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-fg-muted text-[0.78rem]">
                        {formatCost(run.estimated_cost_usd)}
                      </td>
                      <td className="py-2.5 px-4 text-fg-muted whitespace-nowrap text-[0.78rem]">
                        {formatDate(run.created_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Right column */}
        <div className="space-y-4">
          {/* Pass Rate by Model */}
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-5">
            <h2 className="text-[0.95rem] font-semibold text-fg mb-4">
              Pass Rate by Model
            </h2>
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Pulse key={i} className="h-6 w-full" />
                ))}
              </div>
            ) : modelPassRates.length === 0 ? (
              <p className="text-fg-muted text-[0.82rem] text-center py-4">
                No data available.
              </p>
            ) : (
              <div className="space-y-3">
                {modelPassRates.map(({ model, rate, passed, total, cost }) => (
                  <div key={model}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-mono text-[0.78rem] font-semibold text-fg">
                        {model}
                      </span>
                      <span className="font-mono text-[0.72rem] text-fg-muted">
                        {passed}/{total} &middot; {formatCost(cost)}
                      </span>
                    </div>
                    <div className="h-2 rounded-full bg-bg-alt overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all duration-500 ${
                          rate >= 70
                            ? "bg-accent"
                            : rate >= 40
                              ? "bg-warning"
                              : "bg-danger"
                        }`}
                        style={{ width: `${rate}%` }}
                      />
                    </div>
                    <div className="text-right">
                      <span
                        className={`font-mono text-[0.72rem] font-semibold ${
                          rate >= 70
                            ? "text-accent"
                            : rate >= 40
                              ? "text-warning"
                              : "text-danger"
                        }`}
                      >
                        {rate}%
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Run Activity */}
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-5">
            <h2 className="text-[0.95rem] font-semibold text-fg mb-4">
              Run Activity
            </h2>
            {dailyActivity.length === 0 ? (
              <p className="text-fg-muted text-[0.82rem] text-center py-4">
                No activity data.
              </p>
            ) : (
              <>
                <div className="flex items-end gap-1 h-16">
                  {dailyActivity.map((day) => {
                    const passH = (day.pass / maxDailyTotal) * 100;
                    const failH = (day.fail / maxDailyTotal) * 100;
                    return (
                      <div
                        key={day.date}
                        className="flex-1 flex flex-col justify-end gap-px"
                        title={`${day.date}: ${day.pass} pass, ${day.fail} fail`}
                        style={{ height: "100%" }}
                      >
                        {day.fail > 0 && (
                          <div
                            className="w-full rounded-t-sm bg-danger opacity-70"
                            style={{ height: `${failH}%`, minHeight: day.fail > 0 ? "2px" : 0 }}
                          />
                        )}
                        {day.pass > 0 && (
                          <div
                            className="w-full rounded-t-sm bg-accent"
                            style={{ height: `${passH}%`, minHeight: day.pass > 0 ? "2px" : 0 }}
                          />
                        )}
                      </div>
                    );
                  })}
                </div>
                <div className="flex justify-between mt-1.5">
                  <span className="text-[0.65rem] text-fg-muted">
                    {formatDateShort(dailyActivity[0].date)}
                  </span>
                  <span className="text-[0.65rem] text-fg-muted">
                    {formatDateShort(dailyActivity[dailyActivity.length - 1].date)}
                  </span>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Bottom row: worst + best scenarios */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Hardest scenarios */}
        <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
          <div className="px-5 pt-4 pb-2">
            <h2 className="text-[0.95rem] font-semibold text-fg">
              Hardest Scenarios
            </h2>
            <p className="text-[0.72rem] text-fg-muted">Lowest pass rate (min 2 runs)</p>
          </div>
          {worstScenarios.length === 0 ? (
            <p className="text-fg-muted text-[0.82rem] text-center py-6">
              Not enough data.
            </p>
          ) : (
            <table className="w-full text-[0.82rem]">
              <tbody>
                {worstScenarios.map((s) => (
                  <tr
                    key={s.scenario_id}
                    className="border-t border-border-subtle hover:bg-accent-subtle transition-colors"
                  >
                    <td className="py-2 px-5 font-medium text-fg">
                      <Link to={`/bench/runs?scenario=${s.scenario_id}`} className="text-fg hover:text-accent">
                        {s.scenario_id}
                      </Link>
                    </td>
                    <td className="py-2 px-3 font-mono text-[0.78rem] text-fg-muted text-right">
                      {s.passed}/{s.runs}
                    </td>
                    <td className="py-2 px-5 text-right w-20">
                      <span
                        className={`font-mono text-[0.78rem] font-semibold ${
                          s.rate >= 70
                            ? "text-accent"
                            : s.rate >= 40
                              ? "text-warning"
                              : "text-danger"
                        }`}
                      >
                        {s.rate}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Best scenarios */}
        <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
          <div className="px-5 pt-4 pb-2">
            <h2 className="text-[0.95rem] font-semibold text-fg">
              Easiest Scenarios
            </h2>
            <p className="text-[0.72rem] text-fg-muted">Highest pass rate (min 2 runs)</p>
          </div>
          {bestScenarios.length === 0 ? (
            <p className="text-fg-muted text-[0.82rem] text-center py-6">
              Not enough data.
            </p>
          ) : (
            <table className="w-full text-[0.82rem]">
              <tbody>
                {bestScenarios.map((s) => (
                  <tr
                    key={s.scenario_id}
                    className="border-t border-border-subtle hover:bg-accent-subtle transition-colors"
                  >
                    <td className="py-2 px-5 font-medium text-fg">
                      <Link to={`/bench/runs?scenario=${s.scenario_id}`} className="text-fg hover:text-accent">
                        {s.scenario_id}
                      </Link>
                    </td>
                    <td className="py-2 px-3 font-mono text-[0.78rem] text-fg-muted text-right">
                      {s.passed}/{s.runs}
                    </td>
                    <td className="py-2 px-5 text-right w-20">
                      <span className="font-mono text-[0.78rem] font-semibold text-accent">
                        {s.rate}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {/* ── Trigger Modal ── */}
      {showTriggerModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
          <div className="bg-bg-elevated border border-border rounded-xl shadow-lg w-full max-w-md p-6 space-y-4">
            <h2 className="text-[1.1rem] font-bold text-fg">Run Benchmark</h2>
            <div className="space-y-3">
              <label className="block">
                <span className="text-[0.78rem] font-semibold text-fg-muted">Model</span>
                <input
                  type="text"
                  value={triggerModel}
                  onChange={(e) => setTriggerModel(e.target.value)}
                  className="mt-1 w-full rounded-md border border-border bg-bg px-3 py-1.5 text-[0.85rem] text-fg focus:outline-none focus:ring-1 focus:ring-accent"
                />
              </label>
              <label className="block">
                <span className="text-[0.78rem] font-semibold text-fg-muted">Provider</span>
                <input
                  type="text"
                  value={triggerProvider}
                  onChange={(e) => setTriggerProvider(e.target.value)}
                  className="mt-1 w-full rounded-md border border-border bg-bg px-3 py-1.5 text-[0.85rem] text-fg focus:outline-none focus:ring-1 focus:ring-accent"
                />
              </label>
              <fieldset>
                <legend className="text-[0.78rem] font-semibold text-fg-muted mb-1">Scenarios</legend>
                <div className="space-y-1">
                  {scenarios.length === 0 && (
                    <span className="text-[0.8rem] text-fg-muted">No scenarios found. Sync scenarios via API first.</span>
                  )}
                  {scenarios.map((s) => (
                    <label key={s.id} className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={triggerScenarios.has(s.id)}
                        onChange={() => toggleScenario(s.id)}
                        className="accent-accent"
                      />
                      <span className="text-[0.82rem] text-fg">{s.title || s.id}</span>
                    </label>
                  ))}
                </div>
              </fieldset>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                onClick={() => setShowTriggerModal(false)}
                className="text-[0.82rem] px-4 py-1.5 rounded-md border border-border text-fg-muted hover:text-fg transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={startTrigger}
                disabled={triggerScenarios.size === 0}
                className="text-[0.82rem] font-semibold px-4 py-1.5 rounded-md bg-accent text-white hover:opacity-90 transition-opacity disabled:opacity-40 cursor-pointer"
              >
                Start
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Progress Overlay ── */}
      {triggerProgress && triggerProgress.status !== "completed" && triggerProgress.status !== "failed" && (
        <div className="fixed bottom-6 right-6 z-50 w-80 bg-bg-elevated border border-border rounded-xl shadow-lg p-4 space-y-3">
          <h3 className="text-[0.9rem] font-bold text-fg">Benchmark Running</h3>
          <ul className="space-y-1">
            {Object.entries(triggerProgress.scenarios).map(([name, st]) => (
              <li key={name} className="flex items-center gap-2 text-[0.82rem]">
                <span>
                  {st === "pending" && "\u23F3"}
                  {st === "running" && "\uD83D\uDD04"}
                  {st === "passed" && "\u2705"}
                  {st === "failed" && "\u274C"}
                </span>
                <span className={st === "running" ? "text-fg font-medium" : "text-fg-muted"}>
                  {name}
                </span>
              </li>
            ))}
          </ul>
          <p className="text-[0.72rem] text-fg-muted">
            {Object.values(triggerProgress.scenarios).filter((s) => s === "passed" || s === "failed").length}
            /{Object.keys(triggerProgress.scenarios).length} completed
          </p>
        </div>
      )}
    </div>
  );
}

/* ── StatCard ── */

function StatCard({
  label,
  value,
  detail,
  borderColor,
}: {
  label: string;
  value: string;
  detail?: string;
  borderColor: string;
}) {
  return (
    <div
      className={`bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-4 border-l-[3px] ${borderColor} hover:shadow-[var(--shadow-card-lg)] hover:-translate-y-px transition-all`}
    >
      <p className="text-[0.72rem] font-semibold uppercase tracking-wide text-fg-muted mb-1">
        {label}
      </p>
      <p className="font-mono text-[1.5rem] font-bold text-fg leading-tight tracking-tight">
        {value}
      </p>
      {detail && (
        <p className="text-[0.72rem] text-fg-muted mt-1 leading-snug">
          {detail}
        </p>
      )}
    </div>
  );
}
