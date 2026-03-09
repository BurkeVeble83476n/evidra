# Prometheus Positioning Backlog

This backlog captures the next two cheap positioning changes after the category wedge update.

## 1. Make The First User Journey Observability-First

Goal:
- Make the primary product story `run/record -> signal_summary + score_band -> alert/dashboard`.

Why:
- Prometheus and Grafana buyers care first about what they can alert on and visualize, not about architecture internals.
- The current landing flow still spends too much attention on system shape before user outcome.

Cheap implementation scope:
- Reorder the landing page so the first illustrated workflow is observability-first.
- Demote self-hosted details lower on the page.
- Add one compact `install -> run -> alert` path near the hero or directly after it.

Acceptance criteria:
- The first workflow shown on the landing page is `run/record -> signal_summary + score_band -> alert/dashboard`.
- Self-hosted is visibly secondary to CLI/MCP in the first screenful.
- README and landing page use the same observability-first sequence.

## 2. Add One Concrete Prometheus Outcome Block

Goal:
- Answer `what would I alert on tomorrow if I installed this?`

Why:
- Without one concrete metric and one concrete alert/dashboard example, the product still reads as a framework instead of an observability tool.

Cheap implementation scope:
- Add one metrics example to the landing page and README.
- Add one PromQL alert example.
- Link both to the observability guide.

Acceptance criteria:
- Landing page includes one real metric example such as `evidra_signal_total` or a score-band metric.
- Landing page or README includes one PromQL alert example tied to a real operator outcome.
- The example explicitly mentions Prometheus and Grafana, not generic “observability backends”.
