# TwinBid API Contract

Контракт между фронтом кабинета и бэкендом. Фронт хранит JWT в `localStorage` и
шлёт его в заголовке `Authorization: Bearer <token>`. Все JSON-ответы — UTF-8.

> Источник данных:
> - **Postgres** — всё, кроме статистики откруток.
> - **ClickHouse** — любая статистика (impressions, clicks, spent, CTR, разрезы).
> - **S3** — файлы креативов (presigned URL).

Base URL фронта берётся из `VITE_API_BASE_URL`. Все ручки начинаются с `/api`.

---

## 0. Общие правила

- Ошибки: `{ "error": { "code": "string", "message": "string", "fields"?: { [k]: string } } }` со статусами 4xx/5xx.
- Пагинация: query `?limit=50&offset=0`, ответ `{ items: [...], total: number }`.
- Время: ISO 8601 в UTC (`2025-04-22T10:00:00Z`), даты — `YYYY-MM-DD`.
- HASHMAP-таргетинги (country/language/device_type/os/browser/site_id/ip):
  объект `{ "<value>": 1 | 0 }`. `1` = whitelist, `0` = blacklist.
  Пустой объект `{}` = таргетинг не применён.

---

## 1. Auth (Postgres `users`)

### POST `/api/auth/signup`
Body: `{ email, password, full_name?, manager_telegram }` (manager_telegram приходит с фронта как константа, по умолчанию `"GregTwinbid"`).
Resp 201: `{ access_token, refresh_token, user: User }`

### POST `/api/auth/login`
Body: `{ email, password }`.
Resp 200: `{ access_token, refresh_token, user: User }`

### POST `/api/auth/refresh`
Body: `{ refresh_token }`. Resp 200: `{ access_token, refresh_token }`

### POST `/api/auth/logout`
Auth required. Resp 204.

### `User` schema
```json
{
  "login": "user@example.com",
  "mail": "user@example.com",
  "name": "Ivan",
  "telegram": "@ivan",
  "manager_telegram": "GregTwinbid",
  "balance": 0,
  "timezone": "utc_3",
  "email_notifications": true,
  "campaign_status_notifications": true,
  "low_balance_notifications": true,
  "campaign_balanse_notifications": true,
  "balance_treshold": 100
}
```

---

## 2. Profile (Postgres `users`)

### GET `/api/profile` → `User`
### PATCH `/api/profile`
Body: any subset of `{ name, telegram, timezone, email_notifications, campaign_status_notifications, low_balance_notifications, campaign_balanse_notifications, balance_treshold }`.
Resp: `User`.

> Поле `manager_telegram` не редактируется через фронт — только админом со стороны бэка.

---

## 3. Campaigns (Postgres `campaigns`)

### `Campaign` schema (соответствует таблице)
```json
{
  "campaing_id": "uuid",
  "user_id": "uuid",
  "campaign_name": "string",
  "format_type": "banner | popunder | native | push",
  "brand_name": "string?",
  "h": 250, "w": 300,
  "status": "active | paused | draft | completed | moderation",
  "traffic_type": "mainstream | adult | mixed",
  "vertical": ["Dating", "Nutra"],
  "pricing_model": "cpm | cpc",
  "base_price_cpm": 0.05,
  "base_price_cpc": 0.0001,
  "evenness_by_slot_mode": false,
  "goal_total_dollars": 1000,
  "cum_done_dollars": 0,
  "start_ts": "2025-04-22T00:00:00Z",
  "end_ts": "2025-05-22T23:59:59Z",
  "active_intervals": [["mon,1","thu,2"], ["wed,3","fri,5"]],
  "country": { "US": 1, "RU": 1 },
  "language": {},
  "device_type": { "mobile": 1 },
  "os": {},
  "browser": {},
  "site_id": {},
  "ip": {}
}
```

> Если расписание выключено — `active_intervals = [["mon,0","sun,23"]]`.
> `cum_done_dollars` приходит ТОЛЬКО с бэка (не пишется фронтом).

### GET `/api/campaigns?status=&limit=&offset=` → `{ items: Campaign[], total }`
### GET `/api/campaigns/:id` → `Campaign`
### POST `/api/campaigns` (без `cum_done_dollars`, без `campaing_id`) → `Campaign`
### PATCH `/api/campaigns/:id` → `Campaign`
### DELETE `/api/campaigns/:id` → 204
### POST `/api/campaigns/:id/status` body `{ status }` → `Campaign`

---

## 4. Creatives (Postgres `pop_creatives` / `ban_creatives` / `ipp_creatives` / `nat_creatives`)

Тип креатива выбирается по `format_type` кампании:
- `popunder` → `pop_creatives`
- `banner` → `ban_creatives`
- `push` → `ipp_creatives` (in-page push)
- `native` → `nat_creatives`

### Общая `Creative` форма (ответ бэка)
```json
{
  "id": "uuid",
  "campaign_id": "uuid",
  "creative_name": "string",
  "link": "https://target.example",
  "trackers_macros": { "{CLICK_ID}": 1, "{ZONE_ID}": 0 },

  // banner only:
  "w": 300, "h": 250,
  // banner / push / native (заполняются БЭКОМ после загрузки файла):
  "name": "banner_300x250.png",
  "presigned_s3_url": "https://s3.amazonaws.com/...&X-Amz-Signature=...",
  // push / native:
  "title": "string",
  "description": "string"
}
```

> Поле `name` — это имя файла загруженной картинки. Его выставляет бэк при загрузке; фронт отображает его в кабинете как подпись к картинке.
> `presigned_s3_url` — временный signed URL для чтения, который бэк добавляет в ответы GET. Фронт использует его в `<img src>`.
> Фронт никогда не пишет ни `name`, ни `presigned_s3_url`. Никаких `s3_file_path` / `file_format` в контракте больше нет.

### GET `/api/campaigns/:id/creatives` → `Creative[]` (с `presigned_s3_url`, `name`)
Внутренний клиентский метод фронта, который вызывает эту ручку, называется **`readCreatives`**.

### POST `/api/campaigns/:id/creatives`
**Multipart form-data** (фронт всегда шлёт multipart, даже если файла нет):
- JSON-поля: `creative_name`, `link`, `trackers_macros`, `w?`, `h?`, `title?`, `description?`;
- `file` *(опционально)* — бинарь картинки;
- `filename` *(опционально, обязательно если есть `file`)* — имя файла.

Бэк сам кладёт файл в S3 **и записывает имя файла в поле `name` строки креатива.** Resp: `Creative` (с `presigned_s3_url` и `name`, если файл был).

### PATCH `/api/creatives/:id`
То же самое: multipart form-data с любым подмножеством JSON-полей и опциональными `file` + `filename`. Если `file` пришёл — бэк перезаписывает картинку и обновляет `name`.

### DELETE `/api/creatives/:id` → 204

---

## 5. Transactions & promo (Postgres `user_transactions`, `promocodes`)

### `UserTransaction`
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "transaction_time": "iso",
  "transaction_id": "string",
  "payment_method": "usdt_trc20 | usdt_erc20 | ...",
  "bonus_amount": 25,
  "promocode_id": "uuid?",
  "transaction_hash": "string?",
  "deposit_amount": 100,
  "total_balance_increase": 125,
  "status": "draft | pending | approved | rejected | cancelled",
  "currency": "usdt",
  "created_at": "iso",
  "updated_at": "iso"
}
```

### Поток пополнения
1. `POST /api/transactions` — создаёт транзакцию. Фронт присылает **все** поля `UserTransaction`, которые может посчитать сам (`user_id`, `transaction_time`, `transaction_id`, `payment_method`, `bonus_amount`, `promocode_id`, `transaction_hash`, `deposit_amount`, `total_balance_increase`, `status`, `currency`). Бэк присваивает только PK `id` и проставляет `created_at` / `updated_at`:
   ```json
   {
     "user_id": "uuid",
     "transaction_time": "iso",
     "transaction_id": "string",
     "payment_method": "usdt_trc20",
     "bonus_amount": 25,
     "promocode_id": "uuid?",
     "transaction_hash": "string?",
     "deposit_amount": 100,
     "total_balance_increase": 125,
     "status": "draft | pending",
     "currency": "usdt"
   }
   ```
   Resp: `UserTransaction`.
2. `PATCH /api/transactions/:id` — частичное обновление (например `{ transaction_hash, status: "pending" }`). Resp: `UserTransaction`.
3. `POST /api/transactions/:id/cancel` → `cancelled`. Resp: `UserTransaction`.
4. `GET /api/transactions?status=&limit=&offset=` → история.

> Внутренние клиентские методы фронта: `listTransactions`, `createTransaction`, `patchTransaction`, `cancelTransaction`.

### Promo
- GET `/api/promocodes/:code` → `{ id, promocode_text, bonus_percent, usage_count, usage_limit, valid_from, valid_to }`.
  Бэк проверяет: код активен, не достигнут `usage_limit`, не использован этим юзером ранее (через `user_transactions.promocode_id`).
  Если нельзя — 400 `{ error: { code: "PROMO_LIMIT" | "PROMO_USED" | "PROMO_EXPIRED", message } }`.

---

## 6. Notifications (Postgres `notifications`)

### Schema
```json
{
  "id": "uuid",
  "user_id": "uuid",
  "transaction_id": "uuid?",
  "campaign_id": "uuid?",
  "deposit_amount": 100,
  "status": "active | inactive",
  "text": "string",
  "type": "incomplete_topup | low_balance | campaign_status | other"
}
```

### GET `/api/notifications?status=active` → `Notification[]`
### POST `/api/notifications` body `{ type, text, transaction_id?, campaign_id?, deposit_amount? }` → `Notification` (создание с фронта, например для незавершённой транзакции).
### PATCH `/api/notifications/:id` body `{ status }` → `Notification` (отметка как `inactive`).

> Уведомление о незавершённой транзакции переводится в `inactive` ТОЛЬКО при отмене транзакции (`POST /api/transactions/:id/cancel`) или при её успешном завершении (статус `pending`/`approved`).

---

## 7. Statistics — ClickHouse

> Все ручки ниже — read-only выборки из ClickHouse. Ни одна не пишет в Postgres.

### POST `/api/stats/query`
Универсальная ручка для страницы Статистика.
Body:
```json
{
  "from": "2025-04-15", "to": "2025-04-22",
  "campaign_ids": ["uuid"],
  "group_by": ["date","campaign","country","format","creative","os","browser","device_type","language"],
  "filters": { "country": ["US","RU"], "device_type": ["mobile"] }
}
```
Resp:
```json
{
  "rows": [
    { "date": "2025-04-22", "campaign_id": "uuid", "country": "US",
      "impressions": 12345, "clicks": 67, "spent": 12.34, "ctr": 0.005425 }
  ],
  "totals": { "impressions": 12345, "clicks": 67, "spent": 12.34, "ctr": 0.005425 }
}
```

### GET `/api/stats/campaign/:id/summary?from=&to=` → `{ impressions, clicks, spent, ctr }`
Используется в строках списка кампаний (`DashboardCampaigns`) и в обзоре (`DashboardOverview`/`StatsCards`).

### GET `/api/stats/overview?from=&to=` → `{ impressions, clicks, spent, ctr }`
Агрегат по всем кампаниям пользователя (для `StatsCards` на overview).

---

## 8. Маппинг фронт-экранов на ручки

| Экран | Источник |
|---|---|
| `/dashboard` overview cards | `GET /api/stats/overview` (ClickHouse) + `GET /api/profile` |
| Список кампаний на overview/campaigns | `GET /api/campaigns` (Postgres) + `GET /api/stats/campaign/:id/summary` (ClickHouse) на каждую строку (или батч `POST /api/stats/query` с `group_by:["campaign"]`) |
| `/dashboard/statistics` | `POST /api/stats/query` (ClickHouse) |
| `/dashboard/balance` баланс/история | `GET /api/profile`, `GET /api/transactions`, `POST /api/transactions`, `PATCH /api/transactions/:id`, `POST /api/transactions/:id/cancel` |
| Создание/редактирование кампании | `POST/PATCH /api/campaigns`, `POST /api/creatives/upload-url`, CRUD `/api/creatives` |
| Уведомления (колокольчик) | `GET/POST/PATCH /api/notifications` |
| Настройки | `GET/PATCH /api/profile` |
| Auth | `/api/auth/*` |

---

## 9. Фронт-флаги

- `VITE_API_BASE_URL` — base URL бэка (например `https://api.twinbid.com`).
- `VITE_USE_MOCK` — `"true"` использует mock-провайдер с фикстурами в `src/api/mocks/*`. По умолчанию `true` пока бэк не готов.
