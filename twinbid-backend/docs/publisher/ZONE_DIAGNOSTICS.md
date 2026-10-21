# TwinBid Publisher Zone Diagnostics — реализация, проверки и развёртывание

Дата повторной проверки восстановленной версии: 21.10.2026.

Основа строго из `proj232(2).zip`, `FProj233(1).zip` и `TwinBid_Frontend_Publishers_FINAL_v3(2).zip`. Финансовая бизнес-логика не менялась. Это реализация новой диагностики, а не повторное применение полной миграции RTB V4.

## Артефакты и применение

1. `proj232_publisher_zone_diagnostics.patch` — Git-патч относительно корня RTB, где находится `go.mod`.
2. `FProj233_publisher_zone_diagnostics.patch` — Git-патч относительно `twinbid-backend/`, где находится `go.mod` backend кабинета.
3. `TwinBid_Frontend_Publishers_Zone_Diagnostics_FINAL.zip` — полный frontend с исходниками, lockfile и исходным `.env.production` (`VITE_USE_MOCK=false`), без node_modules/dist.

```bash
# В чистом распакованном proj232(2).zip:
git apply --check /path/proj232_publisher_zone_diagnostics.patch
git apply /path/proj232_publisher_zone_diagnostics.patch
go build ./...
go build -o ./bin/clickhouse-loader ./cmd/clickhouse-loader

# В каталоге twinbid-backend из чистого FProj233(1).zip:
git apply --check /path/FProj233_publisher_zone_diagnostics.patch
git apply /path/FProj233_publisher_zone_diagnostics.patch
go build ./...

# В распакованном frontend:
npm ci
npm test
npm run build
npx tsc --noEmit -p tsconfig.app.json
npm run lint
```

Go: 1.25.11. ClickHouse для интеграционных проверок: 25.8.18.1. Redis: 7.0.15. Node: 24.19.0. SQL требует поддержки Refreshable MV с APPEND (ClickHouse 24.9+); перед rollout используйте тестовый стенд с фактической версией production. Поведение проверено на указанной версии, не на всех выпусках ClickHouse.

## Что изменено

RTB:

- `cmd/clickhouse-loader/main.go`: отдельный `--publisher-zone-diagnostics-only`; обычный запуск существующей БД не запускает полный финансовый DDL.
- `internal/services/clickhouse-loader/publisher_zone_diagnostics.go`: схема, refresh SELECT, миграция и безопасная проверка запуска Loader.
- `internal/services/clickhouse-loader/createDb.go`: убрано создание старого request projection; для новой БД добавлена новая диагностика. Остальной финансовый DDL сохранён.
- `publisher_zone_diagnostics_test.go`, `internal/services/pub/zone_diagnostics_test.go`: новые интеграционные проверки.
- `publisher_pop_revenue_v2_test.go`: проверка обычного перезапуска использует реальный новый путь EnsureLoaderSchema, а не финансовый cutover.
- `docs/publisher/API_MATRIX.md`, `ZONE_DIAGNOSTICS.md`: контракт и инструкции.

Backend кабинета:

- `internal/publisher/zone_diagnostics.go`: DTO, активные коды владельца из PostgreSQL, endpoints и правило активности.
- `internal/stats/publisher_zones.go`: данные зон, проверка свежести/покрытия, глобальные итоги и совместимые технические counters общей статистики.
- `internal/publisher/reports.go`, `service.go`: новый контракт существующего URL, новый summary URL, уведомления о проблемах используют то же правило последнего часа.
- `internal/stats/publisher.go`: полностью убраны обращения к publisher_requests_logical; финансовая ветка UNION по agg_stats сохранена.
- `attention_test.go`, `zone_diagnostics_test.go`, `publisher_zones_test.go`, `publisher_format_test.go`: обновлённые и новые тесты.
- `docs/publisher/API_MATRIX.md`, `ZONE_DIAGNOSTICS.md`.

Frontend:

- `src/api/publisher.ts`, `publisherContract.ts`: новые DTO и проверка ответов.
- `src/components/publisher/PublisherDiagnostics.tsx`: таблица зон, пагинация, UTC, HTTP counters и две карточки активности.
- `src/pages/PublisherOverview.tsx`: реальные серверные counts активных/проблемных зон.
- `src/contexts/PublisherContext.tsx`: обновление диагностики раз в минуту.
- `src/test/publisher-fixtures.ts`, `publisher-zone-diagnostics.test.tsx`: fixtures только для тестов и 5 новых UI-тестов.
- `PUBLISHER_ZONE_DIAGNOSTICS.md`: этот отчёт.

Ранее скрытые requests/fills/no_fills/errors не возвращены в карточки, таблицы или графики статистики. Их поля в API/TypeScript сохранены. HTTP 200/204/4xx/5xx отображаются только в технической диагностике.

## Схема и корректность обновлений

`ads.publisher_zone_hourly` — ReplacingMergeTree(revision), партиционирование по UTC-дню event_bucket. Основные измерения: event_hour, pub_user_id, ad_code_id, zone_id, site_id. Дополнительно сохранены пятиминутный event_bucket, измерения для действующих фильтров, revision, refreshed_at, expires_at, heartbeat и компактный массив счётчиков по секундам. HTTP counters не считаются показами или деньгами.

`ads.mv_publisher_zone_hourly` — **REFRESH EVERY 5 MINUTE APPEND**, без INSERT-trigger. APPEND используется для записи ПОЛНОЙ новой версии интервала в ReplacingMergeTree, не для суммирования перекрывающихся приращений. Каждый читатель использует FINAL до SUM. Повторный refresh и фоновые merges не увеличивают итог. Нельзя читать эту таблицу через обычный SUM без FINAL.

Каждый refresh:

1. Читает физическую ads.ortb напрямую, независимо от наличия ortb_logical.
2. PREWHERE ограничивает created_at последними 65 минутами; Publisher определяется по заполненным доверенным pub_user_id/ad_code_id/zone_id/site_id.
3. Дедуплицирует по pub_user_id + стабильному logical_event_id, выбирая argMin по (created_at,uuid). Пустой legacy logical_event_id заменяется UUID. Предполагается неизменяемое содержимое Kafka-события при replay.
4. Полностью заменяет снимки последних 45–50 минут (граница округляется до пяти минут). Более старые интервалы запечатаны: их нельзя пересчитывать из уже усечённого источника с TTL 1 час.
5. Записывает heartbeat **предыдущего завершившегося успешно** refresh из system.view_refreshes.last_success_time. Он не подтверждает собственный APPEND: при ошибке INSERT SELECT некоторые блоки могли уже попасть в таблицу. Неуспешный run не продвигает покрытие. Первый bootstrap считывает доступный час; marker покрытия записывается отдельной командой только после успешного завершения INSERT.

Внутри каждого пятиминутного снимка секунды нужны для точного скользящего интервала `[UTC now, округлённое вниз до секунды − 1 час; UTC now, округлённое вниз до секунды)`. Это не сумма двух календарных часов. Counts отражают последнюю завершённую обработку; свежие ещё не обновлённые события появятся после следующего refresh.

TTL = event_bucket + INTERVAL 7 DAY. Это семь дней истории интервалов; поля expires_at фильтруются также при чтении. Физическое удаление ClickHouse выполняет асинхронно при TTL-merge: нельзя обещать удаление байтов точно в секунду истечения. Принудительное MATERIALIZE TTL проверено в тесте. Никаких последних 50 HTTP-запросов или длинного журнала event IDs новая таблица не хранит.

## Реестр, API и изоляция

В этом исходном проекте зоны/сайты появляются после первого запроса: отдельного PostgreSQL-каталога вручную созданных зон нет. PostgreSQL ad_codes — авторитетный список активных неархивированных кодов владельца. Для каждого такого кода используется существующий **долговечный publisher_inventory без TTL**, а не список из недавних ORTB-событий. Поэтому ранее обнаруженная зона остаётся в результате при полном отсутствии трафика за час. Никогда не обнаруженная зона не имеет известного site/zone ID; такой ID не выдумывается.

Новые запросы ограничены одновременно владельцем JWT и списком его активных кодов. В SQL используются bound параметры. Прямой URL диагностики кода сначала проходит существующий ownerCode. Пагинация стабильна по ad_code_id/zone_id/site_id; summary counts считаются по всему реестру, не только по текущей странице.

- `GET /api/publisher/zone-diagnostics?limit=1&offset=0`: rows, active_zones, problem_zones, limit, offset, window_from, window_to, refreshed_at. Overview получает общие counts, limit=1 не урезает итоги.
- `GET /api/publisher/ad-codes/{id}/diagnostics?limit=50&offset=0`: `{ad_code,zones:ZoneDiagnostics,runtime_ready}`. URL сохранён, events удалён из нового контракта.
- Row: ad_code_id, zone_id, site_id, hostname, status, requests_200, requests_204, requests_4xx, requests_5xx, last_request_at.
- Активна: requests_200>0 ИЛИ requests_204>0. Ошибки не отменяют успешную активность. Только ошибки или отсутствие успешных запросов → problem.
- last_request_at берётся из долговечного реестра и может быть старше часа.
- Ошибка ClickHouse, устаревший heartbeat или разрыв refresh-покрытия → API error/503, без подмены нулевыми данными. Runtime-ready и HTTP-активность — разные признаки.

Общая статистика/инвентарь больше не используют publisher_requests_logical. Скрытые технические counters там берутся из новых снимков и теперь ограничены их семидневной историей. Денежная статистика, impressions/clicks, lifetime revenue и баланс продолжают использовать прежние финансовые источники. Старые скрытые шестимесячные request counters из старой диагностики не подмешиваются.

## Проверка реальных HTTP-путей

ORTB: code UInt16, event_time/created_at DateTime64(3,'UTC'), logical_event_id/pub_user_id/ad_code_id/zone_id String, site_id Nullable(String). Loader сохраняет code и Publisher IDs из Kafka; logicalOrtbEventID сохраняет стабильность replay.

Изучены pub/server.go, pub/ortb.go, sspAdapter/web и clickhouse-loader/ortb.go. В существующем протоколе Pub подготавливает один UUID, Adapter атомарно захватывает его и завершает кодом RTB; Loader переносит это событие. Проверки фактических Adapter paths и Pub prewrite/commit охватывают 200/204. Новые проверки отказов до admission охватывают 403/429/502/503 с сохранением доверенных owner/code/site/zone IDs.

Не все внешние HTTP-ошибки попадают в ORTB: отказы до установления доверенной идентичности кода, некорректный JSON/CORS, часть ошибок Redis/телеметрии, потерянные события и неизвестный исход после admission нельзя надёжно приписать зоне. Они не дорисовываются. HTTP-code в ORTB — сохранённый результат RTB, а не обязательно внешний код SDK: например, no-fill RTB 204 может быть обёрнут Pub в HTTP 200 с JSON status=no_fill. Запрет на изменение routing/биллинга соблюдён; производственные HTTP-ветки не переписывались.

## Фундаментальные ограничения

Источник с TTL 1 час не позволяет восстановить произвольные опоздавшие события или пропущенную неделю после длительного простоя без дополнительного долговечного источника. Refreshable MV не устраняет это ограничение.

Реализовано поддерживаемое окно опоздания до 30 минут при нормальном refresh каждые 5 минут, перерыве между успешными обновлениями не более 10 минут и разумном времени выполнения запроса (целевой бюджет до 5 минут). Более старые события за пределами изменяемых 45–50 минут не включаются в запечатанные интервалы. Replay такого старого события не удваивает сохранённый снимок. Событие, пришедшее впервые слишком поздно, может быть пропущено.

Если был долгий простой, предыдущие потерянные интервалы не объявляются пустыми. API проверяет цепочку heartbeat за требуемый час и возвращает 503, пока не накопится достаточное непрерывное покрытие. При первом развёртывании доступен только фактически сохранившийся сырой час; семидневная история накапливается вперёд. Повторно вручную bootstrap нельзя использовать как доказательство восстановления потерянных прошлых данных.

Диагностика имеет задержку до 5 минут плюс длительность запроса. Подтверждение последнего успешного refresh записывается следующим run; после перезапуска ClickHouse первый refresh может ещё не иметь last_success_time, поэтому до следующего run возможен временный 503 вместо непроверенных данных. На самой границе скользящего часа точность — одна секунда; это явно отражено window_from/window_to. Данные Redis/Kafka/Loader должны вообще достигнуть ORTB, чтобы участвовать в счётчиках.

## Фактически выполненные проверки

Все перечисленные ниже проверки повторены для восстановленных исходников, а не перенесены из предыдущего исчезнувшего workspace.

- `npm ci`: успешно, lockfile не менялся.
- `npm test`: **57 файлов, 322 теста успешно**, включая 5 новых тестов диагностики и существующие проверки скрытых метрик/Format, payouts request_id, авторизации и прочего UI.
- `npm run build`: успешно; стандартное предупреждение о больших chunks сохранено.
- `npx tsc --noEmit -p tsconfig.app.json`: успешно.
- `npm run lint`: 0 ошибок, 38 существующих предупреждений.
- `go build ./...` в обоих backend: успешно.
- RTB `go test -race -count=1` для clickhouse-loader, pub, sspAdapter/web, sspAdapter/billing, sspAdapter/outbox, callback: успешно на настоящих локальных ClickHouse/Redis.
- Cabinet `go test -race -count=1` для stats, publisher, pubcontract: успешно на схеме, созданной фактическим RTB CreateDB в отдельной тестовой БД. Последняя правка сохранения last_request_at также проверена этим прогоном.
- Новые RTB zone tests повторены с `-race -count=3`: успешно. Проверены legacy writer cutover без удаления старого чтения/данных, dedup Kafka, повторный snapshot, late event, sealed replay после удаления источника, merge, UTC, TTL, принудительный провал APPEND через CHECK constraint без ложного heartbeat и неизменность финансового writer DDL.
- Actual CH cabinet tests: only 200, only 204, mixed 200/500, errors only, silent registry zone, totals независимо от страницы, чужой owner, недопустимый code, пустой список кодов, stale refresh.
- Owner/active-code фильтрация PG проверена через существующий тестовый SQL connector; live PostgreSQL для полного HTTP→PG→CH стенда не поднимался.
- Playwright против production dist с изолированными HTTP fixtures: RU/EN/zh-CN × 320/375/1440 px, всего 9 вариантов; overview и реальный code dialog/diagnostics, 8 колонок, отсутствие горизонтального переполнения document и page errors. Мобильный screenshot также визуально просмотрен. Таблица имеет собственную горизонтальную прокрутку внутри диалога.
- Оба патча: `git apply --check` успешно на заново распакованных исходных архивах; патчи применены, все полученные файлы побайтно совпали с проверенными рабочими копиями. Полный ZIP: CRC успешно, 313 файлов; все исходные файлы сохранены, содержимое совпадает с проверенной версией, `.env.production` не изменён.

**Полный `go test ./...` в обоих backend запускался и НЕ зелёный.** Ошибки воспроизведены отдельно на чистых исходных архивах:

- RTB: internal/coder; internal/config (разбор некорректной пары); internal/geoBadIp (DSP V25 rules); internal/services/adv/service (набор diagnostic definitions); internal/services/orchestrator/service (go vet: log.Printf с `%w`).
- Cabinet: internal/campaigns (validation type models/video) и internal/topups (immediate/static wallet сценарии).

Эти посторонние тесты не исправлялись, так как пользователь запретил менять финансовую и рекламодательскую логику. Успех целевых пакетов и сборок не выдаётся за успех полного набора.

Ограничения тестового стенда: нет живого Kafka broker, настоящих DSP, production ClickHouse cluster/failover и полного live PostgreSQL. Интеграционные тесты реальных SQL/Redis путей и неизменность исходников финансов не заменяют приёмку на production-подобном стенде.

## Производительность

Новая диагностика не добавляет MV, выполняемую на каждом INSERT в ORTB. Существующая лёгкая discovery MV publisher_inventory сохранена без изменений.

Refresh SELECT ограничен двумя потоками, 512 MiB max_memory_usage, внешней агрегацией после 128 MiB. Нужен запас временного диска для spill. Отсутствие нагрузки на INSERT не означает отсутствие конкуренции за CPU/диск с другими запросами.

Повторный синтетический benchmark окончательного SELECT (включая чтение system.view_refreshes) на этом локальном ClickHouse: 10% Publisher traffic, 100 зон, временное окно 40 минут, отдельная таблица без финансовых MV:

| Всего строк ORTB | Два времени SELECT | Прочитано байтов | Снимков в результате |
|---|---:|---:|---:|
| 1 000 000 | 0,460 / 0,308 с | 155 683 792 | 815 |
| 5 000 000 | 1,522 / 1,385 с | 761 500 824 | 815 |

Это не тест непрерывных 40 000 RPS. Peak RSS отдельно не измерялся. Около 144 млн сырых строк в час при таком RPS и фактическая кардинальность могут дать другой профиль: на стенде измерить system.query_log, длительность refresh, IO и spill. Для чтения system.view_refreshes нужны соответствующие права Loader. Сканирование существующей ORTB всё ещё имеет стоимость. Не обещается независимость задержек биллинга от общих ресурсов ClickHouse при произвольной нагрузке.

## Миграция и безопасный порядок rollout

1. Сохранить текущие версии трёх приложений и `SHOW CREATE TABLE/VIEW` существующей диагностики. Проверить версию ClickHouse и права CREATE/INSERT/SELECT/ALTER/SYSTEM REFRESH/WAIT/DROP VIEW для диагностики. Проверить запас диска, память, возможность выполнения refresh. Финансовые таблицы/offsets не сбрасывать.
2. Применить RTB patch и собрать новый Loader. Пока старые сервисы обрабатывают трафик, запустить **один экземпляр отдельной команды** с тем же существующим конфигурационным окружением Loader:

   ```bash
   ./bin/clickhouse-loader --publisher-zone-diagnostics-only
   ```

   Команда создаёт только новую таблицу/MV, bootstrap доступного часа, выполняет START/REFRESH/WAIT, проверяет last_success_time и exception в system.view_refreshes и лишь после подтверждённого успеха останавливает старый writer `mv_ortb_to_publisher_requests`. Старая таблица publisher_requests и обычный VIEW publisher_requests_logical остаются для безопасного отката. При ошибке до этого последнего шага старый writer не удаляется. Не запускать миграцию одновременно в нескольких экземплярах.

3. Не перезапускать старый Loader после cutover: его прежний полный DDL снова создаёт старую диагностику и затрагивает финансовые MV. Заменить его новым Loader. Обычный новый запуск на существующей ortb делает только проверку наличия объектов, без полного DDL/refresh/mutations. Отсутствие новой диагностики выдаёт warning, не блокируя Kafka/финансовый ingest.
4. Проверить refresh в `system.view_refreshes` (status, exception, last_success_time) и heartbeat. Проверить `SHOW CREATE TABLE ads.publisher_zone_hourly`; все читатели — FINAL. Проверить отсутствие старого writer. Не удалять старые таблицы/данные в этой задаче.
5. Развернуть backend кабинета и frontend согласованно, в одном коротком окне или через blue/green: контракт code diagnostics поменял events на zones. Убедиться, что новые клиент и сервер работают в паре. Не отключать RunRevenueSync.
6. Выполнить приёмку JWT двух пользователей, проверки 200/204/mixed/errors/silent, cards, filters, paginator, mobile и копирования кода. Сверить финансовые отчёты/балансы и завершение текущих Loader INSERTs; финансовые replay/repair не запускать.

**Не использовать для этой задачи `--schema-only`, `--publisher-pop-repair`, полный RTB V4 DDL или CREATE OR REPLACE финансовых MV.** Эти старые команды сохранены для других явных процедур, но не нужны для диагностики. Запуск полного CreateDB предназначен для действительно пустой БД; он не является путём миграции действующей схемы.

Откат: вернуть согласованную пару cabinet/frontend. Если нужна старая диагностика, восстановить только её старый writer из заранее сохранённого CREATE; данные/view оставлены. Не запускать полный старый Loader DDL для отката и не replay финансовых потоков. Новую диагностику можно оставить без читателей до отдельного решения об очистке. Старые counters после остановки legacy writer не будут автоматически догоняться в старой версии — это диагностическое, не финансовое ограничение отката.

Не менялись `/adm`, `/burl`, `/nurl`, DSP/SSP routing, Simple/Complex, fact_impressions/fact_clicks, traffic_volume_hourly, финансовый SQL agg_stats, Publisher POP billing, antifraud/dedup денег, JWT, баланс/ledger/payouts/request_id, RunRevenueSync и рекламодательский UI. Старые production HTTP handlers RTB/Pub не правились. Никаких новых финансовых начислений по BAN/NAT/IPP/VID нет.
