import { useState, useCallback } from "react";

interface CodeBlockProps {
  code: string;
  className?: string;
}

export function CodeBlock({ code, className = "" }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(code).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  }, [code]);

  return (
    <div
      className={`relative bg-[var(--color-code-bg,var(--color-bg-alt))] border border-border rounded-[10px] overflow-hidden ${className}`}
    >
      <div className="flex items-center gap-1.5 px-4 py-2.5 bg-[var(--color-code-header,var(--color-accent-tint))] border-b border-border">
        <span className="w-2 h-2 rounded-full bg-red-400" />
        <span className="w-2 h-2 rounded-full bg-yellow-400" />
        <span className="w-2 h-2 rounded-full bg-emerald-400" />
      </div>
      <button
        onClick={handleCopy}
        aria-label="Copy code"
        className="absolute top-2 right-3 bg-bg-elevated border border-border rounded px-2 py-0.5 cursor-pointer text-xs font-mono text-fg-muted transition-all hover:border-accent hover:text-fg"
      >
        {copied ? "copied!" : "copy"}
      </button>
      <pre className="px-6 py-5 overflow-x-auto font-mono text-sm leading-7 text-fg-body whitespace-pre">
        {code}
      </pre>
    </div>
  );
}
