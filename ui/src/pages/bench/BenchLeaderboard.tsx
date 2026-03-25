import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { useApi } from "../../hooks/useApi";
import { evidenceModeParam } from "../../lib/catalogData.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";

/* ── Types ── */

interface LeaderboardEntry {
  model: string;
  scenarios: number;
  runs: number;
  pass_rate: number;
  avg_duration: number;
  avg_cost: number;
  total_cost: number;
}

interface LeaderboardResponse {
  models: LeaderboardEntry[];
  evidence_mode: string;
}

interface ModelStats {
  model: string;
  runs: number;
  passed: number;
  failed: number;
  rate: number;
  scenarios: number;
  avgDuration: number;
  totalCost: number;
  costPerRun: number;
  costPerPass: number;
}

type SortKey = keyof Pick<
  ModelStats,
  "rate" | "runs" | "avgDuration" | "costPerRun" | "costPerPass" | "scenarios"
>;

const SORT_OPTIONS: { key: SortKey; label: string; desc: boolean }[] = [
  { key: "rate", label: "Pass Rate", desc: true },
  { key: "costPerPass", label: "Cost/Pass", desc: false },
  { key: "avgDuration", label: "Duration", desc: false },
  { key: "runs", label: "Runs", desc: true },
  { key: "scenarios", label: "Scenarios", desc: true },
];

function formatDuration(s: number): string {
  if (s < 60) return `${s.toFixed(1)}s`;
  return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`;
}

function formatCost(usd: number): string {
  if (usd === 0) return "$0.00";
  if (usd < 0.001) return `$${usd.toFixed(4)}`;
  if (usd < 0.01) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(2)}`;
}

function rateColor(rate: number): string {
  if (rate >= 90) return "text-accent";
  if (rate >= 70) return "text-accent";
  if (rate >= 50) return "text-warning";
  return "text-danger";
}

function rateBg(rate: number): string {
  if (rate >= 90) return "bg-accent";
  if (rate >= 70) return "bg-accent";
  if (rate >= 50) return "bg-warning";
  if (rate > 0) return "bg-danger";
  return "bg-fg-muted";
}

function medalEmoji(rank: number): string {
  if (rank === 0) return "\uD83E\uDD47";
  if (rank === 1) return "\uD83E\uDD48";
  if (rank === 2) return "\uD83E\uDD49";
  return "";
}

/* ── Component ── */

export function BenchLeaderboard() {
  const { request } = useApi();
  const { mode, setMode } = useEvidenceMode();
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [sortKey, setSortKey] = useState<SortKey>("rate");
  const [sortDesc, setSortDesc] = useState(true);

  useEffect(() => {
    request<LeaderboardResponse>(`/v1/bench/leaderboard${evidenceModeParam("?", mode)}`)
      .then((res) => setEntries(res.models ?? []))
      .catch(() => setEntries([]))
      .finally(() => setLoading(false));
  }, [request, mode]);

  const models = useMemo(() => {
    return entries.map((e): ModelStats => {
      const passed = Math.round((e.pass_rate / 100) * e.runs);
      return {
        model: e.model,
        runs: e.runs,
        passed,
        failed: e.runs - passed,
        rate: e.pass_rate,
        scenarios: e.scenarios,
        avgDuration: e.avg_duration,
        totalCost: e.total_cost,
        costPerRun: e.avg_cost,
        costPerPass: passed > 0 ? e.total_cost / passed : Infinity,
      };
    });
  }, [entries]);

  const sorted = useMemo(() => {
    const arr = [...models];
    arr.sort((a, b) => {
      const va = a[sortKey] ?? -1;
      const vb = b[sortKey] ?? -1;
      if (va === Infinity && vb === Infinity) return 0;
      if (va === Infinity || va === -1) return 1;
      if (vb === Infinity || vb === -1) return -1;
      return sortDesc ? (vb as number) - (va as number) : (va as number) - (vb as number);
    });
    return arr;
  }, [models, sortKey, sortDesc]);

  const totalRuns = models.reduce((s, m) => s + m.runs, 0);
  const totalPassed = models.reduce((s, m) => s + m.passed, 0);
  const totalCost = models.reduce((s, m) => s + m.totalCost, 0);
  const totalModels = models.length;

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDesc(!sortDesc);
    } else {
      const opt = SORT_OPTIONS.find((o) => o.key === key);
      setSortKey(key);
      setSortDesc(opt?.desc ?? true);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading leaderboard...
      </div>
    );
  }

  return (
    <div className="max-w-[980px] mx-auto px-8 py-10 space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
            Model Leaderboard
          </h1>
          <p className="text-[0.85rem] text-fg-muted mt-0.5">
            {totalModels} models ranked across {totalRuns} benchmark runs
          </p>
        </div>
        <div className="flex gap-1 text-[0.72rem] font-medium">
          {(["all", "proxy", "smart", "direct"] as const).map((m) => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={`px-2.5 py-1 rounded-md transition-colors ${
                mode === m
                  ? "bg-accent text-white"
                  : "bg-bg-alt text-fg-muted hover:text-fg"
              }`}
            >
              {m === "all" ? "All" : m}
            </button>
          ))}
        </div>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MiniCard label="Models" value={String(totalModels)} />
        <MiniCard label="Total Runs" value={String(totalRuns)} />
        <MiniCard
          label="Overall Pass Rate"
          value={`${totalRuns > 0 ? ((totalPassed / totalRuns) * 100).toFixed(1) : 0}%`}
        />
        <MiniCard label="Total Cost" value={`$${totalCost.toFixed(2)}`} />
      </div>

      {/* Leaderboard table */}
      <div className="bg-bg-elevated border border-border-subtle rounded-[10px] overflow-hidden shadow-[var(--shadow-card)]">
        <table className="w-full text-[0.82rem]">
          <thead>
            <tr className="border-b border-border bg-bg-alt">
              <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5 w-10">
                #
              </th>
              <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5">
                Model
              </th>
              {SORT_OPTIONS.map((opt) => (
                <th
                  key={opt.key}
                  className="text-right text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5 cursor-pointer hover:text-accent transition-colors whitespace-nowrap"
                  onClick={() => handleSort(opt.key)}
                >
                  {opt.label}{" "}
                  {sortKey === opt.key ? (
                    <span className="text-accent">{sortDesc ? "\u2193" : "\u2191"}</span>
                  ) : (
                    <span className="opacity-30">{"\u2195"}</span>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map((m, i) => (
              <tr
                key={m.model}
                className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
              >
                {/* Rank */}
                <td className="px-4 py-3 text-center">
                  {i < 3 ? (
                    <span className="text-[1.1rem]">{medalEmoji(i)}</span>
                  ) : (
                    <span className="font-mono text-fg-muted text-[0.78rem]">{i + 1}</span>
                  )}
                </td>

                {/* Model name */}
                <td className="px-4 py-3">
                  <Link
                    to={`/bench/runs?model=${m.model}`}
                    className="font-mono text-[0.85rem] font-semibold text-fg hover:text-accent transition-colors"
                    style={{ textDecoration: "none" }}
                  >
                    {m.model}
                  </Link>
                  <div className="flex items-center gap-2 mt-1">
                    <div className="flex-1 max-w-[120px] h-1.5 rounded-full bg-bg-alt overflow-hidden">
                      <div
                        className={`h-full rounded-full ${rateBg(m.rate)}`}
                        style={{ width: `${m.rate}%` }}
                      />
                    </div>
                    <span className={`font-mono text-[0.7rem] font-semibold ${rateColor(m.rate)}`}>
                      {m.passed}/{m.runs}
                    </span>
                  </div>
                </td>

                {/* Pass Rate */}
                <td className="px-4 py-3 text-right">
                  <span className={`font-mono text-[0.85rem] font-bold ${rateColor(m.rate)}`}>
                    {m.rate.toFixed(1)}%
                  </span>
                </td>

                {/* Cost per Pass */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {m.costPerPass === Infinity ? "\u2014" : formatCost(m.costPerPass)}
                </td>

                {/* Avg Duration */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {formatDuration(m.avgDuration)}
                </td>

                {/* Total Runs */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {m.runs}
                </td>

                {/* Scenarios */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {m.scenarios}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Key insights */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <InsightCard
          title="Most Reliable"
          model={sorted[0]?.model ?? "\u2014"}
          stat={`${sorted[0]?.rate.toFixed(1) ?? 0}% pass rate`}
          detail={`${sorted[0]?.passed ?? 0}/${sorted[0]?.runs ?? 0} runs passed`}
          accent="accent"
        />
        <InsightCard
          title="Best Value"
          model={(() => {
            const viable = models.filter((m) => m.rate >= 50 && m.costPerRun > 0);
            viable.sort((a, b) => a.costPerPass - b.costPerPass);
            return viable[0]?.model ?? "\u2014";
          })()}
          stat={(() => {
            const viable = models.filter((m) => m.rate >= 50 && m.costPerRun > 0);
            viable.sort((a, b) => a.costPerPass - b.costPerPass);
            return viable[0] ? `${formatCost(viable[0].costPerPass)} per pass` : "\u2014";
          })()}
          detail={(() => {
            const viable = models.filter((m) => m.rate >= 50 && m.costPerRun > 0);
            viable.sort((a, b) => a.costPerPass - b.costPerPass);
            return viable[0] ? `${viable[0].rate.toFixed(0)}% rate, ${formatCost(viable[0].costPerRun)}/run` : "";
          })()}
          accent="info"
        />
        <InsightCard
          title="Fastest"
          model={(() => {
            const viable = models.filter((m) => m.rate >= 50);
            viable.sort((a, b) => a.avgDuration - b.avgDuration);
            return viable[0]?.model ?? "\u2014";
          })()}
          stat={(() => {
            const viable = models.filter((m) => m.rate >= 50);
            viable.sort((a, b) => a.avgDuration - b.avgDuration);
            return viable[0] ? formatDuration(viable[0].avgDuration) : "\u2014";
          })()}
          detail={(() => {
            const viable = models.filter((m) => m.rate >= 50);
            viable.sort((a, b) => a.avgDuration - b.avgDuration);
            return viable[0] ? `${viable[0].rate.toFixed(0)}% rate across ${viable[0].scenarios} scenarios` : "";
          })()}
          accent="warning"
        />
      </div>
    </div>
  );
}

/* ── Sub-components ── */

function MiniCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-bg-elevated border border-border-subtle rounded-lg p-3 shadow-[var(--shadow-card)]">
      <p className="text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
        {label}
      </p>
      <p className="font-mono text-[1.1rem] font-bold text-fg mt-0.5">{value}</p>
    </div>
  );
}

function InsightCard({
  title,
  model,
  stat,
  detail,
  accent,
}: {
  title: string;
  model: string;
  stat: string;
  detail: string;
  accent: string;
}) {
  return (
    <div className={`bg-bg-elevated border border-border-subtle rounded-[10px] p-4 shadow-[var(--shadow-card)] border-l-[3px] border-l-${accent}`}>
      <p className="text-[0.72rem] font-semibold uppercase tracking-wide text-fg-muted mb-1">
        {title}
      </p>
      <p className="font-mono text-[0.92rem] font-bold text-fg">{model}</p>
      <p className={`font-mono text-[0.78rem] font-semibold text-${accent} mt-0.5`}>{stat}</p>
      {detail && <p className="text-[0.7rem] text-fg-muted mt-0.5">{detail}</p>}
    </div>
  );
}
