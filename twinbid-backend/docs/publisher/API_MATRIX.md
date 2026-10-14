# API matrix



Использовать существующий JWT-клиент кабинета с Bearer auth/refresh и распаковкой `{success,errorMsg,data}`. Конкретный файл фронтенда не проверен: его исходники не приложены. Не создавать второй login, токены или auth-provider. `/api/profile` и `/api/auth/...` остаются прежними. Owner определяется только JWT `sub`; в publisher-запросах не передавать `user_id`.

| Метод и путь | JSON / query | Ответ в `data` |
|---|---|---|
| GET `/api/publisher/ad-codes` | `limit=1..500`, `offset=0..1000000` | `{items:Code[],limit,offset,public_script_url}` |
| POST `/api/publisher/ad-codes` | `{name,format:"popunder",traffic_type,device,trigger_click,frequency_minutes,max_per_session}` | `Code` |
| GET `/api/publisher/ad-codes/{id}` | внутренний UUID Code | `Code`, чужой ID → 404 |
| PATCH `/api/publisher/ad-codes/{id}` | изменяемые поля + `version` прочитанного Code | новая версия `Code`; старый version → 409 |
| DELETE `/api/publisher/ad-codes/{id}` | — | `{archived:true}`; история сохраняется |
| POST `/api/publisher/ad-codes/{id}/duplicate` | — | новый `Code`, новый ID/CODE, без старых Zone/earnings |
| GET `/api/publisher/ad-codes/{id}/diagnostics` | — | `{ad_code,events:Diagnostic[],runtime_ready}`; до 50 последних запросов |
| GET `/api/publisher/sites` | pagination | `InventoryReport`; lifetime discovery, только собственные источники |
| GET `/api/publisher/zones` | pagination, optional `site_id`, `ad_code_id` | `InventoryReport`, lifetime Zone discovery |
| GET `/api/publisher/overview` | — | `{report,balance,from,to}`: 7 дней и текущий ledger balance |
| POST `/api/publisher/stats/query` | `StatsRequest` ниже | `Report` |
| GET `/api/publisher/balance` | — | `{balance,reserved,available_for_payout,cumulative_earned,paid_out,revenue_status}` — decimal strings |
| GET `/api/publisher/payouts` | pagination | `{items:Payout[],limit,offset}` |
| POST `/api/publisher/payouts` | `{amount:"75.00",method:"usdt_trc20"|"usdt_erc20",wallet,request_id:UUID}` | `Payout`; retry сохраняет тот же request_id |
| POST `/api/publisher/payouts/{id}/cancel` | — | 403: по принятому решению отменяет только оператор |
| GET `/api/publisher/settings` | — | `{codeAlerts,payoutMethod,wallet}` |
| PATCH `/api/publisher/settings` | полный объект этих трёх настроек | сохранённые настройки |
| GET `/api/publisher/sync-health` | — | собственный backlog, ready/state; epoch только admin |

`Code`:

```ts
interface PublisherCodeDTO {
  ad_code_id: string; user_id: string; short_code: string; name: string;
  format: "popunder"; traffic_type: "adult" | "mainstream";
  device: "all" | "desktop" | "mobile"; is_active: boolean; archived: boolean;
  trigger_click: 1 | 2; frequency_minutes: number; max_per_session: number | null;
  version: number; synced_version: number; created_at: string;
}
```

`max_per_session=null` означает unlimited, не `0`. `frequency_minutes` от 0.1 до 10080; cap 1..100000. ID/owner/CODE/version/epoch при создании клиент не задаёт. Backend проверяет эти ограничения независимо от UI.

```ts
interface StatsRequest {
  from: string; to: string; // UTC ISO/RFC3339, [from,to), <=366 дней
  group_by: "date"|"hour"|"country"|"device"|"os"|"browser"|"site"|"zone"|"ad_code";
  timezone?: string; // отсутствует → timezone профиля
  site_id?: string; zone_id?: string; ad_code_id?: string;
  countries?: string[]; devices?: string[]; oses?: string[]; browsers?: string[];
  limit?: number; offset?: number;
}
interface PublisherMetrics {
  requests: number; errors: number; fills: number; no_fills: number; impressions: number; clicks: number;
  ctr: number; revenue_usd: string;
}
interface PublisherReport {
  effective_from: string; effective_to: string; time_resolution: "1h_utc";
  rows: Array<PublisherMetrics & {
    bucket: string; site_id: string; zone_id: string; ad_code_id: string;
    hostname: string; last_request_at: string|null;
  }>;
  totals: PublisherMetrics; limit: number; offset: number;
  revenue_status: "agg_stats_pending"|"agg_stats_ready"|"agg_stats_stale"|"agg_stats_unavailable";
}
interface PublisherInventoryReport {
 rows: PublisherReport["rows"]; limit: number; offset: number;
 metric_period: "last_6_months"; revenue_period: "lifetime";
}
interface PublisherPayoutDTO {
  id: string; amount: string; method: "usdt_trc20"|"usdt_erc20"; wallet: string;
  status: "pending"|"paid"|"canceled"; created_at: string; transfer_reference: string;
}
```

Денежные значения — decimal strings; CTR/totals брать с сервера. MVP format —
Popunder. Publisher inventory хранит lifetime discovery ID/hostname/min/max.
Request/error/no-fill detail имеет TTL 6 месяцев, wide ORTB — 1 час. Фактические
счётчики и revenue из agg_stats не имеют этого TTL. Inventory activity показывает
последние 6 месяцев, lifetime revenue — SUM агрегата за всю историю. No-events
status означает отсутствие данных в выбранном периоде. PG ledger/balance/reserve/
payout history используются для бухгалтерского состояния, не второй аналитический
денежной агрегации. Source fields и время группировки описаны ниже.

Пример запроса:

```ts
import { http } from "@/api/http";
const stats = await http<PublisherReport>("/api/publisher/stats/query", {
  method: "POST",
  body: {
    from: "2026-10-05T00:00:00Z", to: "2026-10-12T00:00:00Z",
    group_by: "hour", timezone: "Europe/Moscow", limit: 100, offset: 0,
    countries: ["US"], devices: ["mobile"]
  }
});
```

Ошибки: 400 — JSON/поля/период/настройки; 401 — JWT; 403 — права/операторская отмена; 404 — неизвестный или чужой Code; 409 — конфликт version, недостаточный available, изменённый payout idempotency payload или финализированная выплата; 503 — зависимость/статистика недоступна. Не превращать ошибку запроса в пустые успешные totals и не откатывать UI на демо.



## Требования фронтенду

В Code форме выбор adult/mainstream обязателен, без третьего варианта и без
предвыбранного значения. POST и PATCH должны явно отправлять traffic_type;
отсутствие/null/неподдерживаемая строка → 400. При PATCH прочитать и отправить
текущий допустимый тип вместе с version. Duplicate сохраняет тип исходного Code.
Source revenue статусы теперь agg_stats_pending/ready/stale/unavailable; отражают
состояние авто sync. Не считать баланс по winners/request fills. Balance дополнен
cumulative_earned, paid_out и revenue_status. Накопленный доход, текущий остаток,
резерв и выплаченные суммы показывать отдельно. Money всегда decimal strings.
Reports money/impressions/clicks — shared aggregate; requests/errors — projection.
Report effective_from/effective_to/time_resolution='1h_utc' сообщают часовые границы.
Lifetime inventory revenue читается из агрегата; последняя 6-month активность —
только выбранный период, не обещание TTL у agg_stats.

Источник фронтенда не приложен, UI файлы не исправлялись. Runtime загружается из pub;
не вызывать /confirm для page click или денежных событий.

## Внешняя интеграция бота

POST /api/publisher/payouts/{id}/decision использует Bearer JWT с
sub=BOT_ADMIN_USER_ID, payload {status:'paid'|'canceled',transfer_reference:'...'}.
Для paid нужен подтверждённый reference; uncovered reserve даёт 409. Owner GET
payouts и Code diagnostics остаются прежними. Бот не должен вызывать финансовый
endpoint: он всегда отвечает 409, POP auto sync — единственный producer.
Bot callbacks/buttons не изменены: исходников нет.
