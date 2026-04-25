import type {
  ApiUser, ApiCampaign, ApiCreative, ApiUserTransaction, ApiPromocode,
  ApiNotification, StatsQueryRequest, StatsQueryResponse, StatsSummary,
  AuthResponse, AuthTokens, ApiEnvelope,
} from "./types";

// -- Persistence (so the mock survives page reloads) -----------------------
const STORAGE_KEY = "twinbid_mock_state_v1";
const SESSION_KEY = "twinbid_mock_session_v1";

const now = () => new Date().toISOString();
const uid = () => Math.random().toString(36).slice(2, 10);

const defaultUser: ApiUser = {
  login: "demo@twinbid.com",
  mail: "demo@twinbid.com",
  name: "Demo User",
  telegram: null,
  manager_telegram: "GregTwinbid",
  balance: 0,
  timezone: "utc_3",
  email_notifications: true,
  campaign_status_notifications: true,
  low_balance_notifications: true,
  campaign_balanse_notifications: true,
  balance_treshold: 100,
};

interface MockState {
  user: ApiUser;
  campaigns: ApiCampaign[];
  creatives: ApiCreative[];
  transactions: ApiUserTransaction[];
  notifications: ApiNotification[];
}

function loadState(): MockState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (parsed.topups && !parsed.transactions) {
        parsed.transactions = parsed.topups;
        delete parsed.topups;
      }
      return parsed;
    }
  } catch { /* ignore */ }
  return { user: { ...defaultUser }, campaigns: [], creatives: [], transactions: [], notifications: [] };
}
function saveState() {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(state)); } catch { /* ignore */ }
}

const state: MockState = loadState();

interface MockSession { user_id: string; email: string; full_name: string; }
function loadSession(): MockSession | null {
  try {
    const raw = localStorage.getItem(SESSION_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch { return null; }
}
function saveSession(s: MockSession | null) {
  try {
    if (s) localStorage.setItem(SESSION_KEY, JSON.stringify(s));
    else localStorage.removeItem(SESSION_KEY);
  } catch { /* ignore */ }
}

function delay<T>(v: T, ms = 80): Promise<T> {
  return new Promise(r => setTimeout(() => r(v), ms));
}

/** Wrap a value into a successful envelope (with delay). */
function ok<T>(v: T, ms?: number): Promise<ApiEnvelope<T>> {
  return delay({ success: true, errorMsg: "", data: v } as ApiEnvelope<T>, ms);
}
/** Wrap an error message into a failed envelope. */
function fail<T>(errorMsg: string, ms?: number): Promise<ApiEnvelope<T>> {
  return delay({ success: false, errorMsg } as ApiEnvelope<T>, ms);
}

// -- Promo fixtures (so the UI has something to validate against) ----------
const promoFixtures: Record<string, { id: string; bonus_percent: number }> = {
  TWINBID25:  { id: "promo-twinbid25",  bonus_percent: 25 },
  WELCOME10:  { id: "promo-welcome10",  bonus_percent: 10 },
};

/**
 * Raw provider shape — every method resolves to `ApiEnvelope<T>`. Both the
 * mock and http providers implement this interface; `api/index.ts` unwraps
 * envelopes (throwing `ApiError(errorMsg)` on `success: false`) so callers
 * see plain `T`.
 */
export const mockProvider = {
  // -- auth ---------------------------------------------------------------
  async signup(body: { email: string; password: string; full_name?: string; manager_telegram: string }): Promise<ApiEnvelope<AuthResponse>> {
    state.user = {
      ...defaultUser,
      login: body.email,
      mail: body.email,
      name: body.full_name || "",
      manager_telegram: body.manager_telegram,
    };
    saveState();
    saveSession({ user_id: "mock-user", email: body.email, full_name: body.full_name || "" });
    return ok({ access_token: "mock-access", refresh_token: "mock-refresh", user: state.user });
  },
  async login(body: { email: string; password: string }): Promise<ApiEnvelope<AuthResponse>> {
    state.user = { ...state.user, login: body.email, mail: body.email };
    saveState();
    saveSession({ user_id: "mock-user", email: body.email, full_name: state.user.name });
    return ok({ access_token: "mock-access", refresh_token: "mock-refresh", user: state.user });
  },
  async refresh(_body: { refresh_token: string }): Promise<ApiEnvelope<AuthTokens>> {
    return ok({ access_token: "mock-access", refresh_token: "mock-refresh" });
  },
  async logout(): Promise<ApiEnvelope<void>> {
    saveSession(null);
    return ok(undefined as unknown as void);
  },
  async getSession(): Promise<ApiEnvelope<MockSession | null>> {
    return ok(loadSession());
  },
  async changePassword(_body: { new_password: string }): Promise<ApiEnvelope<void>> {
    return ok(undefined as unknown as void);
  },

  // -- profile ------------------------------------------------------------
  async getProfile(): Promise<ApiEnvelope<ApiUser>> { return ok(state.user); },
  async patchProfile(patch: Partial<ApiUser>): Promise<ApiEnvelope<ApiUser>> {
    state.user = { ...state.user, ...patch };
    saveState();
    return ok(state.user);
  },

  // -- campaigns ----------------------------------------------------------
  async listCampaigns(): Promise<ApiEnvelope<{ items: ApiCampaign[]; total: number }>> {
    return ok({ items: state.campaigns, total: state.campaigns.length });
  },
  async getCampaign(id: string): Promise<ApiEnvelope<ApiCampaign | undefined>> {
    const c = state.campaigns.find(c => c.campaing_id === id);
    if (!c) return fail("Campaign not found");
    return ok(c);
  },
  async createCampaign(body: Omit<ApiCampaign, "campaing_id" | "user_id" | "cum_done_dollars">): Promise<ApiEnvelope<ApiCampaign>> {
    const c: ApiCampaign = { ...body, campaing_id: uid(), user_id: "mock-user", cum_done_dollars: 0 };
    state.campaigns.unshift(c);
    saveState();
    return ok(c);
  },
  async patchCampaign(id: string, patch: Partial<ApiCampaign>): Promise<ApiEnvelope<ApiCampaign>> {
    const i = state.campaigns.findIndex(c => c.campaing_id === id);
    if (i < 0) return fail("Campaign not found");
    state.campaigns[i] = { ...state.campaigns[i], ...patch };
    saveState();
    return ok(state.campaigns[i]);
  },
  async deleteCampaign(id: string): Promise<ApiEnvelope<void>> {
    state.campaigns = state.campaigns.filter(c => c.campaing_id !== id);
    state.creatives = state.creatives.filter(cr => cr.campaign_id !== id);
    saveState();
    return ok(undefined as unknown as void);
  },

  // -- creatives ----------------------------------------------------------
  async readCreatives(campaignId: string): Promise<ApiEnvelope<ApiCreative[]>> {
    return ok(state.creatives.filter(c => c.campaign_id === campaignId));
  },
  async createCreative(
    campaignId: string,
    body: Omit<ApiCreative, "id" | "campaign_id">,
    file?: File,
    filename?: string,
  ): Promise<ApiEnvelope<ApiCreative>> {
    const c = { ...body, id: uid(), campaign_id: campaignId } as ApiCreative;
    if (file) {
      const dataUrl = await new Promise<string>((resolve, reject) => {
        const r = new FileReader();
        r.onload = () => resolve(String(r.result));
        r.onerror = () => reject(r.error);
        r.readAsDataURL(file);
      });
      (c as any).name = filename || file.name;
      (c as any).presigned_s3_url = dataUrl;
    }
    state.creatives.push(c);
    saveState();
    return ok(c);
  },
  async patchCreative(
    id: string,
    patch: Partial<ApiCreative>,
    file?: File,
    filename?: string,
  ): Promise<ApiEnvelope<ApiCreative>> {
    const i = state.creatives.findIndex(c => c.id === id);
    if (i < 0) return fail("Creative not found");
    state.creatives[i] = { ...state.creatives[i], ...patch } as ApiCreative;
    if (file) {
      const dataUrl = await new Promise<string>((resolve, reject) => {
        const r = new FileReader();
        r.onload = () => resolve(String(r.result));
        r.onerror = () => reject(r.error);
        r.readAsDataURL(file);
      });
      (state.creatives[i] as any).name = filename || file.name;
      (state.creatives[i] as any).presigned_s3_url = dataUrl;
    }
    saveState();
    return ok(state.creatives[i]);
  },
  async deleteCreative(id: string): Promise<ApiEnvelope<void>> {
    state.creatives = state.creatives.filter(c => c.id !== id);
    saveState();
    return ok(undefined as unknown as void);
  },

  // -- transactions -------------------------------------------------------
  async listTransactions(): Promise<ApiEnvelope<{ items: ApiUserTransaction[]; total: number }>> {
    return ok({ items: state.transactions, total: state.transactions.length });
  },
  async createTransaction(
    body: Omit<ApiUserTransaction, "id" | "created_at" | "updated_at">,
  ): Promise<ApiEnvelope<ApiUserTransaction>> {
    const t: ApiUserTransaction = {
      ...body,
      id: uid(),
      created_at: now(),
      updated_at: now(),
    };
    state.transactions.unshift(t);
    saveState();
    return ok(t);
  },
  async patchTransaction(id: string, patch: Partial<ApiUserTransaction>): Promise<ApiEnvelope<ApiUserTransaction>> {
    const i = state.transactions.findIndex(t => t.id === id);
    if (i < 0) return fail("Transaction not found");
    state.transactions[i] = { ...state.transactions[i], ...patch, updated_at: now() };
    saveState();
    return ok(state.transactions[i]);
  },
  async cancelTransaction(id: string): Promise<ApiEnvelope<ApiUserTransaction>> {
    return this.patchTransaction(id, { status: "cancelled" });
  },

  // -- promo --------------------------------------------------------------
  async getPromocode(code: string): Promise<ApiEnvelope<ApiPromocode>> {
    const upper = code.trim().toUpperCase();
    const fix = promoFixtures[upper];
    if (!fix) return fail("Promocode not found");
    return ok({
      id: fix.id,
      promocode_text: upper,
      bonus_percent: fix.bonus_percent,
      usage_count: 0,
      usage_limit: null,
      valid_from: null,
      valid_to: null,
    });
  },

  // -- notifications ------------------------------------------------------
  async listNotifications(): Promise<ApiEnvelope<ApiNotification[]>> { return ok(state.notifications); },
  async createNotification(body: Omit<ApiNotification, "id" | "user_id" | "status">): Promise<ApiEnvelope<ApiNotification>> {
    const n: ApiNotification = { ...body, id: uid(), user_id: "mock-user", status: "active" };
    state.notifications.push(n);
    saveState();
    return ok(n);
  },
  async patchNotification(id: string, patch: Partial<ApiNotification>): Promise<ApiEnvelope<ApiNotification>> {
    const i = state.notifications.findIndex(n => n.id === id);
    if (i < 0) return fail("Notification not found");
    state.notifications[i] = { ...state.notifications[i], ...patch };
    saveState();
    return ok(state.notifications[i]);
  },

  // -- ClickHouse stats ---------------------------------------------------
  async statsQuery(req: StatsQueryRequest): Promise<ApiEnvelope<StatsQueryResponse>> {
    const ids = req.campaign_ids?.length ? req.campaign_ids : state.campaigns.map(c => c.campaing_id);
    const groupBy = req.group_by?.[0] || "campaign";

    const rng = (seed: string) => {
      let h = 0;
      for (let i = 0; i < seed.length; i++) h = (Math.imul(31, h) + seed.charCodeAt(i)) | 0;
      return () => { h = Math.imul(h ^ (h >>> 16), 0x45d9f3b); h = Math.imul(h ^ (h >>> 13), 0x45d9f3b); return ((h ^ (h >>> 16)) >>> 0) / 4294967296; };
    };

    const buckets = (() => {
      switch (groupBy) {
        case "date": {
          const out: string[] = [];
          const today = new Date();
          for (let i = 29; i >= 0; i--) {
            const d = new Date(today); d.setDate(d.getDate() - i);
            out.push(d.toISOString().slice(0, 10));
          }
          return out;
        }
        case "hour": {
          const out: string[] = [];
          const today = new Date();
          for (let i = 13; i >= 0; i--) {
            const d = new Date(today); d.setDate(d.getDate() - i);
            const day = d.toISOString().slice(0, 10);
            for (let h = 0; h < 24; h++) out.push(`${day} ${String(h).padStart(2, "0")}:00`);
          }
          return out;
        }
        case "country":     return ["US","GB","DE","FR","BR","IN","JP","RU","AU","CA","ES","IT","KR","TR","PL"];
        case "browser":     return ["Chrome","Safari","Firefox","Edge","Opera","Samsung Internet"];
        case "device_type": return ["Mobile","Desktop","Tablet","Smart TV"];
        case "os":          return ["Android","iOS","Windows","macOS","Linux","ChromeOS"];
        case "language":    return ["en","ru","de","fr","es","pt","it","ja","ko"];
        case "format":      return ["banner","push","popunder","native"];
        case "site_id":     return ["site_landing_1","site_banner_top","site_video_pre","site_native_feed","site_push_main","site_pop_exit"];
        case "creative":    return state.creatives.filter(c => ids.includes(c.campaign_id)).map(c => c.id);
        case "campaign":
        default:            return ids;
      }
    })();

    let totalImp = 0, totalClicks = 0, totalSpent = 0;
    const rows = buckets.map(bucket => {
      const r = rng(`${groupBy}|${bucket}|${ids.join(",")}`);
      const impressions = Math.floor(r() * 80000) + 1000;
      const clicks = Math.floor(impressions * (0.005 + r() * 0.03));
      const spent = Math.round((impressions / 1000) * (0.5 + r() * 2.5) * 100) / 100;
      const ctr = impressions > 0 ? Number(((clicks / impressions) * 100).toFixed(2)) : 0;
      totalImp += impressions; totalClicks += clicks; totalSpent += spent;
      return { [groupBy]: bucket, impressions, clicks, spent, ctr };
    });

    const totalCtr = totalImp > 0 ? Number(((totalClicks / totalImp) * 100).toFixed(2)) : 0;
    return ok({ rows, totals: { impressions: totalImp, clicks: totalClicks, spent: totalSpent, ctr: totalCtr } });
  },
  async statsCampaignSummary(id: string): Promise<ApiEnvelope<StatsSummary>> {
    const env = await this.statsQuery({ from: "", to: "", campaign_ids: [id], group_by: ["campaign"] });
    const r = env.data?.rows[0];
    return ok(r
      ? { impressions: Number(r.impressions), clicks: Number(r.clicks), spent: Number(r.spent), ctr: Number(r.ctr) }
      : { impressions: 0, clicks: 0, spent: 0, ctr: 0 });
  },
  async statsOverview(): Promise<ApiEnvelope<StatsSummary>> {
    const env = await this.statsQuery({ from: "", to: "", group_by: ["campaign"] });
    return ok(env.data?.totals ?? { impressions: 0, clicks: 0, spent: 0, ctr: 0 });
  },
};

/** Raw provider type — every method returns an `ApiEnvelope<T>`. */
export type RawApiProvider = typeof mockProvider;

/**
 * Unwrapped provider type — same method names, but each return promise
 * resolves to plain `T` (envelope unwrapped by `api/index.ts`).
 */
type Unwrap<T> = T extends ApiEnvelope<infer U> ? U : T;
type UnwrapPromise<T> = T extends Promise<infer U> ? Unwrap<U> : Unwrap<T>;
export type ApiProvider = {
  [K in keyof RawApiProvider]: RawApiProvider[K] extends (...args: infer A) => Promise<infer R>
    ? (...args: A) => Promise<UnwrapPromise<Promise<R>>>
    : never;
};
