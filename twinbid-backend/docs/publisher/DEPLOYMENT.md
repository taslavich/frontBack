# Развёртывание

1. Применить только REVISED patch к чистой базе; команды см. README_APPLY.md.
2. CH Loader выполняет существующий CreateDB с новыми idempotent ALTER и
publisher views; развернуть DDL и новые Kafka/CH readers до включения pub.
Не менять TTL, spend/recovery/auction алгоритмы. Миграция требует существующего
окна обслуживания, так как CreateDB пересоздаёт refreshable views.
3. Backend Publisher всегда включён: `PUBLISHER_ENABLED` больше не читается.
   Обязательны `PUBLISHER_PUBLIC_URL` (HTTPS), `PUBLISHER_REDIS_ADDRS`,
   `PUBLISHER_REDIS_DB` (явный номер выделенной свободной Redis DB).
   Можно использовать действующий Redis по адресу `buffer.twinbidexchange.com:7005`
   с отдельной проверенной DB (например 6, если она свободна); DB 5 занята
   advertiser POP recovery и запрещена для Publisher. Наличие пустого keyspace
   само по себе не доказывает, что DB никем не зарезервирована.
   `PUBLISHER_REDIS_CLUSTER=false` для отдельных логических DB; Redis Cluster
   поддерживает только DB 0. При старте backend проверяет доступность Publisher
   Redis через PING (timeout 3s), ошибка подключения останавливает запуск.
   Настройки DB, адреса, TLS и учётных данных должны совпадать с RTB Publisher
   Server; для изменения его DB с 0 требуется отдельный совместимый RTB-патч.
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
Frontend должен отправлять обязательный traffic_type adult/mainstream при POST;
при PATCH поле необязательно, отсутствие сохраняет существующий тип.
10. Старый `requests.db` — outbox ПРЕЖНЕГО патча, его сообщения нельзя автоматически
переотправлять вместе с новым потоком. Сохранить копию и сверить pending UUID с уже
загруженной статистикой. Новый error outbox использует отдельный `errors-v2.db`.
