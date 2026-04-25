import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { HelpCircle, AlertTriangle, Info, CalendarIcon } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { format } from "date-fns";
import type { PricingModel, TrafficQuality } from "@/contexts/CampaignContext";
import { useLanguage } from "@/contexts/LanguageContext";

const formatCpmLimits: Record<string, Record<TrafficQuality, { min: number; rec: number }>> = {
  banner: {
    common: { min: 0.01, rec: 0.05 },
    high: { min: 0.01, rec: 0.07 },
    ultra: { min: 0.01, rec: 0.14 },
  },
  native: {
    common: { min: 0.01, rec: 0.05 },
    high: { min: 0.01, rec: 0.07 },
    ultra: { min: 0.01, rec: 0.14 },
  },
  push: {
    common: { min: 0.005, rec: 0.01 },
    high: { min: 0.005, rec: 0.017 },
    ultra: { min: 0.005, rec: 0.035 },
  },
  popunder: {
    common: { min: 0.3, rec: 1.8 },
    high: { min: 0.7, rec: 3.0 },
    ultra: { min: 0.9, rec: 4.7 },
  },
};

const CPC_MULTIPLIER = 1.7 / 1000;

function getPriceLimits(formatKey: string, quality: TrafficQuality, model: PricingModel) {
  const limits = formatCpmLimits[formatKey] || formatCpmLimits.banner;
  const vals = limits[quality];
  if (model === "cpm") return vals;
  // Push values are already in CPC; only popunder needs conversion
  if (formatKey === "push") return vals;
  return { min: +(vals.min * CPC_MULTIPLIER).toFixed(5), rec: +(vals.rec * CPC_MULTIPLIER).toFixed(5) };
}

function getAvailableModels(formatKey: string): PricingModel[] {
  if (formatKey === "popunder") return ["cpm", "cpc"];
  if (formatKey === "push") return ["cpc"];
  return ["cpm"];
}

function parseNumericValue(val: string): number {
  return parseFloat(val.replace(",", ".")) || 0;
}

interface BudgetSectionProps {
  formatKey: string;
  totalBudget: string;
  setTotalBudget: (v: string) => void;
  priceValue: string;
  setPriceValue: (v: string) => void;
  pricingModel: PricingModel;
  setPricingModel: (v: PricingModel) => void;
  trafficQuality: TrafficQuality;
  setTrafficQuality: (v: TrafficQuality) => void;
  startDate: string;
  setStartDate: (v: string) => void;
  endDate: string;
  setEndDate: (v: string) => void;
  evenSpend: boolean;
  setEvenSpend: (v: boolean) => void;
  errors?: Record<string, string>;
}

export function BudgetSection({
  formatKey, totalBudget, setTotalBudget,
  priceValue, setPriceValue, pricingModel, setPricingModel,
  trafficQuality, setTrafficQuality, startDate, setStartDate, endDate, setEndDate,
  evenSpend, setEvenSpend,
  errors = {},
}: BudgetSectionProps) {
  const { t } = useLanguage();
  const availableModels = getAvailableModels(formatKey);
  const limits = getPriceLimits(formatKey, trafficQuality, pricingModel);
  const priceNum = parseNumericValue(priceValue);
  const isBelowMin = priceValue !== "" && priceNum < limits.min;
  const isBelowRec = priceValue !== "" && priceNum >= limits.min && priceNum < limits.rec;

  // End date validation
  const endDateInvalid = endDate ? (() => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    return new Date(endDate) < today;
  })() : false;

  const trafficInfo: Record<TrafficQuality, { label: string; desc: string }> = {
    common: { label: "Usual", desc: t("budget.trafficCommon") },
    high: { label: "High Quality", desc: t("budget.trafficHigh") },
    ultra: { label: "Ultra High Quality", desc: t("budget.trafficUltra") },
  };

  if (availableModels.length === 1 && pricingModel !== availableModels[0]) {
    setPricingModel(availableModels[0]);
  }

  const startDateObj = startDate ? new Date(startDate + "T00:00:00") : undefined;
  const endDateObj = endDate ? new Date(endDate + "T00:00:00") : undefined;
  const today = new Date();
  today.setHours(0, 0, 0, 0);

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <Label>{t("budget.totalBudget")}</Label>
        <div className="relative max-w-xs">
          <Input value={totalBudget} onChange={(e) => setTotalBudget(e.target.value)}
            placeholder="1000" className={cn("bg-background border-border pr-8", (errors.totalBudget) && "border-destructive")} />
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
        </div>
        {errors.totalBudget && <p className="text-xs text-destructive">{errors.totalBudget}</p>}
        <p className="text-xs text-muted-foreground">{t("budget.totalBudgetHint")}</p>
      </div>

      <div className="space-y-2">
        <Label>{t("budget.trafficType")}</Label>
        <div className="flex flex-wrap gap-2">
          {(["common", "high", "ultra"] as const).map((q) => (
            <div key={q} className="flex items-center gap-1">
              <Button type="button" variant="outline" size="sm"
                onClick={() => setTrafficQuality(q)}
                className={cn(
                  trafficQuality === q
                    ? "bg-primary text-primary-foreground border-primary"
                    : "border-border"
                )}>
                {trafficInfo[q].label}
              </Button>
              <Tooltip>
                <TooltipTrigger asChild>
                  <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help" />
                </TooltipTrigger>
                <TooltipContent side="top" className="max-w-xs">
                  <p className="text-sm">{trafficInfo[q].desc}</p>
                </TooltipContent>
              </Tooltip>
            </div>
          ))}
        </div>
      </div>

      {availableModels.length > 1 && (
        <div className="space-y-2">
          <Label>{t("budget.pricingModel")}</Label>
          <div className="flex gap-2">
            {availableModels.map((m) => (
              <Button key={m} type="button" variant="outline" size="sm"
                onClick={() => setPricingModel(m)}
                className={cn(
                  pricingModel === m
                    ? "bg-primary text-primary-foreground border-primary"
                    : "border-border"
                )}>
                {m.toUpperCase()}
              </Button>
            ))}
          </div>
        </div>
      )}

      <div className="space-y-2">
        <Label>{pricingModel === "cpm" ? t("budget.cpmLabel") : t("budget.cpcLabel")} *</Label>
        <div className="relative max-w-xs">
          <Input value={priceValue} onChange={(e) => setPriceValue(e.target.value)}
            placeholder={String(limits.rec)}
            className={cn("bg-background border-border pr-8", (isBelowMin || errors.priceValue) && "border-destructive")} />
          <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
        </div>
        <div className="space-y-1">
          <p className="text-xs text-muted-foreground">
            {t("budget.min")}: ${limits.min} · {t("budget.recommended")}: ${limits.rec}
          </p>
          {isBelowMin && (
            <div className="flex items-center gap-1 text-destructive">
              <AlertTriangle className="h-3 w-3" />
              <p className="text-xs">{t("budget.belowMin")} (${limits.min})</p>
            </div>
          )}
          {isBelowRec && (
            <div className="flex items-center gap-1 text-yellow-500">
              <Info className="h-3 w-3" />
              <p className="text-xs">{t("budget.belowRec")}</p>
            </div>
          )}
        </div>
        {errors.priceValue && <p className="text-xs text-destructive">{errors.priceValue}</p>}
      </div>

      <div className="grid grid-cols-2 gap-4 max-w-sm">
        <div className="space-y-2">
          <Label>{t("budget.startDate")} *</Label>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className={cn(
                "w-full justify-start text-left font-normal gap-2",
                !startDate && "text-muted-foreground",
                errors.startDate && "border-destructive"
              )}>
                <CalendarIcon className="h-4 w-4" />
                {startDateObj ? format(startDateObj, "dd.MM.yyyy") : t("budget.selectDate")}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
              <Calendar
                mode="single"
                selected={startDateObj}
                onSelect={(d) => d && setStartDate(format(d, "yyyy-MM-dd"))}
                className="p-3 pointer-events-auto"
              />
            </PopoverContent>
          </Popover>
          {errors.startDate && <p className="text-xs text-destructive">{errors.startDate}</p>}
        </div>
        <div className="space-y-2">
          <Label>{t("budget.endDate")} *</Label>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className={cn(
                "w-full justify-start text-left font-normal gap-2",
                !endDate && "text-muted-foreground",
                (endDateInvalid || errors.endDate) && "border-destructive"
              )}>
                <CalendarIcon className="h-4 w-4" />
                {endDateObj ? format(endDateObj, "dd.MM.yyyy") : t("budget.selectDate")}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-auto p-0" align="start">
              <Calendar
                mode="single"
                selected={endDateObj}
                onSelect={(d) => d && setEndDate(format(d, "yyyy-MM-dd"))}
                disabled={(date) => date < today}
                className="p-3 pointer-events-auto"
              />
            </PopoverContent>
          </Popover>
          {(endDateInvalid || errors.endDate) && <p className="text-xs text-destructive">{errors.endDate || t("budget.endDateError")}</p>}
        </div>
        {errors.dates && <p className="text-xs text-destructive col-span-2">{errors.dates}</p>}
      </div>

      <div className="flex items-center gap-3">
        <Switch checked={evenSpend} onCheckedChange={setEvenSpend} />
        <Label className="cursor-pointer" onClick={() => setEvenSpend(!evenSpend)}>{t("budget.evenSpend")}</Label>
        <Tooltip>
          <TooltipTrigger asChild>
            <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help" />
          </TooltipTrigger>
          <TooltipContent side="top" className="max-w-xs">
            <p className="text-sm">{t("budget.evenSpendTooltip")}</p>
          </TooltipContent>
        </Tooltip>
      </div>
    </div>
  );
}

export function parseNumeric(val: string): number {
  return parseFloat(val.replace(",", ".")) || 0;
}
