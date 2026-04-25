// Shared API types. Mirror the contract in API_CONTRACT.md.
// These are the *transport* shapes (snake_case) the backend will return.
// UI contexts may map them into their own camelCase view models.

export type CampaignStatus = "active" | "paused" | "draft" | "completed" | "moderation";
export type PricingModel = "cpm" | "cpc";
export type TrafficType = "mainstream" | "adult" | "mixed";
export type FormatType = "banner" | "popunder" | "native" | "push";
export type TopupStatus = "draft" | "pending" | "approved" | "rejected" | "cancelled";
export type NotificationType = "incomplete_topup" | "low_balance" | "campaign_status" | "other";
export type NotificationStatus = "active" | "inactive";

/** HASHMAP targeting: { value: 1 (white) | 0 (black) }. Empty = no targeting. */
export type TargetingMap = Record<string, 0 | 1>;

/** Schedule interval like ["mon,1", "thu,2"]. */
export type ScheduleInterval = [string, string];

export interface ApiUser {
  login: string;
  mail: string;
  name: string;
  telegram: string | null;
  manager_telegram: string;
  balance: number;
  timezone: string;
  email_notifications: boolean;
  campaign_status_notifications: boolean;
  low_balance_notifications: boolean;
  campaign_balanse_notifications: boolean;
  balance_treshold: number;
}

export interface ApiCampaign {
  campaing_id: string;
  user_id: string;
  campaign_name: string;
  format_type: FormatType;
  brand_name?: string | null;
  h?: number | null;
  w?: number | null;
  status: CampaignStatus;
  traffic_type: TrafficType;
  vertical: string[];
  pricing_model: PricingModel;
  base_price_cpm: number;
  base_price_cpc: number;
  evenness_by_slot_mode: boolean;
  goal_total_dollars: number;
  /** filled by backend only */
  cum_done_dollars: number;
  start_ts: string;
  end_ts: string;
  active_intervals: ScheduleInterval[];
  country: TargetingMap;
  language: TargetingMap;
  device_type: TargetingMap;
  os: TargetingMap;
  browser: TargetingMap;
  site_id: TargetingMap;
  ip: TargetingMap;
}

export interface ApiCreativeBase {
  id: string;
  campaign_id: string;
  creative_name: string;
  link: string;
  trackers_macros: Record<string, 0 | 1>;
}
export interface ApiPopCreative extends ApiCreativeBase {}
export interface ApiBanCreative extends ApiCreativeBase {
  w: number;
  h: number;
  /** File name of the uploaded creative image (set by backend on upload). */
  name?: string;
  /** Presigned read URL returned by the backend (GET creative). Not sent on write. */
  presigned_s3_url?: string;
}
export interface ApiIppCreative extends ApiCreativeBase {
  title: string;
  description: string;
  name?: string;
  presigned_s3_url?: string;
}
export interface ApiNatCreative extends ApiCreativeBase {
  title: string;
  description: string;
  name?: string;
  presigned_s3_url?: string;
}
export type ApiCreative =
  | ApiPopCreative
  | ApiBanCreative
  | ApiIppCreative
  | ApiNatCreative;

export interface ApiUserTransaction {
  id: string;
  user_id: string;
  transaction_time: string;
  transaction_id: string;
  payment_method: string;
  bonus_amount: number;
  promocode_id: string | null;
  transaction_hash: string | null;
  deposit_amount: number;
  total_balance_increase: number;
  status: TopupStatus;
  currency: string;
  created_at: string;
  updated_at: string;
}

export interface ApiPromocode {
  id: string;
  promocode_text: string;
  bonus_percent: number;
  usage_count: number;
  usage_limit: number | null;
  valid_from: string | null;
  valid_to: string | null;
}

export interface ApiNotification {
  id: string;
  user_id: string;
  transaction_id: string | null;
  campaign_id: string | null;
  deposit_amount: number | null;
  status: NotificationStatus;
  text: string;
  type: NotificationType;
}

// ---- ClickHouse statistics ----
export type StatsGroupBy =
  | "date" | "hour" | "campaign" | "country" | "format"
  | "creative" | "os" | "browser" | "device_type" | "language" | "site_id";

export interface StatsQueryRequest {
  from: string; // YYYY-MM-DD
  to: string;
  campaign_ids?: string[];
  group_by: StatsGroupBy[];
  filters?: Partial<Record<StatsGroupBy, string[]>>;
}

export interface StatsRow {
  // any of the group_by keys present
  [key: string]: string | number | undefined;
  impressions: number;
  clicks: number;
  spent: number;
  ctr: number;
}

export interface StatsQueryResponse {
  rows: StatsRow[];
  totals: { impressions: number; clicks: number; spent: number; ctr: number };
}

export interface StatsSummary {
  impressions: number;
  clicks: number;
  spent: number;
  ctr: number;
}

// ---- Auth ----
export interface AuthTokens {
  access_token: string;
  refresh_token: string;
}
export interface AuthResponse extends AuthTokens {
  user: ApiUser;
}
