# PassimPay Invoice Link backend integration

This patch keeps the existing `static_wallet` top-up flow and adds a parallel
`passimpay_invoice` flow through the existing transactions API.

## Runtime configuration

Copy the values from `passimpay.env.example` into the API environment and set:

- `PASSIMPAY_PLATFORM_ID`
- `PASSIMPAY_API_KEY`

Configure this Notification URL in the PassimPay platform settings:

```text
https://api.twinbidexchange.com/api/webhooks/passimpay
```

The API key stays on the backend. The frontend never calls PassimPay directly.

## Payment channels

- Missing `payment_channel` is treated as `static_wallet` for backward compatibility.
- `static_wallet` keeps the existing transaction-hash submission and manual moderation flow.
- `passimpay_invoice` creates a PassimPay invoice and returns `payment_url` in the transaction object.

## API behavior

- `POST /api/transactions` creates both payment types.
- `PATCH /api/transactions/{id}` only accepts `transaction_hash` and only for `static_wallet`.
  Legacy extra fields may still be sent by the old frontend, but they no longer change financial data.
- `GET /api/transactions/{id}` and `GET /api/transactions` return local and provider statuses.
- `POST /api/webhooks/passimpay` is public and protected by PassimPay `x-signature`.
- The old bot callback URLs are preserved and require **both** a valid backend JWT and `X-Bot-Secret`:
  - `POST /api/transactions/{id}/approve_admin`
  - `POST /api/transactions/{id}/cancel_admin`
  The Telegram bot must send `Authorization: Bearer <access_token>` and `X-Bot-Secret` in the same request.
- The unsafe user self-approval endpoint is removed.

## Important financial behavior

- The generic deposit webhook is not treated as proof of full invoice payment.
  The backend checks `/v3/orderstatus` and credits only when the authoritative
  invoice status is `paid`.
- `waiting` and partial payments are saved but are not credited automatically.
- `error` marks an uncredited local invoice top-up as rejected.
- Repeated webhooks are idempotent because `credited_at` is written in the same
  database transaction as the user balance increase.
- A background reconciliation task checks unfinished invoices in case webhook
  delivery was missed.
- The requested deposit amount plus the server-calculated promo bonus is credited
  only once after full payment confirmation.

## Database migration

The existing startup migration automatically:

- adds PassimPay/provider columns to `user_transactions`;
- marks existing rows as `static_wallet`;
- backfills `credited_at` for already approved historical top-ups;
- creates the `payment_webhook_events` audit table;
- adds indexes and the payment-channel constraint.

No manual SQL execution is required when the application already runs
`db.InitDBAndMigrate` during startup.
