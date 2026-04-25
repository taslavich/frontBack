import { ArrowRight, Zap } from "lucide-react";
import { Button } from "@/components/ui/button";
import { AuthDialog } from "./AuthDialog";
import { useLanguage } from "@/contexts/LanguageContext";

export function HeroSection() {
  const { t } = useLanguage();

  return (
    <section className="relative min-h-screen flex items-center justify-center pt-20 overflow-hidden">
      <div className="absolute inset-0 overflow-hidden">
        <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-primary/20 rounded-full blur-[120px]" />
        <div className="absolute bottom-1/4 right-1/4 w-80 h-80 bg-accent/20 rounded-full blur-[100px]" />
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-primary/10 rounded-full blur-[150px]" />
      </div>

      <div
        className="absolute inset-0 opacity-[0.03]"
        style={{
          backgroundImage: `linear-gradient(hsl(var(--foreground)) 1px, transparent 1px),
                           linear-gradient(90deg, hsl(var(--foreground)) 1px, transparent 1px)`,
          backgroundSize: '60px 60px',
          maskImage: 'linear-gradient(to bottom, black 40%, transparent 80%)',
          WebkitMaskImage: 'linear-gradient(to bottom, black 40%, transparent 80%)',
        }}
      />

      <div className="container mx-auto px-4 relative z-10">
        <div className="max-w-4xl mx-auto text-center">
          <div className="inline-flex items-center gap-2 px-4 py-2 rounded-full glass mb-8 animate-fade-in">
            <Zap className="w-4 h-4 text-primary" />
            <span className="text-sm text-muted-foreground">{t("hero.badge")}</span>
          </div>

          <h1 className="text-4xl md:text-6xl lg:text-7xl font-bold mb-6 leading-tight animate-fade-in">
            <span className="text-foreground">{t("hero.title1")}</span>
            <span className="gradient-text">{t("hero.title2")}</span>
          </h1>

          <p className="text-lg md:text-xl text-muted-foreground max-w-3xl mx-auto mb-8 leading-relaxed animate-fade-in">
            {t("hero.subtitle")}{" "}
            <span className="text-foreground font-semibold">{t("hero.subtitleSites")}</span>{" "}
            {t("hero.subtitleEnd")}
          </p>

          <div className="flex flex-col sm:flex-row items-center justify-center gap-4 mb-12 animate-fade-in">
            <AuthDialog
              defaultTab="register"
              trigger={
                <Button size="lg" className="gradient-primary text-primary-foreground hover:opacity-90 glow-primary text-lg px-8 py-6 h-auto">
                  {t("hero.cta")}
                  <ArrowRight className="ml-2 w-5 h-5" />
                </Button>
              }
            />
            <Button size="lg" variant="outline" className="text-lg px-8 py-6 h-auto border-border hover:bg-secondary">
              {t("hero.learnMore")}
            </Button>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-3 gap-6 md:gap-12 max-w-2xl mx-auto animate-fade-in">
            <div className="text-center">
              <div className="text-3xl md:text-4xl font-bold gradient-text mb-1">1M+</div>
              <div className="text-sm text-muted-foreground">{t("hero.statSites")}</div>
            </div>
            <div className="text-center">
              <div className="text-3xl md:text-4xl font-bold gradient-text mb-1">100+</div>
              <div className="text-sm text-muted-foreground">{t("hero.statNetworks")}</div>
            </div>
            <div className="text-center col-span-2 md:col-span-1">
              <div className="text-3xl md:text-4xl font-bold gradient-text mb-1">24/7</div>
              <div className="text-sm text-muted-foreground">{t("hero.statSupport")}</div>
            </div>
          </div>
        </div>
      </div>

      <div className="absolute bottom-8 left-1/2 -translate-x-1/2 animate-bounce items-center justify-center hidden scroll-indicator">
        <div className="w-6 h-10 rounded-full border-2 border-muted-foreground/30 flex items-start justify-center p-2">
          <div className="w-1 h-2 bg-muted-foreground rounded-full" />
        </div>
      </div>
    </section>
  );
}
