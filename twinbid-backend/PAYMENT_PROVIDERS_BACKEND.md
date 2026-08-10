# Payment providers backend

The transactions API supports two invoice providers behind one top-up service:

- `passimpay`
- `cryptomus`

Provider-specific HTTP clients, request signatures and webhook signatures stay isolated.
Balance credit, promocode accounting, idempotency and reconciliation are shared.

## Creating an invoice

`POST /api/transactions` requires an explicit `provider` for invoice payments.
There is no implicit provider fallback.

PassimPay example:

```json
{
  "provider": "passimpay",
  "deposit_amount": 100,
  "currency": "USD"
}
```

Cryptomus example:

```json
{
  "provider": "cryptomus",
  "deposit_amount": 100,
  "currency": "USD"
}
```

If `provider` is missing, empty, or unsupported, the API returns `400 Bad Request`
and no local transaction/provider invoice is created.

The legacy manual wallet flow remains available only when explicitly selected:

```json
{
  "payment_channel": "static_wallet",
  "payment_method": "USDT TRC20",
  "deposit_amount": 100,
  "currency": "USD"
}
```

For invoice providers `payment_channel` can be omitted. The backend derives it from
`provider` (`passimpay_invoice` or `cryptomus_invoice`). If it is supplied, it must
match the selected provider.

## Webhooks

Webhook routes are deliberately separate:

```text
POST /api/webhooks/passimpay
POST /api/webhooks/cryptomus
```

This allows nginx to maintain a distinct IP allowlist for each provider. Backend
signature verification is still mandatory and is performed independently for each
provider.

A valid webhook is treated only as a notification. Before changing user balance,
the backend calls the corresponding provider status endpoint. Only the normalized
`paid` state can reach the idempotent credit path.

## Reconciliation

Each provider has its own ticker configuration, but both use the same reconciliation
service. Pending/create-unknown invoices are queried only within that provider's
`payment_channel`, preventing cross-provider order resolution.

## Cryptomus configuration

See `cryptomus.env.example`. The required credentials are:

- `CRYPTOMUS_MERCHANT_UUID`
- `CRYPTOMUS_PAYMENT_API_KEY`
- `CRYPTOMUS_WEBHOOK_URL`

The default Cryptomus invoice uses USD and `CRYPTOMUS_SUBTRACT_PERCENT=100`, so the
client bears the Cryptomus payment commission. Change it to `0` if the merchant
should bear the payment commission instead.
