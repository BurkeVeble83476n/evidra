import { useState, useEffect } from "react";

type Status = "checking" | "healthy" | "unhealthy";

export function useHealthCheck(endpoint: string, intervalMs = 30000) {
  const [status, setStatus] = useState<Status>("checking");

  useEffect(() => {
    const controller = new AbortController();
    const check = () => {
      fetch(endpoint, { signal: controller.signal })
        .then((r) => setStatus(r.ok ? "healthy" : "unhealthy"))
        .catch(() => {
          if (!controller.signal.aborted) setStatus("unhealthy");
        });
    };
    check();
    const id = setInterval(check, intervalMs);
    return () => {
      clearInterval(id);
      controller.abort();
    };
  }, [endpoint, intervalMs]);

  return status;
}
