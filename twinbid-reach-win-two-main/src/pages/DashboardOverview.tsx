import { useEffect, useMemo } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useLanguage } from "@/contexts/LanguageContext";
import { useCampaigns } from "@/contexts/CampaignContext";
import { BalanceCard } from "@/components/dashboard/BalanceCard";
import { Eye, MousePointer, Target } from "lucide-react";
import { useCampaignStats, statOf } from "@/hooks/use-campaign-stats";

export default function DashboardOverview() {
  const { t } = useLanguage();
  const { campaigns, refetch } = useCampaigns();

  useEffect(() => { refetch(); }, []);

  // Only show campaigns that are not draft or completed
  const activeCampaigns = useMemo(
    () => campaigns.filter(c => c.status !== "draft" && c.status !== "completed"),
    [campaigns]
  );
  const ids = useMemo(() => activeCampaigns.map(c => c.id), [activeCampaigns]);
  const { byId, totals } = useCampaignStats(ids);

  const ctr = totals.impressions > 0 ? ((totals.clicks / totals.impressions) * 100).toFixed(2) : "0.00";

  const stats = [
    { label: t("statsCards.impressions"), value: totals.impressions.toLocaleString(), icon: Eye, color: "text-primary" },
    { label: t("statsCards.clicks"), value: totals.clicks.toLocaleString(), icon: MousePointer, color: "text-primary" },
    { label: t("statsCards.ctr"), value: `${ctr}%`, icon: Target, color: "text-primary" },
  ];

  const statusConfig: Record<string, { label: string; className: string }> = {
    active: { label: t("status.active"), className: "bg-green-500/10 text-green-500 border-green-500/20" },
    paused: { label: t("status.paused"), className: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20" },
    moderation: { label: t("status.moderation"), className: "bg-purple-500/10 text-purple-500 border-purple-500/20" },
  };

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <div className="lg:col-span-3">
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            {stats.map((stat) => (
              <Card key={stat.label} className="bg-card border-border">
                <CardContent className="p-6">
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm text-muted-foreground">{stat.label}</p>
                      <p className="text-2xl font-bold mt-1">{stat.value}</p>
                    </div>
                    <div className={`h-12 w-12 rounded-lg bg-muted flex items-center justify-center ${stat.color}`}>
                      <stat.icon className="h-6 w-6" />
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        </div>
        <div className="lg:col-span-1"><BalanceCard /></div>
      </div>
      <Card className="bg-card border-border">
        <CardHeader><CardTitle>{t("campaigns.title")}</CardTitle></CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("overview.id")}</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("overview.name")}</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("overview.status")}</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("overview.impressions")}</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("overview.spent")}</th>
                </tr>
              </thead>
              <tbody>
                {activeCampaigns.length === 0 ? (
                  <tr><td colSpan={5} className="py-12 text-center text-muted-foreground">{t("campaigns.notFound")}</td></tr>
                ) : activeCampaigns.map((c) => {
                  const s = statOf(byId, c.id);
                  return (
                    <tr key={c.id} className="border-b border-border/50">
                      <td className="py-3 px-4 text-muted-foreground font-mono text-sm">{c.id}</td>
                      <td className="py-3 px-4 font-medium">{c.name}</td>
                      <td className="py-3 px-4">
                        <Badge variant="outline" className={cn("font-normal", statusConfig[c.status]?.className)}>
                          {statusConfig[c.status]?.label}
                        </Badge>
                      </td>
                      <td className="py-3 px-4">{s.impressions.toLocaleString()}</td>
                      <td className="py-3 px-4">${s.spent.toLocaleString()}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
