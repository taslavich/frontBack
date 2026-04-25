import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { User, Bell, Shield, Save } from "lucide-react";
import { toast } from "sonner";
import { useLanguage } from "@/contexts/LanguageContext";
import { useProfile } from "@/contexts/ProfileContext";
import { api } from "@/api";
import { notifyError } from "@/lib/apiStatus";

export default function DashboardSettings() {
  const { t } = useLanguage();
  const { profile, loading, updateProfile } = useProfile();

  const [contactName, setContactName] = useState("");
  const [email, setEmail] = useState("");
  const [telegram, setTelegram] = useState("");
  const [timezone, setTimezone] = useState("utc_3");
  const [notifyCampaign, setNotifyCampaign] = useState(true);
  const [notifyBalance, setNotifyBalance] = useState(true);
  // Independent toggle for the campaign-budget alert. Previously bound to the
  // same state as `notifyCampaign`, which caused both flags to flip together
  // and always send `true` to the backend.
  const [notifyBudget, setNotifyBudget] = useState(true);
  const [balanceThreshold, setBalanceThreshold] = useState("100");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (profile) {
      setContactName(profile.fullName || "");
      setEmail(profile.email || "");
      setTelegram(profile.telegram || "");
      setTimezone(profile.timezone || "utc_3");
      setNotifyCampaign(profile.notifyCampaignStatus);
      setNotifyBalance(profile.notifyLowBalance);
      setNotifyBudget(profile.notifyCampaignBudget);
      setBalanceThreshold(String(profile.balanceThreshold));
    }
  }, [profile]);

  const handleSaveProfile = async () => {
    setSaving(true);
    try {
      await updateProfile({
        fullName: contactName,
        telegram,
        timezone,
      });
      toast.success(t("settings.saved"));
    } catch (err) {
      notifyError(t("settings.saveError") || "Error saving profile", err);
    }
    setSaving(false);
  };

  const handleSaveNotifications = async () => {
    setSaving(true);
    try {
      await updateProfile({
        notifyCampaignStatus: notifyCampaign,
        notifyLowBalance: notifyBalance,
        notifyCampaignBudget: notifyBudget,
        balanceThreshold: parseInt(balanceThreshold) || 100,
      });
      toast.success(t("settings.saved"));
    } catch (err) {
      notifyError(t("settings.saveError") || "Error saving notifications", err);
    }
    setSaving(false);
  };

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [repeatPassword, setRepeatPassword] = useState("");

  const handleChangePassword = async () => {
    if (newPassword !== repeatPassword) {
      toast.error(t("settings.passwordMismatch") || "Passwords don't match");
      return;
    }
    if (newPassword.length < 6) {
      toast.error(t("settings.passwordTooShort") || "Password must be at least 6 characters");
      return;
    }
    try {
      await api.changePassword({ new_password: newPassword });
      toast.success(t("settings.passwordUpdated"));
      setCurrentPassword("");
      setNewPassword("");
      setRepeatPassword("");
    } catch (e: any) {
      notifyError(t("settings.passwordError") || "Error updating password", e);
    }
  };

  if (loading) return <div className="py-12 text-center text-muted-foreground">Loading...</div>;

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold">{t("settings.title")}</h2>
        <p className="text-muted-foreground text-sm">{t("settings.subtitle")}</p>
      </div>

      <Tabs defaultValue="profile" className="space-y-6">
        <TabsList className="bg-card border border-border">
          <TabsTrigger value="profile" className="gap-2"><User className="h-4 w-4" /> {t("settings.profile")}</TabsTrigger>
          <TabsTrigger value="notifications" className="gap-2"><Bell className="h-4 w-4" /> {t("settings.notifications")}</TabsTrigger>
          <TabsTrigger value="security" className="gap-2"><Shield className="h-4 w-4" /> {t("settings.security")}</TabsTrigger>
        </TabsList>

        <TabsContent value="profile">
          <Card className="bg-card border-border">
            <CardHeader><CardTitle className="text-lg">{t("settings.profile")}</CardTitle></CardHeader>
            <CardContent className="space-y-4">
              <div className="grid sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label>{t("settings.name")}</Label>
                  <Input value={contactName} onChange={(e) => setContactName(e.target.value)} className="bg-background border-border" />
                </div>
                <div className="space-y-2">
                  <Label>{t("settings.email")}</Label>
                  <Input value={email} disabled className="bg-muted border-border text-muted-foreground cursor-not-allowed" />
                </div>
                <div className="space-y-2">
                  <Label>{t("settings.telegram")}</Label>
                  <Input value={telegram} onChange={(e) => setTelegram(e.target.value)} placeholder="@username" className="bg-background border-border" />
                </div>
                <div className="space-y-2">
                  <Label>{t("settings.timezone")}</Label>
                  <Select value={timezone} onValueChange={setTimezone}>
                    <SelectTrigger className="bg-background border-border"><SelectValue /></SelectTrigger>
                    <SelectContent className="bg-card border-border">
                      <SelectItem value="utc_m5">UTC-5 (EST)</SelectItem>
                      <SelectItem value="utc_0">UTC+0 (GMT)</SelectItem>
                      <SelectItem value="utc_1">UTC+1 (CET)</SelectItem>
                      <SelectItem value="utc_2">UTC+2 (EET)</SelectItem>
                      <SelectItem value="utc_3">UTC+3 (MSK)</SelectItem>
                      <SelectItem value="utc_5_30">UTC+5:30 (IST)</SelectItem>
                      <SelectItem value="utc_8">UTC+8 (CST)</SelectItem>
                      <SelectItem value="utc_9">UTC+9 (JST)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <Button onClick={handleSaveProfile} disabled={saving} className="bg-primary hover:bg-primary/90">
                <Save className="h-4 w-4 mr-2" />{t("settings.save")}
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="notifications">
          <Card className="bg-card border-border">
            <CardHeader><CardTitle className="text-lg">{t("settings.notifications")}</CardTitle></CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-4">
                <h4 className="text-sm font-medium text-muted-foreground">{t("settings.emailNotifications")}</h4>
                <div className="flex items-center justify-between">
                  <Label>{t("settings.campaignStatus")}</Label>
                  <Switch checked={notifyCampaign} onCheckedChange={setNotifyCampaign} />
                </div>
                <div className="flex items-center justify-between">
                  <Label>{t("settings.lowBalance")}</Label>
                  <Switch checked={notifyBalance} onCheckedChange={setNotifyBalance} />
                </div>
                <div className="flex items-center justify-between">
                  <div>
                    <Label>{t("settings.campaignBudgetAlert")}</Label>
                    <p className="text-xs text-muted-foreground mt-0.5">{t("settings.campaignBudgetAlertDesc")}</p>
                  </div>
                  <Switch checked={notifyBudget} onCheckedChange={setNotifyBudget} />
                </div>
              </div>
              <Separator />
              <div className="space-y-2 max-w-xs">
                <Label>{t("settings.balanceThreshold")}</Label>
                <div className="relative">
                  <Input value={balanceThreshold} onChange={(e) => setBalanceThreshold(e.target.value)} className="bg-background border-border pr-8" />
                  <span className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
                </div>
              </div>
              <Button onClick={handleSaveNotifications} disabled={saving} className="bg-primary hover:bg-primary/90">
                <Save className="h-4 w-4 mr-2" />{t("settings.save")}
              </Button>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security">
          <Card className="bg-card border-border">
            <CardHeader><CardTitle className="text-lg">{t("settings.security")}</CardTitle></CardHeader>
            <CardContent className="space-y-6">
              <div className="space-y-4">
                <div className="space-y-2">
                  <Label>{t("settings.currentPassword")}</Label>
                  <Input type="password" placeholder="••••••••" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} className="bg-background border-border max-w-sm" />
                </div>
                <div className="space-y-2">
                  <Label>{t("settings.newPassword")}</Label>
                  <Input type="password" placeholder="••••••••" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} className="bg-background border-border max-w-sm" />
                </div>
                <div className="space-y-2">
                  <Label>{t("settings.repeatPassword")}</Label>
                  <Input type="password" placeholder="••••••••" value={repeatPassword} onChange={(e) => setRepeatPassword(e.target.value)} className="bg-background border-border max-w-sm" />
                </div>
                <Button onClick={handleChangePassword} className="bg-primary hover:bg-primary/90">{t("settings.changePassword")}</Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
