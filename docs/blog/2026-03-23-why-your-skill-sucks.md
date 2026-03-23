# Why Your Skill Sucks

You wrote a skill prompt for your AI agent. It looks great — diagnosis protocol, safety rules, operational discipline. Your agent fixes broken deployments 4x faster.

Ship it?

We tested role-based skills across 12 real infrastructure scenarios on 4 models. Here's what happened.

## The Setup

[infra-bench](https://lab.evidra.cc) runs AI agents against real Kubernetes clusters and Terraform projects. No mocks. Kind clusters, real kubectl, real failures. The agent gets a task ("the deployment is broken"), tools (kubectl, terraform, helm), and a turn budget. Fix it or fail.

We tested two modes:
- **Baseline**: no skill — the model uses its own judgment
- **With skill**: a compact ~300-token role prompt (k8s-admin for Kubernetes, platform-eng for Terraform)

Same model, same scenarios, same cluster. The only difference: did we tell the agent how to think?

## The Results

### Kubernetes Scenarios (8 scenarios, L2-L3)

| Model | Baseline | With k8s-admin skill | Delta |
|---|---|---|---|
| Claude Sonnet 4 | **8/8** | **8/8** | 0 |
| Gemini 2.5 Flash | 6/8 | 5/8 | **-1** |
| GPT-4o | 4/6 | 4/8 | **-2** |
| DeepSeek Chat | 6/7 | 6/8 | 0 |

### Terraform Scenarios (4 scenarios, L2-L3)

| Model | Baseline | With platform-eng skill | Delta |
|---|---|---|---|
| Claude Sonnet 4 | 3/4 | **4/4** | **+1** |
| Gemini 2.5 Flash | 3/4 | 2/4 | **-1** |
| GPT-4o | 2/4 | 2/4 | 0 |
| DeepSeek Chat | 3/4 | 3/4 | 0 |

## The Pattern

**Strong models don't need your skill.** Claude Sonnet 4 scored 8/8 on Kubernetes without any skill. Adding the k8s-admin skill didn't improve anything — it was already diagnosing before fixing, checking blast radius, making targeted changes. The skill just described what it was already doing.

**Weak models get hurt by your skill.** GPT-4o lost 2 scenarios when we added the k8s-admin skill. The skill says "check events and conditions before logs." For a kubeconfig connectivity issue, the agent needed to inspect the kubeconfig file — not Kubernetes events. The skill imposed a wrong mental model.

**Skills help on specific tasks and break others.** The platform-eng skill helped Claude Sonnet pass terraform-import-existing (FAIL → PASS) because the skill specifically teaches "prefer import over destroy-recreate." But the same skill pattern made Gemini fail terraform-state-drift (PASS → FAIL) because it followed the skill's diagnostic protocol instead of just reading the plan diff.

## Why Skills Break Things

A skill prompt is a mental model injection. You're telling the agent: "think like THIS kind of engineer." That works when the scenario matches the model. It breaks when:

1. **The skill is too procedural.** "Run terraform plan first, then read .tf files, then check state" — great for state management, wrong for a simple image tag fix. The agent follows the procedure and burns turns on unnecessary diagnosis.

2. **The skill overrides good instincts.** A model that would naturally read the error message and fix it in 2 turns now follows your 5-step protocol and times out.

3. **The skill scope is wrong.** A k8s-admin skill teaches deployment patterns. But kubeconfig issues aren't deployment issues — the agent needs to think about TLS and cluster connectivity, not pod scheduling.

## The Real Problem

You can't know whether a skill helps without testing it on real scenarios. Prompt engineering intuition fails here. The skill that cuts L1 scenarios from 17 to 4 turns is the same skill that makes L2 scenarios fail entirely.

We proved this with our first skill experiment months ago:

```
Without skill: 17 turns, PASS (L1 broken-deployment)
With skill:     4 turns, PASS — 4x faster

Same skill, harder scenario:
Without skill: 12 turns, PASS (L2 crashloop-backoff)
With skill:     4 turns, FAIL — skipped diagnosis
```

The skill made the agent skip diagnosis and jump to a fix pattern. On L1 (obvious problem), that's a speedup. On L2 (requires investigation), it's a failure.

## What Actually Works

**For strong models (Claude Sonnet 4, GPT-5.2):** Don't add skills. They're already good. Your skill is at best neutral, at worst destructive.

**For mid-tier models (Gemini Flash, DeepSeek):** Test every skill variant against your actual scenarios. A skill that helps on 6 scenarios but breaks 2 is a net negative if those 2 are production-critical.

**For weak models (Llama 70B, Qwen):** Skills help more here — the structure compensates for weaker reasoning. But test anyway.

**The general rule:** Skills are not universally good or bad. You need to benchmark them against real infrastructure failures to know which help and which hurt.

## How to Test

```bash
# Baseline (no skill)
infra-bench certify --track cka --model sonnet --provider bifrost

# With skill
infra-bench certify --track cka --model sonnet --role k8s-admin --provider bifrost

# Compare
# Did pass rate go up? Did turns go down? Did any scenario flip from PASS to FAIL?
```

62 scenarios. 8 exam-aligned tracks. 5 models. Run your skill against real clusters and get data, not opinions.

**infra-bench**: [lab.evidra.cc](https://lab.evidra.cc) | **Results**: [lab.evidra.cc/results](https://lab.evidra.cc/results)

---

*Data from infra-bench v0.2.1, March 2026. Models tested: Claude Sonnet 4 (Anthropic), GPT-4o (OpenAI), Gemini 2.5 Flash (Google), DeepSeek Chat (DeepSeek), Llama 3.3 70B (Meta/Groq). All runs against real Kind clusters with proxy-mode evidence recording.*
