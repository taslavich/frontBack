import { useMemo, useCallback, useEffect, useState } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Eye, MousePointer, Target, TrendingUp, ArrowUpDown, CalendarIcon, RefreshCw, Filter } from "lucide-react";
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts";
import { format, subDays } from "date-fns";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import type { DateRange } from "react-day-picker";
import { toast } from "sonner";
import { useCampaigns } from "@/contexts/CampaignContext";
import { useLanguage } from "@/contexts/LanguageContext";
import { useStatistics } from "@/contexts/StatisticsContext";
import { formatCountryLabel } from "@/lib/countries";
import { api } from "@/api";
import type { StatsGroupBy } from "@/api/types";

type GroupBy = "dates" | "hours" | "browsers" | "siteid" | "devices" | "os" | "country";
type SortKey = "label" | "impressions" | "clicks" | "spent";
type SortDir = "asc" | "desc";

interface UiRow { label: string; impressions: number; clicks: number; spent: number; }

// UI groupBy → ClickHouse group_by + bucket key in the response row.
const GROUP_MAP: Record<GroupBy, { api: StatsGroupBy }> = {
  dates:    { api: "date" },
  hours:    { api: "hour" },
  browsers: { api: "browser" },
  siteid:   { api: "site_id" },
  devices:  { api: "device_type" },
  os:       { api: "os" },
  country:  { api: "country" },
};

function formatDateLabel(iso: string): string {
  // YYYY-MM-DD → dd.MM.yyyy
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}
function formatHourLabel(raw: string): string {
  // "YYYY-MM-DD HH:00" → "dd.MM.yyyy HH:00"
  const [day, hour] = raw.split(" ");
  return `${formatDateLabel(day)} ${hour}`;
}

function seedRandom(seed: string) {
  let h = 0;
  for (let i = 0; i < seed.length; i++) { h = Math.imul(31, h) + seed.charCodeAt(i) | 0; }
  return () => { h = Math.imul(h ^ (h >>> 16), 0x45d9f3b); h = Math.imul(h ^ (h >>> 13), 0x45d9f3b); return ((h ^ (h >>> 16)) >>> 0) / 4294967296; };
}

// Dictionaries used purely for filter UI options.
const DIMENSION_MAP: Record<string, string[]> = {
  country: ["US","GB","DE","FR","BR","IN","JP","RU","AU","CA","ES","IT","KR","TR","PL"],
  browsers: ["Chrome","Safari","Firefox","Edge","Opera","Samsung Internet"],
  devices: ["Mobile","Desktop","Tablet","Smart TV"],
  os: ["Android","iOS","Windows","macOS","Linux","ChromeOS"],
};

// Multi-select filter component (supports plain string options or {value,label} pairs)
type FilterOption = string | { value: string; label: string };
function MultiSelectFilter({ label, options, selected, onChange }: {
  label: string; options: FilterOption[]; selected: Set<string>;
  onChange: React.Dispatch<React.SetStateAction<Set<string>>>;
}) {
  const { t } = useLanguage();
  const normalized = options.map(o => typeof o === "string" ? { value: o, label: o } : o);
  const toggle = (val: string) => {
    onChange(prev => {
      const next = new Set(prev);
      if (next.has(val)) next.delete(val); else next.add(val);
      return next;
    });
  };
  const displayText = selected.size === 0 ? t("stats.allValues") : `${selected.size} ${t("stats.selected")}`;

  return (
    <div className="flex flex-col gap-1">
      <Label className="text-xs text-muted-foreground">{label}</Label>
      <Popover>
        <PopoverTrigger asChild>
          <Button variant="outline" className="w-[220px] justify-start bg-background border-border h-8 text-sm font-normal text-left truncate">
            {displayText}
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-[260px] p-2" align="start">
          <div className="space-y-1 max-h-56 overflow-y-auto">
            <label className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted/50 cursor-pointer text-sm font-medium border-b border-border pb-2 mb-1">
              <Checkbox checked={selected.size === 0} onCheckedChange={(checked) => { if (checked) onChange(new Set()); }} />
              {t("stats.allValues")}
            </label>
            {normalized.map(opt => (
              <label key={opt.value} className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted/50 cursor-pointer text-sm">
                <Checkbox checked={selected.has(opt.value)} onCheckedChange={() => toggle(opt.value)} />
                <span className="truncate">{opt.label}</span>
              </label>
            ))}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}

export default function DashboardStatistics() {
  const { campaigns } = useCampaigns();
  const { t, lang } = useLanguage();
  
  const {
    selectedCampaignIds, setSelectedCampaignIds,
    selectedCreativeIds, setSelectedCreativeIds,
    dateRange, setDateRange,
    clickCount, setClickCount,
    filterCountry, setFilterCountry,
    filterBrowser, setFilterBrowser,
    filterDevice, setFilterDevice,
    filterOS, setFilterOS,
    groupBy, setGroupBy,
    chartMetric, setChartMetric,
    sortKey, setSortKey,
    sortDir, setSortDir,
    appliedCampaignIds, setAppliedCampaignIds,
    appliedCreativeIds, setAppliedCreativeIds,
    appliedDateRange, setAppliedDateRange,
    appliedFilterCountry, setAppliedFilterCountry,
    appliedFilterBrowser, setAppliedFilterBrowser,
    appliedFilterDevice, setAppliedFilterDevice,
    appliedFilterOS, setAppliedFilterOS,
  } = useStatistics();

  const appliedGroupBy = groupBy;

  const hasActiveFilters = appliedFilterCountry.size > 0 || appliedFilterBrowser.size > 0 || appliedFilterDevice.size > 0 || appliedFilterOS.size > 0;

  const groupLabels: Record<GroupBy, string> = {
    dates: t("stats.byDates"), hours: t("stats.byHours"), browsers: t("stats.byBrowsers"),
    siteid: t("stats.bySiteId"), devices: t("stats.byDevices"), os: t("stats.byOS"), country: t("stats.byCountry"),
  };

  const activeCampaigns = useMemo(() =>
    campaigns.filter(c => c.status === "active" || c.status === "completed" || c.status === "paused"),
    [campaigns]
  );

  const selectedCampaignId = useMemo(() => {
    if (selectedCampaignIds.size === 1) return Array.from(selectedCampaignIds)[0];
    return "";
  }, [selectedCampaignIds]);

  const availableCreatives = useMemo(() => {
    const result: { id: string; label: string }[] = [];
    if (!selectedCampaignId) return result;
    const campaign = campaigns.find(c => c.id === selectedCampaignId);
    if (!campaign) return result;
    (campaign.creatives || []).forEach((cr, idx) => {
      const creativeId = `${selectedCampaignId}.${idx + 1}`;
      const label = cr.name || cr.title || cr.url || `Creative #${idx + 1}`;
      result.push({ id: creativeId, label });
    });
    return result;
  }, [selectedCampaignId, campaigns]);

  // Country filter options with localized name + ISO code
  const countryOptions = useMemo(
    () => DIMENSION_MAP.country.map(code => ({ value: code, label: formatCountryLabel(code, lang) })),
    [lang]
  );

  // On mount: if nothing applied yet, auto-apply "all active campaigns" + last 7 days.
  useEffect(() => {
    if (appliedCampaignIds.size === 0 && activeCampaigns.length > 0) {
      const defaultRange: DateRange = { from: subDays(new Date(), 6), to: new Date() };
      setAppliedCampaignIds(new Set(activeCampaigns.map(c => c.id)));
      // Reflect defaults in the UI controls so the user sees what's applied
      if (!dateRange?.from) setDateRange(defaultRange);
      setAppliedDateRange(appliedDateRange ?? defaultRange);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeCampaigns]);

  const hasSelection = appliedCampaignIds.size > 0 && appliedDateRange?.from;

  const [data, setData] = useState<UiRow[]>([]);

  useEffect(() => {
    if (!hasSelection) { setData([]); return; }
    let cancelled = false;
    const apiGroup = GROUP_MAP[appliedGroupBy].api;
    const from = appliedDateRange?.from ? appliedDateRange.from.toISOString().slice(0, 10) : "";
    const to = appliedDateRange?.to ? appliedDateRange.to.toISOString().slice(0, 10) : from;
    const filters: Partial<Record<StatsGroupBy, string[]>> = {};
    if (appliedFilterCountry.size) filters.country = Array.from(appliedFilterCountry);
    if (appliedFilterBrowser.size) filters.browser = Array.from(appliedFilterBrowser);
    if (appliedFilterDevice.size)  filters.device_type = Array.from(appliedFilterDevice);
    if (appliedFilterOS.size)      filters.os = Array.from(appliedFilterOS);

    api.statsQuery({
      from, to,
      campaign_ids: Array.from(appliedCampaignIds),
      group_by: [apiGroup],
      filters,
    }).then(res => {
      if (cancelled) return;
      const rows: UiRow[] = res.rows.map(r => {
        const raw = String(r[apiGroup] ?? "");
        const label = apiGroup === "date" ? formatDateLabel(raw)
                    : apiGroup === "hour" ? formatHourLabel(raw)
                    : raw;
        return {
          label,
          impressions: Number(r.impressions) || 0,
          clicks: Number(r.clicks) || 0,
          spent: Number(r.spent) || 0,
        };
      });
      setData(rows);
    }).catch(e => { if (!cancelled) console.error("Stats query error:", e); });
    return () => { cancelled = true; };
  }, [appliedCampaignIds, appliedGroupBy, appliedDateRange, appliedFilterCountry, appliedFilterBrowser, appliedFilterDevice, appliedFilterOS, hasSelection]);

  const metricCards = useMemo(() => {
    const totalImpressions = data.reduce((s, r) => s + r.impressions, 0);
    const totalClicks = data.reduce((s, r) => s + r.clicks, 0);
    const totalSpent = data.reduce((s, r) => s + r.spent, 0);
    const ctr = totalImpressions > 0 ? ((totalClicks / totalImpressions) * 100).toFixed(2) : "0.00";
    return [
      { label: t("stats.impressions"), value: totalImpressions.toLocaleString(), icon: Eye },
      { label: t("stats.clicks"), value: totalClicks.toLocaleString(), icon: MousePointer },
      { label: t("stats.ctr"), value: `${ctr}%`, icon: Target },
      { label: t("stats.spent"), value: `$${totalSpent.toLocaleString()}`, icon: TrendingUp },
    ];
  }, [data, t]);

  useEffect(() => {
    if (appliedGroupBy === "dates") { setSortKey("label"); setSortDir("desc"); }
    else if (appliedGroupBy === "hours") { setSortKey("label"); setSortDir("asc"); }
    else { setSortKey("impressions"); setSortDir("desc"); }
  }, [appliedGroupBy]);


   const handleRefresh = useCallback(() => {
    // For "all" campaigns, add all active campaign ids
    if (selectedCampaignIds.size === 0) {
      setAppliedCampaignIds(new Set(activeCampaigns.map(c => c.id)));
    } else {
      setAppliedCampaignIds(new Set(selectedCampaignIds));
    }
    setAppliedCreativeIds(new Set(selectedCreativeIds));
    setAppliedDateRange(dateRange);
    setAppliedFilterCountry(filterCountry);
    setAppliedFilterBrowser(filterBrowser);
    setAppliedFilterDevice(filterDevice);
    setAppliedFilterOS(filterOS);
    toast.success(t("stats.refreshed"));
  }, [selectedCampaignIds, selectedCreativeIds, dateRange, filterCountry, filterBrowser, filterDevice, filterOS, t, activeCampaigns]);

  const handleCampaignSelect = (value: string) => {
    if (value === "all") {
      setSelectedCampaignIds(new Set());
    } else {
      setSelectedCampaignIds(new Set([value]));
    }
    // Reset creative selection when campaign changes
    setSelectedCreativeIds(new Set());
  };

  const handleDateChange = (range: DateRange | undefined) => {
    if (!range) return;
    if (clickCount === 0) { setDateRange({ from: range.from, to: undefined }); setClickCount(1); }
    else if (clickCount === 1) {
      if (range.from && range.to) { setDateRange(range); } else if (range.from) { setDateRange({ from: dateRange?.from, to: range.from }); }
      setClickCount(2);
    } else { setDateRange({ from: range.from || range.to, to: undefined }); setClickCount(1); }
  };

  const chartData = useMemo(() => {
    if (appliedGroupBy !== "dates" && appliedGroupBy !== "hours") return [];
    return [...data].sort((a, b) => a.label.localeCompare(b.label));
  }, [data, appliedGroupBy]);

  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => {
      if (sortKey === "label") return sortDir === "asc" ? a.label.localeCompare(b.label) : b.label.localeCompare(a.label);
      return sortDir === "desc" ? b[sortKey] - a[sortKey] : a[sortKey] - b[sortKey];
    });
  }, [data, sortKey, sortDir]);

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) setSortDir(d => d === "desc" ? "asc" : "desc");
    else { setSortKey(key); setSortDir("desc"); }
  };

  const SortIcon = ({ col }: { col: SortKey }) => (
    <ArrowUpDown className={cn("h-3 w-3 ml-1 inline shrink-0", sortKey === col ? "text-primary" : "text-muted-foreground")} />
  );

  const totals = useMemo(() => ({
    impressions: sortedData.reduce((s, r) => s + r.impressions, 0),
    clicks: sortedData.reduce((s, r) => s + r.clicks, 0),
    spent: sortedData.reduce((s, r) => s + r.spent, 0),
  }), [sortedData]);

  const labelHeader = appliedGroupBy === "dates" ? t("stats.date") : appliedGroupBy === "hours" ? t("stats.dateAndHour") : appliedGroupBy === "browsers" ? t("stats.browser") : appliedGroupBy === "siteid" ? "SiteID" : appliedGroupBy === "os" ? t("stats.os") : appliedGroupBy === "country" ? t("stats.country") : t("stats.device");
  const canSortByLabel = appliedGroupBy === "dates" || appliedGroupBy === "hours";

  // Custom tooltip for hours chart
  const HoursTooltip = ({ active, payload, label }: { active?: boolean; payload?: Array<{ value?: number }>; label?: string }) => {
    if (!active || !payload?.length) return null;
    const metricLabel = chartMetric === "impressions" ? t("stats.impressions") : chartMetric === "clicks" ? t("stats.clicks") : t("stats.spent");
    const value = payload[0]?.value;
    return (
      <div className="rounded-lg border bg-card px-3 py-2 text-sm shadow-md" style={{ borderColor: "hsl(var(--border))" }}>
        <p className="font-medium text-foreground">{label}</p>
        <p className="text-muted-foreground">{metricLabel}: <span className="font-semibold text-foreground">{chartMetric === "spent" ? `$${value?.toLocaleString()}` : value?.toLocaleString()}</span></p>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold">{t("stats.title")}</h2>
        <p className="text-muted-foreground text-sm">{t("stats.subtitle")}</p>
      </div>

      <div className="flex flex-wrap items-end gap-6">
        <div className="flex flex-col gap-2">
          <Label className="text-sm text-muted-foreground font-medium">{t("stats.campaigns")}</Label>
          <Select value={selectedCampaignId || "all"} onValueChange={handleCampaignSelect}>
            <SelectTrigger className="w-[280px] bg-background border-border">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("stats.allCampaigns")}</SelectItem>
              {activeCampaigns.map(c => (
                <SelectItem key={c.id} value={c.id}>
                  <span className="text-muted-foreground mr-1">{c.id}</span> — {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-2">
          <Label className="text-sm text-muted-foreground font-medium">{t("stats.creatives")}</Label>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="w-[260px] justify-start bg-background border-border text-left font-normal" disabled={!selectedCampaignId}>
                {!selectedCampaignId
                  ? t("stats.selectCreative")
                  : selectedCreativeIds.size === 0
                    ? t("stats.allCreatives")
                    : `${t("stats.selected")} ${selectedCreativeIds.size}`}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[320px] p-2" align="start">
              <div className="space-y-1 max-h-64 overflow-y-auto">
                <label className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted/50 cursor-pointer text-sm font-medium border-b border-border pb-2 mb-1">
                  <Checkbox checked={selectedCreativeIds.size === 0} onCheckedChange={(checked) => { if (checked) setSelectedCreativeIds(new Set()); }} />
                  {t("stats.allCreatives")}
                </label>
                {availableCreatives.map(cr => (
                  <label key={cr.id} className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-muted/50 cursor-pointer text-sm">
                    <Checkbox checked={selectedCreativeIds.has(cr.id)} onCheckedChange={() => {
                      setSelectedCreativeIds(prev => {
                        const next = new Set(prev);
                        if (next.has(cr.id)) next.delete(cr.id); else next.add(cr.id);
                        return next;
                      });
                    }} />
                    <span className="truncate">{cr.label}</span>
                  </label>
                ))}
              </div>
            </PopoverContent>
          </Popover>
        </div>

        <div className="flex flex-col gap-2">
          <Label className="text-sm text-muted-foreground font-medium">{t("stats.period")}</Label>
          <div className="flex items-center gap-2">
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" className="w-[220px] justify-start bg-background border-border text-left font-normal">
                  <CalendarIcon className="mr-2 h-4 w-4" />
                  {dateRange?.from ? (
                    dateRange.to && dateRange.from.getTime() !== dateRange.to.getTime() ? (
                      <>{format(dateRange.from, "dd.MM.yy")} — {format(dateRange.to, "dd.MM.yy")}</>
                    ) : format(dateRange.from, "dd.MM.yy")
                  ) : t("stats.selectPeriod")}
                </Button>
              </PopoverTrigger>
              <PopoverContent className="w-auto p-0" align="start">
                <Calendar mode="range" selected={dateRange} onSelect={handleDateChange} numberOfMonths={2} className="p-3 pointer-events-auto" />
              </PopoverContent>
            </Popover>
            {[
              { label: t("stats.today"), getRange: () => { const d = new Date(); return { from: d, to: d }; } },
              { label: t("stats.yesterday"), getRange: () => { const d = subDays(new Date(), 1); return { from: d, to: d }; } },
              { label: t("stats.week"), getRange: () => ({ from: subDays(new Date(), 6), to: new Date() }) },
              { label: t("stats.month"), getRange: () => ({ from: subDays(new Date(), 29), to: new Date() }) },
            ].map((preset) => (
              <Button key={preset.label} variant="outline" size="sm" className="border-border text-xs"
                onClick={() => setDateRange(preset.getRange())}>
                {preset.label}
              </Button>
            ))}
          </div>
        </div>

        <Button onClick={handleRefresh} className="bg-primary hover:bg-primary/90 text-primary-foreground gap-2">
          <RefreshCw className="h-4 w-4" /> {t("stats.refresh")}
        </Button>
      </div>

      {/* Filters */}
      <Card className="bg-card border-border">
        <CardContent className="p-4">
          <div className="flex items-center gap-2 mb-3">
            <Filter className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium">{t("stats.filters")}</span>
            {hasActiveFilters && (
              <Button variant="ghost" size="sm" className="text-xs h-6 px-2" onClick={() => { setFilterCountry(new Set()); setFilterBrowser(new Set()); setFilterDevice(new Set()); setFilterOS(new Set()); }}>
                {t("stats.clearFilters")}
              </Button>
            )}
          </div>
          <div className="flex flex-wrap gap-4">
            <MultiSelectFilter label={t("stats.filterCountry")} options={countryOptions} selected={filterCountry} onChange={setFilterCountry} />
            <MultiSelectFilter label={t("stats.filterBrowser")} options={DIMENSION_MAP.browsers} selected={filterBrowser} onChange={setFilterBrowser} />
            <MultiSelectFilter label={t("stats.filterDevice")} options={DIMENSION_MAP.devices} selected={filterDevice} onChange={setFilterDevice} />
            <MultiSelectFilter label={t("stats.filterOS")} options={DIMENSION_MAP.os} selected={filterOS} onChange={setFilterOS} />
          </div>
        </CardContent>
      </Card>

      {!hasSelection ? (
        <Card className="bg-card border-border">
          <CardContent className="py-16 text-center">
            <p className="text-muted-foreground">{t("stats.selectCampaignAndPeriod")}</p>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            {metricCards.map((m) => (
              <Card key={m.label} className="bg-card border-border">
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm text-muted-foreground">{m.label}</p>
                      <p className="text-2xl font-bold mt-1">{m.value}</p>
                    </div>
                    <div className="h-12 w-12 rounded-lg bg-muted flex items-center justify-center text-primary">
                      <m.icon className="h-6 w-6" />
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          {/* Chart */}
          {(appliedGroupBy === "dates" || appliedGroupBy === "hours") && chartData.length > 0 && (
            <Card className="bg-card border-border">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="text-lg">
                    {appliedGroupBy === "hours" ? t("stats.chartTitleHours") : t("stats.chartTitle")}
                  </CardTitle>
                  <div className="flex gap-1">
                    {(["impressions", "clicks", "spent"] as const).map(m => (
                      <Button key={m} variant={chartMetric === m ? "default" : "outline"} size="sm"
                        onClick={() => setChartMetric(m)}
                        className={cn("text-xs", chartMetric === m ? "bg-primary text-primary-foreground" : "border-border")}>
                        {m === "impressions" ? t("stats.impressions") : m === "clicks" ? t("stats.clicks") : t("stats.spent")}
                      </Button>
                    ))}
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <div className="h-[280px] animate-[reveal-chart_1.2s_ease-out_forwards]" style={{ clipPath: 'inset(0 0 0 0)' }}>
                  <style>{`
                    @keyframes reveal-chart {
                      from { clip-path: inset(0 100% 0 0); }
                      to { clip-path: inset(0 0 0 0); }
                    }
                  `}</style>
                  <ResponsiveContainer width="100%" height="100%">
                    <AreaChart data={chartData}>
                      <defs>
                        <linearGradient id="grad-metric" x1="0" y1="0" x2="0" y2="1">
                          <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
                          <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                        </linearGradient>
                      </defs>
                      <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                      <XAxis
                        dataKey="label"
                        stroke="hsl(var(--muted-foreground))"
                        fontSize={appliedGroupBy === "hours" ? 10 : 12}
                        tickFormatter={appliedGroupBy === "hours" ? (val: string) => val.split(" ")[1] || val : undefined}
                        interval={appliedGroupBy === "hours" ? "preserveStartEnd" : undefined}
                      />
                      <YAxis stroke="hsl(var(--muted-foreground))" fontSize={12} />
                      {appliedGroupBy === "hours" ? (
                        <Tooltip content={<HoursTooltip />} />
                      ) : (
                        <Tooltip contentStyle={{ backgroundColor: "hsl(var(--card))", border: "1px solid hsl(var(--border))", borderRadius: "8px", color: "hsl(var(--foreground))" }} />
                      )}
                      <Area type="monotone" dataKey={chartMetric} stroke="hsl(var(--primary))" fill="url(#grad-metric)" strokeWidth={2} isAnimationActive={false} />
                    </AreaChart>
                  </ResponsiveContainer>
                </div>
              </CardContent>
            </Card>
          )}

          <Card className="bg-card border-border">
            <CardHeader>
              <div className="flex flex-wrap items-center gap-2">
                {(Object.keys(groupLabels) as GroupBy[]).map((g) => (
                  <Button key={g} variant={groupBy === g ? "default" : "outline"} size="sm"
                    onClick={() => setGroupBy(g)}
                    className={cn("min-w-[100px]", groupBy === g ? "bg-primary text-primary-foreground" : "border-border")}>
                    {groupLabels[g]}
                  </Button>
                ))}
              </div>
            </CardHeader>
            <CardContent className="p-0">
              {data.length === 0 ? (
                <div className="py-16 text-center text-muted-foreground"><p>{t("stats.noData")}</p></div>
              ) : (
                <div className="overflow-x-auto overflow-y-hidden">
                  <table className="w-full table-fixed">
                    <thead>
                      <tr className="border-b border-border">
                        <th className={cn("text-left py-3 px-4 text-sm font-medium text-muted-foreground w-[200px]", canSortByLabel && "cursor-pointer select-none")}
                          onClick={() => canSortByLabel && toggleSort("label")}>
                          {labelHeader} {canSortByLabel && <SortIcon col="label" />}
                        </th>
                        <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none w-[140px]" onClick={() => toggleSort("impressions")}>
                          {t("stats.impressions")} <SortIcon col="impressions" />
                        </th>
                        <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none w-[120px]" onClick={() => toggleSort("clicks")}>
                          {t("stats.clicks")} <SortIcon col="clicks" />
                        </th>
                        <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground w-[100px]">{t("stats.ctr")}</th>
                        <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none w-[140px]" onClick={() => toggleSort("spent")}>
                          {t("stats.spent")} <SortIcon col="spent" />
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {sortedData.map((row) => (
                        <tr key={row.label} className="border-b border-border/50 hover:bg-muted/50 transition-colors">
                          <td className="py-3 px-4 font-medium truncate">
                            {appliedGroupBy === "country" ? formatCountryLabel(row.label, lang) : row.label}
                          </td>
                          <td className="py-3 px-4">{row.impressions.toLocaleString()}</td>
                          <td className="py-3 px-4">{row.clicks.toLocaleString()}</td>
                          <td className="py-3 px-4">{row.impressions > 0 ? ((row.clicks / row.impressions) * 100).toFixed(2) : "0.00"}%</td>
                          <td className="py-3 px-4">${row.spent.toLocaleString()}</td>
                        </tr>
                      ))}
                      <tr className="bg-muted/30 font-semibold">
                        <td className="py-3 px-4">{t("stats.total")}</td>
                        <td className="py-3 px-4">{totals.impressions.toLocaleString()}</td>
                        <td className="py-3 px-4">{totals.clicks.toLocaleString()}</td>
                        <td className="py-3 px-4">{totals.impressions > 0 ? ((totals.clicks / totals.impressions) * 100).toFixed(2) : "0.00"}%</td>
                        <td className="py-3 px-4">${totals.spent.toLocaleString()}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
