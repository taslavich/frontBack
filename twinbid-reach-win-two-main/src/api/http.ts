import { ACCESS_TOKEN_KEY, API_BASE_URL } from "./config";

export class ApiError extends Error {
  status: number;
  code?: string;
  fields?: Record<string, string>;
  constructor(status: number, message: string, code?: string, fields?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

interface RequestOptions {
  method?: "GET" | "POST" | "PATCH" | "DELETE" | "PUT";
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined | null>;
  auth?: boolean;
  signal?: AbortSignal;
}

function buildUrl(path: string, query?: RequestOptions["query"]): string {
  const url = new URL(path.startsWith("http") ? path : `${API_BASE_URL}${path}`);
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null) continue;
      url.searchParams.set(k, String(v));
    }
  }
  return url.toString();
}

export async function http<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, query, auth = true, signal } = opts;
  const headers: Record<string, string> = { Accept: "application/json" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth) {
    const token = localStorage.getItem(ACCESS_TOKEN_KEY);
    if (token) headers.Authorization = `Bearer ${token}`;
  }

  let res: Response;
  try {
    res = await fetch(buildUrl(path, query), {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal,
    });
  } catch (error) {
    const message =
      error instanceof TypeError
        ? "Network error: failed to reach API. Check VITE_API_BASE_URL, backend availability, CORS, and HTTPS certificate."
        : "Network error: request failed before receiving a response.";
    throw new ApiError(0, message, "NETWORK_ERROR");
  }

  if (res.status === 204) return undefined as T;

  let data: any = null;
  const text = await res.text();
  if (text) {
    try { data = JSON.parse(text); } catch { data = text; }
  }

  if (!res.ok) {
    // Backend may return either `{ error: { message, code, fields } }`
    // or a flat `{ success: false, errorMsg: "..." }` envelope. Surface
    // whichever is present.
    const err = data?.error;
    const flatMsg = data?.errorMsg;
    throw new ApiError(
      res.status,
      err?.message || flatMsg || (typeof data === "string" ? data : `HTTP ${res.status}`),
      err?.code,
      err?.fields,
    );
  }
  // Return as-is. Envelope handling (`{ success, errorMsg, data }`) is done
  // centrally in `api/index.ts` so both providers behave the same.
  return data as T;
}
