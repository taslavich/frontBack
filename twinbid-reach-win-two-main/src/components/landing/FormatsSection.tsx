import { Layers, FileText, LayoutGrid, Bell } from "lucide-react";
import { useLanguage } from "@/contexts/LanguageContext";

const formatIcons = [Layers, FileText, LayoutGrid, Bell];
const formatNames = ["Popunder", "Native", "Banner", "In-page Push"];
const formatDescKeys = ["formats.popunder.desc", "formats.native.desc", "formats.banner.desc", "formats.push.desc"];

export function FormatsSection() {
  const { t } = useLanguage();

  return (
    <section id="formats" className="py-20 relative">
      <div className="container mx-auto px-4">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold mb-4">
            {t("formats.title")}<span className="gradient-text">{t("formats.title2")}</span>
          </h2>
          <p className="text-muted-foreground text-lg max-w-2xl mx-auto">{t("formats.subtitle")}</p>
        </div>
        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-6 max-w-5xl mx-auto">
          {formatIcons.map((Icon, index) => (
            <div key={index} className="group relative glass rounded-2xl p-6 text-center hover-glow transition-all duration-300">
              <div className="absolute inset-0 rounded-2xl gradient-primary opacity-0 group-hover:opacity-20 transition-opacity duration-300" />
              <div className="relative w-16 h-16 mx-auto rounded-xl bg-secondary flex items-center justify-center mb-5 group-hover:bg-primary/20 transition-colors">
                <Icon className="w-8 h-8 text-primary" />
              </div>
              <h3 className="text-xl font-semibold text-foreground mb-3 relative">{formatNames[index]}</h3>
              <p className="text-muted-foreground text-sm leading-relaxed relative">{t(formatDescKeys[index])}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
