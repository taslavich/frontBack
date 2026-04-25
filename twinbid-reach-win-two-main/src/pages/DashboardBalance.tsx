import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Wallet, Plus, Receipt, Tag } from "lucide-react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { useLanguage } from "@/contexts/LanguageContext";
import { useProfile } from "@/contexts/ProfileContext";
import { useAuth } from "@/contexts/AuthContext";
import { usePendingPayment } from "@/contexts/PendingPaymentContext";
import { api } from "@/api";
import type { ApiUserTransaction } from "@/api/types";

const amounts = [100, 250, 500, 1000, 5000];

const usdtMethods = [
  { id: "usdt_trc20", label: "USDT (TRC-20)", desc: "Tether on Tron" },
  { id: "usdt_erc20", label: "USDT (ERC-20)", desc: "Tether on Ethereum" },
];

type TopupRequest = ApiUserTransaction;

export default function DashboardBalance() {
  const { t } = useLanguage();
  const { user } = useAuth();
  const { profile, loading: profileLoading } = useProfile();
  const {
    pendingPayment, setPendingPayment,
    openDialog, registerRefreshHandler,
  } = usePendingPayment();

  const [selectedAmount, setSelectedAmount] = useState<number | null>(250);
  const [customAmount, setCustomAmount] = useState("");
  const [selectedMethod, setSelectedMethod] = useState("usdt_trc20");
  const [promoCode, setPromoCode] = useState("");
  const [appliedPromo, setAppliedPromo] = useState<{ code: string; bonus: number } | null>(null);
  const [topupRequests, setTopupRequests] = useState<TopupRequest[]>([]);
  const [loadingRequests, setLoadingRequests] = useState(true);

  const balance = profile?.balance ?? 0;

  const fetchTopupRequests = async () => {
    if (!user) return;
    setLoadingRequests(true);
    try {
      const { items } = await api.listTransactions();
      setTopupRequests(items);
    } catch (e) {
      console.error("Topups fetch error:", e);
    } finally {
      setLoadingRequests(false);
    }
  };

  useEffect(() => { fetchTopupRequests(); }, [user]);

  // Allow the global dialog to refresh our history after submission
  useEffect(() => {
    registerRefreshHandler(fetchTopupRequests);
  }, [registerRefreshHandler, user]);

  const finalAmount = customAmount ? parseInt(customAmount) : selectedAmount;

  const handleApplyPromo = async () => {
    const code = promoCode.trim().toUpperCase();
    if (!code) return;
    try {
      const promo = await api.getPromocode(code);
      if (promo.valid_to && new Date(promo.valid_to) < new Date()) {
        toast.error(t("balance.promo.invalid"));
        return;
      }
      if (promo.usage_limit != null && promo.usage_count >= promo.usage_limit) {
        toast.error(t("balance.promo.invalid"));
        return;
      }
      // Reject if this user already has a transaction tied to this promocode.
      const alreadyUsed = topupRequests.some(
        (tx) => tx.promocode_id === promo.id && (!user || tx.user_id === user.id)
      );
      if (alreadyUsed) {
        toast.error(t("balance.promo.alreadyUsed"));
        return;
      }
      setAppliedPromo({ code, bonus: Number(promo.bonus_percent) });
      toast.success(t("balance.promo.applied").replace("{percent}", `${promo.bonus_percent}`));
    } catch {
      setAppliedPromo(null);
      toast.error(t("balance.promo.invalid"));
    }
  };

  const handleRemovePromo = () => {
    setAppliedPromo(null);
    setPromoCode("");
  };

  const handleTopUp = () => {
    if (!finalAmount || finalAmount < 100) return;
    if (pendingPayment) {
      toast.error(t("balance.disabledReason"));
      return;
    }
    setPendingPayment({
      amount: finalAmount,
      method: selectedMethod,
      promo: appliedPromo?.code,
      bonus: appliedPromo?.bonus,
    });
    setAppliedPromo(null);
    setPromoCode("");
    openDialog();
  };

  const formatDate = (dateStr: string) => {
    const d = new Date(dateStr);
    return `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")}.${d.getFullYear()}`;
  };

  const statusMap: Record<string, { label: string; className: string }> = {
    pending: { label: t("balance.pending"), className: "text-yellow-500 border-yellow-500/20" },
    approved: { label: t("balance.completed"), className: "text-green-500 border-green-500/20" },
    rejected: { label: t("balance.rejected") || "Rejected", className: "text-destructive border-destructive/20" },
  };

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold">{t("balance.title")}</h2>
        <p className="text-muted-foreground text-sm">{t("balance.subtitle")}</p>
      </div>

      <div className="grid lg:grid-cols-3 gap-6">
        <Card className="bg-card border-border">
          <CardContent className="p-6">
            <div className="flex items-center gap-3 mb-4">
              <div className="h-12 w-12 rounded-xl bg-primary/20 flex items-center justify-center">
                <Wallet className="h-6 w-6 text-primary" />
              </div>
              <div>
                <p className="text-sm text-muted-foreground">{t("balance.current")}</p>
                <p className="text-3xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
                  {profileLoading ? "..." : `$${balance.toLocaleString()}`}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>

        <Card className="lg:col-span-2 bg-card border-border">
          <CardHeader>
            <CardTitle className="text-lg flex items-center gap-2">
              <Plus className="h-5 w-5" />
              {t("balance.topUp")}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            <div className="space-y-2">
              <Label>{t("balance.amount")}</Label>
              <div className="flex flex-wrap gap-2">
                {amounts.map((a) => (
                  <button key={a} onClick={() => { setSelectedAmount(a); setCustomAmount(""); }}
                    className={cn("py-2 px-4 rounded-lg border text-sm font-medium transition-colors",
                      selectedAmount === a && !customAmount ? "border-primary bg-primary/10 text-primary" : "border-border bg-background hover:border-primary/50"
                    )}>
                    ${a.toLocaleString()}
                  </button>
                ))}
              </div>
              <div className="relative max-w-xs">
                <Input placeholder={t("balance.otherAmount")} value={customAmount}
                  onChange={(e) => { setCustomAmount(e.target.value); setSelectedAmount(null); }}
                  className="bg-background border-border pr-8" />
                <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
              </div>
            </div>

            <div className="space-y-2">
              <Label>{t("balance.paymentMethod")}</Label>
              <div className="grid sm:grid-cols-2 gap-3">
                {usdtMethods.map((m) => (
                  <button key={m.id} onClick={() => setSelectedMethod(m.id)}
                    className={cn("flex flex-col items-start gap-1 p-4 rounded-lg border transition-colors text-left",
                      selectedMethod === m.id ? "border-primary bg-primary/10" : "border-border bg-background hover:border-primary/50"
                    )}>
                    <span className={cn("text-sm font-medium", selectedMethod === m.id ? "text-foreground" : "text-muted-foreground")}>{m.label}</span>
                    <span className="text-xs text-muted-foreground">{m.desc}</span>
                  </button>
                ))}
              </div>
            </div>

            <div className="space-y-2">
              <Label className="flex items-center gap-2">
                <Tag className="h-4 w-4" />
                {t("balance.promo.label")}
              </Label>
              <div className="flex gap-2 max-w-sm">
                <Input
                  placeholder={t("balance.promo.placeholder")}
                  value={promoCode}
                  onChange={(e) => setPromoCode(e.target.value)}
                  className="bg-background border-border uppercase"
                  disabled={!!appliedPromo}
                />
                {appliedPromo ? (
                  <Button variant="outline" onClick={handleRemovePromo} className="border-border shrink-0">
                    {t("balance.promo.remove")}
                  </Button>
                ) : (
                  <Button variant="outline" onClick={handleApplyPromo} className="border-border shrink-0" disabled={!promoCode.trim()}>
                    {t("balance.promo.apply")}
                  </Button>
                )}
              </div>
              {appliedPromo && (
                <p className="text-sm text-primary font-medium">
                  ✓ {t("balance.promo.active").replace("{code}", appliedPromo.code).replace("{percent}", `${appliedPromo.bonus}`)}
                </p>
              )}
            </div>

            <div className="flex flex-col sm:flex-row sm:items-center gap-3">
              <Button onClick={handleTopUp} className="bg-accent hover:bg-accent/90 text-accent-foreground"
                disabled={!finalAmount || finalAmount < 100 || !!pendingPayment}>
                {t("balance.topUpBtn")} {finalAmount ? `$${finalAmount.toLocaleString()}` : ""}
                {appliedPromo && finalAmount ? ` (+${Math.floor(finalAmount * appliedPromo.bonus / 100)}$ ${t("balance.promo.bonusShort")})` : ""}
              </Button>
              {pendingPayment && (
                <p className="text-xs text-yellow-500">{t("balance.disabledReason")}</p>
              )}
            </div>
            <p className="text-xs text-muted-foreground">{t("balance.minAmount")}</p>
          </CardContent>
        </Card>
      </div>

      {/* Transaction History */}
      <Card className="bg-card border-border">
        <CardHeader>
          <CardTitle className="text-lg flex items-center gap-2">
            <Receipt className="h-5 w-5" /> {t("balance.history")}
          </CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            {loadingRequests ? (
              <div className="py-12 text-center text-muted-foreground">Loading...</div>
            ) : topupRequests.length === 0 ? (
              <div className="py-12 text-center text-muted-foreground">{t("balance.noTransactions")}</div>
            ) : (
              <table className="w-full">
                <thead>
                  <tr className="border-b border-border">
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("balance.date")}</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("balance.description")}</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("balance.amountCol")}</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("balance.statusCol")}</th>
                  </tr>
                </thead>
                <tbody>
                  {topupRequests.map((req) => {
                    const methodLabel = usdtMethods.find(m => m.id === req.payment_method)?.label || req.payment_method;
                    const st = statusMap[req.status] || statusMap.pending;
                    return (
                      <tr key={req.id} className="border-b border-border/50 hover:bg-muted/50 transition-colors">
                        <td className="py-3 px-4 text-sm">{formatDate(req.created_at)}</td>
                        <td className="py-3 px-4 text-sm">
                          {t("balance.topUpVia")} · {methodLabel}
                          {req.promocode_id && <span className="text-primary ml-1">({req.promocode_id})</span>}
                        </td>
                        <td className="py-3 px-4 text-sm text-left font-medium text-green-500">
                          +${req.deposit_amount.toLocaleString()}
                        </td>
                        <td className="py-3 px-4 text-left">
                          <Badge variant="outline" className={cn("font-normal", st.className)}>
                            {st.label}
                          </Badge>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
