import { createContext, useContext, useState, useCallback, useEffect } from "react";

interface AuthContextValue {
  apiKey: string | null;
  setApiKey: (key: string | null) => void;
  clearApiKey: () => void;
}

const AuthContext = createContext<AuthContextValue>({
  apiKey: null,
  setApiKey: () => {},
  clearApiKey: () => {},
});

const STORAGE_KEY = "evidra_api_key";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [apiKey, setApiKeyState] = useState<string | null>(() => {
    try {
      return localStorage.getItem(STORAGE_KEY);
    } catch {
      return null;
    }
  });

  const setApiKey = useCallback((key: string | null) => {
    setApiKeyState(key);
    try {
      if (key) {
        localStorage.setItem(STORAGE_KEY, key);
      } else {
        localStorage.removeItem(STORAGE_KEY);
      }
    } catch {
      // Storage unavailable — key lives in memory only.
    }
  }, []);

  const clearApiKey = useCallback(() => setApiKey(null), [setApiKey]);

  // Sync across tabs.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === STORAGE_KEY) {
        setApiKeyState(e.newValue);
      }
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  return (
    <AuthContext.Provider value={{ apiKey, setApiKey, clearApiKey }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
