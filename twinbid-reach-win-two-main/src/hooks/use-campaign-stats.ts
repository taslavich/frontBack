import { useEffect, useState } from "react";
import { api } from "@/api";
import type { StatsSummary } from "@/api/types";

export interface CampaignStatsMap {
  byId: Map<string, StatsSummary>;
  totals: StatsSummary;
  loading: boolean;
}

const EMPTY: StatsSummary = { impressions: 0, clicks: 0, spent: 0, ctr: 0 };

/**
 * Fetches per-campaign aggregated stats from the ClickHouse layer (`api.statsQuery`).
 * Pass an empty array to fetch nothing. The hook re-runs whenever the joined ids change.
 */
export function useCampaignStats(campaignIds: string[]): CampaignStatsMap {
  const [byId, setById] = useState<Map<string, StatsSummary>>(new Map());
  const [totals, setTotals] = useState<StatsSummary>(EMPTY);
  const [loading, setLoading] = useState(false);
  const key = campaignIds.join(",");

  useEffect(() => {
    if (campaignIds.length === 0) {
      setById(new Map());
      setTotals(EMPTY);
      return;
    }
    let cancelled = false;
    setLoading(true);
    api.statsQuery({ from: "", to: "", campaign_ids: campaignIds, group_by: ["campaign"] })
      .then(res => {
        if (cancelled) return;
        const map = new Map<string, StatsSummary>();
        for (const row of res.rows) {
          const id = String(row.campaign ?? "");
          if (!id) continue;
          map.set(id, {
            impressions: Number(row.impressions) || 0,
            clicks: Number(row.clicks) || 0,
            spent: Number(row.spent) || 0,
            ctr: Number(row.ctr) || 0,
          });
        }
        setById(map);
        setTotals(res.totals);
      })
      .catch(e => { if (!cancelled) console.error("Stats query error:", e); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  return { byId, totals, loading };
}

/** Convenience: returns a stat field with a 0 fallback. */
export function statOf(map: Map<string, StatsSummary>, id: string): StatsSummary {
  return map.get(id) ?? EMPTY;
}
