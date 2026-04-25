import { useState, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, MoreHorizontal, Play, Pause, Pencil, Trash2, Eye, Filter, Copy, RotateCcw, XCircle, ArrowUpDown, ArrowUp, ArrowDown } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { useCampaigns, type Campaign } from "@/contexts/CampaignContext";
import { useLanguage } from "@/contexts/LanguageContext";
import { useCampaignStats, statOf } from "@/hooks/use-campaign-stats";

function isDraftComplete(c: Campaign): boolean {
  if (!c.name.trim()) return false;
  if (!c.formatKey) return false;
  if (!c.creatives?.length || !c.creatives[0]?.url?.trim()) return false;
  if (c.budget < 1) return false;
  if (!c.priceValue) return false;
  return true;
}

type SortKey = "name" | "status" | "format" | "budget" | "spent" | "impressions" | "ctr";
type SortDir = "asc" | "desc";

export default function DashboardCampaigns() {
  const navigate = useNavigate();
  const { campaigns, updateCampaign, deleteCampaign: ctxDelete, addCampaign } = useCampaigns();
  const { t } = useLanguage();
  const [viewCampaign, setViewCampaign] = useState<Campaign | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [searchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [sortKey, setSortKey] = useState<SortKey | null>(null);
  const [sortDir, setSortDir] = useState<SortDir>("asc");

  const statusConfig: Record<string, { label: string; className: string }> = {
    active: { label: t("status.active"), className: "bg-green-500/10 text-green-500 border-green-500/20" },
    paused: { label: t("status.paused"), className: "bg-yellow-500/10 text-yellow-500 border-yellow-500/20" },
    draft: { label: t("status.draft"), className: "bg-muted text-muted-foreground border-border" },
    completed: { label: t("status.completed"), className: "bg-blue-500/10 text-blue-500 border-blue-500/20" },
    moderation: { label: t("status.moderation"), className: "bg-purple-500/10 text-purple-500 border-purple-500/20" },
  };

  const filtered = campaigns.filter((c) => {
    const s = c.name.toLowerCase().includes(searchQuery.toLowerCase()) || c.id.includes(searchQuery);
    const st = statusFilter === "all" || c.status === statusFilter;
    return s && st;
  });

  // Per-campaign stats from ClickHouse layer
  const allIds = useMemo(() => campaigns.map(c => c.id), [campaigns]);
  const { byId: statsById } = useCampaignStats(allIds);

  const sorted = useMemo(() => {
    if (!sortKey) return filtered;
    return [...filtered].sort((a, b) => {
      const sa = statOf(statsById, a.id);
      const sb = statOf(statsById, b.id);
      let cmp = 0;
      switch (sortKey) {
        case "name": cmp = a.name.localeCompare(b.name); break;
        case "status": cmp = a.status.localeCompare(b.status); break;
        case "format": cmp = a.format.localeCompare(b.format); break;
        case "budget": cmp = a.budget - b.budget; break;
        case "spent": cmp = sa.spent - sb.spent; break;
        case "impressions": cmp = sa.impressions - sb.impressions; break;
        case "ctr": cmp = sa.ctr - sb.ctr; break;
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
  }, [filtered, sortKey, sortDir, statsById]);

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDir(prev => prev === "asc" ? "desc" : "asc");
    } else {
      setSortKey(key);
      setSortDir("asc");
    }
  };

  const SortIcon = ({ col }: { col: SortKey }) => {
    if (sortKey !== col) return <ArrowUpDown className="h-3 w-3 ml-1 opacity-40" />;
    return sortDir === "asc" ? <ArrowUp className="h-3 w-3 ml-1" /> : <ArrowDown className="h-3 w-3 ml-1" />;
  };

  // Stats based on filtered campaigns
  const totalCount = filtered.length;
  const activeCount = filtered.filter(c => c.status === "active").length;
  const totalBudget = filtered.reduce((s, c) => s + c.budget, 0);
  const totalSpent = filtered.reduce((s, c) => s + statOf(statsById, c.id).spent, 0);

  const toggleStatus = (id: string) => {
    const c = campaigns.find(x => x.id === id);
    if (!c) return;
    if (c.status === "draft" && !isDraftComplete(c)) {
      toast.error(t("campaigns.draftIncomplete"));
      return;
    }
    const ns = c.status === "active" ? "paused" : "active";
    updateCampaign(id, { status: ns as any });
    toast.success(ns === "active" ? t("campaigns.started") : t("campaigns.paused"));
  };

  const handleDelete = () => {
    if (deleteId) {
      ctxDelete(deleteId);
      toast.success(t("campaigns.deleted"));
      setDeleteId(null);
    }
  };

  const duplicateCampaign = (c: Campaign) => {
    const { id: _id, ...rest } = c;
    addCampaign({ ...rest, name: `${c.name} ${t("campaigns.copyPostfix")}`, status: "draft", spent: 0, impressions: 0, clicks: 0, ctr: 0 });
    toast.success(t("campaigns.copied"));
  };

  const handleCancelModeration = (id: string) => {
    updateCampaign(id, { status: "draft" });
    toast.success(t("campaigns.moderationCanceled"));
  };

  const handleRestart = (c: Campaign) => {
    navigate(`/dashboard/campaigns/${c.id}/edit?tab=budget`);
    toast.info(t("campaigns.restarted"));
  };

  return (
    <>
      <div className="space-y-6">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <div>
            <h2 className="text-2xl font-bold">{t("campaigns.title")}</h2>
            <p className="text-muted-foreground text-sm">{t("campaigns.subtitle")}</p>
          </div>
          <Button onClick={() => navigate("/dashboard/campaigns/create")} className="bg-primary hover:bg-primary/90">
            <Plus className="h-4 w-4 mr-2" /> {t("campaigns.create")}
          </Button>
        </div>

        <div className="flex flex-col sm:flex-row gap-3">
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[180px] bg-background border-border">
              <Filter className="h-4 w-4 mr-2" /><SelectValue placeholder={t("campaigns.allStatuses")} />
            </SelectTrigger>
            <SelectContent className="bg-card border-border">
              <SelectItem value="all">{t("campaigns.allStatuses")}</SelectItem>
              <SelectItem value="active">{t("campaigns.activeFilter")}</SelectItem>
              <SelectItem value="paused">{t("campaigns.pausedFilter")}</SelectItem>
              <SelectItem value="draft">{t("campaigns.draftsFilter")}</SelectItem>
              <SelectItem value="moderation">{t("campaigns.moderationFilter")}</SelectItem>
              <SelectItem value="completed">{t("campaigns.completedFilter")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Card className="bg-card border-border"><CardContent className="p-4"><p className="text-sm text-muted-foreground">{t("campaigns.total")}</p><p className="text-2xl font-bold">{totalCount}</p></CardContent></Card>
          <Card className="bg-card border-border"><CardContent className="p-4"><p className="text-sm text-muted-foreground">{t("campaigns.activeCount")}</p><p className="text-2xl font-bold text-green-500">{activeCount}</p></CardContent></Card>
          <Card className="bg-card border-border"><CardContent className="p-4"><p className="text-sm text-muted-foreground">{t("campaigns.budget")}</p><p className="text-2xl font-bold">${totalBudget.toLocaleString()}</p></CardContent></Card>
          <Card className="bg-card border-border"><CardContent className="p-4"><p className="text-sm text-muted-foreground">{t("overview.spent")}</p><p className="text-2xl font-bold">${totalSpent.toLocaleString()}</p></CardContent></Card>
        </div>

        <Card className="bg-card border-border">
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead className="sticky top-0 bg-card z-10">
                  <tr className="border-b border-border">
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground">{t("overview.id")}</th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("name")}>
                      <span className="inline-flex items-center">{t("overview.name")}<SortIcon col="name" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("status")}>
                      <span className="inline-flex items-center">{t("overview.status")}<SortIcon col="status" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("format")}>
                      <span className="inline-flex items-center">{t("campaigns.format")}<SortIcon col="format" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("budget")}>
                      <span className="inline-flex items-center">{t("campaigns.budget")}<SortIcon col="budget" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("spent")}>
                      <span className="inline-flex items-center">{t("overview.spent")}<SortIcon col="spent" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("impressions")}>
                      <span className="inline-flex items-center">{t("overview.impressions")}<SortIcon col="impressions" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground cursor-pointer select-none hover:text-foreground transition-colors" onClick={() => handleSort("ctr")}>
                      <span className="inline-flex items-center">CTR<SortIcon col="ctr" /></span>
                    </th>
                    <th className="text-left py-3 px-4 text-sm font-medium text-muted-foreground"></th>
                  </tr>
                </thead>
                <tbody>
                  {sorted.map((campaign) => {
                    const cs = statOf(statsById, campaign.id);
                    return (
                    <tr key={campaign.id} className="border-b border-border/50 hover:bg-muted/50 transition-colors">
                      <td className="py-4 px-4 text-muted-foreground font-mono text-sm">{campaign.id}</td>
                      <td className="py-4 px-4 font-medium">{campaign.name}</td>
                      <td className="py-4 px-4"><Badge variant="outline" className={cn("font-normal", statusConfig[campaign.status]?.className)}>{statusConfig[campaign.status]?.label}</Badge></td>
                      <td className="py-4 px-4 text-muted-foreground">{campaign.format}</td>
                      <td className="py-4 px-4">${campaign.budget.toLocaleString()}</td>
                      <td className="py-4 px-4">${cs.spent.toLocaleString()}</td>
                      <td className="py-4 px-4">{cs.impressions.toLocaleString()}</td>
                      <td className="py-4 px-4">{cs.ctr}%</td>
                      <td className="py-4 px-4 text-right">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild><Button variant="ghost" size="icon" className="h-8 w-8"><MoreHorizontal className="h-4 w-4" /></Button></DropdownMenuTrigger>
                          <DropdownMenuContent align="end" className="bg-card border-border">
                            <DropdownMenuItem className="gap-2" onClick={() => setViewCampaign(campaign)}><Eye className="h-4 w-4" /> {t("campaigns.view")}</DropdownMenuItem>
                            {campaign.status !== "moderation" && (
                              <DropdownMenuItem className="gap-2" onClick={() => navigate(`/dashboard/campaigns/${campaign.id}/edit`)}><Pencil className="h-4 w-4" /> {t("campaigns.edit")}</DropdownMenuItem>
                            )}
                            <DropdownMenuItem className="gap-2" onClick={() => duplicateCampaign(campaign)}><Copy className="h-4 w-4" /> {t("campaigns.copy")}</DropdownMenuItem>
                            <DropdownMenuSeparator />
                            {campaign.status === "active" && (
                              <DropdownMenuItem className="gap-2" onClick={() => toggleStatus(campaign.id)}><Pause className="h-4 w-4" /> {t("campaigns.pause")}</DropdownMenuItem>
                            )}
                            {campaign.status === "paused" && (
                              <DropdownMenuItem className="gap-2" onClick={() => toggleStatus(campaign.id)}><Play className="h-4 w-4" /> {t("campaigns.start")}</DropdownMenuItem>
                            )}
                            {campaign.status === "draft" && (
                              <DropdownMenuItem className="gap-2" onClick={() => navigate(`/dashboard/campaigns/${campaign.id}/edit`)}><Pencil className="h-4 w-4" /> {t("campaigns.finishCreation")}</DropdownMenuItem>
                            )}
                            {campaign.status === "completed" && (
                              <DropdownMenuItem className="gap-2" onClick={() => handleRestart(campaign)}><RotateCcw className="h-4 w-4" /> {t("campaigns.restart")}</DropdownMenuItem>
                            )}
                            {campaign.status === "moderation" && (
                              <DropdownMenuItem className="gap-2" onClick={() => handleCancelModeration(campaign.id)}><XCircle className="h-4 w-4" /> {t("campaigns.cancelModeration")}</DropdownMenuItem>
                            )}
                            {campaign.status !== "moderation" && (
                              <DropdownMenuItem className="gap-2 text-destructive" onClick={() => setDeleteId(campaign.id)}><Trash2 className="h-4 w-4" /> {t("campaigns.delete")}</DropdownMenuItem>
                            )}
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </td>
                    </tr>
                    );
                  })}
                  {sorted.length === 0 && <tr><td colSpan={9} className="py-12 text-center text-muted-foreground">{t("campaigns.notFound")}</td></tr>}
                </tbody>
              </table>
            </div>
          </CardContent>
        </Card>
      </div>

      <AlertDialog open={!!deleteId} onOpenChange={(open) => !open && setDeleteId(null)}>
        <AlertDialogContent className="bg-card border-border">
          <AlertDialogHeader>
            <AlertDialogTitle>{t("campaigns.deleteConfirm")}</AlertDialogTitle>
            <AlertDialogDescription>{t("campaigns.deleteDesc")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel className="border-border">{t("campaigns.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">{t("campaigns.delete")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <Dialog open={!!viewCampaign} onOpenChange={() => setViewCampaign(null)}>
        <DialogContent className="sm:max-w-[600px] bg-card border-border">
          <DialogHeader><DialogTitle>{viewCampaign?.name}</DialogTitle><DialogDescription>ID: {viewCampaign?.id}</DialogDescription></DialogHeader>
          {viewCampaign && (() => {
            const vs = statOf(statsById, viewCampaign.id);
            return (
            <div className="grid grid-cols-2 gap-4 mt-4">
              {[
                [t("overview.status"), <Badge variant="outline" className={cn("font-normal", statusConfig[viewCampaign.status]?.className)}>{statusConfig[viewCampaign.status]?.label}</Badge>],
                [t("campaigns.format"), viewCampaign.format],
                [t("campaigns.budget"), `$${viewCampaign.budget.toLocaleString()}`],
                [t("overview.spent"), `$${vs.spent.toLocaleString()}`],
                [t("overview.impressions"), vs.impressions.toLocaleString()],
                [t("stats.clicks"), vs.clicks.toLocaleString()],
                ["CTR", `${vs.ctr}%`],
                [t("stats.spent"), `$${viewCampaign.priceValue} (${viewCampaign.pricingModel.toUpperCase()})`],
              ].map(([label, val], i) => (
                <div key={i} className="space-y-1"><p className="text-sm text-muted-foreground">{label as string}</p><div className="font-medium">{val}</div></div>
              ))}
            </div>
            );
          })()}
        </DialogContent>
      </Dialog>
    </>
  );
}
