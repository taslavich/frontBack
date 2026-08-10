# PassimPay Invoice Link backend integration

This integration keeps the existing `static_wallet` flow and adds the parallel
`passimpay_invoice` flow through the existing transactions API.

## Required runtime configuration

Copy the values from `passimpay.env.example` into the API environment and set:

- `PASSIMPAY_PLATFORM_ID`
- `PASSIMPAY_API_KEY`
- `BOT_INTERNAL_SECRET`
- `BOT_ADMIN_USER_ID` — the `users.id` UUID of the service account used by the
  Telegram bot when it logs in and obtains its JWT.

Configure this Notification URL in the PassimPay platform settings:

```text
https://api.twinbidexchange.com/api/webhooks/passimpay
```

The API key stays on the backend. The frontend never calls the PassimPay API directly.

## Payment channels

- Invoice creation requires explicit `"provider": "passimpay"`.
- Missing/empty/unsupported `provider` returns `400 Bad Request` unless the request explicitly selects `payment_channel: "static_wallet"`.
- `static_wallet` keeps transaction-hash submission and manual moderation.
- `passimpay_invoice` is derived from `provider=passimpay` and returns `payment_url` in the local transaction.
- PassimPay invoices are denominated in `USD`; another `currency` value is rejected.
- `deposit_amount` must be positive and contain no more than two decimal places.

## API behavior

- `POST /api/transactions` creates both payment types.
- `PATCH /api/transactions/{id}` only accepts `transaction_hash` and only for
  `static_wallet`. Legacy extra fields are ignored and cannot change financial data.
- `GET /api/transactions/{id}` and `GET /api/transactions` return local and provider statuses.
- `POST /api/webhooks/passimpay` is public and protected by PassimPay `x-signature`.
- Bot callbacks require all of the following:
  - a valid backend JWT;
  - JWT `user_id` equal to `BOT_ADMIN_USER_ID`;
  - a matching `X-Bot-Secret` header.

  Callback URLs remain:

  - `POST /api/transactions/{id}/approve_admin`
  - `POST /api/transactions/{id}/cancel_admin`

- `PATCH /api/profile` explicitly rejects `balance`.
- The unsafe `/api/profile_admin` route is removed.
- User balance can only be increased by the internal, transactional top-up credit operation.

## Financial behavior

- The generic deposit callback is not used as proof of full invoice payment.
  The backend checks the invoice-status endpoint and credits only an authoritative `paid` state.
- `waiting`, including partial payment, is stored without crediting the user.
- `error` rejects an uncredited invoice and releases a reserved promocode slot.
- Balance increase, `credited_at`, and promocode accounting are committed in one SQL transaction.
- Repeated webhook or reconciliation processing cannot credit the same transaction twice.
- Promocode use is reserved atomically when a top-up is created, preventing concurrent
  requests from bypassing per-user or global usage limits.
- If invoice creation returns an uncertain network result, the local transaction is kept
  as `create_unknown` and reconciled by `orderId` instead of being incorrectly rejected.
- Reconciliation is batched, rate-limited, and delayed after provider errors.

## Database migration

The startup migration automatically:

- adds PassimPay/provider and reconciliation columns to `user_transactions`;
- marks historical rows as `static_wallet`;
- backfills `credited_at` for historical approved top-ups;
- normalizes the incorrect legacy promo flags once through `app_schema_migrations`;
- creates `payment_webhook_events`;
- adds the reconciliation and lookup indexes;
- preserves existing transactions and balances.

No manual SQL execution is required when the application already calls
`db.InitDBAndMigrate` during startup.
