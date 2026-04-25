import { Gift, DollarSign } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AuthDialog } from "./AuthDialog";
import { useLanguage } from "@/contexts/LanguageContext";

export function StartConditions() {
  const { t } = useLanguage();

  return (
    <section className="py-20 relative">
      <div className="container mx-auto px-4">
        <div className="max-w-4xl mx-auto">
          <div className="relative rounded-2xl overflow-hidden">
            <div className="absolute inset-0 gradient-accent opacity-10" />
            <div className="absolute inset-0 bg-card/80" />
            <div className="relative p-8 md:p-12">
              <div className="grid md:grid-cols-2 gap-8 items-center">
                <div className="text-center md:text-left">
                  <div className="inline-flex items-center gap-3 mb-4">
                    <div className="w-12 h-12 rounded-xl gradient-primary flex items-center justify-center glow-primary">
                      <DollarSign className="w-6 h-6 text-primary-foreground" />
                    </div>
                    <div>
                      <div className="text-sm text-muted-foreground">{t("start.conditionsLabel")}</div>
                      <div className="text-2xl font-bold text-foreground">{t("start.minDeposit")}</div>
                    </div>
                  </div>
                  <div className="text-5xl md:text-6xl font-bold gradient-text mb-2">$100</div>
                  <p className="text-muted-foreground">{t("start.startSmall")}</p>
                </div>

                <div className="relative">
                  <div className="glass rounded-2xl p-6 border border-primary/30 hover-glow">
                    <div className="flex items-center gap-3 mb-4">
                      <div className="w-10 h-10 rounded-lg bg-primary/20 flex items-center justify-center">
                        <Gift className="w-5 h-5 text-primary" />
                      </div>
                      <span className="text-sm font-medium text-primary">{t("start.bonusBadge")}</span>
                    </div>
                    <div className="text-4xl md:text-5xl font-bold text-foreground mb-3">+25%</div>
                    <p className="text-muted-foreground mb-6">{t("start.bonusDesc")}</p>
                    <AuthDialog
                      defaultTab="register"
                      trigger={
                        <Button className="w-full gradient-primary text-primary-foreground hover:opacity-90 glow-primary">
                          {t("start.getBonus")}
                        </Button>
                      }
                    />
                  </div>
                  <div className="absolute -top-4 -right-4 w-24 h-24 bg-primary/30 rounded-full blur-[60px]" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
