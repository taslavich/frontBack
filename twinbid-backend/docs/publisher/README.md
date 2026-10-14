# Publisher FINAL

Publisher Ad Code принимает только явно выбранный `adult` или `mainstream`.
POST и PATCH без traffic_type / с null / с неподдерживаемой строкой дают 400.
Дублирование сохраняет допустимый тип. Pub выбирает соответствующий существующий
POP feed, без fallback. Старые advertiser режимы/кампании не менялись.

Pub→SSP UUID/HMAC/prewrite/ready→Kafka→CH и zone_id сохранены из REVISED.
Технический /confirm только запускает interval/session quota после открытия bridge,
не вызывает BURL и не начисляет деньги. Браузер переходит к тому же ADM/id;
POP XML извлекается в параметр url того же ADM wrapper. No-fill/blocked open
не расходуют квоту. Первый/второй page click считается JS до аукциона.

Единственная денежная таблица CH — существующая agg_stats. В её конце ключа
добавлены pub_user_id/ad_code_id/zone_id; новый ssp_revenue_views считает
sumIf(win_final_price/1000, format='POP' AND pub_user_id!='') в прежней impressions MV.
Advertiser spend и все прежние формулы/WHERE сохранены. Три остальные writer MV
и IPP rebuild передают ID с нулём SSP revenue. Recovery SQL не переинтерпретируется:
те же recovered facts проходят ту же impressions MV. Requests/errors/no-fill
и lifetime discovery используют прежние компактные проекции, не вторую денежную MV.

Backend читает деньги и факт-счётчики из agg_stats; errors/requests — из requests
projection. Весь отчёт имеет разрешение UTC 1 час; effective_from/effective_to
показывают расширенные часовые границы. UTC+3 и прежние legacy timezone значения
нормализуются в publisher reports. IANA часовые метки показывают начало UTC bucket
в локальной зоне; границы субчасовых/local-day buckets из hour aggregate не восстанавливаются.

Автоматический RunRevenueSync запускается при PUBLISHER_ENABLED. Каждый 30s цикл
берёт PG advisory transaction lock ДО CH snapshot, проверяет sorting key и все
четыре writer MV, получает publisher cumulative и атомарно фиксирует checkpoint,
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
