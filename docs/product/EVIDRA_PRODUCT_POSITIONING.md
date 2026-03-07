# Evidra — Product Positioning

**Evidra — reliability benchmark for infrastructure automation.**

Evidra records infrastructure mutations, measures operational reliability, and produces scorecards.
It works across CI/CD pipelines, shell scripts, IaC workflows, and AI agents.
It is an inspector, not an enforcer: no blocking, only evidence and measurement.

---

## Pitch By Audience

**Platform team (without AI):**
"Evidra records every terraform apply / kubectl deploy in your CI, measures operational reliability, and shows a scorecard. You see retry storms, broken deployments, and config drift before they become incidents."

**Platform team (with AI agents):**
"Your AI agents already run kubectl apply. Evidra measures how reliably they do it. Connect evidra-mcp and the agent reports what it planned and what happened. The score shows which agent is ready for production and which is only safe for staging."

**CTO / CISO:**
"Evidra is a flight recorder plus a reliability score for infrastructure automation. Like a black box for planes, but for your CI/CD and AI agents. Tamper-evident evidence chain, signed entries, compliance-ready audit trail."

---

## Messaging Rule

Use **including AI**, not **for AI**.
Evidra is for all infrastructure automation, with AI agents as one high-value use case.
This lowers adoption friction (no AI program required to start) and keeps strategic upside (when AI expands, Evidra already measures it).

---

## MCP Positioning

MCP is an integration point, not a product feature.

"Evidra speaks MCP — any AI agent that supports MCP can report to Evidra out of the box. Claude Code, Cursor, custom agents: plug in and measure."
