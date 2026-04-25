import type {
  ApiUser, ApiCampaign, ApiCreative, ApiUserTransaction, ApiPromocode,
  ApiNotification, StatsQueryRequest, StatsQueryResponse, StatsSummary,
  AuthResponse, AuthTokens,
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
      // Backwards compat: old key was `topups`.
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

// -- Promo fixtures (so the UI has something to validate against) ----------
const promoFixtures: Record<string, { id: string; bonus_percent: number }> = {
  TWINBID25:  { id: "promo-twinbid25",  bonus_percent: 25 },
  WELCOME10:  { id: "promo-welcome10",  bonus_percent: 10 },
};

export const mockProvider = {
  // -- auth ---------------------------------------------------------------
  async signup(body: { email: string; password: string; full_name?: string; manager_telegram: string }): Promise<AuthResponse> {
    state.user = {
      ...defaultUser,
      login: body.email,
      mail: body.email,
      name: body.full_name || "",
      manager_telegram: body.manager_telegram,
    };
    saveState();
    saveSession({ user_id: "mock-user", email: body.email, full_name: body.full_name || "" });
    return delay({ access_token: "mock-access", refresh_token: "mock-refresh", user: state.user });
  },
  async login(body: { email: string; password: string }): Promise<AuthResponse> {
    state.user = { ...state.user, login: body.email, mail: body.email };
    saveState();
    saveSession({ user_id: "mock-user", email: body.email, full_name: state.user.name });
    return delay({ access_token: "mock-access", refresh_token: "mock-refresh", user: state.user });
  },
  async refresh(_body: { refresh_token: string }): Promise<AuthTokens> {
    return delay({ access_token: "mock-access", refresh_token: "mock-refresh" });
  },
  async logout(): Promise<void> {
    saveSession(null);
    return delay(undefined);
  },
  /** Mock helper: returns the local "session" so AuthContext can hydrate on boot. */
  async getSession(): Promise<MockSession | null> {
    return delay(loadSession());
  },
  async changePassword(_body: { new_password: string }): Promise<void> {
    return delay(undefined);
  },

  // -- profile ------------------------------------------------------------
  async getProfile(): Promise<ApiUser> { return delay(state.user); },
  async patchProfile(patch: Partial<ApiUser>): Promise<ApiUser> {
    state.user = { ...state.user, ...patch };
    saveState();
    return delay(state.user);
  },

  // -- campaigns ----------------------------------------------------------
  async listCampaigns(): Promise<{ items: ApiCampaign[]; total: number }> {
    return delay({ items: state.campaigns, total: state.campaigns.length });
  },
  async getCampaign(id: string): Promise<ApiCampaign | undefined> {
    return delay(state.campaigns.find(c => c.campaing_id === id));
  },
  async createCampaign(body: Omit<ApiCampaign, "campaing_id" | "user_id" | "cum_done_dollars">): Promise<ApiCampaign> {
    const c: ApiCampaign = { ...body, campaing_id: uid(), user_id: "mock-user", cum_done_dollars: 0 };
    state.campaigns.unshift(c);
    saveState();
    return delay(c);
  },
  async patchCampaign(id: string, patch: Partial<ApiCampaign>): Promise<ApiCampaign> {
    const i = state.campaigns.findIndex(c => c.campaing_id === id);
    if (i >= 0) state.campaigns[i] = { ...state.campaigns[i], ...patch };
    saveState();
    return delay(state.campaigns[i]);
  },
  async deleteCampaign(id: string): Promise<void> {
    state.campaigns = state.campaigns.filter(c => c.campaing_id !== id);
    state.creatives = state.creatives.filter(cr => cr.campaign_id !== id);
    saveState();
    return delay(undefined);
  },

  // -- creatives ----------------------------------------------------------
  /** Renamed read method per API contract. */
  async readCreatives(campaignId: string): Promise<ApiCreative[]> {
    return delay(state.creatives.filter(c => c.campaign_id === campaignId));
  },
  async createCreative(
    campaignId: string,
    body: Omit<ApiCreative, "id" | "campaign_id">,
    file?: File,
    filename?: string,
  ): Promise<ApiCreative> {
    const c = { ...body, id: uid(), campaign_id: campaignId } as ApiCreative;
    if (file) {
      // Mock backend: store base64 data URL as the presigned read URL.
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
    return delay(c);
  },
  async patchCreative(
    id: string,
    patch: Partial<ApiCreative>,
    file?: File,
    filename?: string,
  ): Promise<ApiCreative> {
    const i = state.creatives.findIndex(c => c.id === id);
    if (i >= 0) {
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
    }
    saveState();
    return delay(state.creatives[i]);
  },
  async deleteCreative(id: string): Promise<void> {
    state.creatives = state.creatives.filter(c => c.id !== id);
    saveState();
    return delay(undefined);
  },

  // -- transactions -------------------------------------------------------
  async listTransactions(): Promise<{ items: ApiUserTransaction[]; total: number }> {
    return delay({ items: state.transactions, total: state.transactions.length });
  },
  /**
   * Front-end sends every field it can compute (`user_id`, `transaction_time`,
   * `total_balance_increase`, etc.). Backend only assigns the primary `id` and
   * stamps `created_at` / `updated_at`.
   */
  async createTransaction(
    body: Omit<ApiUserTransaction, "id" | "created_at" | "updated_at">,
  ): Promise<ApiUserTransaction> {
    const t: ApiUserTransaction = {
      ...body,
      id: uid(),
      created_at: now(),
      updated_at: now(),
    };
    state.transactions.unshift(t);
    saveState();
    return delay(t);
  },
  async patchTransaction(id: string, patch: Partial<ApiUserTransaction>): Promise<ApiUserTransaction> {
    const i = state.transactions.findIndex(t => t.id === id);
    if (i >= 0) state.transactions[i] = { ...state.transactions[i], ...patch, updated_at: now() };
    saveState();
    return delay(state.transactions[i]);
  },
  async cancelTransaction(id: string): Promise<ApiUserTransaction> {
    return this.patchTransaction(id, { status: "cancelled" });
  },

  // -- promo --------------------------------------------------------------
  async getPromocode(code: string): Promise<ApiPromocode> {
    const upper = code.trim().toUpperCase();
    const fix = promoFixtures[upper];
    if (!fix) throw new Error("Promocode not found");
    return delay({
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
  async listNotifications(): Promise<ApiNotification[]> { return delay(state.notifications); },
  async createNotification(body: Omit<ApiNotification, "id" | "user_id" | "status">): Promise<ApiNotification> {
    const n: ApiNotification = { ...body, id: uid(), user_id: "mock-user", status: "active" };
    state.notifications.push(n);
    saveState();
    return delay(n);
  },
  async patchNotification(id: string, patch: Partial<ApiNotification>): Promise<ApiNotification> {
    const i = state.notifications.findIndex(n => n.id === id);
    if (i >= 0) state.notifications[i] = { ...state.notifications[i], ...patch };
    saveState();
    return delay(state.notifications[i]);
  },

  // -- ClickHouse stats ---------------------------------------------------
  // Deterministic per-campaign synthetic stats so the UI shows lifelike numbers
  // in mock mode. Real backend will replace these with ClickHouse queries.
  async statsQuery(req: StatsQueryRequest): Promise<StatsQueryResponse> {
    const ids = req.campaign_ids?.length ? req.campaign_ids : state.campaigns.map(c => c.campaing_id);
    const groupBy = req.group_by?.[0] || "campaign";

    const rng = (seed: string) => {
      let h = 0;
      for (let i = 0; i < seed.length; i++) h = (Math.imul(31, h) + seed.charCodeAt(i)) | 0;
      return () => { h = Math.imul(h ^ (h >>> 16), 0x45d9f3b); h = Math.imul(h ^ (h >>> 13), 0x45d9f3b); return ((h ^ (h >>> 16)) >>> 0) / 4294967296; };
    };

    // Bucket labels per group_by dimension (mock dictionaries).
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
          // 14 days × 24 hours, label "YYYY-MM-DD HH:00"
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
      // Mix the bucket label with the campaign ids so swapping the campaign
      // selection actually changes the numbers.
      const r = rng(`${groupBy}|${bucket}|${ids.join(",")}`);
      const impressions = Math.floor(r() * 80000) + 1000;
      const clicks = Math.floor(impressions * (0.005 + r() * 0.03));
      const spent = Math.round((impressions / 1000) * (0.5 + r() * 2.5) * 100) / 100;
      const ctr = impressions > 0 ? Number(((clicks / impressions) * 100).toFixed(2)) : 0;
      totalImp += impressions; totalClicks += clicks; totalSpent += spent;
      return { [groupBy]: bucket, impressions, clicks, spent, ctr };
    });

    const totalCtr = totalImp > 0 ? Number(((totalClicks / totalImp) * 100).toFixed(2)) : 0;
    return delay({ rows, totals: { impressions: totalImp, clicks: totalClicks, spent: totalSpent, ctr: totalCtr } });
  },
  async statsCampaignSummary(id: string): Promise<StatsSummary> {
    const res = await this.statsQuery({ from: "", to: "", campaign_ids: [id], group_by: ["campaign"] });
    const r = res.rows[0];
    return r ? { impressions: Number(r.impressions), clicks: Number(r.clicks), spent: Number(r.spent), ctr: Number(r.ctr) }
             : { impressions: 0, clicks: 0, spent: 0, ctr: 0 };
  },
  async statsOverview(): Promise<StatsSummary> {
    const res = await this.statsQuery({ from: "", to: "", group_by: ["campaign"] });
    return res.totals;
  },
};

export type ApiProvider = typeof mockProvider;
