import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { CampaignProvider } from "./contexts/CampaignContext";
import { NotificationProvider } from "./contexts/NotificationContext";
import { LanguageProvider } from "./contexts/LanguageContext";
import { StatisticsProvider } from "./contexts/StatisticsContext";
import { AuthProvider } from "./contexts/AuthContext";
import { ProfileProvider } from "./contexts/ProfileContext";
import { PendingPaymentProvider } from "./contexts/PendingPaymentContext";
import { PendingPaymentDialog } from "./components/PendingPaymentDialog";
import { ProtectedRoute } from "./components/ProtectedRoute";
import Index from "./pages/Index";
import NotFound from "./pages/NotFound";
import Dashboard from "./pages/Dashboard";
import DashboardOverview from "./pages/DashboardOverview";
import DashboardCampaigns from "./pages/DashboardCampaigns";
import DashboardStatistics from "./pages/DashboardStatistics";
import DashboardBalance from "./pages/DashboardBalance";
import DashboardSettings from "./pages/DashboardSettings";
import CreateCampaign from "./pages/CreateCampaign";
import EditCampaign from "./pages/EditCampaign";

const queryClient = new QueryClient();

const App = () => (
  <QueryClientProvider client={queryClient}>
    <TooltipProvider>
      <Toaster />
      <Sonner />
      <BrowserRouter>
        <LanguageProvider>
          <AuthProvider>
            <ProfileProvider>
              <NotificationProvider>
                <PendingPaymentProvider>
                  <CampaignProvider>
                  <StatisticsProvider>
                    <Routes>
                      <Route path="/" element={<Index />} />
                      <Route path="/dashboard" element={<ProtectedRoute><Dashboard /></ProtectedRoute>}>
                        <Route index element={<DashboardOverview />} />
                        <Route path="campaigns" element={<DashboardCampaigns />} />
                        <Route path="campaigns/create" element={<CreateCampaign />} />
                        <Route path="campaigns/:id/edit" element={<EditCampaign />} />
                        <Route path="statistics" element={<DashboardStatistics />} />
                        <Route path="balance" element={<DashboardBalance />} />
                        <Route path="settings" element={<DashboardSettings />} />
                      </Route>
                      <Route path="*" element={<NotFound />} />
                    </Routes>
                    <PendingPaymentDialog />
                    </StatisticsProvider>
                  </CampaignProvider>
                </PendingPaymentProvider>
              </NotificationProvider>
            </ProfileProvider>
          </AuthProvider>
        </LanguageProvider>
      </BrowserRouter>
    </TooltipProvider>
  </QueryClientProvider>
);

export default App;
