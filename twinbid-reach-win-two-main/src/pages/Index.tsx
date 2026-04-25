import { Header } from "@/components/landing/Header";
import { HeroSection } from "@/components/landing/HeroSection";
import { StartConditions } from "@/components/landing/StartConditions";
import { BenefitsSection } from "@/components/landing/BenefitsSection";
import { CashbackSection } from "@/components/landing/CashbackSection";
import { FormatsSection } from "@/components/landing/FormatsSection";
import { StepsSection } from "@/components/landing/StepsSection";
import { CTASection } from "@/components/landing/CTASection";
import { Footer } from "@/components/landing/Footer";

const Index = () => {
  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main>
        <HeroSection />
        <StartConditions />
        <BenefitsSection />
        <CashbackSection />
        <FormatsSection />
        <StepsSection />
        <CTASection />
      </main>
      <Footer />
    </div>
  );
};

export default Index;
