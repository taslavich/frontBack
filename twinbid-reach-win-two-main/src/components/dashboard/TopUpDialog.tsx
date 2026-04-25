import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { CreditCard, Building2, Wallet } from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
import { useLanguage } from "@/contexts/LanguageContext";

interface TopUpDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const amounts = [5000, 10000, 25000, 50000];

export function TopUpDialog({ open, onOpenChange }: TopUpDialogProps) {
  const [selectedAmount, setSelectedAmount] = useState<number | null>(10000);
  const [customAmount, setCustomAmount] = useState("");
  const [selectedMethod, setSelectedMethod] = useState("card");
  const { t } = useLanguage();

  const paymentMethods = [
    { id: "card", label: t("legacy.bankCard"), icon: CreditCard },
    { id: "invoice", label: t("legacy.invoice"), icon: Building2 },
    { id: "wallet", label: t("legacy.eWallet"), icon: Wallet },
  ];

  const finalAmount = customAmount ? parseInt(customAmount) : selectedAmount;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px] bg-card border-border">
        <DialogHeader>
          <DialogTitle className="text-xl">{t("legacy.topUpTitle")}</DialogTitle>
          <DialogDescription>{t("legacy.topUpDesc")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-6 mt-4">
          <div className="space-y-3">
            <Label>{t("legacy.topUpAmount")}</Label>
            <div className="grid grid-cols-4 gap-2">
              {amounts.map((amount) => (
                <button key={amount} onClick={() => { setSelectedAmount(amount); setCustomAmount(""); }}
                  className={cn("py-2 px-3 rounded-lg border text-sm font-medium transition-colors",
                    selectedAmount === amount && !customAmount
                      ? "border-primary bg-primary/10 text-primary"
                      : "border-border bg-background hover:border-primary/50"
                  )}>
                  {amount.toLocaleString()} ₽
                </button>
              ))}
            </div>
            <div className="relative">
              <Input placeholder={t("legacy.otherAmount")} value={customAmount}
                onChange={(e) => { setCustomAmount(e.target.value); setSelectedAmount(null); }}
                className="bg-background border-border pr-8" />
              <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">₽</span>
            </div>
          </div>
          <div className="space-y-3">
            <Label>{t("legacy.paymentMethod")}</Label>
            <div className="space-y-2">
              {paymentMethods.map((method) => (
                <button key={method.id} onClick={() => setSelectedMethod(method.id)}
                  className={cn("w-full flex items-center gap-3 p-4 rounded-lg border transition-colors",
                    selectedMethod === method.id ? "border-primary bg-primary/10" : "border-border bg-background hover:border-primary/50"
                  )}>
                  <method.icon className={cn("h-5 w-5", selectedMethod === method.id ? "text-primary" : "text-muted-foreground")} />
                  <span className={cn("font-medium", selectedMethod === method.id ? "text-foreground" : "text-muted-foreground")}>{method.label}</span>
                </button>
              ))}
            </div>
          </div>
          <Button className="w-full bg-accent hover:bg-accent/90 text-accent-foreground" disabled={!finalAmount || finalAmount < 1000}>
            {t("legacy.pay")} {finalAmount ? `${finalAmount.toLocaleString()} ₽` : ""}
          </Button>
          <p className="text-xs text-center text-muted-foreground">{t("legacy.minAmount")}</p>
        </div>
      </DialogContent>
    </Dialog>
  );
}
