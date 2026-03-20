import { type DragEvent, useCallback } from "react";

interface PaletteItem {
  type: "stack" | "break" | "verify" | "trap";
  label: string;
  description: string;
  color: string;
  icon: string;
}

const ITEMS: PaletteItem[] = [
  {
    type: "stack",
    label: "Stack",
    description: "Application stack to deploy",
    color: "border-l-blue-500",
    icon: "\u{1F4E6}",
  },
  {
    type: "break",
    label: "Break",
    description: "How to break the infrastructure",
    color: "border-l-red-500",
    icon: "\u{1F4A5}",
  },
  {
    type: "verify",
    label: "Verify",
    description: "How to check the fix",
    color: "border-l-emerald-500",
    icon: "\u2713",
  },
  {
    type: "trap",
    label: "Trap",
    description: "Bad behavior to detect",
    color: "border-l-amber-500",
    icon: "\u26A0\uFE0F",
  },
];

export function Palette() {
  const onDragStart = useCallback(
    (event: DragEvent<HTMLDivElement>, nodeType: string) => {
      event.dataTransfer.setData("application/reactflow", nodeType);
      event.dataTransfer.effectAllowed = "move";
    },
    [],
  );

  return (
    <div className="w-[200px] shrink-0 border-r border-border-subtle bg-bg-alt overflow-y-auto">
      <div className="px-3 py-3">
        <h3 className="text-[0.72rem] font-bold text-fg-muted uppercase tracking-wider mb-3">
          Blocks
        </h3>
        <div className="flex flex-col gap-2">
          {ITEMS.map((item) => (
            <div
              key={item.type}
              draggable
              onDragStart={(e) => onDragStart(e, item.type)}
              className={`border-l-4 ${item.color} rounded-md bg-bg-elevated px-3 py-2 cursor-grab active:cursor-grabbing border border-border-subtle hover:border-border hover:shadow-[var(--shadow-card)] transition-all select-none`}
            >
              <div className="flex items-center gap-1.5 mb-0.5">
                <span className="text-sm">{item.icon}</span>
                <span className="text-[0.78rem] font-semibold text-fg">
                  {item.label}
                </span>
              </div>
              <div className="text-[0.68rem] text-fg-muted leading-snug">
                {item.description}
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="px-3 py-3 border-t border-border-subtle">
        <p className="text-[0.68rem] text-fg-muted leading-relaxed">
          Drag a block onto the canvas, then connect blocks together to define
          the puzzle flow.
        </p>
      </div>
    </div>
  );
}
