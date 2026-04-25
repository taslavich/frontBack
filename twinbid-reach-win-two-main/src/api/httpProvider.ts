import { http } from "./http";
import type {
  ApiUser, ApiCampaign, ApiCreative, ApiUserTransaction, ApiPromocode,
  ApiNotification, StatsQueryRequest, StatsQueryResponse, StatsSummary,
  AuthResponse, AuthTokens, ApiEnvelope,
} from "./types";
import type { RawApiProvider } from "./mockProvider";

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

async function multipart<T>(url: string, method: "POST" | "PATCH", fd: FormData): Promise<ApiEnvelope<T>> {
  const r = await fetch(`${API_BASE}${url}`, { method, headers: authHeaders(), body: fd });
  let data: any = null;
  const text = await r.text();
  if (text) { try { data = JSON.parse(text); } catch { data = text; } }
  if (!r.ok) {
    return { success: false, errorMsg: data?.errorMsg || data?.error?.message || `HTTP ${r.status}` };
  }
  // Backend may return an envelope already, or a bare payload.
  if (data && typeof data === "object" && "success" in data) {
    return { errorMsg: "", ...(data as ApiEnvelope<T>) };
  }
  return { success: true, errorMsg: "", data: data as T };
}

// HTTP implementation. Every method returns `ApiEnvelope<T>`; `api/index.ts`
// unwraps it (throws on success:false, returns `data` on success:true) so
// callers see the same shape that mocks used to return directly.
export const httpProvider: RawApiProvider = {
  // auth
  signup: (body) => http<ApiEnvelope<AuthResponse>>("/api/auth/signup", { method: "POST", body, auth: false }),
  login:  (body) => http<ApiEnvelope<AuthResponse>>("/api/auth/login",  { method: "POST", body, auth: false }),
  refresh:(body) => http<ApiEnvelope<AuthTokens>>  ("/api/auth/refresh",{ method: "POST", body, auth: false }),
  logout: ()     => http<ApiEnvelope<void>>        ("/api/auth/logout", { method: "POST" }),
  getSession:    () => http<ApiEnvelope<{ user_id: string; email: string; full_name: string } | null>>("/api/auth/session"),
  changePassword:(body) => http<ApiEnvelope<void>>("/api/auth/password", { method: "POST", body }),

  // profile
  getProfile:   ()     => http<ApiEnvelope<ApiUser>>("/api/profile"),
  patchProfile: (p)    => http<ApiEnvelope<ApiUser>>("/api/profile", { method: "PATCH", body: p }),

  // campaigns
  listCampaigns:   ()      => http<ApiEnvelope<{ items: ApiCampaign[]; total: number }>>("/api/campaigns"),
  getCampaign:     (id)    => http<ApiEnvelope<ApiCampaign>>(`/api/campaigns/${id}`),
  createCampaign:  (body)  => http<ApiEnvelope<ApiCampaign>>("/api/campaigns", { method: "POST", body }),
  patchCampaign:   (id,p)  => http<ApiEnvelope<ApiCampaign>>(`/api/campaigns/${id}`, { method: "PATCH", body: p }),
  deleteCampaign:  (id)    => http<ApiEnvelope<void>>(`/api/campaigns/${id}`, { method: "DELETE" }),

  // creatives
  readCreatives:   (cid)         => http<ApiEnvelope<ApiCreative[]>>(`/api/campaigns/${cid}/creatives`),
  createCreative:  (cid, body, file, filename) =>
    multipart<ApiCreative>(`/api/campaigns/${cid}/creatives`, "POST",
      buildCreativeForm(body as Record<string, unknown>, file, filename)),
  patchCreative:   (id, p, file, filename) =>
    multipart<ApiCreative>(`/api/creatives/${id}`, "PATCH",
      buildCreativeForm(p as Record<string, unknown>, file, filename)),
  deleteCreative:  (id)          => http<ApiEnvelope<void>>(`/api/creatives/${id}`, { method: "DELETE" }),

  // transactions
  listTransactions:   ()        => http<ApiEnvelope<{ items: ApiUserTransaction[]; total: number }>>("/api/transactions"),
  createTransaction:  (body)    => http<ApiEnvelope<ApiUserTransaction>>("/api/transactions", { method: "POST", body }),
  patchTransaction:   (id, p)   => http<ApiEnvelope<ApiUserTransaction>>(`/api/transactions/${id}`, { method: "PATCH", body: p }),
  cancelTransaction:  (id)      => http<ApiEnvelope<ApiUserTransaction>>(`/api/transactions/${id}/cancel`, { method: "POST" }),

  // promo
  getPromocode: (code)    => http<ApiEnvelope<ApiPromocode>>(`/api/promocodes/${encodeURIComponent(code)}`),

  // notifications
  listNotifications:   ()        => http<ApiEnvelope<ApiNotification[]>>("/api/notifications", { query: { status: "active" } }),
  createNotification:  (body)    => http<ApiEnvelope<ApiNotification>>("/api/notifications", { method: "POST", body }),
  patchNotification:   (id, p)   => http<ApiEnvelope<ApiNotification>>(`/api/notifications/${id}`, { method: "PATCH", body: p }),

  // ClickHouse stats
  statsQuery:           (req)    => http<ApiEnvelope<StatsQueryResponse>>("/api/stats/query", { method: "POST", body: req }),
  statsCampaignSummary: (id)     => http<ApiEnvelope<StatsSummary>>(`/api/stats/campaign/${id}/summary`),
  statsOverview:        ()       => http<ApiEnvelope<StatsSummary>>("/api/stats/overview"),
};
