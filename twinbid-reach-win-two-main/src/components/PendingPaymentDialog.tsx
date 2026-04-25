import { useState, useEffect } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Copy, ExternalLink, Send } from "lucide-react";
import { toast } from "sonner";
import { useLanguage } from "@/contexts/LanguageContext";
import { useAuth } from "@/contexts/AuthContext";
import { useNotifications } from "@/contexts/NotificationContext";
import { useProfile } from "@/contexts/ProfileContext";
import { usePendingPayment } from "@/contexts/PendingPaymentContext";

import { api } from "@/api";

const usdtMethods = [
  { id: "usdt_trc20", label: "USDT (TRC-20)", desc: "Tether on Tron", address: "TXkRh4pKz7w9Yb2mN5vQx8Gp3jL6fD0eW" },
  { id: "usdt_erc20", label: "USDT (ERC-20)", desc: "Tether on Ethereum", address: "0x3F7a9c2B1d5E8f4A6C0b9D1e2F3a4B5c6D7e8F9a" },
];

// Track the persistent "payment not completed" notification across the whole app.
let pendingNotifId: string | null = null;

export function PendingPaymentDialog() {
  const { t } = useLanguage();
  const { user } = useAuth();
  const { profile } = useProfile();
  const { notifications, addNotification, removeNotification, attachHandlers } = useNotifications();
  const {
    pendingPayment, setPendingPayment,
    isDialogOpen, openDialog, closeDialog,
    triggerRefresh,
  } = usePendingPayment();

  const [txHash, setTxHash] = useState("");

  useEffect(() => {
    if (isDialogOpen) setTxHash("");
  }, [isDialogOpen]);

  // Re-bind UI handlers to the persisted "incomplete_topup" notification on reload,
  // and rehydrate pendingPayment from it so the user can resume.
  useEffect(() => {
    const persisted = notifications.find(n => n.apiType === "incomplete_topup");
    if (!persisted) return;

    pendingNotifId = persisted.id;

    if (!pendingPayment && persisted.apiPayload?.deposit_amount) {
      setPendingPayment({
        amount: Number(persisted.apiPayload.deposit_amount) || 0,
        method: "usdt_trc20",
      });
    }
    attachHandlers(persisted.id, {
      action: { label: t("balance.notif.completePayment"), onClick: () => openDialog() },
      onDismiss: () => {
        setPendingPayment(null);
        setTxHash("");
        pendingNotifId = null;
        toast.info(t("balance.toast.paymentCanceled"));
      },
    });
    // We intentionally depend only on the notifications list snapshot
  }, [notifications]);

  const currentMethod = usdtMethods.find(m => m.id === pendingPayment?.method);

  const copyAddress = (address: string) => {
    navigator.clipboard.writeText(address);
    toast.success(t("balance.toast.addressCopied"));
  };

  const clearPendingNotif = () => {
    if (pendingNotifId) {
      removeNotification(pendingNotifId);
      pendingNotifId = null;
    }
  };

  const handleSubmitTx = async () => {
    if (!txHash.trim() || !user || !pendingPayment) return;

    let promocodeId: string | null = null;
    if (pendingPayment.promo) {
      try {
        const promo = await api.getPromocode(pendingPayment.promo);
        promocodeId = promo.id;
      } catch (e: any) {
        // Promo unknown to backend — submit without it but tell the user.
        toast.warning(`${t("balance.promo.invalid") || "Promo code lookup failed"}: ${e?.message || e}`);
      }
    }

    try {
      const depositAmount = pendingPayment.amount;
      const bonusPercent = pendingPayment.bonus || 0;
      const bonusAmount = Math.floor((depositAmount * bonusPercent) / 100);
      const nowIso = new Date().toISOString();
      await api.createTransaction({
        user_id: user.id,
        transaction_time: nowIso,
        transaction_id: txHash.trim(),
        payment_method: pendingPayment.method,
        bonus_amount: bonusAmount,
        promocode_id: promocodeId,
        transaction_hash: txHash.trim(),
        deposit_amount: depositAmount,
        total_balance_increase: depositAmount + bonusAmount,
        status: "pending",
        currency: "USDT",
      });
    } catch (e: any) {
      toast.error(`${t("balance.toast.submitError") || "Error submitting payment"}: ${e?.message || e}`);
      console.error(e);
      return;
    }

    toast.success(t("balance.toast.paymentSent"), {
      duration: 8000,
      description: t("balance.toast.paymentSupport"),
    });

    await addNotification({
      title: t("balance.notif.paymentSuccess"),
      description: t("balance.notif.paymentSuccessDesc").replace("${amount}", `$${pendingPayment.amount.toLocaleString()}`),
      type: "info",
      persistent: false,
    });

    closeDialog();
    setPendingPayment(null);
    setTxHash("");
    clearPendingNotif();
    triggerRefresh();
  };

  const handleCancelPayment = () => {
    closeDialog();
    setPendingPayment(null);
    setTxHash("");
    clearPendingNotif();
    toast.info(t("balance.toast.paymentCanceled"));
  };

  const handleOpenChange = async (open: boolean) => {
    if (open) {
      openDialog();
      return;
    }
    // Closing without submitting hash → keep pending payment & raise persistent notification
    if (pendingPayment && !txHash.trim()) {
      closeDialog();
      // Avoid duplicating the notification
      clearPendingNotif();
      const id = await addNotification({
        title: t("balance.notif.notCompleted"),
        description: `${t("balance.notif.noHash")} $${pendingPayment.amount}`,
        type: "warning",
        persistent: true,
        apiType: "incomplete_topup",
        apiPayload: { deposit_amount: pendingPayment.amount },
        action: { label: t("balance.notif.completePayment"), onClick: () => openDialog() },
        onDismiss: () => {
          setPendingPayment(null);
          setTxHash("");
          pendingNotifId = null;
          toast.info(t("balance.toast.paymentCanceled"));
        },
      });
      pendingNotifId = id;
      toast(t("balance.toast.notCompleted"), { duration: 5000 });
    } else {
      closeDialog();
    }
  };

  return (
    <Dialog open={isDialogOpen} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-[500px] bg-card border-border">
        <DialogHeader>
          <DialogTitle>{t("balance.paymentTitle")} {pendingPayment ? `$${pendingPayment.amount.toLocaleString()}` : ""}</DialogTitle>
          <DialogDescription>{t("balance.paymentDesc")}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 mt-2">
          <div className="p-3 rounded-lg bg-primary/10 border border-primary/20">
            <p className="text-sm font-medium">{t("balance.topUpAmount")} <span className="text-primary">${pendingPayment?.amount.toLocaleString()}</span></p>
            {pendingPayment?.bonus ? (
              <p className="text-sm text-primary mt-1">
                + {t("balance.promo.bonusShort")}: +{Math.floor((pendingPayment.amount * pendingPayment.bonus) / 100)}$ ({pendingPayment.promo}, +{pendingPayment.bonus}%)
              </p>
            ) : null}
          </div>

          <div className="space-y-2">
            <Label>{t("balance.walletAddress")} ({currentMethod?.label})</Label>
            <div className="flex gap-2">
              <Input value={currentMethod?.address || ""} readOnly className="bg-background border-border font-mono text-xs" />
              <Button variant="outline" size="icon" onClick={() => copyAddress(currentMethod?.address || "")} className="border-border shrink-0">
                <Copy className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">{t("balance.transferNote")}</p>
          </div>

          <div className="space-y-2">
            <Label>{t("balance.txHash")}</Label>
            <Input value={txHash} onChange={(e) => setTxHash(e.target.value)}
              placeholder="0x..." className="bg-background border-border font-mono text-sm" />
          </div>

          <Button onClick={handleSubmitTx} className="w-full bg-primary hover:bg-primary/90 text-primary-foreground"
            disabled={!txHash.trim()}>
            {t("balance.submit")}
          </Button>

          <Button variant="outline" className="w-full border-border" onClick={handleCancelPayment}>
            {t("balance.cancelPayment")}
          </Button>

          <div className="flex items-start gap-2 p-3 rounded-lg bg-muted/50 border border-border">
            <ExternalLink className="h-4 w-4 text-muted-foreground shrink-0 mt-0.5" />
            <div className="text-xs text-muted-foreground space-y-1">
              <p>{t("balance.supportText")}</p>
              {profile?.managerTelegram && (
                <a
                  href={`https://t.me/${profile.managerTelegram}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1.5 text-primary hover:text-primary/80 transition-colors font-medium"
                >
                  <Send className="h-3.5 w-3.5" />
                  @{profile.managerTelegram}
                </a>
              )}
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
