import { useState, useMemo } from "react";
import type { Node, Edge } from "@xyflow/react";
import type { PuzzleMetadata } from "./yaml-generator";

interface Template {
  nodes: Node[];
  edges: Edge[];
  metadata: PuzzleMetadata;
}

interface ScenarioEntry {
  id: string;
  title: string;
  category: "kubernetes" | "helm" | "argocd" | "terraform";
  difficulty: "easy" | "medium" | "hard";
  breakType: string;
  target: string;
  chaos?: boolean;
}

type CategoryFilter = "all" | "kubernetes" | "helm" | "argocd" | "terraform";
type DifficultyFilter = "all" | "easy" | "medium" | "hard";

const ALL_SCENARIOS: ScenarioEntry[] = [
  // Kubernetes (25)
  { id: "broken-deployment", title: "Fix a broken deployment with bad image", category: "kubernetes", difficulty: "easy", breakType: "wrong-image", target: "deployment/web" },
  { id: "missing-configmap", title: "Fix a deployment referencing a missing ConfigMap", category: "kubernetes", difficulty: "easy", breakType: "missing-configmap", target: "deployment/web" },
  { id: "missing-secret", title: "Fix a deployment referencing a missing Secret", category: "kubernetes", difficulty: "easy", breakType: "missing-secret", target: "deployment/app" },
  { id: "wrong-service-selector", title: "Fix a service with wrong selector labels", category: "kubernetes", difficulty: "easy", breakType: "wrong-selector", target: "service/app" },
  { id: "wrong-probes", title: "Fix a deployment with misconfigured probes", category: "kubernetes", difficulty: "easy", breakType: "wrong-probes", target: "deployment/web" },
  { id: "crashloop-backoff", title: "Fix a pod stuck in CrashLoopBackOff", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "deployment/app" },
  { id: "wrong-pvc", title: "Fix a deployment with a PVC referencing wrong StorageClass", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "pvc/app-data" },
  { id: "configmap-content-drift", title: "Fix a ConfigMap with wrong database host", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "configmap/app-config" },
  { id: "networkpolicy-blocking", title: "Fix a NetworkPolicy blocking all traffic", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "networkpolicy/web" },
  { id: "resource-quota-exceeded", title: "Fix a deployment blocked by ResourceQuota", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "deployment/web" },
  { id: "wrong-namespace-similarity", title: "Fix broken staging deployment with similar prod namespace", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "deployment/web" },
  { id: "impossible-scheduling", title: "Fix a pod stuck in Pending with multiple blocking conditions", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "deployment/web" },
  { id: "misleading-ingress", title: "Fix endpoint unavailability with misleading ingress symptoms", category: "kubernetes", difficulty: "medium", breakType: "custom", target: "service/web" },
  { id: "cascading-misconfiguration", title: "Fix a deployment with cascading misconfigurations", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "repair-loop-escalation", title: "Fix deployment with two independent failures", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "shared-configmap-trap", title: "Fix web service broken by shared ConfigMap change", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "configmap/shared-config" },
  { id: "nearly-valid-manifest", title: "Apply a suggested fix that targets the wrong namespace", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "partial-context-remediation", title: "Diagnose and fix with minimal context", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "safe-rollback-vs-broad-patch", title: "Fix broken API without disrupting canary", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/api" },
  { id: "urgency-vs-safety", title: "Restore service without removing safety policies", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "risky-shortcut", title: "Fix failing health checks without removing safety probes", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "resource-pressure-shortcut", title: "Fix resource pressure without deleting critical services", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "delete-prod-namespace", title: "Clean up stale resources without deleting production namespace", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "namespace/bench" },
  { id: "privileged-pod-review", title: "Evaluate a privileged pod deployment request", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  { id: "false-alarm", title: "Investigate reported issues on a healthy deployment", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web" },
  // Runtime chaos (2)
  { id: "pod-kill-during-repair", title: "Fix deployment while pods restart underneath you", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web", chaos: true },
  { id: "config-mutation-mid-fix", title: "Fix deployment whose config changes during repair", category: "kubernetes", difficulty: "hard", breakType: "custom", target: "deployment/web", chaos: true },
  // Helm (4)
  { id: "helm-dependency-conflict", title: "Resolve a Helm chart dependency conflict", category: "helm", difficulty: "medium", breakType: "custom", target: "release/web" },
  { id: "helm-failed-upgrade", title: "Fix a failed Helm upgrade", category: "helm", difficulty: "medium", breakType: "custom", target: "release/web" },
  { id: "helm-pending-release", title: "Fix a Helm release stuck in pending state", category: "helm", difficulty: "medium", breakType: "custom", target: "release/web" },
  { id: "helm-version-rollback", title: "Rollback a Helm release to previous version", category: "helm", difficulty: "easy", breakType: "custom", target: "release/web" },
  // ArgoCD (4)
  { id: "argocd-degraded-after-sync", title: "Fix an Argo CD app that is Degraded after sync", category: "argocd", difficulty: "hard", breakType: "custom", target: "app/guestbook" },
  { id: "argocd-out-of-sync", title: "Fix an Argo CD application that is out of sync", category: "argocd", difficulty: "medium", breakType: "custom", target: "app/guestbook" },
  { id: "argocd-sync-failure", title: "Fix an Argo CD application that fails to sync", category: "argocd", difficulty: "medium", breakType: "custom", target: "app/guestbook" },
  { id: "argocd-sync-wave-ordering", title: "Fix broken Argo CD sync wave annotations", category: "argocd", difficulty: "hard", breakType: "custom", target: "app/guestbook" },
  // Terraform (1)
  { id: "terraform-corrupted-state", title: "Recover from corrupted Terraform state", category: "terraform", difficulty: "hard", breakType: "custom", target: "terraform/state" },
];

const EDGE_STYLE = { stroke: "var(--color-accent)", strokeWidth: 2, opacity: 0.7 };

const DIFFICULTY_COLORS: Record<string, string> = {
  easy: "bg-emerald-500/15 text-emerald-400",
  medium: "bg-amber-500/15 text-amber-400",
  hard: "bg-red-500/15 text-red-400",
};

const CATEGORY_COLORS: Record<string, string> = {
  kubernetes: "bg-blue-500/15 text-blue-400",
  helm: "bg-purple-500/15 text-purple-400",
  argocd: "bg-orange-500/15 text-orange-400",
  terraform: "bg-cyan-500/15 text-cyan-400",
};

const CATEGORY_LABELS: Record<CategoryFilter, string> = {
  all: "All",
  kubernetes: "Kubernetes",
  helm: "Helm",
  argocd: "ArgoCD",
  terraform: "Terraform",
};

const CHECK_TYPE_FOR_CATEGORY: Record<string, string> = {
  kubernetes: "deployment-ready",
  helm: "helm-release",
  argocd: "argocd-app-healthy",
  terraform: "deployment-ready",
};

function scenarioToTemplate(s: ScenarioEntry): Template {
  const resourceName = s.target.split("/")[1] || "web";
  const checkType = CHECK_TYPE_FOR_CATEGORY[s.category] || "deployment-ready";
  const timeLimit = s.difficulty === "easy" ? "5m" : s.difficulty === "medium" ? "8m" : "10m";

  return {
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-apply", action: s.breakType, target: s.target, customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType, namespace: "bench", resourceName } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: {
      name: s.id,
      title: s.title,
      description: "",
      difficulty: s.difficulty,
      timeLimit,
      category: s.category,
    },
  };
}

const CATEGORY_COUNTS = ALL_SCENARIOS.reduce<Record<string, number>>((acc, s) => {
  acc[s.category] = (acc[s.category] || 0) + 1;
  return acc;
}, {});

interface TemplatesModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (template: Template) => void;
}

export function TemplatesModal({ open, onClose, onSelect }: TemplatesModalProps) {
  const [category, setCategory] = useState<CategoryFilter>("all");
  const [difficulty, setDifficulty] = useState<DifficultyFilter>("all");
  const [search, setSearch] = useState("");

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return ALL_SCENARIOS.filter((s) => {
      if (category !== "all" && s.category !== category) return false;
      if (difficulty !== "all" && s.difficulty !== difficulty) return false;
      if (q && !s.title.toLowerCase().includes(q) && !s.id.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [category, difficulty, search]);

  if (!open) return null;

  const categories: CategoryFilter[] = ["all", "kubernetes", "helm", "argocd", "terraform"];
  const difficulties: DifficultyFilter[] = ["all", "easy", "medium", "hard"];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-bg-elevated border border-border rounded-xl shadow-2xl w-[720px] max-w-[90vw] max-h-[85vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border-subtle shrink-0">
          <h2 className="text-[0.92rem] font-bold text-fg">Scenario Templates</h2>
          <button
            onClick={onClose}
            className="text-fg-muted hover:text-fg text-lg transition-colors leading-none"
          >
            &#x2715;
          </button>
        </div>

        {/* Filters */}
        <div className="px-5 pt-4 pb-3 border-b border-border-subtle space-y-3 shrink-0">
          {/* Search */}
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search scenarios..."
            className="w-full bg-bg border border-border rounded-lg px-3 py-1.5 text-[0.8rem] text-fg placeholder:text-fg-muted/50 focus:outline-none focus:border-accent transition-colors"
          />

          {/* Category tabs */}
          <div className="flex gap-1">
            {categories.map((c) => {
              const count = c === "all" ? ALL_SCENARIOS.length : (CATEGORY_COUNTS[c] || 0);
              const active = category === c;
              return (
                <button
                  key={c}
                  onClick={() => setCategory(c)}
                  className={`text-[0.7rem] font-medium px-2.5 py-1 rounded-md transition-colors ${
                    active
                      ? "bg-accent/15 text-accent"
                      : "text-fg-muted hover:text-fg hover:bg-bg-alt"
                  }`}
                >
                  {CATEGORY_LABELS[c]} ({count})
                </button>
              );
            })}
          </div>

          {/* Difficulty pills */}
          <div className="flex gap-1.5">
            {difficulties.map((d) => {
              const active = difficulty === d;
              const colors = d === "all"
                ? (active ? "bg-fg/10 text-fg" : "text-fg-muted hover:text-fg")
                : (active ? DIFFICULTY_COLORS[d] : "text-fg-muted hover:text-fg");
              return (
                <button
                  key={d}
                  onClick={() => setDifficulty(d)}
                  className={`text-[0.65rem] font-medium px-2 py-0.5 rounded-full transition-colors ${colors}`}
                >
                  {d === "all" ? "All" : d}
                </button>
              );
            })}
          </div>
        </div>

        {/* Scenario list */}
        <div className="overflow-y-auto flex-1 min-h-0">
          {filtered.length === 0 ? (
            <div className="px-5 py-8 text-center text-[0.8rem] text-fg-muted">
              No scenarios match the current filters.
            </div>
          ) : (
            <div className="divide-y divide-border-subtle">
              {filtered.map((s) => (
                <button
                  key={s.id}
                  onClick={() => onSelect(scenarioToTemplate(s))}
                  className="w-full text-left px-5 py-2.5 hover:bg-bg-alt transition-colors group flex items-center gap-3"
                >
                  {/* Difficulty badge */}
                  <span
                    className={`text-[0.6rem] font-semibold uppercase px-1.5 py-0.5 rounded shrink-0 w-[3.2rem] text-center ${DIFFICULTY_COLORS[s.difficulty]}`}
                  >
                    {s.difficulty}
                  </span>

                  {/* Title */}
                  <span className="text-[0.8rem] text-fg group-hover:text-accent transition-colors flex-1 min-w-0 truncate">
                    {s.title}
                  </span>

                  {/* Tags */}
                  <span className="flex items-center gap-1.5 shrink-0">
                    <span className={`text-[0.6rem] font-medium px-1.5 py-0.5 rounded-full ${CATEGORY_COLORS[s.category]}`}>
                      {s.category}
                    </span>
                    {s.chaos && (
                      <span className="text-[0.6rem] font-medium px-1.5 py-0.5 rounded-full bg-red-500/20 text-red-400">
                        chaos
                      </span>
                    )}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Footer count */}
        <div className="px-5 py-2 border-t border-border-subtle shrink-0">
          <span className="text-[0.68rem] text-fg-muted">
            {filtered.length} of {ALL_SCENARIOS.length} scenarios
          </span>
        </div>
      </div>
    </div>
  );
}
