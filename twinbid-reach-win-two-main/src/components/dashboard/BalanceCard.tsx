import { Wallet, Plus } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useNavigate } from "react-router-dom";
import { useLanguage } from "@/contexts/LanguageContext";
import { useProfile } from "@/contexts/ProfileContext";

export function BalanceCard() {
  const navigate = useNavigate();
  const { t } = useLanguage();
  const { profile, loading } = useProfile();

  const balance = profile?.balance ?? 0;

  return (
    <Card className="bg-card border-border h-full">
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground flex items-center gap-2">
          <Wallet className="h-4 w-4" />
          {t("balanceCard.title")}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
          {loading ? "..." : `$${balance.toLocaleString()}`}
        </p>
        <p className="text-xs text-muted-foreground mt-1">
          {t("balanceCard.daysLeft")}
        </p>
        <Button
          onClick={() => navigate("/dashboard/balance")}
          className="w-full mt-4 bg-accent hover:bg-accent/90 text-accent-foreground"
        >
          <Plus className="h-4 w-4 mr-2" />
          {t("balanceCard.topUp")}
        </Button>
      </CardContent>
    </Card>
  );
}
