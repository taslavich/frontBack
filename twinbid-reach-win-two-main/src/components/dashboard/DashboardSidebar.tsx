import { LayoutDashboard, Megaphone, Wallet, Settings, LogOut, BarChart3 } from "lucide-react";
import { cn } from "@/lib/utils";
import { useNavigate, useLocation } from "react-router-dom";
import { useLanguage } from "@/contexts/LanguageContext";
import { useAuth } from "@/contexts/AuthContext";
import twinbidLogo from "@/assets/twinbid-logo.svg";

export function DashboardSidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useLanguage();
  const { signOut } = useAuth();

  const menuItems = [
    { icon: LayoutDashboard, label: t("sidebar.overview"), path: "/dashboard" },
    { icon: Megaphone, label: t("sidebar.campaigns"), path: "/dashboard/campaigns" },
    { icon: BarChart3, label: t("sidebar.statistics"), path: "/dashboard/statistics" },
    { icon: Wallet, label: t("sidebar.balance"), path: "/dashboard/balance" },
    { icon: Settings, label: t("sidebar.settings"), path: "/dashboard/settings" },
  ];

  return (
    <aside className="w-64 bg-card border-r border-border flex flex-col">
      <div className="p-6 border-b border-border">
        <img src={twinbidLogo} alt="TwinBid" className="h-9 cursor-pointer" onClick={() => navigate("/dashboard")} />
      </div>
      <nav className="flex-1 p-4 space-y-2">
        {menuItems.map((item) => {
          const isActive = location.pathname === item.path ||
            (item.path !== "/dashboard" && location.pathname.startsWith(item.path));
          return (
            <button
              key={item.path}
              onClick={() => navigate(item.path)}
              className={cn(
                "w-full flex items-center gap-3 px-4 py-3 rounded-lg text-left transition-colors",
                isActive ? "bg-primary/10 text-primary" : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )}
            >
              <item.icon className="h-5 w-5" />
              <span>{item.label}</span>
            </button>
          );
        })}
      </nav>
      <div className="p-4 border-t border-border">
        <button onClick={async () => { await signOut(); navigate("/"); }} className="w-full flex items-center gap-3 px-4 py-3 rounded-lg text-muted-foreground hover:bg-destructive/10 hover:text-destructive transition-colors">
          <LogOut className="h-5 w-5" /><span>{t("sidebar.logout")}</span>
        </button>
      </div>
    </aside>
  );
}
