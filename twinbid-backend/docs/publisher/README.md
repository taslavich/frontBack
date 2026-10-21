# Publisher FINAL

Publisher Ad Code принимает только явно выбранный `adult` или `mainstream`.
POST без traffic_type / с null / с неподдерживаемой строкой даёт 400.
PATCH без traffic_type сохраняет существующий тип; переданный null или неверный тип → 400.
Дублирование сохраняет допустимый тип. Pub выбирает соответствующий существующий
POP feed, без fallback. Старые advertiser режимы/кампании не менялись.

Pub→SSP UUID/HMAC/prewrite/ready→Kafka→CH и zone_id сохранены из REVISED.
Технический /confirm только запускает interval/session quota после открытия bridge,
не вызывает BURL и не начисляет деньги. Браузер переходит к тому же ADM/id;
Параметры POP, включая XML destination, берутся adapter из Redis; ADM содержит только UUID. No-fill/blocked open
не расходуют квоту. Первый/второй page click считается JS до аукциона.

Единственный накопленный денежный агрегат CH — существующий agg_stats с
pub_user_id/ad_code_id/zone_id. Publisher POP `ssp_revenue_views` начисляется от
уникального clicks_uuid в fact_clicks (`win_final_price/1000`), то есть обращения
к /adm. Долговечный реестр событий дедуплицируется ДО денежной суммы; одна native
refresh task сверяет только dirty UUID-сегменты с aggregate; зависимый checkpoint подтверждает совпадение сумм.
BURL/recovery сохраняют advertiser impressions/spend, но не дают POP SSP revenue.
Publisher MVP поддерживает выплаты только POP: другие format buckets показывают counters с revenue=0. Requests/errors/
no-fill и lifetime discovery остаются в прежних компактных проекциях. Подробности
и обязательный порядок выкладки UUID-only/DDL — в RTB docs/publisher/FINAL_229.md.

Backend читает деньги и факт-счётчики из agg_stats; errors/requests — из requests
projection. Весь отчёт имеет разрешение UTC 1 час; effective_from/effective_to
показывают расширенные часовые границы. UTC+3 и прежние legacy timezone значения
нормализуются в publisher reports. IANA часовые метки показывают начало UTC bucket
в локальной зоне; границы субчасовых/local-day buckets из hour aggregate не восстанавливаются.

Автоматический RunRevenueSync запускается всегда. Каждый 30s цикл
берёт PG advisory transaction lock ДО CH snapshot, проверяет sorting key и все
четыре writer MV и три POP writer/checkpoint MV, получает publisher cumulative и атомарно фиксирует checkpoint,
publisher_revenue cumulative, delta journal и balance+=delta. Не заменяет баланс
кумулятивом и не восстанавливает выплаченные деньги при рестарте. Corrections и
пустой owner в успешно прочитанном полном snapshot дают отрицательную delta;
CH failure не обнуляет накопления. Неизвестный исход commit безопасен при retry.
Отрицательный баланс сохраняется как долг; reserved не стирается, paid transition
блокируется, если balance<reserved. Внешний financial endpoint всегда возвращает
409: единственный authority для POP — агрегат.

Balance отдельно возвращает cumulative_earned, balance, reserved, paid_out,
available_for_payout и revenue_status. agg_stats_pending/ready/stale/unavailable
берутся из реально сохранённого состояния sync. Analytics revenue доступен из CH;
balance отражает последний применённый snapshot. Старые внешние начисления требуют
явного аудита/архивирования и переноса checkpoint, иначе auto sync откажет.

Фронтенд/бот не изменялись. Контракт и необходимые действия — API_MATRIX.md.
Миграции и порядок остановки writers — MIGRATIONS.md. Реальные проверки/пределы —
TEST_REPORT.md и отдельный README_APPLY.md. До деплоя нужен интеграционный прогон
на версии CH/Postgres production: успешная Go сборка не доказывает live DDL.
