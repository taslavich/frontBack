import { useState, useEffect } from "react";
import { Bell, User, X, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
import { useNotifications, type Notification } from "@/contexts/NotificationContext";
import { useLanguage } from "@/contexts/LanguageContext";
import { useAuth } from "@/contexts/AuthContext";
import { useCampaigns } from "@/contexts/CampaignContext";
import { useProfile } from "@/contexts/ProfileContext";
import { useCampaignStats, statOf } from "@/hooks/use-campaign-stats";
import { useMemo } from "react";
import { LanguageSelector } from "@/components/LanguageSelector";
import { cn } from "@/lib/utils";
import { useNavigate } from "react-router-dom";

export function DashboardHeader() {
  const { notifications, addNotification, removeNotification } = useNotifications();
  const { t } = useLanguage();
  const { user } = useAuth();
  const { campaigns } = useCampaigns();
  const { profile } = useProfile();
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();
  const [confirmDismiss, setConfirmDismiss] = useState<Notification | null>(null);

  // Campaign budget alerts - notify when <10% budget remaining (spend from stats)
  const activeIds = useMemo(
    () => campaigns.filter(c => c.status === "active" && c.budget > 0).map(c => c.id),
    [campaigns]
  );
  const { byId: statsById } = useCampaignStats(activeIds);
  useEffect(() => {
    if (!profile?.notifyCampaignStatus) return;
    campaigns
      .filter(c => c.status === "active" && c.budget > 0)
      .forEach(c => {
        const spent = statOf(statsById, c.id).spent;
        if (spent < c.budget * 0.9) return;
        void addNotification({
          title: t("notif.campaignBudgetLow"),
          description: `${c.name}: ${Math.round((1 - spent / c.budget) * 100)}% ${t("notif.budgetRemaining")}`,
          type: "warning",
          persistent: false,
        });
      });
  }, [campaigns, statsById]);

  const handleDismissClick = (n: Notification) => {
    if (n.onDismiss) {
      // Show confirmation dialog for notifications with onDismiss (pending transactions)
      setConfirmDismiss(n);
    } else {
      removeNotification(n.id);
    }
  };

  const handleConfirmCancel = () => {
    if (confirmDismiss) {
      confirmDismiss.onDismiss?.();
      removeNotification(confirmDismiss.id);
      setConfirmDismiss(null);
    }
  };

  return (
    <>
      <header className="h-16 border-b border-border bg-card px-6 flex items-center justify-between">
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <span>{t("header.manager")}:</span>
          {profile?.managerTelegram && (
            <a href={`https://t.me/${profile.managerTelegram}`} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 text-primary hover:text-primary/80 transition-colors font-medium">
              <Send className="h-4 w-4" />
              @{profile.managerTelegram}
            </a>
          )}
        </div>
        <div className="flex items-center gap-4">
          <LanguageSelector />
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button variant="ghost" size="icon" className="relative">
                <Bell className="h-5 w-5" />
                {notifications.length > 0 && (
                  <span className="absolute top-1 right-1 h-4 w-4 bg-accent rounded-full text-[10px] font-bold flex items-center justify-center text-accent-foreground">
                    {notifications.length}
                  </span>
                )}
              </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[340px] p-0" align="end">
              <div className="p-3 border-b border-border">
                <p className="text-sm font-medium">{t("header.notifications")}</p>
              </div>
              <div className="max-h-80 overflow-y-auto">
                {notifications.length === 0 ? (
                  <div className="p-6 text-center text-sm text-muted-foreground">
                    {t("header.noNotifications")}
                  </div>
                ) : (
                  notifications.map((n) => (
                    <div key={n.id} className="p-3 border-b border-border/50 hover:bg-muted/50 transition-colors">
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex-1 min-w-0">
                          <p className={cn("text-sm font-medium", n.type === "warning" && "text-yellow-500", n.type === "error" && "text-destructive")}>
                            {n.title}
                          </p>
                          {n.description && <p className="text-xs text-muted-foreground mt-0.5">{n.description}</p>}
                          {n.action && (
                            <button
                              onClick={() => { n.action!.onClick(); setOpen(false); }}
                              className="text-xs text-primary hover:underline mt-1"
                            >
                              {n.action.label}
                            </button>
                          )}
                        </div>
                        <button onClick={() => handleDismissClick(n)} className="text-muted-foreground hover:text-foreground shrink-0">
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </PopoverContent>
          </Popover>
          <button
            onClick={() => navigate("/dashboard/settings")}
            className="flex items-center gap-3 hover:opacity-80 transition-opacity cursor-pointer"
          >
            <div className="h-9 w-9 rounded-full bg-primary/20 flex items-center justify-center">
              <User className="h-5 w-5 text-primary" />
            </div>
            <div className="hidden sm:block text-left">
              <p className="text-sm font-medium">{user?.email || "user@example.com"}</p>
              <p className="text-xs text-muted-foreground">{t("header.advertiser")}</p>
            </div>
          </button>
        </div>
      </header>

      <AlertDialog open={!!confirmDismiss} onOpenChange={(o) => { if (!o) setConfirmDismiss(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("balance.notif.cancelConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("balance.notif.cancelConfirmDesc")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("balance.notif.cancelConfirmNo")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmCancel} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              {t("balance.notif.cancelConfirmYes")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
