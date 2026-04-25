import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import { Upload, Plus, Trash2, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { useLanguage } from "@/contexts/LanguageContext";
import type { Creative } from "@/contexts/CampaignContext";
// Read a File as a base64 data URL — used as a local preview until the next
// reload, when the backend will return a presigned read URL.
function readFileAsDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(file);
  });
}

const URL_MACROS = [
  "click_id", "site_id", "country_code", "creative_id",
  "campaign_id", "browser", "device", "device_os", "ip_address",
] as const;

interface CreativesEditorProps {
  formatKey: string;
  creatives: Creative[];
  onChange: (creatives: Creative[]) => void;
  errors?: Record<string, string>;
  onClearError?: (...keys: string[]) => void;
}

const generateId = () => String(Date.now()) + Math.random().toString(36).slice(2, 6);

export function CreativesEditor({ formatKey, creatives, onChange, errors = {}, onClearError }: CreativesEditorProps) {
  const { t } = useLanguage();
  const fileInputRefs = useRef<Record<string, HTMLInputElement | null>>({});
  const [uploadingId, setUploadingId] = useState<string | null>(null);

  const showTitle = formatKey === "native" || formatKey === "push";
  const showDescription = formatKey === "native" || formatKey === "push";
  const showImage = formatKey !== "popunder";

  const updateCreative = (id: string, updates: Partial<Creative>) => {
    onChange(creatives.map(c => c.id === id ? { ...c, ...updates } : c));
  };

  const addCreative = () => {
    onChange([...creatives, { id: generateId(), url: "" }]);
  };

  const removeCreative = (id: string) => {
    if (creatives.length <= 1) return;
    onChange(creatives.filter(c => c.id !== id));
  };

  const ALLOWED_IMAGE_TYPES = ["image/png", "image/jpeg", "image/jpg"];
  const ALLOWED_EXTENSIONS = [".png", ".jpg", ".jpeg"];

  const handleImageUpload = async (creativeId: string, e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const ext = file.name.toLowerCase().slice(file.name.lastIndexOf("."));
    if (!ALLOWED_EXTENSIONS.includes(ext) && !ALLOWED_IMAGE_TYPES.includes(file.type)) {
      toast.error(t("create.imageFormatError"));
      e.target.value = "";
      return;
    }
    setUploadingId(creativeId);
    try {
      // Local preview only. The actual file is uploaded by CampaignContext to
      // the backend AFTER the creative row is created — backend then writes
      // s3_file_path itself. Frontend never touches s3_file_path.
      const previewUrl = await readFileAsDataUrl(file);
      updateCreative(creativeId, {
        imageUrl: previewUrl,
        pendingFile: file,
        imageFileName: file.name,
      });
      onClearError?.(`creative_${creativeId}_image`);
      toast.success(t("create.imageUploaded"));
    } catch (err) {
      console.error("Image upload error:", err);
      toast.error(t("create.imageFormatError"));
    } finally {
      setUploadingId(null);
      e.target.value = "";
    }
  };

  const toggleMacro = (creativeId: string, macro: string, currentUrl: string) => {
    const token = `${macro}={${macro}}`;
    // Check if macro already in URL
    if (currentUrl.includes(`{${macro}}`)) {
      // Remove the macro param from URL
      let newUrl = currentUrl;
      // Remove &macro={macro} or ?macro={macro}
      const regexAmp = new RegExp(`[&?]${macro}=\\{${macro}\\}`, "g");
      newUrl = newUrl.replace(regexAmp, "");
      // Fix leading & if ? was removed
      if (newUrl.includes("&") && !newUrl.includes("?")) {
        newUrl = newUrl.replace("&", "?");
      }
      updateCreative(creativeId, { url: newUrl });
    } else {
      // Add macro
      const separator = currentUrl.includes("?") ? "&" : "?";
      updateCreative(creativeId, { url: currentUrl + separator + token });
    }
  };

  return (
    <div className="space-y-4">
      {creatives.map((creative, idx) => {
        // Compute active macros for this creative
        const activeMacros = new Set(URL_MACROS.filter(m => creative.url.includes(`{${m}}`)));

        return (
          <div key={creative.id} className="p-4 rounded-lg border border-border bg-background/30 space-y-4">
            <div className="flex items-center justify-between">
              <p className="text-sm font-medium text-muted-foreground">
                {t("create.creative")} #{idx + 1}
              </p>
              {creatives.length > 1 && (
                <Button type="button" variant="ghost" size="icon" onClick={() => removeCreative(creative.id)}
                  className="h-7 w-7 text-destructive hover:text-destructive">
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </div>

            <div className="space-y-2">
              <Label>{t("create.creativeName")} *</Label>
              <Input value={creative.name || ""} onChange={e => { updateCreative(creative.id, { name: e.target.value }); if (e.target.value.trim()) onClearError?.(`creative_${creative.id}_name`); }}
                placeholder={t("create.creativeNamePlaceholder")}
                className={`bg-background border-border ${errors[`creative_${creative.id}_name`] ? "border-destructive" : ""}`} />
              <p className="text-xs text-muted-foreground">{t("create.creativeNameHint")}</p>
              {errors[`creative_${creative.id}_name`] && <p className="text-xs text-destructive">{errors[`creative_${creative.id}_name`]}</p>}
            </div>

            {showTitle && (
              <div className="space-y-2">
                <Label>{t("create.creativeTitle")} *</Label>
                <Input value={creative.title || ""} onChange={e => { updateCreative(creative.id, { title: e.target.value }); if (e.target.value.trim()) onClearError?.(`creative_${creative.id}_title`); }}
                  placeholder={t("create.titlePlaceholder")}
                  className={`bg-background border-border ${errors[`creative_${creative.id}_title`] ? "border-destructive" : ""}`} />
                {errors[`creative_${creative.id}_title`] && <p className="text-xs text-destructive">{errors[`creative_${creative.id}_title`]}</p>}
              </div>
            )}

            {showDescription && (
              <div className="space-y-2">
                <Label>{t("create.creativeDescription")}</Label>
                <Textarea value={creative.description || ""} onChange={e => updateCreative(creative.id, { description: e.target.value })}
                  placeholder={t("create.descriptionPlaceholder")} className="bg-background border-border resize-none" rows={2} />
              </div>
            )}

            <div className="space-y-2">
              <Label>{t("create.creativeUrl")} *</Label>
              <Input value={creative.url} onChange={e => { updateCreative(creative.id, { url: e.target.value }); if (e.target.value.trim()) onClearError?.(`creative_${creative.id}_url`); }}
                placeholder="https://example.com/landing"
                className={`bg-background border-border ${errors[`creative_${creative.id}_url`] ? "border-destructive" : ""}`} />
              {errors[`creative_${creative.id}_url`] && <p className="text-xs text-destructive">{errors[`creative_${creative.id}_url`]}</p>}
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">{t("create.urlMacrosHint")}</p>
                <div className="flex flex-wrap gap-1.5">
                  {URL_MACROS.map(macro => {
                    const isActive = activeMacros.has(macro);
                    return (
                      <Badge
                        key={macro}
                        variant="outline"
                        className={`cursor-pointer text-xs font-mono transition-colors ${
                          isActive
                            ? "bg-primary/15 border-primary/40 text-primary hover:bg-primary/25"
                            : "hover:bg-primary/10 hover:border-primary/30"
                        }`}
                        onClick={() => toggleMacro(creative.id, macro, creative.url)}
                      >
                        {`{${macro}}`}
                      </Badge>
                    );
                  })}
                </div>
              </div>
            </div>

            {showImage && (
              <div className="space-y-2">
                <Label>{t("create.uploadImage")} *</Label>
                <input
                  ref={el => { fileInputRefs.current[creative.id] = el; }}
                  type="file" accept=".png,.jpg,.jpeg" className="hidden"
                  onChange={e => handleImageUpload(creative.id, e)} />
                <p className="text-xs text-muted-foreground">{t("create.imageFormatHint")}</p>
                <div className="flex items-center gap-3">
                  <Button type="button" variant="outline" disabled={uploadingId === creative.id}
                    onClick={() => fileInputRefs.current[creative.id]?.click()} className="border-border gap-2">
                    {uploadingId === creative.id
                      ? <Loader2 className="h-4 w-4 animate-spin" />
                      : <Upload className="h-4 w-4" />}
                    {t("create.uploadImage")}
                  </Button>
                  {creative.imageFileName && <span className="text-sm text-muted-foreground">{creative.imageFileName}</span>}
                </div>
                {creative.imageUrl && (
                  <img src={creative.imageUrl} alt="Preview" className="mt-2 max-h-32 rounded border border-border" />
                )}
                {errors[`creative_${creative.id}_image`] && <p className="text-xs text-destructive">{errors[`creative_${creative.id}_image`]}</p>}
              </div>
            )}
          </div>
        );
      })}

      {formatKey !== "popunder" && (
        <Button type="button" variant="outline" onClick={addCreative} className="border-border gap-2 w-full">
          <Plus className="h-4 w-4" /> {t("create.addCreative")}
        </Button>
      )}
    </div>
  );
}
