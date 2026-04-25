import { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ArrowLeft } from "lucide-react";
import { toast } from "sonner";
import { useCampaigns, type TargetingState, type PricingModel, type TrafficQuality, type TrafficType, type ListMode, type Creative, type Vertical, VERTICALS } from "@/contexts/CampaignContext";
import { useNotifications } from "@/contexts/NotificationContext";
import { TargetingSection, targetingConfigs } from "@/components/dashboard/TargetingSection";
import { BudgetSection } from "@/components/dashboard/BudgetSection";
import { CreativesEditor } from "@/components/dashboard/CreativesEditor";
import { useLanguage } from "@/contexts/LanguageContext";

const formatLabels: Record<string, string> = {
  banner: "Banner", popunder: "Popunder", native: "Native", push: "In-page Push",
};

const bannerSizes = ["300x100", "300x250", "300x600", "728x90"];

const allScheduleItems = (): string[] => {
  const days = ["monday","tuesday","wednesday","thursday","friday","saturday","sunday"];
  const items: string[] = [];
  for (const d of days) for (let h = 0; h < 24; h++) items.push(`${d}:${h}`);
  return items;
};

const defaultTargeting = (): Record<string, TargetingState> =>
  Object.fromEntries(targetingConfigs.map(c =>
    c.key === "schedule"
      ? [c.key, { mode: "white" as ListMode, items: allScheduleItems() }]
      : [c.key, { mode: "none" as ListMode, items: [] }]
  ));

const generateId = () => String(Date.now()) + Math.random().toString(36).slice(2, 6);

export default function CreateCampaign() {
  const navigate = useNavigate();
  const { addCampaign } = useCampaigns();
  const { t } = useLanguage();
  const { addNotification } = useNotifications();
  const [step, setStep] = useState(1);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [trafficType, setTrafficType] = useState<TrafficType>("mainstream");
  const [name, setName] = useState("");
  const [adFormat, setAdFormat] = useState("");
  const [bannerSize, setBannerSize] = useState("");
  const [brandName, setBrandName] = useState("");
  const [verticals, setVerticals] = useState<Vertical[]>([]);
  const [creatives, setCreatives] = useState<Creative[]>([{ id: generateId(), url: "" }]);
  const [lists, setLists] = useState<Record<string, TargetingState>>(defaultTargeting());
  const [totalBudget, setTotalBudget] = useState("");
  const [priceValue, setPriceValue] = useState("");
  const [pricingModel, setPricingModel] = useState<PricingModel>("cpm");
  const [trafficQuality, setTrafficQuality] = useState<TrafficQuality>("common");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [evenSpend, setEvenSpend] = useState(false);
  const savedAsDraft = useRef(false);

  const clearError = (...keys: string[]) => setErrors(prev => {
    const next = { ...prev };
    keys.forEach(k => delete next[k]);
    return next;
  });

  // Reactively clear budget/date/price errors
  useEffect(() => { if (totalBudget && parseNum(totalBudget) >= 1) clearError("totalBudget"); }, [totalBudget]);
  useEffect(() => { if (startDate) clearError("startDate", "dates"); }, [startDate]);
  useEffect(() => { if (endDate) { const today = new Date(); today.setHours(0,0,0,0); if (new Date(endDate) >= today) clearError("endDate", "dates"); } }, [endDate]);
  useEffect(() => {
    if (priceValue) {
      const pv = parseNum(priceValue);
      const formatMins: Record<string, Record<TrafficQuality, number>> = {
        banner: { common: 0.01, high: 0.01, ultra: 0.01 },
        native: { common: 0.01, high: 0.01, ultra: 0.01 },
        push: { common: 0.005, high: 0.005, ultra: 0.005 },
        popunder: { common: 0.3, high: 0.7, ultra: 0.9 },
      };
      const mins = formatMins[adFormat] || formatMins.banner;
      const minCpm = mins[trafficQuality];
      const min = pricingModel === "cpc" ? +(minCpm * 1.7 / 1000).toFixed(5) : minCpm;
      if (pv >= min) clearError("priceValue");
    }
  }, [priceValue, pricingModel, trafficQuality, adFormat]);

  const updateList = (key: string, updates: Partial<TargetingState>) => {
    setLists(prev => ({ ...prev, [key]: { ...prev[key], ...updates } }));
  };

  const showBannerSize = adFormat === "banner";
  const showBrandName = adFormat === "native" || adFormat === "push";

  const validateStep1 = () => {
    const e: Record<string, string> = {};
    if (!name.trim()) e.name = t("create.required");
    if (!adFormat) e.adFormat = t("create.selectFormatError");
    if (adFormat === "banner" && !bannerSize) e.bannerSize = t("create.required");

    // Validate creatives
    creatives.forEach(c => {
      if (!c.name?.trim()) e[`creative_${c.id}_name`] = t("create.required");
      if (!c.url.trim()) e[`creative_${c.id}_url`] = t("create.required");
      if (adFormat !== "popunder" && !c.imageUrl) e[`creative_${c.id}_image`] = t("create.required");
      if ((adFormat === "native" || adFormat === "push") && !c.title?.trim()) e[`creative_${c.id}_title`] = t("create.required");
    });

    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const parseNum = (v: string) => parseFloat(v.replace(",", ".")) || 0;

  const validateStep3 = () => {
    const e: Record<string, string> = {};
    const tb = parseNum(totalBudget);
    if (!totalBudget || isNaN(tb) || tb < 1) e.totalBudget = t("edit.errorBudgetMin");
    const pv = parseNum(priceValue);
    const { min } = getMinPrice();
    if (!priceValue || isNaN(pv) || pv < min) e.priceValue = `${t("budget.belowMin")} ($${min})`;
    if (!startDate) e.startDate = t("create.required");
    if (!endDate) e.endDate = t("create.required");
    if (endDate) {
      const today = new Date(); today.setHours(0, 0, 0, 0);
      if (new Date(endDate) < today) e.endDate = t("create.endDateError");
    }
    if (!startDate || !endDate) e.dates = t("create.endDateRequired");
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const getMinPrice = () => {
    const formatMins: Record<string, Record<TrafficQuality, number>> = {
      banner: { common: 0.01, high: 0.01, ultra: 0.01 },
      native: { common: 0.01, high: 0.01, ultra: 0.01 },
      push: { common: 0.005, high: 0.005, ultra: 0.005 },
      popunder: { common: 0.3, high: 0.7, ultra: 0.9 },
    };
    const mins = formatMins[adFormat] || formatMins.banner;
    const minCpm = mins[trafficQuality];
    if (pricingModel === "cpc") return { min: +(minCpm * 1.7 / 1000).toFixed(5) };
    return { min: minCpm };
  };

  const handleNext = async () => {
    if (step === 1 && !validateStep1()) return;
    if (step === 3) { if (!validateStep3()) return; await handleCreate(); return; }
    setStep(step + 1);
    setErrors({});
  };

  const handleCreate = async () => {
    try {
      const id = await addCampaign({
        name: name.trim(), status: "moderation", format: formatLabels[adFormat] || adFormat,
        formatKey: adFormat, trafficType, verticals, budget: parseNum(totalBudget), dailyBudget: null,
        spent: 0, impressions: 0, clicks: 0, ctr: 0, pricingModel, priceValue: parseNum(priceValue),
        trafficQuality, startDate, endDate, creatives,
        targeting: Object.fromEntries(Object.entries(lists).map(([k, v]) => [k, { mode: v.mode, items: v.items }])),
        evenSpend, bannerSize: adFormat === "banner" ? bannerSize : undefined,
        brandName: showBrandName ? brandName : undefined,
      });
      if (!id) {
        // addCampaign resolved without an id (treated as silent failure)
        toast.error(t("create.failed") || "Failed to create campaign");
        return;
      }
      savedAsDraft.current = true;
      toast.success(t("create.created"));
      navigate("/dashboard/campaigns");
    } catch (e: any) {
      toast.error(`${t("create.failed") || "Failed to create campaign"}: ${e?.message || e}`);
    }
  };

  const saveDraft = async () => {
    if (savedAsDraft.current) return;
    if (!name.trim() && !adFormat) return;
    savedAsDraft.current = true;
    try {
      await addCampaign({
        name: name.trim() || "Draft", status: "draft",
        format: formatLabels[adFormat] || adFormat || "",
        formatKey: adFormat || "", trafficType, verticals, budget: totalBudget ? parseNum(totalBudget) : 0,
        dailyBudget: null,
        spent: 0, impressions: 0, clicks: 0, ctr: 0, pricingModel, priceValue: priceValue ? parseNum(priceValue) : 0,
        trafficQuality, startDate, endDate, creatives,
        targeting: Object.fromEntries(Object.entries(lists).map(([k, v]) => [k, { mode: v.mode, items: v.items }])),
        evenSpend, bannerSize: adFormat === "banner" ? bannerSize : undefined,
        brandName: showBrandName ? brandName : undefined,
      });
    } catch (e: any) {
      toast.error(`${t("create.failed") || "Failed to save draft"}: ${e?.message || e}`);
    }
  };

  const handleBack = async () => {
    const wasSaved = !savedAsDraft.current && (name.trim() || adFormat);
    await saveDraft();
    if (wasSaved) {
      void addNotification({ title: t("create.draftSaved"), description: t("create.draftSavedDesc"), type: "warning" });
    }
    navigate("/dashboard/campaigns");
  };

  useEffect(() => { return () => {}; }, []);

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={handleBack}><ArrowLeft className="h-5 w-5" /></Button>
        <div>
          <h2 className="text-2xl font-bold">{t("create.title")}</h2>
          <p className="text-muted-foreground text-sm">{t("create.step")} {step} {t("create.of")} 3</p>
        </div>
      </div>

      <div className="flex gap-2">
        {[1, 2, 3].map((s) => (
          <div key={s} className={`h-1.5 flex-1 rounded-full transition-colors ${s <= step ? "bg-primary" : "bg-muted"}`} />
        ))}
      </div>

      <Card className="bg-card border-border">
        <CardHeader>
          <CardTitle>
            {step === 1 && t("create.step1")}
            {step === 2 && t("create.step2")}
            {step === 3 && t("create.step3")}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          {step === 1 && (
            <>
              <div className="space-y-2">
                <Label>{t("create.trafficType")} *</Label>
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
                <p className="text-xs text-muted-foreground">{t("create.trafficTypeHint")}</p>
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
                <Label>{t("create.campaignName")}</Label>
                <Input value={name} onChange={(e) => { setName(e.target.value); if (e.target.value.trim()) clearError("name"); }}
                  placeholder={t("create.campaignNamePlaceholder")}
                  className={`bg-background border-border ${errors.name ? "border-destructive" : ""}`} />
                {errors.name && <p className="text-xs text-destructive">{errors.name}</p>}
              </div>
              <div className="space-y-2">
                <Label>{t("create.adFormat")}</Label>
                <Select value={adFormat} onValueChange={(v) => { setAdFormat(v); clearError("adFormat"); setCreatives([{ id: generateId(), url: "" }]); }}>
                  <SelectTrigger className={`bg-background border-border ${errors.adFormat ? "border-destructive" : ""}`}>
                    <SelectValue placeholder={t("create.selectFormat")} />
                  </SelectTrigger>
                  <SelectContent className="bg-card border-border">
                    {Object.entries(formatLabels).map(([val, label]) => (
                      <SelectItem key={val} value={val}>{label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {errors.adFormat && <p className="text-xs text-destructive">{errors.adFormat}</p>}
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

              {adFormat && (
                <>
                  <div className="pt-2">
                    <p className="text-sm font-medium text-muted-foreground mb-3">{t("create.creatives")}</p>
                    <CreativesEditor formatKey={adFormat} creatives={creatives} onChange={setCreatives} errors={errors} onClearError={clearError} />
                  </div>
                </>
              )}
            </>
          )}

          {step === 2 && <TargetingSection lists={lists} onUpdate={updateList} />}

          {step === 3 && (
            <BudgetSection
              formatKey={adFormat}
              totalBudget={totalBudget} setTotalBudget={setTotalBudget}
              priceValue={priceValue} setPriceValue={setPriceValue}
              pricingModel={pricingModel} setPricingModel={setPricingModel}
              trafficQuality={trafficQuality} setTrafficQuality={setTrafficQuality}
              startDate={startDate} setStartDate={setStartDate}
              endDate={endDate} setEndDate={setEndDate}
              evenSpend={evenSpend} setEvenSpend={setEvenSpend}
              errors={errors}
            />
          )}
        </CardContent>
      </Card>

      <div className="flex justify-between">
        {step > 1 ? (
          <Button variant="outline" onClick={() => { setStep(step - 1); setErrors({}); }} className="border-border">{t("create.back")}</Button>
        ) : <div />}
        <Button onClick={handleNext}
          className={step < 3 ? "bg-primary hover:bg-primary/90 text-primary-foreground" : "bg-accent hover:bg-accent/90 text-accent-foreground"}>
          {step < 3 ? t("create.next") : t("create.createBtn")}
        </Button>
      </div>
    </div>
  );
}
