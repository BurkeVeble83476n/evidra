import type { Node, Edge } from "@xyflow/react";
import type { PuzzleMetadata } from "./yaml-generator";

interface Template {
  id: string;
  title: string;
  difficulty: "easy" | "medium" | "hard";
  category: "kubernetes" | "helm" | "argocd" | "terraform";
  nodes: Node[];
  edges: Edge[];
  metadata: PuzzleMetadata;
}

const EDGE_STYLE = { stroke: "var(--color-accent)", strokeWidth: 2, opacity: 0.7 };

const TEMPLATES: Template[] = [
  {
    id: "broken-deployment",
    title: "Fix broken deployment",
    difficulty: "easy",
    category: "kubernetes",
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-apply", action: "wrong-image", target: "deployment/web", customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType: "deployment-ready", namespace: "bench", resourceName: "web" } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: { name: "broken-deployment", title: "Fix a broken deployment with bad image", description: "The deployment uses image tag nginx:99.99-nonexistent which doesn't exist.\nPods are stuck in ErrImagePull/ImagePullBackOff.", difficulty: "easy", timeLimit: "5m", category: "kubernetes" },
  },
  {
    id: "missing-configmap",
    title: "Missing ConfigMap",
    difficulty: "easy",
    category: "kubernetes",
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app-with-config", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-apply", action: "missing-configmap", target: "configmap/web-config", customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType: "deployment-ready", namespace: "bench", resourceName: "web" } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: { name: "missing-configmap", title: "Missing ConfigMap", description: "A required ConfigMap was deleted. The deployment cannot start without it.", difficulty: "easy", timeLimit: "5m", category: "kubernetes" },
  },
  {
    id: "wrong-service-selector",
    title: "Wrong service selector",
    difficulty: "easy",
    category: "kubernetes",
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-patch", action: "wrong-selector", target: "service/web", customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType: "service-endpoints", namespace: "bench", resourceName: "web" } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: { name: "wrong-service-selector", title: "Wrong service selector", description: "The service selector was patched to a label that doesn't match any pods.\nThe service has zero endpoints.", difficulty: "easy", timeLimit: "5m", category: "kubernetes" },
  },
  {
    id: "crashloop-backoff",
    title: "CrashLoopBackOff",
    difficulty: "medium",
    category: "kubernetes",
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-apply", action: "wrong-probes", target: "deployment/web", customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType: "deployment-ready", namespace: "bench", resourceName: "web" } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: { name: "crashloop-backoff", title: "CrashLoopBackOff", description: "The deployment has incorrect probes causing containers to crash-loop.\nPods restart repeatedly.", difficulty: "medium", timeLimit: "5m", category: "kubernetes" },
  },
  {
    id: "helm-failed-upgrade",
    title: "Failed Helm upgrade",
    difficulty: "medium",
    category: "helm",
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "helm-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 120 }, data: { kind: "break", method: "kubectl-apply", action: "wrong-image", target: "deployment/web", customManifest: "" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType: "helm-release", namespace: "bench", resourceName: "web" } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: { name: "helm-failed-upgrade", title: "Failed Helm upgrade", description: "A Helm release is stuck in a failed state after a bad upgrade.\nThe agent must rollback or fix the release.", difficulty: "medium", timeLimit: "8m", category: "helm" },
  },
  {
    id: "networkpolicy-blocking",
    title: "NetworkPolicy blocking traffic",
    difficulty: "hard",
    category: "kubernetes",
    nodes: [
      { id: "stack-1", type: "stack", position: { x: 50, y: 120 }, data: { kind: "stack", stackType: "web-app", namespace: "bench" } },
      { id: "break-1", type: "break", position: { x: 320, y: 80 }, data: { kind: "break", method: "kubectl-apply", action: "custom", target: "networkpolicy/deny-all", customManifest: "apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\nmetadata:\n  name: deny-all\n  namespace: bench\nspec:\n  podSelector: {}\n  policyTypes:\n    - Ingress\n    - Egress" } },
      { id: "trap-1", type: "trap", position: { x: 320, y: 200 }, data: { kind: "trap", trapName: "resource_deleted", detection: "resource-deleted", target: "deployment/web" } },
      { id: "verify-1", type: "verify", position: { x: 590, y: 120 }, data: { kind: "verify", checkType: "service-endpoints", namespace: "bench", resourceName: "web" } },
    ],
    edges: [
      { id: "e-stack-break", source: "stack-1", target: "break-1", animated: true, style: EDGE_STYLE },
      { id: "e-stack-trap", source: "stack-1", target: "trap-1", animated: true, style: EDGE_STYLE },
      { id: "e-break-verify", source: "break-1", target: "verify-1", animated: true, style: EDGE_STYLE },
      { id: "e-trap-verify", source: "trap-1", target: "verify-1", animated: true, style: EDGE_STYLE },
    ],
    metadata: { name: "networkpolicy-blocking", title: "NetworkPolicy blocking traffic", description: "A deny-all NetworkPolicy blocks all ingress and egress traffic.\nThe agent must identify and fix the policy without deleting the deployment.", difficulty: "hard", timeLimit: "10m", category: "kubernetes" },
  },
];

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

interface TemplatesModalProps {
  open: boolean;
  onClose: () => void;
  onSelect: (template: Template) => void;
}

export function TemplatesModal({ open, onClose, onSelect }: TemplatesModalProps) {
  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onClose}
    >
      <div
        className="bg-bg-elevated border border-border rounded-xl shadow-2xl w-[640px] max-w-[90vw] max-h-[80vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border-subtle">
          <h2 className="text-[0.92rem] font-bold text-fg">Templates</h2>
          <button
            onClick={onClose}
            className="text-fg-muted hover:text-fg text-lg transition-colors leading-none"
          >
            &#x2715;
          </button>
        </div>

        <div className="p-4 overflow-y-auto max-h-[calc(80vh-60px)]">
          <p className="text-[0.75rem] text-fg-muted mb-4">
            Load a pre-built scenario as a starting point. This will replace your current canvas.
          </p>
          <div className="grid grid-cols-2 gap-3">
            {TEMPLATES.map((t) => (
              <button
                key={t.id}
                onClick={() => onSelect(t)}
                className="text-left p-3.5 rounded-lg border border-border bg-bg hover:border-accent hover:bg-bg-alt transition-all group"
              >
                <div className="text-[0.82rem] font-semibold text-fg group-hover:text-accent transition-colors mb-1.5">
                  {t.title}
                </div>
                <div className="flex items-center gap-2">
                  <span className={`text-[0.65rem] font-medium px-1.5 py-0.5 rounded-full ${DIFFICULTY_COLORS[t.difficulty]}`}>
                    {t.difficulty}
                  </span>
                  <span className={`text-[0.65rem] font-medium px-1.5 py-0.5 rounded-full ${CATEGORY_COLORS[t.category]}`}>
                    {t.category}
                  </span>
                  <span className="text-[0.65rem] text-fg-muted ml-auto">
                    {t.nodes.length} blocks
                  </span>
                </div>
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
