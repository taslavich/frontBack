import { useState } from "react";
import { Mail, Send } from "lucide-react";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Copy } from "lucide-react";
import { toast } from "sonner";
import { useLanguage } from "@/contexts/LanguageContext";
import twinbidLogo from "@/assets/twinbid-logo.svg";

export function Footer() {
  const { t } = useLanguage();
  const [emailOpen, setEmailOpen] = useState(false);
  const email = "twinbid@twinbidex.com";

  const copyEmail = () => {
    navigator.clipboard.writeText(email);
    toast.success("Email скопирован");
  };

  return (
    <footer className="py-12 border-t border-border">
      <div className="container mx-auto px-4">
        <div className="flex flex-col md:flex-row items-center justify-between gap-8">
          <div className="flex flex-col items-center md:items-start gap-2">
            <a href="#" className="flex items-center gap-2">
              <img src={twinbidLogo} alt="TwinBid" className="h-8" />
            </a>
            <p className="text-sm text-muted-foreground">© {new Date().getFullYear()} TwinBid. All rights reserved.</p>
          </div>
          <div className="flex flex-wrap items-center justify-center gap-6 text-sm">
            <a href="#" className="text-muted-foreground hover:text-foreground transition-colors">{t("footer.privacy")}</a>
            <a href="#" className="text-muted-foreground hover:text-foreground transition-colors">{t("footer.terms")}</a>
            <a href="#" className="text-muted-foreground hover:text-foreground transition-colors">{t("footer.docs")}</a>
          </div>
          <div className="flex items-center gap-4">
            <button onClick={() => setEmailOpen(true)} className="w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-primary/20 transition-colors" title="Email">
              <Mail className="w-5 h-5" />
            </button>
            <a href="https://t.me/GregTwinbid" target="_blank" rel="noopener noreferrer" className="w-10 h-10 rounded-lg bg-secondary flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-primary/20 transition-colors" title="Telegram: @GregTwinbid">
              <Send className="w-5 h-5" />
            </a>
          </div>
        </div>
      </div>

      <Dialog open={emailOpen} onOpenChange={setEmailOpen}>
        <DialogContent className="sm:max-w-[400px] bg-card border-border">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2"><Mail className="h-5 w-5" /> Email</DialogTitle>
          </DialogHeader>
          <div className="flex items-center gap-2 mt-2">
            <span className="flex-1 font-mono text-sm bg-background border border-border rounded-lg px-3 py-2">{email}</span>
            <Button variant="outline" size="icon" onClick={copyEmail} className="border-border shrink-0">
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </footer>
  );
}
