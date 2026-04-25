// Public entry point for the api layer.
// Switch between mocked fixtures and the real backend via VITE_USE_MOCK.
//
// Both providers return `ApiEnvelope<T> = { success, errorMsg, data }`.
// This module wraps every method so callers receive plain `T` (envelope
// unwrapped). On `success: false` an `ApiError` is thrown carrying the
// backend's `errorMsg` — `runApi`/`notifyError` then surface it as a toast.

import { USE_MOCK } from "./config";
import { mockProvider, type ApiProvider, type RawApiProvider } from "./mockProvider";
import { httpProvider } from "./httpProvider";
import { ApiError } from "./http";
import type { ApiEnvelope } from "./types";

const raw: RawApiProvider = USE_MOCK ? mockProvider : httpProvider;

function unwrap<T>(env: ApiEnvelope<T>): T {
  if (!env || typeof env !== "object") return env as unknown as T;
  if (env.success === true) return env.data as T;
  throw new ApiError(200, env.errorMsg || "Request failed");
}

/** Build a proxy that calls `raw[method](...args)` then unwraps the envelope. */
function makeUnwrapped(provider: RawApiProvider): ApiProvider {
  const out: any = {};
  for (const key of Object.keys(provider) as (keyof RawApiProvider)[]) {
    const fn = (provider as any)[key];
    if (typeof fn !== "function") continue;
    out[key] = async (...args: unknown[]) => unwrap(await fn.apply(provider, args));
  }
  return out as ApiProvider;
}

export const api: ApiProvider = makeUnwrapped(raw);

export * from "./types";
export { ApiError } from "./http";
export { API_BASE_URL, USE_MOCK, ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from "./config";
