TwinBid proj234 / FProj234 — финальный hardening Publisher Zone Diagnostics

Основа: только `proj234.zip`, `FProj234.zip`, `TwinBid_Frontend_Publishers_Zone_Diagnostics_FINAL(1).zip`. Патчи содержат новые изменения относительно этих архивов. Старые патчи повторно не применять. Проверено 21 октября 2026 года.

Внесены четыре доработки:

1. Диагностическая миграция проверяет engine и все 19 необходимых исходных колонок ORTB, их типы/доступность для INSERT, а также DEFAULT created_at: действующий Loader оставляет время вставки серверному now()/now64(). Недостающие поля добавляются через ADD COLUMN IF NOT EXISTS. Проверяются engine/key/колонки долговечного publisher_inventory, отсутствие TTL у реестра, определение его discovery MV и refreshable-тип уже существующей диагностической MV. Отсутствующие inventory/MV создаются по текущему DDL; доступные 65 минут ORTB подхватываются ограниченным backfill. При несовместимости команда возвращает понятную ошибку до переключения, полный CreateDB не вызывается. Существующие колонки/типы/DEFAULT не переписываются. При утраченных uuid/event_time/created_at и уже атрибутированных исторических строках миграция останавливается: безопасно восстановить такую историю из DEFAULT невозможно.
2. Inventory API сохраняет `metric_period: "last_6_months"` как совместимый устаревающий alias, теперь явно ограниченный `metric_period_scope: "impressions_clicks_ctr"`. Добавлены `request_metric_period: "last_7_days"`, `request_metric_source: "publisher_zone_hourly"`, `advertising_metric_source: "agg_stats"`. `revenue_period: "lifetime"` сохранён; строка с данными имеет `metrics_status: "mixed_periods"`. Окна SELECT для impressions/clicks/CTR остались шестимесячными, lifetime SELECT не изменён. Фактическая техническая история может быть короче семи дней после первого bootstrap/простоя; семь дней — предел хранения, не обещание восстановленной истории. DTO, Zod, fixtures, тесты и подпись видимых метрик согласованы.
3. Активные неархивированные коды по-прежнему получаются из PostgreSQL по владельцу JWT. API дополнительно возвращает `codes_without_zones` и отдельный `undiscovered_codes: [{ad_code_id,status:"no_zones_detected"}]`. Это коды без записей в долговечном inventory, не вымышленные зоны и не доказательство HTTP-ошибки. `active_zones`/`problem_zones` считают только настоящие обнаруженные зоны. Все ранее обнаруженные зоны сохраняются при отсутствии трафика. В кодовом диалоге добавлено пояснение с проверкой установки/наличия трафика, в обзоре — отдельная карточка. RU/EN/zh-CN и пагинация сохранены. Оба списка (зоны и undiscovered_codes) используют тот же limit/offset независимо; оба summary считаются глобально. Статусы зон остаются прежними: 200 ИЛИ 204 → active; смешанные ошибки успех не отменяют.
4. Исходный SELECT проверен на 12 и 36 млн ORTB-строк. На 36 млн обнаружен реальный MEMORY_LIMIT_EXCEEDED при лимите 512 MiB. Сохранены прежние GROUP BY/argMin и вся архитектура; per-query лимит диагностического SELECT подобран до 1 GiB по фактически измеренному пику. Лимиты двух потоков и spill после 128 MiB сохранены. Миграция обновляет QUERY уже существующей **только диагностической** MV через ALTER TABLE ... MODIFY QUERY, сохраняя REFRESH EVERY 5 MINUTE APPEND, target и данные. Старый провал refresh не блокирует исправление его запроса; новый refresh обязан завершиться успешно до остановки legacy writer. При ошибке обновления возобновляется ранее остановленная диагностическая MV.

Изменены production-файлы: RTB `publisher_zone_diagnostics.go` и новый `publisher_zone_preflight.go`; cabinet `internal/publisher/reports.go`, `zone_diagnostics.go`, `internal/stats/publisher_zones.go`; frontend `src/api/publisher.ts`, `publisherContract.ts`, `PublisherDiagnostics.tsx`, `PublisherInventoryTable.tsx`. Также изменены/добавлены целевые тесты и документация. Обзор использует существующий компонент карточек; его остальные функции не переписывались.

Финансовые исходники и DDL `fact_impressions`, `fact_clicks`, `agg_stats`, `traffic_volume_hourly`, Publisher POP revenue, /adm,/burl,/nurl, DSP/SSP routing, Simple/Complex, антифрод, RunRevenueSync, выплаты/ledger/баланс и рекламодательский UI не менялись. В production остаётся `VITE_USE_MOCK=false`. Старые requests/fills/no_fills/errors не возвращены в статистический интерфейс.

Проверки для этой версии:

- `go build ./...` в RTB и backend кабинета — успешно.
- Go race-тесты publisher/stats — успешно; целевые SQL-тесты RTB preflight/zone snapshots и cabinet zone diagnostics на настоящем локальном ClickHouse 25.8.18.1 — успешно. Проверены отсутствующие Publisher-поля/таблицы/MV, несовместимые типы/DEFAULT/TTL, повторная миграция, обновление ранее созданной MV, Kafka replay, повторные снимки, UTC, TTL, late events, неудачный refresh и сохранность legacy данных/финансового writer.
- Redis 7.0.15: целевые тесты Pub 200/204 и отказов 403/429/502/503 — успешно. Тесты kafka-loader проверяют HMGET Publisher-полей → protobuf; отдельный новый SQL-тест выполняет настоящий protobuf decode → insertBatchOrtb → inventory и refresh для 200/204/403/503. Не только DEFAULT: проверены записанные owner/code/site/zone/request/logical IDs и timestamps. Живой Kafka broker не поднимался; реальный production-трафик не отправлялся.
- Проверены owner-bound PG-запрос, исключение неактивных/архивированных кодов, отдельное состояние кода без зон, отсутствие подставных zone_id/site_id, глобальные counts и пагинация. PostgreSQL для этой проверки заменён существующим SQL test connector; ClickHouse-запросы выполнены реально. Полный live HTTP→PG→Kafka→CH стенд не поднимался.
- `npm ci` — успешно. Целевые frontend-тесты: **4 файла, 35 тестов успешно** (`--maxWorkers=1`). При параллельном benchmark один прежний длинный UI-тест превысил 15 секунд; после завершения нагрузки весь целевой набор прошёл последовательно без изменения таймаута.
- `npm run build`, `npx tsc --noEmit -p tsconfig.app.json`, ESLint изменённых TS/TSX — успешно. Сборка сохраняет существующие предупреждения о размере chunks/Browserslist.
- Playwright против production dist с изолированными HTTP fixtures: 9 сочетаний RU/EN/zh-CN × 320/375/1440 px. Проверены три независимых счётчика обзора, восемь колонок зон, уведомление кода без зон, отсутствие фиктивной строки/таблицы и горизонтального переполнения страницы. Ошибок JavaScript не обнаружено.
- Оба новых патча: git apply --check, реальное применение на чистых новых архивах и побайтное сравнение результата с проверенными файлами. Cabinet patch проверен из внешнего корня, с единственным префиксом twinbid-backend/. ZIP проверен по CRC и содержимому; все исходники и исходный .env.production сохранены.
- Полный `go test ./...` и повторное полное ревью RTB/кабинета **не выполнялись по условию задачи**. Ранее известные посторонние ошибки не исправлялись.

Команды применения — именно из указанных корней:

```bash
cd ~/gpt_test_3
git apply --check ~/Downloads/proj234_publisher_zone_diagnostics_FINAL.patch
git apply ~/Downloads/proj234_publisher_zone_diagnostics_FINAL.patch
go build ./...
go build -o bin/clickhouse-loader ./cmd/clickhouse-loader

cd ~/frontBack
git apply --check ~/Downloads/FProj234_publisher_zone_diagnostics_FINAL_FROM_FRONTBACK_ROOT.patch
git apply ~/Downloads/FProj234_publisher_zone_diagnostics_FINAL_FROM_FRONTBACK_ROOT.patch
go -C twinbid-backend build ./...
```

Для frontend распаковать `TwinBid_Frontend_Publishers_Zone_Diagnostics_FINAL_v2.zip` и из его корня выполнить:

```bash
npm ci
npm test -- --maxWorkers=1 src/test/publisher-zone-diagnostics.test.tsx src/test/publisher-inventory.test.tsx src/test/publisher-final-ui.test.tsx src/test/publisher-api.test.ts
npm run build
npx tsc --noEmit -p tsconfig.app.json
```

Порядок развёртывания:

1. Сохранить текущие binaries и SHOW CREATE для диагностических объектов/реестра. Проверить права миграционного пользователя на чтение system.tables/system.columns/system.view_refreshes, ADD COLUMN только ORTB, создание inventory/его MV/диагностики, ALTER QUERY диагностической MV и SYSTEM STOP/WAIT/START/REFRESH. Для прекращения legacy writer нужен DROP именно его VIEW. По замерам новый диагностический запрос может потребовать до 1 GiB; это локальный per-query лимит, глобальные настройки ClickHouse не меняются.
2. Из `~/gpt_test_3`, с существующим конфигурационным окружением Loader, запустить **один экземпляр**:

   ```bash
   ./bin/clickhouse-loader --publisher-zone-diagnostics-only
   ```

   Команда не запускает Kafka consumers и финансовый DDL. Если discovery MV отсутствовала, создаётся предусмотренный существующей архитектурой лёгкий min/max обработчик для реестра; тяжёлая диагностика HTTP остаётся Refreshable MV. Bootstrap доступного часа нужен только новой диагностике, backfill inventory ограничен 65 минутами. История старых строк без Publisher-атрибуции не выдумывается. Добавления идемпотентны; это несколько DDL-операций, не общая транзакция. При ошибке исправить указанную несовместимость и повторить команду; не запускать полный CreateDB, --schema-only, --publisher-pop-repair или полный RTB V4 DDL. Legacy request writer прекращается только после подтверждённого успешного нового refresh; его таблица/read VIEW остаются нетронутыми.
3. Установить новый Loader обычным способом. Убедиться, что Pub и Kafka Loader соответствуют исходникам proj234: добавление колонок не научит старые binaries передавать Publisher IDs. RunRevenueSync и финансовые workers не отключать. Сначала обновить backend кабинета, затем frontend: старые имена API-полей сохранены, новый frontend требует расширенный контракт.
4. Проверить refresh и контрольный реальный Publisher-запрос. HTTP 204 должен подтверждать active без обязательного показа/дохода. Проверить одного владельца и недоступность его кодов другому владельцу; код без зон должен иметь отдельное состояние и не прибавлять problem_zones.

Полезные SQL после установки (здесь используется штатная БД ads):

```sql
SHOW CREATE TABLE ads.publisher_inventory;
SHOW CREATE TABLE ads.mv_ortb_to_publisher_inventory;
SHOW CREATE TABLE ads.mv_publisher_zone_hourly;
SELECT database,view,status,last_success_time,next_refresh_time,exception
FROM system.view_refreshes
WHERE database='ads' AND view='mv_publisher_zone_hourly';

SELECT created_at,event_time,code,pub_user_id,ad_code_id,zone_id,site_id,logical_event_id
FROM ads.ortb
PREWHERE created_at>=now64(3,'UTC')-INTERVAL 10 MINUTE
WHERE pub_user_id!=''
ORDER BY created_at DESC LIMIT 10;
```

`GET /api/publisher/zone-diagnostics?limit=1&offset=0` должен возвращать три отдельных summary. `GET /api/publisher/ad-codes/{id}/diagnostics` — настоящие зоны либо code-level no_zones_detected. У inventory API должны быть новые scope/source/period поля. Проверить это штатным JWT без подмены данных. При stale/gapped refresh сохраняется 503; отсутствие данных не превращается в успешные нулевые HTTP-счётчики.

Ограниченный benchmark выполнен на отдельном локальном ClickHouse, не на production. Фикстура использует реальный ORTB DDL, UUID-подобные logical IDs, 10% Publisher-строк, 10% повторов среди Publisher, одного owner/code, 1000 зон и короткие остальные идентификаторы. Времена распределены по часу, 5% строк искусственно старше часа; TTL/merges фикстуры при замерах приостановлены для постоянного набора данных. Исходник занимал около 2,08 GB на диске. Это не непрерывный ingest 40 000 RPS и не копия production-кардинальности/IO. Показаны завершённые SELECT; failed исходный вариант не объявляется успешным.

| Вариант | Строк ORTB | Время SELECT | Пик memory_usage | CPU user+system | Внешняя агрегация, сжатые записи |
|---|---:|---:|---:|---:|---:|
| Исходный, 512 MiB | 12 млн | 7,45 / 6,20 с | 136 MiB во втором проходе | 10,17 с во втором проходе | 53 MiB во втором проходе |
| Исходный, 512 MiB | 36 млн | ошибка памяти дважды | достигнут предел 512 MiB | запросы не завершены | spill происходил, но лимита не хватило |
| Итоговый, 1 GiB | 36 млн | 46,57 / 41,05 с | 702 / 554 MiB | 85,74 / 75,80 с | 184,5 / 182,3 MiB |

Прочитано 7 480 537 600 логических байтов на каждый итоговый проход; наблюдаемый пик временных файлов — около 185/182 MiB. EXPLAIN подтверждает передачу created_at в PREWHERE и использование индексов, однако на большой фикстуре выбраны все 4464 гранулы: 65 минут почти покрывают часовой источник, а старые строки попали в общие гранулы. Publisher-фильтр уменьшает данные для GROUP BY, но не устраняет чтение исходных колонок. Wide bid_responses_raw/adv_rtb_responses_raw не читаются. Основной расход памяти — точная дедупликация logical IDs; её смысл не менялся. FINAL и повторные снимки проверены SQL-тестами, включая replay/merge/UTC/TTL. Дополнительно на 36 млн исходных строк реально вставлены два снимка в ReplacingMergeTree: 12 100 физических / 6 050 логических строк до merge; суммы после второго снимка и после OPTIMIZE FINAL совпали с первым (1 350 000 HTTP 200 и 1 070 000 HTTP 4xx; прочие два счётчика этой фикстуры — ноль). Покрытие всех четырёх классов HTTP отдельно проверено целевыми SQL-тестами.

Параллельно выполнялся отдельный контрольный SELECT по таблице из миллиона строк: baseline p95 около 16 ms, под итоговым refresh около 20–35 ms. Конкуренция за ресурсы наблюдается. Это не доказательство отсутствия влияния на реальные финансовые запросы: их production-набор, фоновые merges, ingest, холодный cache, распределённый ClickHouse и вся исходная кардинальность здесь не воспроизведены. Значения памяти получены из system.query_log; полный RSS сервера отдельно не атрибутировался этому запросу.

36 млн — четверть часового ориентира 144 млн строк. Полные 144 млн, непрерывные 40 000 RPS и работа при таком объёме в каждом пятиминутном цикле **не подтверждены**. Нельзя линейно гарантировать время/память или отсутствие помех production. В пределах проверенного объёма оба итоговых прохода уложились в пять минут; расписание осталось прежним. Источник с часовым TTL по-прежнему не позволяет восстановить произвольно поздние/потерянные события после долгого простоя. Предыдущие ограничения окна late events и проверки heartbeat сохранены.

Для воспроизведения ограниченного опыта на отдельном локальном ClickHouse (HTTP 18123, query_log/ProfileEvents включены):

```bash
python3 docs/publisher/benchmark_zone_diagnostics.py --disposable-local \
  --rows 12000000 36000000 --memory-mib 1024 --output /tmp/zone-benchmark-final
```

Скрипт создаёт собственную БД zone_bench_<UUID>, использует только localhost, не читает production-таблицы и оставляет результаты в указанном output. Для сравнения лимита имеется `--memory-mib 512`. При доступе к temp каталогу тестового сервера можно задать ZONE_BENCH_TEMP; без него пик файлов помечается неизвестным, spill берётся из query_log. Сам опубликованный скрипт дополнительно проверен на 100 000 строк; основные измерения выше получены из того же генератора и точного диагностического SELECT. Машинные результаты исходного и итогового опытов приложены внутри RTB patch в docs/publisher/benchmark_*_results.json.
