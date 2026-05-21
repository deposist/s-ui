# Changelog (Русский)

В этом файле зафиксированы все значимые изменения проекта.

Это русскоязычный changelog. Английская версия — в `CHANGELOG-EN.md`,
китайская — в `CHANGELOG-ZH.md`.

## Unreleased

## [1.5.3-beta] — 2026-05-20 — агрегированные исправления + upstream-парити (#1114)

### Сводка multi-chat поставки (P0-P5)

#### Безопасность

- [P0] Усилена SSRF-фильтрация и повторная проверка адреса на этапе dial;
  ужесточена валидация путей/симлинков при restore из бэкапа.
- [P1] Усилен жизненный цикл CSRF/session, включая обновление токена после
  logout/logout-all и более строгую обработку WS-токенов.
- [P2] Расширены защитные проверки secret/settings и guardrails миграций.
- [P3] Добавлен аудит listen-fallback и выровнена надёжность restart-пути.

#### Надёжность / целостность данных

- [P0] Закрыты race-сценарии в tracker/session options/audit writer.
- [P1] Стабилизированы realtime fallback и frontend unit-test harness.
- [P2] Добавлены reset hooks, tracker wait guards и проверки миграций с
  foreign keys.
- [P3] Унифицировано планирование рестартов и уменьшены глобальные
  side-effects через начальный DI-срез.
- [P4] Оставшиеся service runtime globals вынесены за DI-compatible runtime
  при сохранении совместимости zero-value сервисов.
- [P5] Завершён cleanup logging backend без изменения поведения API endpoint'ов.

#### API и runtime-поведение

- [P0] Усилена обработка trusted proxies и безопасная классификация ошибок
  импорта.
- [P1] Ужесточены потоки realtime/session/CSRF и taxonomy ошибок Telegram.
- [P2] Нормализованы batching и timeout-поведение на тяжёлых data-path.
- [P3] Добавлен начальный `slog` adapter-path для поэтапной миграции с
  `op/go-logging`.
- [P4] `slog` стал основным logger facade; `op/go-logging` изолирован за
  deprecated compatibility API.
- [P5] Удалены deprecated `logger.InitLogger`/`logger.GetLogger`; вывод logger
  facade полностью переведён на стандартный `log/slog` с сохранением panel/core
  log-buffer.
- [P5] Legacy-модуль `github.com/op/go-logging` удалён из `go.mod` и `go.sum`.
- [P4] Добавлена проверяемая policy revalidation для sing-box tracker под
  `github.com/sagernet/sing-box v1.13.11`.
- [P4] Добавлена проверяемая SemVer release/version policy; migration code
  больше не даунгрейдит future `settings.version`.

#### Frontend

- [P1] Исправлен Vitest harness в `frontend/vitest.config.ts`.
- [P1/P2] Согласованы очистка CSRF-кэша, границы request dedupe и поведение
  realtime degraded-mode.

#### Тесты и верификация

- Baseline и фазовые отчёты:
  - `plans/lint-baseline.txt`
  - `plans/lint-baseline-normalized.txt`
  - `plans/fix-validation.txt` (P0)
  - `plans/p1-validation.txt` (P1)
  - `plans/p2-validation.txt` (P2)
  - `plans/p3-architecture-validation.txt` (P3)
  - `plans/p4-architecture-debt-validation.txt` (P4)
  - `plans/p5-logging-cleanup-validation.txt` (P5)
- Для каждой фазы есть точечные проверки и финальный pass-набор команд в
  соответствующем validation-артефакте.

### Трассируемость (multi-chat policy)

- Каждый завершённый пункт помечается фазовым тэгом: `[P0]`, `[P1]`, `[P2]`,
  `[P3]`, `[P4]`, `[P5]`.
- Добавляйте ссылки в формате: `(ref: <commit|PR|chat-id>)`.
- Для сквозных изменений используйте объединённые тэги, например `[P1/P2]`.
- Отложенный архитектурный долг держите отдельным блоком, не смешивая с
  завершёнными изменениями.

### Замечания по обновлению (агрегированное окно)

- Рассматривайте P0->P5 как единое релизное окно; перед апгрейдом делайте
  полный SQLite-бэкап.
- До production rollout проверьте в staging изменения в session/CSRF/realtime
  и listen fallback.
- Используйте фазовые validation-файлы выше как доказательство верификации
  апгрейда.
- Внешним Go-интеграциям, импортировавшим `logger.InitLogger` или
  `logger.GetLogger`, нужно перейти на `logger.Init(logger.Level*)`,
  `logger.Slog(source)` или `slog.Default()`.

### Откат (агрегированное окно)

- Для отката восстановите pre-window SQLite snapshot и предыдущий binary/image.
- Если откат пересекает изменения поведения session/token, после downgrade
  инвалидируйте активные сессии и ротируйте admin credentials.

### Отложенный архитектурный долг

- [P5] В рамках P5 deferred-пунктов нет. Legacy-зависимость `op/go-logging` и
  deprecated logger compatibility API удалены.

### Шаблон для следующих multi-chat релизов

- Используйте доменные секции: Security, Reliability/Data integrity,
  API/Runtime, Frontend, Tests.
- Маркируйте каждый пункт фазовыми тэгами и добавляйте traceability refs.
- Явно добавляйте `Upgrade notes` и `Rollback` для всего агрегированного окна.

### Исправлено

- TUIC subscription/share links и Clash export теперь включают
  `udp_relay_mode`, сохраняя заданное значение и используя `quic` по умолчанию
  для generated links, если режим не задан.

### Добавлено

- Scheduled и manual encrypted backup SQLite-БД в Telegram. Backup passphrase
  задаётся только во вкладке Telegram, фича выключена по умолчанию. Новые
  settings и дефолты: `telegramBackupEnabled="false"`,
  `telegramBackupPassphrase=""`, `telegramBackupCron=""`,
  `telegramBackupExcludeTables="stats,client_ips,audit_events,changes"` и
  `telegramBackupMaxSizeMB="45"`. Новые manual trigger маршруты:
  `POST /api/telegram/backup/run` и `POST /apiv2/telegram/backup/run`.
- Restore теперь автоматически распознаёт загружаемые backup envelope с magic
  `SUI-TGBKP\x00` и показывает поле Backup passphrase в Backup & Restore.
  Plaintext `.db` по-прежнему принимается без этого поля.
- Существующая кнопка Backup может опционально скачать тот же encrypted
  envelope через чекбокс «Encrypt with Telegram backup passphrase». Чекбокс
  по умолчанию снят, plaintext-поведение сохранено, а существующий endpoint
  `getdb` использует новый non-breaking query-параметр
  `encryptTelegramBackup=true`.
- Основной бинарник релиза теперь включает `s-ui decrypt-backup` для offline
  расшифровки envelope. Отдельный артефакт не нужен.
- `docs/scope-matrix.md` документирует операцию `tg_backup_run`.

### Изменено

- BREAKING: legacy `POST /api/telegram/backup` и
  `POST /apiv2/telegram/backup` теперь делегируют в новый Telegram backup
  service. `backupKey` удалён из всех ответов, требуется
  `telegramBackupEnabled=true`, успешный ответ дополнен `trigger="manual"`.
  Периода обратной совместимости нет. Строгий миграционный шаг: после апгрейда
  включить `telegramBackupEnabled` во вкладке Telegram, иначе legacy-вызов
  вернёт HTTP 503 с `errorClass=disabled`.
- В `util/secretbox` добавлены `EncryptBytes` и `DecryptBytes` для работы с
  секретами как с байтами.
- В `api/rateLimit.go` добавлен общий manual Telegram backup bucket для всех
  четырёх manual trigger маршрутов: 3 попытки за 60 секунд с `Retry-After`.
- Новые типы audit-events: `tg_backup_sent`, `tg_backup_failed`,
  `tg_backup_passphrase_changed`, `tg_backup_manual_encrypted` и
  `tg_backup_restore_failed`.

### Замечания по обновлению

- Сделайте бэкап SQLite-БД перед апгрейдом. При работе через systemd
  остановите `s-ui`, скопируйте `s-ui.db` плюс `-wal`/`-shm`-сайдкары и
  затем запустите сервис снова.
- Telegram database backup остаётся выключенным, пока во вкладке Telegram не
  включён `telegramBackupEnabled` и не задан Backup passphrase.
- Интеграции, вызывающие legacy Telegram backup endpoints, должны учесть
  удаление поля `backupKey` и новый HTTP 503 `disabled`, пока настройка не
  включена.

### Откат

- Для отката восстановите SQLite-бэкап, сделанный перед апгрейдом, и прежний
  бинарник/image.
- Зашифрованные `.db.aes` файлы остаются расшифровываемыми тем passphrase,
  которым были созданы, через любой бинарник с `s-ui decrypt-backup`.

## [1.5.2-beta-hotfix2] — 2026-05-18 — снятие legacy unique-индекса client_ips

### Исправлено

- Ошибка `UNIQUE constraint failed: client_ips.client_name, client_ips.ip`
  при автобэкапе перед миграцией с 3x-ui. С 1.5.x колонка
  `client_ips.ip` — legacy-поле только для backfill, для новых строк
  пустое; настоящий уникальный ключ — `(client_name, ip_hash)`. В
  модели оставался устаревший `gorm:"index:idx_client_ips_client_ip,unique"`
  на `(client_name, ip)`, и `database/backup.go` через `AutoMigrate`
  пересоздавал плохой индекс во временной backup-БД, после чего
  чанковая копия `client_ips` падала, как только у клиента было больше
  одной строки с пустым `ip`. После этого hotfix'а единственный
  unique-индекс модели — `(client_name, ip_hash)`.

### Изменено

- `database/model/model.go` — убран тег
  `idx_client_ips_client_ip,unique` с `ClientIP.ClientName` и
  `ClientIP.IP`.
- `cmd/migration/1_5.go` — миграция ветки `1.5` теперь дропает
  устаревший `idx_client_ips_client_ip` и создаёт partial non-unique
  `idx_client_ips_client_legacy_ip ON client_ips(client_name, ip)
  WHERE ip IS NOT NULL AND ip != ''` для быстрых legacy-lookup.
  Миграция полностью идемпотентна (`DROP INDEX IF EXISTS` /
  `CREATE INDEX IF NOT EXISTS`), поэтому уже обновлённые до
  `1.5.2-beta` инсталлы чисто прогонят её повторно, когда runner
  снова войдёт в ветку `1.5` при следующем старте.
- `database/db.go: ensureIndexes` — дропает устаревший unique-индекс
  на каждом `InitDB`. Это рантайм-страховка для случаев, когда
  `MigrateDb` обходится (например, восстановление старого бэкапа мимо
  панели), и одновременно гарантирует, что временная backup-БД из
  `GetDb("")` не получит плохой индекс.

### Замечания

- Новых колонок, таблиц, настроек, endpoint'ов, scope'ов и переменных
  окружения нет. Совмещается с чанковыми helper'ами из предыдущего
  hotfix'а.
- Регресс:
  - `cmd/migration/migration_1_5_test.go` — падает, если устаревший
    индекс снова создаётся, и принимает несколько строк с пустым `ip`
    для одного клиента.
  - `database/db_test.go: TestInitDBDropsObsoleteClientIPUniqueIndex` —
    поднимает БД старой формы с уже созданным legacy-индексом и
    проверяет, что `InitDB` его убирает.
  - `database/backup_test.go: TestGetDbHandlesHashedClientIPsWithEmptyLegacyIP` —
    переносит несколько строк `ip_hash` с пустым `ip` для одного
    клиента через `GetDb("")`.

## [1.5.2-beta-hotfix] — 2026-05-18 — чанки в бэкапе и безопасность SPA при апгрейде

### Исправлено

- Ошибка `too many SQL variables` при бэкапе БД и миграции с 3x-ui на
  инсталлах с большими таблицами `stats`, `client_ips`, `audit_events`,
  `changes` или `clients`. Функция бэкапа в `database/backup.go` больше
  не генерирует один многострочный `INSERT VALUES (...)`, превышающий
  compile-time лимит SQLite (`SQLITE_MAX_VARIABLE_NUMBER = 999` в
  `mattn/go-sqlite3`). Это разблокирует `WritePreImportBackup` и миграцию
  с 3x-ui на боевых базах (≈40k+ строк в `stats`).
- Протухший `index.html` после апгрейда больше не ломает вкладку Clients.
  `/<base>/assets/*` теперь возвращает честный 404 для отсутствующих
  файлов вместо SPA-fallback'а, и браузеры не получают `text/html`
  на запрос JS-модуля (`Failed to load module script` /
  `Failed to fetch dynamically imported module`). `index.html` отдаётся
  с `Cache-Control: no-cache, no-store, must-revalidate`, хэшированные
  ассеты остаются `public, max-age=31536000, immutable`.
- Vue Router слушает `vite:preloadError` и делает один защищённый
  `window.location.reload()` (флаг в `sessionStorage` исключает петлю),
  поэтому вкладки с прошлым билдом сами подхватывают новый бандл.
- `service/client.go` (`addbulk`, `editbulk`, `ResetClients`,
  `DepleteClients`) и `database/importxui/history_routing.go` (импорт
  исторического трафика) нарезают bulk-`Save`/`Create` через новые
  helper'ы из `database/bulk.go` (`SafeSQLiteBatchSize`,
  `CreateInBatchesSafe`, `SaveInBatchesSafe`). Reset/deplete-задачи и
  импорт исторических `stats` больше не падают на инсталлах с тысячами
  клиентов.

### Замечания

- Миграций схемы, новых endpoint'ов, scope'ов и переменных окружения нет.
- Регресс в `database/backup_test.go` создаёт ≈43k строк `stats` плюс
  5k `client_ips` и проверяет, что `GetDb("")` сохраняет количество строк.

## [1.5.2-beta] — 2026-05-18 — пакет миграции из 3x-ui

### Добавлено

- Импорт конфигурации 3x-ui: CLI `s-ui import-xui`, HTTP-endpoint
  `POST /api/import-xui` и отдельная секция «Migrate from 3x-ui» в модале
  Backup & Restore. Импорт выполняется одной транзакцией с автобэкапом,
  поддерживает стратегии `merge`/`replace`/`skip` и пишет audit-события
  `xui_import`.
- Полный мастер миграции на странице `/migrate-xui`: per-object plan/apply
  с валидацией `Source.Hash`, WebSocket-события `xui_import_progress`,
  предпросмотр JSON, откат к автобэкапу и выгрузка отчёта в JSON/Markdown.
  Отчёты лежат в `audit_events.details`.
- Удалённые источники 3x-ui через `--remote ssh://...` и `--remote http://...`
  (xuihttp) и команда `s-ui sync-xui` для периодической инкрементальной
  синхронизации. SSH использует host-key TOFU и таблицу `xui_known_hosts`;
  HTTP поддерживает login-flow 3x-ui.
- Зашифрованные `xui_sync_profiles` (AES-GCM с HKDF-SHA256 от
  `config.GetSecret()`, override через `XUI_PROFILE_KEY_FILE`), миграция
  схемы `cmd/migration/1_7.go`, cron-job `xuiSyncJob` и UI
  `/migrate-xui/schedule` для управления профилями.
- Best-effort импорт исторического трафика (`client_traffics`/
  `outbound_traffics` → агрегаты в `stats`) и routing-правил Xray
  (`geosite:*`/`geoip:*`, block, direct) в sing-box `route.rules`/
  `dns.servers`. Balancers попадают в warning'и.
- Новый scope токена `xui_remote`, обязательный для всех удалённых/sync
  endpoint'ов; локальные `/api/import-xui*` остаются под `database`/`admin`.
  `XUI_DISABLE_REMOTE=1` отключает удалённые источники и cron-режим.

### Замечания

- `test-db/` содержит локальные фикстуры импорта 3x-ui с реальными боевыми
  данными и больше не трекается в репозитории (см. `.gitignore`). Тесты,
  которым нужны эти фикстуры, на CI скипаются автоматически; запускайте их
  локально, когда `test-db/` положены рядом.

## [1.5.1-beta] — 2026-05-17 — закрытие технического долга и UI

### Безопасность

- Telegram-уведомления теперь отправляются через асинхронную bounded-очередь
  с retry/backoff и audit-событиями переполнения/неудачи, поэтому handler
  логина и другие пути никогда не блокируются сетевыми сбоями Telegram.
- Payload Telegram-событий, audit-детали, история changes и caption бэкапов
  проходят через redaction: bot-токены, креденшелы прокси, API-токены и
  ключи бэкапа не пишутся в логи, audit, changes и captions.
- Realtime WebSocket handshake проверяет Origin по allow-list, применяет
  per-IP rate-limit, отвергает повторное использование одноразового токена,
  использует ping/pong heartbeat, idle-close и close-all при ротации сессий.
- `GET /api/security/audit` для API-токенов требует scope `admin`,
  ограничен по rate-limit, поддерживает cursor pagination и валидированные
  фильтры `event`/`severity`.
- `POST /api/telegram/test` требует scope `admin` для API-токенов и пишет
  audit-событие, содержащее только `success`/`errorClass`-метаданные.
- Добавлен middleware security headers для админ-панели и subscription-сервера;
  на ответах подписки выставляется `Cache-Control: no-store`.
- Пароль администратора при свежей установке больше не пишется в журнал:
  сгенерированное значение однократно сохраняется в
  `<dataDir>/initial-admin.txt` с правами только владельца, а startup выводит
  только путь к файлу.
- `s-ui admin -show` больше не выводит сохранённый password hash; команда
  показывает username и подсказку по сбросу пароля.
- Фронтенд очищает cached CSRF-token после logout, logout-all и realtime
  session-rotation close, поэтому следующий mutating request получает новый
  токен.
- `install.sh` скачивает release-файл `*.sha256` и проверяет Linux-тарбол
  через `sha256sum -c` перед распаковкой.
- Добавлен PR CI workflow для Go vet/race tests и frontend lint/unit/build
  проверок.
- Админские web-сессии переведены на server-side SQLite-store: cookie в
  браузере содержит только подписанный session ID, а данные сессии хранятся
  локально в таблице `sessions`.

### Privacy и подписки

- История клиентских IP по умолчанию хранится как соль+SHA-256 хэш,
  показ raw-IP отключён без явного opt-in, retention обслуживается cron GC.
- IP-лимит по умолчанию работает в режиме `monitor`; в `enforce` отбрасываются
  только новые сверхлимитные подключения, активные сессии не разрываются.
- Все запланированные subscription-настройки сохраняются и используются в
  link-, JSON- и Clash-ответах подписок. Subscription-пути валидируются по
  reserved-prefixes, заголовки санитизируются централизованно, per-IP
  rate-limit подписок настраивается.
- `POST /api/rotateSubSecret` ротирует per-client subscription-секрет с
  audit-событием. При `subSecretRequired=true` legacy name-URL отвечают 404.

### Telegram и observability

- Egress Telegram может идти через валидируемые HTTP/HTTPS/SOCKS5-прокси,
  настройки которых хранятся как secret-aware. Классы ошибок нормализованы
  до `unauthorized`, `chat_not_found`, `rate_limited`, `network`, `unknown`.
- Реализованы CPU-hysteresis алерты, scheduled Telegram-отчёты и зашифрованный
  экспорт БД-бэкапа в Telegram; всё остаётся opt-in.
- История observability теперь использует bounded buckets `2s`, `30s`, `1m`,
  `5m`, заполняется cron-job, API-параметры `metric`/`bucket`/`since`
  валидируются.
- `GET /api/logs` принимает ограниченные `count`, `level`, `source` и
  substring-`filter`; `GET /api/version` делает fail-soft 1h-cached
  GitHub release-check.
- Импорт/экспорт БД получают cap 64 MiB, проверку SQLite magic, временную
  staging-копию, read-only `PRAGMA integrity_check` и audit-события.

### Frontend

- Добавлен realtime-store фронта со state-машиной websocket
  reconnect/degraded и polling-fallback'ом.
- Добавлены secret-aware-поля настроек, которые показывают `••• stored •••`
  и никогда не отправляют placeholder как секрет.
- Добавлен IP-history modal с маской raw-IP по умолчанию и подтверждением
  перед показом raw-IP админу.
- Добавлены views Telegram-настроек и Audit. Audit-страница использует
  cursor pagination и server-side фильтры `event`/`severity`.

### Packaging и CI

- Docker-сборка теперь содержит аргумент `CRONET_GO_VERSION`,
  синхронизированный с `release.yml`, и документирует датированный fallback
  на upstream `latest` prebuilt-артефакт `libcronet`, пока нет
  commit-addressable assets.
- Дефолтный `TZ` Docker-образа теперь совпадает с дефолтом панели
  `Europe/Moscow`.
- Ручной release workflow теперь по умолчанию использует tag
  `v1.5.1-beta`.
- Container entrypoint больше не запускает дублирующую автоматическую
  миграцию перед стартом; для ручного migration-only запуска используйте
  `SUI_MIGRATE_ONLY=1`.
- Миграционный runner теперь выполняет SQLite WAL checkpoint только после
  успешного commit транзакции, что исправляет `database table is locked` при
  обновлении с `1.4.x` до `1.5.1-beta`.
- Админский фронтенд больше не зависит от inline-скрипта с base path:
  строгий Content Security Policy соблюдается, а кастомные web path корректно
  применяются к API, CSRF и realtime fallback-запросам.

### Тесты

- Добавлено или расширено покрытие: миграция secret-настроек, redaction,
  IP-monitor cache/enforce, audit-фильтрация и rate-limit, header-injection
  в подписках и 404 на legacy URL, realtime Origin/replay-token/heartbeat,
  миграции, frontend WS- и IP-хелперы.
- Проверки в текущем workspace: `go vet ./...`, `go test ./...`,
  `npm run test:unit`, `npm run build`, `npm run lint` — зелёные. Race-тесты
  требуют CGO и C-компилятор; на Windows-машине без `gcc` они не запустятся.

### Замечания по обновлению

- Сделайте бэкап SQLite-БД перед апгрейдом. При работе через systemd
  остановите `s-ui`, скопируйте `s-ui.db` плюс `-wal`/`-shm`-сайдкары и
  затем запустите сервис снова.
- Поддержка legacy `/apiv2/*` `Token`-заголовка остаётся временной.
  Переведите интеграции на `Authorization: Bearer <token>` до Sunset:
  `Sat, 15 Aug 2026 00:00:00 GMT`.
- Все новые фичи остаются off by default, за исключением realtime WS
  c polling-фолбэком и monitor-only IP-tracking.

## [1.5.0] — 2026-05-15 — фундамент безопасности и realtime-платформа

### Безопасность

- Добавлено действие в Admins panel «Logout all admins»: ротирует
  session generation и очищает cookie инициатора. API-токены не отзываются.
- Добавлен AES-GCM/HKDF secretbox-helper для чувствительных настроек.
  Новые secret-aware-настройки шифруются ключом `SUI_SECRETBOX_KEY` либо
  legacy ключом `settings.secret` (со startup-предупреждением).
- Secret-aware-настройки маскируются в `api/settings` как `<key>HasSecret`;
  сохранение пустого значения оставляет ранее сохранённый секрет.
- Добавлены таблица `audit_events`, redaction-helper, retention-настройка
  и эндпоинт `/api/security/audit`. Login, logout, logout-all-admins,
  смена пароля, создание/удаление API-токена пишут redacted-events.
- Добавлена CSRF-защита для browser `/api/*`-mutating-запросов.
  `GET /api/csrf` выдаёт session-bound токен, фронт шлёт его как
  `X-CSRF-Token`, при невалидном/просроченном — HTTP 403. Bearer-token
  `/apiv2/*` запросы не аффектятся.
- API-токены мигрированы из plaintext в salted SHA-256 (`installSalt`);
  новые токены показываются один раз, в БД хранится hash и prefix,
  включение/отключение через Admins UI.
- `/apiv2/*` принимает `Authorization: Bearer <token>` как основной
  способ передачи API-токена. Legacy `Token`-header работает, пишет
  audit-events, возвращает `Deprecation` и `Sunset: Sat, 15 Aug 2026
  00:00:00 GMT`.
- Добавлены per-client subscription-секреты. Поддерживаются маршруты
  `/sub/<secret>`, `/sub/json/<secret>`, `/sub/clash/<secret>`,
  `/json/<secret>`, `/clash/<secret>`; legacy `/sub/<name>` остаётся
  включённым пока `subSecretRequired=false`.
- Subscription-эндпоинты санитизируют response-заголовки, валидируют
  настроенные subscription-пути и применяют per-IP rate-limit.

### API

- Добавлены grouped placeholders для будущих 1.5.0-маршрутов
  (security/notification/observability/bulk outbound-check) с
  сохранением одноуровневых `/api/<action>`.
- Добавлены `GET /api/observability/history`,
  `GET /api/observability/core-history`, `GET /api/version`.
- Добавлен `POST /api/checkOutbounds` — bounded-bulk-проверка
  outbounds: concurrency 8, timeout 5s per outbound, общий 60s,
  валидатор HTTPS/public-IP target.
- Добавлен disabled-by-default Telegram-сервис и
  `POST /api/telegram/test`. Bot-token и proxy-настройки —
  secret-aware. Login, logout-all-admins и core-restart события
  оповещают только при включённом Telegram.
- Добавлена основа authenticated realtime WebSocket
  (`/api/realtime/ws-token`, `/api/realtime/ws`) с одноразовыми
  токенами, bounded client queues, per-user/per-IP лимитами и
  polling-фолбэком на фронте. `logoutAllAdmins` закрывает активные
  realtime-сокеты с close code `4401`.
- Добавлен batched IP-monitoring клиента с `client_ips`, per-client
  `limitIp` и `ipLimitMode`, last-online/IP-count метаданными,
  audited clear-action из Admins и UI-контролами в Clients.
  `monitor` — режим по умолчанию; `enforce` отбрасывает только новые
  сверхлимитные подключения и не разрывает активные.

### Локализация

- `install.sh` и `s-ui` management-меню также предлагают китайский
  как пункт **3. 中文**; `SUI_LANG=zh` поддерживается для
  non-interactive установок.

## [1.4.3] — 2026-05-15 — обновление sing-box runtime

Этот выпуск обновляет встроенный sing-box runtime с `v1.13.4` до
`v1.13.11` и оставляет панель, REST API, формы фронта и схему БД
неизменными.

### Runtime

- Обновлено `github.com/sagernet/sing-box` до `v1.13.11`.
- Принят соответствующий upstream-набор зависимостей: `sing v0.8.9`,
  `sing-tun v0.8.9`, `sing-quic v0.6.1` и апрельские 2026
  `cronet-go`-модули, нужные NaiveProxy.
- Linux release-workflow закреплён на полный SHA коммита `cronet-go`
  `e4926ba205fae5351e3d3eeafff7e7029654424a`, чтобы релизные сборки
  не опирались на короткий префикс.

### Совместимость и безопасность

- Миграция БД не требуется; хранимый JSON inbound/outbound/endpoint/service
  остаётся совместимым с `sing-box v1.13.11`.
- Новых полей в Web UI не добавлено: 1.13.5–1.13.11 содержат только
  фиксы и runtime-обновления, включая fake-ip DNS fix, NaiveProxy
  update и process searcher regression fix.
- Production-апгрейд должен использовать полный release-архив или
  пересобранный image, чтобы обновлённый `libcronet.so`/`libcronet.dll`
  оставался синхронен с новым бинарём.

### Verification

- `go mod verify`
- `go test ./...`
- `go test -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,with_tailscale" ./...`

## [1.4.2-beta] — 2026-05-14 — security and reliability hardening

Этот выпуск переписывает значительную часть слоя авторизации, транзакций
и запуска ядра, защищает загрузчик внешних подписок от SSRF, делает
импорт легаси-бэкапов и обновление 1.x → 1.4.2 безопасным «поверх», а
также добавляет двуязычный установщик и меню управления.

### Безопасность

- Хранение паролей через bcrypt с автоматической миграцией
  plaintext → bcrypt при первом успешном логине.
- Никаких `admin/admin` по умолчанию: при свежей установке генерируется
  случайный 24-символьный пароль и однократно выводится в журнал.
- Лимит входа (5 неуспешных попыток / 15 минут / блок 15 минут) с
  ограниченным потреблением памяти.
- `X-Forwarded-For` учитывается только если задана переменная
  `SUI_TRUSTED_PROXIES`; цепочка обходится справа налево, чтобы
  крайнее левое (поддельное) значение не могло обойти IP-логику.
- Защищённые cookie сессии: `HttpOnly` + `SameSite=Lax` + `Secure`
  при HTTPS.
- Параметризованный SQL и whitelist идентификаторов в выборке
  пользователей по inbound.
- Загрузчик внешних подписок: только http/https, блок приватных
  адресов, лимит размера 4 МиБ, защита от DNS rebinding (повторная
  валидация IP при dial); опционально включается через
  `SUI_ALLOW_PRIVATE_SUB_URLS=true`.
- Domain validator стал case-insensitive и корректно работает с IPv6
  host literals.

### Надёжность

- Бэкап включает таблицы `services` и `tokens`.
- Восстановление бэкапа от 1.4.x работает корректно: WAL/SHM
  сайдкары живой БД больше не «портят» загруженный файл.
- WARP: новый эндпоинт `v0a4005`, заголовки реального клиента,
  TLS 1.2+, фоллбэк на `v0a2158`, ретраи; больше не падает с
  «TLS handshake timeout» / «EOF» на средних каналах.
- Защита от EADDRNOTAVAIL после переноса базы: если в `webListen` /
  `subListen` сохранён IP, которого нет на сервере, панель пишет
  предупреждение и слушает на всех интерфейсах вместо краша.
- Индексы для горячих запросов `stats`, `changes`, `clients`.
- SQLite-пул: `MaxOpen=8`, `MaxIdle=4`, `_busy_timeout=10000` —
  избавились от штормов `SQLITE_BUSY` при записи статистики.
- Транзакционные коммиты проверяются; runtime-изменения core
  применяются только после успешного commit'а.
- Пользовательские рестарты ядра обходят cron-cooldown, ошибки
  старта корректно прокидываются наверх.
- Чистая (race-free) синхронизация: жизненный цикл core, online
  stats, last-update, хранилище токенов v2.
- HTTP-серверы получили `Read/Write/Header/Idle` таймауты и
  `tls.MinVersion = 1.2`.

### Импорт легаси-бэкапов и обновление

- `migration.MigrateDb` возвращает ошибку вместо `log.Fatal` —
  ошибка миграции больше не убивает процесс панели.
- `ImportDB` возвращает БД к предыдущему состоянию при ошибке миграции.
- Новый `database.AdaptToCurrentVersion` запускается после каждого
  `InitDB` и импорта: перешивает plaintext-пароли в bcrypt, обновляет
  индексы, поднимает `settings.version`.
- `app.Init` запускает миграции до открытия БД, поэтому новый
  бинарник поверх существующей базы 1.x обновляет её автоматически
  при первом старте.

### Фронтенд / тулинг

- ESLint flat-config; `lint` без авто-фикса.
- 0 уязвимостей по `npm audit --audit-level=high`.
- Axios подключён через экспортируемый instance, `AbortController`
  вместо устаревшего `CancelToken`, дедуп ограничен GET/HEAD/OPTIONS.
- `v-html` убран из логов, импорта правил, IP-листов, gauge-плитки.
- Code splitting восстановлен; исправлено распространение
  `enableTraffic=false`.
- Роутер больше не пытается читать HttpOnly-cookie через
  `document.cookie` — фикс «после логина выкидывает на /login».

### Локализация и значения по умолчанию

- `install.sh` и меню `s-ui` теперь двуязычные (английский /
  русский). Язык выбирается при первом запуске и сохраняется в
  `/etc/s-ui/lang`. Переключить язык можно из меню (пункт 21) или
  переменной `SUI_LANG=en|ru`.
- Часовой пояс панели по умолчанию: `Europe/Moscow`.
- Локаль фронтенда по умолчанию: `en` (`zhHans` была раньше).
  Существующие браузеры сохраняют свой выбор языка из
  `localStorage`.

### Репозиторий

- Go-модуль переименован в `github.com/deposist/s-ui-rus-inst`.
- Установка/релизы и docker-образ ссылаются на
  `deposist/s-ui-rus-inst` / `ghcr.io/deposist/s-ui-rus-inst`.

## Гайд по обновлению (русский)

Обновление можно делать прямо поверх, без потери данных и без полной
перенастройки. При старте панели автоматически выполняется
`cmd/migration` → `database.AdaptToCurrentVersion`: схема БД
подтягивается до актуальной версии, добавляются недостающие индексы,
все ваши настройки, inbounds/outbounds/клиенты/tls/сервисы и
API-токены остаются на месте, а пароль админа в открытом виде
автоматически перешьётся в bcrypt при первом успешном логине. Бэкапы,
сделанные на старых версиях S-UI (1.0/1.1/1.2/1.3), можно восстановить
напрямую через панель — миграция применяется к загруженному бэкапу в
том же потоке.

1. Сделайте бэкап на всякий случай:
   - через панель: **Backup → Backup**, сохраните файл `s-ui_*.db`;
   - либо скопируйте файл вручную: `cp /usr/local/s-ui/db/s-ui.db /root/s-ui.db.bak`.
2. Остановите сервис: `systemctl stop s-ui`.
3. Замените бинарник или docker-образ на новую сборку:
   - вручную: распакуйте свежий архив в `/usr/local/s-ui/`;
   - docker: поменяйте тег образа на `ghcr.io/deposist/s-ui-rus-inst`
     и выполните `docker compose pull && docker compose up -d`.
4. Запустите сервис: `systemctl start s-ui`.
5. Зайдите в панель так же, как раньше. Пароль будет автоматически
   заменён на bcrypt-хеш после первого успешного логина — никаких
   ручных действий не нужно.

После апгрейда стоит проверить:

- Если панель работает за reverse-proxy и вам важно видеть реальный IP
  клиента в логах входа, выставьте переменную окружения
  `SUI_TRUSTED_PROXIES` со списком CIDR ваших прокси (например
  `127.0.0.1/32,10.0.0.0/8`). Без этой переменной заголовок
  `X-Forwarded-For` игнорируется и в журналах будет адрес прокси.
- Если внешние подписки берутся с локального адреса
  (`http://127.0.0.1:…/sub`), выставьте `SUI_ALLOW_PRIVATE_SUB_URLS=true`.
- Если вы устанавливали панель старым скриптом (`deposist/s-ui`),
  один раз обновите его на новый репозиторий:
  `wget -O /usr/bin/s-ui https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh && chmod +x /usr/bin/s-ui`.
- Лимит входа — 5 неуспешных попыток с одного IP за 15 минут блокируют
  IP на 15 минут. Если вы вводили пароль с ошибками много раз, подождите
  блок-окно или перезапустите сервис, чтобы счётчик сбросился.

## Откат

Если что-то пошло не так, достаточно восстановить бэкап:

1. `systemctl stop s-ui`.
2. `cp /root/s-ui.db.bak /usr/local/s-ui/db/s-ui.db`.
3. Восстановите предыдущий бинарь или верните `docker compose` на
   предыдущий тег образа.
4. `systemctl start s-ui`.

Префикс bcrypt в колонке `users.password` совместим с предыдущим бинарём
в том смысле, что старый бинарь просто не сматчит хешированный пароль —
в этом случае `s-ui admin -reset` восстанавливает известные креденшелы.
Данные в безопасности; на откате может потребоваться только CLI-сброс
пароля админа.
