# Миграции FINAL

## PostgreSQL traffic_type

EnsurePublisherSchema в транзакции удаляет default traffic_type и старый именованный
CHECK, добавляет новый CHECK IN ('adult','mainstream') NOT VALID. Новые INSERT/UPDATE
сразу проверяются. Если недопустимых строк нет, CHECK автоматически VALIDATE.
Если есть — строки не переименовываются и их владельцы не меняются. Runtime/duplicate/
GET не принимают их, список/sync обрабатывает только допустимые строки.
Оператор должен получить явный выбор владельца и исправить каждую строку:

```sql
SELECT ad_code_id,user_id,traffic_type FROM ad_codes
WHERE traffic_type NOT IN ('adult','mainstream');
-- Только после явного выбора типа; UUID конкретного кода, никакого массового mapping:
UPDATE ad_codes SET traffic_type='adult',version=version+1,updated_at=now()
WHERE ad_code_id='<reviewed-code-uuid>';
ALTER TABLE ad_codes VALIDATE CONSTRAINT ad_codes_traffic_type_check;
```

Можно оставить ошибочные строки блокированными до выбора; NOT VALID не разрешает
новые неверные значения. Не пытаться менять их is_active до исправления типа:
CHECK проверяет любую UPDATE строки. Пользовательский advertiser timezone не мигрируется.

Ledger ad_code_id допускает NULL только для account cumulative record; прежние
Code/FK/ledger rows сохраняются. Добавляются publisher_revenue_checkpoints,
publisher_revenue_deltas и состояние sync. Первое начисление идёт от нулевого
checkpoint, только если старого внешнего источника нет. Внешний endpoint отключён.
Если внешние кредиты уже есть, сохранить аудит и явно согласовать их перенос/
архивирование и стартовый checkpoint; нельзя просто повторно зачислять весь cumulative.

## ClickHouse shared agg_stats

Требуется maintenance window: приостановить pub и publisher revenue sync/backend,
остановить ВСЕ CH ingestion writers фактов и экземпляры CH Loader. Kafka сохраняет
входящий поток. Сохранить backup agg_stats/schema и advertiser SUM по выбранным
периодам. Незавершённые async inserts должны быть подтверждены/дренированы до DDL.
Не выполнять параллельный CreateDB несколькими экземплярами.

CreateDB сначала останавливает прежние recovery writers (как исходная версия) и
fact-producing refreshable views. В schema block дополнительно DROP старую
impressions→agg MV. Затем в ОДНОМ ALTER добавляет три String ключа БЕЗ DEFAULT и
MODIFY ORDER BY с прежним prefix плюс этими колонками в конце. После атомарного
расширения отдельный ALTER устанавливает DEFAULT ''. Старые rows имеют пустые ID
и 0 SSP revenue. Новая таблица создаётся сразу с полным ключом/defaults.
Документация ограничения:
https://clickhouse.com/docs/reference/statements/alter/order-by

Все четыре writer определения заменяются, не сохраняются под IF NOT EXISTS.
IPP delete/rebuild сохраняет исходную двухчасовую границу и формулы, протягивая ID
и ноль revenue. Новая impressions MV создаётся ДО исходного POP catch-up; recovery
и recurring SQL остаются прежними кроме ранее согласованных attribution projections.
Обычная история не переагрегируется. Не запускать backfill фактов в agg повторно:
это удвоит advertiser spend. Старый unattributed history не получает выдуманных publisher ID.

После CreateDB проверить ключ/4 MV, SUM advertiser before/after и новую revenue;
затем возобновить CH consumers, pub и backend sync. Kafka gap догоняет штатный
ingest. Не растягивать окно за TTL существующих input buffers. Пример SQL проверки:

```sql
SELECT sorting_key FROM system.tables WHERE database='ads' AND name='agg_stats';
SELECT name,create_table_query FROM system.tables WHERE database='ads'
 AND name IN ('mv_fact_impressions_to_agg_stats','mv_fact_clicks_to_agg_stats',
 'mv_fact_clicks_wins_to_agg_stats','mv_fact_conversions_to_agg_stats');
SELECT win_user_id,win_cid,sum(spend_views_table),sum(spend_clicks_table)
FROM ads.agg_stats GROUP BY win_user_id,win_cid ORDER BY win_user_id,win_cid;
SELECT pub_user_id,ad_code_id,site_id,zone_id,sum(ssp_revenue_views)
FROM ads.agg_stats WHERE format='POP' GROUP BY pub_user_id,ad_code_id,site_id,zone_id;
```

Заменить ads фактической DB. Source должен читаться с одной согласованной точки
(не чередовать lagging replicas: снижение cumulative трактуется как correction).
Частично ранее добавленные pub key columns без включения в sorting key нельзя
молча лечить MODIFY ORDER BY: CH запретит использование existing columns. В таком
случае остановиться на конкретной DDL ошибке и провести отдельный audited migration,
сохранив исходную таблицу; не удалять/переагрегировать её автоматически.

Rollback: отключить pub/revenue producer, оставить таблицы/расширенный key/ledger
и Kafka offsets. Не запускать старый CreateDB, который уберёт suffix sorting key;
это может слить разных владельцев. Откат кода не отменяет признанные деньги.
Нужен dry-run интеграционного теста и backup перед production migration. Live DDL
не объявляется доказанным без прогонов на настоящей версии сервера.
