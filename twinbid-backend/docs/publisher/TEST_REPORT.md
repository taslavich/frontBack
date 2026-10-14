# Проверки FINAL

PASS: Go build/tests изменённых пакетов, strict traffic validation и обязательный
POST контракт (PATCH проверяется в условном PG test), scoped aggregate SQL, shared agg writer/schema contracts.
Результат race/Redis/Node проверки и применения к чистым базам — README_APPLY.md.

Условные настоящие CH tests: populated legacy agg migration, четыре writer MV,
ordinary/recovered POP SSP CPM, non-publisher zero, publisher key separation,
advertiser SUM preservation, repeat CreateDB и small-fixture parts/bytes/timing.
Запуск: PUBLISHER_TEST_CLICKHOUSE_ADDR=host:9000 go test -v ./internal/services/clickhouse-loader.

Условные настоящие PG tests: cumulative повтор/рестарт/несколько instance,
late increases, отрицательные delta, payout/reserve/debt и rollback invalid snapshot.
Запуск: PUBLISHER_TEST_POSTGRES_DSN=<disposable db> PUB_TEST_REDIS_ADDR=<disposable redis>
go test -race -v ./internal/publisher. DB предварительно содержит исходную users schema.

В текущем окружении CH binary download отказал 403. Postgres initdb отказал root,
а sandbox не разрешил non-root UID mapping. Эти интеграции SKIP, не E2E PASS.
Chromium/Kafka broker не подняты. CPU/RPS/MV throughput и влияние на production
advertiser latency не измерялись. Миграцию необходимо прогнать на версии production
с указанными fixture tests до live deploy; успешная компиляция не заменяет этот прогон.
