import { useState, useEffect } from "react";

type Status = "checking" | "healthy" | "unhealthy";

export function useHealthCheck(endpoint: string, intervalMs = 30000) {
  const [status, setStatus] = useState<Status>("checking");

  useEffect(() => {
    const check = () => {
      fetch(endpoint)
        .then((r) => setStatus(r.ok ? "healthy" : "unhealthy"))
        .catch(() => setStatus("unhealthy"));
    };
    check();
    const id = setInterval(check, intervalMs);
    return () => clearInterval(id);
  }, [endpoint, intervalMs]);

  return status;
}
