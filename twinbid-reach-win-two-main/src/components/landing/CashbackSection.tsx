import { DollarSign, TrendingUp, Zap } from "lucide-react";
import { useLanguage } from "@/contexts/LanguageContext";

const tiers = [
  { min: 1000, percent: 3, icon: DollarSign },
  { min: 5000, percent: 5, icon: TrendingUp },
  { min: 10000, percent: 10, icon: Zap },
];

export function CashbackSection() {
  const { t } = useLanguage();

  return (
    <section className="py-20 relative">
      <div className="absolute top-0 right-0 w-80 h-80 bg-accent/10 rounded-full blur-[140px]" />
      <div className="container mx-auto px-4 relative z-10">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold mb-4">
            {t("cashback.title")}
          </h2>
          <p className="text-muted-foreground text-lg max-w-2xl mx-auto">
            {t("cashback.subtitle")}
          </p>
        </div>

        <div className="grid md:grid-cols-3 gap-6 max-w-4xl mx-auto">
          {tiers.map((tier) => (
            <div
              key={tier.min}
              className="group glass rounded-2xl p-8 text-center hover-glow transition-all duration-300 flex flex-col items-center"
            >
              <div className="w-14 h-14 rounded-xl gradient-primary flex items-center justify-center mb-5 group-hover:glow-primary transition-shadow">
                <tier.icon className="w-7 h-7 text-primary-foreground" />
              </div>
              <div className="text-4xl font-bold gradient-text mb-2">
                {tier.percent}%
              </div>
              <p className="text-muted-foreground text-sm mb-1">
                {t("cashback.from")} ${tier.min.toLocaleString()}
              </p>
              <p className="text-xs text-muted-foreground">
                {t("cashback.perWeek")}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
