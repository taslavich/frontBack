// Frontend API configuration. Real backend URL is provided via env;
// while it's not deployed we default to a local placeholder and run on mocks.

export const API_BASE_URL: string =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "http://localhost:8080";

/** When true, the api layer returns mocked fixtures instead of hitting the backend. */
export const USE_MOCK: boolean =
  (import.meta.env.VITE_USE_MOCK as string | undefined) !== "false";

export const ACCESS_TOKEN_KEY = "twinbid_access_token";
export const REFRESH_TOKEN_KEY = "twinbid_refresh_token";
