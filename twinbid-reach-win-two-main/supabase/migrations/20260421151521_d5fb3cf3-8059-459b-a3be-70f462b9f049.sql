ALTER TABLE public.profiles
ADD COLUMN IF NOT EXISTS manager_telegram text NOT NULL DEFAULT 'GregTwinbid';

UPDATE public.profiles SET manager_telegram = 'GregTwinbid' WHERE manager_telegram IS NULL OR manager_telegram = '';