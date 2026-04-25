import { useState, useEffect, useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ArrowLeft, Save, AlertCircle } from "lucide-react";
import { toast } from "sonner";
import { useCampaigns, type TargetingState, type PricingModel, type TrafficQuality, type TrafficType, type Creative, type Vertical, VERTICALS } from "@/contexts/CampaignContext";
import { TargetingSection } from "@/components/dashboard/TargetingSection";
import { BudgetSection } from "@/components/dashboard/BudgetSection";
import { CreativesEditor } from "@/components/dashboard/CreativesEditor";
import { useLanguage } from "@/contexts/LanguageContext";

const bannerSizes = ["300x100", "300x250", "300x600", "728x90"];

export default function EditCampaign() {
  const navigate = useNavigate();
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const defaultTab = searchParams.get("tab") || "general";
  const { getCampaign, updateCampaign } = useCampaigns();
  const { t } = useLanguage();
  const campaign = getCampaign(id || "");

  const [name, setName] = useState("");
  const [bannerSize, setBannerSize] = useState("");
  const [brandName, setBrandName] = useState("");
  const [creatives, setCreatives] = useState<Creative[]>([]);
  const [initialCreatives, setInitialCreatives] = useState<Creative[]>([]);
  const [lists, setLists] = useState<Record<string, TargetingState>>({});
  const [totalBudget, setTotalBudget] = useState("");
  const [priceValue, setPriceValue] = useState("");
  const [pricingModel, setPricingModel] = useState<PricingModel>("cpm");
  const [trafficQuality, setTrafficQuality] = useState<TrafficQuality>("common");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [evenSpend, setEvenSpend] = useState(false);
  const [trafficType, setTrafficType] = useState<TrafficType>("mainstream");
  const [initialTrafficType, setInitialTrafficType] = useState<TrafficType>("mainstream");
  const [verticals, setVerticals] = useState<Vertical[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (campaign) {
      setName(campaign.name);
      setBannerSize(campaign.bannerSize || "");
      setBrandName(campaign.brandName || "");
      const crvs = campaign.creatives?.length ? campaign.creatives : [{ id: "migrated", url: "" }];
      setCreatives(crvs);
      setInitialCreatives(JSON.parse(JSON.stringify(crvs)));
      const targeting = campaign.targeting || {};
      // Default schedule to all days/hours if not set
      if (!targeting.schedule || targeting.schedule.mode === "none" || !targeting.schedule.items?.length) {
        const allItems: string[] = [];
        const days = ["monday","tuesday","wednesday","thursday","friday","saturday","sunday"];
        for (const d of days) for (let h = 0; h < 24; h++) allItems.push(`${d}:${h}`);
        targeting.schedule = { mode: "white", items: allItems };
      }
      setLists(targeting);
      setTotalBudget(String(campaign.budget));
      setPriceValue(String(campaign.priceValue));
      setPricingModel(campaign.pricingModel);
      setTrafficQuality(campaign.trafficQuality);
      setStartDate(campaign.startDate);
      setEndDate(campaign.endDate);
      setEvenSpend(campaign.evenSpend ?? false);
      setTrafficType(campaign.trafficType || "mainstream");
      setInitialTrafficType(campaign.trafficType || "mainstream");
      setVerticals(campaign.verticals || []);
      setInitialBannerSize(campaign.bannerSize || "");
    }
  }, [campaign]);

  const hasCreativeChanged = useMemo(() => {
    return JSON.stringify(creatives) !== JSON.stringify(initialCreatives);
  }, [creatives, initialCreatives]);

  const [initialBannerSize, setInitialBannerSize] = useState("");
  const isRestart = campaign?.status === "completed";
  const showBannerSize = campaign?.formatKey === "banner";
  const showBrandName = campaign?.formatKey === "native" || campaign?.formatKey === "push";
  const hasBannerSizeChanged = showBannerSize && bannerSize !== initialBannerSize;
  const hasTrafficTypeChanged = trafficType !== initialTrafficType;
  const needsModeration = hasCreativeChanged || hasTrafficTypeChanged || hasBannerSizeChanged;

  const clearError = (...keys: string[]) => setErrors(prev => {
    const next = { ...prev };
    keys.forEach(k => delete next[k]);
    return next;
  });

  // Reactively clear budget/date/price errors
  useEffect(() => { if (totalBudget && parseFloat(totalBudget.replace(",",".")) >= 1) clearError("totalBudget"); }, [totalBudget]);
  useEffect(() => { if (startDate) clearError("startDate"); }, [startDate]);
  useEffect(() => { if (endDate) { const today = new Date(); today.setHours(0,0,0,0); if (new Date(endDate) >= today) clearError("endDate"); } }, [endDate]);
  useEffect(() => { if (name.trim()) clearError("name"); }, [name]);
  useEffect(() => {
    if (priceValue && campaign) {
      const pv = parseFloat(priceValue.replace(",", ".")) || 0;
      const formatMins: Record<string, Record<TrafficQuality, number>> = {
        banner: { common: 0.01, high: 0.01, ultra: 0.01 },
        native: { common: 0.01, high: 0.01, ultra: 0.01 },
        push: { common: 0.005, high: 0.005, ultra: 0.005 },
        popunder: { common: 0.3, high: 0.7, ultra: 0.9 },
      };
      const mins = formatMins[campaign.formatKey] || formatMins.banner;
      const minCpm = mins[trafficQuality];
      const min = pricingModel === "cpc" ? +(minCpm * 1.7 / 1000).toFixed(5) : minCpm;
      if (pv >= min) clearError("priceValue");
    }
  }, [priceValue, pricingModel, trafficQuality, campaign]);

  const updateList = (key: string, updates: Partial<TargetingState>) => {
    setLists(prev => ({ ...prev, [key]: { ...prev[key], ...updates } }));
  };

  if (!campaign) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">{t("edit.notFound")}</p>
        <Button variant="outline" onClick={() => navigate("/dashboard/campaigns")} className="mt-4">{t("create.back")}</Button>
      </div>
    );
  }

  const parseNum = (v: string) => parseFloat(v.replace(",", ".")) || 0;

  const handleSave = async () => {
    const e: Record<string, string> = {};
    const tb = parseNum(totalBudget);
    if (!totalBudget || isNaN(tb) || tb < 1) e.totalBudget = t("edit.errorBudgetMin");

    const formatMins: Record<string, Record<TrafficQuality, number>> = {
      banner: { common: 0.01, high: 0.01, ultra: 0.01 },
      native: { common: 0.01, high: 0.01, ultra: 0.01 },
      push: { common: 0.005, high: 0.005, ultra: 0.005 },
      popunder: { common: 0.3, high: 0.7, ultra: 0.9 },
    };
    const mins = formatMins[campaign.formatKey] || formatMins.banner;
    const minCpm = mins[trafficQuality];
    const min = pricingModel === "cpc" ? +(minCpm * 1.7 / 1000).toFixed(5) : minCpm;
    const pv = parseNum(priceValue);
    if (!priceValue || isNaN(pv) || pv < min) e.priceValue = `${t("budget.belowMin")} ($${min})`;

    if (!startDate) e.startDate = t("create.required");
    if (!endDate) e.endDate = t("create.required");
    if (endDate) {
      const today = new Date(); today.setHours(0, 0, 0, 0);
      if (new Date(endDate) < today) e.endDate = t("create.endDateError");
    }
    if (!name.trim()) e.name = t("create.required");

    creatives.forEach(c => {
      if (!c.name?.trim()) e[`creative_${c.id}_name`] = t("create.required");
      if (!c.url.trim()) e[`creative_${c.id}_url`] = t("create.required");
      if (campaign.formatKey !== "popunder" && !c.imageUrl) e[`creative_${c.id}_image`] = t("create.required");
      if ((campaign.formatKey === "native" || campaign.formatKey === "push") && !c.title?.trim()) e[`creative_${c.id}_title`] = t("create.required");
    });

    if (campaign.formatKey === "banner" && !bannerSize) e.bannerSize = t("create.required");

    setErrors(e);
    if (Object.keys(e).length > 0) return;

    let newStatus = campaign.status;
    if (campaign.status === "draft") {
      newStatus = "moderation";
    } else if (isRestart) {
      newStatus = needsModeration ? "moderation" : "active";
    } else if (needsModeration) {
      newStatus = "moderation";
    }

    try {
      await updateCampaign(campaign.id, {
        name: name.trim(), creatives, trafficType, verticals,
        targeting: Object.fromEntries(Object.entries(lists).map(([k, v]) => [k, { mode: v.mode, items: v.items }])),
        budget: tb, dailyBudget: null,
        priceValue: pv, pricingModel, trafficQuality, startDate, endDate, evenSpend, status: newStatus,
        bannerSize: showBannerSize ? bannerSize : undefined,
        brandName: showBrandName ? brandName : undefined,
      });
    } catch (err: any) {
      toast.error(`${t("edit.saveFailed") || "Failed to save campaign"}: ${err?.message || err}`);
      return;
    }

    if (campaign.status === "draft") {
      toast.success(t("edit.savedModeration"));
    } else if (isRestart) {
      toast.success(needsModeration ? t("edit.savedModeration") : t("edit.restartedActive"));
    } else {
      toast.success(needsModeration ? t("edit.savedModeration") : t("edit.saved"));
    }
    navigate("/dashboard/campaigns");
  };

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate("/dashboard/campaigns")}><ArrowLeft className="h-5 w-5" /></Button>
        <div>
          <h2 className="text-2xl font-bold">{t("edit.title")}</h2>
          <p className="text-muted-foreground text-sm">ID: {id}</p>
        </div>
      </div>

      {needsModeration && (
        <div className="flex items-center gap-2 p-3 rounded-lg bg-yellow-500/10 border border-yellow-500/20">
          <AlertCircle className="h-4 w-4 text-yellow-500 shrink-0" />
          <p className="text-sm text-yellow-500">{t("edit.moderationWarning")}</p>
        </div>
      )}

      <Tabs defaultValue={defaultTab}>
        <TabsList className="bg-card border border-border">
          <TabsTrigger value="general">{t("edit.general")}</TabsTrigger>
          <TabsTrigger value="targeting">{t("edit.targeting")}</TabsTrigger>
          <TabsTrigger value="budget">{t("edit.budget")}</TabsTrigger>
        </TabsList>

        <TabsContent value="general">
          <Card className="bg-card border-border">
            <CardContent className="space-y-5 pt-6">
              <div className="space-y-2">
                <Label>{t("create.trafficType")}</Label>
                <Select value={trafficType} onValueChange={(v) => { setTrafficType(v as TrafficType); if (v === "mainstream") setVerticals(prev => prev.filter(x => x !== "Adult")); }}>
                  <SelectTrigger className="bg-background border-border">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent className="bg-card border-border">
                    <SelectItem value="mainstream">{t("create.mainstream")}</SelectItem>
                    <SelectItem value="adult">{t("create.adult")}</SelectItem>
                    <SelectItem value="mixed">{t("create.mixed")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>{t("create.vertical")}</Label>
                <div className="flex flex-wrap gap-2">
                  {VERTICALS.filter(v => trafficType === "mainstream" ? v !== "Adult" : true).map(v => {
                    const isChecked = verticals.includes(v);
                    return (
                      <button
                        key={v}
                        type="button"
                        onClick={() => setVerticals(prev => isChecked ? prev.filter(x => x !== v) : [...prev, v])}
                        className={`px-3 py-1 text-xs rounded-full border transition-colors ${
                          isChecked
                            ? "bg-primary/15 border-primary/40 text-primary"
                            : "bg-background border-border text-muted-foreground hover:border-primary/30"
                        }`}
                      >
                        {v}
                      </button>
                    );
                  })}
                </div>
              </div>
              <div className="space-y-2">
                <Label>{t("edit.name")} *</Label>
                <Input value={name} onChange={(e) => { setName(e.target.value); if (e.target.value.trim()) clearError("name"); }}
                  className={`bg-background border-border ${errors.name ? "border-destructive" : ""}`} />
                {errors.name && <p className="text-xs text-destructive">{errors.name}</p>}
              </div>
              <div className="space-y-2">
                <Label>{t("edit.formatLabel")}</Label>
                <Input value={campaign.format} disabled className="bg-muted border-border text-muted-foreground cursor-not-allowed" />
                <p className="text-xs text-muted-foreground">{t("edit.formatLocked")}</p>
              </div>

              {showBannerSize && (
                <div className="space-y-2">
                  <Label>{t("create.bannerSize")} *</Label>
                  <Select value={bannerSize} onValueChange={(v) => { setBannerSize(v); clearError("bannerSize"); }}>
                    <SelectTrigger className={`bg-background border-border ${errors.bannerSize ? "border-destructive" : ""}`}>
                      <SelectValue placeholder={t("create.selectBannerSize")} />
                    </SelectTrigger>
                    <SelectContent className="bg-card border-border">
                      {bannerSizes.map(s => <SelectItem key={s} value={s}>{s}</SelectItem>)}
                    </SelectContent>
                  </Select>
                  {errors.bannerSize && <p className="text-xs text-destructive">{errors.bannerSize}</p>}
                </div>
              )}

              {showBrandName && (
                <div className="space-y-2">
                  <Label>{t("create.brandName")}</Label>
                  <Input value={brandName} onChange={(e) => setBrandName(e.target.value)}
                    placeholder={t("create.brandNamePlaceholder")} className="bg-background border-border" />
                </div>
              )}

              <div className="pt-2">
                <p className="text-sm font-medium text-muted-foreground mb-3">{t("create.creatives")}</p>
                <CreativesEditor formatKey={campaign.formatKey} creatives={creatives} onChange={setCreatives} errors={errors} onClearError={clearError} />
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="targeting">
          <Card className="bg-card border-border">
            <CardHeader><CardTitle className="text-lg">{t("edit.targeting")}</CardTitle></CardHeader>
            <CardContent><TargetingSection lists={lists} onUpdate={updateList} /></CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="budget">
          <Card className="bg-card border-border">
            <CardContent className="pt-6">
              <BudgetSection
                formatKey={campaign.formatKey}
                totalBudget={totalBudget} setTotalBudget={setTotalBudget}
                priceValue={priceValue} setPriceValue={setPriceValue}
                pricingModel={pricingModel} setPricingModel={setPricingModel}
                trafficQuality={trafficQuality} setTrafficQuality={setTrafficQuality}
                startDate={startDate} setStartDate={setStartDate}
                endDate={endDate} setEndDate={setEndDate}
                evenSpend={evenSpend} setEvenSpend={setEvenSpend}
                errors={errors}
              />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <Button onClick={handleSave} className="bg-primary hover:bg-primary/90 text-primary-foreground">
        <Save className="h-4 w-4 mr-2" /> {t("edit.save")}
      </Button>
    </div>
  );
}
