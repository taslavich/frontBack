import { useState, useRef, useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Plus, X } from "lucide-react";
import type { TargetingState, ListMode } from "@/contexts/CampaignContext";
import { useLanguage } from "@/contexts/LanguageContext";
import { cn } from "@/lib/utils";
import { toast } from "sonner";

const countryNames: Record<string, string> = {
  US:"United States",GB:"United Kingdom",DE:"Germany",FR:"France",IT:"Italy",ES:"Spain",BR:"Brazil",RU:"Russia",IN:"India",JP:"Japan",KR:"South Korea",CN:"China",AU:"Australia",CA:"Canada",MX:"Mexico",AR:"Argentina",CO:"Colombia",PL:"Poland",NL:"Netherlands",SE:"Sweden",NO:"Norway",DK:"Denmark",FI:"Finland",CZ:"Czech Republic",AT:"Austria",CH:"Switzerland",BE:"Belgium",PT:"Portugal",GR:"Greece",TR:"Turkey",UA:"Ukraine",RO:"Romania",HU:"Hungary",BG:"Bulgaria",HR:"Croatia",SK:"Slovakia",SI:"Slovenia",LT:"Lithuania",LV:"Latvia",EE:"Estonia",IE:"Ireland",IL:"Israel",SA:"Saudi Arabia",AE:"UAE",EG:"Egypt",ZA:"South Africa",NG:"Nigeria",KE:"Kenya",TH:"Thailand",VN:"Vietnam",PH:"Philippines",ID:"Indonesia",MY:"Malaysia",SG:"Singapore",TW:"Taiwan",HK:"Hong Kong",NZ:"New Zealand",CL:"Chile",PE:"Peru"
};

const languageNames: Record<string, string> = {
  EN:"English",ES:"Spanish",FR:"French",DE:"German",IT:"Italian",PT:"Portuguese",RU:"Russian",ZH:"Chinese",JA:"Japanese",KO:"Korean",AR:"Arabic",HI:"Hindi",TR:"Turkish",PL:"Polish",NL:"Dutch",SV:"Swedish",NO:"Norwegian",DA:"Danish",FI:"Finnish",CS:"Czech",RO:"Romanian",HU:"Hungarian",BG:"Bulgarian",HR:"Croatian",SK:"Slovak",SL:"Slovenian",LT:"Lithuanian",LV:"Latvian",ET:"Estonian",EL:"Greek",HE:"Hebrew",TH:"Thai",VI:"Vietnamese",ID:"Indonesian",MS:"Malay",UK:"Ukrainian",SR:"Serbian",BS:"Bosnian",MK:"Macedonian",SQ:"Albanian",KA:"Georgian",HY:"Armenian",AZ:"Azerbaijani",UZ:"Uzbek",KK:"Kazakh",TG:"Tajik",KY:"Kyrgyz",MN:"Mongolian",MY:"Burmese",KM:"Khmer",LO:"Lao",BN:"Bengali",TA:"Tamil",TE:"Telugu",ML:"Malayalam",KN:"Kannada",MR:"Marathi",GU:"Gujarati",PA:"Punjabi",SI:"Sinhala",NE:"Nepali"
};

const targetingOptions: Record<string, string[]> = {
  country: Object.keys(countryNames),
  language: Object.keys(languageNames),
  deviceType: ["Mobile","Desktop","Tablet","Smart TV","Console"],
  os: ["Android","iOS","Windows","macOS","Linux","ChromeOS","HarmonyOS"],
  browser: ["Chrome","Safari","Firefox","Edge","Opera","Samsung Internet","UC Browser","Brave","Vivaldi","Yandex Browser"],
  sites: [],
};

const DAYS = ["monday","tuesday","wednesday","thursday","friday","saturday","sunday"] as const;

const targetingConfigKeys = [
  "country", "language", "deviceType", "os", "browser", "schedule", "sites", "ip",
];

export const targetingConfigs = targetingConfigKeys.map(key => ({ key, labelKey: `targeting.${key}` }));

interface TargetingSectionProps {
  lists: Record<string, TargetingState>;
  onUpdate: (key: string, updates: Partial<TargetingState>) => void;
}

function AutocompleteInput({
  options, value, onChange, onAdd, existingItems, placeholder, t,
}: {
  options: string[];
  value: string;
  onChange: (v: string) => void;
  onAdd: (v: string) => void;
  existingItems: string[];
  placeholder: string;
  t: (key: string) => string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const keepOpenRef = useRef(false);

  const getDisplayLabel = (option: string) => {
    if (countryNames[option]) return `${countryNames[option]} (${option})`;
    if (languageNames[option]) return `${languageNames[option]} (${option})`;
    return option;
  };

  const filtered = options.filter(o => {
    const display = getDisplayLabel(o);
    return display.toLowerCase().includes(value.toLowerCase()) && !existingItems.includes(o);
  }).slice(0, 20);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (keepOpenRef.current) { keepOpenRef.current = false; return; }
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const handleAdd = (item: string) => {
    keepOpenRef.current = true;
    onAdd(item);
    onChange("");
    setOpen(true);
    setTimeout(() => { inputRef.current?.focus(); setOpen(true); }, 10);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (filtered.length > 0) handleAdd(filtered[0]);
    }
  };

  return (
    <div ref={ref} className="relative flex-1">
      <Input
        ref={inputRef}
        value={value}
        onChange={(e) => { onChange(e.target.value); setOpen(true); }}
        onFocus={() => setOpen(true)}
        placeholder={placeholder}
        className="bg-background border-border"
        onKeyDown={handleKeyDown}
      />
      {open && filtered.length > 0 && (
        <div className="absolute top-full left-0 right-0 z-50 mt-1 max-h-48 overflow-y-auto bg-card border border-border rounded-md shadow-lg">
          {filtered.map(option => (
            <button key={option} type="button"
              className="w-full text-left px-3 py-2 text-sm hover:bg-muted/50 transition-colors"
              onMouseDown={(e) => { e.preventDefault(); e.stopPropagation(); handleAdd(option); }}>
              {getDisplayLabel(option)}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// Schedule: items are stored as "monday:0", "monday:5", "tuesday:23", etc.
// Table-based UI: days on vertical axis, hours on horizontal axis
const DAY_SHORT: Record<string, Record<string, string>> = {
  ru: { monday: "Пн", tuesday: "Вт", wednesday: "Ср", thursday: "Чт", friday: "Пт", saturday: "Сб", sunday: "Вс" },
  en: { monday: "Mo", tuesday: "Tu", wednesday: "We", thursday: "Th", friday: "Fr", saturday: "Sa", sunday: "Su" },
};

function SchedulePicker({ items, onUpdate, t }: { items: string[]; onUpdate: (items: string[]) => void; t: (key: string) => string }) {
  const { lang } = useLanguage();
  const [isMouseDown, setIsMouseDown] = useState(false);
  const [selectMode, setSelectMode] = useState(true);
  const itemSet = useMemo(() => new Set(items), [items]);

  useEffect(() => {
    const up = () => setIsMouseDown(false);
    window.addEventListener("mouseup", up);
    return () => window.removeEventListener("mouseup", up);
  }, []);

  const toggleCell = (key: string, forceMode?: boolean) => {
    const has = itemSet.has(key);
    const shouldSelect = forceMode !== undefined ? forceMode : !has;
    if (shouldSelect && !has) onUpdate([...items, key]);
    else if (!shouldSelect && has) onUpdate(items.filter(i => i !== key));
  };

  const handleMouseDown = (key: string) => {
    setIsMouseDown(true);
    const has = itemSet.has(key);
    setSelectMode(!has);
    toggleCell(key);
  };

  const handleMouseEnter = (key: string) => {
    if (!isMouseDown) return;
    toggleCell(key, selectMode);
  };

  const toggleDay = (day: string) => {
    const allHours = Array.from({ length: 24 }, (_, i) => `${day}:${i}`);
    const allSelected = allHours.every(k => itemSet.has(k));
    if (allSelected) {
      onUpdate(items.filter(i => !i.startsWith(`${day}:`)));
    } else {
      const without = items.filter(i => !i.startsWith(`${day}:`));
      onUpdate([...without, ...allHours]);
    }
  };

  const toggleHour = (hour: number) => {
    const allKeys = DAYS.map(d => `${d}:${hour}`);
    const allSelected = allKeys.every(k => itemSet.has(k));
    if (allSelected) {
      onUpdate(items.filter(i => !allKeys.includes(i)));
    } else {
      const without = items.filter(i => !allKeys.includes(i));
      onUpdate([...without, ...allKeys]);
    }
  };

  const toggleAll = () => {
    const total = DAYS.length * 24;
    if (itemSet.size >= total) {
      onUpdate([]);
    } else {
      const all: string[] = [];
      for (const d of DAYS) for (let h = 0; h < 24; h++) all.push(`${d}:${h}`);
      onUpdate(all);
    }
  };

  const shorts = DAY_SHORT[lang] || DAY_SHORT.en;

  return (
    <div className="space-y-1.5">
      <p className="text-xs text-muted-foreground">{t("targeting.scheduleHint")}</p>
      <div className="overflow-x-auto">
        <table className="border-collapse select-none" style={{ tableLayout: "fixed" }}>
          <thead>
            <tr>
              <th className="p-0">
                <button type="button" onClick={toggleAll}
                  className="w-8 h-5 text-[9px] font-medium text-muted-foreground hover:bg-muted rounded transition-colors">
                  {t("targeting.selectAll")}
                </button>
              </th>
              {Array.from({ length: 24 }, (_, h) => (
                <th key={h} className="p-0">
                  <button type="button" onClick={() => toggleHour(h)}
                    className="w-5 h-5 text-[9px] font-medium text-muted-foreground hover:bg-muted rounded transition-colors">
                    {h}
                  </button>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {DAYS.map(day => (
              <tr key={day}>
                <td className="p-0">
                  <button type="button" onClick={() => toggleDay(day)}
                    className="w-8 h-5 text-[9px] font-medium text-muted-foreground hover:bg-muted rounded transition-colors text-left pl-0.5">
                    {shorts[day]}
                  </button>
                </td>
                {Array.from({ length: 24 }, (_, h) => {
                  const key = `${day}:${h}`;
                  const active = itemSet.has(key);
                  return (
                    <td key={h} className="p-px">
                      <button type="button"
                        onMouseDown={(e) => { e.preventDefault(); handleMouseDown(key); }}
                        onMouseEnter={() => handleMouseEnter(key)}
                        className={cn(
                          "w-5 h-5 rounded-sm transition-colors cursor-pointer",
                          active ? "bg-green-500" : "bg-muted/40 hover:bg-muted"
                        )} />
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// Sites input with validation
function SitesInput({ items, onAdd, t }: { items: string[]; onAdd: (items: string[]) => void; t: (key: string) => string }) {
  const [value, setValue] = useState("");

  const handleAdd = () => {
    const raw = value.trim();
    if (!raw) return;
    if (/\s/.test(raw) || raw.includes('"') || raw.includes("'")) {
      toast.error(t("targeting.sitesFormatError"));
      return;
    }
    const sites = raw.split(",").filter(s => s.trim()).map(s => s.trim());
    const valid = sites.filter(s => !items.includes(s));
    if (valid.length > 0) onAdd(valid);
    setValue("");
  };

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">{t("targeting.sitesHint")}</p>
      <div className="flex gap-2">
        <Input value={value} onChange={e => setValue(e.target.value)}
          placeholder="'12345','abdjhx'" className="bg-background border-border flex-1"
          onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleAdd(); } }} />
        <Button type="button" size="icon" variant="outline" onClick={handleAdd} className="border-border shrink-0">
          <Plus className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

// IP input with IPv4 validation only
const IPV4_RE = /^(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)$/;

function isValidIp(v: string): boolean {
  return IPV4_RE.test(v);
}

function IpInput({ items, onAdd, t }: { items: string[]; onAdd: (newItems: string[]) => void; t: (key: string) => string }) {
  const [value, setValue] = useState("");

  const handleAdd = () => {
    const raw = value.trim();
    if (!raw) return;
    const ips = raw.split(",").map(s => s.trim()).filter(Boolean);
    const invalid = ips.filter(ip => !isValidIp(ip));
    if (invalid.length > 0) {
      toast.error(t("targeting.ipFormatError"));
      return;
    }
    const valid = ips.filter(ip => !items.includes(ip));
    if (valid.length > 0) onAdd(valid);
    setValue("");
  };

  return (
    <div className="space-y-2">
      <p className="text-xs text-muted-foreground">{t("targeting.ipHint")}</p>
      <div className="flex gap-2">
        <Input value={value} onChange={e => setValue(e.target.value)}
          placeholder="192.168.1.1, 10.0.0.1" className="bg-background border-border flex-1"
          onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleAdd(); } }} />
        <Button type="button" size="icon" variant="outline" onClick={handleAdd} className="border-border shrink-0">
          <Plus className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}

function ListItem({ config, list: rawList, onUpdate }: {
  config: typeof targetingConfigs[0];
  list: TargetingState;
  onUpdate: (updates: Partial<TargetingState>) => void;
}) {
  const list = { mode: rawList?.mode ?? "none", items: rawList?.items ?? [] };
  const { t } = useLanguage();
  const [inputValue, setInputValue] = useState("");
  const options = targetingOptions[config.key] || [];
  const isSchedule = config.key === "schedule";
  const isSites = config.key === "sites";
  const isIp = config.key === "ip";

  const getDisplayLabel = (item: string) => {
    if (countryNames[item]) return `${countryNames[item]} (${item})`;
    if (languageNames[item]) return `${languageNames[item]} (${item})`;
    return item;
  };

  const addItem = (item: string) => {
    if (item && !list.items.includes(item)) {
      onUpdate({ items: [...list.items, item] });
    }
  };

  const removeItem = (item: string) => {
    onUpdate({ items: list.items.filter(i => i !== item) });
  };

  // For schedule, only show white/black (not "none" label, use "Off")
  const modeButtons = isSchedule
    ? (["none", "white"] as const)
    : (["none", "white", "black"] as const);

  return (
    <div className="space-y-3 p-4 rounded-lg bg-background/50 border border-border/50">
      <div className="flex items-center justify-between">
        <Label className="font-medium">{t(config.labelKey)}</Label>
        <div className="flex gap-1.5">
          {modeButtons.map((m) => (
            <Button key={m} type="button" size="sm" variant="outline"
              onClick={() => onUpdate({ mode: m })}
              className={
                list.mode === m
                  ? m === "white" ? "bg-green-600 text-white border-green-600 hover:bg-green-700 hover:text-white"
                    : m === "black" ? "bg-red-600 text-white border-red-600 hover:bg-red-700 hover:text-white"
                    : "bg-primary text-primary-foreground border-primary"
                  : "border-border"
              }>
              {m === "none" ? t("targeting.off") : isSchedule ? t("targeting.on") : m === "white" ? "White" : "Black"}
            </Button>
          ))}
        </div>
      </div>
      {list.mode !== "none" && (
        <div className="space-y-2">
          {isSchedule ? (
            <SchedulePicker items={list.items} onUpdate={(items) => onUpdate({ items })} t={t} />
          ) : isSites ? (
            <SitesInput items={list.items} onAdd={(newItems) => onUpdate({ items: [...list.items, ...newItems] })} t={t} />
          ) : isIp ? (
            <IpInput items={list.items} onAdd={(newItems) => onUpdate({ items: [...list.items, ...newItems] })} t={t} />
          ) : (
            <AutocompleteInput
              options={options} value={inputValue} onChange={setInputValue}
              onAdd={addItem} existingItems={list.items}
              placeholder={t("targeting.autocompletePlaceholder")} t={t}
            />
          )}
          {list.items.length > 0 && !isSchedule && (
            <div className="flex flex-wrap gap-1.5">
              {list.items.map((item) => (
                <Badge key={item} variant="outline"
                  className={`gap-1 ${list.mode === "white" ? "border-green-500/30 text-green-400" : "border-red-500/30 text-red-400"}`}>
                  {getDisplayLabel(item)}<X className="h-3 w-3 cursor-pointer" onClick={() => removeItem(item)} />
                </Badge>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function TargetingSection({ lists, onUpdate }: TargetingSectionProps) {
  const { t } = useLanguage();

  // Migrate old dayOfWeek/hour data to schedule format
  const effectiveLists = { ...lists };
  if ((lists.dayOfWeek?.mode !== "none" && lists.dayOfWeek?.items?.length) || (lists.hour?.mode !== "none" && lists.hour?.items?.length)) {
    if (!lists.schedule || lists.schedule.mode === "none") {
      const days = lists.dayOfWeek?.items?.length ? lists.dayOfWeek.items.map(d => d.replace("day.", "")) : DAYS.slice();
      const hours = lists.hour?.items?.length ? lists.hour.items : Array.from({ length: 24 }, (_, i) => String(i));
      const scheduleItems: string[] = [];
      for (const day of days) {
        for (const h of hours) {
          scheduleItems.push(`${day}:${h}`);
        }
      }
      effectiveLists.schedule = { mode: "white", items: scheduleItems };
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-sm text-muted-foreground">{t("targeting.description")}</p>
      {targetingConfigs.map((config) => (
        <ListItem
          key={config.key}
          config={config}
          list={effectiveLists[config.key] || { mode: "none", items: [] }}
          onUpdate={(updates) => onUpdate(config.key, updates)}
        />
      ))}
    </div>
  );
}
