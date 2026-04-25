import { Plus, MoreHorizontal, Play, Pause, Pencil, Trash2, Eye } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useState } from "react";
import { CreateCampaignDialog } from "./CreateCampaignDialog";
import { cn } from "@/lib/utils";
import { useLanguage } from "@/contexts/LanguageContext";

interface Campaign {
  id: string;
  name: string;
  status: "active" | "paused" | "draft";
  format: string;
  budget: number;
  spent: number;
  impressions: number;
  clicks: number;
}

const campaigns: Campaign[] = [
  { id: "1", name: "Summer Sale 2024", status: "active", format: "Banner", budget: 50000, spent: 23400, impressions: 45230, clicks: 2890 },
  { id: "2", name: "New Collection", status: "active", format: "Video", budget: 100000, spent: 67800, impressions: 89120, clicks: 5670 },
  { id: "3", name: "Brand Campaign", status: "paused", format: "Banner", budget: 30000, spent: 15200, impressions: 28900, clicks: 1240 },
  { id: "4", name: "Test Campaign", status: "draft", format: "CTV", budget: 25000, spent: 0, impressions: 0, clicks: 0 },
];

export function CampaignsList() {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const { t } = useLanguage();

  const statusConfig: Record<string, { label: string; className: string }> = {
    active: { label: t("status.active"), className: "bg-green-500/10 text-green-500 border-green-500/20" },
    paused: { label: t("status.paused"), className: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20" },
    draft: { label: t("status.draft"), className: "bg-muted text-muted-foreground border-border" },
  };

  return (
    <>
      <Card className="bg-card border-border">
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>{t("legacy.campaigns")}</CardTitle>
          <Button onClick={() => setIsCreateOpen(true)} className="bg-primary hover:bg-primary/90">
            <Plus className="h-4 w-4 mr-2" />
            {t("legacy.createCampaign")}
          </Button>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.campaignName")}</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.status")}</th>
                  <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.format")}</th>
                  <th className="text-right py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.budget")}</th>
                  <th className="text-right py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.spent")}</th>
                  <th className="text-right py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.impressions")}</th>
                  <th className="text-right py-3 px-4 text-sm font-medium text-muted-foreground">{t("legacy.clicks")}</th>
                  <th className="text-right py-3 px-4 text-sm font-medium text-muted-foreground"></th>
                </tr>
              </thead>
              <tbody>
                {campaigns.map((campaign) => (
                  <tr key={campaign.id} className="border-b border-border/50 hover:bg-muted/50 transition-colors">
                    <td className="py-4 px-4"><span className="font-medium">{campaign.name}</span></td>
                    <td className="py-4 px-4">
                      <Badge variant="outline" className={cn("font-normal", statusConfig[campaign.status].className)}>
                        {statusConfig[campaign.status].label}
                      </Badge>
                    </td>
                    <td className="py-4 px-4 text-muted-foreground">{campaign.format}</td>
                    <td className="py-4 px-4 text-right">${campaign.budget.toLocaleString()}</td>
                    <td className="py-4 px-4 text-right">${campaign.spent.toLocaleString()}</td>
                    <td className="py-4 px-4 text-right">{campaign.impressions.toLocaleString()}</td>
                    <td className="py-4 px-4 text-right">{campaign.clicks.toLocaleString()}</td>
                    <td className="py-4 px-4 text-right">
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="h-8 w-8"><MoreHorizontal className="h-4 w-4" /></Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="bg-card border-border">
                          <DropdownMenuItem className="gap-2"><Eye className="h-4 w-4" /> {t("legacy.view")}</DropdownMenuItem>
                          <DropdownMenuItem className="gap-2"><Pencil className="h-4 w-4" /> {t("legacy.edit")}</DropdownMenuItem>
                          {campaign.status === "active" ? (
                            <DropdownMenuItem className="gap-2"><Pause className="h-4 w-4" /> {t("legacy.pause")}</DropdownMenuItem>
                          ) : (
                            <DropdownMenuItem className="gap-2"><Play className="h-4 w-4" /> {t("legacy.start")}</DropdownMenuItem>
                          )}
                          <DropdownMenuItem className="gap-2 text-destructive"><Trash2 className="h-4 w-4" /> {t("legacy.delete")}</DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
      <CreateCampaignDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} />
    </>
  );
}
