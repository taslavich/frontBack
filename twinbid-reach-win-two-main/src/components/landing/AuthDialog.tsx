import { useState } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useNavigate } from "react-router-dom";
import { useLanguage } from "@/contexts/LanguageContext";
import { useAuth } from "@/contexts/AuthContext";
import { toast } from "sonner";

interface AuthDialogProps {
  trigger?: React.ReactNode;
  defaultTab?: "login" | "register";
}

export function AuthDialog({ trigger, defaultTab = "login" }: AuthDialogProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [loginEmail, setLoginEmail] = useState("");
  const [loginPassword, setLoginPassword] = useState("");
  const [regName, setRegName] = useState("");
  const [regEmail, setRegEmail] = useState("");
  const [regPassword, setRegPassword] = useState("");
  const [regConfirm, setRegConfirm] = useState("");
  const navigate = useNavigate();
  const { t } = useLanguage();
  const { signIn, signUp } = useAuth();

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    const { error } = await signIn(loginEmail, loginPassword);
    setLoading(false);
    if (error) {
      toast.error(error);
      return;
    }
    setOpen(false);
    navigate("/dashboard");
  };

  const handleRegister = async (e: React.FormEvent) => {
    e.preventDefault();
    if (regPassword !== regConfirm) {
      toast.error(t("auth.passwordMismatch") || "Passwords do not match");
      return;
    }
    if (regPassword.length < 6) {
      toast.error(t("auth.passwordTooShort") || "Password must be at least 6 characters");
      return;
    }
    setLoading(true);
    const { error } = await signUp(regEmail, regPassword, regName);
    setLoading(false);
    if (error) {
      toast.error(error);
      return;
    }
    toast.success(t("auth.checkEmail") || "Check your email to confirm your account");
    setOpen(false);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger || <Button>{t("nav.login")}</Button>}
      </DialogTrigger>
      <DialogContent className="sm:max-w-[425px] bg-card border-border">
        <DialogHeader>
          <DialogTitle className="text-2xl font-bold text-center">
            <span className="bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">TwinBid</span>
          </DialogTitle>
        </DialogHeader>
        <Tabs defaultValue={defaultTab} className="w-full">
          <TabsList className="grid w-full grid-cols-2 bg-muted">
            <TabsTrigger value="login">{t("auth.login")}</TabsTrigger>
            <TabsTrigger value="register">{t("auth.register")}</TabsTrigger>
          </TabsList>
          <TabsContent value="login" className="space-y-4 mt-4">
            <form onSubmit={handleLogin} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email-login">{t("auth.email")}</Label>
                <Input id="email-login" type="email" placeholder="your@email.com" className="bg-background border-border"
                  value={loginEmail} onChange={(e) => setLoginEmail(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password-login">{t("auth.password")}</Label>
                <Input id="password-login" type="password" placeholder="••••••••" className="bg-background border-border"
                  value={loginPassword} onChange={(e) => setLoginPassword(e.target.value)} required />
              </div>
              <Button type="submit" className="w-full bg-primary hover:bg-primary/90" disabled={loading}>
                {loading ? "..." : t("auth.loginBtn")}
              </Button>
            </form>
          </TabsContent>
          <TabsContent value="register" className="space-y-4 mt-4">
            <form onSubmit={handleRegister} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="name">{t("auth.name")}</Label>
                <Input id="name" placeholder="John Doe" className="bg-background border-border"
                  value={regName} onChange={(e) => setRegName(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="email-register">{t("auth.email")}</Label>
                <Input id="email-register" type="email" placeholder="your@email.com" className="bg-background border-border"
                  value={regEmail} onChange={(e) => setRegEmail(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password-register">{t("auth.password")}</Label>
                <Input id="password-register" type="password" placeholder="••••••••" className="bg-background border-border"
                  value={regPassword} onChange={(e) => setRegPassword(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password-confirm">{t("auth.confirmPassword")}</Label>
                <Input id="password-confirm" type="password" placeholder="••••••••" className="bg-background border-border"
                  value={regConfirm} onChange={(e) => setRegConfirm(e.target.value)} required />
              </div>
              <Button type="submit" className="w-full bg-accent hover:bg-accent/90 text-accent-foreground" disabled={loading}>
                {loading ? "..." : t("auth.registerBtn")}
              </Button>
            </form>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
