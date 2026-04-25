import { UserPlus, Target, Wallet, Rocket } from "lucide-react";
import { useLanguage } from "@/contexts/LanguageContext";

const stepIcons = [UserPlus, Target, Wallet, Rocket];

export function StepsSection() {
  const { t } = useLanguage();

  const steps = stepIcons.map((icon, i) => ({
    icon,
    number: String(i + 1).padStart(2, "0"),
    title: t(`steps.${i + 1}.title`),
    description: t(`steps.${i + 1}.desc`),
  }));

  return (
    <section id="steps" className="py-20 relative overflow-hidden">
      <div className="absolute top-0 right-0 w-96 h-96 bg-primary/10 rounded-full blur-[150px]" />
      <div className="absolute bottom-0 left-1/4 w-72 h-72 bg-accent/10 rounded-full blur-[120px]" />
      <div className="container mx-auto px-4 relative z-10">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold mb-4">
            {t("steps.title1")}<span className="gradient-text">{t("steps.title2")}</span>
          </h2>
          <p className="text-muted-foreground text-lg max-w-2xl mx-auto">{t("steps.subtitle")}</p>
        </div>
        <div className="max-w-4xl mx-auto">
          <div className="relative">
            <div className="absolute left-8 md:left-1/2 top-0 bottom-0 w-px bg-gradient-to-b from-primary via-accent to-primary/30 hidden sm:block" />
            {steps.map((step, index) => (
              <div
                key={index}
                className={`relative flex flex-col sm:flex-row items-start gap-6 mb-12 last:mb-0 ${index % 2 === 0 ? 'sm:flex-row' : 'sm:flex-row-reverse'}`}
              >
                <div className={`flex-1 ${index % 2 === 0 ? 'sm:text-right' : 'sm:text-left'}`}>
                  <div className={`glass rounded-2xl p-6 hover-glow transition-all inline-block ${index % 2 === 0 ? 'sm:ml-auto' : 'sm:mr-auto'}`}>
                    <div className={`flex items-center gap-3 mb-3 ${index % 2 === 0 ? 'sm:flex-row-reverse' : ''}`}>
                      <div className="w-10 h-10 rounded-lg bg-primary/20 flex items-center justify-center">
                        <step.icon className="w-5 h-5 text-primary" />
                      </div>
                      <span className="text-3xl font-bold gradient-text">{step.number}</span>
                    </div>
                    <h3 className="text-xl font-semibold text-foreground mb-2">{step.title}</h3>
                    <p className="text-muted-foreground">{step.description}</p>
                  </div>
                </div>
                <div className="hidden sm:flex absolute left-8 sm:left-1/2 -translate-x-1/2 w-4 h-4 rounded-full gradient-primary glow-primary" />
                <div className="flex-1 hidden sm:block" />
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
