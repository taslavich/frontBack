# Memory: index.md
Updated: now

# Project Memory

## Core
TwinBid aggregator. Supabase (PostgreSQL, Auth, Storage) + Vercel. Enforce RLS.
Dark Tech style: Mint/coral on dark bg. RU/EN localization.
Global page scroll only (no nested fixed-height scrolling).
All table columns are left-aligned, including numbers.
Number inputs support dot/comma; spinners are strictly disabled.
Contact emails open a copy modal (never use mailto: links).
.env must always include VITE_API_BASE_URL=https://twinbid.io and VITE_USE_MOCK=false.
src/api/http.ts fetch must be wrapped in try/catch throwing ApiError(0, ..., "NETWORK_ERROR").

## Memories
- [Auth UI](mem://features/authentication-ui) — Modal login/registration, optional name, no phone field
- [Layout patterns](mem://style/layout-patterns) — 3+2 grid, global scroll, left-aligned tables
- [Dashboard Overview](mem://features/dashboard/overview) — Shows full list of campaigns, search removed
- [Finances](mem://features/dashboard/finances) — USD via USDT (min $100), promo codes, strict topup blocks
- [Settings](mem://features/dashboard/settings) — Limited fields (name, email, TG, tz), 2FA/API removed
- [Notifications](mem://features/dashboard/notifications) — Bell header, persistent alerts for topups/balance
- [Landing Page](mem://features/landing-page) — Cashback tiers, 25% promo, smart scroll indicator
- [Statistics](mem://features/dashboard/statistics) — Empty by default, 7/30 presets, specific groupings
- [Navigation](mem://features/dashboard/navigation) — Header shows manager contact, logo to Overview
- [Architecture](mem://technical/architecture-and-state-management) — Contexts for global state (Campaign, Profile, Notification, Stats)
- [Backend Infrastructure](mem://technical/backend-infrastructure) — Profiles trigger, RLS, campaign-creatives bucket
- [Data Modeling](mem://technical/data-modeling) — Creative IDs as float {campaign_id}.{creative_num}
- [Ad Formats](mem://features/ad-formats) — Banner/Push/Popunder bid models, tracking macros
- [Campaigns](mem://features/dashboard/campaigns) — Traffic types, High/Ultra quality, targeting, 24/7 schedule
- [API env & http.ts](mem://preferences/api-env-and-http) — Required env vars and http.ts fetch try/catch
