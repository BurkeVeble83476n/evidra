import { useEffect, useState, useCallback } from "react";
import { useAuth } from "../context/AuthContext";
import { useApi } from "../hooks/useApi";

interface EvidenceEntry {
  id: string;
  type: string;
  tool: string;
  operation: string;
  resource: string;
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
  const { apiKey, setApiKey } = useAuth();
  const { request } = useApi();
  const [entries, setEntries] = useState<EvidenceEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [scorecard, setScorecard] = useState<Scorecard | null>(null);
  const [period, setPeriod] = useState("30d");
  const [actorFilter, setActorFilter] = useState("");
  const [actors, setActors] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [keyInput, setKeyInput] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const actorParam = actorFilter ? `&actor=${encodeURIComponent(actorFilter)}` : "";
      const [entriesRes, scorecardRes] = await Promise.all([
        request<EntriesResponse>(`/v1/evidence/entries?limit=100&period=${period}${actorParam}`),
        request<Scorecard>(`/v1/evidence/scorecard?period=${period}`),
      ]);
      const allEntries = entriesRes.entries || [];

      // Build actor list from the full (unfiltered) set.
      if (!actorFilter) {
        const unique = [...new Set(allEntries.map((e) => e.actor).filter(Boolean))].sort();
        setActors(unique);
      }

      // Client-side filter fallback: if the API didn't filter (old deployment),
      // apply the filter here so the UI always shows filtered results.
      const filtered = actorFilter
        ? allEntries.filter((e) => e.actor === actorFilter)
        : allEntries;
      setEntries(filtered);
      setTotal(actorFilter ? filtered.length : entriesRes.total);
      setScorecard(scorecardRes);
    } catch {
      // Auth or API error — leave empty.
    }
    setLoading(false);
  }, [request, period, actorFilter]);

  useEffect(() => {
    if (apiKey) load();
  }, [load, apiKey]);

  if (!apiKey) {
    return (
      <div className="max-w-md mx-auto px-4 py-24 text-center">
        <h1 className="text-2xl font-bold text-fg mb-4">Evidence Chain</h1>
        <p className="text-fg-muted mb-6">Enter your API key to view evidence.</p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (keyInput.trim()) setApiKey(keyInput.trim());
          }}
          className="flex gap-2"
        >
          <input
            type="text"
            value={keyInput}
            onChange={(e) => setKeyInput(e.target.value)}
            placeholder="API key (e.g. dev-api-key)"
            className="flex-1 px-3 py-2 rounded-lg border border-bg-alt bg-bg text-fg text-sm"
          />
          <button
            type="submit"
            className="px-4 py-2 rounded-lg bg-accent text-white text-sm font-medium hover:bg-accent-bright"
          >
            Connect
          </button>
        </form>
      </div>
    );
  }

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
          <span className="mx-1 text-fg-muted">|</span>
          <select
            value={actorFilter}
            onChange={(e) => setActorFilter(e.target.value)}
            className="px-2 py-1 rounded text-sm bg-bg-alt text-fg border border-border-subtle"
          >
            <option value="">All actors</option>
            {actors.map((a) => (
              <option key={a} value={a}>{a}</option>
            ))}
          </select>
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
                <th className="py-2 px-3" title="When the operation was recorded">Time</th>
                <th className="py-2 px-3" title="Operation type: prescribe (before mutation) or report (after mutation)">Type</th>
                <th className="py-2 px-3" title="Infrastructure tool and operation (e.g. kubectl apply, helm install)">Tool / Operation</th>
                <th className="py-2 px-3" title="Target resource affected by the operation">Resource</th>
                <th className="py-2 px-3" title="Risk classification: critical, high, medium, or low — based on blast radius and reversibility">Risk</th>
                <th className="py-2 px-3" title="Outcome verdict: success, failure, error, or declined">Verdict</th>
                <th className="py-2 px-3" title="Entity that performed the operation (agent, human, or system)">Actor</th>
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
                  <td className="py-2 px-3 font-mono text-xs">
                    {e.tool || "—"}{e.operation && e.operation !== "tools/call" ? ` ${e.operation}` : ""}
                  </td>
                  <td className="py-2 px-3 font-mono text-xs text-fg-muted">{e.resource || "—"}</td>
                  <td className="py-2 px-3">{riskBadge(e.risk_level)}</td>
                  <td className="py-2 px-3">{verdictBadge(e.verdict)}</td>
                  <td className="py-2 px-3 text-xs text-fg-muted">{e.actor || "—"}</td>
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
