import { useMemo } from "react";
import { Eye, MousePointer, Target } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { useLanguage } from "@/contexts/LanguageContext";
import { useCampaigns } from "@/contexts/CampaignContext";
import { useCampaignStats } from "@/hooks/use-campaign-stats";

export function StatsCards() {
  const { t } = useLanguage();
  const { campaigns } = useCampaigns();
  const ids = useMemo(() => campaigns.map(c => c.id), [campaigns]);
  const { totals } = useCampaignStats(ids);

  const ctr = totals.impressions > 0 ? ((totals.clicks / totals.impressions) * 100).toFixed(2) : "0.00";

  const stats = [
    { label: t("statsCards.impressions"), value: totals.impressions.toLocaleString(), icon: Eye, color: "text-primary" },
    { label: t("statsCards.clicks"), value: totals.clicks.toLocaleString(), icon: MousePointer, color: "text-primary" },
    { label: t("statsCards.ctr"), value: `${ctr}%`, icon: Target, color: "text-primary" },
  ];

  return (
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
  );
}
