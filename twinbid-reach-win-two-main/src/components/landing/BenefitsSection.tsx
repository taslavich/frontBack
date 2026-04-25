import { LayoutDashboard, TrendingUp, Eye, ShieldCheck, Brain } from "lucide-react";
import { useLanguage } from "@/contexts/LanguageContext";

const benefitIcons = [LayoutDashboard, TrendingUp, Eye, ShieldCheck, Brain];

export function BenefitsSection() {
  const { t } = useLanguage();

  const benefits = benefitIcons.map((icon, i) => ({
    icon,
    title: t(`benefits.${i + 1}.title`),
    description: t(`benefits.${i + 1}.desc`),
  }));

  return (
    <section id="benefits" className="py-20 relative">
      <div className="absolute top-1/2 left-0 w-72 h-72 bg-primary/10 rounded-full blur-[120px]" />
      <div className="absolute bottom-0 right-0 w-96 h-96 bg-accent/10 rounded-full blur-[150px]" />
      <div className="container mx-auto px-4 relative z-10">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold mb-4">
            {t("benefits.title1")}<span className="gradient-text">TwinBid</span>{t("benefits.title2")}
          </h2>
          <p className="text-muted-foreground text-lg max-w-2xl mx-auto">{t("benefits.subtitle")}</p>
        </div>
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {benefits.slice(0, 3).map((benefit, index) => (
            <BenefitCard key={index} benefit={benefit} index={index} />
          ))}
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-6 lg:max-w-[calc(66.666%+0.75rem)] mx-auto">
          {benefits.slice(3).map((benefit, index) => (
            <BenefitCard key={index + 3} benefit={benefit} index={index + 3} />
          ))}
        </div>
      </div>
    </section>
  );
}

function BenefitCard({ benefit, index }: { benefit: { icon: any; title: string; description: string }; index: number }) {
  return (
    <div className="group glass rounded-2xl p-6 hover-glow transition-all duration-300">
      <div className="w-14 h-14 rounded-xl gradient-primary flex items-center justify-center mb-5 group-hover:glow-primary transition-shadow">
        <benefit.icon className="w-7 h-7 text-primary-foreground" />
      </div>
      <div className="inline-flex items-center justify-center w-8 h-8 rounded-full bg-secondary text-sm font-bold text-muted-foreground mb-4">
        {index + 1}
      </div>
      <h3 className="text-xl font-semibold text-foreground mb-3">{benefit.title}</h3>
      <p className="text-muted-foreground leading-relaxed">{benefit.description}</p>
    </div>
  );
}
