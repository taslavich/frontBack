import { createContext, useContext, useState, useCallback, type ReactNode } from "react";

type Lang = "ru" | "en";

const translations: Record<string, Record<Lang, string>> = {
  // Header / Nav
  "nav.benefits": { ru: "Преимущества", en: "Benefits" },
  "nav.formats": { ru: "Форматы", en: "Formats" },
  "nav.howToStart": { ru: "Как начать", en: "How to start" },
  "nav.login": { ru: "Войти", en: "Log in" },
  "nav.register": { ru: "Регистрация", en: "Sign up" },

  // Hero
  "hero.badge": { ru: "Рекламная платформа нового поколения", en: "Next-generation ad platform" },
  "hero.title1": { ru: "TwinBid — ", en: "TwinBid — " },
  "hero.title2": { ru: "рекламная платформа-агрегатор", en: "advertising aggregator platform" },
  "hero.subtitle": { ru: "Объединяет трафик сотен рекламных сетей. Одна регистрация — и вы получаете доступ к инвентарю более чем", en: "Combines traffic from hundreds of ad networks. One registration — and you get access to inventory from more than" },
  "hero.subtitleSites": { ru: "1 млн сайтов", en: "1M websites" },
  "hero.subtitleEnd": { ru: "через единый кабинет закупки, аналитики и оптимизации.", en: "through a single dashboard for buying, analytics and optimization." },
  "hero.cta": { ru: "Зарегистрироваться", en: "Sign up" },
  "hero.learnMore": { ru: "Узнать больше", en: "Learn more" },
  "hero.statSites": { ru: "Сайтов", en: "Websites" },
  "hero.statNetworks": { ru: "Рекламных сетей", en: "Ad networks" },
  "hero.statSupport": { ru: "Поддержка", en: "Support" },

  // Start Conditions
  "start.conditionsLabel": { ru: "Условия старта", en: "Start conditions" },
  "start.minDeposit": { ru: "Минимальный депозит", en: "Minimum deposit" },
  "start.startSmall": { ru: "Начните с небольшого бюджета и масштабируйтесь", en: "Start with a small budget and scale up" },
  "start.bonusBadge": { ru: "Бонус новым пользователям", en: "Bonus for new users" },
  "start.bonusDesc": { ru: "к бюджету на первое пополнение — зарегистрируйтесь и получите дополнительные средства на запуск кампаний", en: "to your budget on first deposit — sign up and get extra funds to launch campaigns" },
  "start.getBonus": { ru: "Получить бонус", en: "Get bonus" },

  // Benefits
  "benefits.title1": { ru: "Почему ", en: "Why " },
  "benefits.title2": { ru: " — это удобно?", en: " is convenient?" },
  "benefits.subtitle": { ru: "Всё, что нужно для эффективной рекламы, в одном месте", en: "Everything you need for effective advertising, in one place" },
  "benefits.1.title": { ru: "Единый кабинет вместо десятков площадок", en: "One dashboard instead of dozens of platforms" },
  "benefits.1.desc": { ru: "Одна интеграция, один баланс, общая статистика и логика управления кампаниями. Вы экономите время команды и быстрее масштабируете то, что приносит результат.", en: "One integration, one balance, shared statistics and campaign management logic. You save team time and scale what works faster." },
  "benefits.2.title": { ru: "Максимальный инвентарь без потолков по CPM и CPC моделям", en: "Maximum inventory without CPM and CPC model limits" },
  "benefits.2.desc": { ru: "Трафик сотни тысяч сайтов уже доступен на нашей площадке. Остается только зарегистрироваться в пару кликов!", en: "Traffic from hundreds of thousands of sites is already available on our platform. Just sign up in a few clicks!" },
  "benefits.3.title": { ru: "Оплата только за реальный показ", en: "Pay only for real impressions" },
  "benefits.3.desc": { ru: "Мы засчитываем исключительно фактически показанную пользователю рекламу: учитываются только прогруженные страницы, а баннер/креатив должен находиться в зоне видимости пользователя определенное время. Меньше пустых расходов и больше отдачи от рекламы.", en: "We only count ads actually shown to users: only loaded pages count, and the banner/creative must be in the user's viewport for a certain time. Less wasted spend and more ad ROI." },
  "benefits.4.title": { ru: "Встроенная антифрод-система", en: "Built-in anti-fraud system" },
  "benefits.4.desc": { ru: "Система фильтрует подозрительные действия и снижает долю некачественного трафика, тем самым защищая бюджет от мусора.", en: "The system filters suspicious activity and reduces low-quality traffic, protecting your budget from waste." },
  "benefits.5.title": { ru: "AI-оптимизация кампаний: больше эффективности при том же бюджете", en: "AI campaign optimization: more efficiency with the same budget" },
  "benefits.5.desc": { ru: "Алгоритмы оптимизации перераспределяют показы в пользу сегментов и источников с лучшей отдачей, чтобы реклама попадала к максимально релевантной аудитории.", en: "Optimization algorithms redistribute impressions to segments and sources with the best returns, so ads reach the most relevant audience." },

  // Formats
  "formats.title": { ru: "Форматы ", en: "Ad " },
  "formats.title2": { ru: "рекламы", en: "formats" },
  "formats.subtitle": { ru: "Выбирайте оптимальный формат для вашей кампании", en: "Choose the optimal format for your campaign" },
  "formats.popunder.desc": { ru: "Полноэкранная реклама, открывающаяся в новой вкладке. Максимальная видимость и высокий CTR.", en: "Full-screen ad opening in a new tab. Maximum visibility and high CTR." },
  "formats.native.desc": { ru: "Органично интегрированная реклама, которая соответствует стилю площадки. Высокое доверие пользователей.", en: "Natively integrated ads matching the platform's style. High user trust." },
  "formats.banner.desc": { ru: "Классические баннеры различных размеров. Узнаваемость бренда и широкий охват аудитории.", en: "Classic banners of various sizes. Brand awareness and wide audience reach." },
  "formats.push.desc": { ru: "Push-уведомления прямо на странице без подписки. Мгновенное привлечение внимания.", en: "Push notifications right on the page without subscription. Instant attention grabbing." },

  // Steps
  "steps.title1": { ru: "Как ", en: "How to " },
  "steps.title2": { ru: "начать?", en: "get started?" },
  "steps.subtitle": { ru: "Получи доступ к премиальному неограниченному трафику с сотен тысяч сайтов уже сегодня", en: "Get access to premium unlimited traffic from hundreds of thousands of sites today" },
  "steps.1.title": { ru: "Зарегистрируйте кабинет", en: "Register an account" },
  "steps.1.desc": { ru: "Зарегистрируйте кабинет пользователя — это займёт всего пару минут.", en: "Register a user account — it only takes a couple of minutes." },
  "steps.2.title": { ru: "Создайте кампанию", en: "Create a campaign" },
  "steps.2.desc": { ru: "Создайте свою первую кампанию, используя десятки точных таргетингов.", en: "Create your first campaign using dozens of precise targeting options." },
  "steps.3.title": { ru: "Пополните баланс", en: "Top up your balance" },
  "steps.3.desc": { ru: "Напишите менеджеру и пополните баланс. Получите +25% от суммы пополнения бонусом на первый депозит. Минимум — $100.", en: "Contact a manager and top up your balance. Get +25% of the deposit amount as a bonus on your first deposit. Minimum — $100." },
  "steps.4.title": { ru: "Масштабируйте", en: "Scale up" },
  "steps.4.desc": { ru: "Масштабируйте свои кампании с защитой от фрода и AI-оптимизацией.", en: "Scale your campaigns with fraud protection and AI optimization." },

  // CTA
  "cta.badge": { ru: "Начните уже сегодня", en: "Start today" },
  "cta.title": { ru: "Получи доступ к премиальному неограниченному трафику", en: "Get access to premium unlimited traffic" },
  "cta.subtitle": { ru: "Присоединяйтесь к TwinBid и получите +25% на первый депозит. Сотни тысяч сайтов уже ждут вашу рекламу.", en: "Join TwinBid and get +25% on your first deposit. Hundreds of thousands of sites are already waiting for your ads." },
  "cta.register": { ru: "Зарегистрироваться", en: "Sign up" },
  "cta.contact": { ru: "Связаться с нами", en: "Contact us" },
  "cta.trust1": { ru: "✓ Минимальный депозит $100", en: "✓ Minimum deposit $100" },
  "cta.trust2": { ru: "✓ +25% бонус", en: "✓ +25% bonus" },
  "cta.trust3": { ru: "✓ Мгновенный старт", en: "✓ Instant start" },

  // Footer
  "footer.privacy": { ru: "Политика конфиденциальности", en: "Privacy Policy" },
  "footer.terms": { ru: "Условия использования", en: "Terms of Service" },
  "footer.docs": { ru: "Документация", en: "Documentation" },

  // Auth Dialog
  "auth.login": { ru: "Вход", en: "Log in" },
  "auth.register": { ru: "Регистрация", en: "Sign up" },
  "auth.email": { ru: "Email", en: "Email" },
  "auth.password": { ru: "Пароль", en: "Password" },
  "auth.name": { ru: "Имя (опционально)", en: "Name (optional)" },
  "auth.phone": { ru: "Телефон", en: "Phone" },
  "auth.confirmPassword": { ru: "Подтвердите пароль", en: "Confirm password" },
  "auth.loginBtn": { ru: "Войти", en: "Log in" },
  "auth.registerBtn": { ru: "Зарегистрироваться", en: "Sign up" },
  "auth.passwordMismatch": { ru: "Пароли не совпадают", en: "Passwords do not match" },
  "auth.passwordTooShort": { ru: "Пароль должен содержать минимум 6 символов", en: "Password must be at least 6 characters" },
  "auth.checkEmail": { ru: "Проверьте вашу почту для подтверждения аккаунта", en: "Check your email to confirm your account" },

  // Dashboard Sidebar
  "sidebar.overview": { ru: "Обзор", en: "Overview" },
  "sidebar.campaigns": { ru: "Кампании", en: "Campaigns" },
  "sidebar.statistics": { ru: "Статистика", en: "Statistics" },
  "sidebar.balance": { ru: "Баланс", en: "Balance" },
  "sidebar.settings": { ru: "Настройки", en: "Settings" },
  "sidebar.logout": { ru: "Выйти", en: "Log out" },

  // Dashboard Header
  "header.search": { ru: "Поиск кампаний...", en: "Search campaigns..." },
  "header.notifications": { ru: "Уведомления", en: "Notifications" },
  "header.noNotifications": { ru: "Нет уведомлений", en: "No notifications" },
  "header.advertiser": { ru: "Рекламодатель", en: "Advertiser" },
  "header.manager": { ru: "Персональный менеджер", en: "Personal manager" },

  // Dashboard Overview
  "overview.recentCampaigns": { ru: "Кампании", en: "Campaigns" },
  "overview.id": { ru: "ID", en: "ID" },
  "overview.name": { ru: "Название", en: "Name" },
  "overview.status": { ru: "Статус", en: "Status" },
  "overview.impressions": { ru: "Показы", en: "Impressions" },
  "overview.spent": { ru: "Расход", en: "Spent" },

  // StatsCards (overview)
  "statsCards.impressions": { ru: "Показы", en: "Impressions" },
  "statsCards.clicks": { ru: "Клики", en: "Clicks" },
  "statsCards.ctr": { ru: "CTR", en: "CTR" },

  // BalanceCard (overview)
  "balanceCard.title": { ru: "Баланс", en: "Balance" },
  "balanceCard.daysLeft": { ru: "≈ 14 дней показов", en: "≈ 14 days of impressions" },
  "balanceCard.topUp": { ru: "Пополнить", en: "Top up" },

  // Statuses
  "status.active": { ru: "Активна", en: "Active" },
  "status.paused": { ru: "На паузе", en: "Paused" },
  "status.draft": { ru: "Черновик", en: "Draft" },
  "status.completed": { ru: "Завершена", en: "Completed" },
  "status.moderation": { ru: "На модерации", en: "In moderation" },

  // Balance page
  "balance.title": { ru: "Баланс и платежи", en: "Balance & Payments" },
  "balance.subtitle": { ru: "Пополнение USDT", en: "USDT top-up" },
  "balance.current": { ru: "Текущий баланс", en: "Current balance" },
  "balance.spentToday": { ru: "Потрачено сегодня", en: "Spent today" },
  "balance.spentWeek": { ru: "Потрачено за неделю", en: "Spent this week" },
  "balance.estimatedDays": { ru: "Хватит примерно на", en: "Estimated for" },
  "balance.days": { ru: "дней", en: "days" },
  "balance.topUp": { ru: "Пополнить баланс", en: "Top up balance" },
  "balance.amount": { ru: "Сумма", en: "Amount" },
  "balance.otherAmount": { ru: "Другая сумма", en: "Custom amount" },
  "balance.network": { ru: "Сеть", en: "Network" },
  "balance.topUpBtn": { ru: "Пополнить", en: "Top up" },
  "balance.minAmount": { ru: "Минимальная сумма — $100. Бонус +25% на первый депозит.", en: "Minimum amount — $100. +25% bonus on first deposit." },
  "balance.promo.label": { ru: "Промокод", en: "Promo code" },
  "balance.promo.placeholder": { ru: "Введите промокод", en: "Enter promo code" },
  "balance.promo.apply": { ru: "Применить", en: "Apply" },
  "balance.promo.remove": { ru: "Убрать", en: "Remove" },
  "balance.promo.applied": { ru: "Промокод применён! Бонус +{percent}% к пополнению", en: "Promo code applied! +{percent}% bonus to deposit" },
  "balance.promo.invalid": { ru: "Промокод не найден", en: "Promo code not found" },
  "balance.promo.alreadyUsed": { ru: "Вы уже использовали этот промокод", en: "You have already used this promo code" },
  "balance.promo.active": { ru: "Промокод {code} — бонус +{percent}%", en: "Promo code {code} — +{percent}% bonus" },
  "balance.promo.bonusShort": { ru: "бонус", en: "bonus" },
  "balance.paymentTitle": { ru: "Оплата", en: "Payment" },
  "balance.paymentDesc": { ru: "Переведите точную сумму на указанный адрес и вставьте хэш транзакции", en: "Transfer the exact amount to the specified address and paste the transaction hash" },
  "balance.topUpAmount": { ru: "Сумма пополнения:", en: "Top-up amount:" },
  "balance.walletAddress": { ru: "Адрес кошелька", en: "Wallet address" },
  "balance.transferNote": { ru: "Переведите точную сумму. Средства зачислятся после подтверждения сети.", en: "Transfer the exact amount. Funds will be credited after network confirmation." },
  "balance.txHash": { ru: "Хэш транзакции *", en: "Transaction hash *" },
  "balance.submit": { ru: "Отправить", en: "Submit" },
  "balance.cancelPayment": { ru: "Отменить оплату", en: "Cancel payment" },
  "balance.supportText": { ru: "По вопросам оплаты обращайтесь к вашему персональному менеджеру.", en: "For payment inquiries, please contact your personal manager." },
  "balance.inTelegram": { ru: "", en: "" },
  "balance.history": { ru: "История операций", en: "Transaction history" },
  "balance.date": { ru: "Дата", en: "Date" },
  "balance.description": { ru: "Описание", en: "Description" },
  "balance.amountCol": { ru: "Сумма", en: "Amount" },
  "balance.statusCol": { ru: "Статус", en: "Status" },
  "balance.topUpVia": { ru: "Пополнение", en: "Top-up" },
  "balance.completed": { ru: "Выполнено", en: "Completed" },
  "balance.pending": { ru: "В обработке", en: "Processing" },
  "balance.noTransactions": { ru: "Нет операций", en: "No transactions" },

  // Balance toasts
  "balance.toast.pendingExists": { ru: "У вас есть незавершённая оплата. Завершите или отмените её.", en: "You have a pending payment. Complete or cancel it." },
  "balance.toast.enterHash": { ru: "Введите хэш транзакции", en: "Enter transaction hash" },
  "balance.toast.paymentSent": { ru: "Платёж отправлен на проверку. Средства появятся на балансе в ближайшее время.", en: "Payment sent for review. Funds will appear on your balance shortly." },
  "balance.toast.paymentSupport": { ru: "По вопросам оплаты обращайтесь к вашему персональному менеджеру", en: "For payment inquiries please contact your personal manager" },
  "balance.toast.paymentCanceled": { ru: "Оплата отменена", en: "Payment cancelled" },
  "balance.toast.addressCopied": { ru: "Адрес скопирован", en: "Address copied" },
  "balance.toast.notCompleted": { ru: "Оплата не завершена. Проверьте уведомления.", en: "Payment not completed. Check notifications." },
  "balance.notif.notCompleted": { ru: "Оплата не завершена", en: "Payment not completed" },
  "balance.notif.noHash": { ru: "Вы не отправили хэш транзакции на", en: "You did not submit the transaction hash for" },
  "balance.notif.completePayment": { ru: "Завершить оплату", en: "Complete payment" },
  "balance.disabledReason": { ru: "У вас есть незавершённая оплата — завершите её или отмените, чтобы создать новую.", en: "You have an unfinished payment — complete or cancel it to start a new one." },
  "balance.notif.cancelConfirmTitle": { ru: "Отменить транзакцию?", en: "Cancel transaction?" },
  "balance.notif.cancelConfirmDesc": { ru: "Вы уверены, что хотите отменить незавершённую транзакцию? Это действие нельзя будет отменить.", en: "Are you sure you want to cancel the pending transaction? This action cannot be undone." },
  "balance.notif.cancelConfirmYes": { ru: "Да, отменить", en: "Yes, cancel" },
  "balance.notif.cancelConfirmNo": { ru: "Нет, оставить", en: "No, keep it" },
  "balance.notif.lowBalance": { ru: "Низкий баланс", en: "Low balance" },
  "balance.notif.lowBalanceDesc": { ru: "Ваш баланс составляет", en: "Your balance is" },
  "balance.notif.recommend": { ru: "Рекомендуем пополнить счёт.", en: "We recommend topping up." },
  "balance.notif.topUp": { ru: "Пополнить", en: "Top up" },
  "balance.notif.paymentSuccess": { ru: "Платёж отправлен", en: "Payment submitted" },
  "balance.notif.paymentSuccessDesc": { ru: "Платёж на сумму ${amount} отправлен на проверку. По вопросам обратитесь к вашему менеджеру.", en: "Payment of ${amount} sent for review. For any questions please contact your manager." },

  // Statistics
  "stats.title": { ru: "Статистика", en: "Statistics" },
  "stats.subtitle": { ru: "Подробная аналитика по вашим кампаниям", en: "Detailed analytics for your campaigns" },
  "stats.campaigns": { ru: "Кампании", en: "Campaigns" },
  "stats.allCampaigns": { ru: "Все кампании", en: "All campaigns" },
  "stats.selected": { ru: "Выбрано:", en: "Selected:" },
  "stats.period": { ru: "Период", en: "Period" },
  "stats.selectPeriod": { ru: "Выберите период", en: "Select period" },
  "stats.selectCampaign": { ru: "Выберите кампанию", en: "Select campaign" },
  "stats.selectAll": { ru: "Все кампании", en: "All campaigns" },
  "stats.refresh": { ru: "Обновить", en: "Refresh" },
  "stats.refreshed": { ru: "Статистика обновлена", en: "Statistics refreshed" },
  "stats.impressions": { ru: "Показы", en: "Impressions" },
  "stats.clicks": { ru: "Клики", en: "Clicks" },
  "stats.ctr": { ru: "CTR", en: "CTR" },
  "stats.spent": { ru: "Расход", en: "Spent" },
  "stats.chartTitle": { ru: "Динамика показов", en: "Impressions dynamics" },
  "stats.byDates": { ru: "По датам", en: "By dates" },
  "stats.byHours": { ru: "По часам", en: "By hours" },
  "stats.byBrowsers": { ru: "По браузерам", en: "By browsers" },
  "stats.bySiteId": { ru: "По SiteID", en: "By SiteID" },
  "stats.byDevices": { ru: "По устройствам", en: "By devices" },
  "stats.byOS": { ru: "По ОС", en: "By OS" },
  "stats.byCountry": { ru: "По странам", en: "By countries" },
  "stats.date": { ru: "Дата", en: "Date" },
  "stats.dateAndHour": { ru: "Дата и час", en: "Date and hour" },
  "stats.browser": { ru: "Браузер", en: "Browser" },
  "stats.device": { ru: "Устройство", en: "Device" },
  "stats.os": { ru: "ОС", en: "OS" },
  "stats.country": { ru: "Страна", en: "Country" },
  "stats.total": { ru: "Итого", en: "Total" },
  "stats.noData": { ru: "Нет данных для выбранных фильтров", en: "No data for selected filters" },
  "stats.selectCampaignAndPeriod": { ru: "Выберите кампанию и период для просмотра статистики", en: "Select a campaign and period to view statistics" },
  "stats.filters": { ru: "Фильтры", en: "Filters" },
  "stats.filterCountry": { ru: "Страна", en: "Country" },
  "stats.filterBrowser": { ru: "Браузер", en: "Browser" },
  "stats.filterDevice": { ru: "Устройство", en: "Device" },
  "stats.filterOS": { ru: "ОС", en: "OS" },
  "stats.allValues": { ru: "Все", en: "All" },
  "stats.clearFilters": { ru: "Сбросить фильтры", en: "Clear filters" },
  "stats.creatives": { ru: "Креативы", en: "Creatives" },
  "stats.selectCreative": { ru: "Выберите креатив", en: "Select creative" },
  "stats.allCreatives": { ru: "Все креативы", en: "All creatives" },
  "stats.today": { ru: "Сегодня", en: "Today" },
  "stats.yesterday": { ru: "Вчера", en: "Yesterday" },
   "stats.week": { ru: "7 дней", en: "7 days" },
   "stats.month": { ru: "30 дней", en: "30 days" },
  "stats.chartTitleHours": { ru: "Динамика по часам", en: "Hourly dynamics" },

  // Campaigns page
  "campaigns.title": { ru: "Кампании", en: "Campaigns" },
  "campaigns.subtitle": { ru: "Управляйте вашими рекламными кампаниями", en: "Manage your ad campaigns" },
  "campaigns.create": { ru: "Создать кампанию", en: "Create campaign" },
  "campaigns.searchPlaceholder": { ru: "Поиск по названию или ID...", en: "Search by name or ID..." },
  "campaigns.allStatuses": { ru: "Все статусы", en: "All statuses" },
  "campaigns.activeFilter": { ru: "Активные", en: "Active" },
  "campaigns.pausedFilter": { ru: "На паузе", en: "Paused" },
  "campaigns.draftsFilter": { ru: "Черновики", en: "Drafts" },
  "campaigns.moderationFilter": { ru: "На модерации", en: "In moderation" },
  "campaigns.completedFilter": { ru: "Завершённые", en: "Completed" },
  "campaigns.total": { ru: "Всего", en: "Total" },
  "campaigns.activeCount": { ru: "Активных", en: "Active" },
  "campaigns.budget": { ru: "Бюджет", en: "Budget" },
  "campaigns.format": { ru: "Формат", en: "Format" },
  "campaigns.view": { ru: "Просмотр", en: "View" },
  "campaigns.edit": { ru: "Редактировать", en: "Edit" },
  "campaigns.copy": { ru: "Копировать", en: "Copy" },
  "campaigns.copyPostfix": { ru: "(копия)", en: "(copy)" },
  "campaigns.pause": { ru: "Приостановить", en: "Pause" },
  "campaigns.start": { ru: "Запустить", en: "Start" },
  "campaigns.finishCreation": { ru: "Завершить создание", en: "Finish creation" },
  "campaigns.delete": { ru: "Удалить", en: "Delete" },
  "campaigns.notFound": { ru: "Кампании не найдены", en: "No campaigns found" },
  "campaigns.deleteConfirm": { ru: "Удалить кампанию?", en: "Delete campaign?" },
  "campaigns.deleteDesc": { ru: "Это действие нельзя отменить. Кампания будет удалена навсегда.", en: "This action cannot be undone. The campaign will be permanently deleted." },
  "campaigns.cancel": { ru: "Отмена", en: "Cancel" },
  "campaigns.started": { ru: "Кампания запущена", en: "Campaign started" },
  "campaigns.paused": { ru: "Кампания приостановлена", en: "Campaign paused" },
  "campaigns.deleted": { ru: "Кампания удалена", en: "Campaign deleted" },
  "campaigns.copied": { ru: "Кампания скопирована", en: "Campaign copied" },
  "campaigns.restart": { ru: "Перезапустить", en: "Restart" },
  "campaigns.restarted": { ru: "Обновите даты и сохраните для перезапуска", en: "Update dates and save to restart" },
  "campaigns.cancelModeration": { ru: "Отменить модерацию", en: "Cancel moderation" },
  "campaigns.moderationCanceled": { ru: "Модерация отменена", en: "Moderation cancelled" },
  "campaigns.draftIncomplete": { ru: "Завершите настройку кампании перед запуском", en: "Complete campaign setup before launching" },

  // Settings
  "settings.title": { ru: "Настройки", en: "Settings" },
  "settings.subtitle": { ru: "Управление аккаунтом и предпочтениями", en: "Account and preferences management" },
  "settings.profile": { ru: "Профиль", en: "Profile" },
  "settings.notifications": { ru: "Уведомления", en: "Notifications" },
  "settings.security": { ru: "Безопасность", en: "Security" },
  "settings.name": { ru: "Имя", en: "Name" },
  "settings.email": { ru: "Email", en: "Email" },
  "settings.telegram": { ru: "Telegram", en: "Telegram" },
  "settings.timezone": { ru: "Часовой пояс", en: "Timezone" },
  "settings.save": { ru: "Сохранить", en: "Save" },
  "settings.saved": { ru: "Настройки сохранены", en: "Settings saved" },
  "settings.emailNotifications": { ru: "Email-уведомления", en: "Email notifications" },
  "settings.campaignStatus": { ru: "Изменения статуса кампаний", en: "Campaign status changes" },
  "settings.lowBalance": { ru: "Низкий баланс", en: "Low balance" },
  "settings.balanceThreshold": { ru: "Порог баланса для уведомления", en: "Balance threshold for notification" },
  "settings.currentPassword": { ru: "Текущий пароль", en: "Current password" },
  "settings.newPassword": { ru: "Новый пароль", en: "New password" },
  "settings.repeatPassword": { ru: "Повторите пароль", en: "Repeat password" },
  "settings.changePassword": { ru: "Сменить пароль", en: "Change password" },
  "settings.passwordUpdated": { ru: "Пароль обновлён", en: "Password updated" },

  // Create Campaign
  "create.title": { ru: "Создание кампании", en: "Create campaign" },
  "create.step": { ru: "Шаг", en: "Step" },
  "create.of": { ru: "из", en: "of" },
  "create.step1": { ru: "Основная информация и креатив", en: "Basic info and creative" },
  "create.step2": { ru: "Таргетинг и списки", en: "Targeting and lists" },
  "create.step3": { ru: "Бюджет и расписание", en: "Budget and schedule" },
  "create.trafficType": { ru: "Тип трафика", en: "Traffic type" },
  "create.mainstream": { ru: "Мейнстрим", en: "Mainstream" },
  "create.adult": { ru: "Адалт", en: "Adult" },
  "create.mixed": { ru: "Миксед (смешанный)", en: "Mixed" },
  "create.trafficTypeHint": { ru: "Где вы хотите показывать кампанию", en: "Where do you want to show the campaign" },
  "create.campaignName": { ru: "Название кампании *", en: "Campaign name *" },
  "create.campaignNamePlaceholder": { ru: "Например: Летняя распродажа", en: "E.g.: Summer sale" },
  "create.adFormat": { ru: "Формат рекламы *", en: "Ad format *" },
  "create.selectFormat": { ru: "Выберите формат", en: "Select format" },
  "create.creativeFor": { ru: "Креатив для формата", en: "Creative for format" },
  "create.creatives": { ru: "Креативы", en: "Creatives" },
  "create.creative": { ru: "Креатив", en: "Creative" },
  "create.addCreative": { ru: "Добавить креатив", en: "Add creative" },
  "create.bannerSize": { ru: "Размер баннера", en: "Banner size" },
  "create.selectBannerSize": { ru: "Выберите размер", en: "Select size" },
  "create.brandName": { ru: "Название бренда", en: "Brand name" },
  "create.brandNamePlaceholder": { ru: "Например: MyBrand", en: "E.g.: MyBrand" },
  "create.optional": { ru: "опционально", en: "optional" },
  "create.creativeName": { ru: "Название креатива", en: "Creative name" },
  "create.creativeNamePlaceholder": { ru: "Например: Баннер лето 2024", en: "E.g.: Summer banner 2024" },
  "create.creativeNameHint": { ru: "Используется для фильтрации по креативам в статистике", en: "Used for filtering by creatives in statistics" },
  "create.titlePlaceholder": { ru: "Заголовок объявления", en: "Ad headline" },
  "create.descriptionPlaceholder": { ru: "Описание объявления...", en: "Ad description..." },
  "create.required": { ru: "Обязательное поле", en: "Required field" },
  "create.selectFormatError": { ru: "Выберите формат", en: "Select format" },
  "create.back": { ru: "Назад", en: "Back" },
  "create.next": { ru: "Далее", en: "Next" },
  "create.createBtn": { ru: "Создать кампанию", en: "Create campaign" },
  "create.created": { ru: "Кампания создана и отправлена на модерацию!", en: "Campaign created and sent for moderation!" },
  "create.draftSaved": { ru: "Кампания сохранена в черновики", en: "Campaign saved as draft" },
  "create.draftSavedDesc": { ru: "Создание кампании не завершено. Вы можете продолжить редактирование в любое время.", en: "Campaign creation is not finished. You can continue editing at any time." },
  "create.uploadImage": { ru: "Загрузить изображение", en: "Upload image" },
  "create.imageUploaded": { ru: "Изображение загружено", en: "Image uploaded" },
  "create.imageFormatError": { ru: "Неверный формат. Поддерживаются: PNG, JPG, JPEG", en: "Invalid format. Supported: PNG, JPG, JPEG" },
  "create.imageFormatHint": { ru: "Поддерживаемые форматы: PNG, JPG, JPEG", en: "Supported formats: PNG, JPG, JPEG" },
  "create.creativeTitle": { ru: "Заголовок", en: "Title" },
  "create.creativeDescription": { ru: "Описание", en: "Description" },
  "create.creativeUrl": { ru: "Ссылка", en: "URL" },
  "create.urlMacrosHint": { ru: "Нажмите для добавления макросов отслеживания:", en: "Click to add tracking macros:" },
  "create.endDateError": { ru: "Дата окончания не может быть раньше сегодняшнего дня", en: "End date cannot be earlier than today" },
  "create.endDateRequired": { ru: "Укажите корректные даты для завершения создания", en: "Specify valid dates to complete creation" },

  // Edit Campaign
  "edit.title": { ru: "Редактирование кампании", en: "Edit campaign" },
  "edit.notFound": { ru: "Кампания не найдена", en: "Campaign not found" },
  "edit.general": { ru: "Основное", en: "General" },
  "edit.targeting": { ru: "Таргетинг и списки", en: "Targeting and lists" },
  "edit.budget": { ru: "Бюджет", en: "Budget" },
  "edit.name": { ru: "Название", en: "Name" },
  "edit.formatLabel": { ru: "Формат рекламы", en: "Ad format" },
  "edit.formatLocked": { ru: "Формат нельзя изменить после создания", en: "Format cannot be changed after creation" },
  "edit.moderationWarning": { ru: "Изменён контент кампании — после сохранения она будет отправлена на модерацию", en: "Campaign content changed — it will be sent for moderation after saving" },
  "edit.save": { ru: "Сохранить", en: "Save" },
  "edit.savedModeration": { ru: "Кампания сохранена и отправлена на модерацию", en: "Campaign saved and sent for moderation" },
  "edit.saved": { ru: "Кампания сохранена", en: "Campaign saved" },

  // Targeting Section
  "targeting.description": { ru: "Для каждого параметра выберите режим: Whitelist (только эти значения) или Blacklist (исключить эти значения)", en: "For each parameter choose a mode: Whitelist (only these values) or Blacklist (exclude these values)" },
  "targeting.country": { ru: "Страны", en: "Countries" },
  "targeting.deviceType": { ru: "Тип устройства", en: "Device type" },
  "targeting.os": { ru: "ОС", en: "OS" },
  "targeting.browser": { ru: "Браузер", en: "Browser" },
  "targeting.schedule": { ru: "Расписание показов", en: "Display schedule" },
  "targeting.sites": { ru: "ID сайта", en: "Site ID" },
  "targeting.ip": { ru: "IP-адреса", en: "IP addresses" },
  "targeting.language": { ru: "Язык", en: "Language" },
  "targeting.off": { ru: "Выкл", en: "Off" },
  "targeting.on": { ru: "Вкл", en: "On" },
  "targeting.freeTextPlaceholder": { ru: "Введите значение...", en: "Enter value..." },
  "targeting.autocompletePlaceholder": { ru: "Начните вводить...", en: "Start typing..." },
  "targeting.addCustom": { ru: "Добавить", en: "Add" },
  "targeting.sitesHint": { ru: "Добавляйте ID сайтов по одному или через запятую (например: '12345','abdjhx')", en: "Add site IDs one by one or separated by commas (e.g.: '12345','abdjhx')" },
  "targeting.sitesFormatError": { ru: "Неверный формат. Используйте формат без пробелов и кавычек", en: "Invalid format. No spaces or quotes allowed" },
  "targeting.scheduleHint": { ru: "Кликните или протяните мышью для выбора часов. Кликните на день/час для выделения целого ряда/столбца", en: "Click or drag to select hours. Click a day/hour header to toggle an entire row/column" },
  "targeting.selectAll": { ru: "Все", en: "All" },
  "targeting.deselectAll": { ru: "Снять", en: "Clear" },
  "targeting.ipHint": { ru: "Введите IPv4 адреса через запятую", en: "Enter IPv4 addresses, comma-separated" },
  "targeting.ipFormatError": { ru: "Неверный формат IP. Допускаются только IPv4 адреса", en: "Invalid IP format. Only IPv4 addresses are allowed" },

  // Days of week
  "day.monday": { ru: "Понедельник", en: "Monday" },
  "day.tuesday": { ru: "Вторник", en: "Tuesday" },
  "day.wednesday": { ru: "Среда", en: "Wednesday" },
  "day.thursday": { ru: "Четверг", en: "Thursday" },
  "day.friday": { ru: "Пятница", en: "Friday" },
  "day.saturday": { ru: "Суббота", en: "Saturday" },
  "day.sunday": { ru: "Воскресенье", en: "Sunday" },

  // Budget Section
  "budget.totalBudget": { ru: "Общий бюджет *", en: "Total budget *" },
  "budget.totalBudgetHint": { ru: "Обязательное поле. Минимум $1", en: "Required field. Minimum $1" },
  "budget.dailyBudget": { ru: "Дневной бюджет", en: "Daily budget" },
  "budget.dailyBudgetPlaceholder": { ru: "Без ограничений", en: "No limit" },
  "budget.trafficType": { ru: "Тип трафика *", en: "Traffic type *" },
  "budget.trafficCommon": { ru: "Фильтрация на стороне партнёра + фильтрация TwinBid. Базовый сегмент: хорошие объёмы при контроле качества.", en: "Partner-side filtering + TwinBid filtering. Base segment: good volumes with quality control." },
  "budget.trafficHigh": { ru: "Жёсткие фильтры у партнёра + усиленная фильтрация TwinBid. Меньше объёмов, но минимальная вероятность фрода.", en: "Strict partner filters + enhanced TwinBid filtering. Lower volumes but minimal fraud probability." },
  "budget.trafficUltra": { ru: "Фильтры партнёра + фильтрация TwinBid + сторонние fraud-checker системы. Максимальная глубина проверки и защита от фрода.", en: "Partner filters + TwinBid filtering + third-party fraud-checker systems. Maximum verification depth and fraud protection." },
  "budget.pricingModel": { ru: "Модель оплаты *", en: "Pricing model *" },
  "budget.cpmLabel": { ru: "CPM (стоимость за 1000 показов)", en: "CPM (cost per 1000 impressions)" },
  "budget.cpcLabel": { ru: "CPC (стоимость за клик)", en: "CPC (cost per click)" },
  "budget.min": { ru: "Мин", en: "Min" },
  "budget.recommended": { ru: "Рекомендованная", en: "Recommended" },
  "budget.belowMin": { ru: "Ставка ниже минимальной", en: "Bid is below minimum" },
  "budget.belowRec": { ru: "Ставка ниже рекомендованной — может быть мало показов", en: "Bid is below recommended — may result in few impressions" },
  "budget.startDate": { ru: "Дата начала", en: "Start date" },
  "budget.endDate": { ru: "Дата окончания", en: "End date" },
  "budget.endDateError": { ru: "Дата окончания не может быть раньше сегодняшнего дня", en: "End date cannot be earlier than today" },
  "budget.evenSpend": { ru: "Равномерное распределение бюджета", en: "Even budget distribution" },
  "budget.evenSpendTooltip": { ru: "Бюджет кампании будет равномерно распределён на весь период работы кампании, чтобы избежать преждевременного расходования средств.", en: "The campaign budget will be evenly distributed across the entire campaign period to avoid premature spending." },

  // Legacy components
  "legacy.campaigns": { ru: "Рекламные кампании", en: "Ad campaigns" },
  "legacy.createCampaign": { ru: "Создать кампанию", en: "Create campaign" },
  "legacy.campaignName": { ru: "Название", en: "Name" },
  "legacy.status": { ru: "Статус", en: "Status" },
  "legacy.format": { ru: "Формат", en: "Format" },
  "legacy.budget": { ru: "Бюджет", en: "Budget" },
  "legacy.spent": { ru: "Потрачено", en: "Spent" },
  "legacy.impressions": { ru: "Показы", en: "Impressions" },
  "legacy.clicks": { ru: "Клики", en: "Clicks" },
  "legacy.view": { ru: "Просмотр", en: "View" },
  "legacy.edit": { ru: "Редактировать", en: "Edit" },
  "legacy.pause": { ru: "Приостановить", en: "Pause" },
  "legacy.start": { ru: "Запустить", en: "Start" },
  "legacy.delete": { ru: "Удалить", en: "Delete" },
  "legacy.topUpTitle": { ru: "Пополнение баланса", en: "Top up balance" },
  "legacy.topUpDesc": { ru: "Выберите сумму и способ оплаты", en: "Choose amount and payment method" },
  "legacy.topUpAmount": { ru: "Сумма пополнения", en: "Top-up amount" },
  "legacy.otherAmount": { ru: "Другая сумма", en: "Custom amount" },
  "legacy.paymentMethod": { ru: "Способ оплаты", en: "Payment method" },
  "legacy.bankCard": { ru: "Банковская карта", en: "Bank card" },
  "legacy.invoice": { ru: "Счёт для юрлица", en: "Invoice for company" },
  "legacy.eWallet": { ru: "Электронный кошелёк", en: "E-wallet" },
  "legacy.pay": { ru: "Оплатить", en: "Pay" },
  "legacy.minAmount": { ru: "Минимальная сумма пополнения — 1 000 ₽", en: "Minimum top-up amount — 1,000 ₽" },
  "legacy.createTitle": { ru: "Создание кампании", en: "Create campaign" },
  "legacy.step": { ru: "Шаг", en: "Step" },
  "legacy.basicInfo": { ru: "Основная информация", en: "Basic information" },
  "legacy.targeting": { ru: "Таргетинг", en: "Targeting" },
  "legacy.budgetSchedule": { ru: "Бюджет и расписание", en: "Budget and schedule" },
  "legacy.campaignNameLabel": { ru: "Название кампании", en: "Campaign name" },
  "legacy.campaignNamePlaceholder": { ru: "Например: Летняя распродажа", en: "E.g.: Summer sale" },
  "legacy.adFormat": { ru: "Формат рекламы", en: "Ad format" },
  "legacy.selectFormat": { ru: "Выберите формат", en: "Select format" },
  "legacy.descLabel": { ru: "Описание (опционально)", en: "Description (optional)" },
  "legacy.descPlaceholder": { ru: "Опишите цели кампании...", en: "Describe campaign goals..." },
  "legacy.geography": { ru: "География", en: "Geography" },
  "legacy.selectRegion": { ru: "Выберите регион", en: "Select region" },
  "legacy.audienceAge": { ru: "Возраст аудитории", en: "Audience age" },
  "legacy.from": { ru: "От", en: "From" },
  "legacy.to": { ru: "До", en: "To" },
  "legacy.interests": { ru: "Интересы", en: "Interests" },
  "legacy.selectCategories": { ru: "Выберите категории", en: "Select categories" },
  "legacy.dailyBudget": { ru: "Дневной бюджет", en: "Daily budget" },
  "legacy.dailyMin": { ru: "Минимум 1 000 ₽ в день", en: "Minimum 1,000 ₽ per day" },
  "legacy.totalBudgetOptional": { ru: "Общий бюджет (опционально)", en: "Total budget (optional)" },
  "legacy.noLimit": { ru: "Без ограничений", en: "No limit" },
  "legacy.startDate": { ru: "Дата начала", en: "Start date" },
  "legacy.endDate": { ru: "Дата окончания", en: "End date" },
  "legacy.back": { ru: "Назад", en: "Back" },
  "legacy.next": { ru: "Далее", en: "Next" },
  "legacy.createBtn": { ru: "Создать кампанию", en: "Create campaign" },

  // NotFound
  "notFound.title": { ru: "404", en: "404" },
  "notFound.message": { ru: "Страница не найдена", en: "Page not found" },
  "notFound.back": { ru: "Вернуться на главную", en: "Return to Home" },


  // Vertical
  "create.vertical": { ru: "Вертикаль", en: "Vertical" },

  // Cashback
  "cashback.title": { ru: "Система кэшбэка", en: "Cashback system" },
  "cashback.subtitle": { ru: "Получайте возврат бюджета за объём трат — чем больше тратите, тем больше экономите", en: "Get budget returns based on spend volume — the more you spend, the more you save" },
  "cashback.perWeek": { ru: "в неделю", en: "per week" },
  "cashback.from": { ru: "от", en: "from" },

  // Budget date picker
  "budget.selectDate": { ru: "Выберите дату", en: "Select date" },

  // Edit campaign errors
  "edit.errorBudgetMin": { ru: "Бюджет кампании должен быть не менее $1", en: "Campaign budget must be at least $1" },
  "edit.restartedActive": { ru: "Кампания перезапущена", en: "Campaign restarted" },

  // Payment
  "balance.paymentMethod": { ru: "Способ оплаты", en: "Payment method" },

  // Settings - campaign budget alert
  "settings.campaignBudgetAlert": { ru: "Бюджет кампании", en: "Campaign budget" },
  "settings.campaignBudgetAlertDesc": { ru: "Уведомление при остатке менее 10% бюджета кампании", en: "Notification when less than 10% of campaign budget remains" },

  // Budget notification
  "notif.campaignBudgetLow": { ru: "Бюджет кампании заканчивается", en: "Campaign budget running low" },
  "notif.budgetRemaining": { ru: "бюджета осталось", en: "budget remaining" },
};

interface LanguageContextType {
  lang: Lang;
  setLang: (lang: Lang) => void;
  t: (key: string) => string;
}

const LanguageContext = createContext<LanguageContextType | null>(null);

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLang] = useState<Lang>(() => {
    try {
      const stored = localStorage.getItem("twinbid_lang");
      if (stored === "en" || stored === "ru") return stored;
    } catch {}
    return "ru";
  });

  const handleSetLang = useCallback((newLang: Lang) => {
    setLang(newLang);
    localStorage.setItem("twinbid_lang", newLang);
  }, []);

  const t = useCallback((key: string): string => {
    const entry = translations[key];
    if (!entry) return key;
    return entry[lang] || entry.ru || key;
  }, [lang]);

  return (
    <LanguageContext.Provider value={{ lang, setLang: handleSetLang, t }}>
      {children}
    </LanguageContext.Provider>
  );
}

export function useLanguage() {
  const ctx = useContext(LanguageContext);
  if (!ctx) throw new Error("useLanguage must be used within LanguageProvider");
  return ctx;
}
