import { createContext, useContext, useState, type ReactNode } from "react";

export type EvidenceMode = "all" | "proxy" | "smart" | "direct";

interface EvidenceModeCtx {
  mode: EvidenceMode;
  setMode: (m: EvidenceMode) => void;
}

const VALID_MODES: EvidenceMode[] = ["all", "proxy", "smart", "direct"];

const EvidenceModeContext = createContext<EvidenceModeCtx>({
  mode: "all",
  setMode: () => {},
});

export function EvidenceModeProvider({ children }: { children: ReactNode }) {
  const [mode, setModeState] = useState<EvidenceMode>(() => {
    const saved = localStorage.getItem("evidra-bench-evidence-mode");
    if (saved && VALID_MODES.includes(saved as EvidenceMode)) return saved as EvidenceMode;
    return "all";
  });

  const setMode = (m: EvidenceMode) => {
    setModeState(m);
    localStorage.setItem("evidra-bench-evidence-mode", m);
  };

  return (
    <EvidenceModeContext.Provider value={{ mode, setMode }}>
      {children}
    </EvidenceModeContext.Provider>
  );
}

export function useEvidenceMode() {
  return useContext(EvidenceModeContext);
}
