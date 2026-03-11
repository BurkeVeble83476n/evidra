import { useAuth } from "../context/AuthContext";
import { useCallback } from "react";

export interface ApiError {
  status: number;
  message: string;
}

export function useApi() {
  const { apiKey } = useAuth();

  const request = useCallback(
    async <T = unknown>(
      path: string,
      options: RequestInit = {},
    ): Promise<T> => {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
        ...(options.headers as Record<string, string>),
      };
      if (apiKey) {
        headers["Authorization"] = `Bearer ${apiKey}`;
      }

      const res = await fetch(path, { ...options, headers });
      if (!res.ok) {
        let message = res.statusText;
        try {
          const body = await res.json();
          message = body.error || body.message || message;
        } catch {
          // Non-JSON error body.
        }
        throw { status: res.status, message } as ApiError;
      }
      return res.json() as Promise<T>;
    },
    [apiKey],
  );

  return { request };
}
