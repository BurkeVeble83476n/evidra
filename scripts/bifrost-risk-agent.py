#!/usr/bin/env python3
"""Bifrost OpenAI-compatible artifact risk assessor for Evidra benchmark experiments."""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run Bifrost artifact risk assessment")
    parser.add_argument("--model-id", required=True, help="Model id, for example anthropic/claude-3-5-haiku")
    parser.add_argument("--artifact", default=os.getenv("EVIDRA_ARTIFACT_PATH", ""), help="Path to artifact file")
    parser.add_argument("--expected-json", default=os.getenv("EVIDRA_EXPECTED_JSON", ""), help="Path to expected.json")
    parser.add_argument("--output", default=os.getenv("EVIDRA_AGENT_OUTPUT", ""), help="Output JSON path")
    parser.add_argument(
        "--prompt-file",
        default=os.getenv(
            "EVIDRA_PROMPT_FILE",
            str(Path(__file__).resolve().parents[1] / "prompts/experiments/runtime/system_instructions.txt"),
        ),
        help="Prompt instructions file path",
    )
    parser.add_argument("--temperature", type=float, default=0.0, help="Sampling temperature")
    parser.add_argument("--max-tokens", type=int, default=700, help="Max completion tokens")
    parser.add_argument(
        "--base-url",
        default=os.getenv("EVIDRA_BIFROST_BASE_URL", "http://localhost:8080/openai"),
        help="Bifrost OpenAI-compatible base URL (without /chat/completions suffix)",
    )
    parser.add_argument(
        "--bifrost-vk",
        default=os.getenv("EVIDRA_BIFROST_VK", ""),
        help="Optional Bifrost virtual key header value (x-bf-vk)",
    )
    parser.add_argument(
        "--auth-bearer",
        default=os.getenv("EVIDRA_BIFROST_AUTH_BEARER", ""),
        help="Optional bearer token for Authorization header",
    )
    parser.add_argument(
        "--extra-headers-json",
        default=os.getenv("EVIDRA_BIFROST_EXTRA_HEADERS_JSON", ""),
        help="Optional JSON object merged into request headers",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=float,
        default=float(os.getenv("EVIDRA_BIFROST_TIMEOUT_SECONDS", "120")),
        help="HTTP timeout in seconds",
    )
    return parser.parse_args()


def fail(msg: str) -> None:
    print(f"bifrost-risk-agent: FAIL {msg}", file=sys.stderr)
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
    try:
        data = json.loads(txt)
        if isinstance(data, dict):
            return data
    except json.JSONDecodeError:
        pass

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


def extract_message_content(response_obj: dict) -> str:
    choices = response_obj.get("choices", [])
    if not isinstance(choices, list) or not choices:
        return ""
    msg = choices[0].get("message", {})
    if not isinstance(msg, dict):
        return ""
    content = msg.get("content", "")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for block in content:
            if isinstance(block, dict):
                if block.get("type") == "text" and isinstance(block.get("text"), str):
                    parts.append(block["text"])
                elif isinstance(block.get("content"), str):
                    parts.append(block["content"])
        return "\n".join(parts)
    return ""


def post_chat_completions(
    base_url: str,
    headers: dict[str, str],
    payload: dict,
    timeout_seconds: float,
) -> dict:
    url = base_url.rstrip("/") + "/chat/completions"
    req = urllib.request.Request(
        url=url,
        data=json.dumps(payload, ensure_ascii=True).encode("utf-8"),
        headers=headers,
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout_seconds) as resp:  # noqa: S310
            body = resp.read().decode("utf-8")
    except urllib.error.HTTPError as exc:
        try:
            body = exc.read().decode("utf-8")
        except Exception:  # pragma: no cover
            body = ""
        fail(f"http {exc.code} from bifrost endpoint: {body[:300]}")
    except urllib.error.URLError as exc:
        fail(f"connection error to bifrost endpoint: {exc.reason}")

    try:
        obj = json.loads(body)
    except json.JSONDecodeError as exc:
        fail(f"invalid JSON from bifrost endpoint: {exc}")
    if not isinstance(obj, dict):
        fail("bifrost endpoint returned non-object JSON")
    return obj


def main() -> None:
    args = parse_args()

    if not args.output:
        fail("missing output path (pass --output or set EVIDRA_AGENT_OUTPUT)")

    artifact_text = read_text(args.artifact, "artifact")
    prompt_text = read_text(args.prompt_file, "prompt file")
    expected = load_expected(args.expected_json)

    contract_version = parse_contract_version(prompt_text)
    system_prompt = strip_contract_header(prompt_text)
    user_prompt = build_user_prompt(artifact_text, expected)

    headers: dict[str, str] = {"Content-Type": "application/json"}
    if args.bifrost_vk:
        headers["x-bf-vk"] = args.bifrost_vk
    if args.auth_bearer:
        headers["Authorization"] = f"Bearer {args.auth_bearer}"
    if args.extra_headers_json:
        try:
            parsed_headers = json.loads(args.extra_headers_json)
        except json.JSONDecodeError as exc:
            fail(f"invalid --extra-headers-json: {exc}")
        if not isinstance(parsed_headers, dict):
            fail("--extra-headers-json must decode to an object")
        for key, value in parsed_headers.items():
            headers[str(key)] = str(value)

    payload = {
        "model": args.model_id,
        "temperature": args.temperature,
        "max_tokens": args.max_tokens,
        "messages": [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": user_prompt},
        ],
    }

    response_obj = post_chat_completions(
        base_url=args.base_url,
        headers=headers,
        payload=payload,
        timeout_seconds=args.timeout_seconds,
    )
    parsed = extract_json(extract_message_content(response_obj))
    out = normalize_output(parsed)
    out["prompt_contract_version"] = contract_version
    out["model_id"] = args.model_id
    out["bifrost_base_url"] = args.base_url

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(out, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
