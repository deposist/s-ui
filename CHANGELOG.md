# Changelog

All notable changes to this project are documented in this file.

## [1.5.0] - 2026-05-15 - security foundation and realtime platform

### Security

- Added an Admins panel action to invalidate all admin web sessions at once.
  The action rotates the session generation and clears the initiator's current
  cookie; API tokens are not revoked.
- Added an AES-GCM/HKDF secretbox helper for sensitive settings. New
  secret-aware settings are encrypted with `SUI_SECRETBOX_KEY` when set, or
  with the legacy `settings.secret` compatibility key with a startup warning.
- Secret-aware settings are masked from `api/settings` as `<key>HasSecret`;
  saving an empty value keeps the previously stored secret.
- Added the `audit_events` table, redaction helper, retention setting, and
  `/api/security/audit` endpoint. Login, logout, logout-all-admins, credential
  changes, and API token create/delete actions now write redacted audit events.
- Added CSRF protection for browser `/api/*` mutating requests. `GET /api/csrf`
  issues a session-bound token, frontend requests send it as `X-CSRF-Token`,
  and invalid or expired tokens return HTTP 403. Bearer-token `/apiv2/*`
  requests are not affected.
- API tokens are now migrated from plaintext to salted SHA-256 hashes using
  the per-install `installSalt`; new tokens are shown only once, stored as
  hash/prefix metadata, and can be enabled or disabled from the Admins UI.
- `/apiv2/*` now accepts `Authorization: Bearer <token>` as the primary API
  token transport. The legacy `Token` header still works, emits audit events,
  and returns `Deprecation` plus `Sunset: Sat, 15 Aug 2026 00:00:00 GMT`.
- Added per-client subscription secrets. New `/sub/<secret>`,
  `/sub/json/<secret>`, `/sub/clash/<secret>`, `/json/<secret>`, and
  `/clash/<secret>` routes are supported; legacy `/sub/<name>` remains enabled
  until `subSecretRequired=true`.
- Subscription endpoints now sanitize response headers, validate configured
  subscription paths, and apply a per-IP rate limit.

### API

- Added grouped API route placeholders for the `1.5.0` security,
  notification, observability, and bulk outbound-check work while preserving
  the existing one-level `/api/<action>` endpoints.
- Added `GET /api/observability/history`,
  `GET /api/observability/core-history`, and `GET /api/version`.
- Added `POST /api/checkOutbounds` for bounded bulk outbound checks with
  concurrency `8`, per-outbound timeout `5s`, total timeout `60s`, and an
  HTTPS/public-IP target validator.
- Added disabled-by-default Telegram notification service and
  `POST /api/telegram/test`. Bot token and proxy-related settings are
  secret-aware; login, logout-all-admins, and core restart events notify only
  when Telegram is explicitly enabled.

### Localization

- `install.sh` and the `s-ui` management menu now also offer Chinese as
  option **3. 中文**; `SUI_LANG=zh` is supported for non-interactive installs.

## [1.4.3] - 2026-05-15 - sing-box runtime update

This release updates the embedded sing-box runtime from `v1.13.4` to
`v1.13.11` and keeps the panel, REST API, frontend forms, and database
schema unchanged.

### Runtime

- Updated `github.com/sagernet/sing-box` to `v1.13.11`.
- Accepted the matching upstream dependency set, including `sing v0.8.9`,
  `sing-tun v0.8.9`, `sing-quic v0.6.1`, and the April 2026 `cronet-go`
  modules required by NaiveProxy.
- Pinned the Linux release workflow to the full `cronet-go` commit
  `e4926ba205fae5351e3d3eeafff7e7029654424a` so release builds do not use a
  short commit prefix for the source checkout.

### Compatibility and Security

- No database migration is required; stored inbound/outbound/endpoint/service
  JSON remains compatible with `sing-box v1.13.11`.
- No web UI fields were added because `sing-box 1.13.5` through `1.13.11`
  only contain fixes and runtime updates, including the fake-ip DNS fix,
  NaiveProxy update, and process searcher regression fix.
- Production upgrades should deploy the full release archive or rebuilt image
  so the updated `libcronet.so`/`libcronet.dll` stays in sync with the new
  binary.

### Verification

- `go mod verify`
- `go test ./...`
- `go test -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,with_tailscale" ./...`

## [1.4.2-beta] — 2026-05-14 — security and reliability hardening

This release rewrites large parts of the auth, transaction, and runtime
control flow, hardens the external-subscription fetcher against SSRF,
and renames the Go module to `github.com/deposist/s-ui-rus-inst`.

The full backend test suite (`go test`, `go test -race`,
`go test -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale"`)
and the full frontend pipeline (`npm ci`, `npm run build`, `npm run lint`,
`npm audit --audit-level=high`) pass clean.

---

### Highlights

- Plaintext passwords replaced with bcrypt; existing accounts migrate
  transparently on first successful login.
- First-run admin password is randomly generated and printed once to the
  application log (no more shipped `admin/admin`).
- Login rate limiter (5 failures / 15 minutes / 15 minutes block) with
  bounded memory.
- Bilingual (English/Russian) `install.sh` and `s-ui` management menu;
  language pickable on first run, switchable from menu item **21.
  Language**, persisted in `/etc/s-ui/lang`. Default language is English.
- Default panel timezone changed from `Asia/Shanghai` to `Europe/Moscow`.
- Default frontend locale changed from Simplified Chinese to English
  (existing installations keep their saved `localStorage.locale`).
- External subscription URL fetcher rejects private/loopback/link-local
  targets and re-validates the resolved IP at dial time, blocking
  DNS-rebinding attacks.
- Configuration saves no longer leave the panel and sing-box out of sync
  on commit/start failures.
- Race-free core lifecycle, online-stats tracking, last-update
  bookkeeping, and v2 token store.
- Frontend code splitting re-enabled; `v-html` removed from the
  remaining surfaces; `AbortController` replaces deprecated
  `axios.CancelToken`.

### Breaking / behaviour changes

- **Module path**: `github.com/admin8800/s-ui` → `github.com/deposist/s-ui-rus-inst`.
  Source consumers must update imports. Pre-built binaries are unaffected.
- **Default admin password**: on a fresh database, a random 24-character
  password is generated. Look for the line
  `created initial admin user. username=admin password=...` in the
  application log on first start. **Existing databases keep their
  configured admin user**; nothing is reset.
- **`X-Forwarded-For`**: ignored unless `SUI_TRUSTED_PROXIES` lists the
  immediate client. When set, the chain is walked **right-to-left** and
  the first non-trusted hop wins. Previously the leftmost (easily
  spoofed) value was returned.
- **Login lockout**: 5 failed logins from the same client IP within 15
  minutes block that IP for 15 minutes.
- **Subscription fetcher TLS**: `InsecureSkipVerify` was removed.
  Self-signed origins must now use a certificate trusted by the system
  store.
- **Subscription fetcher private targets**: blocked by default. Set
  `SUI_ALLOW_PRIVATE_SUB_URLS=true` to opt back in (e.g. for `127.0.0.1`
  origins on the same host).
- **Sub fetcher size cap**: responses larger than 4 MiB are rejected.
- **Cookie store**: cookies are now `HttpOnly`, `SameSite=Lax`, and
  `Secure` when the request is HTTPS (directly or via a trusted proxy
  that sent `X-Forwarded-Proto: https`).
- **Frontend dedupe**: only `GET`/`HEAD`/`OPTIONS` requests are deduped;
  concurrent mutating requests no longer cancel each other.

### Security
| Severity | Change |
| --- | --- |
| High | Replaced plaintext password storage with bcrypt hashes (`util/common/password.go`). Existing entries are detected via the `bcrypt:` prefix or the `$2[aby]$` cost markers. |
| High | Lazy migration: a successful login with an unhashed password updates the DB record to a bcrypt hash. |
| High | Fixed `admin/admin` default removed; first-run admin password is randomly generated by `common.Random(24)` and logged once (`database/db.go.initUser`). |
| High | Login rate limiter introduced (`api/rateLimit.go`), with periodic state pruning and a hard cap of 4096 tracked keys to prevent unbounded memory growth. |
| High | Hardened session cookies with `HttpOnly`, `SameSite=Lax`, and HTTPS-aware `Secure` (`api/session.go`). |
| High | `X-Forwarded-For` is only consulted when `SUI_TRUSTED_PROXIES` is set; the parser now walks the chain right-to-left and returns the first non-trusted hop instead of the easily spoofed leftmost value (`api/utils.go`). |
| High | Replaced unsafe SQL string concatenation with parameterized queries in `service/config.go.GetChanges` and `service/config.go.CheckChanges`. |
| High | Static identifier allow-list inside the inbound user-fetch SQL builder (`service/inbounds.go.fetchUsersByCondition`) so future inbound types cannot become a SQL-injection vector. |
| High | Removed default TLS verification bypass for external subscription fetches (`util/subToJson.go`). |
| High | External subscription URL validation: HTTP/HTTPS only, blocks `localhost`/private/link-local/multicast/unspecified by default, opt-in via `SUI_ALLOW_PRIVATE_SUB_URLS=true`, response capped at 4 MiB. |
| High | DNS-rebinding-resistant dialer: a custom `http.Transport.DialContext` re-validates each resolved IP and dials the validated address directly, so an attacker DNS that swaps records between validation and dial cannot escape the filter. |
| Medium | Replaced `error` swallowing in `WarpService.getWarpInfo`/`RegisterWarp`/`SetWarpLicense` with explicit status-code and JSON-parse checks; replaced manual JSON formatting with `encoding/json` to avoid escaping bugs. |
| Medium | Domain validator middleware now compares case-insensitively and handles bare IPv6 hosts. |

### Reliability / data integrity
- Backup export now includes the `services` and API `tokens` tables (`database/backup.go`).
- Backup import (UI: **Backup → Restore**) now also runs the schema migrations and the post-migration adapter (`database.AdaptToCurrentVersion`) automatically. Old backups (S-UI 1.0/1.1/1.2/1.3 layouts, plaintext passwords, missing `services`/`tokens` tables, missing `version` row) are upgraded to the current shape on the fly. If migration fails, the previous live database is restored and an error is returned to the panel — no half-migrated state on disk.
- Schema migrations (`cmd/migration`) now return errors instead of calling `log.Fatal`, so a bad import no longer kills the panel process; the version pin is upserted instead of expecting an existing row.
- The same migration + adaptation pipeline runs at panel start (`app.Init`), so a fresh binary on top of an existing 1.x database upgrades automatically.
- Added `database.AdaptToCurrentVersion`, an idempotent post-migration step that:
  - rehashes any plaintext passwords with bcrypt (legacy backups before this fork shipped them in clear);
  - re-applies the new `idx_stats_lookup`/`idx_changes_lookup`/`idx_clients_name` indexes;
  - bumps the `settings.version` row to the build version so the migration runner short-circuits next time.
- Database path construction uses `filepath.Join` instead of string concatenation.
- Database init creates `idx_stats_lookup`, `idx_changes_lookup`, and `idx_clients_name` indexes for the hottest queries (`database/db.go.ensureIndexes`).
- SQLite connection pool tuned: `SetMaxOpenConns(8)`, `SetMaxIdleConns(4)`, `SetConnMaxLifetime(time.Hour)`, with `_busy_timeout=10000` and `_journal_mode=WAL` already in the DSN. Avoids `SQLITE_BUSY` storms during stats inserts.
- Transaction commits in `service.config.Save`, `service.stats.SaveStats`, and `service.client.DepleteClients` are checked; a failed commit is now reported up the call chain instead of being silently dropped.
- Configuration saves only mutate sing-box runtime state **after** a successful DB commit. The previous behaviour could end with a runtime change applied but a rolled-back DB.
- User-driven core restarts (`RestartCore`) bypass the cron cooldown so the API reflects the real start status. The cron `CheckCoreJob` continues to respect the cooldown.
- Inbound restart and `GetSingboxInfo` are now nil-safe against a concurrent core stop/start (previously could panic with `nil pointer dereference` on `corePtr.GetInstance().ConnTracker()`).
- Race-detector-clean synchronization around:
  - API tokens (`api/apiV2Handler.go`, now a `map[string]TokenInMemory` with O(1) lookup).
  - Online stats (`service/stats.go.onlineResources`) — readers receive a deep copy under `RWMutex`.
  - Core running state and instance pointer (`core/main.go.Core`).
  - Last-update bookkeeping (`service/config.go.LastUpdate`).
- HTTP server now sets `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `tls.Config.MinVersion = tls.VersionTLS12` for both the panel and the subscription server.

### Frontend / tooling
- Fixed `npm ci` by syncing `package-lock.json`.
- Migrated ESLint to flat config (`frontend/eslint.config.mjs`).
- Lint script now reports without auto-fixing (`"lint": "eslint ."`).
- `npm audit --audit-level=high` reports 0 vulnerabilities.
- Axios setup moved onto the exported instance; deprecated `CancelToken` replaced with `AbortController`. Dedupe limited to idempotent reads.
- Removed unsafe `v-html` from `Logs.vue`, `RuleImport.vue`, the IP lists in `Main.vue`, and the gauge tile (`components/tiles/Gauge.vue`).
- Fixed `enableTraffic=false` not propagating to the store, `loadClients` crashing on empty results, and the unused filtered status request list in `Main.vue.reloadData`.
- Re-enabled Vite code splitting; bundle output uses `[hash].js`/`[hash].css` filenames.

### Localization & defaults
- `install.sh` and the `s-ui` management menu are now bilingual
  (English / Russian). On first run the user is asked to pick a
  language; the choice is stored in `/etc/s-ui/lang` and reused on
  subsequent runs. `SUI_LANG=en|ru` overrides interactively or in CI.
- Added menu item **21. Language** so the user can switch UI language
  without editing files.
- Default `timeLocation` for the panel changed from `Asia/Shanghai`
  to `Europe/Moscow`.
- Default frontend locale (and Vuetify locale) changed from
  `zhHans` (Simplified Chinese) to `en`. The user-selected locale
  saved in `localStorage` is still honoured, so existing browsers
  keep their language.

### Repository / packaging
- Go module renamed to `github.com/deposist/s-ui-rus-inst`; all internal imports updated.
- `frontend/go.mod` keeps root-level `go` commands away from `frontend/node_modules`.
- README, `install.sh`, `s-ui.sh`, `docker-compose.yml` updated to point at `https://github.com/deposist/s-ui-rus-inst` and `ghcr.io/deposist/s-ui-rus-inst`.

### Tests
New regression tests:
- `util/common/password_test.go` — hashing, plaintext detection, migration flag.
- `util/subToJson_test.go` — URL validation rejects `file://`, `localhost`, RFC1918, IPv6 loopback; opt-in restores private targets.
- `util/subToJson_dial_test.go` — dialer hook rejects loopback addresses post-validation; opt-in allows them.
- `service/setting_test.go` — default port omission for `subURI`.
- `database/backup_test.go` — backup includes `services` and `tokens`.
- `database/adapt_test.go` — legacy plaintext password rehashing during import is correct, idempotent, and bumps `settings.version`.
- `api/rateLimit_test.go` — block on max failures, reset clears state, concurrent access.
- `api/utils_test.go` — XFF parsing matrix (untrusted client, rightmost untrusted hop, all-trusted fallback, spoofed XFF from untrusted client).

### Verification

| Command | Result |
| --- | --- |
| `go build ./...` | ✅ |
| `go vet ./...` | ✅ |
| `go test -count=1 ./...` | ✅ |
| `go test -count=1 -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale" ./...` | ✅ |
| `go test -race -count=1 ./...` | ✅ (requires CGO and a C compiler, e.g. `C:\msys64\ucrt64\bin\gcc.exe`) |
| `npm ci` | ✅ |
| `npm run build` | ✅ |
| `npm run lint` | ✅ |
| `npm audit --audit-level=high` | ✅ (0 vulnerabilities) |

---

## Изменения 1.4.2-beta (на русском)

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

---

## Upgrade guide / Гайд по обновлению

### English (TL;DR)
You can upgrade in place without losing data or reconfiguring the server.
The DB schema is migrated automatically on every panel start
(`app.Init` → `cmd/migration` → `database.AdaptToCurrentVersion`),
existing settings/inbounds/outbounds/clients/tokens stay intact, and
plaintext admin passwords migrate to bcrypt automatically on the next
login. Backups taken from older S-UI builds (1.0/1.1/1.2/1.3) can be
restored straight from the panel and will be brought up to the current
schema in the same flow.

1. Make a backup, just in case:
   - via panel: **Backup → Backup**, save the resulting `s-ui_*.db`;
   - or copy the file: `cp /usr/local/s-ui/db/s-ui.db /root/s-ui.db.bak`.
2. Stop the service: `systemctl stop s-ui`.
3. Replace the binary or the docker image with the new build:
   - manual: extract the new tarball into `/usr/local/s-ui/`;
   - docker: bump the image tag to `ghcr.io/deposist/s-ui-rus-inst` and `docker compose pull && docker compose up -d`.
4. Start the service: `systemctl start s-ui`.
5. Log in as usual. Your password is stored in plaintext today; the
   panel hashes it transparently on first successful login.

What you should review after the upgrade:
- If the panel sits behind a reverse proxy and you relied on
  `X-Forwarded-For` (e.g. for IP audit logs), set
  `SUI_TRUSTED_PROXIES=10.0.0.0/8,192.168.0.0/16,…` to the CIDRs your
  proxy lives in. Without this variable, XFF is ignored and audit logs
  show the proxy IP instead of the real client.
- If you fetch external subscriptions from a private endpoint
  (`http://127.0.0.1:…/sub` etc.), set `SUI_ALLOW_PRIVATE_SUB_URLS=true`.
- If you used the old install / update script (`deposist/s-ui`), grab
  the new one once: `wget -O /usr/bin/s-ui https://raw.githubusercontent.com/deposist/s-ui-rus-inst/main/s-ui.sh && chmod +x /usr/bin/s-ui`.

### Русский
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

### Rollback / Откат

If something goes wrong, restoring your backup is enough:
1. `systemctl stop s-ui`.
2. `cp /root/s-ui.db.bak /usr/local/s-ui/db/s-ui.db`.
3. Either restore the previous binary or `docker compose` to the
   previous image tag.
4. `systemctl start s-ui`.

The bcrypt prefix in the `users.password` column is forward- and
backward-compatible with the old binary in the sense that the old binary
will simply not match a hashed password, in which case `s-ui admin -reset`
restores a known credential. So data is safe; only the admin password
might need a CLI reset on rollback.
