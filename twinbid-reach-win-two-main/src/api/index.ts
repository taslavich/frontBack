// Public entry point for the api layer.
// Switch between mocked fixtures and the real backend via VITE_USE_MOCK.
import { USE_MOCK } from "./config";
import { mockProvider } from "./mockProvider";
import { httpProvider } from "./httpProvider";

export const api = USE_MOCK ? mockProvider : httpProvider;
export * from "./types";
export { ApiError } from "./http";
export { API_BASE_URL, USE_MOCK, ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from "./config";
