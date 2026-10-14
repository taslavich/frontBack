# Развёртывание

1. Применить только REVISED patch к чистой базе; команды см. README_APPLY.md.
2. CH Loader выполняет существующий CreateDB с новыми idempotent ALTER и
publisher views; развернуть DDL и новые Kafka/CH readers до включения pub.
Не менять TTL, spend/recovery/auction алгоритмы. Миграция требует существующего
окна обслуживания, так как CreateDB пересоздаёт refreshable views.
3. Backend: `PUBLISHER_ENABLED=true`; Redis конфигурации должен совпадать с pub.
SQL schema и восстановление/ready guards — существующая процедура publisher sync.
4. Зарегистрировать два разных feed token в обычных POP ADULT/MAINSTREAM maps,
указанных `SSP_FEEDS_POP_ADULT_FILE_PATH` / `SSP_FEEDS_POP_MAINSTREAM_FILE_PATH`.
Согласовать ssp_domain с обычными filters/link maps.
5. Во ВСЕХ SSP Adapter: `PUB_UUID_FEEDS=<adult token>,<mainstream token>` и
`PUB_INTERNAL_SECRET=<тот же случайный secret >=32 символов, что у pub>`.
Никаких новых ENV для ADM Adapter/Router/Bid Engine/Orchestrator/ADV.
6. Pub ENV — cmd/pub/pub.env.example. `REDIS_SHARD_ADDRS` (порядок важен),
DB и ready set точь-в-точь совпадают с действующими ORTB writer/loader.
Это standalone sharded ORTB store, не `PUBLISHER_REDIS_CLUSTER` configuration store.
Redis должен иметь noeviction и действующую durability policy; pub проверяет
noeviction и configuration, и ORTB shard stores.
7. Существующий Nginx обычно передаёт custom request headers. Если текущая
конфигурация их стирает, добавить ВНУТРИ действующих POP locations только:

```nginx
proxy_set_header X-TwinBid-RTB-UUID $http_x_twinbid_rtb_uuid;
proxy_set_header X-TwinBid-RTB-UUID-Time $http_x_twinbid_rtb_uuid_time;
proxy_set_header X-TwinBid-RTB-UUID-Signature $http_x_twinbid_rtb_uuid_signature;
```

Не менять proxy_pass/upstreams/ADM/BURL. HMAC+single-use claim обязательны;
Host/ssp_domain/feed сам по себе недостаточен. Публичный request JSON не содержит
publisher_context; все publisher-поля prewrite делает только pub.
8. `PUB_CONFIRMED_OPEN_ENABLED=true`; проверить оригинальные POP ADM callbacks
в браузере. Bridge подтверждает своё открытие, а не загрузку сайта рекламодателя.
9. Прочитать MIGRATIONS.md: shared agg_stats + paused writers. Backend автоматически
синхронизирует POP cumulative; внешний financial endpoint отключён (409).
PUBLISHER_REVENUE_SYNC_INTERVAL=30s / PUBLISHER_REVENUE_SYNC_TIMEOUT=30s.
Frontend должен отправлять обязательный traffic_type adult/mainstream.
10. Старый `requests.db` — outbox ПРЕЖНЕГО патча, его сообщения нельзя автоматически
переотправлять вместе с новым потоком. Сохранить копию и сверить pending UUID с уже
загруженной статистикой. Новый error outbox использует отдельный `errors-v2.db`.
