# evidra-mcp as DevOps MCP Server

**Date:** 2026-03-22
**Goal:** Evolve evidra-mcp from evidence-only to a complete DevOps MCP server that replaces kubectl-mcp-server with token-efficient output and built-in evidence recording.

## Problem

Today's DevOps MCP landscape:

```
Agent → AgentGateway → kubectl-mcp-server → kubectl → cluster
                     → evidra-mcp → prescribe/report (evidence only)
```

Three problems:

1. **Two MCP servers needed.** Agent vendors must configure and maintain both.
2. **kubectl-mcp-server is a token bomb.** Raw `kubectl get -o json` returns 2000+ tokens per call — managed fields, last-applied-config, annotation noise. Agents burn context window on information they don't need.
3. **No safety layer.** kubectl-mcp-server executes anything. No evidence, no audit trail, no mutation awareness.

## Solution

One MCP server that does everything:

```
Agent → AgentGateway → evidra-mcp → kubectl/helm/terraform/aws → cluster
                                  → auto-evidence (built-in)
                                  → token-efficient output
```

### What evidra-mcp becomes

| Capability | Status | Description |
|---|---|---|
| Evidence recording | Exists | prescribe_smart, prescribe_full, report, get_event |
| Proxy mode | Exists | Wraps upstream MCP server, auto-records mutations |
| **Command execution** | **New** | run_command tool — executes kubectl/helm/terraform/aws |
| **Smart output** | **New** | Token-efficient response formatting |
| **Mutation detection** | **New** | Auto-prescribe/report for mutations, passthrough for reads |
| **Command allowlist** | **New** | Security: only approved commands |

## Smart Output — The Competitive Advantage

### The token problem

A single `kubectl get deployment web -n bench -o json` returns ~2000 tokens.
80% is noise: managedFields, last-applied-config, resourceVersion, uid, etc.

An agent doing a typical diagnosis (get deployment, get pods, describe pod,
get events, get logs) burns 8000-10000 tokens on raw output alone. That's
$0.01-0.03 per diagnosis in input tokens — and it fills the context window
with irrelevant data, degrading reasoning quality.

### The smart output approach

evidra-mcp parses kubectl output and returns only what matters:

#### Deployments

Raw (2000 tokens):
```json
{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"annotations":{"deployment.kubernetes.io/revision":"3","kubectl.kubernetes.io/last-applied-configuration":"{...}"},"managedFields":[{...}],...},"spec":{...},"status":{...}}
```

Smart (80 tokens):
```
deployment/web (bench): 0/2 ready
  image: nginx:99.99-nonexistent
  condition: Progressing=False
  event: Failed to pull image "nginx:99.99-nonexistent"
```

#### Pods

Raw (3000 tokens per pod):
```json
{"items":[{"metadata":{...},"spec":{"containers":[{"name":"nginx","image":"nginx:99.99","resources":{...},"volumeMounts":[...],...}],...},"status":{"phase":"Pending","containerStatuses":[{"state":{"waiting":{"reason":"ErrImagePull","message":"..."}},...}],...}}]}
```

Smart (100 tokens):
```
pod/web-58f84cfdf9-j8jwd (bench): ErrImagePull
  container nginx: waiting (ErrImagePull)
  message: rpc error: manifest for nginx:99.99-nonexistent not found
```

#### Events

Raw (500 tokens per event):
```json
{"items":[{"type":"Warning","reason":"Failed","message":"Failed to pull image...","firstTimestamp":"...","lastTimestamp":"...","count":5,...}]}
```

Smart (40 tokens):
```
events (bench, last 5):
  Warning Failed (x5): Failed to pull image "nginx:99.99-nonexistent"
  Normal  Scheduled: Successfully assigned bench/web-xxx to node
```

#### Logs

Raw (unlimited — can be megabytes):
```
2026-03-22T10:00:00Z line 1
2026-03-22T10:00:01Z line 2
... (10000 lines)
```

Smart (capped, errors highlighted):
```
logs web-xxx/nginx (last 50 lines, 3 errors):
  [ERROR] 2026-03-22T10:00:00Z connection refused to db:5432
  [ERROR] 2026-03-22T10:00:01Z retry failed
  ... (48 more lines)
```

### Token savings estimate

| Operation | kubectl-mcp-server | evidra-mcp | Savings |
|---|---|---|---|
| Get deployment | ~2000 tokens | ~80 tokens | 96% |
| Get pods (3 pods) | ~6000 tokens | ~300 tokens | 95% |
| Describe pod | ~3000 tokens | ~150 tokens | 95% |
| Get events | ~2000 tokens | ~100 tokens | 95% |
| Get logs (100 lines) | ~1500 tokens | ~200 tokens | 87% |
| **Typical diagnosis** | **~14500 tokens** | **~830 tokens** | **94%** |

At $3/M input tokens (GPT-4o): $0.044 → $0.002 per diagnosis. **20x cheaper.**

More importantly: 830 tokens of signal vs 14500 tokens of noise means the
agent reasons better. Clean context = better decisions.

## Architecture

### run_command tool

```go
// Tool definition
{
  "name": "run_command",
  "description": "Execute infrastructure commands (kubectl, helm, terraform, aws)",
  "input_schema": {
    "type": "object",
    "properties": {
      "command": {"type": "string", "description": "The command to execute"},
      "smart_output": {"type": "boolean", "default": true, "description": "Return token-efficient output"}
    },
    "required": ["command"]
  }
}
```

### Execution flow

```
Agent calls run_command("kubectl get deployment web -n bench")
  │
  ├── 1. Parse command
  │     └── Is it in the allowlist? (kubectl, helm, terraform, aws, cat, grep, etc.)
  │
  ├── 2. Detect mutation?
  │     ├── Read-only (get, describe, logs) → execute directly
  │     └── Mutation (apply, delete, patch) → auto-prescribe → execute → auto-report
  │
  ├── 3. Execute command
  │     └── subprocess with KUBECONFIG, timeout, env vars
  │
  ├── 4. Format output
  │     ├── smart_output=true → parse and summarize (see Smart Output above)
  │     └── smart_output=false → raw output (escape hatch)
  │
  └── 5. Return result
        └── Tool result with formatted output + exit code
```

### Mutation detection

Already exists in `pkg/proxy/mutation.go`:

```go
func IsMutation(command string) bool {
  // kubectl apply, delete, patch, create, replace, scale, rollout restart
  // helm install, upgrade, uninstall, rollback
  // terraform apply, destroy, import
}
```

### Auto-evidence for mutations

When a mutation is detected:

1. **Before execution:** Auto-generate prescribe record
   - Tool: extracted from command (kubectl, helm, etc.)
   - Operation: extracted from command (apply, delete, etc.)
   - Resource: extracted from command args

2. **Execute the command**

3. **After execution:** Auto-generate report record
   - Exit code
   - Success/failure verdict
   - Output summary

This is proxy mode behavior, but built into the tool — no upstream MCP
server needed.

### Command allowlist

Same as infra-bench:

```go
var allowedPrefixes = []string{
  "kubectl", "helm", "argocd", "terraform", "aws",
  "cat", "echo", "grep", "head", "tail", "wc", "ls", "find",
  "jq", "yq", "openssl",
}
```

Blocked: interactive commands (kubectl exec -it, kubectl edit, etc.)

## Smart Output Parser

### Implementation approach

Parse the command to determine the resource type, then format accordingly:

```go
func formatSmartOutput(command string, rawOutput string, exitCode int) string {
  if exitCode != 0 {
    return truncateError(rawOutput, 500)
  }

  switch {
  case isGetDeployment(command):
    return formatDeployment(rawOutput)
  case isGetPods(command):
    return formatPods(rawOutput)
  case isDescribe(command):
    return formatDescribe(rawOutput)
  case isGetEvents(command):
    return formatEvents(rawOutput)
  case isLogs(command):
    return formatLogs(rawOutput, 50)
  case isGetServices(command):
    return formatServices(rawOutput)
  case isGetSecrets(command):
    return formatSecrets(rawOutput) // names only, no data
  case isHelmList(command):
    return formatHelmReleases(rawOutput)
  default:
    return truncate(rawOutput, 2000) // fallback: cap at 2000 chars
  }
}
```

### What each formatter strips

| Resource | Keep | Strip |
|---|---|---|
| Deployment | replicas, image, conditions, recent events | managedFields, last-applied-config, uid, resourceVersion, labels (if standard) |
| Pod | phase, container status, restart count, node | volumes detail, tolerations, service account details |
| Service | type, ports, selector, endpoints count | session affinity, IP families |
| Secret | name, type, key names | data values (security!), annotations |
| ConfigMap | name, key names, first 100 chars of values | full values if >100 chars |
| Events | type, reason, message, count | full timestamps, source details |
| Logs | last N lines, error lines highlighted | everything beyond N lines |

### Escape hatch

`smart_output: false` bypasses formatting and returns raw output. The agent
can request this when it needs the full JSON (e.g., for jq processing).

Also: if the command already includes `-o json` or `-o yaml`, respect that
and return raw (the agent explicitly asked for structured output).

## Server Modes

### Mode 1: Standalone (new default)

```bash
evidra-mcp --evidence-dir /tmp/evidence
```

Tools: prescribe_smart, prescribe_full, report, get_event, **run_command**

The agent uses `run_command` for all infrastructure operations. Evidence
is auto-recorded for mutations.

### Mode 2: Proxy (existing)

```bash
evidra-mcp --proxy -- kubectl-mcp-server
```

Wraps an upstream MCP server. Intercepts tool calls and auto-records
evidence. No `run_command` tool — uses upstream's tools.

### Mode 3: Evidence-only (existing)

```bash
evidra-mcp --no-commands
```

Only evidence tools. No command execution. For agents that have their
own command execution.

## Integration with AgentGateway

### Config (agentgateway config.yaml)

```yaml
servers:
  - name: evidra
    url: http://evidra-mcp:3001/mcp
    transport: streamable-http
```

One server. Agent gets kubectl + helm + terraform + aws + evidence
through one MCP connection.

### Docker Compose

```yaml
evidra-mcp:
  image: ghcr.io/vitas/evidra-mcp:latest
  command: ["-transport", "streamable-http", "-port", "3001",
            "-evidence-dir", "/tmp/evidence"]
  environment:
    - KUBECONFIG=/kube/config
  volumes:
    - kubeconfig:/kube
```

Replaces both `mcp-backend` (kubectl-mcp-server) and the current
evidence-only `evidra-mcp` service.

## Implementation Plan

### Phase 1: run_command tool (4 hours)

1. Add `run_command` handler to `pkg/mcpserver/`
2. Command parsing, allowlist, blocked subcommands
3. Mutation detection with auto-prescribe/report
4. Register tool in `NewServer()`
5. Tests

### Phase 2: Smart output (4 hours)

1. Output parser for deployments, pods, services, events, logs
2. JSON stripping (managedFields, last-applied-config)
3. Describe summarizer
4. Log truncation with error highlighting
5. Escape hatch (`smart_output: false`, `-o json`)
6. Benchmark: measure token reduction

### Phase 3: AgentGateway integration (2 hours)

1. Update kagent-bench docker-compose.yml
2. Remove kubectl-mcp-server service
3. Replace with single evidra-mcp service
4. Test with kagent end-to-end

## Success Metrics

- **Token reduction:** 90%+ on typical diagnosis operations
- **Drop-in replacement:** kagent works with zero prompt changes
- **Evidence coverage:** 100% of mutations auto-recorded
- **Latency overhead:** <10ms per command (parsing + formatting)

## Why This Wins

1. **For agent vendors:** One MCP server instead of two. Less config, less maintenance.
2. **For agent users:** 20x cheaper operations. Better agent decisions from cleaner context.
3. **For the ecosystem:** Every agent using evidra-mcp automatically generates evidence. The more agents use it, the more data flows through evidra.
4. **For the hackathon:** "We replaced kubectl-mcp-server with something 20x more efficient. Here's the proof."

## What Stays Private

- The 51 exam scenarios (proprietary certification content)
- Certification grading logic
- Behavioral signal detection algorithms

## What's Open Source

- evidra-mcp server with run_command + smart output
- Evidence recording (prescribe/report protocol)
- Command allowlist and mutation detection
- AgentGateway integration config
