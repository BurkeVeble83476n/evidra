#!/usr/bin/env python3
"""LiteLLM artifact risk assessor for Evidra benchmark experiments.

Reads artifact + expected metadata and writes normalized JSON to EVIDRA_AGENT_OUTPUT.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run LiteLLM artifact risk assessment")
    parser.add_argument("--model-id", required=True, help="LiteLLM model id, e.g. anthropic/claude-3-5-haiku")
    parser.add_argument("--artifact", default=os.getenv("EVIDRA_ARTIFACT_PATH", ""), help="Path to artifact file")
    parser.add_argument("--expected-json", default=os.getenv("EVIDRA_EXPECTED_JSON", ""), help="Path to expected.json")
    parser.add_argument("--output", default=os.getenv("EVIDRA_AGENT_OUTPUT", ""), help="Output JSON path")
    parser.add_argument(
        "--prompt-file",
        default=os.getenv(
            "EVIDRA_PROMPT_FILE",
            str(Path(__file__).resolve().parents[1] / "prompts/experiments/litellm/system_instructions.txt"),
        ),
        help="Prompt instructions file path",
    )
    parser.add_argument("--temperature", type=float, default=0.0, help="Sampling temperature")
    parser.add_argument("--max-tokens", type=int, default=700, help="Max completion tokens")
    return parser.parse_args()


def fail(msg: str) -> None:
    print(f"litellm-risk-agent: FAIL {msg}", file=sys.stderr)
    raise SystemExit(1)


def read_text(path: str, what: str) -> str:
    if not path:
        fail(f"missing {what} path")
    p = Path(path)
    if not p.is_file():
        fail(f"{what} not found: {path}")
    return p.read_text(encoding="utf-8")


def parse_contract_version(text: str) -> str:
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line:
            continue
        if line.startswith("<!--") and line.endswith("-->"):
            line = line[4:-3].strip()
        if line.startswith("#"):
            line = line[1:].strip()
        if not line.lower().startswith("contract:"):
            return "unknown"
        value = line.split(":", 1)[1].strip()
        return value if value else "unknown"
    return "unknown"


def strip_contract_header(text: str) -> str:
    out = []
    skipped = False
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not skipped and (not line):
            continue
        if not skipped:
            probe = line
            if probe.startswith("<!--") and probe.endswith("-->"):
                probe = probe[4:-3].strip()
            if probe.startswith("#"):
                probe = probe[1:].strip()
            if probe.lower().startswith("contract:"):
                skipped = True
                continue
        skipped = True
        out.append(raw_line)
    return "\n".join(out).strip()


def load_expected(path: str) -> dict:
    if not path:
        return {}
    p = Path(path)
    if not p.is_file():
        return {}
    try:
        return json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return {}


def build_user_prompt(artifact_text: str, expected: dict) -> str:
    case_id = str(expected.get("case_id", "unknown"))
    category = str(expected.get("category", "unknown"))
    difficulty = str(expected.get("difficulty", "unknown"))
    return (
        "Assessment mode: classify this infrastructure artifact.\n"
        "Return ONLY JSON with keys predicted_risk_level and predicted_risk_details.\n"
        f"case_id={case_id}\n"
        f"category={category}\n"
        f"difficulty={difficulty}\n\n"
        "Artifact:\n"
        "-----BEGIN ARTIFACT-----\n"
        f"{artifact_text}\n"
        "-----END ARTIFACT-----\n"
    )


def extract_json(text: str) -> dict:
    txt = text.strip()
    # First try direct parse
    try:
        data = json.loads(txt)
        if isinstance(data, dict):
            return data
    except json.JSONDecodeError:
        pass

    # Fallback: first JSON object slice
    match = re.search(r"\{.*\}", txt, flags=re.DOTALL)
    if match:
        try:
            data = json.loads(match.group(0))
            if isinstance(data, dict):
                return data
        except json.JSONDecodeError:
            pass
    return {}


def normalize_output(raw: dict) -> dict:
    level = str(raw.get("predicted_risk_level", raw.get("risk_level", "unknown")) or "unknown").strip().lower()
    allowed_levels = {"low", "medium", "high", "critical", "unknown"}
    if level not in allowed_levels:
        level = "unknown"

    details = raw.get("predicted_risk_details", raw.get("predicted_risk_tags", raw.get("risk_tags", [])))
    if not isinstance(details, list):
        details = []
    clean_details = sorted({str(x).strip() for x in details if str(x).strip()})

    return {"predicted_risk_level": level, "predicted_risk_details": clean_details}


def main() -> None:
    args = parse_args()

    try:
        from litellm import completion
    except Exception as exc:  # pragma: no cover
        fail(f"litellm import failed: {exc}")

    if not args.output:
        fail("missing output path (pass --output or set EVIDRA_AGENT_OUTPUT)")

    artifact_text = read_text(args.artifact, "artifact")
    prompt_text = read_text(args.prompt_file, "prompt file")
    expected = load_expected(args.expected_json)

    contract_version = parse_contract_version(prompt_text)
    system_prompt = strip_contract_header(prompt_text)
    user_prompt = build_user_prompt(artifact_text, expected)

    response = completion(
        model=args.model_id,
        temperature=args.temperature,
        max_tokens=args.max_tokens,
        messages=[
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
    )

    content = response.choices[0].message.content if response and response.choices else ""
    parsed = extract_json(content or "")
    out = normalize_output(parsed)
    out["prompt_contract_version"] = contract_version
    out["model_id"] = args.model_id

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(out, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
