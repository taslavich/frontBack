import { useLanguage } from "@/contexts/LanguageContext";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function LanguageSelector({ variant = "default" }: { variant?: "default" | "ghost" }) {
  const { lang, setLang } = useLanguage();

  return (
    <div className="flex items-center gap-1">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setLang("ru")}
        className={cn(
          "px-2 py-1 h-7 text-xs font-medium",
          lang === "ru" ? "bg-primary/10 text-primary" : "text-muted-foreground hover:text-foreground"
        )}
      >
        RU
      </Button>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setLang("en")}
        className={cn(
          "px-2 py-1 h-7 text-xs font-medium",
          lang === "en" ? "bg-primary/10 text-primary" : "text-muted-foreground hover:text-foreground"
        )}
      >
        EN
      </Button>
    </div>
  );
}
