// Centralised UI feedback for API calls.
//
// The backend is expected to return responses that may include a
// `success: boolean` and `errorMsg: string` envelope. This module wraps
// promises so every user-triggered action shows a consistent toast
// (success message, or the backend error). A global feature flag
// (`window.error_showed`, set in `src/main.tsx`) lets us turn error
// surfacing on/off in one place.

import { toast } from "sonner";
import { ApiError } from "@/api/http";

export function isErrorShown(): boolean {
  // Default to true if the flag was not set yet.
  const w = (typeof window !== "undefined" ? (window as any) : {}) as Record<string, unknown>;
  return w.error_showed !== false;
}

function extractMessage(e: unknown): string {
  if (!e) return "Unknown error";
  if (e instanceof ApiError) {
    // ApiError stores either {message} parsed from the backend, or `errorMsg`
    // when it was forwarded by `http.ts`.
    return e.message || `HTTP ${e.status}`;
  }
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  try { return JSON.stringify(e); } catch { return String(e); }
}

/** Show success toast, but skip when `successMsg` is empty. */
export function notifySuccess(successMsg?: string) {
  if (successMsg) toast.success(successMsg);
}

/** Show error toast, gated by the global `error_showed` flag. */
export function notifyError(prefix: string, e: unknown) {
  if (!isErrorShown()) {
    console.error(prefix, e);
    return;
  }
  const msg = extractMessage(e);
  toast.error(prefix ? `${prefix}: ${msg}` : msg);
  console.error(prefix, e);
}

/**
 * Run an API call and surface the result through the toast system.
 * - On success: shows `successMsg` (if provided).
 * - On failure: shows `errorPrefix: <backend errorMsg | thrown message>`.
 *
 * Returns the resolved value, or `undefined` on error.
 */
export async function runApi<T>(
  fn: () => Promise<T>,
  opts: { successMsg?: string; errorPrefix?: string } = {},
): Promise<T | undefined> {
  try {
    const res = await fn();
    // If the backend returns an envelope with success:false, treat as error.
    const env = res as unknown as { success?: boolean; errorMsg?: string };
    if (env && typeof env === "object" && env.success === false) {
      notifyError(opts.errorPrefix || "Error", env.errorMsg || "Request failed");
      return undefined;
    }
    notifySuccess(opts.successMsg);
    return res;
  } catch (e) {
    notifyError(opts.errorPrefix || "Error", e);
    return undefined;
  }
}
