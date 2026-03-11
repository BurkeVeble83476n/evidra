import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Mock API plugin for local development without a running backend.
// Only active in dev mode — production builds are unaffected.
function mockApiPlugin(): Plugin {
  return {
    name: "mock-api",
    configureServer(server) {
      // Mock POST /v1/keys — simulates key generation.
      server.middlewares.use((req, res, next) => {
        if (req.url !== "/v1/keys" || req.method !== "POST") return next();

        const inviteSecret = req.headers["x-invite-secret"] as string;
        if (!inviteSecret || inviteSecret !== "demo") {
          res.writeHead(403, { "Content-Type": "application/json" });
          res.end(JSON.stringify({ error: "invite required" }));
          return;
        }

        let body = "";
        req.on("data", (chunk: Buffer) => { body += chunk.toString(); });
        req.on("end", () => {
          const parsed = body ? JSON.parse(body) : {};
          const suffix = Math.random().toString(36).substring(2, 26);
          const key = `ev1_${btoa(suffix).replace(/=+$/, "")}`;
          res.writeHead(201, { "Content-Type": "application/json" });
          res.end(JSON.stringify({
            key,
            prefix: key.substring(0, 8),
            tenant_id: `tnt_${Math.random().toString(36).substring(2, 10)}`,
            label: parsed.label || "",
            created_at: new Date().toISOString(),
          }));
        });
      });

      // Mock GET /healthz
      server.middlewares.use((_req, res, next) => {
        if (_req.url !== "/healthz" || _req.method !== "GET") return next();
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("ok");
      });

      // Mock GET /readyz
      server.middlewares.use((_req, res, next) => {
        if (_req.url !== "/readyz" || _req.method !== "GET") return next();
        res.writeHead(200, { "Content-Type": "text/plain" });
        res.end("ok");
      });

      // Mock GET /v1/evidence/scorecard
      server.middlewares.use((_req, res, next) => {
        if (!_req.url?.startsWith("/v1/evidence/scorecard") || _req.method !== "GET") return next();
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify({
          score: 96.5,
          band: "good",
          basis: "sufficient",
          confidence: "high",
          total_entries: 47,
          signal_summary: {
            protocol_violation: { detected: false, weight: 0.35, count: 0 },
            artifact_drift: { detected: true, weight: 0.30, count: 2 },
            retry_loop: { detected: true, weight: 0.20, count: 1 },
            thrashing: { detected: false, weight: 0.15, count: 0 },
            blast_radius: { detected: false, weight: 0.10, count: 0 },
            risk_escalation: { detected: false, weight: 0.10, count: 0 },
            new_scope: { detected: true, weight: 0.05, count: 3 },
            repair_loop: { detected: false, weight: -0.05, count: 0 },
          },
        }));
      });

      // Mock GET /v1/evidence/entries
      server.middlewares.use((_req, res, next) => {
        if (!_req.url?.startsWith("/v1/evidence/entries") || _req.method !== "GET") return next();
        const url = new URL(_req.url, "http://localhost");
        const limit = parseInt(url.searchParams.get("limit") || "20", 10);
        const offset = parseInt(url.searchParams.get("offset") || "0", 10);
        const now = Date.now();
        const allEntries = [
          { id: "01JD1A2B3C", type: "prescription", tool: "kubectl", operation: "apply", scope: "ns/production", risk_level: "medium", actor: "claude-code", created_at: new Date(now - 120000).toISOString() },
          { id: "01JD1A2B3D", type: "report", tool: "kubectl", operation: "apply", scope: "ns/production", risk_level: "medium", actor: "claude-code", verdict: "success", exit_code: 0, created_at: new Date(now - 115000).toISOString() },
          { id: "01JD1A2B3E", type: "prescription", tool: "helm", operation: "upgrade", scope: "ns/staging", risk_level: "low", actor: "github-actions", created_at: new Date(now - 300000).toISOString() },
          { id: "01JD1A2B3F", type: "report", tool: "helm", operation: "upgrade", scope: "ns/staging", risk_level: "low", actor: "github-actions", verdict: "success", exit_code: 0, created_at: new Date(now - 295000).toISOString() },
          { id: "01JD1A2B3G", type: "prescription", tool: "terraform", operation: "apply", scope: "aws/eu-west-1", risk_level: "high", actor: "codex", created_at: new Date(now - 600000).toISOString() },
          { id: "01JD1A2B3H", type: "report", tool: "terraform", operation: "apply", scope: "aws/eu-west-1", risk_level: "high", actor: "codex", verdict: "success", exit_code: 0, created_at: new Date(now - 590000).toISOString() },
          { id: "01JD1A2B3I", type: "prescription", tool: "kubectl", operation: "delete", scope: "ns/dev", risk_level: "medium", actor: "claude-code", created_at: new Date(now - 900000).toISOString() },
          { id: "01JD1A2B3J", type: "report", tool: "kubectl", operation: "delete", scope: "ns/dev", risk_level: "medium", actor: "claude-code", verdict: "declined", created_at: new Date(now - 895000).toISOString() },
          { id: "01JD1A2B3K", type: "prescription", tool: "kubectl", operation: "apply", scope: "ns/staging", risk_level: "low", actor: "github-actions", created_at: new Date(now - 1200000).toISOString() },
          { id: "01JD1A2B3L", type: "report", tool: "kubectl", operation: "apply", scope: "ns/staging", risk_level: "low", actor: "github-actions", verdict: "success", exit_code: 0, created_at: new Date(now - 1195000).toISOString() },
          { id: "01JD1A2B3M", type: "prescription", tool: "terraform", operation: "plan", scope: "aws/us-east-1", risk_level: "low", actor: "codex", created_at: new Date(now - 1500000).toISOString() },
          { id: "01JD1A2B3N", type: "report", tool: "terraform", operation: "plan", scope: "aws/us-east-1", risk_level: "low", actor: "codex", verdict: "success", exit_code: 0, created_at: new Date(now - 1495000).toISOString() },
        ];
        const paged = allEntries.slice(offset, offset + limit);
        res.writeHead(200, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ entries: paged, total: 47 }));
      });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), mockApiPlugin()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {},
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./test/setup.ts"],
    exclude: ["e2e/**", "node_modules/**"],
  },
});
