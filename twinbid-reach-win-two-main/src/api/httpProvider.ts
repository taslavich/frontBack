import { http } from "./http";
import type {
  ApiUser, ApiCampaign, ApiCreative, ApiUserTransaction, ApiPromocode,
  ApiNotification, StatsQueryRequest, StatsQueryResponse, StatsSummary,
  AuthResponse, AuthTokens,
} from "./types";
import type { ApiProvider } from "./mockProvider";

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? "";

/** Build a multipart body that carries JSON fields plus an optional file+filename. */
function buildCreativeForm(body: Record<string, unknown>, file?: File, filename?: string): FormData {
  const fd = new FormData();
  for (const [k, v] of Object.entries(body)) {
    if (v === undefined || v === null) continue;
    fd.append(k, typeof v === "string" ? v : JSON.stringify(v));
  }
  if (file) {
    fd.append("file", file, filename || file.name);
    fd.append("filename", filename || file.name);
  }
  return fd;
}

function authHeaders(): Record<string, string> {
  const tok = localStorage.getItem("twinbid_access_token");
  return tok ? { Authorization: `Bearer ${tok}` } : {};
}

async function multipart<T>(url: string, method: "POST" | "PATCH", fd: FormData): Promise<T> {
  const r = await fetch(`${API_BASE}${url}`, { method, headers: authHeaders(), body: fd });
  if (!r.ok) throw new Error(`${method} ${url} failed: ${r.status}`);
  return r.json() as Promise<T>;
}

// HTTP implementation. Hits the real backend described in API_CONTRACT.md.
export const httpProvider: ApiProvider = {
  // auth
  signup: (body) => http<AuthResponse>("/api/auth/signup", { method: "POST", body, auth: false }),
  login:  (body) => http<AuthResponse>("/api/auth/login",  { method: "POST", body, auth: false }),
  refresh:(body) => http<AuthTokens>  ("/api/auth/refresh",{ method: "POST", body, auth: false }),
  logout: ()     => http<void>        ("/api/auth/logout", { method: "POST" }),
  getSession:    () => http<{ user_id: string; email: string; full_name: string } | null>("/api/auth/session"),
  changePassword:(body) => http<void>("/api/auth/password", { method: "POST", body }),

  // profile
  getProfile:   ()     => http<ApiUser>("/api/profile"),
  patchProfile: (p)    => http<ApiUser>("/api/profile", { method: "PATCH", body: p }),

  // campaigns
  listCampaigns:   ()      => http<{ items: ApiCampaign[]; total: number }>("/api/campaigns"),
  getCampaign:     (id)    => http<ApiCampaign>(`/api/campaigns/${id}`),
  createCampaign:  (body)  => http<ApiCampaign>("/api/campaigns", { method: "POST", body }),
  patchCampaign:   (id,p)  => http<ApiCampaign>(`/api/campaigns/${id}`, { method: "PATCH", body: p }),
  deleteCampaign:  (id)    => http<void>(`/api/campaigns/${id}`, { method: "DELETE" }),

  // creatives — read uses the renamed endpoint, writes go as multipart so the
  // backend receives `file` + `filename` together with the rest of the fields.
  readCreatives:   (cid)         => http<ApiCreative[]>(`/api/campaigns/${cid}/creatives`),
  createCreative:  (cid, body, file, filename) =>
    multipart<ApiCreative>(`/api/campaigns/${cid}/creatives`, "POST",
      buildCreativeForm(body as Record<string, unknown>, file, filename)),
  patchCreative:   (id, p, file, filename) =>
    multipart<ApiCreative>(`/api/creatives/${id}`, "PATCH",
      buildCreativeForm(p as Record<string, unknown>, file, filename)),
  deleteCreative:  (id)          => http<void>(`/api/creatives/${id}`, { method: "DELETE" }),

  // transactions
  listTransactions:   ()        => http<{ items: ApiUserTransaction[]; total: number }>("/api/transactions"),
  createTransaction:  (body)    => http<ApiUserTransaction>("/api/transactions", { method: "POST", body }),
  patchTransaction:   (id, p)   => http<ApiUserTransaction>(`/api/transactions/${id}`, { method: "PATCH", body: p }),
  cancelTransaction:  (id)      => http<ApiUserTransaction>(`/api/transactions/${id}/cancel`, { method: "POST" }),

  // promo
  getPromocode: (code)    => http<ApiPromocode>(`/api/promocodes/${encodeURIComponent(code)}`),

  // notifications
  listNotifications:   ()        => http<ApiNotification[]>("/api/notifications", { query: { status: "active" } }),
  createNotification:  (body)    => http<ApiNotification>("/api/notifications", { method: "POST", body }),
  patchNotification:   (id, p)   => http<ApiNotification>(`/api/notifications/${id}`, { method: "PATCH", body: p }),

  // ClickHouse stats
  statsQuery:           (req)    => http<StatsQueryResponse>("/api/stats/query", { method: "POST", body: req }),
  statsCampaignSummary: (id)     => http<StatsSummary>(`/api/stats/campaign/${id}/summary`),
  statsOverview:        ()       => http<StatsSummary>("/api/stats/overview"),
};
