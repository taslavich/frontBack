
-- Profiles table
CREATE TABLE public.profiles (
  id UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
  email TEXT,
  full_name TEXT,
  telegram TEXT,
  timezone TEXT DEFAULT 'utc_3',
  balance NUMERIC NOT NULL DEFAULT 0,
  notify_campaign_status BOOLEAN NOT NULL DEFAULT true,
  notify_low_balance BOOLEAN NOT NULL DEFAULT true,
  balance_threshold NUMERIC NOT NULL DEFAULT 100,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own profile" ON public.profiles FOR SELECT TO authenticated USING (auth.uid() = id);
CREATE POLICY "Users can update own profile" ON public.profiles FOR UPDATE TO authenticated USING (auth.uid() = id);
CREATE POLICY "Users can insert own profile" ON public.profiles FOR INSERT TO authenticated WITH CHECK (auth.uid() = id);

-- Auto-create profile on signup
CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
  INSERT INTO public.profiles (id, email, full_name)
  VALUES (NEW.id, NEW.email, COALESCE(NEW.raw_user_meta_data->>'full_name', ''));
  RETURN NEW;
END;
$$;

CREATE TRIGGER on_auth_user_created
  AFTER INSERT ON auth.users
  FOR EACH ROW EXECUTE FUNCTION public.handle_new_user();

-- Campaign status enum
CREATE TYPE public.campaign_status AS ENUM ('active', 'paused', 'draft', 'completed', 'moderation');
CREATE TYPE public.pricing_model AS ENUM ('cpm', 'cpc');
CREATE TYPE public.traffic_quality AS ENUM ('common', 'high', 'ultra');
CREATE TYPE public.traffic_type AS ENUM ('mainstream', 'adult', 'mixed');

-- Campaigns table
CREATE TABLE public.campaigns (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  status campaign_status NOT NULL DEFAULT 'draft',
  format TEXT NOT NULL,
  format_key TEXT NOT NULL,
  budget NUMERIC NOT NULL DEFAULT 0,
  daily_budget NUMERIC,
  spent NUMERIC NOT NULL DEFAULT 0,
  impressions BIGINT NOT NULL DEFAULT 0,
  clicks BIGINT NOT NULL DEFAULT 0,
  ctr NUMERIC NOT NULL DEFAULT 0,
  pricing_model pricing_model NOT NULL DEFAULT 'cpm',
  price_value NUMERIC NOT NULL DEFAULT 0,
  traffic_quality traffic_quality NOT NULL DEFAULT 'common',
  traffic_type traffic_type NOT NULL DEFAULT 'mainstream',
  verticals TEXT[] NOT NULL DEFAULT '{}',
  start_date DATE,
  end_date DATE,
  targeting JSONB NOT NULL DEFAULT '{}',
  even_spend BOOLEAN NOT NULL DEFAULT false,
  banner_size TEXT,
  brand_name TEXT,
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.campaigns ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own campaigns" ON public.campaigns FOR SELECT TO authenticated USING (auth.uid() = user_id);
CREATE POLICY "Users can insert own campaigns" ON public.campaigns FOR INSERT TO authenticated WITH CHECK (auth.uid() = user_id);
CREATE POLICY "Users can update own campaigns" ON public.campaigns FOR UPDATE TO authenticated USING (auth.uid() = user_id);
CREATE POLICY "Users can delete own campaigns" ON public.campaigns FOR DELETE TO authenticated USING (auth.uid() = user_id);

-- Creatives table
CREATE TABLE public.creatives (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id UUID NOT NULL REFERENCES public.campaigns(id) ON DELETE CASCADE,
  name TEXT,
  url TEXT NOT NULL,
  image_url TEXT,
  image_file_name TEXT,
  title TEXT,
  description TEXT,
  storage_path TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.creatives ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own creatives" ON public.creatives FOR SELECT TO authenticated
  USING (EXISTS (SELECT 1 FROM public.campaigns WHERE campaigns.id = creatives.campaign_id AND campaigns.user_id = auth.uid()));
CREATE POLICY "Users can insert own creatives" ON public.creatives FOR INSERT TO authenticated
  WITH CHECK (EXISTS (SELECT 1 FROM public.campaigns WHERE campaigns.id = creatives.campaign_id AND campaigns.user_id = auth.uid()));
CREATE POLICY "Users can update own creatives" ON public.creatives FOR UPDATE TO authenticated
  USING (EXISTS (SELECT 1 FROM public.campaigns WHERE campaigns.id = creatives.campaign_id AND campaigns.user_id = auth.uid()));
CREATE POLICY "Users can delete own creatives" ON public.creatives FOR DELETE TO authenticated
  USING (EXISTS (SELECT 1 FROM public.campaigns WHERE campaigns.id = creatives.campaign_id AND campaigns.user_id = auth.uid()));

-- Topup requests
CREATE TYPE public.topup_status AS ENUM ('pending', 'approved', 'rejected');

CREATE TABLE public.topup_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  amount NUMERIC NOT NULL,
  payment_method TEXT NOT NULL,
  tx_hash TEXT,
  promo_code TEXT,
  bonus_percent NUMERIC DEFAULT 0,
  status topup_status NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ,
  processed_by UUID REFERENCES auth.users(id)
);

ALTER TABLE public.topup_requests ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own topup requests" ON public.topup_requests FOR SELECT TO authenticated USING (auth.uid() = user_id);
CREATE POLICY "Users can insert own topup requests" ON public.topup_requests FOR INSERT TO authenticated WITH CHECK (auth.uid() = user_id);

-- Transactions history
CREATE TYPE public.transaction_type AS ENUM ('topup', 'spend', 'promo_bonus', 'refund');

CREATE TABLE public.transactions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  type transaction_type NOT NULL,
  amount NUMERIC NOT NULL,
  description TEXT,
  reference_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.transactions ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own transactions" ON public.transactions FOR SELECT TO authenticated USING (auth.uid() = user_id);

-- Promo codes
CREATE TABLE public.promo_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code TEXT NOT NULL UNIQUE,
  bonus_percent NUMERIC NOT NULL DEFAULT 0,
  is_active BOOLEAN NOT NULL DEFAULT true,
  max_uses INTEGER,
  current_uses INTEGER NOT NULL DEFAULT 0,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE public.promo_codes ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Anyone can read active promo codes" ON public.promo_codes FOR SELECT TO authenticated USING (is_active = true);

-- Promo code usage
CREATE TABLE public.promo_usage (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  promo_code_id UUID NOT NULL REFERENCES public.promo_codes(id),
  topup_request_id UUID REFERENCES public.topup_requests(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, promo_code_id)
);

ALTER TABLE public.promo_usage ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Users can view own promo usage" ON public.promo_usage FOR SELECT TO authenticated USING (auth.uid() = user_id);
CREATE POLICY "Users can insert own promo usage" ON public.promo_usage FOR INSERT TO authenticated WITH CHECK (auth.uid() = user_id);

-- Storage bucket for campaign creatives
INSERT INTO storage.buckets (id, name, public) VALUES ('campaign-creatives', 'campaign-creatives', true);

-- Storage RLS
CREATE POLICY "Users can upload creatives" ON storage.objects FOR INSERT TO authenticated
  WITH CHECK (bucket_id = 'campaign-creatives' AND (storage.foldername(name))[1] = auth.uid()::text);
CREATE POLICY "Users can view own creatives" ON storage.objects FOR SELECT TO authenticated
  USING (bucket_id = 'campaign-creatives' AND (storage.foldername(name))[1] = auth.uid()::text);
CREATE POLICY "Users can delete own creatives" ON storage.objects FOR DELETE TO authenticated
  USING (bucket_id = 'campaign-creatives' AND (storage.foldername(name))[1] = auth.uid()::text);
CREATE POLICY "Public can view creatives" ON storage.objects FOR SELECT TO anon
  USING (bucket_id = 'campaign-creatives');

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$;

CREATE TRIGGER update_profiles_updated_at BEFORE UPDATE ON public.profiles FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
CREATE TRIGGER update_campaigns_updated_at BEFORE UPDATE ON public.campaigns FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
