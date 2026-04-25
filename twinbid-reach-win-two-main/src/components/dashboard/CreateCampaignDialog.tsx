import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useState } from "react";
import { useLanguage } from "@/contexts/LanguageContext";

interface CreateCampaignDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateCampaignDialog({ open, onOpenChange }: CreateCampaignDialogProps) {
  const [step, setStep] = useState(1);
  const { t } = useLanguage();

  const handleCreate = () => {
    onOpenChange(false);
    setStep(1);
  };

  const stepLabels = [t("legacy.basicInfo"), t("legacy.targeting"), t("legacy.budgetSchedule")];

  return (
    <Dialog open={open} onOpenChange={(open) => { onOpenChange(open); if (!open) setStep(1); }}>
      <DialogContent className="sm:max-w-[600px] bg-card border-border">
        <DialogHeader>
          <DialogTitle className="text-xl">{t("legacy.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("legacy.step")} {step} {t("create.of")} 3: {stepLabels[step - 1]}
          </DialogDescription>
        </DialogHeader>

        <div className="flex gap-2 mb-6">
          {[1, 2, 3].map((s) => (
            <div key={s} className={`h-1 flex-1 rounded-full ${s <= step ? "bg-primary" : "bg-muted"}`} />
          ))}
        </div>

        {step === 1 && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">{t("legacy.campaignNameLabel")}</Label>
              <Input id="name" placeholder={t("legacy.campaignNamePlaceholder")} className="bg-background border-border" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="format">{t("legacy.adFormat")}</Label>
              <Select>
                <SelectTrigger className="bg-background border-border"><SelectValue placeholder={t("legacy.selectFormat")} /></SelectTrigger>
                <SelectContent className="bg-card border-border">
                  <SelectItem value="banner">Banner</SelectItem>
                  <SelectItem value="video">Video</SelectItem>
                  <SelectItem value="ctv">CTV/OTT</SelectItem>
                  <SelectItem value="audio">Audio</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="description">{t("legacy.descLabel")}</Label>
              <Textarea id="description" placeholder={t("legacy.descPlaceholder")} className="bg-background border-border resize-none" rows={3} />
            </div>
          </div>
        )}

        {step === 2 && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>{t("legacy.geography")}</Label>
              <Select>
                <SelectTrigger className="bg-background border-border"><SelectValue placeholder={t("legacy.selectRegion")} /></SelectTrigger>
                <SelectContent className="bg-card border-border">
                  <SelectItem value="russia">Russia</SelectItem>
                  <SelectItem value="moscow">Moscow</SelectItem>
                  <SelectItem value="spb">Saint Petersburg</SelectItem>
                  <SelectItem value="regions">Regions</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>{t("legacy.audienceAge")}</Label>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs text-muted-foreground">{t("legacy.from")}</Label>
                  <Select>
                    <SelectTrigger className="bg-background border-border"><SelectValue placeholder="18" /></SelectTrigger>
                    <SelectContent className="bg-card border-border">
                      {[18, 25, 35, 45, 55].map((age) => (
                        <SelectItem key={age} value={age.toString()}>{age}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div>
                  <Label className="text-xs text-muted-foreground">{t("legacy.to")}</Label>
                  <Select>
                    <SelectTrigger className="bg-background border-border"><SelectValue placeholder="65+" /></SelectTrigger>
                    <SelectContent className="bg-card border-border">
                      {[25, 35, 45, 55, 65].map((age) => (
                        <SelectItem key={age} value={age.toString()}>{age === 65 ? "65+" : String(age)}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
            </div>
            <div className="space-y-2">
              <Label>{t("legacy.interests")}</Label>
              <Select>
                <SelectTrigger className="bg-background border-border"><SelectValue placeholder={t("legacy.selectCategories")} /></SelectTrigger>
                <SelectContent className="bg-card border-border">
                  <SelectItem value="tech">Technology</SelectItem>
                  <SelectItem value="fashion">Fashion</SelectItem>
                  <SelectItem value="auto">Auto</SelectItem>
                  <SelectItem value="travel">Travel</SelectItem>
                  <SelectItem value="finance">Finance</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
        )}

        {step === 3 && (
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="budget">{t("legacy.dailyBudget")}</Label>
              <div className="relative">
                <Input id="budget" type="number" placeholder="10000" className="bg-background border-border pr-8" />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">₽</span>
              </div>
              <p className="text-xs text-muted-foreground">{t("legacy.dailyMin")}</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="total-budget">{t("legacy.totalBudgetOptional")}</Label>
              <div className="relative">
                <Input id="total-budget" type="number" placeholder={t("legacy.noLimit")} className="bg-background border-border pr-8" />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">₽</span>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label>{t("legacy.startDate")}</Label>
                <Input type="date" className="bg-background border-border" />
              </div>
              <div className="space-y-2">
                <Label>{t("legacy.endDate")}</Label>
                <Input type="date" className="bg-background border-border" />
              </div>
            </div>
          </div>
        )}

        <div className="flex justify-between mt-6">
          {step > 1 ? (
            <Button variant="outline" onClick={() => setStep(step - 1)}>{t("legacy.back")}</Button>
          ) : <div />}
          {step < 3 ? (
            <Button onClick={() => setStep(step + 1)} className="bg-primary hover:bg-primary/90">{t("legacy.next")}</Button>
          ) : (
            <Button onClick={handleCreate} className="bg-accent hover:bg-accent/90 text-accent-foreground">{t("legacy.createBtn")}</Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
