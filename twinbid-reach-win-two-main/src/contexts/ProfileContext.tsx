import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from "react";
import { api } from "@/api";
import type { ApiUser } from "@/api/types";
import { useAuth } from "@/contexts/AuthContext";

export interface Profile {
  id: string;
  email: string | null;
  fullName: string | null;
  telegram: string | null;
  timezone: string | null;
  balance: number;
  balanceThreshold: number;
  notifyCampaignStatus: boolean;
  notifyLowBalance: boolean;
  /** Toggle for the "campaign budget alert" notification. Persisted as `campaign_balanse_notifications` on the backend. */
  notifyCampaignBudget: boolean;
  managerTelegram: string;
}

interface ProfileContextType {
  profile: Profile | null;
  loading: boolean;
  refetch: () => Promise<void>;
  updateProfile: (updates: Partial<Omit<Profile, "id">>) => Promise<void>;
}

const ProfileContext = createContext<ProfileContextType | null>(null);

function fromApi(u: ApiUser, id: string): Profile {
  return {
    id,
    email: u.mail,
    fullName: u.name,
    telegram: u.telegram,
    timezone: u.timezone,
    balance: Number(u.balance) || 0,
    balanceThreshold: Number(u.balance_treshold) || 100,
    notifyCampaignStatus: u.campaign_status_notifications,
    notifyLowBalance: u.low_balance_notifications,
    notifyCampaignBudget: u.campaign_balanse_notifications,
    managerTelegram: u.manager_telegram || "",
  };
}

export function ProfileProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchProfile = useCallback(async () => {
    if (!user) { setProfile(null); setLoading(false); return; }
    setLoading(true);
    try {
      const u = await api.getProfile();
      setProfile(fromApi(u, user.id));
    } catch (e) {
      console.error("Profile fetch error:", e);
    } finally {
      setLoading(false);
    }
  }, [user]);

  useEffect(() => { fetchProfile(); }, [fetchProfile]);

  const updateProfile = useCallback(async (updates: Partial<Omit<Profile, "id">>) => {
    if (!user) return;
    const patch: Partial<ApiUser> = {};
    if (updates.fullName !== undefined) patch.name = updates.fullName ?? "";
    if (updates.email !== undefined) patch.mail = updates.email ?? "";
    if (updates.telegram !== undefined) patch.telegram = updates.telegram;
    if (updates.timezone !== undefined) patch.timezone = updates.timezone ?? "utc_3";
    if (updates.balanceThreshold !== undefined) patch.balance_treshold = updates.balanceThreshold;
    if (updates.notifyCampaignStatus !== undefined) patch.campaign_status_notifications = updates.notifyCampaignStatus;
    if (updates.notifyLowBalance !== undefined) patch.low_balance_notifications = updates.notifyLowBalance;
    if (updates.notifyCampaignBudget !== undefined) patch.campaign_balanse_notifications = updates.notifyCampaignBudget;
    const updated = await api.patchProfile(patch);
    setProfile(fromApi(updated, user.id));
  }, [user]);

  return (
    <ProfileContext.Provider value={{ profile, loading, refetch: fetchProfile, updateProfile }}>
      {children}
    </ProfileContext.Provider>
  );
}

export function useProfile() {
  const ctx = useContext(ProfileContext);
  if (!ctx) throw new Error("useProfile must be used within ProfileProvider");
  return ctx;
}
