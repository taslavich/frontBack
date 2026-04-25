import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from "react";
import { api } from "@/api";
import type { ApiNotification, NotificationType } from "@/api/types";
import { useAuth } from "@/contexts/AuthContext";

export interface Notification {
  id: string;
  title: string;
  description?: string;
  type: "info" | "warning" | "error";
  action?: { label: string; onClick: () => void };
  persistent?: boolean; // when true, also persisted to backend so it survives reloads
  onDismiss?: () => void; // called when user clicks X to dismiss
  // Backend linkage (only set for persisted notifications loaded from API)
  apiId?: string;
  apiType?: NotificationType;
  apiPayload?: {
    transaction_id?: string | null;
    campaign_id?: string | null;
    deposit_amount?: number | null;
  };
}

interface NotificationContextType {
  notifications: Notification[];
  /** Add a notification. If `persistent` is true and `apiType` is provided, it is also saved on the backend. */
  addNotification: (n: Omit<Notification, "id">) => Promise<string>;
  removeNotification: (id: string) => void;
  clearAll: () => void;
  /**
   * Re-attach UI handlers (action / onDismiss) to a persisted notification after reload.
   * Used by features that need their callbacks restored once contexts are mounted.
   */
  attachHandlers: (
    apiId: string,
    handlers: { action?: Notification["action"]; onDismiss?: Notification["onDismiss"] },
  ) => void;
}

const NotificationContext = createContext<NotificationContextType | null>(null);

function severityFor(type?: NotificationType): Notification["type"] {
  switch (type) {
    case "incomplete_topup":
    case "low_balance":
      return "warning";
    default:
      return "info";
  }
}

function titleFor(n: ApiNotification): string {
  // Backend stores a single `text`. Use the first line as the title, the rest as description.
  const [head] = (n.text || "").split("\n");
  return head || n.type;
}
function descFor(n: ApiNotification): string | undefined {
  const parts = (n.text || "").split("\n");
  return parts.length > 1 ? parts.slice(1).join("\n") : undefined;
}

export function NotificationProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [notifications, setNotifications] = useState<Notification[]>([]);

  // Hydrate from backend on login / mount
  useEffect(() => {
    if (!user) { setNotifications([]); return; }
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listNotifications();
        if (cancelled) return;
        setNotifications(
          list
            .filter(n => n.status === "active")
            .map<Notification>(n => ({
              id: n.id,
              apiId: n.id,
              apiType: n.type,
              apiPayload: {
                transaction_id: n.transaction_id,
                campaign_id: n.campaign_id,
                deposit_amount: n.deposit_amount,
              },
              title: titleFor(n),
              description: descFor(n),
              type: severityFor(n.type),
              persistent: true,
            })),
        );
      } catch (e) {
        console.error("listNotifications failed", e);
      }
    })();
    return () => { cancelled = true; };
  }, [user]);

  const addNotification = useCallback(async (n: Omit<Notification, "id">): Promise<string> => {
    // De-dupe by title (existing behavior)
    const existing = notifications.find(p => p.title === n.title);
    if (existing) return existing.id;

    let apiId: string | undefined;
    if (n.persistent && n.apiType) {
      try {
        const created = await api.createNotification({
          transaction_id: n.apiPayload?.transaction_id ?? null,
          campaign_id: n.apiPayload?.campaign_id ?? null,
          deposit_amount: n.apiPayload?.deposit_amount ?? null,
          text: n.description ? `${n.title}\n${n.description}` : n.title,
          type: n.apiType,
        });
        apiId = created.id;
      } catch (e) {
        console.error("createNotification failed", e);
      }
    }

    const id = apiId || (Date.now().toString(36) + Math.random().toString(36).slice(2, 6));
    setNotifications(prev => {
      if (prev.some(p => p.title === n.title)) return prev;
      return [...prev, { ...n, id, apiId }];
    });
    return id;
  }, [notifications]);

  const removeNotification = useCallback((id: string) => {
    setNotifications(prev => {
      const target = prev.find(n => n.id === id);
      if (target?.apiId) {
        // Mark as inactive on backend (best-effort)
        api.patchNotification(target.apiId, { status: "inactive" }).catch(e =>
          console.error("patchNotification(inactive) failed", e),
        );
      }
      return prev.filter(n => n.id !== id);
    });
  }, []);

  const clearAll = useCallback(() => {
    setNotifications(prev => {
      prev.forEach(n => {
        if (n.apiId) {
          api.patchNotification(n.apiId, { status: "inactive" }).catch(() => {});
        }
      });
      return [];
    });
  }, []);

  const attachHandlers = useCallback<NotificationContextType["attachHandlers"]>((apiId, handlers) => {
    setNotifications(prev => prev.map(n =>
      n.apiId === apiId
        ? { ...n, action: handlers.action ?? n.action, onDismiss: handlers.onDismiss ?? n.onDismiss }
        : n,
    ));
  }, []);

  return (
    <NotificationContext.Provider value={{ notifications, addNotification, removeNotification, clearAll, attachHandlers }}>
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotifications() {
  const ctx = useContext(NotificationContext);
  if (!ctx) throw new Error("useNotifications must be used within NotificationProvider");
  return ctx;
}
