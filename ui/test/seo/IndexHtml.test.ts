import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const TITLE = "Evidra — Evidence Protocol and Benchmark for AI Infrastructure Agents";
const DESCRIPTION =
  "Evidra is the evidence protocol and benchmark for AI infrastructure agents. Record intent, outcome, and refusal with the prescribe/report lifecycle, analyze reliability across agents, pipelines, and GitOps controllers, and evaluate agent behavior on real infrastructure failures.";
const KEYWORDS =
  "AI infrastructure agents, MCP, infrastructure benchmark, agent benchmark, evidence protocol, prescribe report, GitOps reliability, Argo CD, CI/CD reliability";

function loadDocument() {
  const html = readFileSync(resolve(process.cwd(), "index.html"), "utf8");
  return new DOMParser().parseFromString(html, "text/html");
}

function metaContent(doc: Document, selector: string) {
  return doc.querySelector(selector)?.getAttribute("content");
}

describe("index.html SEO metadata", () => {
  it("reflects the current project goals and benchmark positioning", () => {
    const doc = loadDocument();

    expect(doc.title).toBe(TITLE);
    expect(metaContent(doc, 'meta[name="description"]')).toBe(DESCRIPTION);
    expect(metaContent(doc, 'meta[name="keywords"]')).toBe(KEYWORDS);
    expect(metaContent(doc, 'meta[name="robots"]')).toBe("index, follow");
    expect(metaContent(doc, 'meta[property="og:title"]')).toBe(TITLE);
    expect(metaContent(doc, 'meta[property="og:description"]')).toBe(DESCRIPTION);
    expect(metaContent(doc, 'meta[property="og:type"]')).toBe("website");
    expect(metaContent(doc, 'meta[name="twitter:card"]')).toBe("summary");
    expect(metaContent(doc, 'meta[name="twitter:title"]')).toBe(TITLE);
    expect(metaContent(doc, 'meta[name="twitter:description"]')).toBe(DESCRIPTION);
  });
});
