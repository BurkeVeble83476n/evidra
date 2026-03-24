import { useEffect, useState, useCallback } from "react";
import { useApi } from "../hooks/useApi";

interface EvidenceEntry {
  id: string;
  type: string;
  tool: string;
  operation: string;
  scope: string;
  risk_level: string;
  actor: string;
  verdict: string;
  exit_code: number | null;
  created_at: string;
}

interface EntriesResponse {
  entries: EvidenceEntry[];
  total: number;
  limit: number;
  offset: number;
}

interface Scorecard {
  score: number;
  band: string;
  basis: string;
  total_entries: number;
  scoring_version?: string;
  signal_summary: Record<
    string,
    { detected: boolean; weight: number; count: number }
  >;
}

const PERIODS = [
  { label: "1h", value: "1h" },
  { label: "24h", value: "24h" },
  { label: "7d", value: "7d" },
  { label: "30d", value: "30d" },
];

function riskBadge(level: string) {
  if (!level) return null;
  const colors: Record<string, string> = {
    critical: "bg-red-600 text-white",
    high: "bg-orange-500 text-white",
    medium: "bg-yellow-500 text-black",
    low: "bg-green-600 text-white",
  };
  return (
    <span
      className={`px-2 py-0.5 rounded text-xs font-semibold ${colors[level] || "bg-gray-400 text-white"}`}
    >
      {level}
    </span>
  );
}

function verdictBadge(verdict: string) {
  if (!verdict) return null;
  const colors: Record<string, string> = {
    success: "text-green-600",
    failure: "text-red-600",
    error: "text-red-500",
    declined: "text-yellow-600",
  };
  return (
    <span className={`font-semibold text-sm ${colors[verdict] || "text-fg-muted"}`}>
      {verdict}
    </span>
  );
}

function typeBadge(type: string) {
  return (
    <span
      className={`px-2 py-0.5 rounded text-xs font-mono ${
        type === "prescribe"
          ? "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
          : "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200"
      }`}
    >
      {type}
    </span>
  );
}

function signalDot(detected: boolean, count: number) {
  if (!detected) return <span className="text-green-500">●</span>;
  if (count > 2) return <span className="text-red-500">● {count}</span>;
  return <span className="text-yellow-500">● {count}</span>;
}

function formatTime(iso: string) {
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function formatDate(iso: string) {
  const d = new Date(iso);
  return d.toLocaleDateString([], { month: "short", day: "numeric" });
}

export function Evidence() {
  const { request } = useApi();
  const [entries, setEntries] = useState<EvidenceEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [scorecard, setScorecard] = useState<Scorecard | null>(null);
  const [period, setPeriod] = useState("30d");
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [entriesRes, scorecardRes] = await Promise.all([
        request<EntriesResponse>(`/v1/evidence/entries?limit=100&period=${period}`),
        request<Scorecard>(`/v1/evidence/scorecard?period=${period}`),
      ]);
      setEntries(entriesRes.entries || []);
      setTotal(entriesRes.total);
      setScorecard(scorecardRes);
    } catch {
      // Auth or API error — leave empty.
    }
    setLoading(false);
  }, [request, period]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="max-w-6xl mx-auto px-4 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-fg">Evidence Chain</h1>
        <div className="flex items-center gap-2">
          {PERIODS.map((p) => (
            <button
              key={p.value}
              onClick={() => setPeriod(p.value)}
              className={`px-3 py-1 rounded text-sm font-medium transition ${
                period === p.value
                  ? "bg-accent text-white"
                  : "bg-bg-alt text-fg-muted hover:text-fg"
              }`}
            >
              {p.label}
            </button>
          ))}
          <button
            onClick={load}
            className="ml-2 px-3 py-1 rounded text-sm bg-bg-alt text-fg-muted hover:text-fg"
          >
            ↻
          </button>
        </div>
      </div>

      {/* Scorecard summary */}
      {scorecard && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-bg-alt rounded-lg p-4">
            <div className="text-sm text-fg-muted">Score</div>
            <div className="text-2xl font-bold text-fg">
              {scorecard.score === -1 ? "—" : scorecard.score}
            </div>
            <div className="text-xs text-fg-muted">{scorecard.band}</div>
          </div>
          <div className="bg-bg-alt rounded-lg p-4">
            <div className="text-sm text-fg-muted">Operations</div>
            <div className="text-2xl font-bold text-fg">{scorecard.total_entries}</div>
            <div className="text-xs text-fg-muted">{total} total entries</div>
          </div>
          <div className="bg-bg-alt rounded-lg p-4">
            <div className="text-sm text-fg-muted">Signals</div>
            <div className="flex flex-wrap gap-2 mt-1">
              {scorecard.signal_summary &&
                Object.entries(scorecard.signal_summary).map(([name, sig]) => (
                  <span key={name} className="text-xs" title={`${name}: ${sig.count}`}>
                    {signalDot(sig.detected, sig.count)}{" "}
                    <span className="text-fg-muted">{name.replace("_", " ")}</span>
                  </span>
                ))}
            </div>
          </div>
          <div className="bg-bg-alt rounded-lg p-4">
            <div className="text-sm text-fg-muted">Period</div>
            <div className="text-lg font-semibold text-fg">{period}</div>
            <div className="text-xs text-fg-muted">{scorecard.scoring_version}</div>
          </div>
        </div>
      )}

      {/* Evidence table */}
      {loading ? (
        <div className="text-center py-12 text-fg-muted">Loading...</div>
      ) : entries.length === 0 ? (
        <div className="text-center py-12 text-fg-muted">
          No evidence entries found for this period.
        </div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-bg-alt text-left text-fg-muted">
                <th className="py-2 px-3">Time</th>
                <th className="py-2 px-3">Type</th>
                <th className="py-2 px-3">Tool</th>
                <th className="py-2 px-3">Operation</th>
                <th className="py-2 px-3">Risk</th>
                <th className="py-2 px-3">Verdict</th>
                <th className="py-2 px-3">Actor</th>
                <th className="py-2 px-3 text-right">ID</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e) => (
                <tr
                  key={e.id}
                  className="border-b border-bg-alt/50 hover:bg-bg-alt/30 transition"
                >
                  <td className="py-2 px-3 whitespace-nowrap">
                    <span className="text-fg-muted text-xs">{formatDate(e.created_at)}</span>{" "}
                    <span className="font-mono text-xs">{formatTime(e.created_at)}</span>
                  </td>
                  <td className="py-2 px-3">{typeBadge(e.type)}</td>
                  <td className="py-2 px-3 font-mono text-xs">{e.tool || "—"}</td>
                  <td className="py-2 px-3 text-xs">{e.operation || "—"}</td>
                  <td className="py-2 px-3">{riskBadge(e.risk_level)}</td>
                  <td className="py-2 px-3">{verdictBadge(e.verdict)}</td>
                  <td className="py-2 px-3 text-xs text-fg-muted">{e.actor || "—"}</td>
                  <td className="py-2 px-3 text-right">
                    <span className="font-mono text-xs text-fg-muted">{e.id.slice(-8)}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {total > entries.length && (
            <div className="text-center py-3 text-sm text-fg-muted">
              Showing {entries.length} of {total} entries
            </div>
          )}
        </div>
      )}
    </div>
  );
}
