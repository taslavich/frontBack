import { createContext, useContext, useState, useCallback, useRef, type ReactNode } from "react";

export interface PendingPaymentData {
  amount: number;
  method: string;
  promo?: string;
  bonus?: number;
}

interface PendingPaymentContextType {
  pendingPayment: PendingPaymentData | null;
  setPendingPayment: (p: PendingPaymentData | null) => void;
  isDialogOpen: boolean;
  openDialog: () => void;
  closeDialog: () => void;
  // Notify Balance page to refresh history after submission
  registerRefreshHandler: (fn: () => void) => void;
  triggerRefresh: () => void;
}

const PendingPaymentContext = createContext<PendingPaymentContextType | null>(null);

export function PendingPaymentProvider({ children }: { children: ReactNode }) {
  const [pendingPayment, setPendingPayment] = useState<PendingPaymentData | null>(null);
  const [isDialogOpen, setDialogOpen] = useState(false);
  const refreshRef = useRef<() => void>(() => {});

  const openDialog = useCallback(() => setDialogOpen(true), []);
  const closeDialog = useCallback(() => setDialogOpen(false), []);

  const registerRefreshHandler = useCallback((fn: () => void) => {
    refreshRef.current = fn;
  }, []);
  const triggerRefresh = useCallback(() => {
    refreshRef.current?.();
  }, []);

  return (
    <PendingPaymentContext.Provider value={{
      pendingPayment, setPendingPayment,
      isDialogOpen, openDialog, closeDialog,
      registerRefreshHandler, triggerRefresh,
    }}>
      {children}
    </PendingPaymentContext.Provider>
  );
}

export function usePendingPayment() {
  const ctx = useContext(PendingPaymentContext);
  if (!ctx) throw new Error("usePendingPayment must be used within PendingPaymentProvider");
  return ctx;
}
